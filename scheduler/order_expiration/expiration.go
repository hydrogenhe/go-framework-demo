// Package order_expiration 调度到期订单，关单业务统一交给 order_server.OrderService。
package order_expiration

import (
	"context"
	"errors"
	"sync"
	"time"

	"go-framework-demo/db"
	"go-framework-demo/logger"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"

	"github.com/panjf2000/ants/v2"
	"gorm.io/gorm"
)

const (
	// PaymentWindow 订单默认支付窗口，沿用模型定义的时长。
	PaymentWindow = model.OrderPaymentWindow
	// ScanInterval 超时订单扫描的默认轮次间隔，3秒。
	ScanInterval  = 3 * time.Second
	// ScanBatchSize 超时订单扫描每批最多读取的订单数。
	ScanBatchSize = 50
	// WorkerLimit 超时关单协程池允许的最大并发任务数。
	WorkerLimit   = 200
)

// scanSource 只提供调度所需的时间和分页查询，不承担关单业务。
type scanSource interface {
	DatabaseNow(context.Context) (time.Time, error)
	FindExpiredPage(context.Context, *db.ExpiredOrderQuery) ([]*model.Order, error)
}

type orderCloser interface {
	CloseIfExpired(context.Context, int64, bool) (bool, error)
}

// RunScheduler 立即扫描；一轮内持续分页，满载时等待空位，一轮结束后等待 interval。
func RunScheduler(ctx context.Context, database *gorm.DB, interval time.Duration) {
	if database == nil {
		logger.Error(ctx, "订单超时任务需要 MySQL 连接")
		return
	}
	if interval <= 0 {
		interval = ScanInterval
	}
	genQuery := query.Use(database)
	orders, err := db.NewOrderDb(ctx, genQuery)
	if err != nil {
		logger.Error(ctx, "初始化订单超时查询失败", "error", err)
		return
	}
	service := order_server.NewOrderService(ctx, genQuery)
	// 非阻塞提交配合可取消的容量等待，不让 Submit 卡住退出流程。
	pool, err := ants.NewPool(WorkerLimit, ants.WithNonblocking(true))
	if err != nil {
		logger.Error(ctx, "初始化订单超时协程池失败", "error", err)
		return
	}
	defer pool.Release()
	slots := make(chan struct{}, WorkerLimit)
	var tasks sync.WaitGroup
	defer tasks.Wait()
	for ctx.Err() == nil {
		if err := runScanRound(ctx, orders, service, pool, slots, &tasks); err != nil && ctx.Err() == nil {
			logger.Error(ctx, "扫描超时订单失败", "error", err)
		}
		if err := waitInterval(ctx, interval); err != nil {
			return
		}
	}
}

// runScanRound 使用数据库时间固定本轮边界；失败、被锁订单不阻塞后续游标。
func runScanRound(ctx context.Context, source scanSource, closer orderCloser, pool *ants.Pool, slots chan struct{}, tasks *sync.WaitGroup) error {
	queryCtx, cancel := context.WithTimeout(ctx, order_server.CloseTimeout)
	cutoff, err := source.DatabaseNow(queryCtx)
	cancel()
	if err != nil {
		return err
	}
	input := db.ExpiredOrderQuery{
		Status: int32(model.OrderStatusPendingPayment),
		Cutoff: cutoff,
	}
	for {
		reserved, err := reserveSlots(ctx, slots)
		if err != nil {
			return err
		}
		input.Limit = reserved
		queryCtx, cancel := context.WithTimeout(ctx, order_server.CloseTimeout)
		rows, err := source.FindExpiredPage(queryCtx, &input)
		cancel()
		if err != nil {
			releaseSlots(slots, reserved)
			return err
		}
		releaseSlots(slots, reserved-len(rows))
		for index, row := range rows {
			tasks.Add(1)
			task := newCloseTask(ctx, closer, row.ID, slots, tasks)
			if err := submitTask(ctx, pool, task); err != nil {
				tasks.Done()
				releaseSlots(slots, len(rows)-index)
				return err
			}
		}
		if len(rows) < input.Limit {
			return nil
		}
		last := rows[len(rows)-1]
		input.AfterExpireTime = last.ExpireTime
		input.AfterID = last.ID
	}
}

// reserveSlots 在查库前领取容量，满载时可取消地等待；每批最多50条。
func reserveSlots(ctx context.Context, slots chan struct{}) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case slots <- struct{}{}:
	}
	reserved := 1
	for reserved < ScanBatchSize {
		select {
		case slots <- struct{}{}:
			reserved++
		default:
			return reserved, nil
		}
	}
	return reserved, nil
}

func releaseSlots(slots chan struct{}, count int) {
	for i := 0; i < count; i++ {
		<-slots
	}
}

func newCloseTask(ctx context.Context, closer orderCloser, orderID int64, slots chan struct{}, tasks *sync.WaitGroup) func() {
	return func() {
		defer tasks.Done()
		defer releaseSlots(slots, 1)
		defer recoverCloseTask(ctx, orderID)
		taskCtx, cancel := context.WithTimeout(ctx, order_server.CloseTimeout)
		defer cancel()
		closed, err := closer.CloseIfExpired(taskCtx, orderID, false)
		if err != nil {
			logger.Error(taskCtx, "关闭超时订单失败", "order_id", orderID, "error", err)
			return
		}
		if closed {
			logger.Info(taskCtx, "超时订单已关闭并释放库存", "order_id", orderID)
		}
	}
}

func recoverCloseTask(ctx context.Context, orderID int64) {
	if value := recover(); value != nil {
		logger.Error(ctx, "关闭超时订单发生panic", "order_id", orderID, "panic", value)
	}
}

// submitTask 处理任务刚释放容量但 ants 尚未回收 Worker 的短暂窗口。
func submitTask(ctx context.Context, pool *ants.Pool, task func()) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := pool.Submit(task)
		if !errors.Is(err, ants.ErrPoolOverload) {
			return err
		}
		if err := waitInterval(ctx, time.Millisecond); err != nil {
			return err
		}
	}
}

func waitInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ProcessExpiredBatch 保留批次入口，将业务处理委托给订单超时服务。
func ProcessExpiredBatch(ctx context.Context, database *gorm.DB, limit int) error {
	if database == nil {
		return errors.New("订单超时任务需要 MySQL 连接")
	}
	if limit <= 0 {
		limit = ScanBatchSize
	}
	service := order_server.NewOrderService(ctx, query.Use(database))
	return service.ProcessExpiredBatch(ctx, limit)
}

// CloseIfExpired 兼容现有调用方，与 MQ 消费者统一调用相同的关单服务。
func CloseIfExpired(ctx context.Context, database *gorm.DB, orderID int64) (bool, error) {
	if database == nil {
		return false, errors.New("订单超时任务需要 MySQL 连接")
	}
	service := order_server.NewOrderService(ctx, query.Use(database))
	return service.CloseIfExpired(ctx, orderID, false)
}
