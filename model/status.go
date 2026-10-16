package model

import (
	appconst "go-framework-demo/const"
	"time"
)

// OrderPaymentWindow 订单默认支付窗口，10分钟。
const OrderPaymentWindow = 10 * time.Minute

type OrderStatus = appconst.OrderStatus
type OrderItemStatus = appconst.OrderItemStatus

const (
	// OrderStatusPendingPayment 订单待支付。
	OrderStatusPendingPayment = appconst.OrderStatusPendingPayment
	// OrderStatusPaid 订单已支付。
	OrderStatusPaid = appconst.OrderStatusPaid
	// OrderStatusCompleted 订单已完成。
	OrderStatusCompleted = appconst.OrderStatusCompleted
	// OrderStatusCancelled 订单已取消。
	OrderStatusCancelled = appconst.OrderStatusCancelled

	// OrderItemStatusPendingShipment 订单明细待发货。
	OrderItemStatusPendingShipment = appconst.OrderItemStatusPendingShipment
	// OrderItemStatusShipped 订单明细已发货。
	OrderItemStatusShipped = appconst.OrderItemStatusShipped
)
