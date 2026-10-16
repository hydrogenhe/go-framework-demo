package es

import (
	"fmt"
	appconst "go-framework-demo/const"
	"go-framework-demo/es/es_model"
	"go-framework-demo/model"
)

// OrderFromModels 是 CDC 与对账共用的订单文档映射。
func OrderFromModels(o *model.Order, items []*model.OrderItem) (*es_model.Order, error) {
	if o.CreateTime.IsZero() {
		return nil, fmt.Errorf("订单 %d 缺少创建时间，请先执行订单时间迁移", o.ID)
	}
	doc := &es_model.Order{
		IsDel:      int64(o.IsDel),
		ID:         o.ID,
		OrderNo:    o.OrderNo,
		UserID:     o.UserID,
		AmountFen:  o.Amount,
		Status:     appconst.OrderStatus(o.Status),
		CreateTime: o.CreateTime,
		UpdateTime: o.UpdateTime,
		Items:      make([]es_model.OrderItem, 0, len(items)),
	}
	for _, item := range items {
		orderItem := es_model.OrderItem{
			ID:           item.ID,
			IsDel:        int64(item.IsDel),
			ProductID:    item.ProductID,
			ProductName:  item.ProductName,
			Image:        item.Image,
			Quantity:     item.Quantity,
			UnitPriceFen: item.UnitPriceFen,
			Status:       appconst.OrderItemStatus(item.Status),
			UpdateTime:   item.UpdateTime,
		}
		doc.Items = append(doc.Items, orderItem)
	}
	return doc, nil
}
