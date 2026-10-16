package product_server

import (
	"context"
	"errors"
	appconst "go-framework-demo/const"
	"go-framework-demo/db"
	"go-framework-demo/es"
	"go-framework-demo/logger"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/redis"
	"gorm.io/gorm"
)

// ProductServiceInterface 列出商品业务的全部公开方法。
type ProductServiceInterface interface {
	CreateModel(ctx context.Context, product *model.Product) (*model.Product, error)
	Export(ctx context.Context, input *ProductQueryInput) (*ProductExport, error)
	List(ctx context.Context, input *ProductQueryInput) (*ProductPage, error)
	GetDetail(ctx context.Context, id int64) (*model.Product, error)
	Create(ctx context.Context, name string, price int64, stock int32, image string) (*model.Product, error)
	Update(ctx context.Context, id int64, input *ProductUpdateInput) (*model.Product, error)
}

type ProductService struct {
	productDb    db.ProductDbInterface
	productCache redis.ProductCacheInterface
}

var _ ProductServiceInterface = (*ProductService)(nil)

func NewProductService(ctx context.Context, txQ ...*query.Query) *ProductService {
	d, e := db.NewProductDb(ctx, txQ...)
	if e != nil {
		return &ProductService{}
	}
	service := &ProductService{productDb: d}
	if redis.RedisClient != nil {
		service.productCache = redis.NewProductCache()
	}
	return service
}

// GetDetail 使用缓存优先策略查询商品详情；缓存不可用时降级回源数据库。
func (s *ProductService) GetDetail(ctx context.Context, id int64) (*model.Product, error) {
	if id <= 0 {
		return nil, appconst.NewInvalidParamsError("商品ID必须为正整数")
	}
	if s != nil && s.productCache != nil {
		product, hit, err := s.productCache.GetProduct(ctx, id)
		if err != nil {
			logger.Warn(ctx, "读取商品详情缓存失败，回源数据库", "product_id", id, "error", err)
		} else if hit && product != nil {
			return product, nil
		}
	}
	if s == nil || s.productDb == nil {
		return nil, appconst.NewInternalError(errors.New("product service is not initialized"))
	}
	product, err := s.productDb.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appconst.NewNotFoundError("商品不存在")
	}
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	s.fillProductCache(ctx, product)
	return product, nil
}

func (s *ProductService) fillProductCache(ctx context.Context, product *model.Product) {
	if s == nil || s.productCache == nil || product == nil {
		return
	}
	if err := s.productCache.SetProduct(ctx, product); err != nil {
		logger.Warn(ctx, "商品详情缓存回填失败", "product_id", product.ID, "error", err)
	}
}

type ProductPage struct {
	Items    []*model.Product
	Total    int64
	Page     int
	PageSize int
}

func (s *ProductService) List(ctx context.Context, input *ProductQueryInput) (*ProductPage, error) {
	if err := ctx.Err(); err != nil {
		return nil, appconst.NewInternalError(err)
	}
	options, err := buildProductSearchOptions(input, true)
	if err != nil {
		return nil, err
	}
	if s == nil || s.productDb == nil {
		return nil, appconst.NewInternalError(errors.New("product service is not initialized"))
	}
	ids, total, err := es.GetProductIDsPage(ctx, options)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	products, err := s.loadProductDetails(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := &ProductPage{Items: products, Total: total, Page: input.Page, PageSize: input.PageSize}
	return result, nil
}

// loadProductDetails 批量读取 MySQL 最新详情，并恢复 ES 的命中顺序。
func (s *ProductService) loadProductDetails(ctx context.Context, ids []int64) ([]*model.Product, error) {
	rows, err := s.productDb.GetByIDs(ctx, ids)
	if err != nil {
		logger.Error(ctx, "批量查询商品详情失败", "method", "loadProductDetails", "product_ids", ids, "error", err)
		return nil, appconst.NewInternalError(err)
	}
	byID := make(map[int64]*model.Product, len(rows))
	for _, product := range rows {
		byID[product.ID] = product
	}
	products := make([]*model.Product, 0, len(rows))
	for _, id := range ids {
		if product, ok := byID[id]; ok {
			products = append(products, product)
		}
	}
	return products, nil
}

func (s *ProductService) Create(ctx context.Context, name string, price int64, stock int32, image string) (*model.Product, error) {
	// MySQL 保存业务事实数据，ES 通过 Canal → RocketMQ → 商品 CDC 消费者异步同步。
	product := &model.Product{
		Name:  name,
		Price: price,
		Stock: stock,
		Image: image,
	}
	return s.CreateModel(ctx, product)
}

// CreateModel 接收已由调用方构建的内部商品模型。
func (s *ProductService) CreateModel(ctx context.Context, product *model.Product) (*model.Product, error) {
	if err := ValidateProduct(product); err != nil {
		return nil, err
	}
	if s == nil || s.productDb == nil {
		return nil, appconst.NewInternalError(errors.New("product service is not initialized"))
	}
	created, err := s.productDb.Create(ctx, product)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	return created, nil
}
func (s *ProductService) Update(ctx context.Context, id int64, input *ProductUpdateInput) (*model.Product, error) {
	// 这里仅更新 MySQL，由 Binlog 链路统一同步 ES 查询副本。
	if id <= 0 {
		return nil, appconst.NewInvalidParamsError("商品ID必须为正整数")
	}
	if err := ValidateProductUpdate(input); err != nil {
		return nil, err
	}
	if s == nil || s.productDb == nil {
		return nil, appconst.NewInternalError(errors.New("product service is not initialized"))
	}
	product, err := s.productDb.Update(ctx, id, input)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appconst.NewNotFoundError("商品不存在")
	}
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	return product, nil
}
