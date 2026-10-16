package handler

import (
	"context"
	"errors"
	"go-framework-demo/logger"
	"gorm.io/gorm"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/db"
	"go-framework-demo/es"
)

func orderESSyncHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		msgCtx := messageContext(ctx, msg)
		if err := handleOneOrderCDCMessage(msgCtx, msg); err != nil {
			logConsumeFailure(msgCtx, "ORDER-ES", msg, err)
			return consumer.ConsumeRetryLater, err
		}
	}
	return consumer.ConsumeSuccess, nil
}

func handleOneOrderCDCMessage(ctx context.Context, msg *primitive.MessageExt) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	event, err := parseCanalEvent(msg, "order", "order_item")
	if err != nil {
		return err
	}
	if event.IsDDL {
		return nil
	}
	for _, row := range event.Data {
		orderID, err := int64Value(row["id"])
		if event.Table == "order_item" {
			orderID, err = int64Value(row["order_id"])
		}
		if err != nil {
			return err
		}
		if err := SyncOrderFromMySQL(ctx, orderID); err != nil {
			return err
		}
	}
	logger.Info(ctx, "[ORDER-ES] 订单Binlog已同步到ES", "msg_id", msg.MsgId, "database", event.Database, "table", event.Table, "type", event.EventType)
	return nil
}

func SyncOrderFromMySQL(ctx context.Context, id int64) error {
	unlock := lockCDC(es.ES_ORDER, id)
	defer unlock()
	q, err := db.NewOrderDb(ctx)
	if err != nil {
		return err
	}
	o, err := q.GetByIDIncludingDeleted(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return indexDeletedDocument(ctx, es.ES_ORDER, id)
	}
	if err != nil {
		return err
	}
	itemsDB, err := db.NewOrderItemDb(ctx)
	if err != nil {
		return err
	}
	items, err := itemsDB.GetByOrderIDIncludingDeleted(ctx, id)
	if err != nil {
		return err
	}
	doc, err := es.OrderFromModels(o, items)
	if err != nil {
		return err
	}
	_, err = es.IndexOrderContext(ctx, doc)
	return err
}
