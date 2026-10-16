package appconst

// OrderStatus 表示订单支付及完成状态。
type OrderStatus int32

const (
	// OrderStatusPendingPayment 订单待支付。
	OrderStatusPendingPayment OrderStatus = 1
	// OrderStatusPaid 订单已支付。
	OrderStatusPaid OrderStatus = 2
	// OrderStatusCompleted 订单已完成。
	OrderStatusCompleted OrderStatus = 3
	// OrderStatusCancelled 订单已取消。
	OrderStatusCancelled OrderStatus = 4
)

// OrderItemStatus 表示订单明细的发货状态。
type OrderItemStatus int32

const (
	// OrderItemStatusPendingShipment 订单明细待发货。
	OrderItemStatusPendingShipment OrderItemStatus = 1
	// OrderItemStatusShipped 订单明细已发货。
	OrderItemStatusShipped OrderItemStatus = 2
)
