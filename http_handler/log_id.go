package http_handler

import (
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go-framework-demo/logger"
)

// logIDHeader HTTP请求和响应中传递日志追踪ID的头名称。
const logIDHeader = "X-Log-ID"

var validLogID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

func requestLogIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		logID := c.GetHeader(logIDHeader)
		if !validLogID.MatchString(logID) {
			logID = logger.NewLogID()
		}
		ctx := c.Request.Context()
		ctx = logger.WithLogID(ctx, logID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(logIDHeader, logID)
		c.Next()
	}
}

func zapAccessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		ctx := c.Request.Context()
		logger.Info(ctx, "HTTP请求完成",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(started),
			"client_ip", c.ClientIP(),
		)
	}
}

func zapRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				ctx := c.Request.Context()
				logger.Error(ctx, "HTTP请求发生panic",
					"panic", recovered,
					"stack", debug.Stack(),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
