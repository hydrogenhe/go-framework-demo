package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/model"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	handler "go-framework-demo/http_handler"
	"go-framework-demo/server/product_server"
)

type productTransport func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试提供的 HTTP 响应函数，模拟外部服务的请求和响应。
func (f productTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestProductPagination 使用 ES 替身，验证商品分页参数、总数、字符串 ID 及请求取消的传递。
func TestProductPagination(t *testing.T) {
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to test database hydration")
	}
	mysql.InitMySQL()
	q := mysql.GetGenQuery()
	product := &model.Product{Name: "pagination_" + util.GenerateAlnum(8), Price: 100, Stock: 2}
	if err := q.Product.WithContext(context.Background()).Create(product); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q.Product.WithContext(context.Background()).Unscoped().Where(q.Product.ID.Eq(product.ID)).Delete()
	})

	transport := productTransport(func(r *http.Request) (*http.Response, error) {
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["from"] != float64(20) || body["size"] != float64(20) || body["track_total_hits"] != true || body["sort"] == nil {
			t.Errorf("incorrect page query: %v", body)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"hits":{"total":{"value":45,"relation":"eq"},"hits":[{"_id":"%d"}]}}`, product.ID))), Request: r}, nil
	})
	old := es.ElasticClient
	httpClient := &http.Client{
		Transport: transport,
	}
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetHttpClient(httpClient), elastic.SetSniff(false), elastic.SetHealthcheck(false))
	if err != nil {
		t.Fatal(err)
	}
	es.ElasticClient = client
	defer func() { es.ElasticClient = old; client.Stop() }()
	r := testRouter(t)
	handler.RegisterRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/products/list?page=2&page_size=20", nil))
	var envelope http_msg.Response[http_msg.ProductPageResp]
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope.Data
	if envelope.Code != 0 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if w.Code != 200 || result.Total != 45 || result.Page != 2 || result.PageSize != 20 || len(result.Items) != 1 || result.Items[0].ID != fmt.Sprint(product.ID) {
		t.Fatalf("incorrect response: %d %s", w.Code, w.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (&product_server.ProductService{}).List(ctx, &product_server.ProductQueryInput{Page: 2, PageSize: 20})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("request cancellation lost: %v", err)
	}
}

// TestProductPaginationInvalidParameters 验证非法页码、每页条数及超出查询窗口的参数返回 400。
func TestProductPaginationInvalidParameters(t *testing.T) {
	r := testRouter(t)
	handler.RegisterRoutes(r)
	for _, query := range []string{"page=0", "page=-1", "page=x", "page=", "page_size=0", "page_size=101", "page_size=x", "page=501&page_size=20", "page=9223372036854775807&page_size=100", "stock_min=-1", "stock_min=20&stock_max=10", "price_min=200&price_max=100", "price_min=1.5", "created_from=invalid", "created_from=2026-10-14T00:00:00Z&created_to=2026-10-01T00:00:00Z"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/products/list?"+query, nil))
		if w.Code != 400 {
			t.Errorf("%s: got %d", query, w.Code)
		}
	}
}
