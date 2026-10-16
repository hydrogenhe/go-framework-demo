package test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	redisclient "github.com/go-redis/redis/v8"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
	"go-framework-demo/redis"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAdminIdentity 验证管理员身份必须同时匹配 ID、姓名和邮箱，拒绝伪造身份。
func TestAdminIdentity(t *testing.T) {
	for _, user := range []*model.User{nil, {ID: 2, Name: "hwt", Email: handler.AdminEmail}, {ID: 1, Name: "someone", Email: handler.AdminEmail}, {ID: 1, Name: "hwt", Email: "other@qq.com"}} {
		if handler.IsAdmin(user) {
			t.Fatal("accepted spoofed admin")
		}
	}
	adminUser := &model.User{
		ID:    1,
		Name:  "hwt",
		Email: handler.AdminEmail,
	}
	if !handler.IsAdmin(adminUser) {
		t.Fatal("admin rejected")
	}
}

// TestProtectedRoutesRequireLogin 验证受保护接口在没有登录凭证时返回 401。
func TestProtectedRoutesRequireLogin(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"PUT", "/api/admin/orders/1/user"}, {"PUT", "/api/orders/1/items/2/snapshot"}, {"PUT", "/api/auth/session"}, {"PUT", "/api/products/list"}, {"PUT", "/api/products/1/detail"}, {"PUT", "/api/logs/query"}, {"PUT", "/api/auth/logout"}, {"PUT", "/api/products/image"}, {"PUT", "/api/products/export"}, {"PUT", "/api/products"}, {"PUT", "/api/products/1"}, {"PUT", "/api/orders"}, {"PUT", "/api/orders/list?user_id=1"}, {"PUT", "/api/orders/1/ship"}} {
		r := testRouter(t)
		handler.RegisterRoutes(r)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		if w.Code != 401 {
			t.Fatalf("%s: got %d", route.path, w.Code)
		}
	}
}

func TestProductDetailAuthenticatesBeforeValidatingID(t *testing.T) {
	r := testRouter(t)
	handler.RegisterRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/products/not-a-number/detail", nil))
	if w.Code != 401 {
		t.Fatalf("detail route must require login before ID validation, got %d", w.Code)
	}
}

// testRouter 创建测试路由；必要时设置禁止连接的 Redis 客户端占位，并在测试结束后清理。
func testRouter(t *testing.T) *gin.Engine {
	// 标记为辅助函数，让测试失败信息定位到调用方。
	t.Helper()
	gin.SetMode(gin.TestMode)
	// 客户端仅用于占位；执行 Redis 命令会连接失败，不模拟真实会话。
	if redis.RedisClient == nil {
		options := &redisclient.Options{
			Dialer: func(context.Context, string, string) (net.Conn, error) {
				return nil, errors.New("test router does not connect to Redis")
			},
			MaxRetries: -1, // 关闭命令重试，让连接失败尽快返回。
		}
		client := redisclient.NewClient(options)
		redis.RedisClient = client
		// 测试结束后关闭本函数创建的客户端，恢复全局变量。
		t.Cleanup(func() { client.Close(); redis.RedisClient = nil })
	}
	return gin.New()
}

// TestAdminAuthorization 验证普通用户即使使用管理员姓名和邮箱，也不能访问管理员接口。
func TestAdminAuthorization(t *testing.T) {
	r := testRouter(t)
	r.Use(func(c *gin.Context) {
		authenticatedUser := &model.User{
			ID:    2,
			Name:  "hwt",
			Email: handler.AdminEmail,
		}
		c.Set("authenticatedUser", authenticatedUser)
	})
	r.PUT("/product", handler.RequireAdmin(), func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/product", nil))
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}

// TestOrderStringIDBinding 验证订单 ID 按字符串解析且不丢失大整数精度，拒绝数字格式的用户 ID。
func TestOrderStringIDBinding(t *testing.T) {
	var request handler.OrderRequest
	if err := json.Unmarshal([]byte(`{"user_id":"9007199254740993","items":[{"product_id":"9007199254740995","quantity":1,"unit_price_fen":10}]}`), &request); err != nil {
		t.Fatal(err)
	}
	if request.UserID != 9007199254740993 || request.Items[0].ProductID != 9007199254740995 {
		t.Fatal("IDs lost precision")
	}
	if err := json.Unmarshal([]byte(`{"user_id":9007199254740993}`), &request); err == nil {
		t.Fatal("accepted numeric ID")
	}
}

// TestCannotOrderForAnotherUser 验证登录用户不能替其他用户下单，越权请求在调用订单服务前返回 403。
func TestCannotOrderForAnotherUser(t *testing.T) {
	r := testRouter(t)
	r.Use(func(c *gin.Context) {
		authenticatedUser := &model.User{
			ID: 2,
		}
		c.Set("authenticatedUser", authenticatedUser)
	})
	// 故意不提供订单服务，确认越权校验在进入业务逻辑之前返回。
	r.PUT("/orders", handler.CreateOrderHandler(nil))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/orders", strings.NewReader(`{"user_id":"1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}
