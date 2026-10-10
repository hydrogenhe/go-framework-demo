package db

import (
	"context"
	"errors"
	"fmt"

	"go-framework-demo/model"
	"go-framework-demo/query"
)

// OrderItemDbInterface 订单明细数据访问接口。
type OrderItemDbInterface interface {
	Create(item *model.OrderItem) (*model.OrderItem, error)
	GetByID(id int64) (*model.OrderItem, error)
	GetByOrderID(orderID int64) ([]*model.OrderItem, error)
	GetByProductID(productID int64) ([]*model.OrderItem, error)
	Delete(id int64) (bool, error)
}

// OrderItemDb 订单明细数据访问层。
type OrderItemDb struct{ GenQuery *query.Query }

func NewOrderItemDb(ctx context.Context) (*OrderItemDb, error) {
	genQuery := GetTxQuery(ctx, nil)
	if genQuery == nil || !genQuery.Available() {
		return nil, errors.New("Gen查询器实例无效")
	}
	return &OrderItemDb{GenQuery: genQuery}, nil
}

func NewTxOrderItemDb(txQ *query.Query) (*OrderItemDb, error) {
	if txQ == nil || !txQ.Available() {
		return nil, errors.New("事务版Gen查询器无效")
	}
	return &OrderItemDb{GenQuery: txQ}, nil
}

// Create 创建一条订单明细。
func (d *OrderItemDb) Create(item *model.OrderItem) (*model.OrderItem, error) {
	if d == nil || d.GenQuery == nil || !d.GenQuery.Available() {
		return nil, errors.New("OrderItemDb未初始化")
	}
	if item == nil || item.OrderID <= 0 || item.ProductID <= 0 || item.Quantity <= 0 || item.UnitPriceFen <= 0 {
		return nil, errors.New("订单明细参数无效")
	}
	if err := d.GenQuery.OrderItem.WithContext(context.Background()).Create(item); err != nil {
		return nil, fmt.Errorf("创建订单明细失败：%w", err)
	}
	return item, nil
}

func (d *OrderItemDb) GetByID(id int64) (*model.OrderItem, error) {
	if id <= 0 {
		return nil, errors.New("查询失败：订单明细ID必须大于0")
	}
	item, err := d.GenQuery.OrderItem.WithContext(context.Background()).Where(d.GenQuery.OrderItem.ID.Eq(id)).First()
	if err != nil {
		return nil, fmt.Errorf("查询订单明细失败：%w", err)
	}
	return item, nil
}

func (d *OrderItemDb) GetByOrderID(orderID int64) ([]*model.OrderItem, error) {
	if orderID <= 0 {
		return nil, errors.New("查询失败：订单ID必须大于0")
	}
	items, err := d.GenQuery.OrderItem.WithContext(context.Background()).Where(d.GenQuery.OrderItem.OrderID.Eq(orderID)).Find()
	if err != nil {
		return nil, fmt.Errorf("查询订单明细失败：%w", err)
	}
	return items, nil
}

func (d *OrderItemDb) GetByProductID(productID int64) ([]*model.OrderItem, error) {
	if productID <= 0 {
		return nil, errors.New("查询失败：商品ID必须大于0")
	}
	items, err := d.GenQuery.OrderItem.WithContext(context.Background()).Where(d.GenQuery.OrderItem.ProductID.Eq(productID)).Find()
	if err != nil {
		return nil, fmt.Errorf("查询商品订单明细失败：%w", err)
	}
	return items, nil
}

func (d *OrderItemDb) Delete(id int64) (bool, error) {
	item, err := d.GetByID(id)
	if err != nil {
		return false, err
	}
	if _, err = d.GenQuery.OrderItem.WithContext(context.Background()).Delete(item); err != nil {
		return false, fmt.Errorf("删除订单明细失败：%w", err)
	}
	return true, nil
}
