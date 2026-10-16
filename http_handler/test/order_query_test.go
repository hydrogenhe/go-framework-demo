package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
)

func TestOrderQueryInvalidParameters(t *testing.T) {
	for _, query := range []string{"status=0", "status=5", "sort=bad", "price_min=-1", "price_min=20&price_max=10", "price_min=1.5", "page=0", "page_size=101", "page=501&page_size=20", "created_from=bad"} {
		t.Run(query, func(t *testing.T) {
			router := testRouter(t)
			user := &model.User{ID: 2}
			router.PUT("/orders/list", func(c *gin.Context) { c.Set("authenticatedUser", user) }, handler.ListOrdersHandler(nil))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/orders/list?user_id=2&"+query, nil))
			if response.Code != 400 {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
		})
	}
}
