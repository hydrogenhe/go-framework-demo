package order

import (
	"context"
	appconst "go-framework-demo/const"
	"go-framework-demo/server/order_server"
	"strings"
	"testing"
)

// TestOrderNoLength 验证超长订单号在访问数据库前被拒绝，25字符边界能够通过长度校验。
func TestOrderNoLength(t *testing.T) {
	var service *order_server.OrderService
	for _, length := range []int{26, 30, 32, 38} {
		no := strings.Repeat("A", length)
		_, err := service.CreateOrder(context.Background(), no, 1, 100)
		if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
			t.Fatalf("length %d: %v", length, err)
		}
		inputs := []order_server.OrderItemInput{{ProductID: 1, Quantity: 1, UnitPriceFen: 100}}
		_, err = service.CreateOrderWithItems(context.Background(), no, 1, inputs)
		if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
			t.Fatalf("items length %d: %v", length, err)
		}
	}
	_, err := service.CreateOrder(context.Background(), strings.Repeat("A", 25), 1, 100)
	if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInternal {
		t.Fatalf("25 characters rejected before service initialization: %v", err)
	}
}
