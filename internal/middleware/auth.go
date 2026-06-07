package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"webssh/internal/auth"
)

const (
	CtxUserID = "user_id"
	CtxRole   = "user_role"
)

func AuthRequired(authSvc *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
			return
		}

		claims, err := authSvc.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": err.Error()})
			return
		}

		user, err := authSvc.GetUserByID(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户不存在或已禁用"})
			return
		}

		c.Set(CtxUserID, user.ID)
		c.Set(CtxRole, user.Role)
		c.Next()
	}
}

func GetUserID(c *gin.Context) uuid.UUID {
	v, ok := c.Get(CtxUserID)
	if !ok {
		return uuid.Nil
	}
	id, _ := v.(uuid.UUID)
	return id
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	if t := c.Query("token"); t != "" {
		return t
	}
	return ""
}
