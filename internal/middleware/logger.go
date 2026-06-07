package middleware

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

var logger *log.Logger

func InitLogger(dataDir string) {
	if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0755); err != nil {
		log.Printf("创建日志目录失败 (将仅输出到 stderr): %v", err)
		return
	}
	logFile, err := os.OpenFile(
		filepath.Join(dataDir, "logs", "webssh.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.Printf("打开日志文件失败 (将仅输出到 stderr): %v", err)
		return
	}
	logger = log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		clientIP := c.ClientIP()

		msg := fmt.Sprintf("%s %s %d %v %s", method, path, statusCode, latency, clientIP)
		if logger != nil {
			logger.Println(msg)
		}
		if statusCode >= 500 {
			log.Println(msg)
		}
	}
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				c.AbortWithStatusJSON(500, gin.H{
					"code": 500,
					"msg":  "服务器内部错误",
				})
			}
		}()
		c.Next()
	}
}
