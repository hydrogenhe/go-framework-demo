package order

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"go-framework-demo/util"
)

// 每个用例使用外层事务；业务必须沿用传入 Query，测试结束后整体回滚。
func TestExpirationTransaction(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to use local MySQL")
	}
	if err := mysql.InitMySQL(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name              string
		status            int32
		future            bool
		invalidSecondItem bool
		closed            bool
		wantError         bool
	}{
		{name: "expired closes once", status: int32(appconst.OrderStatusPendingPayment), closed: true},
		{name: "not due", status: int32(appconst.OrderStatusPendingPayment), future: true},
		{name: "already paid", status: int32(appconst.OrderStatusPaid)},
		{name: "stock restored before error rolls back", status: int32(appconst.OrderStatusPendingPayment), invalidSecondItem: true, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tx := mysql.GetDB().WithContext(ctx).Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			product := model.Product{ID: util.GetID(), Name: "expiration transaction fixture", Price: 100, Stock: 5}
			expireTime := time.Now().Add(-time.Minute)
			if tc.future {
				expireTime = time.Now().Add(time.Hour)
			}
			order := model.Order{ID: util.GetID(), OrderNo: strconv.FormatInt(util.GetID(), 10), UserID: util.GetID(), Amount: 200, Status: tc.status, ExpireTime: expireTime}
			item := model.OrderItem{ID: util.GetID(), OrderID: order.ID, ProductID: product.ID, Quantity: 2, UnitPriceFen: 100}
			if err := tx.Create(&product).Error; err != nil {
				t.Fatal(err)
			}
			if err := tx.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			if err := tx.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			if tc.invalidSecondItem {
				invalid := model.OrderItem{ID: item.ID + 1, OrderID: order.ID, ProductID: product.ID, Quantity: 0, UnitPriceFen: 100}
				if err := tx.Create(&invalid).Error; err != nil {
					t.Fatal(err)
				}
			}
			service := order_server.NewOrderService(ctx, query.Use(tx))
			closed, err := service.CloseIfExpired(ctx, order.ID, false)
			if (err != nil) != tc.wantError || closed != tc.closed {
				t.Fatalf("closed=%v err=%v", closed, err)
			}
			if tc.closed {
				closed, err = service.CloseIfExpired(ctx, order.ID, false)
				if err != nil || closed {
					t.Fatalf("repeat close: closed=%v err=%v", closed, err)
				}
			}
			var actualOrder model.Order
			var actualProduct model.Product
			if err := tx.First(&actualOrder, "id = ?", order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err := tx.First(&actualProduct, "id = ?", product.ID).Error; err != nil {
				t.Fatal(err)
			}
			wantStock, wantStatus := int32(5), tc.status
			if tc.closed {
				wantStock, wantStatus = 7, int32(appconst.OrderStatusCancelled)
			}
			if actualProduct.Stock != wantStock || actualOrder.Status != wantStatus {
				t.Fatalf("stock=%d status=%d; want %d %d", actualProduct.Stock, actualOrder.Status, wantStock, wantStatus)
			}
		})
	}
}
