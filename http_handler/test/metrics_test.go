package test

import (
	"fmt"
	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPMetricsStatus 验证真实 HTTP 指标入口按状态码累计请求，4xx 不进入 5xx 指标，未知路径统一聚合。
func TestHTTPMetricsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := handler.InitHTTP(":0")
	router := server.Handler.(*gin.Engine)
	router.GET("/test-metrics-status", func(c *gin.Context) {
		switch c.Query("result") {
		case "bad":
			c.Status(400)
		case "error":
			c.Status(500)
		default:
			c.Status(200)
		}
	})
	for _, path := range []string{"/test-metrics-status", "/test-metrics-status?result=bad", "/test-metrics-status?result=error", "/unknown-metrics-a", "/unknown-metrics-b"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	body := response.Body.String()
	for _, status := range []int{200, 400, 500} {
		expected := fmt.Sprintf(`http_requests_total{route="/test-metrics-status",status="%d"} 1`, status)
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %s: %s", expected, body)
		}
	}
	if !strings.Contains(body, `http_request_errors_total{route="/test-metrics-status"} 1`) {
		t.Fatal("5xx counter incorrect")
	}
	if !strings.Contains(body, `http_requests_total{route="unmatched",status="404"}`) || strings.Contains(body, `route="/unknown-metrics-`) {
		t.Fatal("unknown paths not aggregated")
	}
	if !strings.Contains(body, `le="0.01"`) || strings.Contains(body, `le="10ms"`) {
		t.Fatal("histogram bounds must use numeric seconds")
	}
}
