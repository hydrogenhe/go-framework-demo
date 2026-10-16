package handler

import (
	"context"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/logger"
	"go-framework-demo/mysql"
	"go-framework-demo/rocketmq/message"
	"go-framework-demo/server/order_server"
	"strconv"
)

// orderBusinessHandler 消费订单业务事件；ES 副本由独立 CDC 链路维护。
func orderBusinessHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {

	for _, msg := range msgs {
		msgCtx := messageContext(ctx, msg)
		if err := handleOneOrderBusinessMessage(msgCtx, msg); err != nil {
			logConsumeFailure(msgCtx, "ORDER-BIZ", msg, err)
			return consumer.ConsumeRetryLater, err
		}
	}
	return consumer.ConsumeSuccess, nil
}

func handleOneOrderBusinessMessage(ctx context.Context, msg *primitive.MessageExt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil {
		return fmt.Errorf("订单业务消息为空")
	}
	event, err := message.ParseOrderMessage(string(msg.Body))
	if err != nil {
		return err
	}
	if event.OrderID == "" {
		return fmt.Errorf("订单业务消息缺少order_id")
	}
	if event.Action == message.ActionExpire {
		orderID, err := strconv.ParseInt(event.OrderID, 10, 64)
		if err != nil || orderID <= 0 {
			return fmt.Errorf("订单超时消息的order_id无效")
		}
		service := order_server.NewOrderService(ctx, mysql.GetGenQuery())
		_, err = service.CloseIfExpired(ctx, orderID, true)
		return err
	}
	if event.UserID == "" {
		return fmt.Errorf("订单业务消息缺少order_id或user_id")
	}
	if event.Action == message.ActionCreate {
		if event.Amount <= 0 || len(event.Items) == 0 {
			return fmt.Errorf("订单创建消息缺少有效金额或明细")
		}
		for _, item := range event.Items {
			if item.ProductID == "" || item.Quantity <= 0 || item.UnitPriceFen <= 0 {
				return fmt.Errorf("订单创建消息的商品明细无效")
			}
		}
	}
	// 当前示例只记录已校验的业务事件，具体业务动作明确后在此扩展。
	// ES 索引同步属于 CDC 链路，不在订单业务 Handler 中执行。
	logger.Info(ctx, "[ORDER-BIZ] 订单业务事件", "msg_id", msg.MsgId, "action", event.Action, "order_id", event.OrderID, "user_id", event.UserID, "amount", event.Amount, "items", len(event.Items))
	return nil
}
