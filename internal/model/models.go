package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type User struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	Username     string     `db:"username" json:"username"`
	Email        string     `db:"email" json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"`
	Role         string     `db:"role" json:"role"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	LastLoginAt  *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
}

type Server struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	UserID        uuid.UUID  `db:"user_id" json:"user_id"`
	Name          string     `db:"name" json:"name"`
	Host          string     `db:"host" json:"host"`
	Port          int        `db:"port" json:"port"`
	Username      string     `db:"username" json:"username"`
	AuthType      string     `db:"auth_type" json:"auth_type"`
	PasswordEnc   string     `db:"password_enc" json:"-"`
	PrivateKeyEnc string     `db:"private_key_enc" json:"-"`
	PassphraseEnc string     `db:"passphrase_enc" json:"-"`
	Description   string     `db:"description" json:"description"`
	GroupName     string     `db:"group_name" json:"group"`
	Tags          string     `db:"tags" json:"tags"`
	IsFavorite    bool       `db:"is_favorite" json:"is_favorite"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	LastConnectAt *time.Time `db:"last_connect_at" json:"last_connect_at,omitempty"`
}

type Session struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	UserID     uuid.UUID  `db:"user_id" json:"user_id"`
	ServerID   uuid.UUID  `db:"server_id" json:"server_id"`
	Name       string     `db:"name" json:"name"`
	Status     string     `db:"status" json:"status"`
	Cols       int        `db:"cols" json:"cols"`
	Rows       int        `db:"rows" json:"rows"`
	WorkingDir string     `db:"working_dir" json:"working_dir"`
	EnvVars    string     `db:"env_vars" json:"env_vars"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
	ClosedAt   *time.Time `db:"closed_at" json:"closed_at,omitempty"`
}

type PortForward struct {
	ID         uuid.UUID `db:"id" json:"id"`
	UserID     uuid.UUID `db:"user_id" json:"user_id"`
	ServerID   uuid.UUID `db:"server_id" json:"server_id"`
	Name       string    `db:"name" json:"name"`
	Type       string    `db:"type" json:"type"`
	LocalHost  string    `db:"local_host" json:"local_host"`
	LocalPort  int       `db:"local_port" json:"local_port"`
	RemoteHost string    `db:"remote_host" json:"remote_host"`
	RemotePort int       `db:"remote_port" json:"remote_port"`
	Status     string    `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type Setting struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Key       string    `db:"key" json:"key"`
	Value     string    `db:"value" json:"value"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

const (
	AuthTypePassword = "password"
	AuthTypeKey      = "key"

	SessionStatusConnecting   = "connecting"
	SessionStatusConnected    = "connected"
	SessionStatusDisconnected = "disconnected"
	SessionStatusError        = "error"

	PortForwardTypeLocal  = "local"
	PortForwardTypeRemote = "remote"

	PortForwardStatusStopped = "stopped"
	PortForwardStatusRunning = "running"
	PortForwardStatusError   = "error"

	RoleAdmin = "admin"
	RoleUser  = "user"
)

var Schema = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_login_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
	`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,

	`CREATE TABLE IF NOT EXISTS servers (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 22,
		username TEXT NOT NULL,
		auth_type TEXT NOT NULL DEFAULT 'password',
		password_enc TEXT,
		private_key_enc TEXT,
		passphrase_enc TEXT,
		description TEXT,
		group_name TEXT,
		tags TEXT,
		is_favorite BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_connect_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_servers_user_id ON servers(user_id)`,

	`CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		server_id TEXT NOT NULL,
		name TEXT,
		status TEXT NOT NULL DEFAULT 'connecting',
		cols INTEGER NOT NULL DEFAULT 80,
		rows INTEGER NOT NULL DEFAULT 24,
		working_dir TEXT,
		env_vars TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		closed_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_server_id ON sessions(server_id)`,

	`CREATE TABLE IF NOT EXISTS port_forwards (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		server_id TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'local',
		local_host TEXT NOT NULL DEFAULT '127.0.0.1',
		local_port INTEGER NOT NULL,
		remote_host TEXT NOT NULL,
		remote_port INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'stopped',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_port_forwards_user_id ON port_forwards(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_port_forwards_server_id ON port_forwards(server_id)`,

	`CREATE TABLE IF NOT EXISTS settings (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, key)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings(user_id)`,
}

func Migrate(db *sqlx.DB) error {
	for _, query := range Schema {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}
