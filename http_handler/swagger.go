package http_handler

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	"go-framework-demo/docs"
	"net/http"
)

// swaggerHandler 直接展示独立 OpenAPI 文档，不扫描业务代码注释。
func swaggerHandler() gin.HandlerFunc {
	assets := http.StripPrefix("/swagger/", swaggerFiles.NewHandler())
	return func(c *gin.Context) {
		switch c.Param("any") {
		case "/", "":
			c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html")
		case "/index.html":
			c.Data(http.StatusOK, "text/html; charset=utf-8", docs.HTML)
		case "/openapi.yaml":
			c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.YAML)
		case "/openapi.json", "/doc.json":
			c.Data(http.StatusOK, "application/json; charset=utf-8", docs.JSON)
		default:
			assets.ServeHTTP(c.Writer, c.Request)
		}
	}
}
