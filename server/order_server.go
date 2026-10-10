package server

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go-framework-demo/db"
	"go-framework-demo/model"
)

// OrderService 订单业务逻辑层。
// 负责参数校验，并复用 OrderDb 完成订单数据访问。
type OrderService struct {
	orderDb *db.OrderDb
}

// OrderItemInput 创建订单时提交的商品明细。
type OrderItemInput struct {
	ProductID    int64
	Quantity     int32
	UnitPriceFen int64
}

// CreateOrderWithItems 在一个事务中创建订单及其商品明细。
func (s *OrderService) CreateOrderWithItems(ctx context.Context, orderNo string, userID int64, inputs []OrderItemInput) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, errors.New("order service is not initialized")
	}
	if orderNo == "" || userID <= 0 || len(inputs) == 0 {
		return nil, errors.New("invalid order params: orderNo, userID and items are required")
	}
	items := make([]*model.OrderItem, 0, len(inputs))
	for _, input := range inputs {
		items = append(items, &model.OrderItem{
			ProductID: input.ProductID, Quantity: input.Quantity, UnitPriceFen: input.UnitPriceFen,
		})
	}
	order, err := s.orderDb.CreateWithItems(orderNo, userID, items)
	if err != nil {
		log.Printf("创建订单及明细失败: orderNo=%s, err=%v", orderNo, err)
		return nil, fmt.Errorf("create order with items failed: %w", err)
	}
	return order, nil
}

// NewOrderService 初始化订单服务。
func NewOrderService(ctx context.Context) *OrderService {
	orderDb, err := db.NewOrderDb(ctx)
	if err != nil {
		log.Printf("初始化OrderDb失败: %v", err)
		return &OrderService{}
	}

	return &OrderService{orderDb: orderDb}
}

// CreateOrder 创建订单。
// amountFen 使用分作为单位，避免浮点数带来的金额精度问题。
func (s *OrderService) CreateOrder(ctx context.Context, orderNo string, userID int64, amountFen int64) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, errors.New("order service is not initialized")
	}
	if orderNo == "" || userID <= 0 || amountFen <= 0 {
		return nil, errors.New("invalid order params: orderNo can't be empty, userID and amountFen must be positive")
	}

	order, err := s.orderDb.Create(orderNo, userID, amountFen)
	if err != nil {
		log.Printf("DB创建订单失败: orderNo=%s, err=%v", orderNo, err)
		return nil, fmt.Errorf("create order db failed: %w", err)
	}
	return order, nil
}

// GetOrderByID 根据订单ID查询订单。
func (s *OrderService) GetOrderByID(ctx context.Context, orderID int64) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, errors.New("order service is not initialized")
	}
	if orderID <= 0 {
		return nil, errors.New("invalid order id: id must be positive")
	}

	order, err := s.orderDb.GetByID(orderID)
	if err != nil {
		log.Printf("DB查询订单失败: orderID=%d, err=%v", orderID, err)
		return nil, fmt.Errorf("get order db failed: %w", err)
	}
	return order, nil
}

// GetOrderByOrderNo 根据订单号查询订单。
func (s *OrderService) GetOrderByOrderNo(ctx context.Context, orderNo string) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, errors.New("order service is not initialized")
	}
	if orderNo == "" {
		return nil, errors.New("invalid order number: orderNo can't be empty")
	}

	order, err := s.orderDb.GetByOrderNo(orderNo)
	if err != nil {
		log.Printf("DB查询订单失败: orderNo=%s, err=%v", orderNo, err)
		return nil, fmt.Errorf("get order by number db failed: %w", err)
	}
	return order, nil
}

// GetOrdersByUserID 查询用户的全部订单。
func (s *OrderService) GetOrdersByUserID(ctx context.Context, userID int64) ([]*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, errors.New("order service is not initialized")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id: id must be positive")
	}

	orders, err := s.orderDb.GetByUserID(userID)
	if err != nil {
		log.Printf("DB查询用户订单失败: userID=%d, err=%v", userID, err)
		return nil, fmt.Errorf("get user orders db failed: %w", err)
	}
	return orders, nil
}

// UpdateOrder 更新订单信息。
func (s *OrderService) UpdateOrder(ctx context.Context, orderID int64, updateData map[string]interface{}) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, errors.New("order service is not initialized")
	}
	if orderID <= 0 || len(updateData) == 0 {
		return nil, errors.New("invalid update params: id must be positive, updateData can't be empty")
	}

	order, err := s.orderDb.Update(orderID, updateData)
	if err != nil {
		log.Printf("DB更新订单失败: orderID=%d, err=%v", orderID, err)
		return nil, fmt.Errorf("update order db failed: %w", err)
	}
	return order, nil
}

// DeleteOrder 删除订单。
func (s *OrderService) DeleteOrder(ctx context.Context, orderID int64) (bool, error) {
	if s == nil || s.orderDb == nil {
		return false, errors.New("order service is not initialized")
	}
	if orderID <= 0 {
		return false, errors.New("invalid order id: id must be positive")
	}

	ok, err := s.orderDb.Delete(orderID)
	if err != nil {
		log.Printf("DB删除订单失败: orderID=%d, err=%v", orderID, err)
		return false, fmt.Errorf("delete order db failed: %w", err)
	}
	if !ok {
		return false, errors.New("order was not deleted")
	}
	return true, nil
}

// BatchCreateOrders 批量创建订单，并由 DB 层保证事务一致性。
func (s *OrderService) BatchCreateOrders(ctx context.Context, orders []*model.Order) error {
	if s == nil || s.orderDb == nil {
		return errors.New("order service is not initialized")
	}
	if len(orders) == 0 {
		return errors.New("orders can't be empty")
	}

	if err := s.orderDb.BatchCreateOrders(orders); err != nil {
		log.Printf("DB批量创建订单失败: err=%v", err)
		return fmt.Errorf("batch create orders db failed: %w", err)
	}
	return nil
}
