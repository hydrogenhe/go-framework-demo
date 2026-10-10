package es_model

import (
	"errors"
	"time"
)

// Order 和 OrderItem 定义在 es 包中，模型文件保留给用户索引结构。

// OrderItem 订单中的商品快照。
type OrderItem struct {
	ProductID    int64  `json:"product_id"`
	ProductName  string `json:"product_name"`
	Quantity     int32  `json:"quantity"`
	UnitPriceFen int64  `json:"unit_price_fen"`
}

// Order ES 订单文档，Items 使用 nested 数组写入。
type Order struct {
	ID              int64       `json:"id"`
	OrderNo         string      `json:"order_no"`
	UserID          int64       `json:"user_id"`
	AmountFen       int64       `json:"amount_fen"`
	Status          string      `json:"status,omitempty"`
	CreateTime      time.Time   `json:"-"`
	CreateTimeStamp int64       `json:"create_time_stamp"`
	Items           []OrderItem `json:"items"`
}

func (o *Order) ToMap() (map[string]interface{}, error) {
	if o.ID <= 0 || o.OrderNo == "" || o.UserID <= 0 || o.AmountFen <= 0 || o.CreateTime.IsZero() || len(o.Items) == 0 {
		return nil, errors.New("订单字段或订单明细无效")
	}
	for _, item := range o.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.UnitPriceFen <= 0 {
			return nil, errors.New("订单商品明细无效")
		}
	}
	mp := map[string]interface{}{
		"id":         o.ID,
		"order_no":   o.OrderNo,
		"user_id":    o.UserID,
		"amount_fen": o.AmountFen, "status": o.Status,
		"create_time":       o.CreateTime.Format(time.DateTime),
		"create_time_stamp": o.CreateTime.UnixMilli(),
		"items":             o.Items,
	}
	return mp, nil
}
