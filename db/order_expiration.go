package db

import (
	"context"
	"errors"
	"time"

	"go-framework-demo/logger"
	"go-framework-demo/model"
	"gorm.io/gen/field"
	"gorm.io/gorm/clause"
)

// FindExpiredIDs 使用数据库时间及 (status, expire_time) 索引查找一批订单。
func (o *OrderDb) FindExpiredIDs(ctx context.Context, status int32, limit int) ([]int64, error) {
	orderQuery := o.GenQuery.Order
	var ids []int64
	err := orderQuery.WithContext(ctx).Where(orderQuery.Status.Eq(status), field.NewUnsafeFieldRaw("expire_time <= CURRENT_TIMESTAMP(3)")).Order(orderQuery.ExpireTime).Limit(limit).Pluck(orderQuery.ID, &ids)
	return ids, err
}

// ExpiredOrderQuery 使用固定截止时间及到期时间、ID 联合游标读取到期订单。
type ExpiredOrderQuery struct {
	Status          int32
	Cutoff          time.Time
	AfterExpireTime time.Time
	AfterID         int64
	Limit           int
}

// FindExpiredPage 只读取调度需要的 ID 和截止时间，不读取完整订单。
func (o *OrderDb) FindExpiredPage(ctx context.Context, input *ExpiredOrderQuery) ([]*model.Order, error) {
	if input == nil || input.Limit <= 0 || input.Cutoff.IsZero() {
		return nil, errors.New("到期订单分页参数无效")
	}
	orderQuery := o.GenQuery.Order
	rows := orderQuery.WithContext(ctx).Select(orderQuery.ID, orderQuery.ExpireTime).
		Where(orderQuery.Status.Eq(input.Status), orderQuery.ExpireTime.Lte(input.Cutoff))
	if !input.AfterExpireTime.IsZero() {
		rows = rows.Where(orderQuery.WithContext(ctx).
			Where(orderQuery.ExpireTime.Gt(input.AfterExpireTime)).
			Or(orderQuery.WithContext(ctx).Where(orderQuery.ExpireTime.Eq(input.AfterExpireTime), orderQuery.ID.Gt(input.AfterID))))
	}
	return rows.Order(orderQuery.ExpireTime, orderQuery.ID).Limit(input.Limit).Find()
}

// DatabaseNow 获取当前事务连接上的数据库时间。
func (o *OrderDb) DatabaseNow(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := o.GenQuery.UnderlyingDB().WithContext(ctx).Raw("SELECT CURRENT_TIMESTAMP(3)").Scan(&now).Error
	return now, err
}

// GetByIDForUpdateIncludingDeleted 锁定订单（含软删除记录）；waitLock=false 时使用 NOWAIT，true 时等待行锁。
func (o *OrderDb) GetByIDForUpdateIncludingDeleted(ctx context.Context, id int64, waitLock bool) (*model.Order, error) {
	orderQuery := o.GenQuery.Order
	locking := clause.Locking{Strength: "UPDATE"}
	if !waitLock {
		locking.Options = "NOWAIT"
	}
	return orderQuery.WithContext(ctx).Unscoped().Clauses(locking).Where(orderQuery.ID.Eq(id)).First()
}

// UpdateExpiredStatus 按当前状态和数据库截止时间执行条件更新。
func (o *OrderDb) UpdateExpiredStatus(ctx context.Context, id int64, source, target int32) (int64, error) {
	orderQuery := o.GenQuery.Order
	result, err := orderQuery.WithContext(ctx).Unscoped().Where(orderQuery.ID.Eq(id), orderQuery.Status.Eq(source), field.NewUnsafeFieldRaw("expire_time <= CURRENT_TIMESTAMP(3)")).UpdateColumn(orderQuery.Status, target)
	if err != nil {
		logger.Error(ctx, "[UpdateExpiredStatus] 更新订单状态失败", "table", "order", "order_id", id, "error", err)
	}
	return result.RowsAffected, err
}
