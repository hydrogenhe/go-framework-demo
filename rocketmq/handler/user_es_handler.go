package handler

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/db"
	"go-framework-demo/logger"
	"gorm.io/gorm"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
)

func userESSyncHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		msgCtx := messageContext(ctx, msg)
		if err := handleOneUserCDCMessage(msgCtx, msg); err != nil {
			logConsumeFailure(msgCtx, "USER-ES", msg, err)
			return consumer.ConsumeRetryLater, err
		}
	}
	return consumer.ConsumeSuccess, nil
}

func handleOneUserCDCMessage(ctx context.Context, msg *primitive.MessageExt) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return handleUserCDC(ctx, msg)
}

func handleUserCDC(ctx context.Context, msg *primitive.MessageExt) error {
	event, err := parseCanalEvent(msg, "user")
	if err != nil {
		return err
	}
	if event.IsDDL {
		return nil
	}
	for _, row := range event.Data {
		id, err := int64Value(row["id"])
		if err != nil {
			return err
		}
		if err := SyncUserFromMySQL(ctx, id); err != nil {
			return err
		}
	}
	logger.Info(ctx, "[USER-ES] 用户Binlog已同步到ES", "msg_id", msg.MsgId, "database", event.Database, "table", event.Table, "type", event.EventType)
	return nil
}

func SyncUserFromMySQL(ctx context.Context, id int64) error {
	unlock := lockCDC(es.ES_USER, id)
	defer unlock()
	userDB, err := db.NewUserDb(ctx)
	if err != nil {
		return err
	}
	latest, err := userDB.GetByIDIncludingDeleted(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return indexDeletedDocument(ctx, es.ES_USER, id)
	}
	if err != nil {
		return fmt.Errorf("查询最新用户状态失败: %w", err)
	}
	user := &es_model.User{
		IsDel: int64(latest.IsDel),
		ID:    latest.ID, Name: latest.Name, Age: latest.Age, Email: latest.Email, CreateTime: latest.CreateTime, UpdateTime: latest.UpdateTime,
	}
	_, err = es.IndexUserContext(ctx, user)
	return err
}
