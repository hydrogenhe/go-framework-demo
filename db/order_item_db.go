package db

import (
	"context"
	"errors"
	"fmt"

	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
)

// OrderItemDbInterface 订单明细数据访问接口。
type OrderItemDbInterface interface {
	MarkAllShipped(ctx context.Context, orderID int64) error
	GetByOrderIDs(ctx context.Context, ids []int64) ([]*model.OrderItem, error)
	Create(ctx context.Context, item *model.OrderItem) (*model.OrderItem, error)
	BatchCreate(ctx context.Context, items []*model.OrderItem) error
	GetByID(ctx context.Context, id int64) (*model.OrderItem, error)
	GetByOrderID(ctx context.Context, orderID int64) ([]*model.OrderItem, error)
	GetByProductID(ctx context.Context, productID int64) ([]*model.OrderItem, error)
	Delete(ctx context.Context, id int64) (bool, error)
	UpdateStatus(ctx context.Context, id int64, status int32) error
	GetByOrderIDIncludingDeleted(ctx context.Context, id int64) ([]*model.OrderItem, error)
	GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.OrderItem, error)
}

// OrderItemDb 订单明细数据访问层。
type OrderItemDb struct{ GenQuery *query.Query }

var _ OrderItemDbInterface = (*OrderItemDb)(nil)

func NewOrderItemDb(ctx context.Context, txQ ...*query.Query) (*OrderItemDb, error) {
	var genQuery *query.Query
	if len(txQ) > 0 && txQ[0] != nil {
		genQuery = txQ[0]
	} else {
		genQuery = mysql.GetGenQuery()
	}
	if genQuery == nil || !genQuery.Available() {
		return nil, errors.New("Gen查询器实例无效")
	}
	return &OrderItemDb{GenQuery: genQuery}, nil
}

// Create 创建一条订单明细。
func (d *OrderItemDb) Create(ctx context.Context, item *model.OrderItem) (*model.OrderItem, error) {
	if d == nil || d.GenQuery == nil || !d.GenQuery.Available() {
		return nil, errors.New("OrderItemDb未初始化")
	}
	orderItemQuery := d.GenQuery.OrderItem
	if err := orderItemQuery.WithContext(ctx).Create(item); err != nil {
		return nil, fmt.Errorf("创建订单明细失败：%w", err)
	}
	return item, nil
}

// BatchCreate 在调用方事务中批量写入订单明细。
func (d *OrderItemDb) BatchCreate(ctx context.Context, items []*model.OrderItem) error {
	if d == nil || d.GenQuery == nil || !d.GenQuery.Available() {
		return errors.New("OrderItemDb未初始化")
	}
	if len(items) == 0 {
		return nil
	}
	orderItemQuery := d.GenQuery.OrderItem
	if err := orderItemQuery.WithContext(ctx).CreateInBatches(items, len(items)); err != nil {
		return fmt.Errorf("批量创建订单明细失败：%w", err)
	}
	return nil
}

func (d *OrderItemDb) GetByID(ctx context.Context, id int64) (*model.OrderItem, error) {
	if id <= 0 {
		return nil, errors.New("查询失败：订单明细ID必须大于0")
	}
	orderItemQuery := d.GenQuery.OrderItem
	item, err := orderItemQuery.WithContext(ctx).Where(orderItemQuery.ID.Eq(id)).First()
	if err != nil {
		return nil, fmt.Errorf("查询订单明细失败：%w", err)
	}
	return item, nil
}

func (d *OrderItemDb) GetByOrderID(ctx context.Context, orderID int64) ([]*model.OrderItem, error) {
	if orderID <= 0 {
		return nil, errors.New("查询失败：订单ID必须大于0")
	}
	orderItemQuery := d.GenQuery.OrderItem
	items, err := orderItemQuery.WithContext(ctx).Where(orderItemQuery.OrderID.Eq(orderID)).Find()
	if err != nil {
		return nil, fmt.Errorf("查询订单明细失败：%w", err)
	}
	return items, nil
}

func (d *OrderItemDb) GetByProductID(ctx context.Context, productID int64) ([]*model.OrderItem, error) {
	if productID <= 0 {
		return nil, errors.New("查询失败：商品ID必须大于0")
	}
	orderItemQuery := d.GenQuery.OrderItem
	items, err := orderItemQuery.WithContext(ctx).Where(orderItemQuery.ProductID.Eq(productID)).Find()
	if err != nil {
		return nil, fmt.Errorf("查询商品订单明细失败：%w", err)
	}
	return items, nil
}

func (d *OrderItemDb) Delete(ctx context.Context, id int64) (bool, error) {
	item, err := d.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	orderItemQuery := d.GenQuery.OrderItem
	if _, err = orderItemQuery.WithContext(ctx).Delete(item); err != nil {
		return false, fmt.Errorf("删除订单明细失败：%w", err)
	}
	return true, nil
}

func (d *OrderItemDb) UpdateStatus(ctx context.Context, id int64, status int32) error {
	if id <= 0 || (status != int32(model.OrderItemStatusPendingShipment) && status != int32(model.OrderItemStatusShipped)) {
		return errors.New("订单明细状态无效")
	}
	orderItemQuery := d.GenQuery.OrderItem
	r, err := orderItemQuery.WithContext(ctx).Where(orderItemQuery.ID.Eq(id)).Update(orderItemQuery.Status, status)
	if err != nil {
		return err
	}
	if r.RowsAffected == 0 {
		_, err := d.GetByID(ctx, id)
		return err
	}
	return nil
}

// GetByOrderIDIncludingDeleted 仅供 CDC 重建完整订单副本。
func (d *OrderItemDb) GetByOrderIDIncludingDeleted(ctx context.Context, id int64) ([]*model.OrderItem, error) {
	q := d.GenQuery.OrderItem
	return q.WithContext(ctx).Unscoped().Where(q.OrderID.Eq(id)).Find()
}

// GetByIDIncludingDeleted 显式读取订单明细，包括已软删除记录。
func (d *OrderItemDb) GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.OrderItem, error) {
	q := d.GenQuery.OrderItem
	return q.WithContext(ctx).Unscoped().Where(q.ID.Eq(id)).First()
}

func (d *OrderItemDb) GetByOrderIDs(ctx context.Context, ids []int64) ([]*model.OrderItem, error) {
	if len(ids) == 0 {
		return []*model.OrderItem{}, nil
	}
	itemQuery := d.GenQuery.OrderItem
	return itemQuery.WithContext(ctx).Where(itemQuery.OrderID.In(ids...)).Order(itemQuery.ID).Find()
}

func (d *OrderItemDb) MarkAllShipped(ctx context.Context, orderID int64) error {
	itemQuery := d.GenQuery.OrderItem
	_, err := itemQuery.WithContext(ctx).Where(itemQuery.OrderID.Eq(orderID)).Update(itemQuery.Status, int32(model.OrderItemStatusShipped))
	return err
}
