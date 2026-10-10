package test

import (
	"context"
	"testing"

	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

func TestOrderItemCreateAndGet(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	userDB, err := db.NewUserDb(ctx)
	if err != nil {
		t.Fatalf("初始化UserDb失败: %v", err)
	}
	user, err := userDB.Create("item_test_"+util.GenerateAlnum(8), 30, "huapin")
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
	order, err := orderDB.Create("O_ITEM_TEST_"+util.GenerateAlnum(8), user.ID, 17600)
	if err != nil {
		t.Fatalf("创建测试订单失败: %v", err)
	}

	itemDB, err := db.NewOrderItemDb(ctx)
	if err != nil {
		t.Fatalf("初始化OrderItemDb失败: %v", err)
	}
	item, err := itemDB.Create(&model.OrderItem{OrderID: order.ID, ProductID: product.ID, Quantity: 2, UnitPriceFen: 8800})
	if err != nil {
		t.Fatalf("创建订单明细失败: %v", err)
	}
	found, err := itemDB.GetByID(item.ID)
	if err != nil {
		t.Fatalf("查询订单明细失败: %v", err)
	}
	if found.OrderID != order.ID || found.ProductID != product.ID || found.Quantity != 2 || found.UnitPriceFen != 8800 {
		t.Fatalf("查询订单明细数据不一致: got=%+v", found)
	}
}

func TestOrderCreateWithItemsTransaction(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	userDB, _ := db.NewUserDb(ctx)
	user, err := userDB.Create("tx_item_test_"+util.GenerateAlnum(8), 30, "huapin")
	if err != nil {
		t.Fatalf("创建事务测试用户失败: %v", err)
	}
	product := &model.Product{Name: "tx_product_" + util.GenerateAlnum(8), Price: 1200, Stock: 10}
	if err := mysql.GetGenQuery().Product.WithContext(ctx).Create(product); err != nil {
		t.Fatalf("创建事务测试商品失败: %v", err)
	}
	orderDB, _ := db.NewOrderDb(ctx)
	order, err := orderDB.CreateWithItems("O_TX_TEST_"+util.GenerateAlnum(8), user.ID, []*model.OrderItem{
		{ProductID: product.ID, Quantity: 2, UnitPriceFen: 1200},
	})
	if err != nil {
		t.Fatalf("订单事务创建失败: %v", err)
	}
	itemDB, _ := db.NewOrderItemDb(ctx)
	items, err := itemDB.GetByOrderID(order.ID)
	if err != nil {
		t.Fatalf("查询事务订单明细失败: %v", err)
	}
	if len(items) != 1 || items[0].ProductID != product.ID || items[0].Quantity != 2 {
		t.Fatalf("事务订单明细数据不正确: got=%+v", items)
	}
}
