package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"webssh/internal/auth"
	"webssh/internal/config"
	"webssh/internal/handler"
	"webssh/internal/middleware"
	"webssh/internal/model"
	"webssh/internal/sftpserver"
	"webssh/internal/sshclient"
	"webssh/internal/wsbridge"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径（可选）")
	flag.Parse()

	if err := config.Init(*configPath); err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}
	cfg := config.AppConfig

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	middleware.InitLogger(cfg.DataDir)

	db, err := config.InitDatabase(cfg)
	if err != nil {
		log.Printf("数据库初始化失败: %v", err)
		log.Printf("提示: 请检查数据目录 %s 是否存在且可写 (容器部署时挂载的宿主机目录需 chown 给容器用户)", cfg.DataDir)
		log.Printf("      修复方法: 在宿主机执行 chown -R 1000:1000 %s 或 docker compose run --user root webssh chown -R 1000:1000 /app/data", cfg.DataDir)
		log.Fatalf("服务因数据库不可用而退出")
	}
	defer db.Close()

	if err := model.Migrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	authSvc := auth.NewAuthService(db, cfg)
	encSvc := auth.NewEncryptionService(cfg)
	sshMgr := sshclient.NewManager()
	pfMgr := sshclient.NewPortForwardManager()
	sftpSvc := sftpserver.NewSFTPService()

	wsbridge.SetSessionManager(sshMgr)

	authH := handler.NewAuthHandler(authSvc)
	serverH := handler.NewServerHandler(db, encSvc)
	sessionH := handler.NewSessionHandler(db, cfg, encSvc, sshMgr, sftpSvc)
	pfH := handler.NewPortForwardHandler(db, cfg, encSvc, pfMgr, sshMgr)
	settingH := handler.NewSettingHandler(db)

	r := setupRouter(db, authSvc, authH, serverH, sessionH, pfH, settingH, cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("WebSSH 服务启动: %s (data=%s)", addr, cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务异常退出: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务关闭异常: %v", err)
	}
	sshMgr.Close()
	pfMgr.Close()
	log.Println("服务已退出")
}

func setupRouter(db *sqlx.DB, authSvc *auth.AuthService,
	authH *handler.AuthHandler, serverH *handler.ServerHandler, sessionH *handler.SessionHandler,
	pfH *handler.PortForwardHandler, settingH *handler.SettingHandler, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.Logger())

	corsCfg := cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(corsCfg))

	staticDir := "./web/dist"
	if env := os.Getenv("WEBSSH_STATIC_DIR"); env != "" {
		staticDir = env
	}
	if abs, err := filepath.Abs(staticDir); err == nil {
		staticDir = abs
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now()})
	})

	api := r.Group("/api")
	{
		api.GET("/auth/status", authH.Status)
		api.POST("/auth/register", authH.Register)
		api.POST("/auth/login", authH.Login)

		authed := api.Group("")
		authed.Use(middleware.AuthRequired(authSvc))
		{
			authed.GET("/auth/me", authH.Me)
			authed.POST("/auth/logout", authH.Logout)
			authed.POST("/auth/change-password", authH.ChangePassword)

			authed.GET("/servers", serverH.List)
			authed.GET("/servers/:id", serverH.Get)
			authed.POST("/servers", serverH.Create)
			authed.PUT("/servers/:id", serverH.Update)
			authed.DELETE("/servers/:id", serverH.Delete)
			authed.POST("/servers/:id/favorite", serverH.ToggleFavorite)

			authed.POST("/servers/:id/connect", sessionH.Connect)
			authed.POST("/sessions/:id/disconnect", sessionH.Disconnect)
			authed.GET("/sessions", sessionH.ListSessions)

			authed.GET("/servers/:id/sftp/list", sessionH.SFTPList)
			authed.POST("/servers/:id/sftp/upload", sessionH.SFTPUpload)
			authed.GET("/servers/:id/sftp/download", sessionH.SFTPDownload)
			authed.DELETE("/servers/:id/sftp/delete", sessionH.SFTPDelete)
			authed.POST("/servers/:id/sftp/mkdir", sessionH.SFTPMkdir)
			authed.POST("/servers/:id/sftp/rename", sessionH.SFTPRename)
			authed.GET("/servers/:id/sftp/read", sessionH.SFTPReadFile)
			authed.POST("/servers/:id/sftp/write", sessionH.SFTPWriteFile)

			authed.GET("/port-forwards", pfH.List)
			authed.POST("/port-forwards", pfH.Create)
			authed.DELETE("/port-forwards/:id", pfH.Delete)
			authed.POST("/port-forwards/:id/start", pfH.Start)
			authed.POST("/port-forwards/:id/stop", pfH.Stop)

			authed.GET("/settings", settingH.List)
			authed.GET("/settings/:key", settingH.Get)
			authed.POST("/settings", settingH.Set)
		}

		ws := r.Group("/ws")
		ws.Use(middleware.AuthRequired(authSvc))
		{
			ws.GET("/ssh/:id", wsbridge.HandleSSHWS)
		}
	}

	if _, err := os.Stat(staticDir); err == nil {
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.StaticFile("/favicon.svg", filepath.Join(staticDir, "favicon.svg"))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if len(path) >= 4 && path[:4] == "/api" {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "接口不存在"})
				return
			}
			c.File(filepath.Join(staticDir, "index.html"))
		})
		log.Printf("静态文件目录: %s", staticDir)
	} else {
		log.Printf("警告: 静态文件目录不存在: %s", staticDir)
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"msg":  "WebSSH API 服务运行中",
				"data": gin.H{
					"endpoints": []string{
						"GET  /healthz",
						"GET  /api/auth/status",
						"POST /api/auth/register",
						"POST /api/auth/login",
					},
				},
			})
		})
	}

	return r
}
