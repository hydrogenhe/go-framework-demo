package es

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olivere/elastic/v7"
	"go-framework-demo/es/es_model"
	"go-framework-demo/logger"
	"go-framework-demo/model"
	"strconv"
	"strings"
	"time"
)

// ES_PRODUCT 商品搜索副本的索引名称。
const ES_PRODUCT = "product_test"

type ProductSearchOptions struct {
	ID        *int64
	Name      *string
	Page      *int
	PageSize  *int
	AscUpdate *bool
	// AscCreate 保留旧调用方兼容；现在控制更新时间排序方向。
	AscCreate   *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	MinStock    *int32
	MaxStock    *int32
	MinPrice    *int64
	MaxPrice    *int64
}

// BuildProductElasticQuery 组装可选查询条件，不依赖 ES 客户端。
func BuildProductElasticQuery(opt *ProductSearchOptions) (*elastic.SearchSource, error) {
	if err := ValidateProductSearchOptions(opt); err != nil {
		return nil, err
	}
	q := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("is_del", 0))
	if opt.ID != nil {
		q.Filter(elastic.NewTermQuery("id", *opt.ID))
	}
	if opt.Name != nil && strings.TrimSpace(*opt.Name) != "" {
		q.Must(elastic.NewMatchQuery("name", strings.TrimSpace(*opt.Name)))
	}
	if opt.CreatedFrom != nil || opt.CreatedTo != nil {
		r := elastic.NewRangeQuery("create_time_stamp")
		if opt.CreatedFrom != nil {
			r.Gte(opt.CreatedFrom.UnixMilli())
		}
		if opt.CreatedTo != nil {
			r.Lte(opt.CreatedTo.UnixMilli())
		}
		q.Filter(r)
	}
	if opt.MinStock != nil || opt.MaxStock != nil {
		r := elastic.NewRangeQuery("stock")
		if opt.MinStock != nil {
			r.Gte(*opt.MinStock)
		}
		if opt.MaxStock != nil {
			r.Lte(*opt.MaxStock)
		}
		q.Filter(r)
	}
	if opt.MinPrice != nil || opt.MaxPrice != nil {
		r := elastic.NewRangeQuery("price")
		if opt.MinPrice != nil {
			r.Gte(*opt.MinPrice)
		}
		if opt.MaxPrice != nil {
			r.Lte(*opt.MaxPrice)
		}
		q.Filter(r)
	}
	source := elastic.NewSearchSource().Query(q)
	if opt.Page != nil || opt.PageSize != nil {
		if opt.Page == nil || opt.PageSize == nil {
			return nil, fmt.Errorf("分页参数 page 和 page_size 必须同时提供")
		}
		page, pageSize := *opt.Page, *opt.PageSize
		if page < 1 || pageSize < 1 || pageSize > 100 || page > 10000/pageSize {
			return nil, fmt.Errorf("分页参数无效：page_size 必须为 1–100，分页范围不能超过 10000 条")
		}
		source.From((page - 1) * pageSize).Size(pageSize)
	}
	ascUpdate := false
	if opt.AscUpdate != nil {
		ascUpdate = *opt.AscUpdate
	} else if opt.AscCreate != nil {
		ascUpdate = *opt.AscCreate
	}
	source.Sort("update_time_stamp", ascUpdate)
	return source, nil
}

func ProductFromModel(p *model.Product) *es_model.Product {
	if p == nil {
		return nil
	}
	return &es_model.Product{IsDel: int64(p.IsDel), ID: p.ID, Name: p.Name, Price: p.Price, Stock: p.Stock, Image: p.Image, CreateTime: p.CreateTime, UpdateTime: p.UpdateTime}
}

func IndexProduct(p *es_model.Product) (string, error) {
	return IndexProductContext(context.Background(), p)
}

func IndexProductContext(ctx context.Context, p *es_model.Product) (string, error) {
	doc, err := p.ToMap()
	if err != nil {
		return "", err
	}
	return IndexDocContext(ctx, ES_PRODUCT, fmt.Sprintf("%d", p.ID), doc)
}

// GetProducts 根据传入的可选条件查询商品。
func GetProducts(opt *ProductSearchOptions) ([]*es_model.Product, error) {
	items, _, err := GetProductsPage(context.Background(), opt)
	return items, err
}

// GetProductsPage 优先返回最新商品，创建时间相同时按 ID 保持稳定排序。
func GetProductsPage(ctx context.Context, opt *ProductSearchOptions) ([]*es_model.Product, int64, error) {
	if opt == nil {
		return nil, 0, fmt.Errorf("商品查询参数不能为空")
	}
	c, err := clientReady()
	if err != nil {
		return nil, 0, err
	}
	source, err := BuildProductElasticQuery(opt)
	if err != nil {
		return nil, 0, err
	}
	started := time.Now()
	result, err := c.Search(ES_PRODUCT).SearchSource(source).Sort("id", false).TrackTotalHits(true).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("ES查询失败: %w", err)
	}
	products := make([]*es_model.Product, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		var product es_model.Product
		if err := json.Unmarshal(hit.Source, &product); err != nil {
			return nil, 0, err
		}
		products = append(products, &product)
	}
	fields := []any{"method", "GetProductsPage", "index", ES_PRODUCT, "returned_count", len(products), "total", result.TotalHits(), "elapsed_ms", float64(time.Since(started)) / float64(time.Millisecond)}
	if opt.Page != nil {
		fields = append(fields, "page", *opt.Page)
	}
	if opt.PageSize != nil {
		fields = append(fields, "page_size", *opt.PageSize)
	}
	logger.Info(ctx, "ES查询完成", fields...)
	return products, result.TotalHits(), nil
}

func GetProductByID(id int64) (*es_model.Product, error) {
	return GetProductByIDContext(context.Background(), id)
}

// GetProductByIDContext 使用请求上下文查询商品。
func GetProductByIDContext(ctx context.Context, id int64) (*es_model.Product, error) {
	searchOptions := ProductSearchOptions{
		ID: &id,
	}
	items, _, err := GetProductsPage(ctx, &searchOptions)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("商品不存在: %d", id)
	}
	return items[0], nil
}

// GetProductIDsPage 只从 ES 获取筛选后的商品 ID，详情由业务层回查 MySQL。
func GetProductIDsPage(ctx context.Context, opt *ProductSearchOptions) ([]int64, int64, error) {
	if opt == nil {
		return nil, 0, fmt.Errorf("商品查询参数不能为空")
	}
	c, err := clientReady()
	if err != nil {
		return nil, 0, err
	}
	source, err := BuildProductElasticQuery(opt)
	if err != nil {
		return nil, 0, err
	}
	started := time.Now()
	result, err := c.Search(ES_PRODUCT).SearchSource(source).Sort("id", false).TrackTotalHits(true).FetchSource(false).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("ES查询商品ID失败: %w", err)
	}
	ids := make([]int64, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		id, err := strconv.ParseInt(hit.Id, 10, 64)
		if err != nil || id <= 0 {
			return nil, 0, fmt.Errorf("ES商品文档ID无效: %q", hit.Id)
		}
		ids = append(ids, id)
	}
	fields := []any{"method", "GetProductIDsPage", "index", ES_PRODUCT, "returned_count", len(ids), "total", result.TotalHits(), "elapsed_ms", float64(time.Since(started)) / float64(time.Millisecond)}
	if opt.Page != nil {
		fields = append(fields, "page", *opt.Page)
	}
	if opt.PageSize != nil {
		fields = append(fields, "page_size", *opt.PageSize)
	}
	logger.Info(ctx, "ES查询完成", fields...)
	return ids, result.TotalHits(), nil
}
