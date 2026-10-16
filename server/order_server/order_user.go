package order_server

import (
	"context"
	"errors"

	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"gorm.io/gorm"
)

// OrderUserDetail 仅包含管理员查看订单用户所需的资料。
type OrderUserDetail struct {
	ID    int64
	Name  string
	Email string
}

// GetAdminOrderUser 供管理员订单入口使用，始终读取数据库最新资料。
func (s *OrderService) GetAdminOrderUser(ctx context.Context, orderID int64) (*OrderUserDetail, error) {
	if orderID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单ID必须为正整数")
	}
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	order, err := s.orderDb.GetByID(ctx, orderID)
	if err != nil {
		return nil, orderServiceError(err)
	}
	users, err := db.NewUserDb(ctx, s.genQuery)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	user, err := users.GetByID(ctx, order.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appconst.NewNotFoundError("用户不存在或已删除")
	}
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	detail := &OrderUserDetail{ID: user.ID, Name: user.Name, Email: user.Email}
	return detail, nil
}
