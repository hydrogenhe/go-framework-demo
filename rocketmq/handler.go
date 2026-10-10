package rocketmq

import (
	"context"
	"log"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// 初始化注册回调函数
func init() {
	RegisterConsumeHandler("user_info_change", userInfoChangeHandler)
	RegisterConsumeHandler("user_notify_push", userNotifyPushHandler)
	RegisterConsumeHandler("order", orderMsgHandler)
	RegisterConsumeHandler("product", productMsgHandler)
}

// 用户信息变动处理
func userInfoChangeHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		log.Printf("[USER-INFO] 处理用户信息变动: msgId=%s, body=%s", msg.MsgId, string(msg.Body))
	}
	return consumer.ConsumeSuccess, nil
}

// 用户通知推送
func userNotifyPushHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		log.Printf("[USER-NOTIFY] 处理用户通知推送: msgId=%s, body=%s", msg.MsgId, string(msg.Body))
	}
	return consumer.ConsumeSuccess, nil
}

// 订单处理
func orderMsgHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		log.Printf("[ORDER] 处理订单消息: msgId=%s, body=%s", msg.MsgId, string(msg.Body))
	}
	return consumer.ConsumeSuccess, nil
}

// 商品处理
func productMsgHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		log.Printf("[PRODUCT] 处理商品消息: msgId=%s, body=%s", msg.MsgId, string(msg.Body))
	}
	return consumer.ConsumeSuccess, nil
}
