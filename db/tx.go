package db

import (
	"context"
	"fmt"
	"go-framework-demo/mysql"
	"go-framework-demo/query" // Gen 生成的 query 包
)

// TxContextKey 上下文传递事务 Query 的 key
type TxContextKey struct{}

// WithTx 全局事务封装（复用 Gen 内置 Transaction 方法）
func WithTx(ctx context.Context, fn func(ctx context.Context, txQ *query.Query) error) error {
	// 校验全局 Query
	GlobalQuery := mysql.GetGenQuery()
	if GlobalQuery == nil || !GlobalQuery.Available() {
		return fmt.Errorf("全局 Query 未初始化，请先调用 InitDB")
	}

	// 调用 Gen 内置的 Transaction 方法（适配你的 Gen 代码）
	return GlobalQuery.Transaction(func(txQ *query.Query) error {
		// 将事务 Query 存入上下文
		ctx := context.WithValue(ctx, TxContextKey{}, txQ)
		// 执行业务逻辑
		return fn(ctx, txQ)
	})
}

// GetTxQuery 从上下文获取事务 Query
func GetTxQuery(ctx context.Context, defaultQ *query.Query) *query.Query {
	// 优先级：上下文事务 Query > 传入的 defaultQ > 全局 GlobalQuery
	if txQ, ok := ctx.Value(TxContextKey{}).(*query.Query); ok && txQ != nil {
		return txQ
	}
	if defaultQ != nil && defaultQ.Available() {
		return defaultQ
	}
	return mysql.GetGenQuery()
}
