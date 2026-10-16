package product

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/olivere/elastic/v7"
	"github.com/xuri/excelize/v2"
	"go-framework-demo/es"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/product_server"
	"go-framework-demo/server/test/support"
	"go-framework-demo/util"
)

// TestIntegrationProductExport 验证导出遍历所有 ES 页面，批量回查最新 DB 详情，并保留 ID 精度与文本。
func TestIntegrationProductExport(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		products := []*model.Product{{Name: "=1+1", Price: 1234, Stock: 0}, {Name: "最新商品", Price: 5678, Stock: 9}, {Name: "已删除", Price: 100, Stock: 1}}
		if err := q.Product.WithContext(ctx).Create(products...); err != nil {
			t.Fatal(err)
		}
		if _, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(products[2].ID)).Delete(); err != nil {
			t.Fatal(err)
		}
		missingID := util.GetID()
		page, cleared := 0, false
		transport := productIDTransport(func(request *http.Request) (*http.Response, error) {
			body := `{"_scroll_id":"export-scroll","hits":{"total":{"value":4,"relation":"eq"},"hits":[]}}`
			if request.Method == http.MethodDelete {
				cleared = true
				body = `{"succeeded":true,"num_freed":1}`
			} else {
				page++
				if page == 1 {
					var source map[string]json.RawMessage
					if err := json.NewDecoder(request.Body).Decode(&source); err != nil {
						t.Fatal(err)
					}
					if source["from"] != nil || string(source["_source"]) != "false" {
						t.Fatalf("invalid export query: %v", source)
					}
					body = fmt.Sprintf(`{"_scroll_id":"export-scroll","hits":{"total":{"value":4,"relation":"eq"},"hits":[{"_id":"%d"},{"_id":"%d"}]}}`, products[1].ID, products[2].ID)
				} else if page == 2 {
					body = fmt.Sprintf(`{"_scroll_id":"export-scroll","hits":{"total":{"value":4,"relation":"eq"},"hits":[{"_id":"%d"},{"_id":"%d"}]}}`, products[0].ID, missingID)
				}
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		})
		client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(&http.Client{Transport: transport}))
		if err != nil {
			t.Fatal(err)
		}
		previous := es.ElasticClient
		es.ElasticClient = client
		defer func() { client.Stop(); es.ElasticClient = previous }()
		service := product_server.NewProductService(ctx, q)
		result, err := service.Export(ctx, &product_server.ProductQueryInput{Page: 1, PageSize: 1})
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(result.Path)
		if result.Rows != 2 || !cleared {
			t.Fatalf("invalid full export: rows=%d cleared=%v", result.Rows, cleared)
		}
		workbook, err := excelize.OpenFile(result.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer workbook.Close()
		rows, err := workbook.GetRows("商品1")
		if err != nil || len(rows) != 3 {
			t.Fatalf("wrong exported rows: %v %v", rows, err)
		}
		if len(rows[0]) != 8 || rows[0][0] != "商品ID" || rows[0][7] != "删除标记" {
			t.Fatalf("wrong headers: %v", rows[0])
		}
		first, second := rows[1], rows[2]
		if len(first) != 8 || first[0] != fmt.Sprint(products[1].ID) || first[1] != "最新商品" || first[2] != "56.78" || second[1] != "=1+1" || second[4] != "0" {
			t.Fatalf("wrong exported values: %v %v", first, second)
		}
		for _, cell := range []string{"A2", "A3", "B3"} {
			formula, err := workbook.GetCellFormula("商品1", cell)
			if err != nil || formula != "" {
				t.Fatalf("text interpreted as formula: %s %q %v", cell, formula, err)
			}
			cellType, err := workbook.GetCellType("商品1", cell)
			if err != nil || cellType != excelize.CellTypeInlineString && cellType != excelize.CellTypeSharedString {
				t.Fatalf("text type lost: %s %v %v", cell, cellType, err)
			}
		}
		panes, err := workbook.GetPanes("商品1")
		if err != nil || !panes.Freeze || panes.YSplit != 1 || panes.TopLeftCell != "A2" {
			t.Fatalf("header not frozen: %+v %v", panes, err)
		}
		width, err := workbook.GetColWidth("商品1", "F")
		if err != nil || width != 70 {
			t.Fatalf("image column width: %v %v", width, err)
		}

		tables, err := workbook.GetTables("商品1")
		if err != nil || len(tables) != 1 || tables[0].Range != "A1:H3" {
			t.Fatalf("filter table missing: %+v %v", tables, err)
		}

		// 用真实 Handler 检查下载响应，并确认下载后不遗留临时文件。
		temporary := t.TempDir()
		t.Setenv("TMPDIR", temporary)
		page = 0
		router := gin.New()
		router.PUT("/api/products/export", handler.ExportProductsHandler(service))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/products/export", nil))
		if response.Code != http.StatusOK || response.Header().Get("X-Export-Count") != "2" || !strings.Contains(response.Header().Get("Content-Disposition"), ".xlsx") {
			t.Fatalf("wrong download response: %d %v", response.Code, response.Header())
		}
		if _, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len())); err != nil {
			t.Fatalf("download is not an XLSX archive: %v", err)
		}
		remaining, err := os.ReadDir(temporary)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("download temporary files leaked: %v %v", remaining, err)
		}
	})
}
