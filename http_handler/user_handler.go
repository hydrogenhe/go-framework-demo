package http_handler

import (
	appconst "go-framework-demo/const"
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/model"
	"go-framework-demo/server/user_server"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 注册并登录。
func (h routeHandlers) createUser(c *gin.Context) {
	ctx := c.Request.Context()
	var req http_msg.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, appconst.NewInvalidParamsError("请求参数格式不正确"))
		return
	}

	profile := &model.User{
		Name:  req.Name,
		Age:   req.Age,
		Email: req.Email,
	}
	user, err := h.userService.CreateModel(ctx, profile, req.Password)
	if err != nil {
		writeError(c, err)
		return
	}

	authResponse(c, user, http.StatusCreated)
}

// LoginHandler 返回用户登录接口的 HTTP 处理函数，供测试或其他调用方复用。
func LoginHandler(s *user_server.UserService) gin.HandlerFunc {
	return (routeHandlers{userService: s}).login
}

// 登录。
func (h routeHandlers) login(c *gin.Context) {
	ctx := c.Request.Context()
	var req http_msg.LoginReq
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, appconst.NewInvalidParamsError("请求参数格式不正确"))
		return
	}
	u, e := h.userService.Authenticate(ctx, req.Email, req.Password)
	if e != nil {
		writeError(c, e)
		return
	}
	authResponse(c, u, http.StatusOK)
}

func writeError(c *gin.Context, err error) {
	msgErr := appconst.NewInternalError(err)
	c.Error(err)
	response := http_msg.Response[any]{
		Code:    msgErr.Code,
		Message: msgErr.Message,
	}
	c.JSON(msgErr.HTTPStatus, response)
}
