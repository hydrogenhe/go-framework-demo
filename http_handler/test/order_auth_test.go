package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
)

// TestOrderUserIDAccess 验证订单接口的身份与用户 ID 校验；仅管理员查询允许跨用户访问。
func TestOrderUserIDAccess(t *testing.T) {
	user := &model.User{
		ID: 2,
	}
	admin := &model.User{
		ID:    1,
		Name:  "hwt",
		Email: handler.AdminEmail,
	}
	for _, input := range []struct {
		Name   string
		User   *model.User
		UserID string
		List   bool
		Want   int
	}{
		{Name: "create missing session", UserID: "2", Want: 401},
		{Name: "list missing session", UserID: "2", List: true, Want: 401},
		{Name: "create own ID", User: user, UserID: "2", Want: 500},
		{Name: "create other ID", User: user, UserID: "1", Want: 403},
		{Name: "admin cannot create for another user", User: admin, UserID: "2", Want: 403},
		{Name: "create zero ID", User: user, UserID: "0", Want: 400},
		{Name: "create negative ID", User: user, UserID: "-1", Want: 400},
		{Name: "create malformed ID", User: user, UserID: "bad", Want: 400},
		{Name: "list own ID", User: user, UserID: "2", List: true, Want: 500},
		{Name: "list other ID", User: user, UserID: "1", List: true, Want: 403},
		{Name: "admin can query another user", User: admin, UserID: "2", List: true, Want: 500},
		{Name: "list missing ID", User: user, List: true, Want: 400},
		{Name: "list malformed ID", User: user, UserID: "bad", List: true, Want: 400},
		{Name: "list overflow ID", User: user, UserID: "9223372036854775808", List: true, Want: 400},
		{Name: "admin cannot query zero ID", User: admin, UserID: "0", List: true, Want: 400},
		{Name: "list negative ID", User: user, UserID: "-1", List: true, Want: 400},
	} {
		t.Run(input.Name, func(t *testing.T) {
			router := testRouter(t)
			router.Use(func(c *gin.Context) {
				if input.User != nil {
					c.Set("authenticatedUser", input.User)
				}
			})
			router.PUT("/orders", handler.CreateOrderHandler(nil))
			router.PUT("/orders/list", handler.ListOrdersHandler(nil))
			path := "/orders"
			body := fmt.Sprintf(`{"order_no":"auth_test","user_id":%q,"items":[{"product_id":"1","quantity":1,"unit_price_fen":100}]}`, input.UserID)
			if input.List {
				path = "/orders/list?user_id=" + input.UserID
				body = ""
			}
			request := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != input.Want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, input.Want, response.Body.String())
			}
		})
	}
}
