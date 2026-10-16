package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/logger"
)

// WalkProductIDsSearchAfter 用 PIT 固定索引快照，以最后一条记录的 sort 值继续查询。
func WalkProductIDsSearchAfter(ctx context.Context, opt *ProductSearchOptions, handle func([]int64) error) error {
	if opt == nil {
		return fmt.Errorf("商品查询参数不能为空")
	}
	options := *opt
	options.Page, options.PageSize = nil, nil
	source, err := BuildProductElasticQuery(&options)
	if err != nil {
		return err
	}
	source.Size(500).Sort("id", false).Sort("_shard_doc", false).FetchSource(false).TrackTotalHits(false)
	return WalkSearchAfter(ctx, ES_PRODUCT, source, 3, func(hits []*elastic.SearchHit) error {
		ids, err := productHitIDs(hits)
		if err != nil {
			return err
		}
		return handle(ids)
	})
}

// WalkSearchAfter 复用 PIT 分页核心；source 必须包含稳定排序及页大小。
// 回调可读取 _source 或版本信息，调用方负责设置相应查询选项。
func WalkSearchAfter(ctx context.Context, index string, source *elastic.SearchSource, sortKeys int, handle func([]*elastic.SearchHit) error) error {
	if source == nil || handle == nil || sortKeys < 1 {
		return fmt.Errorf("无效的遍历参数")
	}
	client, err := clientReady()
	if err != nil {
		return err
	}
	opened, err := client.OpenPointInTime(index).KeepAlive("1m").Do(ctx)
	if err != nil {
		return fmt.Errorf("打开遍历PIT失败: %w", err)
	}
	pitID := opened.Id
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := client.ClosePointInTime(pitID).Do(cleanup); err != nil {
			logger.Warn(ctx, "关闭遍历PIT失败", "method", "WalkSearchAfter", "error", err)
		}
	}()
	// _shard_doc 是 PIT 内稳定的最终排序键，防止同一排序值导致漏读或重复。
	var after []interface{}
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		source.PointInTime(elastic.NewPointInTimeWithKeepAlive(pitID, "1m"))
		if len(after) > 0 {
			source.SearchAfter(after...)
		}
		body, err := source.Source()
		if err != nil {
			return err
		}
		// PIT 请求不能指定索引；其查询目标已经由 PIT 确定。
		response, err := client.PerformRequest(ctx, elastic.PerformRequestOptions{Method: "POST", Path: "/_search", Body: body})
		if err != nil {
			return fmt.Errorf("search_after查询文档失败: %w", err)
		}
		var result elastic.SearchResult
		decoder := json.NewDecoder(bytes.NewReader(response.Body))
		// sort 中包含 int64 商品 ID，保留 json.Number，避免 float64 舍入导致游标失真。
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			return fmt.Errorf("解析search_after结果失败: %w", err)
		}
		if result.TimedOut || (result.Shards != nil && result.Shards.Failed > 0) {
			return fmt.Errorf("search_after查询超时或存在失败分片")
		}
		if result.PitId != "" {
			pitID = result.PitId
		}
		if result.Hits == nil || len(result.Hits.Hits) == 0 {
			break
		}
		last := result.Hits.Hits[len(result.Hits.Hits)-1]
		if len(last.Sort) != sortKeys {
			return fmt.Errorf("search_after结果缺少完整排序游标")
		}
		if err := handle(result.Hits.Hits); err != nil {
			return err
		}
		after = last.Sort
		count += len(result.Hits.Hits)
	}
	logger.Info(ctx, "ES遍历完成", "method", "WalkSearchAfter", "mode", "search_after", "index", index, "matched_count", count)
	return nil
}
