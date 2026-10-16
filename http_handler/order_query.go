package http_handler

import (
	"fmt"
	"go-framework-demo/http_handler/http_msg"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appconst "go-framework-demo/const"
	"go-framework-demo/server/order_server"
)

func bindOrderQuery(c *gin.Context) (*http_msg.OrderQueryReq, error) {
	var req http_msg.OrderQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func parseOrderQuery(req *http_msg.OrderQueryReq, userID int64) (*order_server.OrderQueryInput, error) {
	if req == nil {
		return nil, fmt.Errorf("订单查询参数不能为空")
	}
	input := &order_server.OrderQueryInput{UserID: userID, Sort: queryParamDefault(req.Sort, "updated_desc")}
	if req.Status != nil {
		value, err := strconv.ParseInt(strings.TrimSpace(*req.Status), 10, 32)
		if err != nil || value < 1 || value > 4 {
			return nil, fmt.Errorf("status 必须为1、2、3或4")
		}
		status := appconst.OrderStatus(value)
		input.Status = &status
	}
	if req.PriceMin != nil {
		value, err := strconv.ParseInt(strings.TrimSpace(*req.PriceMin), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("price_min 必须为整数分")
		}
		input.MinPrice = &value
	}
	if req.PriceMax != nil {
		value, err := strconv.ParseInt(strings.TrimSpace(*req.PriceMax), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("price_max 必须为整数分")
		}
		input.MaxPrice = &value
	}
	if req.CreatedFrom != nil {
		value, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*req.CreatedFrom))
		if err != nil {
			return nil, fmt.Errorf("created_from 必须为带时区的日期时间")
		}
		input.CreatedFrom = &value
	}
	if req.CreatedTo != nil {
		value, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*req.CreatedTo))
		if err != nil {
			return nil, fmt.Errorf("created_to 必须为带时区的日期时间")
		}
		input.CreatedTo = &value
	}
	page, err := strconv.Atoi(queryParamDefault(req.Page, "1"))
	if err != nil {
		return nil, fmt.Errorf("page 必须为整数")
	}
	pageSize, err := strconv.Atoi(queryParamDefault(req.PageSize, "20"))
	if err != nil {
		return nil, fmt.Errorf("page_size 必须为整数")
	}
	input.Page, input.PageSize = page, pageSize
	return input, nil
}

func parseOrderUserID(value *string, required bool) (int64, error) {
	if value == nil {
		if required {
			return 0, fmt.Errorf("user_id 不能为空")
		}
		return 0, nil
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(*value), 10, 64)
	if err != nil || userID <= 0 {
		return 0, fmt.Errorf("用户ID必须为正整数")
	}
	return userID, nil
}
