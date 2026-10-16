package reconciliation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/model"
	"gorm.io/gorm"
)

type productSource struct {
	IsDel           *int64  `json:"is_del"`
	ID              *int64  `json:"id"`
	Name            *string `json:"name"`
	Price           *int64  `json:"price"`
	Stock           *int32  `json:"stock"`
	Image           *string `json:"image"`
	CreateTimeStamp *int64  `json:"create_time_stamp"`
	UpdateTimeStamp *int64  `json:"update_time_stamp"`
}

func compareProduct(source *productSource, target *model.Product) []string {
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
	if source.Name == nil || *source.Name != target.Name {
		fields = append(fields, "name")
	}
	if source.Price == nil || *source.Price != target.Price {
		fields = append(fields, "price")
	}
	if source.Stock == nil || *source.Stock != target.Stock {
		fields = append(fields, "stock")
	}
	if source.Image == nil || *source.Image != target.Image {
		fields = append(fields, "image")
	}
	if source.CreateTimeStamp == nil || *source.CreateTimeStamp != target.CreateTime.UnixMilli() {
		fields = append(fields, "create_time_stamp")
	}
	if !target.UpdateTime.IsZero() && (source.UpdateTimeStamp == nil || *source.UpdateTimeStamp != target.UpdateTime.UnixMilli()) {
		fields = append(fields, "update_time_stamp")
	}
	return fields
}

func forwardProducts(ctx context.Context, db *gorm.DB) error {
	var last int64
	for {
		var targets []*model.Product
		if err := queryAndPause(ctx, func() error {
			return db.WithContext(ctx).Unscoped().Table("product").Where("id > ?", last).Order("id ASC").Limit(batchSize).Find(&targets).Error
		}); err != nil {
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
		sourceRecords, err := fetchSources(ctx, es.ES_PRODUCT, ids)
		if err != nil {
			return err
		}
		sourceMap, err := decodeProducts(sourceRecords)
		if err != nil {
			return err
		}
		errorMap := make(map[int64]*model.Product)
		for _, target := range targets {
			source, exists := sourceMap[target.ID]
			fields := []string{"es缺失数据"}
			if exists {
				fields = compareProduct(source, target)
			}
			if len(fields) > 0 {
				errorMap[target.ID] = target
				logUpdate(ctx, es.ES_PRODUCT, target.ID, fields)
			}
		}
		if err := updateProducts(ctx, errorMap, sourceRecords); err != nil {
			return err
		}
	}
}

func decodeProducts(sourceRecords map[int64]sourceRecord) (map[int64]*productSource, error) {
	sourceMap := make(map[int64]*productSource, len(sourceRecords))
	for id, record := range sourceRecords {
		var source productSource
		if err := json.Unmarshal(record.raw, &source); err != nil {
			return nil, fmt.Errorf("解析 product id=%d: %w", id, err)
		}
		sourceMap[id] = &source
	}
	return sourceMap, nil
}

func reverseProducts(ctx context.Context, db *gorm.DB) error {
	return es.WalkSearchAfter(ctx, es.ES_PRODUCT, reverseQuery(), 1, func(hits []*elastic.SearchHit) error { return reconcileProducts(ctx, db, hits) })
}

func reconcileProducts(ctx context.Context, db *gorm.DB, hits []*elastic.SearchHit) error {
	ids, sourceRecords, err := hitSources(hits)
	if err != nil {
		return err
	}
	var targets []*model.Product
	if err := queryAndPause(ctx, func() error {
		return db.WithContext(ctx).Unscoped().Table("product").Where("id IN ?", ids).Find(&targets).Error
	}); err != nil {
		return err
	}
	targetMap := make(map[int64]*model.Product, len(targets))
	for _, target := range targets {
		targetMap[target.ID] = target
	}
	updateMap := make(map[int64]*model.Product)
	var deleteIDs []int64
	for _, id := range ids {
		target, exists := targetMap[id]
		if !exists || target.IsDel == 1 {
			deleteIDs = append(deleteIDs, id)
			continue
		}
		var source productSource
		if err := json.Unmarshal(sourceRecords[id].raw, &source); err != nil {
			return fmt.Errorf("解析 product id=%d: %w", id, err)
		}
		fields := compareProduct(&source, target)
		if len(fields) > 0 {
			updateMap[id] = target
			logUpdate(ctx, es.ES_PRODUCT, id, fields)
		}
	}
	if err := updateProducts(ctx, updateMap, sourceRecords); err != nil {
		return err
	}
	return deleteSources(ctx, es.ES_PRODUCT, deleteIDs, sourceRecords)
}

func updateProducts(ctx context.Context, updateMap map[int64]*model.Product, sourceMap map[int64]sourceRecord) error {
	bulk := es.ElasticClient.Bulk()
	for id, target := range updateMap {
		var doc map[string]interface{}
		var err error
		if target.IsDel == 1 {
			doc = map[string]interface{}{"id": target.ID, "is_del": int64(1)}
		} else {
			doc, err = es.ProductFromModel(target).ToMap()
		}
		if err != nil {
			return fmt.Errorf("构建 product id=%d: %w", id, err)
		}
		if err := addUpdate(bulk, es.ES_PRODUCT, id, doc, sourceMap); err != nil {
			return err
		}
	}
	return applyBulk(ctx, bulk)
}
