package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
)

type productSearchTransport func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试提供的 HTTP 响应函数，模拟外部服务的请求和响应。
func (f productSearchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestProductListUsesCreationTimeDescending 验证商品查询的过滤、分页、精确总数及创建时间和 ID 倒序排列。
func TestProductListUsesCreationTimeDescending(t *testing.T) {
	httpClient := &http.Client{
		Transport: productSearchTransport(func(request *http.Request) (*http.Response, error) {
			var body struct {
				From           int             `json:"from"`
				Size           int             `json:"size"`
				TrackTotalHits bool            `json:"track_total_hits"`
				Query          json.RawMessage `json:"query"`
				Sort           []map[string]struct {
					Order string `json:"order"`
				} `json:"sort"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.From != 20 || body.Size != 20 || !body.TrackTotalHits {
				t.Fatalf("incorrect pagination or totals: %+v", body)
			}
			if !strings.Contains(string(body.Query), `"term":{"id":1}`) || !strings.Contains(string(body.Query), `"match":{"name":{"query":"商品"}}`) {
				t.Fatalf("missing product filters: %s", body.Query)
			}
			if !strings.Contains(string(body.Query), `"is_del":0`) {
				t.Fatalf("missing soft delete filter: %s", body.Query)
			}
			if len(body.Sort) != 2 || body.Sort[0]["update_time_stamp"].Order != "desc" || body.Sort[1]["id"].Order != "desc" {
				t.Fatalf("incorrect product sorting: %+v", body.Sort)
			}
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Request:    request,
				Body:       io.NopCloser(strings.NewReader(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_source":{"id":1,"name":"商品","price":1000,"stock":10,"image":"","create_time_stamp":1791777600123}}]}}`)),
			}, nil
		}),
	}
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	previous := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
	id, name := int64(1), "商品"
	input := struct {
		Options  es.ProductSearchOptions
		Page     int
		PageSize int
	}{
		Options:  es.ProductSearchOptions{ID: &id, Name: &name},
		Page:     2,
		PageSize: 20,
	}
	input.Options.Page = &input.Page
	input.Options.PageSize = &input.PageSize
	items, total, err := es.GetProductsPage(context.Background(), &input.Options)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].CreateTimeStamp != 1791777600123 {
		t.Fatal("product creation time missing from search results")
	}
}

// TestProductSearchOptionalConditions 验证商品查询默认选项、显式零值、升序排序和不完整分页参数。
func TestProductSearchOptionalConditions(t *testing.T) {
	page, size, zeroID, emptyName, asc := 2, 20, int64(0), "", true
	for _, input := range []struct {
		Name        string
		Options     es.ProductSearchOptions
		WantError   bool
		WantPage    bool
		WantAsc     bool
		WantFilters bool
	}{
		{Name: "defaults"},
		{Name: "explicit zero values and ascending", Options: es.ProductSearchOptions{ID: &zeroID, Name: &emptyName, Page: &page, PageSize: &size, AscCreate: &asc}, WantPage: true, WantAsc: true, WantFilters: true},
		{Name: "incomplete pagination", Options: es.ProductSearchOptions{Page: &page}, WantError: true},
	} {
		t.Run(input.Name, func(t *testing.T) {
			source, err := es.BuildProductElasticQuery(&input.Options)
			if input.WantError {
				if err == nil {
					t.Fatal("expected invalid pagination error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			body, err := source.Source()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if (doc["from"] != nil) != input.WantPage || (doc["size"] != nil) != input.WantPage {
				t.Fatalf("unexpected pagination: %s", raw)
			}
			order := "desc"
			if input.WantAsc {
				order = "asc"
			}
			if !strings.Contains(string(doc["sort"]), `"update_time_stamp":{"order":"`+order+`"}`) || strings.Contains(string(doc["sort"]), `"id"`) {
				t.Fatalf("unexpected builder sorting: %s", raw)
			}
			if input.WantFilters && (!strings.Contains(string(doc["query"]), `"term":{"id":0}`)) {
				t.Fatalf("explicit zero values ignored: %s", raw)
			}
		})
	}
}
