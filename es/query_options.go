package es

import (
	"fmt"
	"github.com/olivere/elastic/v7"
	appconst "go-framework-demo/const"
	"strconv"
	"time"
)

type UserSearchOptions struct {
	ID        *int64
	Name      *string
	Email     *string
	Page      *int
	PageSize  *int
	AscUpdate *bool
	// AscCreate 保留旧调用方兼容；现在控制更新时间排序方向。
	AscCreate *bool
}

func BuildUserElasticQuery(opt *UserSearchOptions) (*elastic.SearchSource, error) {
	if opt == nil {
		return nil, fmt.Errorf("用户查询参数不能为空")
	}
	q := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("is_del", 0))
	if opt.ID != nil {
		q.Filter(elastic.NewTermQuery("user_id", strconv.FormatInt(*opt.ID, 10)))
	}
	if opt.Name != nil {
		q.Must(elastic.NewMatchQuery("name", *opt.Name))
	}
	if opt.Email != nil {
		q.Filter(elastic.NewTermQuery("email.keyword", *opt.Email))
	}
	source := elastic.NewSearchSource().Query(q)
	ascUpdate := opt.AscUpdate
	if ascUpdate == nil {
		ascUpdate = opt.AscCreate
	}
	if err := applySearchOptions(source, opt.Page, opt.PageSize, ascUpdate); err != nil {
		return nil, err
	}
	return source, nil
}

type OrderSearchOptions struct {
	ID          *int64
	OrderNo     *string
	UserID      *int64
	ProductIDs  *[]int64
	Page        *int
	PageSize    *int
	AscCreate   *bool
	AscUpdate   *bool
	Status      *appconst.OrderStatus
	MinPrice    *int64
	MaxPrice    *int64
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

func BuildOrderElasticQuery(opt *OrderSearchOptions) (*elastic.SearchSource, error) {
	if err := ValidateOrderSearchOptions(opt); err != nil {
		return nil, err
	}
	q := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("is_del", 0))
	if opt.ID != nil {
		q.Filter(elastic.NewIdsQuery().Ids(strconv.FormatInt(*opt.ID, 10)))
	}
	if opt.OrderNo != nil {
		q.Filter(elastic.NewTermQuery("order_no", *opt.OrderNo))
	}
	if opt.UserID != nil {
		q.Filter(elastic.NewTermQuery("user_id", strconv.FormatInt(*opt.UserID, 10)))
	}
	if opt.ProductIDs != nil {
		ids := []interface{}{}
		for _, id := range *opt.ProductIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		q.Filter(elastic.NewNestedQuery("items", elastic.NewBoolQuery().Filter(elastic.NewTermsQuery("items.product_id", ids...), elastic.NewTermQuery("items.is_del", 0))))
	}
	if opt.Status != nil {
		q.Filter(elastic.NewTermQuery("status", *opt.Status))
	}
	if opt.MinPrice != nil || opt.MaxPrice != nil {
		r := elastic.NewRangeQuery("amount_fen")
		if opt.MinPrice != nil {
			r.Gte(*opt.MinPrice)
		}
		if opt.MaxPrice != nil {
			r.Lte(*opt.MaxPrice)
		}
		q.Filter(r)
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
	source := elastic.NewSearchSource().Query(q)
	if opt.Sort == "price_asc" || opt.Sort == "price_desc" {
		source.Sort("amount_fen", opt.Sort == "price_asc")
	}
	var ascUpdate *bool
	ascUpdate = opt.AscUpdate
	if ascUpdate == nil {
		ascUpdate = opt.AscCreate
	}
	if opt.Sort == "updated_asc" {
		ascending := true
		ascUpdate = &ascending
	} else if opt.Sort == "updated_desc" || opt.Sort == "newest" {
		ascending := false
		ascUpdate = &ascending
	}
	if err := applySearchOptions(source, opt.Page, opt.PageSize, ascUpdate); err != nil {
		return nil, err
	}
	return source, nil
}

// applySearchOptions 添加可选分页参数及更新时间排序。
func applySearchOptions(source *elastic.SearchSource, page, pageSize *int, ascCreate *bool) error {
	if page != nil || pageSize != nil {
		if page == nil || pageSize == nil {
			return fmt.Errorf("分页参数 page 和 page_size 必须同时提供")
		}
		if *page < 1 || *pageSize < 1 || *pageSize > 100 || *page > 10000 / *pageSize {
			return fmt.Errorf("分页参数无效：page_size 必须为 1–100，分页范围不能超过 10000 条")
		}
		source.From((*page - 1) * *pageSize).Size(*pageSize)
	}
	ascending := false
	if ascCreate != nil {
		ascending = *ascCreate
	}
	source.Sort("update_time_stamp", ascending)
	return nil
}
