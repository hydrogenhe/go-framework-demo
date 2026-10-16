package handler

import (
	"context"
	"fmt"
	"go-framework-demo/logger"

	"errors"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/db"
	"go-framework-demo/es"
	"go-framework-demo/model"
	"go-framework-demo/redis"
	"gorm.io/gorm"
)

func productESSyncHandler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		msgCtx := messageContext(ctx, msg)
		if err := handleOneProductCDCMessage(msgCtx, msg); err != nil {
			logConsumeFailure(msgCtx, "PRODUCT-CDC", msg, err)
			return consumer.ConsumeRetryLater, err
		}
	}
	return consumer.ConsumeSuccess, nil
}

func handleOneProductCDCMessage(ctx context.Context, msg *primitive.MessageExt) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	event, err := parseCanalEvent(msg, "product")
	if err != nil {
		return err
	}
	if event.IsDDL {
		return nil
	}
	for _, row := range event.Data {
		id, err := int64Value(row["id"])
		if err != nil {
			return fmt.Errorf("解析商品ID失败: %w", err)
		}
		if err := SyncProductFromMySQL(ctx, id); err != nil {
			return err
		}
	}
	logger.Info(ctx, "[PRODUCT-CDC] 商品Binlog已同步到ES和Redis", "msg_id", msg.MsgId, "database", event.Database, "table", event.Table, "type", event.EventType)
	return nil
}

func SyncProductFromMySQL(ctx context.Context, id int64) error {
	unlock := lockCDC(es.ES_PRODUCT, id)
	defer unlock()
	productDB, err := db.NewProductDb(ctx)
	if err != nil {
		return err
	}
	product, err := productDB.GetByIDIncludingDeleted(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return syncProductSideEffects(ctx, id, nil)
	}
	if err != nil {
		return err
	}
	return syncProductSideEffects(ctx, id, product)
}

// syncProductSideEffects 独立尝试 ES 和 Redis；一端失败不会阻止另一端执行。
func syncProductSideEffects(ctx context.Context, id int64, product *model.Product) error {
	if product == nil {
		esErr := indexDeletedDocument(ctx, es.ES_PRODUCT, id)
		cacheErr := deleteProductCache(ctx, id)
		return errors.Join(wrapSyncError("写入商品ES删除标记失败", esErr), cacheErr)
	}
	_, esErr := es.IndexProductContext(ctx, es.ProductFromModel(product))
	var cacheErr error
	if product.IsDel != 0 {
		cacheErr = deleteProductCache(ctx, id)
	} else {
		cacheErr = setProductCache(ctx, product)
	}
	return errors.Join(wrapSyncError("写入商品ES文档失败", esErr), cacheErr)
}

func setProductCache(ctx context.Context, product *model.Product) error {
	if redis.RedisClient == nil {
		return nil
	}
	productCache := redis.NewProductCache()
	if err := productCache.SetProduct(ctx, product); err != nil {
		return fmt.Errorf("写入商品Redis缓存失败: %w", err)
	}
	return nil
}

func deleteProductCache(ctx context.Context, id int64) error {
	if redis.RedisClient == nil {
		return nil
	}
	productCache := redis.NewProductCache()
	if err := productCache.DelProduct(ctx, id); err != nil {
		return fmt.Errorf("删除商品Redis缓存失败: %w", err)
	}
	return nil
}

func wrapSyncError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
