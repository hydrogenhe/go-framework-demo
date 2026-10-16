package http_msg

import "time"

// 所有 ID 在 JSON 中必须是十进制字符串；绑定后以 int64 交给业务层。
// 同一用户重试时复用 OrderNo；相同订单号但下单内容不同会被拒绝。
type OrderCreateReq struct {
	OrderNo string         `json:"order_no"`
	UserID  int64          `json:"user_id,string"`
	Items   []OrderItemReq `json:"items"`
}
type OrderItemReq struct {
	ProductID      int64   `json:"product_id,string"`
	ProductVersion int32   `json:"product_version,omitempty"`
	Quantity       int32   `json:"quantity"`
	UnitPriceFen   int64   `json:"unit_price_fen"`
	Name           *string `json:"name,omitempty"`
	Image          *string `json:"image,omitempty"`
	Stock          *int32  `json:"stock,omitempty"`
}
type OrderQueryReq struct {
	UserID      *string `form:"user_id"`
	Status      *string `form:"status"`
	PriceMin    *string `form:"price_min"`
	PriceMax    *string `form:"price_max"`
	CreatedFrom *string `form:"created_from"`
	CreatedTo   *string `form:"created_to"`
	Sort        *string `form:"sort"`
	Page        *string `form:"page"`
	PageSize    *string `form:"page_size"`
}
type OrderResp struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	OrderNo    string    `json:"order_no"`
	Amount     int64     `json:"amount"`
	Status     int32     `json:"status"`
	CreateTime time.Time `json:"create_time"`
	ExpireTime time.Time `json:"expire_time"`
	UpdateTime time.Time `json:"update_time"`
}
type OrderItemResp struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"order_id"`
	ProductID      string    `json:"product_id"`
	ProductVersion int32     `json:"product_version"`
	Quantity       int32     `json:"quantity"`
	UnitPriceFen   int64     `json:"unit_price_fen"`
	Status         int32     `json:"status"`
	Image          string    `json:"image"`
	ProductName    string    `json:"product_name"`
	UpdateTime     time.Time `json:"update_time"`
}
type OrderListResp struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	OrderNo    string          `json:"order_no"`
	Amount     int64           `json:"amount"`
	Status     int32           `json:"status"`
	CreateTime time.Time       `json:"create_time"`
	ExpireTime time.Time       `json:"expire_time"`
	Items      []OrderItemResp `json:"items"`
}
type OrderPageResp struct {
	Items    []OrderListResp `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type OrderPathReq struct {
	ID     string `uri:"id" binding:"required"`
	ItemID string `uri:"item_id"`
}
