package es_model

import (
	"errors"
	"time"
)

// Product 是 product_test 索引中的商品文档。
type Product struct {
	IsDel           int64     `json:"is_del"`
	CreateTime      time.Time `json:"-"` // 仅用于构建展示字段
	CreateTimeStamp int64     `json:"create_time_stamp"`
	UpdateTime      time.Time `json:"-"`
	UpdateTimeStamp int64     `json:"update_time_stamp"`
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Price           int64     `json:"price"`
	Stock           int32     `json:"stock"`
	Image           string    `json:"image"`
}

func (p *Product) ToMap() (map[string]interface{}, error) {
	if p == nil || p.ID <= 0 || p.Name == "" || p.Price <= 0 || p.Stock < 0 || p.CreateTime.IsZero() {
		return nil, errors.New("商品字段无效")
	}
	updateTime := p.UpdateTime
	if updateTime.IsZero() {
		updateTime = p.CreateTime
	}
	return map[string]interface{}{
		"is_del": p.IsDel, "id": p.ID, "name": p.Name, "price": p.Price, "stock": p.Stock, "image": p.Image,
		"create_time": p.CreateTime, "create_time_stamp": p.CreateTime.UnixMilli(),
		"update_time": updateTime, "update_time_stamp": updateTime.UnixMilli()}, nil
}
