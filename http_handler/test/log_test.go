package test

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
	"go-framework-demo/server"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogQueryHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app-2026-10-14.log"), []byte(`{"level":"error","msg":"错误详情","log_id":"trace-123"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		Name, Body string
		User       *model.User
		Status     int
	}{
		{"admin", `{"keyword":"trace","level":"err"}`, &model.User{ID: 1, Name: "hwt", Email: handler.AdminEmail}, 200},
		{"ordinary", `{}`, &model.User{ID: 2}, 403},
		{"anonymous", `{}`, nil, 401},
		{"bad date", `{"date":"../"}`, &model.User{ID: 1, Name: "hwt", Email: handler.AdminEmail}, 400},
	} {
		t.Run(input.Name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if input.User != nil {
					c.Set("authenticatedUser", input.User)
				}
			})
			r.PUT("/logs", handler.LogQueryHandler(server.NewLogService(dir)))
			req := httptest.NewRequest("PUT", "/logs", strings.NewReader(input.Body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != input.Status {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
			if input.Status == 200 {
				var body struct {
					Data server.LogResult `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Data.Total != 1 {
					t.Fatalf("unexpected results: %s", w.Body)
				}
			}
		})
	}
}
