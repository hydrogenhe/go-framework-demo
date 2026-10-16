package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
)

func TestOrderActionAccessAndParameters(t *testing.T) {
	user := &model.User{ID: 2}
	admin := &model.User{ID: 1, Name: "hwt", Email: handler.AdminEmail}
	for _, input := range []struct {
		name, path string
		user       *model.User
		want       int
	}{
		{"pay requires login", "/orders/1/pay", nil, 401},
		{"receive requires login", "/orders/1/receive", nil, 401},
		{"ship requires login", "/orders/1/items/1/ship", nil, 401},
		{"ship requires admin", "/orders/1/items/1/ship", user, 403},
		{"list requires admin", "/admin/orders/list", user, 403},
		{"invalid order ID", "/orders/bad/pay", user, 400},
		{"zero order ID", "/orders/0/receive", user, 400},
		{"invalid item ID", "/orders/1/items/bad/ship", admin, 400},
		{"admin invalid user filter", "/admin/orders/list?user_id=0", admin, 400},
	} {
		t.Run(input.name, func(t *testing.T) {
			router := testRouter(t)
			router.Use(func(c *gin.Context) {
				if input.user != nil {
					c.Set("authenticatedUser", input.user)
				}
			})
			router.PUT("/orders/:id/pay", handler.PayOrderHandler(nil))
			router.PUT("/orders/:id/receive", handler.ConfirmReceiptHandler(nil))
			router.PUT("/orders/:id/items/:item_id/ship", handler.ShipOrderItemHandler(nil))
			router.PUT("/admin/orders/list", handler.ListAdminOrdersHandler(nil))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, input.path, nil))
			if response.Code != input.want {
				t.Fatalf("status=%d want=%d %s", response.Code, input.want, response.Body.String())
			}
		})
	}
}
