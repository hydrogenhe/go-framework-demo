package order

import (
	"context"
	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/test/support"
	"go-framework-demo/server/user_server"

	"go-framework-demo/util"
	"testing"
)

// TestIntegrationOrderValidation 验证价格和快照变化时订单及库存回滚，正常下单正确计算金额、扣库存并保存时间。
func TestIntegrationOrderValidation(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		users := user_server.NewUserService(ctx, q)
		input := support.RandomUserInput()
		user, err := users.CreateUser(ctx, input.Name, input.Age, input.Email, input.Password)
		if err != nil {
			t.Fatal(err)
		}
		product := &model.Product{
			Name:  "validation_product_" + util.GenerateAlnum(8),
			Price: 1000,
			Stock: 10,
			Image: "https://example.com/original-product.png",
		}
		if err = q.Product.WithContext(ctx).Create(product); err != nil {
			t.Fatal(err)
		}
		product, err = q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).First()
		if err != nil {
			t.Fatal(err)
		}
		orders := order_server.NewOrderService(ctx, q)
		staleVersion := []order_server.OrderItemInput{{
			ProductID:      product.ID,
			ProductVersion: product.CurrentVersion + 1,
			Quantity:       1,
			UnitPriceFen:   product.Price,
		}}
		_, err = orders.CreateOrderWithItems(ctx, "stale_"+util.GenerateAlnum(8), user.ID, staleVersion)
		if msg, ok := appconst.AsMsgError(err); !ok || msg.Message != "数据变化，请刷新页面" {
			t.Fatalf("stale product version accepted: %v", err)
		}
		itemsWithChangedPrice := []order_server.OrderItemInput{{
			ProductID:    product.ID,
			Quantity:     1,
			UnitPriceFen: 1,
		}}
		_, err = orders.CreateOrderWithItems(ctx, "validation_"+util.GenerateAlnum(8), user.ID, itemsWithChangedPrice)
		if msg, ok := appconst.AsMsgError(err); !ok || msg.Message != "数据变化，请刷新页面" {
			t.Fatalf("unexpected changed-data result: %v", err)
		}
		afterRejectedOrder, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).First()
		if err != nil || afterRejectedOrder.Stock != 10 {
			t.Fatal("failed order modified inventory")
		}

		// 任一商品快照校验失败时，不应扣减任何商品库存或创建订单。
		second := &model.Product{
			Name:  "validation_second_" + util.GenerateAlnum(8),
			Price: 500,
			Stock: 5,
		}
		if err := q.Product.WithContext(ctx).Create(second); err != nil {
			t.Fatal(err)
		}
		partial := []order_server.OrderItemInput{
			{
				ProductID:    product.ID,
				Quantity:     2,
				UnitPriceFen: product.Price,
			},
			{
				ProductID:    second.ID,
				Quantity:     1,
				UnitPriceFen: 1,
			},
		}
		rejectedNo := "validation_" + util.GenerateAlnum(8)
		if _, err := orders.CreateOrderWithItems(ctx, rejectedNo, user.ID, partial); err == nil {
			t.Fatal("changed second product accepted")
		}
		first, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).First()
		if err != nil || first.Stock != 10 {
			t.Fatal("earlier inventory deduction was not rolled back")
		}
		count, err := q.Order.WithContext(ctx).Where(q.Order.OrderNo.Eq(rejectedNo)).Count()
		if err != nil || count != 0 {
			t.Fatal("rejected order persisted")
		}
		for _, snapshot := range []order_server.OrderItemInput{
			{
				ProductID:    product.ID,
				Quantity:     1,
				UnitPriceFen: product.Price,
				Name:         &second.Name,
			},
			{
				ProductID:    product.ID,
				Quantity:     1,
				UnitPriceFen: product.Price,
				Stock:        &second.Stock,
			},
		} {
			inputs := []order_server.OrderItemInput{snapshot}
			if _, err := orders.CreateOrderWithItems(ctx, "validation_"+util.GenerateAlnum(8), user.ID, inputs); err == nil {
				t.Fatal("changed snapshot accepted")
			}
		}

		items := []order_server.OrderItemInput{{
			ProductID:    product.ID,
			Quantity:     2,
			UnitPriceFen: product.Price,
		}}
		order, err := orders.CreateOrderWithItems(ctx, "validation_"+util.GenerateAlnum(8), user.ID, items)
		if err != nil {
			t.Fatal(err)
		}
		if order.CreateTime.IsZero() {
			t.Fatal("order creation time missing")
		}
		if order.Amount != 2000 {
			t.Fatalf("wrong total %d", order.Amount)
		}
		itemsInOrder, err := q.OrderItem.WithContext(ctx).Where(q.OrderItem.OrderID.Eq(order.ID)).Find()
		if err != nil || len(itemsInOrder) != 1 || itemsInOrder[0].ProductVersion != product.CurrentVersion {
			t.Fatalf("商品资料版本未记录到订单明细: items=%+v err=%v", itemsInOrder, err)
		}
		// 商品换图后，订单明细仍保留下单时的图片。
		if _, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).Update(q.Product.Image, "https://example.com/new-product.png"); err != nil {
			t.Fatal(err)
		}
		savedItems, err := q.OrderItem.WithContext(ctx).Where(q.OrderItem.OrderID.Eq(order.ID)).Find()
		if err != nil || len(savedItems) != 1 || savedItems[0].Image != product.Image {
			t.Fatalf("图片快照未保留: items=%+v err=%v", savedItems, err)
		}
		actual, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).First()
		if err != nil || actual.Stock != 8 {
			t.Fatal("wrong inventory after successful order")
		}
	})
}
