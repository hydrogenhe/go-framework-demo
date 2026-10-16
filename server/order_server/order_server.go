package order_server

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/logger"

	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/redis"
	"go-framework-demo/rocketmq"
	rocketmessage "go-framework-demo/rocketmq/message"
	"go-framework-demo/server/product_server"
	"sort"
	"time"
	"unicode/utf8"

	mysqlerr "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// OrderServiceInterface 列出订单业务的全部公开方法。
type OrderServiceInterface interface {
	GetAdminOrderUser(ctx context.Context, orderID int64) (*OrderUserDetail, error)
	GetItemSnapshot(ctx context.Context, orderID, itemID, userID int64) (*model.OrderItem, error)
	GetAdminItemSnapshot(ctx context.Context, orderID, itemID int64) (*model.OrderItem, error)
	ListAdmin(ctx context.Context, input *OrderQueryInput) (*OrderPage, error)
	PayOrder(ctx context.Context, orderID, userID int64) (*model.Order, error)
	ConfirmReceipt(ctx context.Context, orderID, userID int64) (*model.Order, error)
	ShipOrderItem(ctx context.Context, orderID, itemID int64) (*model.OrderItem, error)
	List(ctx context.Context, input *OrderQueryInput) (*OrderPage, error)
	CreateOrderWithItems(ctx context.Context, orderNo string, userID int64, inputs []OrderItemInput) (*model.Order, error)
	CreateOrder(ctx context.Context, orderNo string, userID int64, amountFen int64) (*model.Order, error)
	GetOrderByID(ctx context.Context, orderID int64) (*model.Order, error)
	GetOrderByOrderNo(ctx context.Context, orderNo string) (*model.Order, error)
	GetOrdersByUserID(ctx context.Context, userID int64) ([]*model.Order, error)
	UpdateOrder(ctx context.Context, orderID int64, input *OrderUpdateInput) (*model.Order, error)
	DeleteOrder(ctx context.Context, orderID int64) (bool, error)
	BatchCreateOrders(ctx context.Context, orders []*model.Order) error
	MarkShipped(ctx context.Context, orderID int64) (*model.Order, error)
}

// OrderService 订单业务逻辑层。
// 负责业务校验、金额计算和事务编排，通过 DB 层访问数据。
type OrderService struct {
	genQuery *query.Query
	orderDb  *db.OrderDb
}

// NewOrderService 初始化订单服务。
func NewOrderService(ctx context.Context, txQ ...*query.Query) *OrderService {
	orderDb, err := db.NewOrderDb(ctx, txQ...)
	if err != nil {
		logger.Error(ctx, "初始化OrderDb失败", "error", err)
		return &OrderService{}
	}

	return &OrderService{genQuery: orderDb.GenQuery, orderDb: orderDb}
}

var _ OrderServiceInterface = (*OrderService)(nil)

// OrderItemInput 创建订单时提交的商品明细。
type OrderItemInput struct {
	ProductID      int64
	ProductVersion int32
	Quantity       int32
	UnitPriceFen   int64
	Name           *string
	Image          *string
	Stock          *int32
}

var errDuplicateOrderKey = errors.New("duplicate user order key")

type preparedOrder struct {
	sortedInputs []OrderItemInput
	productsByID map[int64]*model.Product
	items        []*model.OrderItem
	amount       int64
}

// CreateOrderWithItems 在一个事务中创建订单及其商品明细。
func (s *OrderService) CreateOrderWithItems(ctx context.Context, orderNo string, userID int64, inputs []OrderItemInput) (*model.Order, error) {
	if err := s.validateCreateOrderRequest(orderNo, userID, inputs); err != nil {
		return nil, err
	}
	if existing, found, err := s.findIdempotentOrder(ctx, orderNo, userID, inputs); err != nil {
		return nil, err
	} else if found {
		return existing, nil
	}
	timeoutMinutes := redis.GetOrderPaymentTimeoutMinutes(ctx)

	prepared, err := s.prepareOrder(ctx, orderNo, userID, inputs)
	if err != nil {
		return nil, err
	}
	order := &model.Order{
		OrderNo: orderNo,
		UserID:  userID,
		Amount:  prepared.amount,
		Status:  int32(model.OrderStatusPendingPayment),
	}
	order.CreateTime = time.Now().Truncate(time.Millisecond)
	order.ExpireTime = order.CreateTime.Add(time.Duration(timeoutMinutes) * time.Minute)
	err = db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		products, err := db.NewProductDb(ctx, txQ)
		if err != nil {
			return err
		}
		orders, err := db.NewOrderDb(ctx, txQ)
		if err != nil {
			return err
		}
		itemDB, err := db.NewOrderItemDb(ctx, txQ)
		if err != nil {
			return err
		}
		for _, input := range prepared.sortedInputs {
			product := prepared.productsByID[input.ProductID]
			affected, err := products.DeductStock(ctx, input.ProductID, input.Quantity, product.CurrentVersion)
			if err != nil {
				logger.Error(ctx, "扣减商品库存失败", "product_id", input.ProductID, "error", err)
				return err
			}
			if affected != 1 {
				return appconst.NewConflictError("商品资料或库存已变化，请刷新页面再试")
			}
		}
		if _, err := orders.Create(ctx, order); err != nil {
			logger.Error(ctx, "写入订单失败", "order_id", order.ID, "error", err)
			if isDuplicateOrderKey(err) {
				return fmt.Errorf("%w: %v", errDuplicateOrderKey, err)
			}
			return orderServiceError(err)
		}
		for _, item := range prepared.items {
			item.OrderID = order.ID
		}
		if err := itemDB.BatchCreate(ctx, prepared.items); err != nil {
			logger.Error(ctx, "批量写入订单明细失败", "order_id", order.ID, "item_count", len(prepared.items), "error", err)
			return err
		}
		return nil
	}, s.genQuery)
	if err != nil {
		logger.Error(ctx, "创建订单及明细失败", "order_no", orderNo, "error", err)
		if errors.Is(err, errDuplicateOrderKey) {
			existing, found, lookupErr := s.findIdempotentOrder(ctx, orderNo, userID, inputs)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if found {
				return existing, nil
			}
			return nil, appconst.NewConflictError("订单号已被其他请求使用")
		}
		return nil, appconst.NewInternalError(err)
	}
	s.publishOrderCreated(ctx, order, prepared.items)
	s.scheduleOrderExpiration(ctx, order)
	return order, nil
}

// findIdempotentOrder 只复用同一用户、订单号和商品明细完全一致的订单。
func (s *OrderService) findIdempotentOrder(ctx context.Context, orderNo string, userID int64, inputs []OrderItemInput) (*model.Order, bool, error) {
	order, err := s.orderDb.GetByOrderNoForUser(ctx, userID, orderNo)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, appconst.NewInternalError(err)
	}
	itemsDB, err := db.NewOrderItemDb(ctx, s.genQuery)
	if err != nil {
		return nil, false, appconst.NewInternalError(err)
	}
	items, err := itemsDB.GetByOrderID(ctx, order.ID)
	if err != nil {
		return nil, false, appconst.NewInternalError(err)
	}
	if !sameOrderRequest(order, items, inputs) {
		return nil, false, appconst.NewConflictError("订单号已用于不同的下单内容")
	}
	return order, true, nil
}

// sameOrderRequest 对比重试请求和原订单中持久化的商品快照。
func sameOrderRequest(order *model.Order, items []*model.OrderItem, inputs []OrderItemInput) bool {
	if order == nil || len(items) != len(inputs) {
		return false
	}
	byProductID := make(map[int64]*model.OrderItem, len(items))
	for _, item := range items {
		if item == nil {
			return false
		}
		if _, exists := byProductID[item.ProductID]; exists {
			return false
		}
		byProductID[item.ProductID] = item
	}
	var amount int64
	for _, input := range inputs {
		item, exists := byProductID[input.ProductID]
		if !exists || item.Quantity != input.Quantity || item.UnitPriceFen != input.UnitPriceFen ||
			(input.ProductVersion > 0 && item.ProductVersion != input.ProductVersion) {
			return false
		}
		if (input.Name != nil && item.ProductName != *input.Name) || (input.Image != nil && item.Image != *input.Image) {
			return false
		}
		// Stock 是扣减前的页面快照，不保存在订单明细中；幂等身份由商品、数量、价格及可选展示快照决定。
		price := input.UnitPriceFen
		quantity := int64(input.Quantity)
		if price <= 0 || quantity <= 0 || quantity > int64(^uint64(0)>>1)/price {
			return false
		}
		cost := quantity * price
		if amount > int64(^uint64(0)>>1)-cost {
			return false
		}
		amount += cost
	}
	return order.Amount == amount
}

func isDuplicateOrderKey(err error) bool {
	var duplicate *mysqlerr.MySQLError
	return errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &duplicate) && duplicate.Number == 1062
}

func (s *OrderService) prepareOrder(ctx context.Context, orderNo string, userID int64, inputs []OrderItemInput) (*preparedOrder, error) {
	// 按主键排序，多个商品下单时保持一致的扣减顺序，降低数据库死锁概率。
	sortedInputs := append([]OrderItemInput(nil), inputs...)
	sort.Slice(sortedInputs, func(i, j int) bool { return sortedInputs[i].ProductID < sortedInputs[j].ProductID })
	products, err := db.NewProductDb(ctx, s.genQuery)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	productIDs := make([]int64, 0, len(sortedInputs))
	for _, input := range sortedInputs {
		productIDs = append(productIDs, input.ProductID)
	}
	queriedProducts, err := products.GetByIDs(ctx, productIDs)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	productsByID := make(map[int64]*model.Product, len(queriedProducts))
	for _, product := range queriedProducts {
		productsByID[product.ID] = product
	}
	prepared := &preparedOrder{
		sortedInputs: sortedInputs,
		productsByID: productsByID,
		items:        make([]*model.OrderItem, 0, len(inputs)),
	}
	for _, input := range sortedInputs {
		product, ok := productsByID[input.ProductID]
		if !ok {
			return nil, appconst.NewConflictError("数据变化，请刷新页面")
		}
		if product.Price != input.UnitPriceFen ||
			(input.ProductVersion > 0 && input.ProductVersion != product.CurrentVersion) ||
			(input.Name != nil && *input.Name != product.Name) ||
			(input.Image != nil && *input.Image != product.Image) ||
			(input.Stock != nil && *input.Stock != product.Stock) {
			return nil, appconst.NewConflictError("数据变化，请刷新页面")
		}
		if err := product_server.ValidateProduct(product); err != nil {
			return nil, appconst.NewConflictError("数据变化，请刷新页面")
		}
		if product.Stock < input.Quantity {
			return nil, appconst.NewConflictError("商品库存不足，请刷新后重试")
		}
		prepared.amount += int64(input.Quantity) * product.Price
		prepared.items = append(prepared.items, &model.OrderItem{
			ProductID:      product.ID,
			ProductVersion: product.CurrentVersion,
			ProductName:    product.Name,
			Image:          product.Image,
			Quantity:       input.Quantity,
			UnitPriceFen:   product.Price,
			Status:         int32(model.OrderItemStatusPendingShipment),
		})
	}
	return prepared, nil
}

func (s *OrderService) validateCreateOrderRequest(orderNo string, userID int64, inputs []OrderItemInput) error {
	if err := validateOrderNo(orderNo); err != nil {
		return err
	}
	if orderNo == "" || userID <= 0 || len(inputs) == 0 || len(inputs) > 100 {
		return appconst.NewInvalidParamsError("订单号、用户ID和订单明细不能为空且明细不能超过100条")
	}
	seen := make(map[int64]bool, len(inputs))
	for _, input := range inputs {
		if input.ProductID <= 0 || input.ProductVersion < 0 || input.Quantity <= 0 || input.UnitPriceFen <= 0 {
			return appconst.NewInvalidParamsError("订单明细参数无效")
		}
		if seen[input.ProductID] {
			return appconst.NewInvalidParamsError("同一商品不能重复提交")
		}
		seen[input.ProductID] = true
	}
	if s == nil || s.orderDb == nil {
		return appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	return nil
}

func (s *OrderService) publishOrderCreated(ctx context.Context, order *model.Order, items []*model.OrderItem) {
	if order == nil {
		return
	}
	message, err := rocketmessage.NewOrderMessage(
		rocketmessage.ActionCreate,
		fmt.Sprintf("%d", order.ID),
		fmt.Sprintf("%d", order.UserID),
		order.Amount,
	)
	if err != nil {
		logger.Error(ctx, "构建订单消息失败", "order_id", order.ID, "error", err)
		return
	}
	message.Status = order.Status
	message.Items = make([]rocketmessage.OrderItemMessage, 0, len(items))
	for _, item := range items {
		messageItem := rocketmessage.OrderItemMessage{
			ProductID:    fmt.Sprintf("%d", item.ProductID),
			ProductName:  item.ProductName,
			Image:        item.Image,
			Quantity:     item.Quantity,
			UnitPriceFen: item.UnitPriceFen,
		}
		message.Items = append(message.Items, messageItem)
	}
	body, err := message.ToJSON()
	if err != nil {
		logger.Error(ctx, "序列化订单消息失败", "order_id", order.ID, "error", err)
		return
	}
	if _, err := rocketmq.OrderProducerSendMsg(ctx, []byte(body)); err != nil {
		logger.Error(ctx, "发送订单消息失败", "order_id", order.ID, "error", err)
	}
}

// CreateOrder 创建订单。
// amountFen 使用分作为单位，避免浮点数带来的金额精度问题。
func (s *OrderService) CreateOrder(ctx context.Context, orderNo string, userID int64, amountFen int64) (*model.Order, error) {
	if err := validateOrderNo(orderNo); err != nil {
		return nil, err
	}
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderNo == "" || userID <= 0 || amountFen <= 0 {
		return nil, appconst.NewInvalidParamsError("订单号不能为空，用户ID和金额必须为正整数")
	}
	timeoutMinutes := redis.GetOrderPaymentTimeoutMinutes(ctx)

	input := &model.Order{
		OrderNo: orderNo,
		UserID:  userID,
		Amount:  amountFen,
		Status:  int32(model.OrderStatusPendingPayment),
	}
	input.CreateTime = time.Now().Truncate(time.Millisecond)
	input.ExpireTime = input.CreateTime.Add(time.Duration(timeoutMinutes) * time.Minute)
	order, err := s.orderDb.Create(ctx, input)
	if err != nil {
		logger.Error(ctx, "DB创建订单失败", "order_no", orderNo, "error", err)
		return nil, orderServiceError(err)
	}
	s.scheduleOrderExpiration(ctx, order)
	return order, nil
}

// GetOrderByID 根据订单ID查询订单。
func (s *OrderService) GetOrderByID(ctx context.Context, orderID int64) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单ID必须为正整数")
	}

	order, err := s.orderDb.GetByID(ctx, orderID)
	if err != nil {
		logger.Error(ctx, "DB查询订单失败", "order_id", orderID, "error", err)
		return nil, orderServiceError(err)
	}
	return order, nil
}

// GetOrderByOrderNo 根据订单号查询订单。
func (s *OrderService) GetOrderByOrderNo(ctx context.Context, orderNo string) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderNo == "" {
		return nil, appconst.NewInvalidParamsError("订单号不能为空")
	}

	order, err := s.orderDb.GetByOrderNo(ctx, orderNo)
	if err != nil {
		logger.Error(ctx, "DB查询订单失败", "order_no", orderNo, "error", err)
		return nil, orderServiceError(err)
	}
	return order, nil
}

// GetOrdersByUserID 查询用户的全部订单。
func (s *OrderService) GetOrdersByUserID(ctx context.Context, userID int64) ([]*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if userID <= 0 {
		return nil, appconst.NewInvalidParamsError("用户ID必须为正整数")
	}

	orders, err := s.orderDb.GetByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "DB查询用户订单失败", "user_id", userID, "error", err)
		return nil, orderServiceError(err)
	}
	return orders, nil
}

// UpdateOrder 更新订单信息。
func (s *OrderService) UpdateOrder(ctx context.Context, orderID int64, input *OrderUpdateInput) (*model.Order, error) {
	if s == nil || s.orderDb == nil {
		return nil, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderID <= 0 {
		return nil, appconst.NewInvalidParamsError("订单ID必须为正整数")
	}
	if err := ValidateOrderUpdate(input); err != nil {
		return nil, err
	}
	order, err := s.orderDb.Update(ctx, orderID, input)
	if err != nil {
		logger.Error(ctx, "DB更新订单失败", "order_id", orderID, "error", err)
		return nil, orderServiceError(err)
	}
	return order, nil
}

// DeleteOrder 删除订单。
func (s *OrderService) DeleteOrder(ctx context.Context, orderID int64) (bool, error) {
	if s == nil || s.orderDb == nil {
		return false, appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if orderID <= 0 {
		return false, appconst.NewInvalidParamsError("订单ID必须为正整数")
	}

	ok, err := s.orderDb.Delete(ctx, orderID)
	if err != nil {
		logger.Error(ctx, "DB删除订单失败", "order_id", orderID, "error", err)
		return false, orderServiceError(err)
	}
	if !ok {
		return false, appconst.NewNotFoundError("订单不存在")
	}
	return true, nil
}

// BatchCreateOrders 校验订单，并在业务层事务中批量写入。
func (s *OrderService) BatchCreateOrders(ctx context.Context, orders []*model.Order) error {
	if s == nil || s.orderDb == nil {
		return appconst.NewInternalError(errors.New("order service is not initialized"))
	}
	if len(orders) == 0 {
		return appconst.NewInvalidParamsError("订单列表不能为空")
	}
	timeoutMinutes := redis.GetOrderPaymentTimeoutMinutes(ctx)

	for _, order := range orders {
		if order == nil || order.OrderNo == "" || order.UserID <= 0 || order.Amount <= 0 {
			return appconst.NewInvalidParamsError("订单参数无效")
		}
		if err := validateOrderNo(order.OrderNo); err != nil {
			return err
		}
		if order.Status == 0 {
			order.Status = int32(model.OrderStatusPendingPayment)
		}
		if order.Status < int32(model.OrderStatusPendingPayment) || order.Status > int32(model.OrderStatusCompleted) {
			return appconst.NewInvalidParamsError("订单状态无效")
		}
		createdAt := time.Now()
		if order.CreateTime.IsZero() {
			order.CreateTime = createdAt
		}
		order.ExpireTime = order.CreateTime.Add(time.Duration(timeoutMinutes) * time.Minute)
	}
	if err := db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		ordersDB, err := db.NewOrderDb(ctx, txQ)
		if err != nil {
			return err
		}
		return ordersDB.BatchCreateOrders(ctx, orders)
	}, s.genQuery); err != nil {
		logger.Error(ctx, "DB批量创建订单失败", "error", err)
		return orderServiceError(err)
	}
	for _, order := range orders {
		if order.Status == int32(model.OrderStatusPendingPayment) {
			s.scheduleOrderExpiration(ctx, order)
		}
	}

	return nil
}

// orderServiceError 根据数据库错误类型明确表达订单业务结果。
func orderServiceError(err error) error {
	if _, ok := appconst.AsMsgError(err); ok {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appconst.NewNotFoundError("订单不存在")
	}
	var duplicate *mysqlerr.MySQLError
	if errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062) {
		return appconst.NewConflictError("订单号已存在")
	}
	return appconst.NewInternalError(err)
}

// MaxOrderNoLength 限制订单号为25字符，低于数据库 VARCHAR(30) 上限。
const MaxOrderNoLength = 25

func validateOrderNo(orderNo string) error {
	if orderNo == "" || utf8.RuneCountInString(orderNo) > MaxOrderNoLength {
		return appconst.NewInvalidParamsError("订单号不能为空且不能超过25个字符")
	}
	return nil
}

// PayOrder 模拟支付：只将当前用户的待支付订单改为已支付。
func (s *OrderService) PayOrder(ctx context.Context, orderID, userID int64) (*model.Order, error) {
	return s.changeOrderStatus(ctx, orderID, userID, model.OrderStatusPendingPayment, model.OrderStatusPaid)
}

// ConfirmReceipt 模拟确认收货：已支付→已完成，不依赖真实支付或物流系统。
func (s *OrderService) ConfirmReceipt(ctx context.Context, orderID, userID int64) (*model.Order, error) {
	return s.changeOrderStatus(ctx, orderID, userID, model.OrderStatusPaid, model.OrderStatusCompleted)
}
