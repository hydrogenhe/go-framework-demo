package es

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/logger"
)

// ProductExportUseSearchAfter 导出遍历方案开关：false 使用 Scroll（默认），true 使用 search_after + PIT。
// 仅在代码配置或启动阶段设置，业务请求期间不要修改此全局配置。
var ProductExportUseSearchAfter = false

// ValidateProductSearchOptions 校验列表与导出共用的筛选条件。
func ValidateProductSearchOptions(opt *ProductSearchOptions) error {
	if opt == nil {
		return fmt.Errorf("商品查询参数不能为空")
	}
	if opt.Name != nil && len(*opt.Name) > 255 {
		return fmt.Errorf("商品名称搜索不能超过255字节")
	}
	if opt.CreatedFrom != nil && opt.CreatedFrom.IsZero() || opt.CreatedTo != nil && opt.CreatedTo.IsZero() {
		return fmt.Errorf("创建时间无效")
	}
	if opt.CreatedFrom != nil && opt.CreatedTo != nil && opt.CreatedFrom.After(*opt.CreatedTo) {
		return fmt.Errorf("创建时间起点不能晚于终点")
	}
	if opt.MinStock != nil && *opt.MinStock < 0 || opt.MaxStock != nil && *opt.MaxStock < 0 {
		return fmt.Errorf("库存范围不能为负数")
	}
	if opt.MinStock != nil && opt.MaxStock != nil && *opt.MinStock > *opt.MaxStock {
		return fmt.Errorf("库存下限不能大于上限")
	}
	if opt.MinPrice != nil && *opt.MinPrice < 0 || opt.MaxPrice != nil && *opt.MaxPrice < 0 {
		return fmt.Errorf("价格范围不能为负数")
	}
	if opt.MinPrice != nil && opt.MaxPrice != nil && *opt.MinPrice > *opt.MaxPrice {
		return fmt.Errorf("价格下限不能大于上限")
	}
	return nil
}

// WalkProductIDs 根据配置选择全量遍历方案，两种方案共用筛选条件及批量回查入口。
func WalkProductIDs(ctx context.Context, opt *ProductSearchOptions, handle func([]int64) error) error {
	if ProductExportUseSearchAfter {
		return WalkProductIDsSearchAfter(ctx, opt, handle)
	}
	return walkProductIDsScroll(ctx, opt, handle)
}

func walkProductIDsScroll(ctx context.Context, opt *ProductSearchOptions, handle func([]int64) error) error {
	if opt == nil {
		return fmt.Errorf("商品查询参数不能为空")
	}
	options := *opt
	options.Page, options.PageSize = nil, nil
	source, err := BuildProductElasticQuery(&options)
	if err != nil {
		return err
	}
	client, err := clientReady()
	if err != nil {
		return err
	}
	scroll := client.Scroll(ES_PRODUCT).SearchSource(source).Size(500).Sort("id", false).FetchSource(false).Scroll("1m")
	defer func() {
		// 请求取消后仍清理 ES 游标，避免等待游标自动过期。
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := scroll.Clear(cleanup); err != nil {
			logger.Warn(ctx, "清理商品导出ES游标失败", "method", "WalkProductIDs", "error", err)
		}
	}()
	var count int
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := scroll.Do(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ES遍历商品ID失败: %w", err)
		}
		ids, err := productHitIDs(result.Hits.Hits)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		if err := handle(ids); err != nil {
			return err
		}
		count += len(ids)
	}
	logger.Info(ctx, "ES商品导出ID查询完成", "method", "WalkProductIDs", "mode", "scroll", "index", ES_PRODUCT, "matched_count", count)
	return nil
}

func productHitIDs(hits []*elastic.SearchHit) ([]int64, error) {
	ids := make([]int64, 0, len(hits))
	for _, hit := range hits {
		id, err := strconv.ParseInt(hit.Id, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("ES商品文档ID无效: %q", hit.Id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
