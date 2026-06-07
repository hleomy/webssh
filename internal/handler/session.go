package handler

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"webssh/internal/auth"
	"webssh/internal/config"
	"webssh/internal/middleware"
	"webssh/internal/model"
	"webssh/internal/sftpserver"
	"webssh/internal/sshclient"
)

type SessionHandler struct {
	db        *sqlx.DB
	cfg       *config.Config
	enc       *auth.EncryptionService
	sshMgr    *sshclient.Manager
	sftp      *sftpserver.SFTPService
}

func NewSessionHandler(db *sqlx.DB, cfg *config.Config, enc *auth.EncryptionService,
	mgr *sshclient.Manager, sftp *sftpserver.SFTPService) *SessionHandler {
	return &SessionHandler{db: db, cfg: cfg, enc: enc, sshMgr: mgr, sftp: sftp}
}

type ConnectRequest struct {
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Term string `json:"term"`
}

type ConnectResponse struct {
	SessionID string `json:"session_id"`
}

func (h *SessionHandler) Connect(c *gin.Context) {
	userID := middleware.GetUserID(c)
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}

	var req ConnectRequest
	c.ShouldBindJSON(&req)
	if req.Cols <= 0 {
		req.Cols = 80
	}
	if req.Rows <= 0 {
		req.Rows = 24
	}
	if req.Term == "" {
		req.Term = "xterm-256color"
	}

	server := model.Server{}
	err = h.db.Get(&server, "SELECT * FROM servers WHERE id = ? AND user_id = ?", serverID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "服务器不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	if server.Port == 0 {
		server.Port = h.cfg.SSH.DefaultPort
	}

	password, _ := h.enc.Decrypt(server.PasswordEnc)
	privateKey, _ := h.enc.Decrypt(server.PrivateKeyEnc)
	passphrase, _ := h.enc.Decrypt(server.PassphraseEnc)

	client := sshclient.NewClient(&sshclient.Config{
		Host:       server.Host,
		Port:       server.Port,
		Username:   server.Username,
		Password:   password,
		PrivateKey: privateKey,
		Passphrase: passphrase,
		AuthType:   server.AuthType,
		Timeout:    h.cfg.SSH.ConnectionTimeout,
		KeepAlive:  h.cfg.SSH.KeepAliveInterval,
	})
	if err := client.Connect(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}

	shell, err := client.NewShellSession(req.Cols, req.Rows, req.Term)
	if err != nil {
		client.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建 shell 失败: " + err.Error()})
		return
	}

	sessionID := h.sshMgr.Add(userID, serverID, client, shell)

	_, _ = h.db.Exec("UPDATE servers SET last_connect_at = ? WHERE id = ?", time.Now(), serverID)

	_, _ = h.db.Exec(`
		INSERT INTO sessions (id, user_id, server_id, name, status, cols, rows, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, userID, serverID, server.Name, model.SessionStatusConnected, req.Cols, req.Rows, time.Now(), time.Now())

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": ConnectResponse{SessionID: sessionID},
	})
}

func (h *SessionHandler) Disconnect(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少会话 ID"})
		return
	}
	userID := middleware.GetUserID(c)
	info, ok := h.sshMgr.Get(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "会话不存在"})
		return
	}
	if info.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "无权操作"})
		return
	}
	_ = h.sshMgr.Remove(sessionID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已断开"})
}

func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessions := []model.Session{}
	err := h.db.Select(&sessions, "SELECT * FROM sessions WHERE user_id = ? ORDER BY created_at DESC LIMIT 100", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": sessions})
}

func (h *SessionHandler) sftpSSHClient(c *gin.Context, serverID uuid.UUID) (*sshclient.Client, error) {
	userID := middleware.GetUserID(c)
	server := model.Server{}
	err := h.db.Get(&server, "SELECT * FROM servers WHERE id = ? AND user_id = ?", serverID, userID)
	if err != nil {
		return nil, err
	}
	if server.Port == 0 {
		server.Port = h.cfg.SSH.DefaultPort
	}
	password, _ := h.enc.Decrypt(server.PasswordEnc)
	privateKey, _ := h.enc.Decrypt(server.PrivateKeyEnc)
	passphrase, _ := h.enc.Decrypt(server.PassphraseEnc)

	client := sshclient.NewClient(&sshclient.Config{
		Host:       server.Host,
		Port:       server.Port,
		Username:   server.Username,
		Password:   password,
		PrivateKey: privateKey,
		Passphrase: passphrase,
		AuthType:   server.AuthType,
		Timeout:    h.cfg.SSH.ConnectionTimeout,
		KeepAlive:  0,
	})
	if err := client.Connect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (h *SessionHandler) SFTPList(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	files, err := h.sftp.List(client, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": files})
}

func (h *SessionHandler) SFTPUpload(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	remotePath := c.Query("path")
	if remotePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少路径"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "未上传文件"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取文件失败"})
		return
	}
	defer src.Close()

	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	if err := h.sftp.Upload(client, "", remotePath, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "上传失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "上传成功"})
}

func (h *SessionHandler) SFTPDownload(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	remotePath := c.Query("path")
	if remotePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少路径"})
		return
	}

	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	rc, err := h.sftp.Download(client, remotePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "下载失败: " + err.Error()})
		return
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取失败"})
		return
	}

	filename := remotePath
	if idx := lastIndex(remotePath, "/"); idx >= 0 {
		filename = remotePath[idx+1:]
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (h *SessionHandler) SFTPDelete(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	path := c.Query("path")
	recursive := c.Query("recursive") == "true"

	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	if err := h.sftp.Delete(client, path, recursive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "删除成功"})
}

func (h *SessionHandler) SFTPMkdir(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	path := c.Query("path")
	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	if err := h.sftp.Mkdir(client, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "创建成功"})
}

func (h *SessionHandler) SFTPRename(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	oldPath := c.Query("old")
	newPath := c.Query("new")
	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	if err := h.sftp.Rename(client, oldPath, newPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "重命名失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "重命名成功"})
}

func (h *SessionHandler) SFTPReadFile(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	path := c.Query("path")
	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	content, err := h.sftp.ReadFile(client, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"content": content}})
}

func (h *SessionHandler) SFTPWriteFile(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	client, err := h.sftpSSHClient(c, serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	decoded, _ := base64.StdEncoding.DecodeString(req.Content)
	if err := h.sftp.WriteFile(client, req.Path, string(decoded)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "保存成功"})
}

func lastIndex(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
