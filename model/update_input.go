package model

// ProductUpdateInput 表达商品部分更新；未传或 null 不修改，非 nil 的零值参与更新。
type ProductUpdateInput struct {
	Name  *string `json:"name"`
	Price *int64  `json:"price"`
	Stock *int32  `json:"stock"`
	Image *string `json:"image"`
}

// OrderUpdateInput 仅允许更新订单金额和状态，不允许修改主键、归属和创建时间。
type OrderUpdateInput struct {
	Amount *int64 `json:"amount"`
	Status *int32 `json:"status"`
}
