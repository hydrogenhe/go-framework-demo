package product_server

import (
	"context"
	"testing"
	"time"

	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/redis"
)

func prepareProductDetailTest(t *testing.T) (*redis.ProductCache, *db.ProductDb, *model.Product) {
	t.Helper()
	if err := mysql.InitMySQL(); err != nil {
		t.Fatalf("initialize MySQL for product detail test: %v", err)
	}
	if redis.RedisClient == nil {
		if err := redis.Init(); err != nil {
			t.Fatalf("initialize Redis for product detail test: %v", err)
		}
	}
	ctx := context.Background()
	if err := redis.RedisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect Redis for product detail test: %v", err)
	}
	productDB, err := db.NewProductDb(ctx)
	if err != nil {
		t.Fatalf("initialize product repository: %v", err)
	}
	product := &model.Product{
		Name:  "product_detail_test_" + time.Now().Format("20060102150405.000000000"),
		Price: 100,
		Stock: 1,
		Image: "",
	}
	if _, err := productDB.Create(ctx, product); err != nil {
		t.Fatalf("create MySQL product fixture: %v", err)
	}
	t.Cleanup(func() {
		if _, err := productDB.Delete(ctx, product.ID); err != nil {
			t.Errorf("delete MySQL product fixture: %v", err)
		}
	})
	cache := redis.NewProductCache()
	if err := cache.DelProduct(ctx, product.ID); err != nil {
		t.Fatalf("clear Redis fixture before test: %v", err)
	}
	t.Cleanup(func() {
		if err := cache.DelProduct(ctx, product.ID); err != nil {
			t.Errorf("clear Redis fixture after test: %v", err)
		}
	})
	return cache, productDB, product
}

func TestGetDetailCacheHitSkipsDatabase(t *testing.T) {
	cache, productDB, product := prepareProductDetailTest(t)
	cached := &model.Product{ID: product.ID, Name: "cached"}
	if err := cache.SetProduct(context.Background(), cached); err != nil {
		t.Fatal(err)
	}
	// 软删除数据库记录后，缓存命中仍应直接返回；若错误地回源 DB，此查询将返回商品不存在。
	if _, err := productDB.Delete(context.Background(), product.ID); err != nil {
		t.Fatalf("delete product before cache-hit assertion: %v", err)
	}
	service := &ProductService{productDb: productDB, productCache: cache}

	got, err := service.GetDetail(context.Background(), product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != cached.ID || got.Name != cached.Name {
		t.Fatalf("cache hit should return cached product without DB access: got=%+v", got)
	}
}

func TestGetDetailCacheMissLoadsAndFillsCache(t *testing.T) {
	cache, productDB, product := prepareProductDetailTest(t)
	service := &ProductService{productDb: productDB, productCache: cache}

	got, err := service.GetDetail(context.Background(), product.ID)
	if err != nil {
		t.Fatal(err)
	}
	cached, hit, cacheErr := cache.GetProduct(context.Background(), product.ID)
	if got.ID != product.ID || got.Name != product.Name || cacheErr != nil || !hit || cached.Name != product.Name {
		t.Fatalf("cache miss should read DB and fill Redis: got=%+v cached=%+v hit=%v err=%v", got, cached, hit, cacheErr)
	}
}
