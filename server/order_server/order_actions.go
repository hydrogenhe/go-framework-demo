package order_server

import (
	"context"
	"errors"

	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/query"
)

func (s *OrderService) changeOrderStatus(ctx context.Context, orderID, userID int64, source, target appconst.OrderStatus) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderID <= 0 || userID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单ID和用户ID必须为正整数")
	}
	var result *model.Order
	err := db.WithTx(ctx, func(ctx context.Context, q *query.Query) error {
		orders, err := db.NewOrderDb(ctx, q)
		if err != nil {
			return err
		}
		order, err := orders.GetByIDForUpdate(ctx, orderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return appconst.NewNotFoundError("订单不存在")
		}
		if order.Status == int32(target) {
			result = order
			return nil
		} // 重复点击幂等，不回退状态。
		if order.Status != int32(source) {
			return appconst.NewConflictError("当前订单状态不允许此操作")
		}
		if source == model.OrderStatusPendingPayment {
			databaseNow, err := orders.DatabaseNow(ctx)
			if err != nil {
				return err
			}
			if !order.ExpireTime.After(databaseNow) {
				return appconst.NewConflictError("订单已超过支付期限，正在关闭")
			}
		}
		status := int32(target)
		input := model.OrderUpdateInput{Status: &status}
		result, err = orders.Update(ctx, orderID, &input)
		return err
	}, s.genQuery)
	if err != nil {
		return nil, orderServiceError(err)
	}
	return result, nil
}

// ShipOrderItem 只改变指定明细的发货状态，不改变父订单的支付/完成状态。
func (s *OrderService) ShipOrderItem(ctx context.Context, orderID, itemID int64) (*model.OrderItem, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderID <= 0 || itemID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单ID和明细ID必须为正整数")
	}
	var result *model.OrderItem
	err := db.WithTx(ctx, func(ctx context.Context, q *query.Query) error {
		orders, err := db.NewOrderDb(ctx, q)
		if err != nil {
			return err
		}
		order, err := orders.GetByIDForUpdate(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status != int32(model.OrderStatusPaid) {
			return appconst.NewConflictError("只有已支付订单可以发货")
		}
		items, err := db.NewOrderItemDb(ctx, q)
		if err != nil {
			return err
		}
		item, err := items.GetByID(ctx, itemID)
		if err != nil {
			return err
		}
		if item.OrderID != orderID {
			return appconst.NewNotFoundError("订单明细不存在")
		}
		if item.Status != int32(model.OrderItemStatusShipped) {
			if err := items.UpdateStatus(ctx, itemID, int32(model.OrderItemStatusShipped)); err != nil {
				return err
			}
			item.Status = int32(model.OrderItemStatusShipped)
		}
		result = item
		return nil
	}, s.genQuery)
	if err != nil {
		return nil, orderServiceError(err)
	}
	return result, nil
}

// MarkShipped 保留整单发货接口，但发货不会自动把订单改为已完成。
func (s *OrderService) MarkShipped(ctx context.Context, orderID int64) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单ID必须为正整数")
	}
	var result *model.Order
	err := db.WithTx(ctx, func(ctx context.Context, q *query.Query) error {
		orders, err := db.NewOrderDb(ctx, q)
		if err != nil {
			return err
		}
		order, err := orders.GetByIDForUpdate(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status != int32(model.OrderStatusPaid) {
			return appconst.NewConflictError("只有已支付订单可以发货")
		}
		items, err := db.NewOrderItemDb(ctx, q)
		if err != nil {
			return err
		}
		if err := items.MarkAllShipped(ctx, orderID); err != nil {
			return err
		}
		result = order
		return nil
	}, s.genQuery)
	if err != nil {
		return nil, orderServiceError(err)
	}
	return result, nil
}
