package product

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/product_server"
	"go-framework-demo/server/test/support"
	"io"
	"net/http"
	"strings"
	"testing"
)

type productIDTransport func(*http.Request) (*http.Response, error)

func (f productIDTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestIntegrationProductListReadsMySQL 验证 ES 仅返回 ID，商品详情取自 MySQL，恢复 ES 顺序并过滤已删除记录。
func TestIntegrationProductListReadsMySQL(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		products := []*model.Product{
			{Name: "list_first", Price: 1234, Stock: 7, Image: "https://example.com/current-first.png"},
			{Name: "list_second", Price: 5678, Stock: 9, Image: "https://example.com/current-second.png"},
			{Name: "list_deleted", Price: 100, Stock: 1},
		}
		if err := q.Product.WithContext(ctx).Create(products...); err != nil {
			t.Fatal(err)
		}
		if _, err := q.Product.WithContext(ctx).Where(q.Product.ID.Eq(products[2].ID)).Delete(); err != nil {
			t.Fatal(err)
		}
		empty := false
		transport := productIDTransport(func(r *http.Request) (*http.Response, error) {
			var request map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request["_source"] != false {
				t.Fatal("ES must return IDs without source")
			}
			body := fmt.Sprintf(`{"hits":{"total":{"value":3,"relation":"eq"},"hits":[{"_id":"%d"},{"_id":"%d"},{"_id":"%d"}]}}`, products[1].ID, products[2].ID, products[0].ID)
			if empty {
				body = `{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(&http.Client{Transport: transport}))
		if err != nil {
			t.Fatal(err)
		}
		previous := es.ElasticClient
		es.ElasticClient = client
		defer func() { client.Stop(); es.ElasticClient = previous }()
		service := product_server.NewProductService(ctx, q)
		result, err := service.List(ctx, &product_server.ProductQueryInput{Page: 1, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 3 || len(result.Items) != 2 || result.Items[0].ID != products[1].ID || result.Items[1].ID != products[0].ID {
			t.Fatalf("wrong filtering or order: %+v", result)
		}
		if result.Items[0].Price != 5678 || result.Items[0].Stock != 9 || result.Items[0].Image != products[1].Image {
			t.Fatal("database details were not returned")
		}
		empty = true
		result, err = service.List(ctx, &product_server.ProductQueryInput{Page: 1, PageSize: 20})
		if err != nil || result.Items == nil || len(result.Items) != 0 || result.Total != 0 {
			t.Fatalf("wrong empty result: %+v %v", result, err)
		}
	})
}
