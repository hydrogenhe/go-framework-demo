package test

import (
	"context"
	"testing"

	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

// TestProductCreateAndGet 使用真实 MySQL，验证商品持久化、自动生成 ID 及查询结果。
func TestProductCreateAndGet(t *testing.T) {
	mysql.InitMySQL()
	product := &model.Product{
		Name:  "product_test_" + util.GenerateAlnum(8),
		Price: 19900,
		Stock: 10,
	}
	productDB := mysql.GetGenQuery().Product.WithContext(context.Background())
	if err := productDB.Create(product); err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	if product.ID <= 0 {
		t.Fatal("创建商品后未生成有效ID")
	}

	found, err := mysql.GetGenQuery().Product.WithContext(context.Background()).Where(mysql.GetGenQuery().Product.ID.Eq(product.ID)).First()
	if err != nil {
		t.Fatalf("查询商品失败: %v", err)
	}
	if found.Name != product.Name || found.Price != 19900 || found.Stock != 10 {
		t.Fatalf("查询商品数据不一致: got=%+v", found)
	}
}
