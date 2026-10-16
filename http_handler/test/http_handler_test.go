package test

import (
	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCreateOrderHandlerInvalidRequest 验证订单请求的 JSON 格式错误时返回 400，不调用订单服务。
func TestCreateOrderHandlerInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	user := &model.User{
		ID: 2,
	}
	r.PUT("/api/orders", func(c *gin.Context) { c.Set("authenticatedUser", user) }, handler.CreateOrderHandler(nil))
	req := httptest.NewRequest("PUT", "/api/orders", strings.NewReader(`{"user_id":`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// TestProductRequestBinding 验证商品请求的 JSON 格式错误时返回 400，不调用商品服务。
func TestProductRequestBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/products", handler.CreateProductHandler(nil))
	req := httptest.NewRequest("PUT", "/api/products", strings.NewReader(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// TestBusinessRoutesUsePUT 验证全部业务路由使用 PUT，且不再接受旧的 GET、POST、DELETE 方法。
func TestBusinessRoutesUsePUT(t *testing.T) {
	r := testRouter(t)
	handler.RegisterRoutes(r)
	expected := map[string]bool{
		"/api/auth/register": false, "/api/auth/login": false, "/api/auth/logout": false,
		"/api/products/image":  false,
		"/api/products/export": false,
		"/api/logs/query":      false,
		"/api/products/list":   false, "/api/products/:id/detail": false,
		"/api/products": false, "/api/products/:id": false,
		"/api/admin/orders/list": false, "/api/orders/:id/pay": false, "/api/orders/:id/receive": false, "/api/orders/:id/items/:item_id/ship": false,
		"/api/orders": false, "/api/orders/list": false, "/api/orders/:id/ship": false,
	}
	for _, route := range r.Routes() {
		if route.Method != "PUT" {
			t.Errorf("unexpected method %s for %s", route.Method, route.Path)
		}
		if _, ok := expected[route.Path]; !ok {
			t.Errorf("unexpected path %s", route.Path)
		}
		expected[route.Path] = true
	}
	for path, found := range expected {
		if !found {
			t.Errorf("missing route %s", path)
		}
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/products", nil))
		if w.Code != 404 {
			t.Errorf("old method %s still accepted: %d", method, w.Code)
		}
	}
}
