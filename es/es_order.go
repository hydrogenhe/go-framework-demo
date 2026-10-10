package es

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-framework-demo/es/es_model"
)

type orderSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source es_model.Order `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// GetOrderByID 按订单 ID 从 ES 读取订单及其商品明细。
func GetOrderByID(id int64) (*es_model.Order, error) {
	result, err := SearchDocs(ES_ORDER, BuildOrderElasticQuery(OrderSearchOptions{ID: id}))
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var response orderSearchResponse
	if err := json.Unmarshal(b, &response); err != nil {
		return nil, err
	}
	if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("订单不存在: %d", id)
	}
	return &response.Hits.Hits[0].Source, nil
}

// IndexOrder 将订单及其商品明细写入 order_test 索引。
func IndexOrder(order *es_model.Order) (string, error) {
	if order == nil {
		return "", errors.New("Order对象不能为空")
	}
	doc, err := order.ToMap()
	if err != nil {
		return "", err
	}
	return IndexDoc(ES_ORDER, fmt.Sprintf("%d", order.ID), doc)
}
