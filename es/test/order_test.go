package test

import (
	"encoding/json"
	"testing"
	"time"

	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
	"go-framework-demo/util"
)

type orderSearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string         `json:"_id"`
			Source es_model.Order `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func TestESOrderCRUD(t *testing.T) {
	if err := es.InitES(); err != nil {
		t.Skipf("ES不可用，跳过测试: %v", err)
	}
	order := &es_model.Order{
		ID: util.GetID(), OrderNo: "O_ES_" + util.GenerateAlnum(8),
		UserID: util.GetID(), AmountFen: 19900, Status: "paid", CreateTime: time.Now(),
		Items: []es_model.OrderItem{
			{ProductID: util.GetID(), ProductName: "测试商品A", Quantity: 2, UnitPriceFen: 5000},
			{ProductID: util.GetID(), ProductName: "测试商品B", Quantity: 1, UnitPriceFen: 9900},
		},
	}
	docID, err := es.IndexOrder(order)
	if err != nil {
		t.Fatalf("写入Order失败: %v", err)
	}
	if docID == "" {
		t.Fatal("ES未返回订单文档ID")
	}
	result, err := es.SearchDocs(es.ES_ORDER, es.BuildOrderElasticQuery(es.OrderSearchOptions{UserID: order.UserID}))
	if err != nil {
		t.Fatalf("查询Order失败: %v", err)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("序列化订单查询结果失败: %v", err)
	}
	var response orderSearchResponse
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("订单查询结果反序列化失败: %v", err)
	}
	if response.Hits.Total.Value == 0 {
		t.Fatal("未查询到订单")
	}
	if len(response.Hits.Hits) == 0 || response.Hits.Hits[0].Source.OrderNo != order.OrderNo {
		t.Fatalf("查询到的订单数据不匹配: %+v", response.Hits.Hits)
	}
	if response.Hits.Hits[0].Source.CreateTimeStamp != order.CreateTime.UnixMilli() {
		t.Errorf("订单创建时间戳不匹配，预期：%d，实际：%d", order.CreateTime.UnixMilli(), response.Hits.Hits[0].Source.CreateTimeStamp)
	}
}
