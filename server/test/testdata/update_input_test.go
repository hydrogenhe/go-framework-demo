package testdata

import (
	"context"
	"encoding/json"
	appconst "go-framework-demo/const"
	"go-framework-demo/server"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/product_server"

	"testing"
)

// TestPartialUpdateInputs 验证缺省/null 与显式零值的更新语义，以及订单状态范围。
func TestPartialUpdateInputs(t *testing.T) {
	var input product_server.ProductUpdateInput
	if err := json.Unmarshal([]byte(`{"stock":0,"image":"","name":null}`), &input); err != nil {
		t.Fatal(err)
	}
	if err := product_server.ValidateProductUpdate(&input); err != nil {
		t.Fatal(err)
	}
	if input.Stock == nil || *input.Stock != 0 || input.Image == nil || *input.Image != "" || input.Name != nil {
		t.Fatal("零值或null的解析语义错误")
	}
	if err := product_server.ValidateProductUpdate(&product_server.ProductUpdateInput{}); err == nil {
		t.Fatal("空更新未被拒绝")
	}
	amount, status := int64(100), int32(3)
	orderInput := order_server.OrderUpdateInput{Amount: &amount, Status: &status}
	if err := order_server.ValidateOrderUpdate(&orderInput); err != nil {
		t.Fatalf("有效订单更新被拒绝: %v", err)
	}
	status = 0
	if err := order_server.ValidateOrderUpdate(&orderInput); err == nil {
		t.Fatal("非法订单状态未被拒绝")
	}
	if err := order_server.ValidateOrderUpdate(&order_server.OrderUpdateInput{}); err == nil {
		t.Fatal("空订单更新未被拒绝")
	}
}

// TestNilInputPointers 验证空入参在业务入口被明确拒绝，不发生空指针异常。
func TestNilInputPointers(t *testing.T) {
	ctx := context.Background()
	products := product_server.NewProductService(ctx)
	_, err := products.Update(ctx, 1, nil)
	if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
		t.Fatalf("空商品更新参数未作为业务错误返回: %v", err)
	}
	if err := order_server.ValidateOrderUpdate(nil); err == nil {
		t.Fatal("空订单更新参数未被拒绝")
	}
	_, err = server.NewLogService(t.TempDir()).Query(ctx, nil)
	if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
		t.Fatalf("空日志参数未作为业务错误返回: %v", err)
	}
}
