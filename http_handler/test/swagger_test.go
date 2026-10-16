package test

import (
	"encoding/json"
	handler "go-framework-demo/http_handler"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSwaggerPageAndSpec 验证真实 HTTP 入口提供页面和文档，包含导出、权限及字符串 ID。
func TestSwaggerPageAndSpec(t *testing.T) {
	server := handler.InitHTTP(":0")
	for _, path := range []string{"/swagger/index.html", "/swagger/swagger-ui-bundle.js", "/swagger/openapi.json"} {
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 {
			t.Fatalf("%s returned %d", path, response.Code)
		}
		if path == "/swagger/openapi.json" {
			var spec struct {
				OpenAPI    string                                `json:"openapi"`
				Paths      map[string]map[string]json.RawMessage `json:"paths"`
				Components struct {
					Schemas map[string]struct {
						Properties map[string]struct {
							Type string `json:"type"`
						} `json:"properties"`
					} `json:"schemas"`
				} `json:"components"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &spec); err != nil {
				t.Fatal(err)
			}
			if spec.OpenAPI != "3.0.3" {
				t.Fatal("expected OpenAPI 3 document")
			}
			for _, p := range []string{"/api/auth/login", "/api/products/export", "/api/products/{id}", "/api/orders", "/api/logs/query"} {
				if spec.Paths[p]["put"] == nil {
					t.Fatal("missing route: " + p)
				}
			}
			if spec.Components.Schemas["Product"].Properties["id"].Type != "string" {
				t.Fatal("product ID must be documented as string")
			}
			if !strings.Contains(string(spec.Paths["/api/products/export"]["put"]), "BearerAuth") {
				t.Fatal("export authorization missing")
			}
		}
	}
}
