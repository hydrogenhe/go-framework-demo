package reconciliation

import (
	"context"
	"encoding/json"
	"fmt"
	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
	"go-framework-demo/model"

	"github.com/olivere/elastic/v7"
	"gorm.io/gorm"
)

type userSource struct {
	IsDel           *int64  `json:"is_del"`
	ID              *int64  `json:"user_id"`
	Name            *string `json:"name"`
	Age             *int32  `json:"age"`
	Email           *string `json:"email"`
	CreateTimeStamp *int64  `json:"create_time_stamp"`
	UpdateTimeStamp *int64  `json:"update_time_stamp"`
}

func compareUser(source *userSource, target *model.User) []string {
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
		fields = append(fields, "user_id")
	}
	if source.Name == nil || *source.Name != target.Name {
		fields = append(fields, "name")
	}
	if source.Age == nil || *source.Age != target.Age {
		fields = append(fields, "age")
	}
	if source.Email == nil || *source.Email != target.Email {
		fields = append(fields, "email")
	}
	if source.CreateTimeStamp == nil || *source.CreateTimeStamp != target.CreateTime.UnixMilli() {
		fields = append(fields, "create_time_stamp")
	}
	if !target.UpdateTime.IsZero() && (source.UpdateTimeStamp == nil || *source.UpdateTimeStamp != target.UpdateTime.UnixMilli()) {
		fields = append(fields, "update_time_stamp")
	}
	return fields
}

func forwardUsers(ctx context.Context, db *gorm.DB) error {
	var last int64
	for {
		var targets []*model.User
		if err := queryAndPause(ctx, func() error {
			return db.WithContext(ctx).Unscoped().Table("user").Where("id > ?", last).Order("id ASC").Limit(batchSize).Find(&targets).Error
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
		sourceRecords, err := fetchSources(ctx, es.ES_USER, ids)
		if err != nil {
			return err
		}
		sourceMap, err := decodeUsers(sourceRecords)
		if err != nil {
			return err
		}
		errorMap := make(map[int64]*model.User)
		for _, target := range targets {
			source, exists := sourceMap[target.ID]
			fields := []string{"es缺失数据"}
			if exists {
				fields = compareUser(source, target)
			}
			if len(fields) > 0 {
				errorMap[target.ID] = target
				logUpdate(ctx, es.ES_USER, target.ID, fields)
			}
		}
		if err := updateUsers(ctx, errorMap, sourceRecords); err != nil {
			return err
		}
	}
}

func decodeUsers(sourceRecords map[int64]sourceRecord) (map[int64]*userSource, error) {
	sourceMap := make(map[int64]*userSource, len(sourceRecords))
	for id, record := range sourceRecords {
		var source userSource
		if err := json.Unmarshal(record.raw, &source); err != nil {
			return nil, fmt.Errorf("解析 user id=%d: %w", id, err)
		}
		sourceMap[id] = &source
	}
	return sourceMap, nil
}

func reverseUsers(ctx context.Context, db *gorm.DB) error {
	return es.WalkSearchAfter(ctx, es.ES_USER, reverseQuery(), 1, func(hits []*elastic.SearchHit) error { return reconcileUsers(ctx, db, hits) })
}

func reconcileUsers(ctx context.Context, db *gorm.DB, hits []*elastic.SearchHit) error {
	ids, sourceRecords, err := hitSources(hits)
	if err != nil {
		return err
	}
	var targets []*model.User
	if err := queryAndPause(ctx, func() error {
		return db.WithContext(ctx).Unscoped().Table("user").Where("id IN ?", ids).Find(&targets).Error
	}); err != nil {
		return err
	}
	targetMap := make(map[int64]*model.User, len(targets))
	for _, target := range targets {
		targetMap[target.ID] = target
	}
	updateMap := make(map[int64]*model.User)
	var deleteIDs []int64
	for _, id := range ids {
		target, exists := targetMap[id]
		if !exists || target.IsDel == 1 {
			deleteIDs = append(deleteIDs, id)
			continue
		}
		var source userSource
		if err := json.Unmarshal(sourceRecords[id].raw, &source); err != nil {
			return fmt.Errorf("解析 user id=%d: %w", id, err)
		}
		fields := compareUser(&source, target)
		if len(fields) > 0 {
			updateMap[id] = target
			logUpdate(ctx, es.ES_USER, id, fields)
		}
	}
	if err := updateUsers(ctx, updateMap, sourceRecords); err != nil {
		return err
	}
	return deleteSources(ctx, es.ES_USER, deleteIDs, sourceRecords)
}

func updateUsers(ctx context.Context, updateMap map[int64]*model.User, sourceMap map[int64]sourceRecord) error {
	bulk := es.ElasticClient.Bulk()
	for id, target := range updateMap {
		var doc map[string]interface{}
		var err error
		if target.IsDel == 1 {
			doc = map[string]interface{}{"user_id": target.ID, "is_del": int64(1)}
		} else {
			user := es_model.User{IsDel: int64(target.IsDel), ID: target.ID, Name: target.Name, Age: target.Age, Email: target.Email, CreateTime: target.CreateTime}
			doc, err = user.ToMap()
		}
		if err != nil {
			return fmt.Errorf("构建 user id=%d: %w", id, err)
		}
		if err := addUpdate(bulk, es.ES_USER, id, doc, sourceMap); err != nil {
			return err
		}
	}
	return applyBulk(ctx, bulk)
}
