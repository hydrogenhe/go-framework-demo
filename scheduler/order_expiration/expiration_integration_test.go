package order_expiration

import (
	"context"
	"os"
	"testing"
	"time"

	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

func TestCloseExpiredOrderRestocksOnlyOnce(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to use local MySQL")
	}
	if err := mysql.InitMySQL(); err != nil {
		t.Fatal(err)
	}
	database := mysql.GetDB()
	ctx := context.Background()
	product := model.Product{ID: util.GetID(), Name: "expiration fixture", Price: 100, Stock: 5}
	order := model.Order{ID: util.GetID(), OrderNo: "expire_fixture", UserID: util.GetID(), Amount: 200, Status: int32(appconst.OrderStatusPendingPayment), ExpireTime: time.Now().Add(-time.Minute)}
	item := model.OrderItem{ID: util.GetID(), OrderID: order.ID, ProductID: product.ID, Quantity: 2, UnitPriceFen: 100}
	if err := database.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.Unscoped().Where("id = ?", item.ID).Delete(&model.OrderItem{})
		database.Unscoped().Where("id = ?", order.ID).Delete(&model.Order{})
		database.Unscoped().Where("id = ?", product.ID).Delete(&model.Product{})
	})
	if err := database.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	closed, err := CloseIfExpired(ctx, database, order.ID)
	if err != nil || !closed {
		t.Fatalf("first close: closed=%v err=%v", closed, err)
	}
	closed, err = CloseIfExpired(ctx, database, order.ID)
	if err != nil || closed {
		t.Fatalf("second close: closed=%v err=%v", closed, err)
	}
	var actualOrder model.Order
	var actualProduct model.Product
	if err := database.First(&actualOrder, "id = ?", order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.First(&actualProduct, "id = ?", product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if actualOrder.Status != int32(appconst.OrderStatusCancelled) || actualProduct.Stock != 7 {
		t.Fatalf("order status=%d product stock=%d; want cancelled and stock 7", actualOrder.Status, actualProduct.Stock)
	}
}
