package order

import (
	"context"
	"testing"

	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/test/support"
)

func TestIntegrationOrderActions(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		input := support.RandomUserInput()
		user := &model.User{Name: input.Name, Email: input.Email, Age: input.Age, Password: input.Password}
		if err := q.User.WithContext(ctx).Create(user); err != nil {
			t.Fatal(err)
		}
		orders := []*model.Order{
			{OrderNo: "actions_first", UserID: user.ID, Amount: 200, Status: 1},
			{OrderNo: "actions_second", UserID: user.ID, Amount: 100, Status: 1},
			{OrderNo: "actions_other", UserID: user.ID + 1, Amount: 100, Status: 1},
		}
		if err := q.Order.WithContext(ctx).Create(orders...); err != nil {
			t.Fatal(err)
		}
		product := &model.Product{Name: "actions_product", Price: 100, Stock: 5}
		if err := q.Product.WithContext(ctx).Create(product); err != nil {
			t.Fatal(err)
		}
		items := []*model.OrderItem{
			{OrderID: orders[0].ID, ProductID: product.ID, Quantity: 1, UnitPriceFen: 100, Status: 1},
			{OrderID: orders[0].ID, ProductID: product.ID, Quantity: 1, UnitPriceFen: 100, Status: 1},
			{OrderID: orders[1].ID, ProductID: product.ID, Quantity: 1, UnitPriceFen: 100, Status: 1},
		}
		if err := q.OrderItem.WithContext(ctx).Create(items...); err != nil {
			t.Fatal(err)
		}
		service := order_server.NewOrderService(ctx, q)
		if _, err := service.PayOrder(ctx, orders[2].ID, user.ID); err == nil {
			t.Fatal("paid another user's order")
		}
		if _, err := service.ConfirmReceipt(ctx, orders[0].ID, user.ID); err == nil {
			t.Fatal("completed unpaid order")
		}
		if _, err := service.ShipOrderItem(ctx, orders[0].ID, items[0].ID); err == nil {
			t.Fatal("shipped unpaid order")
		}
		paid, err := service.PayOrder(ctx, orders[0].ID, user.ID)
		if err != nil || paid.Status != 2 {
			t.Fatal(paid, err)
		}
		if _, err := service.PayOrder(ctx, orders[0].ID, user.ID); err != nil {
			t.Fatal("repeat pay", err)
		}
		if _, err := service.ShipOrderItem(ctx, orders[0].ID, items[2].ID); err == nil {
			t.Fatal("shipped item from different order")
		}
		shipped, err := service.ShipOrderItem(ctx, orders[0].ID, items[0].ID)
		if err != nil || shipped.Status != 2 {
			t.Fatal(shipped, err)
		}
		if _, err := service.ShipOrderItem(ctx, orders[0].ID, items[0].ID); err != nil {
			t.Fatal("repeat ship", err)
		}
		parent, err := q.Order.WithContext(ctx).Where(q.Order.ID.Eq(orders[0].ID)).First()
		if err != nil {
			t.Fatal(err)
		}
		untouched, err := q.OrderItem.WithContext(ctx).Where(q.OrderItem.ID.Eq(items[1].ID)).First()
		if err != nil {
			t.Fatal(err)
		}
		if parent.Status != 2 || untouched.Status != 1 {
			t.Fatal("shipping modified parent or other item", parent.Status, untouched.Status)
		}
		completed, err := service.ConfirmReceipt(ctx, orders[0].ID, user.ID)
		if err != nil || completed.Status != 3 {
			t.Fatal(completed, err)
		}
		if _, err := service.ConfirmReceipt(ctx, orders[0].ID, user.ID); err != nil {
			t.Fatal("repeat receipt", err)
		}
		if _, err := service.PayOrder(ctx, orders[0].ID, user.ID); err == nil {
			t.Fatal("completed order moved backwards")
		}
		if _, err := service.ShipOrderItem(ctx, orders[0].ID, items[1].ID); err == nil {
			t.Fatal("completed order shipped")
		}
		if _, err := service.MarkShipped(ctx, orders[1].ID); err == nil {
			t.Fatal("batch shipped unpaid order")
		}
		if _, err := service.PayOrder(ctx, orders[1].ID, user.ID); err != nil {
			t.Fatal(err)
		}
		all, err := service.MarkShipped(ctx, orders[1].ID)
		if err != nil || all.Status != 2 {
			t.Fatal(all, err)
		}
		if msg, ok := appconst.AsMsgError(err); ok {
			t.Fatal(msg)
		}
	})
}
