package db

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/logger"
	"time"

	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OrderDbInterface 订单表DB层接口（全程int64分单位，无float64）
type OrderDbInterface interface {
	FindExpiredPage(ctx context.Context, input *ExpiredOrderQuery) ([]*model.Order, error)
	FindExpiredIDs(ctx context.Context, status int32, limit int) ([]int64, error)
	DatabaseNow(ctx context.Context) (time.Time, error)
	GetByIDForUpdateIncludingDeleted(ctx context.Context, id int64, waitLock bool) (*model.Order, error)
	UpdateExpiredStatus(ctx context.Context, id int64, source, target int32) (int64, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*model.Order, error)
	GetByIDForUpdate(ctx context.Context, id int64) (*model.Order, error)
	GetByIDsForUser(ctx context.Context, userID int64, ids []int64) ([]*model.Order, error)
	Create(ctx context.Context, order *model.Order) (*model.Order, error)
	GetByID(ctx context.Context, id int64) (*model.Order, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*model.Order, error)
	GetByOrderNoForUser(ctx context.Context, userID int64, orderNo string) (*model.Order, error)
	GetByUserID(ctx context.Context, userId int64) ([]*model.Order, error)
	Update(ctx context.Context, id int64, input *model.OrderUpdateInput) (*model.Order, error)
	Delete(ctx context.Context, id int64) (bool, error)
	BatchCreateOrders(ctx context.Context, orders []*model.Order) error
	GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.Order, error)
}

type OrderDb struct {
	GenQuery *query.Query // Gen根查询器（绑定普通/事务DB）
}

var _ OrderDbInterface = (*OrderDb)(nil)

// NewOrderDb 使用显式事务 Query，未传入时使用全局 Query。
func NewOrderDb(ctx context.Context, txQ ...*query.Query) (*OrderDb, error) {
	var genQuery *query.Query
	if len(txQ) > 0 && txQ[0] != nil {
		genQuery = txQ[0]
	}

	// 未传入事务查询器时使用全局 Query。
	if genQuery == nil {
		genQuery = mysql.GetGenQuery()
	}

	if genQuery == nil || !genQuery.Available() {
		return nil, errors.New("Gen查询器实例为nil或未绑定有效DB")
	}

	return &OrderDb{
		GenQuery: genQuery,
	}, nil
}

// --------------------------- 核心业务方法（全程int64分单位） ---------------------------

// GetByID 根据ID查询订单
func (o *OrderDb) GetByID(ctx context.Context, id int64) (*model.Order, error) {
	if id <= 0 {
		return nil, errors.New("查询失败：订单ID必须大于0")
	}

	orderQuery := o.GenQuery.Order
	orderDo := orderQuery.WithContext(ctx)
	order, err := orderDo.Where(orderQuery.ID.Eq(id)).First()
	if err != nil {
		logger.Error(ctx, "查询订单系统错误", "order_id", id, "error", err)
		return nil, fmt.Errorf("查询订单失败[系统错误]：%w（ID=%d）", err, id)
	}

	return order, nil
}

// GetByOrderNo 根据订单号查询订单（适配唯一键）
func (o *OrderDb) GetByOrderNo(ctx context.Context, orderNo string) (*model.Order, error) {
	if orderNo == "" {
		return nil, errors.New("查询失败：订单号不能为空")
	}

	orderQuery := o.GenQuery.Order
	orderDo := orderQuery.WithContext(ctx)
	order, err := orderDo.Where(orderQuery.OrderNo.Eq(orderNo)).First()
	if err != nil {
		logger.Error(ctx, "查询订单系统错误", "order_no", orderNo, "error", err)
		return nil, fmt.Errorf("查询订单失败[系统错误]：%w（订单号=%s）", err, orderNo)
	}

	return order, nil
}

// GetByOrderNoForUser 按当前用户和订单号查询订单，用于下单幂等判断。
func (o *OrderDb) GetByOrderNoForUser(ctx context.Context, userID int64, orderNo string) (*model.Order, error) {
	orderQuery := o.GenQuery.Order
	order, err := orderQuery.WithContext(ctx).
		Where(orderQuery.UserID.Eq(userID), orderQuery.OrderNo.Eq(orderNo)).
		First()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error(ctx, "按用户查询订单系统错误", "user_id", userID, "order_no", orderNo, "error", err)
		}
		return nil, fmt.Errorf("按用户查询订单失败: %w", err)
	}
	return order, nil
}

// GetByUserID 根据用户ID查询订单列表（优化无数据场景）
func (o *OrderDb) GetByUserID(ctx context.Context, userId int64) ([]*model.Order, error) {
	if userId <= 0 {
		return nil, errors.New("查询失败：用户ID必须大于0")
	}

	orderQuery := o.GenQuery.Order
	orderDo := orderQuery.WithContext(ctx)
	orders, err := orderDo.Where(orderQuery.UserID.Eq(userId)).Find()
	if err != nil {
		logger.Error(ctx, "查询用户订单列表系统错误", "user_id", userId, "error", err)
		return nil, fmt.Errorf("查询用户订单列表失败[系统错误]：%w（用户ID=%d）", err, userId)
	}

	// 无数据返回空切片（语义精准，调用方无需判nil）
	if len(orders) == 0 {
		logger.Info(ctx, "用户未查询到订单数据", "user_id", userId)
		return []*model.Order{}, nil
	}

	return orders, nil
}

// Update 更新订单（金额仅接收int64分单位）
func (o *OrderDb) Update(ctx context.Context, id int64, input *model.OrderUpdateInput) (*model.Order, error) {
	if id <= 0 {
		return nil, errors.New("更新失败：订单ID必须大于0")
	}
	columns, err := o.buildUpdateColumns(input)
	if err != nil {
		return nil, err
	}

	// 校验订单是否存在
	if _, err := o.GetByID(ctx, id); err != nil {
		return nil, err
	}

	orderQuery := o.GenQuery.Order
	orderDo := orderQuery.WithContext(ctx)
	// 合并为一条 UPDATE，避免逐字段 UpdateColumn 导致部分成功。
	_, err = orderDo.Where(orderQuery.ID.Eq(id)).UpdateColumnSimple(columns...)
	if err != nil {
		logger.Error(ctx, "更新订单系统错误", "order_id", id, "error", err)
		return nil, fmt.Errorf("更新订单失败[系统错误]：%w（ID=%d）", err, id)
	}

	// 返回更新后的订单
	return o.GetByID(ctx, id)
}

// buildUpdateColumns 集中组装订单更新列，保留非 nil 字段的明确赋值。
func (o *OrderDb) buildUpdateColumns(input *model.OrderUpdateInput) ([]field.AssignExpr, error) {
	if input == nil {
		return nil, errors.New("订单更新参数不能为空")
	}
	orderQuery := o.GenQuery.Order
	columns := make([]field.AssignExpr, 0, 2)
	if input.Amount != nil {
		columns = append(columns, orderQuery.Amount.Value(*input.Amount))
	}
	if input.Status != nil {
		columns = append(columns, orderQuery.Status.Value(*input.Status))
	}
	if len(columns) == 0 {
		return nil, errors.New("更新失败：请指定要修改的字段")
	}
	return columns, nil
}

// Delete 删除订单
func (o *OrderDb) Delete(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, errors.New("删除失败：订单ID必须大于0")
	}

	// 校验订单是否存在
	if _, err := o.GetByID(ctx, id); err != nil {
		return false, err
	}

	orderQuery := o.GenQuery.Order
	orderDo := orderQuery.WithContext(ctx)
	result, err := orderDo.Where(orderQuery.ID.Eq(id)).Delete()
	if err != nil {
		logger.Error(ctx, "删除订单系统错误", "order_id", id, "error", err)
		return false, fmt.Errorf("删除订单失败[系统错误]：%w（ID=%d）", err, id)
	}

	return result.RowsAffected > 0, nil
}

// Create 保存由业务层组装好的订单模型。
func (o *OrderDb) Create(ctx context.Context, order *model.Order) (*model.Order, error) {
	orderQuery := o.GenQuery.Order
	if err := orderQuery.WithContext(ctx).Create(order); err != nil {
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}
	return order, nil
}

// BatchCreateOrders 仅在调用方传入的事务中批量写入订单。
func (o *OrderDb) BatchCreateOrders(ctx context.Context, orders []*model.Order) error {
	orderQuery := o.GenQuery.Order
	return orderQuery.WithContext(ctx).Create(orders...)
}

// GetByIDIncludingDeleted 仅供 CDC 读取最新状态，包含软删除记录。
func (o *OrderDb) GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.Order, error) {
	q := o.GenQuery.Order
	return q.WithContext(ctx).Unscoped().Where(q.ID.Eq(id)).First()
}

// GetByIDsForUser 批量回查当前用户的有效订单，防止 ES 旧数据越权。
func (o *OrderDb) GetByIDsForUser(ctx context.Context, userID int64, ids []int64) ([]*model.Order, error) {
	if len(ids) == 0 {
		return []*model.Order{}, nil
	}
	orderQuery := o.GenQuery.Order
	return orderQuery.WithContext(ctx).Where(orderQuery.ID.In(ids...), orderQuery.UserID.Eq(userID)).Find()
}

func (o *OrderDb) GetByIDs(ctx context.Context, ids []int64) ([]*model.Order, error) {
	if len(ids) == 0 {
		return []*model.Order{}, nil
	}
	orderQuery := o.GenQuery.Order
	return orderQuery.WithContext(ctx).Where(orderQuery.ID.In(ids...)).Find()
}

// GetByIDForUpdate 仅在事务内调用，锁住父订单以串行化支付、收货和发货。
func (o *OrderDb) GetByIDForUpdate(ctx context.Context, id int64) (*model.Order, error) {
	orderQuery := o.GenQuery.Order
	return orderQuery.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(orderQuery.ID.Eq(id)).First()
}
