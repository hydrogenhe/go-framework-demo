package http_handler

import (
	"fmt"
	"go-framework-demo/http_handler/http_msg"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/server/product_server"
)

// parseProductQuery 统一解析列表和导出筛选条件，空字段不参与筛选。
func parseProductQuery(c *gin.Context, paginated bool) (*product_server.ProductQueryInput, error) {
	var req http_msg.ProductQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, err
	}
	input := &product_server.ProductQueryInput{}
	input.Sort = queryParamDefault(req.Sort, "updated_desc")
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name != "" {
			input.Name = &name
		}
	}
	if req.PriceMin != nil && *req.PriceMin != "" {
		value, err := strconv.ParseInt(*req.PriceMin, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("price_min 必须为整数分")
		}
		input.MinPrice = &value
	}
	if req.PriceMax != nil && *req.PriceMax != "" {
		value, err := strconv.ParseInt(*req.PriceMax, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("price_max 必须为整数分")
		}
		input.MaxPrice = &value
	}
	if req.StockMin != nil && *req.StockMin != "" {
		value, err := strconv.ParseInt(*req.StockMin, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("stock_min 必须为有效库存整数")
		}
		stock := int32(value)
		input.MinStock = &stock
	}
	if req.StockMax != nil && *req.StockMax != "" {
		value, err := strconv.ParseInt(*req.StockMax, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("stock_max 必须为有效库存整数")
		}
		stock := int32(value)
		input.MaxStock = &stock
	}
	if req.CreatedFrom != nil && *req.CreatedFrom != "" {
		value, err := time.Parse(time.RFC3339Nano, *req.CreatedFrom)
		if err != nil {
			return nil, fmt.Errorf("created_from 必须为带时区的日期时间")
		}
		input.CreatedFrom = &value
	}
	if req.CreatedTo != nil && *req.CreatedTo != "" {
		value, err := time.Parse(time.RFC3339Nano, *req.CreatedTo)
		if err != nil {
			return nil, fmt.Errorf("created_to 必须为带时区的日期时间")
		}
		input.CreatedTo = &value
	}
	if paginated {
		page, err := strconv.Atoi(queryParamDefault(req.Page, "1"))
		if err != nil {
			return nil, fmt.Errorf("page 必须为整数")
		}
		pageSize, err := strconv.Atoi(queryParamDefault(req.PageSize, "20"))
		if err != nil {
			return nil, fmt.Errorf("page_size 必须为整数")
		}
		input.Page, input.PageSize = page, pageSize
	}
	return input, nil
}

// productQueryDefault 仅在参数未传时使用默认值；显式传空串保留为空，让后续校验处理。
func queryParamDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

// exportProducts 将按筛选条件生成的 Excel 作为附件发送，结束后回收临时文件。
// 导出商品Excel。
func (h routeHandlers) exportProducts(c *gin.Context) {
	ctx := c.Request.Context()
	input, err := parseProductQuery(c, false)
	if err != nil {
		writeError(c, appconst.NewInvalidParamsError(err.Error()))
		return
	}
	result, err := h.productService.Export(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	defer os.Remove(result.Path)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Export-Count", strconv.Itoa(result.Rows))
	c.FileAttachment(result.Path, "products-"+time.Now().Format("20060102-150405")+".xlsx")
}

// ExportProductsHandler 返回商品导出处理函数，支持测试注入业务服务。
func ExportProductsHandler(service *product_server.ProductService) gin.HandlerFunc {
	return (routeHandlers{productService: service}).exportProducts
}
