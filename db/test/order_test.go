package test

import (
	"context"
	"testing"

	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

// TestOrderCreateAndGet 使用真实 MySQL，验证订单持久化、自动生成 ID 及查询结果。
func TestOrderCreateAndGet(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()

	userDB, err := db.NewUserDb(ctx)
	if err != nil {
		t.Fatalf("初始化UserDb失败: %v", err)
	}
	user, err := userDB.Create(ctx, fixtureUser(t, "order_test_"+util.GenerateAlnum(8), 30, util.RandomTestEmail(), util.GenerateAlnum(8)))
	if err != nil {
		t.Fatalf("创建订单测试用户失败: %v", err)
	}

	orderDB, err := db.NewOrderDb(ctx)
	if err != nil {
		t.Fatalf("初始化OrderDb失败: %v", err)
	}
	orderNo := "O_TEST_" + util.GenerateAlnum(10)
	orderInput := &model.Order{
		OrderNo: orderNo,
		UserID:  user.ID,
		Amount:  19900,
		Status:  int32(model.OrderStatusPendingPayment),
	}
	created, err := orderDB.Create(ctx, orderInput)
	if err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
	if created.ID <= 0 {
		t.Fatal("创建订单后未生成有效ID")
	}

	found, err := orderDB.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("按ID查询订单失败: %v", err)
	}
	if found.OrderNo != orderNo || found.UserID != user.ID || found.Amount != 19900 {
		t.Fatalf("查询订单数据不一致: got=%+v", found)
	}
}
