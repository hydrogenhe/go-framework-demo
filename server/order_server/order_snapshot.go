package order_server

import (
	"context"
	"errors"

	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/model"
)

// GetItemSnapshot 查询当前用户订单中持久化的商品快照。
func (s *OrderService) GetItemSnapshot(ctx context.Context, orderID, itemID, userID int64) (*model.OrderItem, error) {
	if userID <= 0 {
		return nil, appconst.NewInvalidParamsError("用户ID必须为正整数")
	}
	return s.getItemSnapshot(ctx, orderID, itemID, userID, false)
}

// GetAdminItemSnapshot 仅供已通过管理员权限校验的入口使用。
func (s *OrderService) GetAdminItemSnapshot(ctx context.Context, orderID, itemID int64) (*model.OrderItem, error) {
	return s.getItemSnapshot(ctx, orderID, itemID, 0, true)
}

func (s *OrderService) getItemSnapshot(ctx context.Context, orderID, itemID, userID int64, admin bool) (*model.OrderItem, error) {
	if orderID <= 0 || itemID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单和明细ID必须为正整数")
	}
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	order, err := s.orderDb.GetByID(ctx, orderID)
	if err != nil {
		return nil, orderServiceError(err)
	}
	if !admin && order.UserID != userID {
		return nil, appconst.NewNotFoundError("订单不存在")
	}
	items, err := db.NewOrderItemDb(ctx, s.genQuery)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	item, err := items.GetByID(ctx, itemID)
	if err != nil {
		return nil, orderServiceError(err)
	}
	if item.OrderID != orderID {
		return nil, appconst.NewNotFoundError("订单明细不存在")
	}
	// 名称、图片、成交单价和版本号均来自订单明细，不读取当前商品或缓存。
	return item, nil
}
