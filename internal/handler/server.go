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
	"webssh/internal/middleware"
	"webssh/internal/model"
)

type ServerHandler struct {
	db   *sqlx.DB
	enc  *auth.EncryptionService
}

func NewServerHandler(db *sqlx.DB, enc *auth.EncryptionService) *ServerHandler {
	return &ServerHandler{db: db, enc: enc}
}

type ServerRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=64"`
	Host        string `json:"host" binding:"required"`
	Port        int    `json:"port"`
	Username    string `json:"username" binding:"required"`
	AuthType    string `json:"auth_type" binding:"required,oneof=password key"`
	Password    string `json:"password"`
	PrivateKey  string `json:"private_key"`
	Passphrase  string `json:"passphrase"`
	Description string `json:"description"`
	GroupName   string `json:"group"`
	Tags        string `json:"tags"`
	IsFavorite  bool   `json:"is_favorite"`
}

func (h *ServerHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	servers := []model.Server{}
	err := h.db.Select(&servers,
		"SELECT * FROM servers WHERE user_id = ? ORDER BY is_favorite DESC, name ASC",
		userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": servers})
}

func (h *ServerHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	server := model.Server{}
	err = h.db.Get(&server, "SELECT * FROM servers WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "服务器不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": server})
}

func (h *ServerHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req ServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}

	server, err := h.buildServer(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	_, err = h.db.NamedExec(`
		INSERT INTO servers (id, user_id, name, host, port, username, auth_type,
			password_enc, private_key_enc, passphrase_enc, description, group_name, tags,
			is_favorite, created_at, updated_at)
		VALUES (:id, :user_id, :name, :host, :port, :username, :auth_type,
			:password_enc, :private_key_enc, :passphrase_enc, :description, :group_name, :tags,
			:is_favorite, :created_at, :updated_at)
	`, server)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "创建成功", "data": server})
}

func (h *ServerHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}

	existing := model.Server{}
	err = h.db.Get(&existing, "SELECT * FROM servers WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "服务器不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	var req ServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}

	updated, err := h.buildServer(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	updated.ID = id

	_, err = h.db.Exec(`
		UPDATE servers SET name=?, host=?, port=?, username=?, auth_type=?,
			password_enc=?, private_key_enc=?, passphrase_enc=?,
			description=?, group_name=?, tags=?, is_favorite=?, updated_at=?
		WHERE id=? AND user_id=?
	`,
		updated.Name, updated.Host, updated.Port, updated.Username, updated.AuthType,
		updated.PasswordEnc, updated.PrivateKeyEnc, updated.PassphraseEnc,
		updated.Description, updated.GroupName, updated.Tags, updated.IsFavorite, updated.UpdatedAt,
		id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新成功", "data": updated})
}

func (h *ServerHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	_, err = h.db.Exec("DELETE FROM servers WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "删除成功"})
}

func (h *ServerHandler) ToggleFavorite(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	_, err = h.db.Exec("UPDATE servers SET is_favorite = NOT is_favorite, updated_at = ? WHERE id = ? AND user_id = ?",
		time.Now(), id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "操作成功"})
}

func (h *ServerHandler) buildServer(userID uuid.UUID, req *ServerRequest) (*model.Server, error) {
	now := time.Now()
	server := &model.Server{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        req.Name,
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		AuthType:    req.AuthType,
		Description: req.Description,
		GroupName:   req.GroupName,
		Tags:        req.Tags,
		IsFavorite:  req.IsFavorite,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if req.AuthType == model.AuthTypePassword && req.Password != "" {
		enc, err := h.enc.Encrypt(req.Password)
		if err != nil {
			return nil, err
		}
		server.PasswordEnc = enc
	} else if req.AuthType == model.AuthTypeKey && req.PrivateKey != "" {
		enc, err := h.enc.Encrypt(req.PrivateKey)
		if err != nil {
			return nil, err
		}
		server.PrivateKeyEnc = enc
		if req.Passphrase != "" {
			penc, err := h.enc.Encrypt(req.Passphrase)
			if err != nil {
				return nil, err
			}
			server.PassphraseEnc = penc
		}
	}
	return server, nil
}
