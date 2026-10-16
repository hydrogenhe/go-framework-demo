package http_msg

import (
	"mime/multipart"
	"time"
)

type ProductCreateReq struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
	Stock int32  `json:"stock"`
	Image string `json:"image"`
}

// 未传或 null 不修改，非 nil 的零值参与更新。
type ProductUpdateReq struct {
	Name  *string `json:"name"`
	Price *int64  `json:"price"`
	Stock *int32  `json:"stock"`
	Image *string `json:"image"`
}

// ProductQueryReq 通过 URL query 传入，列表与导出共用。
type ProductQueryReq struct {
	Name        *string `form:"name"`
	PriceMin    *string `form:"price_min"`
	PriceMax    *string `form:"price_max"`
	StockMin    *string `form:"stock_min"`
	StockMax    *string `form:"stock_max"`
	CreatedFrom *string `form:"created_from"`
	CreatedTo   *string `form:"created_to"`
	Sort        *string `form:"sort"`
	Page        *string `form:"page"`
	PageSize    *string `form:"page_size"`
}
type ProductResp struct {
	ID         string    `json:"id"`
	Version    int32     `json:"version"`
	Name       string    `json:"name"`
	Price      int64     `json:"price"`
	Stock      int32     `json:"stock"`
	Image      string    `json:"image"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
}
type ProductPageResp struct {
	Items    []ProductResp `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// ProductImageReq 使用 multipart/form-data，字段 file，限一张 JPEG/PNG，最大 5MB。
type ProductImageReq struct {
	File *multipart.FileHeader `form:"file" binding:"required"`
}
type ProductPathReq struct {
	ID string `uri:"id" binding:"required"`
}
type ProductImageResp struct {
	Image string `json:"image"`
}
