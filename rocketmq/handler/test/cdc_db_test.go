package handler_test

import (
	"context"
	"encoding/json"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"go-framework-demo/redis"
	"go-framework-demo/rocketmq/handler"
	"os"
	"testing"
	"time"
)

var cdcFixtureTime = time.Date(2026, 10, 12, 12, 0, 0, 123000000, time.Local)

// prepareCDCRedis 初始化并验证真实 Redis 客户端，并在测试结束时清理固定商品的缓存键。
func prepareCDCRedis(t *testing.T) {
	t.Helper()
	if redis.RedisClient == nil {
		if err := redis.Init(); err != nil {
			t.Fatalf("initialize Redis for CDC integration test: %v", err)
		}
	}
	ctx := context.Background()
	if err := redis.RedisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect Redis for CDC integration test: %v", err)
	}
	cache := redis.NewProductCache()
	id := int64(9007199254740993)
	if err := cache.DelProduct(ctx, id); err != nil {
		t.Fatalf("clear Redis fixture before CDC test: %v", err)
	}
	t.Cleanup(func() {
		if err := cache.DelProduct(ctx, id); err != nil {
			t.Errorf("clear Redis fixture after CDC test: %v", err)
		}
	})
}

// 实际数据库读取不可被测试复制的业务逻辑替代；事务回滚隔离所有测试数据。
func prepareCDCDB(t *testing.T) {
	t.Helper()
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 for MySQL-backed CDC tests")
	}
	prepareCDCRedis(t)
	mysql.InitMySQL()
	tx := mysql.GetDB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	original := *mysql.GetGenQuery()
	*mysql.GetGenQuery() = *query.Use(tx)
	t.Cleanup(func() { *mysql.GetGenQuery() = original; tx.Rollback() })
	id := int64(9007199254740993)
	// 主键为本测试固定数据，外部事务回滚，不影响已有 Demo 数据。
	for _, table := range []string{"order_item", "order", "product", "user"} {
		if err := tx.Table(table).Where("id = ?", id).Delete(nil).Error; err != nil {
			t.Fatal(err)
		}
	}
	user := model.User{ID: id, Name: "cdc_fixture", Age: 18, Email: "cdc_fixture@example.com", Password: "hashed", CreateTime: cdcFixtureTime}
	product := model.Product{ID: id, Name: "商品", Price: 1000, Stock: 10, CreateTime: cdcFixtureTime}
	order := model.Order{ID: id, OrderNo: "cdc_fixture", UserID: id, Amount: 1000, Status: 1, CreateTime: cdcFixtureTime}
	item := model.OrderItem{ID: id, OrderID: id, ProductID: id, Quantity: 1, UnitPriceFen: 1000, Status: 1}
	for _, record := range []interface{}{&user, &product, &order, &item} {
		if err := tx.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// 验证用户软删、订单明细软删以及订单软删均通过注册的真实 CDC Handler 同步。
func TestCDCSoftDeletedUserOrderAndItem(t *testing.T) {
	prepareCDCDB(t)
	ctx := context.Background()
	q := mysql.GetGenQuery()
	id := int64(9007199254740993)
	if _, err := q.User.WithContext(ctx).Where(q.User.ID.Eq(id)).Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := q.OrderItem.WithContext(ctx).Where(q.OrderItem.ID.Eq(id)).Delete(); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		Group, Table string
		Deleted      int64
	}{
		{"user_es_consumer_group", "user", 1},
		{"order_es_consumer_group", "order_item", 0},
		{"order_es_consumer_group", "order", 1},
	} {
		if input.Table == "order" {
			if _, err := q.Order.WithContext(ctx).Where(q.Order.ID.Eq(id)).Delete(); err != nil {
				t.Fatal(err)
			}
		}
		requests := fakeES(t, false)
		event := canalEvent{Table: input.Table, EventType: "UPDATE", Data: []map[string]interface{}{{"id": id, "order_id": id, "is_del": "0"}}}
		result, err := handler.Handlers()[input.Group](ctx, mqMessage(t, event))
		if err != nil || result != consumer.ConsumeSuccess || len(*requests) != 1 {
			t.Fatalf("%s: %v %v", input.Table, result, err)
		}
		var doc struct {
			IsDel int64 `json:"is_del"`
			Items []struct {
				IsDel int64 `json:"is_del"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte((*requests)[0].Body), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.IsDel != input.Deleted {
			t.Fatalf("%s: wrong deletion state %+v", input.Table, doc)
		}
		if input.Table == "order_item" && (len(doc.Items) != 1 || doc.Items[0].IsDel != 1) {
			t.Fatalf("deleted order item not synchronized: %+v", doc)
		}
	}
}
