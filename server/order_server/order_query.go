package order_server

import (
	"context"
	"errors"
	"time"

	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/es"
	"go-framework-demo/model"
)

type OrderQueryInput struct {
	UserID      int64
	Status      *appconst.OrderStatus
	MinPrice    *int64
	MaxPrice    *int64
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
	Page        int
	PageSize    int
}

type OrderListItem struct {
	ID         int64
	UserID     int64
	OrderNo    string
	Amount     int64
	Status     int32
	CreateTime time.Time
	ExpireTime time.Time
	Items      []*model.OrderItem
}

type OrderPage struct {
	Items    []*OrderListItem
	Total    int64
	Page     int
	PageSize int
}

func (s *OrderService) List(ctx context.Context, input *OrderQueryInput) (*OrderPage, error) {
	if input == nil || input.UserID <= 0 {
		return nil, appconst.NewInvalidParamsError("用户ID必须为正整数")
	}
	return s.listOrders(ctx, input, false)
}

// ListAdmin 管理员可查询全部用户，或指定某个用户。
func (s *OrderService) ListAdmin(ctx context.Context, input *OrderQueryInput) (*OrderPage, error) {
	if input == nil || input.UserID < 0 {
		return nil, appconst.NewInvalidParamsError("用户ID无效")
	}
	return s.listOrders(ctx, input, true)
}

func (s *OrderService) listOrders(ctx context.Context, input *OrderQueryInput, allUsers bool) (*OrderPage, error) {
	if err := ctx.Err(); err != nil {
		return nil, appconst.NewInternalError(err)
	}

	if input.Page < 1 || input.PageSize < 1 || input.PageSize > 100 || input.Page > 10000/input.PageSize {
		return nil, appconst.NewInvalidParamsError("分页范围不能超过10000条，每页1–100条")
	}
	options := es.OrderSearchOptions{
		Status:      input.Status,
		MinPrice:    input.MinPrice,
		MaxPrice:    input.MaxPrice,
		CreatedFrom: input.CreatedFrom,
		CreatedTo:   input.CreatedTo,
		Sort:        input.Sort,
		Page:        &input.Page,
		PageSize:    &input.PageSize,
	}
	if input.UserID > 0 {
		options.UserID = &input.UserID
	}
	if err := es.ValidateOrderSearchOptions(&options); err != nil {
		return nil, appconst.NewInvalidParamsError(err.Error())
	}
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	ids, total, err := es.GetOrderIDsPage(ctx, &options)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	var targets []*model.Order
	if allUsers && input.UserID == 0 {
		targets, err = s.orderDb.GetByIDs(ctx, ids)
	} else {
		targets, err = s.orderDb.GetByIDsForUser(ctx, input.UserID, ids)
	}
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	targetMap := make(map[int64]*model.Order, len(targets))
	detailIDs := make([]int64, 0, len(targets))
	for _, target := range targets {
		if !matchesOrderFilters(target, input) {
			continue
		}
		targetMap[target.ID] = target
		detailIDs = append(detailIDs, target.ID)
	}
	itemMap := make(map[int64][]*model.OrderItem, len(detailIDs))
	if len(detailIDs) > 0 {
		itemsDB, err := db.NewOrderItemDb(ctx, s.genQuery)
		if err != nil {
			return nil, appconst.NewInternalError(err)
		}
		items, err := itemsDB.GetByOrderIDs(ctx, detailIDs)
		if err != nil {
			return nil, appconst.NewInternalError(err)
		}
		for _, item := range items {
			itemMap[item.OrderID] = append(itemMap[item.OrderID], item)
		}
	}
	rows := make([]*OrderListItem, 0, len(targetMap))
	for _, id := range ids {
		target, exists := targetMap[id]
		if !exists {
			continue
		}
		items := itemMap[id]
		if items == nil {
			items = []*model.OrderItem{}
		}
		row := &OrderListItem{
			ID:         target.ID,
			UserID:     target.UserID,
			OrderNo:    target.OrderNo,
			Amount:     target.Amount,
			Status:     target.Status,
			CreateTime: target.CreateTime,
			ExpireTime: target.ExpireTime,
			Items:      items,
		}
		rows = append(rows, row)
	}
	result := &OrderPage{Items: rows, Total: total, Page: input.Page, PageSize: input.PageSize}
	return result, nil
}

// ES 同步可能滞后；MySQL 当前状态不匹配时不进入当前状态 tab。
func matchesOrderFilters(target *model.Order, input *OrderQueryInput) bool {
	if input.Status != nil && target.Status != int32(*input.Status) {
		return false
	}
	if input.MinPrice != nil && target.Amount < *input.MinPrice || input.MaxPrice != nil && target.Amount > *input.MaxPrice {
		return false
	}
	if input.CreatedFrom != nil && target.CreateTime.Before(*input.CreatedFrom) || input.CreatedTo != nil && target.CreateTime.After(*input.CreatedTo) {
		return false
	}
	return true
}
