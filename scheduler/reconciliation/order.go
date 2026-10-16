package reconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/model"
	"gorm.io/gorm"
)

type orderSource struct {
	IsDel           *int64            `json:"is_del"`
	ID              *int64            `json:"id"`
	OrderNo         *string           `json:"order_no"`
	UserID          *int64            `json:"user_id"`
	AmountFen       *int64            `json:"amount_fen"`
	Status          *int32            `json:"status"`
	CreateTimeStamp *int64            `json:"create_time_stamp"`
	UpdateTimeStamp *int64            `json:"update_time_stamp"`
	Items           []orderItemSource `json:"items"`
}

func compareOrder(source *orderSource, target *model.Order, targetItems []*model.OrderItem) []string {
	var fields []string
	if source.IsDel == nil || *source.IsDel != int64(target.IsDel) {
		fields = append(fields, "is_del")
	}
	if len(fields) > 0 {
		return fields
	}
	if target.IsDel == 1 {
		return nil
	}
	if source.ID == nil || *source.ID != target.ID {
		fields = append(fields, "id")
	}
	if source.OrderNo == nil || *source.OrderNo != target.OrderNo {
		fields = append(fields, "order_no")
	}
	if source.UserID == nil || *source.UserID != target.UserID {
		fields = append(fields, "user_id")
	}
	if source.AmountFen == nil || *source.AmountFen != target.Amount {
		fields = append(fields, "amount_fen")
	}
	if source.Status == nil || *source.Status != target.Status {
		fields = append(fields, "status")
	}
	if source.CreateTimeStamp == nil || *source.CreateTimeStamp != target.CreateTime.UnixMilli() {
		fields = append(fields, "create_time_stamp")
	}
	if !target.UpdateTime.IsZero() && (source.UpdateTimeStamp == nil || *source.UpdateTimeStamp != target.UpdateTime.UnixMilli()) {
		fields = append(fields, "update_time_stamp")
	}
	fields = append(fields, compareOrderItems(source.Items, targetItems)...)
	return fields
}

func loadOrderItems(ctx context.Context, db *gorm.DB, targets []*model.Order) (map[int64][]*model.OrderItem, error) {
	ids := make([]int64, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	targetMap := make(map[int64][]*model.OrderItem, len(ids))
	if len(ids) == 0 {
		return targetMap, nil
	}
	var targetsItems []*model.OrderItem
	if err := queryAndPause(ctx, func() error {
		return db.Unscoped().Where("order_id IN ?", ids).Order("id ASC").Find(&targetsItems).Error
	}); err != nil {
		return nil, err
	}
	for _, target := range targetsItems {
		targetMap[target.OrderID] = append(targetMap[target.OrderID], target)
	}
	return targetMap, nil
}

func loadOrders(ctx context.Context, db *gorm.DB, last int64, ids []int64) ([]*model.Order, map[int64][]*model.OrderItem, error) {
	var targets []*model.Order
	var targetItems map[int64][]*model.OrderItem
	err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Unscoped().Table("order")
		if ids == nil {
			query = query.Where("id > ?", last).Order("id ASC").Limit(batchSize)
		} else {
			query = query.Where("id IN ?", ids)
		}
		if err := queryAndPause(ctx, func() error { return query.Find(&targets).Error }); err != nil {
			return err
		}
		var err error
		targetItems, err = loadOrderItems(ctx, tx, targets)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return targets, targetItems, err
}

func forwardOrders(ctx context.Context, db *gorm.DB) error {
	var last int64
	for {
		targets, targetItems, err := loadOrders(ctx, db.WithContext(ctx), last, nil)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		last = targets[len(targets)-1].ID
		ids := make([]int64, 0, len(targets))
		for _, target := range targets {
			ids = append(ids, target.ID)
		}
		sourceRecords, err := fetchSources(ctx, es.ES_ORDER, ids)
		if err != nil {
			return err
		}
		sourceMap, err := decodeOrders(sourceRecords)
		if err != nil {
			return err
		}
		errorMap := make(map[int64]*model.Order)
		for _, target := range targets {
			source, exists := sourceMap[target.ID]
			fields := []string{"es缺失数据"}
			if exists {
				fields = compareOrder(source, target, targetItems[target.ID])
			}
			if len(fields) > 0 {
				errorMap[target.ID] = target
				logUpdate(ctx, es.ES_ORDER, target.ID, fields)
			}
		}
		if err := updateOrders(ctx, errorMap, sourceRecords, targetItems); err != nil {
			return err
		}
	}
}

func decodeOrders(sourceRecords map[int64]sourceRecord) (map[int64]*orderSource, error) {
	sourceMap := make(map[int64]*orderSource, len(sourceRecords))
	for id, record := range sourceRecords {
		var source orderSource
		if err := json.Unmarshal(record.raw, &source); err != nil {
			return nil, fmt.Errorf("解析 order id=%d: %w", id, err)
		}
		sourceMap[id] = &source
	}
	return sourceMap, nil
}

func reverseOrders(ctx context.Context, db *gorm.DB) error {
	return es.WalkSearchAfter(ctx, es.ES_ORDER, reverseQuery(), 1, func(hits []*elastic.SearchHit) error { return reconcileOrders(ctx, db, hits) })
}

func reconcileOrders(ctx context.Context, db *gorm.DB, hits []*elastic.SearchHit) error {
	ids, sourceRecords, err := hitSources(hits)
	if err != nil {
		return err
	}
	targets, targetItems, err := loadOrders(ctx, db.WithContext(ctx), 0, ids)
	if err != nil {
		return err
	}
	targetMap := make(map[int64]*model.Order, len(targets))
	for _, target := range targets {
		targetMap[target.ID] = target
	}
	updateMap := make(map[int64]*model.Order)
	var deleteIDs []int64
	for _, id := range ids {
		target, exists := targetMap[id]
		if !exists || target.IsDel == 1 {
			deleteIDs = append(deleteIDs, id)
			continue
		}
		var source orderSource
		if err := json.Unmarshal(sourceRecords[id].raw, &source); err != nil {
			return fmt.Errorf("解析 order id=%d: %w", id, err)
		}
		fields := compareOrder(&source, target, targetItems[id])
		if len(fields) > 0 {
			updateMap[id] = target
			logUpdate(ctx, es.ES_ORDER, id, fields)
		}
	}
	if err := updateOrders(ctx, updateMap, sourceRecords, targetItems); err != nil {
		return err
	}
	return deleteSources(ctx, es.ES_ORDER, deleteIDs, sourceRecords)
}

func updateOrders(ctx context.Context, updateMap map[int64]*model.Order, sourceMap map[int64]sourceRecord, targetItems map[int64][]*model.OrderItem) error {
	bulk := es.ElasticClient.Bulk()
	for id, target := range updateMap {
		var doc map[string]interface{}
		var err error
		if target.IsDel == 1 {
			doc = map[string]interface{}{"id": target.ID, "is_del": int64(1)}
		} else {
			doc, err = orderDocument(target, targetItems[id])
		}
		if err != nil {
			return fmt.Errorf("构建 order id=%d: %w", id, err)
		}
		if err := addUpdate(bulk, es.ES_ORDER, id, doc, sourceMap); err != nil {
			return err
		}
	}
	return applyBulk(ctx, bulk)
}

type orderItemSource struct {
	ID              *int64  `json:"id"`
	IsDel           *int64  `json:"is_del"`
	Image           *string `json:"image"`
	ProductID       *int64  `json:"product_id"`
	ProductName     *string `json:"product_name"`
	Quantity        *int32  `json:"quantity"`
	UnitPriceFen    *int64  `json:"unit_price_fen"`
	Status          *int32  `json:"status"`
	UpdateTimeStamp *int64  `json:"update_time_stamp"`
}

func compareOrderItems(sources []orderItemSource, targets []*model.OrderItem) []string {
	var fields []string
	sourceMap := make(map[int64]*orderItemSource, len(sources))
	for i := range sources {
		source := &sources[i]
		if source.ID == nil {
			fields = append(fields, "items.id")
			continue
		}
		if _, exists := sourceMap[*source.ID]; exists {
			fields = append(fields, fmt.Sprintf("items[id=%d].duplicate", *source.ID))
		}
		sourceMap[*source.ID] = source
	}
	targetMap := make(map[int64]*model.OrderItem, len(targets))
	for _, target := range targets {
		targetMap[target.ID] = target
	}
	for _, target := range targets {
		source, exists := sourceMap[target.ID]
		if !exists {
			fields = append(fields, fmt.Sprintf("items[id=%d].missing", target.ID))
			continue
		}
		prefix := fmt.Sprintf("items[id=%d].", target.ID)
		if source.ID == nil || *source.ID != target.ID {
			fields = append(fields, prefix+"id")
		}
		if source.IsDel == nil || *source.IsDel != int64(target.IsDel) {
			fields = append(fields, prefix+"is_del")
		}
		if source.Image == nil || *source.Image != target.Image {
			fields = append(fields, prefix+"image")
		}
		if source.ProductID == nil || *source.ProductID != target.ProductID {
			fields = append(fields, prefix+"product_id")
		}
		if source.ProductName == nil || *source.ProductName != target.ProductName {
			fields = append(fields, prefix+"product_name")
		}
		if source.Quantity == nil || *source.Quantity != target.Quantity {
			fields = append(fields, prefix+"quantity")
		}
		if source.UnitPriceFen == nil || *source.UnitPriceFen != target.UnitPriceFen {
			fields = append(fields, prefix+"unit_price_fen")
		}
		if source.Status == nil || *source.Status != target.Status {
			fields = append(fields, prefix+"status")
		}
		if !target.UpdateTime.IsZero() && (source.UpdateTimeStamp == nil || *source.UpdateTimeStamp != target.UpdateTime.UnixMilli()) {
			fields = append(fields, prefix+"update_time_stamp")
		}
	}
	for id := range sourceMap {
		if _, exists := targetMap[id]; !exists {
			fields = append(fields, fmt.Sprintf("items[id=%d].extra", id))
		}
	}
	return fields
}

func orderDocument(target *model.Order, targetItems []*model.OrderItem) (map[string]interface{}, error) {
	projected, err := es.OrderFromModels(target, targetItems)
	if err != nil {
		return nil, err
	}
	return projected.ToMap()
}
