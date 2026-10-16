package es

import (
	"context"
	"errors"
	"go-framework-demo/es/es_model"
	"go-framework-demo/util"
)

// GetOrderByID 按 ID 从 ES 读取文档。
func GetOrderByID(id int64) (*es_model.Order, error) {
	return GetOrderByIDContext(context.Background(), id)
}

// GetOrderByIDContext 使用请求上下文查询文档。
func GetOrderByIDContext(ctx context.Context, id int64) (*es_model.Order, error) {
	var order es_model.Order
	searchOptions := OrderSearchOptions{
		ID: &id,
	}
	source, err := BuildOrderElasticQuery(&searchOptions)
	if err != nil {
		return nil, err
	}
	source.Sort("id", false)
	if err := SearchOneContext(ctx, ES_ORDER, source, &order); err != nil {
		return nil, err
	}
	items := make([]es_model.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		if item.IsDel == 0 {
			items = append(items, item)
		}
	}
	order.Items = items
	return &order, nil
}

// IndexOrder 将订单及其商品明细写入 order_test 索引。
func IndexOrder(order *es_model.Order) (string, error) {
	return IndexOrderContext(context.Background(), order)
}

func IndexOrderContext(ctx context.Context, order *es_model.Order) (string, error) {
	if order == nil {
		return "", errors.New("Order对象不能为空")
	}
	doc, err := order.ToMap()
	if err != nil {
		return "", err
	}
	return IndexDocContext(ctx, ES_ORDER, util.IDString(order.ID), doc)
}

// UpdateOrder 更新订单及其商品明细 ES 文档。
func UpdateOrder(order *es_model.Order) error {
	if order == nil {
		return errors.New("Order对象不能为空")
	}
	doc, err := order.ToMap()
	if err != nil {
		return err
	}
	return UpdateDoc(ES_ORDER, util.IDString(order.ID), doc)
}
