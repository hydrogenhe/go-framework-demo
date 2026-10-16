package order

import (
	"context"
	"go-framework-demo/es"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/product_server"
	"go-framework-demo/server/user_server"

	"go-framework-demo/util"
	"testing"
	"time"
)

// createOrderFixture 通过真实服务创建测试用户和商品，返回订单服务及下单需要的 ID。
func createOrderFixture(t *testing.T, ctx context.Context) (*order_server.OrderService, int64, int64) {
	t.Helper()
	user, err := user_server.NewUserService(ctx).CreateUser(ctx, "order_test_"+util.GenerateAlnum(8), 30, util.RandomTestEmail(), util.GenerateAlnum(8))
	if err != nil {
		t.Fatalf("创建订单测试用户失败: %v", err)
	}
	product, err := product_server.NewProductService(ctx).Create(ctx, "order_product_"+util.GenerateAlnum(8), 19900, 10, "")
	if err != nil {
		t.Fatalf("创建订单测试商品失败: %v", err)
	}
	return order_server.NewOrderService(ctx), user.ID, product.ID
}

// TestOrderCreate 通过订单服务创建订单，验证写入 MySQL 后返回有效 ID。
func TestOrderCreate(t *testing.T) {
	ctx := context.Background()
	service, userID, productID := createOrderFixture(t, ctx)
	orderItems := []order_server.OrderItemInput{
		{
			ProductID:    productID,
			Quantity:     1,
			UnitPriceFen: 19900,
		},
	}
	order, err := service.CreateOrderWithItems(ctx, "order_test_"+util.GenerateAlnum(8), userID, orderItems)
	if err != nil {
		t.Fatalf("创建订单并写入 MySQL 失败: %v", err)
	}
	if order == nil || order.ID <= 0 {
		t.Fatalf("创建订单未返回有效 ID: %+v", order)
	}
	t.Logf("订单已写入 MySQL，ID=%d", order.ID)
}

// TestOrderCreateES 通过订单服务下单，等待 CDC 同步后验证 ES 能查询到对应订单。
func TestOrderCreateES(t *testing.T) {
	ctx := context.Background()
	service, userID, productID := createOrderFixture(t, ctx)
	orderItems := []order_server.OrderItemInput{
		{
			ProductID:    productID,
			Quantity:     1,
			UnitPriceFen: 19900,
		},
	}
	order, err := service.CreateOrderWithItems(ctx, "order_es_test_"+util.GenerateAlnum(8), userID, orderItems)
	if err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
	t.Logf("订单创建成功，ID=%d，等待 3 秒后查询 ES", order.ID)
	time.Sleep(3 * time.Second)
	indexed, err := es.GetOrderByID(order.ID)
	if err != nil {
		t.Fatalf("ES 查询订单失败: %v", err)
	}
	if indexed.ID != order.ID {
		t.Fatalf("ES 订单 ID 不一致: got=%d want=%d", indexed.ID, order.ID)
	}
	t.Logf("订单已同步到 ES，ID=%d", order.ID)
}
