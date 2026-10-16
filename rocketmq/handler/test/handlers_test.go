package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"go-framework-demo/rocketmq/handler"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	goredis "github.com/go-redis/redis/v8"
	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/mysql"
	"go-framework-demo/redis"
	"go-framework-demo/rocketmq/message"
	"go-framework-demo/util"
)

type transportFunc func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试提供的 HTTP 响应函数，模拟外部服务的请求和响应。
func (f transportFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type esRequest struct {
	Method string
	Path   string
	Body   string
}

// fakeES 创建 ES HTTP 替身，记录写入请求并按需模拟失败，测试结束后恢复原客户端。
func fakeES(t *testing.T, fail bool) *[]esRequest {
	t.Helper()
	requests := []esRequest{}
	httpClient := &http.Client{
		Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
			body := ""
			if request.Body != nil {
				raw, _ := io.ReadAll(request.Body)
				body = string(raw)
			}
			esRequestInput := esRequest{
				Method: request.Method,
				Path:   request.URL.Path,
				Body:   body,
			}
			requests = append(requests, esRequestInput)
			if fail {
				return nil, errors.New("simulated ES failure")
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"_index":"test","_id":"9007199254740993","result":"updated"}`)), Request: request}, nil
		}),
	}
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetMaxRetries(0), elastic.SetHttpClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	previous := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
	return &requests
}

// mqMessage 将测试数据编码为 RocketMQ 消息，供真实消费 Handler 处理。
func mqMessage(t *testing.T, payload interface{}) *primitive.MessageExt {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return &primitive.MessageExt{Message: primitive.Message{
		Body: body,
	}, MsgId: "test-message"}
}

// orderBusinessInput 构造合法的订单业务消息，供消费成功和参数错误测试复用。
func orderBusinessInput() message.OrderMessage {
	return message.OrderMessage{
		Action:  message.ActionCreate,
		OrderID: "9007199254740993",
		UserID:  "1",
		Amount:  2000,
		Items: []message.OrderItemMessage{{
			ProductID:    "9007199254740995",
			Quantity:     2,
			UnitPriceFen: 1000,
		}},
	}
}

// userBusinessInput 构造合法的用户业务消息，供协议区分测试复用。
func userBusinessInput() message.UserMessage {
	return message.UserMessage{
		Action:   message.ActionCreate,
		UserID:   "9007199254740993",
		UserName: "test-user",
	}
}

// TestBusinessConsumersDoNotWriteES 验证订单和用户业务消费者处理业务消息时不会写入 ES。
func TestBusinessConsumersDoNotWriteES(t *testing.T) {
	requests := fakeES(t, false)
	handlers := handler.Handlers()
	for _, test := range []struct {
		Group   string
		Payload interface{}
	}{
		{
			Group:   "order_consumer_group",
			Payload: orderBusinessInput(),
		},
		{
			Group:   "user_consumer_group_info",
			Payload: userBusinessInput(),
		},
		{
			Group:   "user_consumer_group_notify",
			Payload: userBusinessInput(),
		},
	} {
		result, err := handlers[test.Group](context.Background(), mqMessage(t, test.Payload))
		if err != nil || result != consumer.ConsumeSuccess {
			t.Fatalf("%s rejected business message: %v", test.Group, err)
		}
	}
	if len(*requests) != 0 {
		t.Fatal("business consumer wrote Elasticsearch")
	}
}

// TestConsumersRejectOtherProtocol 验证业务消息与 CDC 消息不能混用，协议不匹配时返回重试且不写 ES。
func TestConsumersRejectOtherProtocol(t *testing.T) {
	requests := fakeES(t, false)
	handlers := handler.Handlers()
	for _, group := range []string{"order_es_consumer_group", "user_es_consumer_group", "product_es_consumer_group"} {
		result, err := handlers[group](context.Background(), mqMessage(t, orderBusinessInput()))
		if err == nil || result != consumer.ConsumeRetryLater {
			t.Fatalf("%s silently accepted business JSON", group)
		}
	}
	event := canalEvent{
		Table:     "order",
		EventType: "INSERT",
		Data:      []map[string]interface{}{{"id": "9007199254740993"}},
	}
	for _, group := range []string{"order_consumer_group", "user_consumer_group_info", "user_consumer_group_notify"} {
		result, err := handlers[group](context.Background(), mqMessage(t, event))
		if err == nil || result != consumer.ConsumeRetryLater {
			t.Fatalf("%s silently accepted CDC JSON", group)
		}
	}
	if len(*requests) != 0 {
		t.Fatal("invalid protocol reached ES")
	}
}

// TestCDCConsumersWriteES 验证 CDC 消息写入或删除正确的 ES 索引，并使用业务主键作为文档 ID。
func TestCDCConsumersWriteES(t *testing.T) {
	prepareCDCDB(t)
	requests := fakeES(t, false)
	for _, test := range []struct {
		Group  string
		Event  canalEvent
		Method string
		Index  string
	}{
		{
			Group: "order_es_consumer_group",
			Event: canalEvent{
				Table:     "order",
				EventType: "DELETE",
				Data:      []map[string]interface{}{{"id": "9007199254740993"}},
			},
			Method: "PUT",
			Index:  es.ES_ORDER,
		},
		{
			Group: "product_es_consumer_group",
			Event: canalEvent{
				Table:     "product",
				EventType: "UPDATE",
				Data:      []map[string]interface{}{{"id": int64(9007199254740993), "name": "商品", "price": "1000", "stock": "10", "create_time": "2026-10-12 12:00:00.123"}},
			},
			Method: "PUT",
			Index:  es.ES_PRODUCT,
		},
		{
			Group: "user_es_consumer_group",
			Event: canalEvent{
				Table:     "user",
				EventType: "INSERT",
				Data:      []map[string]interface{}{{"id": "9007199254740993", "name": "用户", "age": "18", "email": util.RandomTestEmail(), "create_time": "2026-10-12 12:00:00"}},
			},
			Method: "PUT",
			Index:  es.ES_USER,
		},
	} {
		before := len(*requests)
		result, err := handler.Handlers()[test.Group](context.Background(), mqMessage(t, test.Event))
		if err != nil || result != consumer.ConsumeSuccess {
			t.Fatalf("%s failed CDC handling: %v", test.Group, err)
		}
		if len(*requests) != before+1 {
			t.Fatalf("%s did not write ES", test.Group)
		}
		request := (*requests)[before]
		if request.Method != test.Method || !strings.Contains(request.Path, "/"+test.Index+"/") || !strings.HasSuffix(request.Path, "/9007199254740993") {
			t.Fatalf("wrong ES request: %+v", request)
		}
	}
}

// TestCDCFailureRetries 模拟 ES 写入失败，验证 CDC 消费者返回重试而不是确认成功。
func TestCDCFailureRetries(t *testing.T) {
	prepareCDCDB(t)
	requests := fakeES(t, true)
	input := canalEvent{
		Table:     "order",
		EventType: "DELETE",
		Data:      []map[string]interface{}{{"id": "9007199254740993"}},
	}
	result, err := handler.Handlers()["order_es_consumer_group"](context.Background(), mqMessage(t, input))
	if err == nil || result != consumer.ConsumeRetryLater || len(*requests) == 0 {
		t.Fatal("ES failure was acknowledged as success")
	}
}

// TestInvalidBusinessMessageRetries 验证缺少订单明细的业务消息返回重试，且不会写入 ES。
func TestInvalidBusinessMessageRetries(t *testing.T) {
	requests := fakeES(t, false)
	input := orderBusinessInput()
	input.Items = nil
	result, err := handler.Handlers()["order_consumer_group"](context.Background(), mqMessage(t, input))
	if err == nil || result != consumer.ConsumeRetryLater {
		t.Fatal("invalid business message acknowledged")
	}
	if len(*requests) != 0 {
		t.Fatal("invalid business message wrote ES")
	}
}

// TestProductCDCCreateTime 验证旧消息字段不能覆盖 MySQL 最新时间和软删除标记。
func TestProductCDCCreateTime(t *testing.T) {
	prepareCDCDB(t)
	q := mysql.GetGenQuery().Product
	if _, err := q.WithContext(context.Background()).Where(q.ID.Eq(9007199254740993)).Delete(); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"INSERT", "UPDATE", "DELETE"} {
		requests := fakeES(t, false)
		event := canalEvent{
			Table: "product", EventType: eventType,
			Data: []map[string]interface{}{{"id": "9007199254740993", "is_del": "0", "create_time": "invalid"}},
		}
		result, err := handler.Handlers()["product_es_consumer_group"](context.Background(), mqMessage(t, event))
		if err != nil || result != consumer.ConsumeSuccess || len(*requests) != 1 {
			t.Fatalf("sync failed: %v %v", result, err)
		}
		var doc struct {
			IsDel           int64     `json:"is_del"`
			CreateTime      time.Time `json:"create_time"`
			CreateTimeStamp int64     `json:"create_time_stamp"`
		}
		if err := json.Unmarshal([]byte((*requests)[0].Body), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.IsDel != 1 || !doc.CreateTime.Equal(cdcFixtureTime) || doc.CreateTimeStamp != cdcFixtureTime.UnixMilli() {
			t.Fatalf("old event replaced database state: %+v", doc)
		}
	}
}

// TestProductCDCUpdatesRedisFromMySQL 验证商品 CDC 依据 MySQL 最新状态回写或清理 Redis。
func TestProductCDCUpdatesRedisFromMySQL(t *testing.T) {
	prepareCDCDB(t)
	prepareCDCRedis(t)
	requests := fakeES(t, false)
	id := int64(9007199254740993)
	cache := redis.NewProductCache()
	if err := cache.DelProduct(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	event := canalEvent{Table: "product", EventType: "INSERT", Data: []map[string]interface{}{{"id": id}}}
	result, err := handler.Handlers()["product_es_consumer_group"](context.Background(), mqMessage(t, event))
	if err != nil || result != consumer.ConsumeSuccess {
		t.Fatalf("active product CDC failed: %v", err)
	}
	cached, hit, err := cache.GetProduct(context.Background(), id)
	if err != nil || !hit || cached.ID != id || cached.Name != "商品" {
		t.Fatalf("active product was not cached from MySQL: product=%+v hit=%v err=%v", cached, hit, err)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one ES write for active product, got %d", len(*requests))
	}

	q := mysql.GetGenQuery().Product
	if _, err := q.WithContext(context.Background()).Where(q.ID.Eq(id)).Delete(); err != nil {
		t.Fatal(err)
	}
	deletedEvent := canalEvent{Table: "product", EventType: "UPDATE", Data: []map[string]interface{}{{"id": id}}}
	result, err = handler.Handlers()["product_es_consumer_group"](context.Background(), mqMessage(t, deletedEvent))
	if err != nil || result != consumer.ConsumeSuccess {
		t.Fatalf("soft-deleted product CDC failed: %v", err)
	}
	if _, hit, err := cache.GetProduct(context.Background(), id); err != nil || hit {
		t.Fatalf("soft-deleted product cache was not cleared: hit=%v err=%v", hit, err)
	}
}

// TestProductCDCSideEffectsIndependent 验证 ES 和 Redis 各自失败时，另一端仍会被尝试。
func TestProductCDCSideEffectsIndependent(t *testing.T) {
	prepareCDCDB(t)
	id := int64(9007199254740993)
	for _, test := range []struct {
		name      string
		failES    bool
		failRedis bool
	}{
		{name: "ES失败仍写Redis", failES: true},
		{name: "Redis失败ES仍写入", failRedis: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cache := redis.NewProductCache()
			if err := cache.DelProduct(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			requests := fakeES(t, test.failES)
			if test.failRedis {
				// 临时切换到不可用地址，验证 Redis 写入失败时 ES 仍会被尝试。
				previousClient := redis.RedisClient
				redis.RedisClient = goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 100 * time.Millisecond})
				failedClient := redis.RedisClient
				t.Cleanup(func() {
					_ = failedClient.Close()
					redis.RedisClient = previousClient
				})
			}
			event := canalEvent{Table: "product", EventType: "UPDATE", Data: []map[string]interface{}{{"id": id}}}
			result, err := handler.Handlers()["product_es_consumer_group"](context.Background(), mqMessage(t, event))
			if err == nil || result != consumer.ConsumeRetryLater {
				t.Fatal("failed side effect must propagate so the consumer can retry")
			}
			if len(*requests) != 1 {
				t.Fatalf("ES side effect was not attempted: %d requests", len(*requests))
			}
			if !test.failRedis {
				cached, hit, cacheErr := cache.GetProduct(context.Background(), id)
				if cacheErr != nil || !hit || cached.ID != id {
					t.Fatalf("Redis side effect was not applied: product=%+v hit=%v err=%v", cached, hit, cacheErr)
				}
			}
		})
	}
}
