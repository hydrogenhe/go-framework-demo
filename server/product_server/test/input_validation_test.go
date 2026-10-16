package product

import (
	"context"
	"encoding/json"
	appconst "go-framework-demo/const"
	"go-framework-demo/server/product_server"
	"testing"
)

// TestProductUpdateValidation 通过商品服务验证字段白名单及名称、价格、库存的类型和范围限制。
func TestProductUpdateValidation(t *testing.T) {
	ctx := context.Background()
	service := product_server.NewProductService(ctx)
	cases := []string{
		`{"name":" "}`, `{"price":1.5}`, `{"price":0}`, `{"price":100000001}`,
		`{"stock":-1}`, `{"stock":1000001}`, `{"stock":null}`, `{"price":"100"}`, `{}`,
	}
	for _, body := range cases {
		var input product_server.ProductUpdateInput
		if err := json.Unmarshal([]byte(body), &input); err != nil {
			continue
		}
		if _, err := service.Update(ctx, 1, &input); err == nil {
			t.Fatalf("invalid update accepted: %s", body)
		} else if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
			t.Fatalf("wrong validation result: %v", err)
		}
	}
}
