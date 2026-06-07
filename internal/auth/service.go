package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"webssh/internal/config"
	"webssh/internal/model"
)

var (
	ErrUserNotFound       = errors.New("用户不存在")
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserExists         = errors.New("用户已存在")
	ErrTokenExpired       = errors.New("登录已过期")
	ErrTokenInvalid       = errors.New("无效的令牌")
	ErrSystemNotInit      = errors.New("系统已初始化，不允许注册")
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	db     *sqlx.DB
	config *config.Config
}

func NewAuthService(db *sqlx.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, config: cfg}
}

func (s *AuthService) HasUsers() (bool, error) {
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM users")
	return count > 0, err
}

func (s *AuthService) Register(username, email, password string) (*model.User, error) {
	exists, err := s.HasUsers()
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSystemNotInit
	}

	var count int
	if err := s.db.Get(&count, "SELECT COUNT(*) FROM users WHERE username = ? OR email = ?", username, email); err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         model.RoleAdmin,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = s.db.NamedExec(`
		INSERT INTO users (id, username, email, password_hash, role, is_active, created_at, updated_at)
		VALUES (:id, :username, :email, :password_hash, :role, :is_active, :created_at, :updated_at)
	`, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(username, password string) (*model.User, string, error) {
	user := &model.User{}
	err := s.db.Get(user, "SELECT * FROM users WHERE username = ? AND is_active = 1", username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	_, _ = s.db.Exec("UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?", now, now, user.ID)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *AuthService) GenerateToken(user *model.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.config.JWT.ExpireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWT.Secret))
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.JWT.Secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func (s *AuthService) GetUserByID(id uuid.UUID) (*model.User, error) {
	user := &model.User{}
	err := s.db.Get(user, "SELECT * FROM users WHERE id = ? AND is_active = 1", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *AuthService) ChangePassword(userID uuid.UUID, oldPassword, newPassword string) error {
	user := &model.User{}
	err := s.db.Get(user, "SELECT * FROM users WHERE id = ?", userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?", string(hash), time.Now(), userID)
	return err
}
