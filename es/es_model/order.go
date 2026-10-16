package es_model

import (
	"errors"
	"time"

	appconst "go-framework-demo/const"
)

// Order 和 OrderItem 定义在 es 包中，模型文件保留给用户索引结构。

// OrderItem 订单中的商品快照。
type OrderItem struct {
	ID              int64                    `json:"id"`
	IsDel           int64                    `json:"is_del"`
	Image           string                   `json:"image"`
	ProductID       int64                    `json:"product_id"`
	ProductName     string                   `json:"product_name"`
	Quantity        int32                    `json:"quantity"`
	UnitPriceFen    int64                    `json:"unit_price_fen"`
	Status          appconst.OrderItemStatus `json:"status"`
	UpdateTime      time.Time                `json:"update_time"`
	UpdateTimeStamp int64                    `json:"update_time_stamp"`
}

// Order ES 订单文档，Items 使用 nested 数组写入。
type Order struct {
	IsDel           int64                `json:"is_del"`
	ID              int64                `json:"id"`
	OrderNo         string               `json:"order_no"`
	UserID          int64                `json:"user_id"`
	AmountFen       int64                `json:"amount_fen"`
	Status          appconst.OrderStatus `json:"status"`
	CreateTime      time.Time            `json:"-"`
	CreateTimeStamp int64                `json:"create_time_stamp"`
	UpdateTime      time.Time            `json:"-"`
	UpdateTimeStamp int64                `json:"update_time_stamp"`
	Items           []OrderItem          `json:"items"`
}

func (o *Order) ToMap() (map[string]interface{}, error) {
	if o.ID <= 0 || o.OrderNo == "" || o.UserID <= 0 || o.AmountFen <= 0 || o.CreateTime.IsZero() || o.Status < appconst.OrderStatusPendingPayment || o.Status > appconst.OrderStatusCancelled {
		return nil, errors.New("订单字段或订单明细无效")
	}
	updateTime := o.UpdateTime
	if updateTime.IsZero() {
		updateTime = o.CreateTime
	}
	for _, item := range o.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.UnitPriceFen <= 0 {
			return nil, errors.New("订单商品明细无效")
		}
		if item.Status < appconst.OrderItemStatusPendingShipment || item.Status > appconst.OrderItemStatusShipped {
			return nil, errors.New("订单商品明细状态无效")
		}
	}
	items := make([]OrderItem, len(o.Items))
	copy(items, o.Items)
	for i := range items {
		if items[i].UpdateTime.IsZero() {
			items[i].UpdateTime = o.UpdateTime
			if items[i].UpdateTime.IsZero() {
				items[i].UpdateTime = o.CreateTime
			}
		}
		items[i].UpdateTimeStamp = items[i].UpdateTime.UnixMilli()
	}
	mp := map[string]interface{}{
		"is_del":     o.IsDel,
		"id":         o.ID,
		"order_no":   o.OrderNo,
		"user_id":    o.UserID,
		"amount_fen": o.AmountFen, "status": o.Status,
		"create_time":       o.CreateTime,
		"create_time_stamp": o.CreateTime.UnixMilli(),
		"update_time":       updateTime,
		"update_time_stamp": updateTime.UnixMilli(),
		"items":             items,
	}
	return mp, nil
}
