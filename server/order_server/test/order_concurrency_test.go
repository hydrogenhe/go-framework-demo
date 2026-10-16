package order

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/server/order_server"
	"go-framework-demo/util"
)

// TestIntegrationOrdersDoNotOversell 验证两个并发订单争抢一份库存时仅一个成功，并确认请求取消能够传递。
func TestIntegrationOrdersDoNotOversell(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to use local MySQL")
	}
	mysql.InitMySQL()
	ctx := context.Background()
	q := mysql.GetGenQuery()
	product := &model.Product{
		Name:  "concurrency_" + util.GenerateAlnum(8),
		Price: 1000,
		Stock: 1,
	}
	if err := q.Product.WithContext(ctx).Create(product); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).Delete(); err != nil {
			t.Error(err)
		}
	})
	inputs := []order_server.OrderItemInput{{
		ProductID:    product.ID,
		Quantity:     1,
		UnitPriceFen: product.Price,
	}}
	orderNos := []string{"concurrent_" + util.GenerateAlnum(8), "concurrent_" + util.GenerateAlnum(8)}
	t.Cleanup(func() {
		orders, err := q.Order.WithContext(ctx).Where(q.Order.OrderNo.In(orderNos...)).Find()
		if err != nil {
			t.Error(err)
			return
		}
		for _, order := range orders {
			if _, err := q.OrderItem.WithContext(ctx).Where(q.OrderItem.OrderID.Eq(order.ID)).Delete(); err != nil {
				t.Error(err)
			}
			if _, err := q.Order.WithContext(ctx).Where(q.Order.ID.Eq(order.ID)).Delete(); err != nil {
				t.Error(err)
			}
		}
	})
	service := order_server.NewOrderService(ctx)
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, no := range orderNos {
		workers.Add(1)
		go func(no string) {
			defer workers.Done()
			<-start
			_, err := service.CreateOrderWithItems(ctx, no, 1, inputs)
			results <- err
		}(no)
	}
	close(start)
	workers.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
			continue
		}
		if msg, ok := appconst.AsMsgError(err); ok && msg.HTTPStatus == 409 {
			conflict++
			continue
		}
		t.Fatalf("unexpected order result: %v", err)
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	found, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).First()
	if err != nil || found.Stock != 0 {
		t.Fatalf("wrong stock: %+v err=%v", found, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = service.CreateOrderWithItems(canceled, "canceled_"+util.GenerateAlnum(8), 1, inputs)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("request cancellation was lost: %v", err)
	}
}
