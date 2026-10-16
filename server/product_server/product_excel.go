package product_server

import (
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
	"go-framework-demo/model"
)

// newProductSheet 使用 Excelize 创建工作表，集中设置表头和显示格式。
func newProductSheet(file *excelize.File, number int) (*excelize.StreamWriter, error) {
	sheetName := fmt.Sprintf("商品%d", number)
	if number == 1 {
		if err := file.SetSheetName("Sheet1", sheetName); err != nil {
			return nil, err
		}
	} else if _, err := file.NewSheet(sheetName); err != nil {
		return nil, err
	}
	stream, err := file.NewStreamWriter(sheetName)
	if err != nil {
		return nil, err
	}
	panes := excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}
	if err := stream.SetPanes(&panes); err != nil {
		return nil, err
	}
	// 可选的显示设置：按内容长度调整列宽，便于阅读；不影响 Excel 数据写入。
	widths := []struct {
		first, last int
		width       float64
	}{{1, 1, 24}, {2, 2, 36}, {3, 5, 16}, {6, 6, 70}, {7, 7, 26}, {8, 8, 12}}
	for _, column := range widths {
		if err := stream.SetColWidth(column.first, column.last, column.width); err != nil {
			return nil, err
		}
	}
	// 可选的表头样式：加粗白字、深色背景，方便与数据行区分。
	header := excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"17212B"}, Pattern: 1}}
	headerStyle, err := file.NewStyle(&header)
	if err != nil {
		return nil, err
	}
	price := excelize.Style{NumFmt: 2}
	priceStyle, err := file.NewStyle(&price)
	if err != nil {
		return nil, err
	}
	if err := stream.SetColStyle(3, 3, priceStyle); err != nil {
		return nil, err
	}
	headers := []interface{}{"商品ID", "商品名称", "价格（元）", "价格（分）", "库存", "图片地址", "创建时间", "删除标记"}
	if err := stream.SetRow("A1", headers, excelize.RowOpts{StyleID: headerStyle}); err != nil {
		return nil, err
	}
	return stream, nil
}

// productExcelRow 按表头顺序转换数据；ID 和商品名称使用文本，业务金额保持整数分。
func productExcelRow(product *model.Product) []interface{} {
	row := []interface{}{
		strconv.FormatInt(product.ID, 10), product.Name, float64(product.Price) / 100,
		product.Price, product.Stock, product.Image,
		product.CreateTime.Format("2006-01-02 15:04:05.000"), int64(product.IsDel),
	}
	return row
}

func finishProductSheet(stream *excelize.StreamWriter, number, row int) error {
	table := excelize.Table{Range: fmt.Sprintf("A1:H%d", row), Name: fmt.Sprintf("Products%d", number)}
	if err := stream.AddTable(&table); err != nil {
		return err
	}
	return stream.Flush()
}
