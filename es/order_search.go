package es

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appconst "go-framework-demo/const"
	"go-framework-demo/logger"
)

func ValidateOrderSearchOptions(opt *OrderSearchOptions) error {
	if opt == nil {
		return fmt.Errorf("订单查询参数不能为空")
	}
	if opt.Status != nil && (*opt.Status < appconst.OrderStatusPendingPayment || *opt.Status > appconst.OrderStatusCancelled) {
		return fmt.Errorf("订单状态必须为1、2、3或4")
	}
	if opt.MinPrice != nil && *opt.MinPrice < 0 || opt.MaxPrice != nil && *opt.MaxPrice < 0 {
		return fmt.Errorf("订单金额不能为负数")
	}
	if opt.MinPrice != nil && opt.MaxPrice != nil && *opt.MinPrice > *opt.MaxPrice {
		return fmt.Errorf("最低金额不能大于最高金额")
	}
	if opt.CreatedFrom != nil && opt.CreatedFrom.IsZero() || opt.CreatedTo != nil && opt.CreatedTo.IsZero() {
		return fmt.Errorf("创建时间无效")
	}
	if opt.CreatedFrom != nil && opt.CreatedTo != nil && opt.CreatedFrom.After(*opt.CreatedTo) {
		return fmt.Errorf("开始时间不能晚于结束时间")
	}
	if opt.Sort != "" && opt.Sort != "newest" && opt.Sort != "updated_desc" && opt.Sort != "updated_asc" && opt.Sort != "price_asc" && opt.Sort != "price_desc" {
		return fmt.Errorf("排序必须为updated_desc、updated_asc、price_asc或price_desc")
	}
	return nil
}

// GetOrderIDsPage ES 只负责筛选、排序和分页；业务详情由 Service 回查 MySQL。
func GetOrderIDsPage(ctx context.Context, opt *OrderSearchOptions) ([]int64, int64, error) {
	client, err := clientReady()
	if err != nil {
		return nil, 0, err
	}
	source, err := BuildOrderElasticQuery(opt)
	if err != nil {
		return nil, 0, err
	}
	started := time.Now()
	result, err := client.Search(ES_ORDER).SearchSource(source).Sort("id", false).FetchSource(false).TrackTotalHits(true).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("查询订单ID失败: %w", err)
	}
	ids := make([]int64, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		id, err := strconv.ParseInt(hit.Id, 10, 64)
		if err != nil || id <= 0 {
			return nil, 0, fmt.Errorf("无效的订单文档ID: %q", hit.Id)
		}
		ids = append(ids, id)
	}
	logger.Info(ctx, "ES订单查询完成", "index", ES_ORDER, "returned_count", len(ids), "total", result.TotalHits(), "elapsed_ms", float64(time.Since(started))/float64(time.Millisecond))
	return ids, result.TotalHits(), nil
}
