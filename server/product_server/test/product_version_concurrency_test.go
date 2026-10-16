package product

import (
	"context"
	"fmt"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/server/product_server"
	"go-framework-demo/util"
	"sync"
	"testing"
)

// TestProductVersionConcurrentUpdates serializes simultaneous catalog edits on the stable product row.
func TestProductVersionConcurrentUpdates(t *testing.T) {
	if err := mysql.InitMySQL(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	service := product_server.NewProductService(ctx)
	product, err := service.Create(ctx, "version_race_"+util.GenerateAlnum(8), 1200, 17, "https://example.com/version.png")
	if err != nil {
		t.Fatal(err)
	}
	query := mysql.GetGenQuery()
	t.Cleanup(func() {
		if err := query.UnderlyingDB().WithContext(ctx).Table(model.TableNameProductVersion).Where("product_id = ?", product.ID).Delete(nil).Error; err != nil {
			t.Errorf("删除测试商品版本失败: %v", err)
		}
		if _, err := query.Product.WithContext(ctx).Unscoped().Where(query.Product.ID.Eq(product.ID)).Delete(); err != nil {
			t.Errorf("删除测试商品失败: %v", err)
		}
	})

	start := make(chan struct{})
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for index := 1; index <= 2; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			name := fmt.Sprintf("version_race_%d_%s", index, util.GenerateAlnum(6))
			input := &product_server.ProductUpdateInput{Name: &name}
			_, err := service.Update(ctx, product.ID, input)
			errs <- err
		}(index)
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	current, err := query.Product.WithContext(ctx).Where(query.Product.ID.Eq(product.ID)).First()
	if err != nil {
		t.Fatal(err)
	}
	var versionCount int64
	if err := query.UnderlyingDB().WithContext(ctx).Table(model.TableNameProductVersion).Where("product_id = ?", product.ID).Count(&versionCount).Error; err != nil {
		t.Fatal(err)
	}
	if current.CurrentVersion != 3 || versionCount != 3 || current.Stock != 17 {
		t.Fatalf("并发更新未串行创建版本或库存被修改: current=%+v versions=%d", current, versionCount)
	}
}
