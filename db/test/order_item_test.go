package test

import (
	"context"
	"testing"

	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

// TestOrderItemCreateAndGet 使用真实 MySQL，验证订单明细持久化和商品、数量、单价的查询结果。
func TestOrderItemCreateAndGet(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	userDB, err := db.NewUserDb(ctx)
	if err != nil {
		t.Fatalf("初始化UserDb失败: %v", err)
	}
	user, err := userDB.Create(ctx, fixtureUser(t, "item_test_"+util.GenerateAlnum(8), 30, util.RandomTestEmail(), util.GenerateAlnum(8)))
	if err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	product := &model.Product{Name: "item_product_" + util.GenerateAlnum(8), Price: 8800, Stock: 20}
	if err := mysql.GetGenQuery().Product.WithContext(ctx).Create(product); err != nil {
		t.Fatalf("创建测试商品失败: %v", err)
	}
	orderDB, err := db.NewOrderDb(ctx)
	if err != nil {
		t.Fatalf("初始化OrderDb失败: %v", err)
	}
	orderInput := &model.Order{
		OrderNo: "O_ITEM_TEST_" + util.GenerateAlnum(8),
		UserID:  user.ID,
		Amount:  17600,
		Status:  int32(model.OrderStatusPendingPayment),
	}
	order, err := orderDB.Create(ctx, orderInput)
	if err != nil {
		t.Fatalf("创建测试订单失败: %v", err)
	}

	itemDB, err := db.NewOrderItemDb(ctx)
	if err != nil {
		t.Fatalf("初始化OrderItemDb失败: %v", err)
	}
	orderItemInput := &model.OrderItem{
		OrderID:      order.ID,
		ProductID:    product.ID,
		Quantity:     2,
		UnitPriceFen: 8800,
	}
	item, err := itemDB.Create(ctx, orderItemInput)
	if err != nil {
		t.Fatalf("创建订单明细失败: %v", err)
	}
	found, err := itemDB.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("查询订单明细失败: %v", err)
	}
	if found.OrderID != order.ID || found.ProductID != product.ID || found.Quantity != 2 || found.UnitPriceFen != 8800 {
		t.Fatalf("查询订单明细数据不一致: got=%+v", found)
	}
}
