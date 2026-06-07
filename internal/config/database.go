package config

import (
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/sqlite"
	"github.com/jmoiron/sqlx"
)

func InitDatabase(cfg *Config) (*sqlx.DB, error) {
	dsn := cfg.Database.DSN
	if dsn == "" {
		dsn = "webssh.db"
	}
	if dir := filepath.Dir(dsn); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据库目录 %s 失败: %w", dir, err)
		}
	}
	db, err := sqlx.Connect(cfg.Database.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpen)
	db.SetMaxIdleConns(cfg.Database.MaxIdle)
	return db, nil
}
