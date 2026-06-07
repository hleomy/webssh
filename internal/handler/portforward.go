package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"webssh/internal/auth"
	"webssh/internal/config"
	"webssh/internal/middleware"
	"webssh/internal/model"
	"webssh/internal/sshclient"
)

type PortForwardHandler struct {
	db     *sqlx.DB
	cfg    *config.Config
	enc    *auth.EncryptionService
	mgr    *sshclient.PortForwardManager
	sshMgr *sshclient.Manager
}

func NewPortForwardHandler(db *sqlx.DB, cfg *config.Config, enc *auth.EncryptionService,
	mgr *sshclient.PortForwardManager, sshMgr *sshclient.Manager) *PortForwardHandler {
	return &PortForwardHandler{db: db, cfg: cfg, enc: enc, mgr: mgr, sshMgr: sshMgr}
}

type PortForwardRequest struct {
	Name       string `json:"name" binding:"required"`
	ServerID   string `json:"server_id" binding:"required"`
	Type       string `json:"type" binding:"required,oneof=local remote"`
	LocalHost  string `json:"local_host"`
	LocalPort  int    `json:"local_port" binding:"required,min=1,max=65535"`
	RemoteHost string `json:"remote_host" binding:"required"`
	RemotePort int    `json:"remote_port" binding:"required,min=1,max=65535"`
}

func (h *PortForwardHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	forwards := []model.PortForward{}
	err := h.db.Select(&forwards, "SELECT * FROM port_forwards WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": forwards})
}

func (h *PortForwardHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req PortForwardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	serverID, err := uuid.Parse(req.ServerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效服务器 ID"})
		return
	}
	if req.LocalHost == "" {
		req.LocalHost = "127.0.0.1"
	}

	f := &model.PortForward{
		ID:         uuid.New(),
		UserID:     userID,
		ServerID:   serverID,
		Name:       req.Name,
		Type:       req.Type,
		LocalHost:  req.LocalHost,
		LocalPort:  req.LocalPort,
		RemoteHost: req.RemoteHost,
		RemotePort: req.RemotePort,
		Status:     model.PortForwardStatusStopped,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err = h.db.NamedExec(`
		INSERT INTO port_forwards (id, user_id, server_id, name, type,
			local_host, local_port, remote_host, remote_port, status, created_at, updated_at)
		VALUES (:id, :user_id, :server_id, :name, :type,
			:local_host, :local_port, :remote_host, :remote_port, :status, :created_at, :updated_at)
	`, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "创建成功", "data": f})
}

func (h *PortForwardHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	_ = h.mgr.Stop(id.String())
	_, err = h.db.Exec("DELETE FROM port_forwards WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "删除成功"})
}

func (h *PortForwardHandler) Start(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}

	f := model.PortForward{}
	err = h.db.Get(&f, "SELECT * FROM port_forwards WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "转发不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	server := model.Server{}
	err = h.db.Get(&server, "SELECT * FROM servers WHERE id = ? AND user_id = ?", f.ServerID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "关联服务器不存在"})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "SSH 连接失败: " + err.Error()})
		return
	}

	ftype := sshclient.ForwardTypeLocal
	if f.Type == model.PortForwardTypeRemote {
		ftype = sshclient.ForwardTypeRemote
	}

	info := &sshclient.ForwardInfo{
		UserID:     f.UserID,
		ServerID:   f.ServerID,
		Type:       ftype,
		LocalHost:  f.LocalHost,
		LocalPort:  f.LocalPort,
		RemoteHost: f.RemoteHost,
		RemotePort: f.RemotePort,
	}

	var startErr error
	if ftype == sshclient.ForwardTypeLocal {
		startErr = h.mgr.StartLocal(info)
	} else {
		startErr = h.mgr.StartRemote(client, info)
	}
	if startErr != nil {
		client.Close()
		_, _ = h.db.Exec("UPDATE port_forwards SET status = ? WHERE id = ?", model.PortForwardStatusError, id)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "启动失败: " + startErr.Error()})
		return
	}

	_, _ = h.db.Exec("UPDATE port_forwards SET status = ? WHERE id = ?", model.PortForwardStatusRunning, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已启动", "data": gin.H{"status": model.PortForwardStatusRunning}})
}

func (h *PortForwardHandler) Stop(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}

	f := model.PortForward{}
	err = h.db.Get(&f, "SELECT * FROM port_forwards WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "转发不存在"})
		return
	}
	_ = h.mgr.Stop(id.String())
	_, _ = h.db.Exec("UPDATE port_forwards SET status = ? WHERE id = ?", model.PortForwardStatusStopped, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已停止"})
}

type SettingHandler struct {
	db *sqlx.DB
}

func NewSettingHandler(db *sqlx.DB) *SettingHandler {
	return &SettingHandler{db: db}
}

func (h *SettingHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	key := c.Param("key")
	setting := model.Setting{}
	err := h.db.Get(&setting, "SELECT * FROM settings WHERE user_id = ? AND key = ?", userID, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"key": key, "value": ""}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": setting})
}

func (h *SettingHandler) Set(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	now := time.Now()
	_, err := h.db.Exec(`
		INSERT INTO settings (id, user_id, key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, uuid.New(), userID, req.Key, req.Value, now, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "保存成功"})
}

func (h *SettingHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	settings := []model.Setting{}
	err := h.db.Select(&settings, "SELECT * FROM settings WHERE user_id = ?", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": settings})
}
