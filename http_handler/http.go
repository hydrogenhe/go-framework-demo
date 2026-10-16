package http_handler

import (
	"go-framework-demo/util"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// InitHTTP 初始化 HTTP 服务及路由。
func InitHTTP(addr string) *http.Server {
	router := gin.New()
	router.Use(requestLogIDMiddleware(), zapAccessLogMiddleware(), zapRecoveryMiddleware(), corsMiddleware(), metricsMiddleware())
	router.PUT("/api/metrics", metricsHandler) // 接收应用指标并写入 Prometheus 指标采集器。
	router.GET("/swagger/*any", swaggerHandler())
	router.GET("/metrics", prometheusHandler) // 暴露 Prometheus 拉取指标的文本接口。
	frontendPath, err := util.ProjectPath(filepath.Join("frontend", "metrics.html"))
	if err != nil {
		panic(err)
	}
	router.StaticFile("/monitor", frontendPath) // 提供指标监控页面。
	RegisterRoutes(router)

	return &http.Server{
		Addr:    addr,
		Handler: router,
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "PUT, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Log-ID")
		c.Header("Access-Control-Expose-Headers", "X-Log-ID, Content-Disposition, X-Export-Count")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
