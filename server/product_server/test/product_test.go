package product

import (
	"context"
	"fmt"
	"go-framework-demo/es"
	"go-framework-demo/server/product_server"
	"go-framework-demo/util"
	"testing"
	"time"
)

type productCreateInput struct {
	Name  string
	Price int64
	Stock int32
	Image string
}

// TestProductCreate 通过商品服务创建商品，验证 MySQL 写入成功并返回有效 ID。
func TestProductCreate(t *testing.T) {
	ctx := context.Background()
	service := product_server.NewProductService(ctx)
	input := productCreateInput{
		Name:  "server_product_" + util.GenerateAlnum(8),
		Price: 19900,
		Stock: 10,
	}
	product, err := service.Create(ctx, input.Name, input.Price, input.Stock, input.Image)
	if err != nil {
		t.Fatalf("创建商品并写入 MySQL 失败: %v", err)
	}
	if product == nil || product.ID <= 0 {
		t.Fatalf("创建商品未返回有效 ID: %+v", product)
	}
	t.Logf("商品已写入 MySQL，ID=%d", product.ID)
}

// TestProductCreateES 创建商品并轮询 ES，验证 CDC 同步后的字段、创建时间及可搜索耗时。
func TestProductCreateES(t *testing.T) {
	ctx := context.Background()
	service := product_server.NewProductService(ctx)
	input := productCreateInput{
		Name:  "server_product_es_" + util.GenerateAlnum(8),
		Price: 19900,
		Stock: 10,
		Image: "", // 图片可选，测试不引用未上传的虚构文件。
	}
	started := time.Now()
	product, err := service.Create(ctx, input.Name, input.Price, input.Stock, input.Image)
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	created := time.Now()
	if product == nil || product.ID <= 0 {
		t.Fatal("创建商品未返回有效 ID")
	}
	// interval 商品同步测试的轮询间隔，100毫秒。
	const interval = 100 * time.Millisecond
	// timeout 商品同步测试的最大等待时长，10秒。
	const timeout = 10 * time.Second
	t.Logf("商品已写入 MySQL，ID=%d，创建耗时=%s；立即查询 ES，随后每隔 %s 轮询，超时上限=%s", product.ID, created.Sub(started), interval, timeout)
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	page, pageSize := 1, 1
	searchOptions := es.ProductSearchOptions{
		ID:       &product.ID,
		Page:     &page,
		PageSize: &pageSize,
	}
	attempts := 0
	var lastErr error
	for {
		attempts++
		items, _, queryErr := es.GetProductsPage(pollCtx, &searchOptions)
		lastErr = queryErr
		if queryErr == nil && len(items) > 0 {
			indexed := items[0]
			observed := time.Now()
			t.Logf("ES首次可搜索，ID=%d，创建返回后耗时=%s，端到端耗时=%s，查询次数=%d", product.ID, observed.Sub(created), observed.Sub(started), attempts)
			if indexed.ID != product.ID || indexed.Name != input.Name || indexed.Price != input.Price || indexed.Stock != input.Stock || indexed.Image != input.Image || indexed.CreateTime.IsZero() {
				t.Fatalf("ES 商品数据不一致: got=%+v want=%+v", indexed, input)
			}
			t.Logf("商品已同步且可搜索，ID=%d，MySQL创建耗时=%s，创建返回后至ES可搜索耗时=%s，端到端耗时=%s，查询次数=%d，轮询间隔=%s", product.ID, created.Sub(started), time.Since(created), time.Since(started), attempts, interval)
			return
		}
		if queryErr == nil {
			lastErr = fmt.Errorf("ES 暂未查到商品 %d", product.ID)
		}
		select {
		case <-pollCtx.Done():
			t.Fatalf("商品同步超时，ID=%d，等待耗时=%s，端到端耗时=%s，查询次数=%d，最后查询结果=%v", product.ID, time.Since(created), time.Since(started), attempts, lastErr)
		case <-ticker.C:
		}
	}
}
