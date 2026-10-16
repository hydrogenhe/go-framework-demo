package order_server

import (
	appconst "go-framework-demo/const"
	"go-framework-demo/model"
)

// 更新参数定义在 model，供 Service 与 DB 共用，避免 DB 依赖业务层。
type OrderUpdateInput = model.OrderUpdateInput

// ValidateOrderUpdate 只校验订单更新参数，不负责组装数据库更新字段。
func ValidateOrderUpdate(input *OrderUpdateInput) error {
	if input == nil {
		return appconst.NewInvalidParamsError("订单更新参数不能为空")
	}
	if input.Amount != nil {
		if *input.Amount <= 0 {
			return appconst.NewInvalidParamsError("订单金额必须为正整数分")
		}
	}
	if input.Status != nil {
		if *input.Status < int32(model.OrderStatusPendingPayment) || *input.Status > int32(model.OrderStatusCompleted) {
			return appconst.NewInvalidParamsError("订单状态无效")
		}
	}
	if input.Amount == nil && input.Status == nil {
		return appconst.NewInvalidParamsError("请指定要修改的订单字段")
	}
	return nil
}
