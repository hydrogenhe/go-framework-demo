package handler

import (
	"context"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/logger"
	"go-framework-demo/rocketmq/message"
)

// 用户业务消费者仅处理 UserMessage，CDC 使用独立消费者。
func userInfoChangeHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		msgCtx := messageContext(ctx, msg)
		if err := handleOneUserBusinessMessage(msgCtx, msg, false); err != nil {
			logConsumeFailure(msgCtx, "USER-BIZ", msg, err)
			return consumer.ConsumeRetryLater, err
		}
	}
	return consumer.ConsumeSuccess, nil
}
func userNotifyPushHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		msgCtx := messageContext(ctx, msg)
		if err := handleOneUserBusinessMessage(msgCtx, msg, true); err != nil {
			logConsumeFailure(msgCtx, "USER-BIZ", msg, err)
			return consumer.ConsumeRetryLater, err
		}
	}
	return consumer.ConsumeSuccess, nil
}
func handleOneUserBusinessMessage(ctx context.Context, msg *primitive.MessageExt, notify bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil {
		return fmt.Errorf("用户业务消息为空")
	}
	event, err := message.ParseUserMessage(string(msg.Body))
	if err != nil {
		return err
	}
	if event.UserID == "" {
		return fmt.Errorf("用户业务消息缺少user_id")
	}
	// 在此扩展业务处理；ES 写入由独立的 CDC 消费者负责。
	label := "USER-BIZ"
	if notify {
		label = "USER-NOTIFY"
	}
	logger.Info(ctx, "用户业务事件", "label", label, "msg_id", msg.MsgId, "action", event.Action, "user_id", event.UserID)
	return nil
}
