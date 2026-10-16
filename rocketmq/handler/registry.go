package handler

import (
	"context"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// Handlers 注册应用的 RocketMQ 消费回调。
func Handlers() map[string]func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	return map[string]func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error){
		"user_es_consumer_group":     userESSyncHandler,
		"user_consumer_group_info":   userInfoChangeHandler,
		"user_consumer_group_notify": userNotifyPushHandler,
		"order_es_consumer_group":    orderESSyncHandler,
		"order_consumer_group":       orderBusinessHandler,
		"product_es_consumer_group":  productESSyncHandler,
	}
}
