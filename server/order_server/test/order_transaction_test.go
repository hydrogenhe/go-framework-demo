package order

import (
	"context"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/test/support"
	"go-framework-demo/util"
	"testing"
)

// TestOrderCreateWithItemsTransaction 通过订单服务下单，验证订单和对应明细成功写入并可查询。
func TestOrderCreateWithItemsTransaction(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	userDB, _ := db.NewUserDb(ctx)
	user, err := userDB.Create(ctx, support.FixtureUser(t, "tx_item_test_"+util.GenerateAlnum(8), 30, util.RandomTestEmail(), util.GenerateAlnum(8)))
	if err != nil {
		t.Fatalf("创建事务测试用户失败: %v", err)
	}
	product := &model.Product{Name: "tx_product_" + util.GenerateAlnum(8), Price: 1200, Stock: 10}
	if err := mysql.GetGenQuery().Product.WithContext(ctx).Create(product); err != nil {
		t.Fatalf("创建事务测试商品失败: %v", err)
	}
	orderService := order_server.NewOrderService(ctx)
	inputs := []order_server.OrderItemInput{{
		ProductID:    product.ID,
		Quantity:     2,
		UnitPriceFen: 1200,
	}}
	order, err := orderService.CreateOrderWithItems(ctx, "O_TX_TEST_"+util.GenerateAlnum(8), user.ID, inputs)
	if err != nil {
		t.Fatalf("订单事务创建失败: %v", err)
	}
	itemDB, _ := db.NewOrderItemDb(ctx)
	items, err := itemDB.GetByOrderID(ctx, order.ID)
	if err != nil {
		t.Fatalf("查询事务订单明细失败: %v", err)
	}
	if len(items) != 1 || items[0].ProductID != product.ID || items[0].Quantity != 2 {
		t.Fatalf("事务订单明细数据不正确: got=%+v", items)
	}
}
