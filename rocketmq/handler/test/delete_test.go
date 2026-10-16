package handler_test

import (
	"context"
	"encoding/json"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"go-framework-demo/rocketmq/handler"
	"testing"
)

// 历史物理删除或旧事件找不到 MySQL 主键时，写软删除墓碑而不是重试或复活旧文档。
func TestCDCMissingRowsMarkDeleted(t *testing.T) {
	prepareCDCDB(t)
	for _, input := range []struct{ Group, Table string }{
		{"product_es_consumer_group", "product"},
		{"user_es_consumer_group", "user"},
		{"order_es_consumer_group", "order"},
	} {
		requests := fakeES(t, false)
		event := canalEvent{Table: input.Table, EventType: "DELETE", Data: []map[string]interface{}{{"id": "9007199254740988"}}}
		result, err := handler.Handlers()[input.Group](context.Background(), mqMessage(t, event))
		if err != nil || result != consumer.ConsumeSuccess || len(*requests) != 1 {
			t.Fatalf("%s: %v %v", input.Group, result, err)
		}
		var doc struct {
			IsDel int64 `json:"is_del"`
		}
		if err := json.Unmarshal([]byte((*requests)[0].Body), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.IsDel != 1 || (*requests)[0].Method != "PUT" {
			t.Fatalf("not a soft delete: %+v", (*requests)[0])
		}
	}
}
