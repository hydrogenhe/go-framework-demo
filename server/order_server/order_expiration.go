package order_server

import (
	"context"
	"encoding/json"
	"errors"
	mysqlerr "github.com/go-sql-driver/mysql"
	"go-framework-demo/model"
	"go-framework-demo/rocketmq"
	"go-framework-demo/rocketmq/message"
	"sort"
	"strconv"
	"time"

	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/logger"
	"go-framework-demo/query"
)

// CloseTimeout 限制一次关单尝试，MQ 与扫描共用。
const CloseTimeout = 15 * time.Second

func (s *OrderService) expirationOrders(ctx context.Context) (*db.OrderDb, error) {
	if s == nil || s.genQuery == nil || !s.genQuery.Available() {
		return nil, appconst.NewInternalError(errors.New("订单服务未初始化"))
	}
	return s.orderDb, nil
}

// ProcessExpiredBatch 查找一批到期订单，单条失败不阻塞其余订单。
func (s *OrderService) ProcessExpiredBatch(ctx context.Context, limit int) error {
	orders, err := s.expirationOrders(ctx)
	if err != nil {
		return err
	}
	if limit <= 0 {
		return appconst.NewInvalidParamsError("扫描批量必须为正整数")
	}
	ids, err := orders.FindExpiredIDs(ctx, int32(appconst.OrderStatusPendingPayment), limit)
	if err != nil {
		return appconst.NewInternalError(err)
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		closed, err := s.CloseIfExpired(ctx, id, false)
		if err != nil {
			logger.Error(ctx, "关闭超时订单失败", "order_id", id, "error", err)
			continue
		}
		if closed {
			logger.Info(ctx, "超时订单已关闭并释放库存", "order_id", id)
		}
	}
	return nil
}

// CloseIfExpired 在锁单事务中幂等关单；waitLock=true 等待行锁，false 时锁冲突跳过。
func (s *OrderService) CloseIfExpired(ctx context.Context, orderID int64, waitLock bool) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, CloseTimeout)
	defer cancel()
	if _, err := s.expirationOrders(ctx); err != nil {
		return false, err
	}
	if orderID <= 0 {
		return false, appconst.NewInvalidParamsError("订单ID必须为正整数")
	}
	closed := false
	err := db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		orders, err := db.NewOrderDb(ctx, txQ)
		if err != nil {
			return err
		}
		order, err := orders.GetByIDForUpdateIncludingDeleted(ctx, orderID, waitLock)
		if err != nil {
			var lockErr *mysqlerr.MySQLError
			if !waitLock && errors.As(err, &lockErr) && lockErr.Number == 3572 {
				return nil // NOWAIT 锁冲突：本次跳过，留给后续扫描。
			}
			return err
		}
		if order.Status != int32(appconst.OrderStatusPendingPayment) {
			return nil
		}
		now, err := orders.DatabaseNow(ctx)
		if err != nil {
			return err
		}
		if order.ExpireTime.After(now) {
			return nil
		}
		itemsDB, err := db.NewOrderItemDb(ctx, txQ)
		if err != nil {
			return err
		}
		items, err := itemsDB.GetByOrderIDIncludingDeleted(ctx, orderID)
		if err != nil {
			return err
		}
		products, err := db.NewProductDb(ctx, txQ)
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ProductID < items[j].ProductID })
		for _, item := range items {
			if item.Quantity <= 0 {
				return appconst.NewConflictError("订单商品数量无效，无法返还库存")
			}
			affected, err := products.RestoreStockIncludingDeleted(ctx, item.ProductID, item.Quantity)
			if err != nil {
				return err
			}
			if affected != 1 {
				return appconst.NewConflictError("订单商品不存在，无法返还库存")
			}
		}
		affected, err := orders.UpdateExpiredStatus(ctx, orderID, int32(appconst.OrderStatusPendingPayment), int32(appconst.OrderStatusCancelled))
		if err != nil {
			return err
		}
		if affected != 1 {
			return appconst.NewConflictError("订单超时关闭状态竞争失败")
		}
		closed = true
		return nil
	}, s.genQuery)
	if err != nil {
		return false, appconst.NewInternalError(err)
	}
	return closed, nil
}

// scheduleOrderExpiration 在事务提交后按订单实际支付截止时间安排 MQ 检查，失败由扫描兜底。
func (s *OrderService) scheduleOrderExpiration(ctx context.Context, order *model.Order) {
	if order == nil {
		return
	}
	level, err := message.ExpirationDelayLevel(time.Until(order.ExpireTime))
	if err != nil {
		logger.Error(ctx, "选择订单超时延迟级别失败，等待扫描任务兜底", "order_id", order.ID, "error", err)
		return
	}
	event, err := message.NewOrderExpirationMessage(strconv.FormatInt(order.ID, 10))
	if err != nil {
		logger.Error(ctx, "构造订单超时消息失败", "order_id", order.ID, "error", err)
		return
	}
	body, err := json.Marshal(event)
	if err != nil {
		logger.Error(ctx, "编码订单超时消息失败", "order_id", order.ID, "error", err)
		return
	}
	if _, err := rocketmq.OrderProducerSendDelayMsg(ctx, body, level); err != nil {
		logger.Error(ctx, "发送订单超时延迟消息失败，等待扫描任务兜底", "order_id", order.ID, "error", err)
	}
}
