package product_server

import (
	appconst "go-framework-demo/const"
	"go-framework-demo/es"
	"time"
)

// ProductQueryInput 列表与导出共用筛选条件；价格使用整数分，时间范围包含端点。
type ProductQueryInput struct {
	Name        *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	MinStock    *int32
	MaxStock    *int32
	MinPrice    *int64
	MaxPrice    *int64
	Page        int
	PageSize    int
	Sort        string
}

func buildProductSearchOptions(input *ProductQueryInput, paginated bool) (*es.ProductSearchOptions, error) {
	if input == nil {
		return nil, appconst.NewInvalidParamsError("商品查询参数不能为空")
	}
	options := &es.ProductSearchOptions{
		Name:        input.Name,
		CreatedFrom: input.CreatedFrom,
		CreatedTo:   input.CreatedTo,
		MinStock:    input.MinStock,
		MaxStock:    input.MaxStock,
		MinPrice:    input.MinPrice,
		MaxPrice:    input.MaxPrice,
	}
	if input.Sort == "updated_asc" {
		ascending := true
		options.AscUpdate = &ascending
	} else if input.Sort != "" && input.Sort != "updated_desc" {
		return nil, appconst.NewInvalidParamsError("商品排序必须为 updated_desc 或 updated_asc")
	}
	if err := es.ValidateProductSearchOptions(options); err != nil {
		return nil, appconst.NewInvalidParamsError(err.Error())
	}
	if paginated {
		if input.Page < 1 || input.PageSize < 1 || input.PageSize > 100 || input.Page > 10000/input.PageSize {
			return nil, appconst.NewInvalidParamsError("分页参数无效：page_size 必须为1–100，分页范围不能超过10000条")
		}
		options.Page, options.PageSize = &input.Page, &input.PageSize
	}
	return options, nil
}
