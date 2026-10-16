package db

import (
	"context"
	"errors"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"gorm.io/gorm"
)

// WithTx 绑定请求上下文并开启事务，回调返回错误时回滚，成功时提交。
// 默认使用全局 Query；传入 baseQ 时沿用该 Query，支持外层事务中的调用。
// 事务 Query 通过回调参数显式传递，不存入上下文。
func WithTx(ctx context.Context, fn func(context.Context, *query.Query) error, baseQ ...*query.Query) error {
	q := mysql.GetGenQuery()
	if len(baseQ) > 0 && baseQ[0] != nil {
		q = baseQ[0]
	}
	if q == nil || !q.Available() {
		return errors.New("事务 Query 未初始化")
	}
	if fn == nil {
		return errors.New("事务回调不能为空")
	}
	return q.UnderlyingDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, query.Use(tx))
	})
}
