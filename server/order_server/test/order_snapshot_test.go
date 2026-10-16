package order

import (
	"context"
	"strconv"
	"testing"

	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/test/support"
	"go-framework-demo/util"
)

func TestIntegrationOrderSnapshot(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		product := &model.Product{ID: util.GetID(), Name: "下单名称", Price: 1100, Stock: 5, Image: "https://example.com/old.png", CurrentVersion: 1}
		order := &model.Order{ID: util.GetID(), UserID: util.GetID(), OrderNo: strconv.FormatInt(util.GetID(), 10), Amount: 2200, Status: 1}
		other := &model.Order{ID: util.GetID(), UserID: order.UserID, OrderNo: strconv.FormatInt(util.GetID(), 10), Amount: 1100, Status: 1}
		if err := q.Product.WithContext(ctx).Create(product); err != nil {
			t.Fatal(err)
		}
		if err := q.Order.WithContext(ctx).Create(order, other); err != nil {
			t.Fatal(err)
		}
		item := &model.OrderItem{ID: util.GetID(), OrderID: order.ID, ProductID: product.ID, ProductVersion: 1, ProductName: product.Name, Image: product.Image, UnitPriceFen: 1100, Quantity: 2, Status: 1}
		otherItem := &model.OrderItem{ID: util.GetID(), OrderID: other.ID, ProductID: product.ID, ProductName: "另一个订单", UnitPriceFen: 1100, Quantity: 1, Status: 1}
		if err := q.OrderItem.WithContext(ctx).Create(item, otherItem); err != nil {
			t.Fatal(err)
		}
		service := order_server.NewOrderService(ctx, q)
		p := q.Product
		if _, err := p.WithContext(ctx).Where(p.ID.Eq(product.ID)).UpdateColumnSimple(p.Name.Value("新名称"), p.Price.Value(9900), p.Image.Value("https://example.com/new.png")); err != nil {
			t.Fatal(err)
		}
		for _, deleted := range []bool{false, true} {
			if deleted {
				if _, err := p.WithContext(ctx).Where(p.ID.Eq(product.ID)).Delete(); err != nil {
					t.Fatal(err)
				}
			}
			snapshot, err := service.GetItemSnapshot(ctx, order.ID, item.ID, order.UserID)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.ProductName != item.ProductName || snapshot.Image != item.Image || snapshot.UnitPriceFen != 1100 || snapshot.ProductVersion != 1 || snapshot.Quantity != 2 {
				t.Fatalf("snapshot changed: %+v", snapshot)
			}
		}
		for _, input := range []struct{ orderID, itemID, userID int64 }{{order.ID, item.ID, order.UserID + 1}, {order.ID, otherItem.ID, order.UserID}} {
			_, err := service.GetItemSnapshot(ctx, input.orderID, input.itemID, input.userID)
			business, ok := appconst.AsMsgError(err)
			if !ok || business.Code != appconst.CodeNotFound {
				t.Fatalf("unauthorized snapshot: %v", err)
			}
		}
		if _, err := service.GetAdminItemSnapshot(ctx, order.ID, item.ID); err != nil {
			t.Fatal(err)
		}
	})
}
