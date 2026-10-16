package handler

import (
	"context"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/logger"
)

func messageContext(ctx context.Context, msg *primitive.MessageExt) context.Context {
	if msg == nil {
		return ctx
	}
	logID := msg.GetProperty("log_id")
	if logID == "" {
		logID = logger.LogID(ctx)
		if logID == "" {
			logID = logger.NewLogID()
		}
		msg.WithProperty("log_id", logID)
	}
	return logger.WithLogID(ctx, logID)
}

func logConsumeFailure(ctx context.Context, kind string, msg *primitive.MessageExt, err error) {
	var topic, id string
	if msg != nil {
		topic, id = msg.Topic, msg.MsgId
	}
	logger.Error(messageContext(ctx, msg), "消费失败，等待重试", "kind", kind, "topic", topic, "msg_id", id, "error", err)
}
