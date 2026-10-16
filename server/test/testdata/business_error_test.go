package testdata

import (
	"context"
	"errors"
	appconst "go-framework-demo/const"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/test/support"
	"go-framework-demo/server/user_server"

	"go-framework-demo/util"
	"testing"
)

// TestIntegrationExplicitBusinessErrors 验证服务明确区分记录不存在、订单号冲突与数据库故障，重复更新成功。
func TestIntegrationExplicitBusinessErrors(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		users := user_server.NewUserService(ctx, q)
		input := support.RandomUserInput()
		user, err := users.CreateUser(ctx, input.Name, input.Age, input.Email, input.Password)
		if err != nil {
			t.Fatal(err)
		}
		orders := order_server.NewOrderService(ctx, q)
		no := "errors_" + util.GenerateAlnum(8)
		order, err := orders.CreateOrder(ctx, no, user.ID, 100)
		if err != nil {
			t.Fatal(err)
		}
		assertCode := func(err error, code int) {
			t.Helper()
			msg, ok := appconst.AsMsgError(err)
			if !ok || msg.Code != code {
				t.Fatalf("want code %d, got %v", code, err)
			}
		}
		_, err = orders.CreateOrder(ctx, no, user.ID, 100)
		assertCode(err, appconst.CodeConflict)
		_, err = orders.GetOrderByOrderNo(ctx, "missing_"+util.GenerateAlnum(8))
		assertCode(err, appconst.CodeNotFound)
		missing := util.GetID()
		_, err = users.GetUserByID(ctx, missing)
		assertCode(err, appconst.CodeNotFound)
		_, err = users.UpdateUser(ctx, missing, map[string]interface{}{"age": int32(20)})
		assertCode(err, appconst.CodeNotFound)
		_, err = orders.UpdateOrder(ctx, order.ID, &order_server.OrderUpdateInput{Amount: &order.Amount})
		if err != nil {
			t.Fatalf("unchanged order update: %v", err)
		}
		amount, status := int64(200), int32(2)
		updateInput := order_server.OrderUpdateInput{Amount: &amount, Status: &status}
		updated, err := orders.UpdateOrder(ctx, order.ID, &updateInput)
		if err != nil || updated.Amount != amount || updated.Status != status || updated.OrderNo != no {
			t.Fatalf("订单多字段更新错误: order=%+v err=%v", updated, err)
		}
		_, err = users.UpdateUser(ctx, user.ID, map[string]interface{}{"age": user.Age})
		if err != nil {
			t.Fatalf("unchanged user update: %v", err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		_, err = orders.GetOrderByID(canceled, order.ID)
		assertCode(err, appconst.CodeInternal)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cause: %v", err)
		}
	})
}
