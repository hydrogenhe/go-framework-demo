package product_server

import (
	"context"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"os"

	appconst "go-framework-demo/const"
	"go-framework-demo/es"
	"go-framework-demo/logger"
)

// ProductExport 指向已生成的临时 Excel；HTTP 发送完成后必须删除临时文件。
type ProductExport struct {
	Path string
	Rows int
}

// Export 从 ES 分批取全部匹配 ID，再批量读取 MySQL 最新详情写入 Excel。
func (s *ProductService) Export(ctx context.Context, input *ProductQueryInput) (*ProductExport, error) {
	options, err := buildProductSearchOptions(input, false)
	if err != nil {
		return nil, err
	}
	if s == nil || s.productDb == nil {
		return nil, appconst.NewInternalError(errors.New("product service is not initialized"))
	}
	// 1. 创建 Excelize 句柄和工作表，约定表头。
	file := excelize.NewFile()
	defer file.Close()
	sheetNumber, rowNumber, count := 1, 1, 0
	stream, err := newProductSheet(file, sheetNumber)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}

	// 2. 分批取得 ES 命中的 MySQL 最新数据；3. 直接调用 Excelize 写入。
	err = es.WalkProductIDs(ctx, options, func(ids []int64) error {
		products, err := s.loadProductDetails(ctx, ids)
		if err != nil {
			return err
		}
		for _, product := range products {
			if err := ctx.Err(); err != nil {
				return err
			}
			if rowNumber == excelize.TotalRows {
				if err := finishProductSheet(stream, sheetNumber, rowNumber); err != nil {
					return err
				}
				sheetNumber++
				stream, err = newProductSheet(file, sheetNumber)
				if err != nil {
					return err
				}
				rowNumber = 1
			}
			rowNumber++
			if err := stream.SetRow(fmt.Sprintf("A%d", rowNumber), productExcelRow(product)); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err == nil {
		err = finishProductSheet(stream, sheetNumber, rowNumber)
	}
	if err != nil {
		logger.Error(ctx, "导出商品Excel失败", "method", "ProductService.Export", "error", err)
		return nil, appconst.NewInternalError(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, appconst.NewInternalError(err)
	}

	// Excelize 保存文件；只有成功结果交给 HTTP 层负责下载后删除。
	temporary, err := os.CreateTemp("", "demo-products-*.xlsx")
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	path := temporary.Name()
	completed := false
	defer func() {
		if !completed {
			_ = os.Remove(path)
		}
	}()
	if err := temporary.Close(); err != nil {
		return nil, appconst.NewInternalError(err)
	}
	if err := file.SaveAs(path); err != nil {
		logger.Error(ctx, "保存商品Excel失败", "method", "ProductService.Export", "error", err)
		return nil, appconst.NewInternalError(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, appconst.NewInternalError(err)
	}
	completed = true
	result := &ProductExport{Path: path, Rows: count}
	logger.Info(ctx, "商品Excel导出完成", "method", "ProductService.Export", "row_count", count)
	return result, nil
}
