package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-framework-demo/logger"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"

	"gorm.io/gen/field"
	"gorm.io/gorm/clause"
)

// ProductDbInterface 列出商品数据访问的全部公开方法。
type ProductDbInterface interface {
	RestoreStockIncludingDeleted(ctx context.Context, id int64, quantity int32) (int64, error)
	List(ctx context.Context) ([]*model.Product, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*model.Product, error)
	Create(ctx context.Context, product *model.Product) (*model.Product, error)
	Update(ctx context.Context, id int64, input *model.ProductUpdateInput) (*model.Product, error)
	GetByID(ctx context.Context, id int64) (*model.Product, error)
	DeductStock(ctx context.Context, id int64, quantity int32, version int32) (int64, error)
	GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.Product, error)
	Delete(ctx context.Context, id int64) (bool, error)
}

type ProductDb struct{ GenQuery *query.Query }

var _ ProductDbInterface = (*ProductDb)(nil)

func NewProductDb(ctx context.Context, txQ ...*query.Query) (*ProductDb, error) {
	q := mysql.GetGenQuery()
	if len(txQ) > 0 && txQ[0] != nil {
		q = txQ[0]
	}
	if q == nil || !q.Available() {
		return nil, errors.New("Gen查询器实例无效")
	}
	return &ProductDb{GenQuery: q}, nil
}
func (p *ProductDb) List(ctx context.Context) ([]*model.Product, error) {
	productQuery := p.GenQuery.Product
	return productQuery.WithContext(ctx).Find()
}

// GetByIDs 根据一组商品 ID 批量查询商品。
func (p *ProductDb) GetByIDs(ctx context.Context, ids []int64) ([]*model.Product, error) {
	if len(ids) == 0 {
		return []*model.Product{}, nil
	}
	productQuery := p.GenQuery.Product
	return productQuery.WithContext(ctx).Where(productQuery.ID.In(ids...)).Find()
}

func (p *ProductDb) Create(ctx context.Context, product *model.Product) (*model.Product, error) {
	if product == nil {
		return nil, errors.New("商品不能为空")
	}
	if product.CurrentVersion <= 0 {
		product.CurrentVersion = 1
	}

	if err := WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		if err := txQ.Product.WithContext(ctx).Create(product); err != nil {
			return fmt.Errorf("创建商品失败: %w", err)
		}
		version := &model.ProductVersion{
			ProductID: product.ID,
			Version:   product.CurrentVersion,
			Name:      product.Name,
			Price:     product.Price,
			Image:     product.Image,
		}
		if err := txQ.ProductVersion.WithContext(ctx).Create(version); err != nil {
			return fmt.Errorf("创建商品初始版本失败: %w", err)
		}
		return nil
	}, p.GenQuery); err != nil {
		return nil, err
	}
	return product, nil
}
func (p *ProductDb) Update(ctx context.Context, id int64, input *model.ProductUpdateInput) (*model.Product, error) {
	if input == nil {
		return nil, errors.New("商品更新参数不能为空")
	}
	var updated *model.Product
	err := WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		productQuery := txQ.Product
		product, err := productQuery.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(productQuery.ID.Eq(id)).First()
		if err != nil {
			return err
		}
		updates := make([]field.AssignExpr, 0, 5)
		catalogChanged := false
		if input.Name != nil {
			updates = append(updates, productQuery.Name.Value(*input.Name))
			product.Name = *input.Name
			catalogChanged = true
		}
		if input.Price != nil {
			updates = append(updates, productQuery.Price.Value(*input.Price))
			product.Price = *input.Price
			catalogChanged = true
		}
		if input.Image != nil {
			updates = append(updates, productQuery.Image.Value(*input.Image))
			product.Image = *input.Image
			catalogChanged = true
		}
		if input.Stock != nil {
			updates = append(updates, productQuery.Stock.Value(*input.Stock))
		}
		if catalogChanged {
			if product.CurrentVersion == int32(^uint32(0)>>1) {
				return errors.New("商品版本号已达到上限")
			}
			product.CurrentVersion++
			updates = append(updates, productQuery.CurrentVersion.Value(product.CurrentVersion))
			version := &model.ProductVersion{
				ProductID: product.ID,
				Version:   product.CurrentVersion,
				Name:      product.Name,
				Price:     product.Price,
				Image:     product.Image,
			}
			if err := txQ.ProductVersion.WithContext(ctx).Create(version); err != nil {
				return fmt.Errorf("创建商品资料版本失败: %w", err)
			}
		}
		if len(updates) > 0 {
			updates = append(updates, productQuery.UpdateTime.Value(time.Now()))
			if _, err := productQuery.WithContext(ctx).Where(productQuery.ID.Eq(id)).UpdateColumnSimple(updates...); err != nil {
				return fmt.Errorf("更新商品失败: %w", err)
			}
		}
		if product, err = productQuery.WithContext(ctx).Where(productQuery.ID.Eq(id)).First(); err != nil {
			return err
		}
		updated = product
		return nil
	}, p.GenQuery)
	return updated, err
}

// GetByID 根据商品 ID 查询商品。
func (p *ProductDb) GetByID(ctx context.Context, id int64) (*model.Product, error) {
	productQuery := p.GenQuery.Product
	return productQuery.WithContext(ctx).Where(productQuery.ID.Eq(id)).First()
}

// DeductStock 仅在商品存在且库存充足时原子扣减库存。
func (p *ProductDb) DeductStock(ctx context.Context, id int64, quantity int32, version int32) (int64, error) {
	productQuery := p.GenQuery.Product
	result, err := productQuery.WithContext(ctx).Where(
		productQuery.ID.Eq(id),
		productQuery.Stock.Gte(quantity),
		productQuery.CurrentVersion.Eq(version),
	).UpdateColumnSimple(productQuery.Stock.Sub(quantity))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// GetByIDIncludingDeleted 仅供 CDC 读取最新状态，包含软删除记录。
func (p *ProductDb) GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.Product, error) {
	q := p.GenQuery.Product
	return q.WithContext(ctx).Unscoped().Where(q.ID.Eq(id)).First()
}

// Delete 将商品标记为已删除，保留数据库记录。
func (p *ProductDb) Delete(ctx context.Context, id int64) (bool, error) {
	q := p.GenQuery.Product
	r, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return r.RowsAffected > 0, err
}

// RestoreStockIncludingDeleted 返还预扣库存，保留原超时关闭对软删除商品的处理。
func (p *ProductDb) RestoreStockIncludingDeleted(ctx context.Context, id int64, quantity int32) (int64, error) {
	productQuery := p.GenQuery.Product
	result, err := productQuery.WithContext(ctx).Unscoped().Where(productQuery.ID.Eq(id)).UpdateColumnSimple(productQuery.Stock.Add(quantity))
	if err != nil {
		logger.Error(ctx, "[RestoreStockIncludingDeleted] 返还库存失败", "table", "product", "product_id", id, "error", err)
	}
	return result.RowsAffected, err
}
