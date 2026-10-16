package product

import (
	"context"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/product_server"
	"go-framework-demo/server/test/support"
	"go-framework-demo/util"
	"testing"
)

// TestIntegrationProductUpdateValidation 验证非法商品字段不能更新，合法价格和库存正确保存且创建时间不变。
func TestIntegrationProductUpdateValidation(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		products := product_server.NewProductService(ctx, q)
		input := &model.Product{
			Name:  "validation_product_" + util.GenerateAlnum(8),
			Price: 1000,
			Stock: 10,
			Image: "https://example.com/product.png",
		}
		created, err := products.Create(ctx, input.Name, input.Price, input.Stock, input.Image)
		if err != nil {
			t.Fatal(err)
		}
		negativeStock := int32(-1)
		tooHighPrice := product_server.MaxProductPriceFen + 1
		for _, data := range []product_server.ProductUpdateInput{{}, {Stock: &negativeStock}, {Price: &tooHighPrice}} {
			if _, err := products.Update(ctx, created.ID, &data); err == nil {
				t.Fatalf("invalid update accepted: %+v", data)
			}
		}
		if created.CreateTime.IsZero() {
			t.Fatal("商品创建时间未生成")
		}
		found, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(created.ID)).First()
		if err != nil || found.CreateTime.IsZero() {
			t.Fatal("商品创建时间未保存")
		}
		price := int64(2000)
		updates := product_server.ProductUpdateInput{Price: &price}
		updated, err := products.Update(ctx, created.ID, &updates)
		if err != nil {
			t.Fatal(err)
		}
		if updated.ID != created.ID || updated.Price != 2000 || updated.Stock != 10 || updated.CurrentVersion != 2 || !updated.CreateTime.Equal(found.CreateTime) {
			t.Fatal("valid update not persisted")
		}
		var versionCount int64
		if err := q.UnderlyingDB().WithContext(ctx).Table(model.TableNameProductVersion).Where("product_id = ?", created.ID).Count(&versionCount).Error; err != nil {
			t.Fatal(err)
		}
		if versionCount != 2 {
			t.Fatalf("商品资料版本数错误: got=%d want=2", versionCount)
		}
		stock := int32(20)
		stockUpdate := product_server.ProductUpdateInput{Stock: &stock}
		updated, err = products.Update(ctx, created.ID, &stockUpdate)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Stock != 20 || updated.CurrentVersion != 2 {
			t.Fatalf("单独调整库存不应推进资料版本: %+v", updated)
		}
		zeroStock, emptyImage := int32(0), ""
		zeroUpdate := product_server.ProductUpdateInput{Stock: &zeroStock, Image: &emptyImage}
		updated, err = products.Update(ctx, created.ID, &zeroUpdate)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Stock != 0 || updated.Image != "" || updated.Price != 2000 || updated.Name != input.Name || updated.CurrentVersion != 3 {
			t.Fatalf("零值未写入或未传字段被修改: %+v", updated)
		}
		stockOnly := int32(5)
		stockOnlyUpdate := product_server.ProductUpdateInput{Stock: &stockOnly}
		updated, err = products.Update(ctx, created.ID, &stockOnlyUpdate)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Stock != 5 || updated.CurrentVersion != 3 {
			t.Fatalf("仅改库存不应创建商品资料版本: %+v", updated)
		}
	})
}
