package test

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/mysql"
	"go-framework-demo/redis"
	"go-framework-demo/server/user_server"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestIntegrationAdminLoginAndSession 使用真实 MySQL 和 Redis，验证管理员登录、会话鉴权及注销后凭证失效。
func TestIntegrationAdminLoginAndSession(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to validate local MySQL and Redis")
	}
	mysql.InitMySQL()
	redis.Init()
	r := testRouter(t)
	r.PUT("/login", handler.LoginHandler(user_server.NewUserService(context.Background())))
	r.PUT("/admin", handler.RequireAuth(), handler.RequireAdmin(), func(c *gin.Context) { c.Status(204) })
	r.PUT("/logout", handler.RequireAuth(), handler.LogoutHandler)
	loginInput := map[string]string{
		"email":    handler.AdminEmail,
		"password": os.Getenv("DEMO_ADMIN_PASSWORD"),
	}
	body, _ := json.Marshal(loginInput)
	request := httptest.NewRequest("PUT", "/login", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatalf("login status %d", w.Code)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["code"] != float64(0) {
		t.Fatal("unexpected response code")
	}
	response = response["data"].(map[string]interface{})
	if response["id"] != "1" || response["role"] != "admin" || response["password"] != nil {
		t.Fatal("invalid login response")
	}
	token, ok := response["token"].(string)
	if !ok || len(token) != 64 {
		t.Fatal("missing session token")
	}
	t.Cleanup(func() { redis.RedisClient.Del(context.Background(), handler.SessionKey(token)) })
	for _, route := range []struct {
		method, path string
		status       int
	}{{"PUT", "/admin", 204}, {"PUT", "/logout", 200}, {"PUT", "/admin", 401}} {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		result := httptest.NewRecorder()
		r.ServeHTTP(result, req)
		if result.Code != route.status {
			t.Fatalf("%s got %d expected %d", route.path, result.Code, route.status)
		}
	}
}
