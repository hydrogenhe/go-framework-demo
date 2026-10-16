// Package reconciliation 以 MySQL 为目标状态，检查并补偿 ES。
package reconciliation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/logger"
	"gorm.io/gorm"
)

// batchSize 每批对账处理的记录数。
const batchSize = 100
// dbQueryPause 对账每次数据库查询后的暂停时长，100毫秒。
const dbQueryPause = 100 * time.Millisecond

var shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

func marker() string { return "Reconciliation+" + time.Now().In(shanghai).Format("2006-01-02") }

type sourceRecord struct {
	raw                json.RawMessage
	seqNo, primaryTerm *int64
}

// Run 先完成所有 DB→ES 补偿，再执行 ES→DB 更新/删除。db 必须是主库连接。
func Run(ctx context.Context, db *gorm.DB) error {
	if db == nil || es.ElasticClient == nil {
		return fmt.Errorf("对账需要已初始化的 MySQL 和 ES 客户端")
	}
	for _, run := range []func(context.Context, *gorm.DB) error{forwardUsers, forwardProducts, forwardOrders, reverseUsers, reverseProducts, reverseOrders} {
		if err := run(ctx, db); err != nil {
			logger.Error(ctx, marker(), "error", err.Error())
			return err
		}
	}
	return nil
}

// RunScheduler 在独立 goroutine 中执行 ES 对账：启动后立即运行，成功或失败后等待 interval 再运行。
func RunScheduler(ctx context.Context, interval time.Duration, db *gorm.DB) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	for {
		started := time.Now()
		logger.Info(ctx, "ES 对账任务开始")
		err := Run(ctx, db)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.Error(ctx, "ES 对账任务失败，等待下个周期重试", "error", err, "elapsed_ms", time.Since(started).Milliseconds())
		} else {
			logger.Info(ctx, "ES 对账任务完成", "elapsed_ms", time.Since(started).Milliseconds())
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// queryAndPause 查询结束后等待一段时间，降低连续对账查询给 MySQL 带来的压力。
func queryAndPause(ctx context.Context, query func() error) error {
	queryErr := query()
	timer := time.NewTimer(dbQueryPause)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		if queryErr != nil {
			return queryErr
		}
		return ctx.Err()
	case <-timer.C:
		return queryErr
	}
}

func fetchSources(ctx context.Context, index string, ids []int64) (map[int64]sourceRecord, error) {
	req := es.ElasticClient.MultiGet()
	for _, id := range ids {
		req.Add(elastic.NewMultiGetItem().Index(index).Id(strconv.FormatInt(id, 10)))
	}
	result, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	if len(result.Docs) != len(ids) {
		return nil, fmt.Errorf("%s 批量读取返回数量异常", index)
	}
	sourceMap := make(map[int64]sourceRecord, len(result.Docs))
	for _, doc := range result.Docs {
		if doc.Error != nil {
			return nil, fmt.Errorf("读取 %s id=%s: %s", index, doc.Id, doc.Error.Reason)
		}
		if !doc.Found {
			continue
		}
		id, err := strconv.ParseInt(doc.Id, 10, 64)
		if err != nil {
			return nil, err
		}
		sourceMap[id] = sourceRecord{raw: doc.Source, seqNo: doc.SeqNo, primaryTerm: doc.PrimaryTerm}
	}
	return sourceMap, nil
}

func hitSources(hits []*elastic.SearchHit) ([]int64, map[int64]sourceRecord, error) {
	ids := make([]int64, 0, len(hits))
	sourceMap := make(map[int64]sourceRecord, len(hits))
	for _, hit := range hits {
		id, err := strconv.ParseInt(hit.Id, 10, 64)
		if err != nil || id <= 0 {
			return nil, nil, fmt.Errorf("无效 ES ID: %q", hit.Id)
		}
		ids = append(ids, id)
		sourceMap[id] = sourceRecord{raw: hit.Source, seqNo: hit.SeqNo, primaryTerm: hit.PrimaryTerm}
	}
	return ids, sourceMap, nil
}

func reverseQuery() *elastic.SearchSource {
	return elastic.NewSearchSource().Query(elastic.NewMatchAllQuery()).Size(batchSize).
		Sort("_shard_doc", true).FetchSource(true).SeqNoAndPrimaryTerm(true).TrackTotalHits(false)
}

func logUpdate(ctx context.Context, index string, id int64, fields []string) {
	logger.Error(ctx, marker(), "index", index, "id", id, "action", "update", "fields", fields, "error", "待补偿 ES 数据")
}

// 写入补偿文档时只序列化要写入的数据，不通过 JSON 转换来比较字段。
func addUpdate(bulk *elastic.BulkService, index string, id int64, doc map[string]interface{}, sourceMap map[int64]sourceRecord) error {
	req := elastic.NewBulkIndexRequest().Index(index).Id(strconv.FormatInt(id, 10)).Doc(doc)
	source, exists := sourceMap[id]
	if !exists {
		req.OpType("create")
	} else {
		if source.seqNo == nil || source.primaryTerm == nil {
			return fmt.Errorf("%s id=%d 缺少版本信息", index, id)
		}
		req.IfSeqNo(*source.seqNo).IfPrimaryTerm(*source.primaryTerm)
	}
	bulk.Add(req)
	return nil
}

func applyBulk(ctx context.Context, bulk *elastic.BulkService) error {
	if bulk.NumberOfActions() == 0 {
		return nil
	}
	result, err := bulk.Refresh("wait_for").Do(ctx)
	if err != nil {
		return err
	}
	for _, item := range result.Items {
		for action, status := range item {
			if status.Status == 409 {
				logger.Warn(ctx, marker(), "index", status.Index, "id", status.Id, "action", action, "error", "ES 已变化，留待下轮对账")
				continue
			}
			if action == "delete" && status.Status == 404 && status.Result == "not_found" {
				continue
			}
			if status.Status >= 300 || status.Error != nil {
				return fmt.Errorf("补偿 %s id=%s action=%s 失败: status=%d error=%v", status.Index, status.Id, action, status.Status, status.Error)
			}
			logger.Info(ctx, marker(), "index", status.Index, "id", status.Id, "action", action, "result", status.Result)
		}
	}
	return nil
}

func deleteSources(ctx context.Context, index string, deleteIDs []int64, sourceMap map[int64]sourceRecord) error {
	bulk := es.ElasticClient.Bulk()
	for _, id := range deleteIDs {
		source := sourceMap[id]
		if source.seqNo == nil || source.primaryTerm == nil {
			return fmt.Errorf("%s id=%d 缺少删除版本信息", index, id)
		}
		logger.Info(ctx, marker(), "index", index, "id", id, "action", "delete", "reason", "MySQL 不存在或已软删除")
		bulk.Add(elastic.NewBulkDeleteRequest().Index(index).Id(strconv.FormatInt(id, 10)).IfSeqNo(*source.seqNo).IfPrimaryTerm(*source.primaryTerm))
	}
	return applyBulk(ctx, bulk)
}
