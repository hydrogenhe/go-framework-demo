package http_handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/model"
	"go-framework-demo/server"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/product_server"
)

func writeSuccess[T any](c *gin.Context, status int, data T) {
	response := http_msg.Response[T]{Code: appconst.CodeOK, Message: "success", Data: data}
	c.JSON(status, response)
}

func productResp(p *model.Product) http_msg.ProductResp {
	response := http_msg.ProductResp{
		ID:         strconv.FormatInt(p.ID, 10),
		Version:    p.CurrentVersion,
		Name:       p.Name,
		Price:      p.Price,
		Stock:      p.Stock,
		Image:      p.Image,
		CreateTime: p.CreateTime,
		UpdateTime: p.UpdateTime,
	}
	return response
}

func productPageResp(p *product_server.ProductPage) http_msg.ProductPageResp {
	response := http_msg.ProductPageResp{
		Items:    make([]http_msg.ProductResp, 0, len(p.Items)),
		Total:    p.Total,
		Page:     p.Page,
		PageSize: p.PageSize,
	}
	for _, item := range p.Items {
		response.Items = append(response.Items, productResp(item))
	}
	return response
}

func orderResp(o *model.Order) http_msg.OrderResp {
	response := http_msg.OrderResp{
		ID:         strconv.FormatInt(o.ID, 10),
		UserID:     strconv.FormatInt(o.UserID, 10),
		OrderNo:    o.OrderNo,
		Amount:     o.Amount,
		Status:     o.Status,
		CreateTime: o.CreateTime,
		ExpireTime: o.ExpireTime,
		UpdateTime: o.UpdateTime,
	}
	return response
}

func orderItemResp(o *model.OrderItem) http_msg.OrderItemResp {
	response := http_msg.OrderItemResp{
		ID:             strconv.FormatInt(o.ID, 10),
		OrderID:        strconv.FormatInt(o.OrderID, 10),
		ProductID:      strconv.FormatInt(o.ProductID, 10),
		ProductVersion: o.ProductVersion,
		Quantity:       o.Quantity,
		UnitPriceFen:   o.UnitPriceFen,
		Status:         o.Status,
		Image:          o.Image,
		ProductName:    o.ProductName,
		UpdateTime:     o.UpdateTime,
	}
	return response
}

func orderPageResp(p *order_server.OrderPage) http_msg.OrderPageResp {
	response := http_msg.OrderPageResp{
		Items:    make([]http_msg.OrderListResp, 0, len(p.Items)),
		Total:    p.Total,
		Page:     p.Page,
		PageSize: p.PageSize,
	}
	for _, o := range p.Items {
		row := http_msg.OrderListResp{
			ID:         strconv.FormatInt(o.ID, 10),
			UserID:     strconv.FormatInt(o.UserID, 10),
			OrderNo:    o.OrderNo,
			Amount:     o.Amount,
			Status:     o.Status,
			CreateTime: o.CreateTime,
			ExpireTime: o.ExpireTime,
			Items:      make([]http_msg.OrderItemResp, 0, len(o.Items)),
		}
		for _, item := range o.Items {
			row.Items = append(row.Items, orderItemResp(item))
		}
		response.Items = append(response.Items, row)
	}
	return response
}

func logResp(p *server.LogResult) http_msg.LogQueryResp {
	response := http_msg.LogQueryResp{
		Keyword:  p.Keyword,
		Level:    p.Level,
		Date:     p.Date,
		File:     p.File,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    p.Total,
		Items:    make([]http_msg.LogEntryResp, 0, len(p.Items)),
	}
	for _, e := range p.Items {
		row := http_msg.LogEntryResp{
			LogID:   e.LogID,
			Line:    e.Line,
			Time:    e.Time,
			Level:   e.Level,
			Message: e.Message,
			Fields:  make([]http_msg.LogFieldResp, 0, len(e.Fields)),
		}
		for _, f := range e.Fields {
			row.Fields = append(row.Fields, http_msg.LogFieldResp{Key: f.Key, Label: f.Label, Value: f.Value})
		}
		response.Items = append(response.Items, row)
	}
	return response
}

// 显式空参数保留为空，由后续校验报错；只有缺省参数使用默认值。
func queryDefault(c *gin.Context, key, value, fallback string) string {
	if _, ok := c.GetQueryArray(key); !ok {
		return fallback
	}
	return value
}
