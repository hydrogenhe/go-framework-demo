package http_handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"go-framework-demo/model"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/product_server"
	"net/http/httptest"
	"strings"
	"testing"
)

// 验证实际 HTTP 编码过程，不依赖模型 MarshalJSON，也不暴露数据库字段。
func TestResponseProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// id 超过JavaScript安全整数范围的测试ID，用于验证字符串序列化。
	const id int64 = 9007199254740993
	cases := []struct {
		name     string
		data     any
		expected []string
	}{
		{"product", productPageResp(&product_server.ProductPage{Items: []*model.Product{{ID: id, Name: "商品", Price: 1234, IsDel: 1}}, Total: 1, Page: 1, PageSize: 20}), []string{`"id":"9007199254740993"`, `"price":1234`, `"items":[`}},
		{"order", orderPageResp(&order_server.OrderPage{Items: []*order_server.OrderListItem{{ID: id, UserID: id, Items: []*model.OrderItem{{ID: id, OrderID: id, ProductID: id}}}}}), []string{`"user_id":"9007199254740993"`, `"product_id":"9007199254740993"`, `"order_id":"9007199254740993"`}},
		{"empty", productPageResp(&product_server.ProductPage{}), []string{`"items":[]`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			writeSuccess(c, 200, tc.data)
			var body struct {
				Code    int             `json:"code"`
				Message string          `json:"message"`
				Data    json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || body.Code != 0 || body.Message != "success" || len(body.Data) == 0 {
				t.Fatalf("invalid envelope: %s", w.Body)
			}
			for _, s := range tc.expected {
				if !strings.Contains(string(body.Data), s) {
					t.Fatalf("missing %s: %s", s, w.Body)
				}
			}
			for _, s := range []string{`"is_del"`, `"password"`} {
				if strings.Contains(w.Body.String(), s) {
					t.Fatalf("leaked internal field: %s", w.Body)
				}
			}
		})
	}
}
