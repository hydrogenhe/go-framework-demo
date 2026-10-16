package handler_test

import (
	"context"
	"encoding/json"
	"go-framework-demo/rocketmq/handler"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

// TestIntegrationOrderCDCKeepsCreationTime 使用真实 MySQL 和 ES 替身，验证重复订单同步及状态更新不会改变创建时间。
func TestIntegrationOrderCDCKeepsCreationTime(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to use local MySQL")
	}
	mysql.InitMySQL()
	ctx := context.Background()
	requests := fakeES(t, false)
	// 消费者使用全局数据库连接，因此先提交测试数据，再通过 Cleanup 删除。
	created := time.Date(2026, 10, 12, 10, 20, 30, 123000000, time.Local)
	order := &model.Order{
		OrderNo:    "cdc_time_" + util.GenerateAlnum(8),
		UserID:     1,
		Amount:     2000,
		Status:     int32(model.OrderStatusPendingPayment),
		CreateTime: created,
	}
	product := &model.Product{
		Name:  "cdc_time_" + util.GenerateAlnum(8),
		Price: 1000,
		Stock: 10,
	}
	q := mysql.GetGenQuery()
	if err := q.Product.WithContext(ctx).Create(product); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { q.Product.WithContext(ctx).Where(q.Product.ID.Eq(product.ID)).Delete() })
	if err := q.Order.WithContext(ctx).Create(order); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { q.Order.WithContext(ctx).Where(q.Order.ID.Eq(order.ID)).Delete() })
	item := &model.OrderItem{
		OrderID:      order.ID,
		Image:        "https://example.com/order-snapshot.png",
		ProductID:    product.ID,
		Quantity:     2,
		UnitPriceFen: 1000,
		Status:       int32(model.OrderItemStatusPendingShipment),
	}
	if err := q.OrderItem.WithContext(ctx).Create(item); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { q.OrderItem.WithContext(ctx).Where(q.OrderItem.ID.Eq(item.ID)).Delete() })
	event := canalEvent{
		Table:     "order",
		EventType: "UPDATE",
		Data:      []map[string]interface{}{{"id": strconv.FormatInt(order.ID, 10)}},
	}
	callback := handler.Handlers()["order_es_consumer_group"]
	expectedStatuses := []int32{int32(model.OrderStatusPendingPayment), int32(model.OrderStatusPaid)}
	for pass := 0; pass < 2; pass++ {
		if pass == 1 {
			updated, err := q.Order.WithContext(ctx).Where(q.Order.ID.Eq(order.ID)).Update(q.Order.Status, expectedStatuses[pass])
			if err != nil {
				t.Fatal(err)
			}
			if updated.RowsAffected != 1 {
				t.Fatalf("update order status affected %d rows, want 1", updated.RowsAffected)
			}
			stored, err := q.Order.WithContext(ctx).Where(q.Order.ID.Eq(order.ID)).First()
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != expectedStatuses[pass] {
				t.Fatalf("MySQL status after update = %d, want %d", stored.Status, expectedStatuses[pass])
			}
			orderDB, err := db.NewOrderDb(ctx)
			if err != nil {
				t.Fatal(err)
			}
			cdcSource, err := orderDB.GetByIDIncludingDeleted(ctx, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if cdcSource.Status != expectedStatuses[pass] {
				t.Fatalf("CDC database reader returned status %d, want %d", cdcSource.Status, expectedStatuses[pass])
			}
		}
		result, err := callback(ctx, mqMessage(t, event))
		if err != nil || result != consumer.ConsumeSuccess {
			t.Fatalf("CDC failed: %v", err)
		}
		var doc struct {
			Items []struct {
				Image string `json:"image"`
			} `json:"items"`
			CreateTimeStamp int64 `json:"create_time_stamp"`
			Status          int32 `json:"status"`
		}
		if err := json.Unmarshal([]byte((*requests)[pass].Body), &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Items) != 1 || doc.Items[0].Image != item.Image {
			t.Fatalf("ES 图片快照错误: %+v", doc.Items)
		}
		if doc.CreateTimeStamp != created.UnixMilli() || doc.Status != expectedStatuses[pass] {
			t.Fatalf("unstable order time or stale status: pass=%d doc=%+v body=%s", pass, doc, (*requests)[pass].Body)
		}
	}
}
