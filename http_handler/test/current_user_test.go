package test

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
	"net/http/httptest"
	"testing"
)

// TestCurrentUserErrors 验证身份缺失、空用户及无效 ID 返回未登录，错误类型返回内部错误且不泄露细节。
func TestCurrentUserErrors(t *testing.T) {
	cases := []struct {
		name   string
		value  interface{}
		set    bool
		status int
		code   int
	}{
		{"身份缺失", nil, false, 401, appconst.CodeUnauthorized},
		{"类型错误", "unexpected", true, 500, appconst.CodeInternal},
		{"空指针", (*model.User)(nil), true, 401, appconst.CodeUnauthorized},
		{"无效ID", &model.User{}, true, 401, appconst.CodeUnauthorized},
	}
	for _, input := range cases {
		t.Run(input.name, func(t *testing.T) {
			for _, path := range []string{"/orders", "/orders/list", "/admin"} {
				router := testRouter(t)
				router.Use(func(c *gin.Context) {
					if input.set {
						c.Set("authenticatedUser", input.value)
					}
				})
				router.GET("/orders", handler.CreateOrderHandler(nil))
				router.GET("/orders/list", handler.ListOrdersHandler(nil))
				router.GET("/admin", handler.RequireAdmin(), func(c *gin.Context) { t.Error("无效身份执行了管理员操作") })
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
				var body appconst.Response
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if response.Code != input.status || body.Code != input.code {
					t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
				}
				if input.status == 500 && body.Message != "系统繁忙，请稍后重试" {
					t.Fatalf("泄露内部错误: %s", body.Message)
				}
			}
		})
	}
}
