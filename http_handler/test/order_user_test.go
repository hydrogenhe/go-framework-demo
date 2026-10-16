package test

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
)

func TestOrderUserRequiresAdmin(t *testing.T) {
	for _, user := range []*model.User{nil, {ID: 2, Name: "普通用户", Email: "user@example.com"}} {
		router := testRouter(t)
		router.Use(func(c *gin.Context) {
			if user != nil {
				c.Set("authenticatedUser", user)
			}
		})
		router.PUT("/admin/orders/:id/user", handler.AdminOrderUserHandler(nil))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("PUT", "/admin/orders/1/user", nil))
		want := 401
		if user != nil {
			want = 403
		}
		if response.Code != want {
			t.Fatalf("status=%d want %d", response.Code, want)
		}
	}
}
