package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ServerConfig struct {
	Host string
	Port int
	Mode string
}

type DatabaseConfig struct {
	Driver  string
	DSN     string
	MaxOpen int
	MaxIdle int
}

type JWTConfig struct {
	Secret      string
	ExpireHours int
}

type SSHConfig struct {
	DefaultPort        int
	ConnectionTimeout  int
	KeepAliveInterval  int
	MaxSessionsPerUser int
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	SSH      SSHConfig
	DataDir  string
}

var AppConfig *Config

func Init(configPath string) error {
	AppConfig = &Config{
		Server: ServerConfig{
			Host: getString("WEBSSH_SERVER_HOST", "0.0.0.0"),
			Port: getInt("WEBSSH_SERVER_PORT", 8090),
			Mode: getString("WEBSSH_SERVER_MODE", "release"),
		},
		Database: DatabaseConfig{
			Driver:  getString("WEBSSH_DATABASE_DRIVER", "sqlite"),
			DSN:     "",
			MaxOpen: getInt("WEBSSH_DATABASE_MAX_OPEN", 20),
			MaxIdle: getInt("WEBSSH_DATABASE_MAX_IDLE", 5),
		},
		JWT: JWTConfig{
			Secret:      getString("WEBSSH_JWT_SECRET", "webssh-default-secret-please-change-in-production"),
			ExpireHours: getInt("WEBSSH_JWT_EXPIRE_HOURS", 168),
		},
		SSH: SSHConfig{
			DefaultPort:        getInt("WEBSSH_SSH_DEFAULT_PORT", 22),
			ConnectionTimeout:  getInt("WEBSSH_SSH_CONNECTION_TIMEOUT", 15),
			KeepAliveInterval:  getInt("WEBSSH_SSH_KEEP_ALIVE_INTERVAL", 30),
			MaxSessionsPerUser: getInt("WEBSSH_SSH_MAX_SESSIONS_PER_USER", 20),
		},
		DataDir: getString("WEBSSH_DATA_DIR", "./data"),
	}

	if configPath != "" {
		loadFromYAML(configPath)
	}

	if err := os.MkdirAll(AppConfig.DataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 创建数据目录 %s 失败: %v\n", AppConfig.DataDir, err)
		fmt.Fprintf(os.Stderr, "提示: 容器部署时该目录由 docker-compose 挂载，需 chown 给容器内的 webssh 用户 (UID 1000)\n")
	}

	if AppConfig.Database.DSN == "" {
		AppConfig.Database.DSN = filepath.Join(AppConfig.DataDir, "webssh.db")
	}

	return nil
}

func getString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func loadFromYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	section := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)

		fullKey := section + "." + key
		applyConfig(fullKey, value)
	}
	return nil
}

func applyConfig(key, value string) {
	switch key {
	case "server.host":
		AppConfig.Server.Host = value
	case "server.port":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.Server.Port = v
		}
	case "server.mode":
		AppConfig.Server.Mode = value
	case "database.driver":
		AppConfig.Database.Driver = value
	case "database.dsn":
		AppConfig.Database.DSN = value
	case "database.max_open":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.Database.MaxOpen = v
		}
	case "database.max_idle":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.Database.MaxIdle = v
		}
	case "jwt.secret":
		AppConfig.JWT.Secret = value
	case "jwt.expire_hours":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.JWT.ExpireHours = v
		}
	case "ssh.default_port":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.SSH.DefaultPort = v
		}
	case "ssh.connection_timeout":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.SSH.ConnectionTimeout = v
		}
	case "ssh.keep_alive_interval":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.SSH.KeepAliveInterval = v
		}
	case "ssh.max_sessions_per_user":
		if v, err := strconv.Atoi(value); err == nil {
			AppConfig.SSH.MaxSessionsPerUser = v
		}
	case "data_dir":
		AppConfig.DataDir = value
	}
}
