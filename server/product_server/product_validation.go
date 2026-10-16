package product_server

import (
	appconst "go-framework-demo/const"
	"go-framework-demo/model"
	"strings"
)

// MaxProductPriceFen 商品价格上限，单位为分。
const MaxProductPriceFen int64 = 100_000_000
// MaxProductStock 商品库存上限，单位为件。
const MaxProductStock int64 = 1_000_000

func ValidateProduct(p *model.Product) error {
	if p == nil || strings.TrimSpace(p.Name) == "" || len(p.Name) > 255 {
		return appconst.NewInvalidParamsError("商品名称不能为空且不能超过255字节")
	}
	if p.Price < 1 || p.Price > MaxProductPriceFen {
		return appconst.NewInvalidParamsError("商品价格必须为0.01-1000000元")
	}
	if p.Stock < 0 || int64(p.Stock) > MaxProductStock {
		return appconst.NewInvalidParamsError("库存必须为0-1000000的整数")
	}
	if len(p.Image) > 4096 {
		return appconst.NewInvalidParamsError("图片地址不能超过4096字节")
	}
	return nil
}

// ValidateProductUpdate 只校验非 nil 的商品更新字段，不负责数据库字段组装。
func ValidateProductUpdate(input *ProductUpdateInput) error {
	if input == nil {
		return appconst.NewInvalidParamsError("商品更新参数不能为空")
	}
	if input.Name != nil {
		if strings.TrimSpace(*input.Name) == "" || len(*input.Name) > 255 {
			return appconst.NewInvalidParamsError("商品名称不能为空且不能超过255字节")
		}
	}
	if input.Price != nil {
		if *input.Price < 1 || *input.Price > MaxProductPriceFen {
			return appconst.NewInvalidParamsError("商品价格必须为0.01-1000000元")
		}
	}
	if input.Stock != nil {
		if *input.Stock < 0 || int64(*input.Stock) > MaxProductStock {
			return appconst.NewInvalidParamsError("库存必须为0-1000000的整数")
		}
	}
	if input.Image != nil {
		if len(*input.Image) > 4096 {
			return appconst.NewInvalidParamsError("图片地址不能超过4096字节")
		}
	}
	if input.Name == nil && input.Price == nil && input.Stock == nil && input.Image == nil {
		return appconst.NewInvalidParamsError("请指定要修改的商品字段")
	}
	return nil
}
