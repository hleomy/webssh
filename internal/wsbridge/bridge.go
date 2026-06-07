package wsbridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"webssh/internal/config"
	"webssh/internal/middleware"
	"webssh/internal/sshclient"
)

type MessageType string

const (
	MsgTypeData    MessageType = "data"
	MsgTypeResize  MessageType = "resize"
	MsgTypePing    MessageType = "ping"
	MsgTypePong    MessageType = "pong"
	MsgTypeClose   MessageType = "close"
	MsgTypeError   MessageType = "error"
	MsgTypeOpen    MessageType = "open"
	MsgTypeConfirm MessageType = "confirm"
)

type Message struct {
	Type    MessageType `json:"type"`
	Data    string      `json:"data,omitempty"`
	Cols    int         `json:"cols,omitempty"`
	Rows    int         `json:"rows,omitempty"`
	Term    string      `json:"term,omitempty"`
	Message string      `json:"message,omitempty"`
}

type Bridge struct {
	conn      *websocket.Conn
	session   *sshclient.Session
	mu        sync.Mutex
	closed    bool
	writeChan chan []byte
	done      chan struct{}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func newBridge() *Bridge {
	return &Bridge{
		writeChan: make(chan []byte, 100),
		done:      make(chan struct{}),
	}
}

func HandleSSHWS(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少会话 ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	cfg := config.AppConfig
	mgr := GetSessionManager()
	info, ok := mgr.Get(sessionID)
	if !ok || info.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "会话不存在"})
		return
	}

	if info.Shell == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "终端未就绪"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	bridge := newBridge()
	bridge.session = info.Shell
	bridge.conn = conn

	go bridge.writePump()
	bridge.readPump()

	if err := mgr.Remove(sessionID); err == nil {
		_ = cfg
	}
}

func (b *Bridge) readPump() {
	defer b.close()
	b.conn.SetReadLimit(1 << 20)
	b.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	b.conn.SetPongHandler(func(string) error {
		b.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, raw, err := b.conn.ReadMessage()
		if err != nil {
			return
		}
		b.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			b.sendError("无效消息格式")
			continue
		}

		switch msg.Type {
		case MsgTypeData:
			data, err := base64.StdEncoding.DecodeString(msg.Data)
			if err != nil {
				b.sendError("数据解码失败")
				continue
			}
			if _, err := b.session.Write(data); err != nil {
				b.sendError("写入 SSH 失败")
				return
			}
		case MsgTypeResize:
			if msg.Cols > 0 && msg.Rows > 0 {
				if err := b.session.Resize(msg.Cols, msg.Rows); err != nil {
					b.sendError("调整窗口大小失败")
				}
			}
		case MsgTypePing:
			pong := Message{Type: MsgTypePong}
			b.writeJSON(pong)
		case MsgTypeClose:
			return
		}
	}
}

func (b *Bridge) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		b.close()
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := b.session.Stdout.Read(buf)
			if err != nil {
				b.mu.Lock()
				if !b.closed {
					close(b.done)
				}
				b.mu.Unlock()
				return
			}
			if n > 0 {
				encoded := base64.StdEncoding.EncodeToString(buf[:n])
				msg := Message{Type: MsgTypeData, Data: encoded}
				data, _ := json.Marshal(msg)
				select {
				case b.writeChan <- data:
				case <-b.done:
					return
				}
			}
		}
	}()

	for {
		select {
		case data, ok := <-b.writeChan:
			if !ok {
				return
			}
			b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := b.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := b.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-b.done:
			return
		}
	}
}

func (b *Bridge) writeJSON(m Message) {
	data, _ := json.Marshal(m)
	b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	b.conn.WriteMessage(websocket.TextMessage, data)
}

func (b *Bridge) sendError(msg string) {
	b.writeJSON(Message{Type: MsgTypeError, Message: msg})
}

func (b *Bridge) close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()

	if b.conn != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		b.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		b.conn.WriteMessage(websocket.CloseMessage, closeMsg)
		b.conn.Close()
	}
	if b.session != nil {
		b.session.Close()
	}
}

var sessionManager *sshclient.Manager

func SetSessionManager(m *sshclient.Manager) {
	sessionManager = m
}

func GetSessionManager() *sshclient.Manager {
	return sessionManager
}

func init() {
	_ = fmt.Sprintf
}
