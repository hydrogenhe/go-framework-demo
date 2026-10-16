package http_handler

import (
	"context"
	"go-framework-demo/http_handler/http_msg"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/server"
)

// 查询应用日志。
func LogQueryHandler(service *server.LogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := currentUser(c)
		if err != nil {
			writeError(c, err)
			return
		}
		if !IsAdmin(user) {
			writeError(c, forbiddenError())
			return
		}
		var req http_msg.LogQueryReq
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		if err := c.ShouldBindJSON(&req); err != nil {
			writeError(c, appconst.NewInvalidParamsError("日志查询参数格式不正确"))
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		result, err := service.Query(ctx, &server.LogQuery{Keyword: req.Keyword, Date: req.Date, Level: req.Level, Page: req.Page, PageSize: req.PageSize})
		if err != nil {
			writeError(c, err)
			return
		}
		writeSuccess(c, http.StatusOK, logResp(result))
	}
}
