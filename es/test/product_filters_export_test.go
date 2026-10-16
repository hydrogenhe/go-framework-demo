package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
)

func TestProductSearchRanges(t *testing.T) {
	name := "hwt"
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	stockMin, stockMax := int32(0), int32(20)
	priceMin, priceMax := int64(0), int64(19900)
	options := es.ProductSearchOptions{Name: &name, CreatedFrom: &start, CreatedTo: &end, MinStock: &stockMin, MaxStock: &stockMax, MinPrice: &priceMin, MaxPrice: &priceMax}
	source, err := es.BuildProductElasticQuery(&options)
	if err != nil {
		t.Fatal(err)
	}
	query, _ := source.Source()
	body, _ := json.Marshal(query)
	for _, fragment := range []string{`"match":{"name":{"query":"hwt"}}`, `"stock":{"from":0,"include_lower":true,"include_upper":true,"to":20}`, `"price":{"from":0,"include_lower":true,"include_upper":true,"to":19900}`, fmt.Sprintf(`"create_time_stamp":{"from":%d,"include_lower":true,"include_upper":true,"to":%d}`, start.UnixMilli(), end.UnixMilli())} {
		if !strings.Contains(string(body), fragment) {
			t.Fatalf("missing %s: %s", fragment, body)
		}
	}
	stockMax = -1
	if _, err := es.BuildProductElasticQuery(&options); err == nil {
		t.Fatal("negative/reversed range accepted")
	}
}

// TestProductExportScrollBeyond10000 验证全量导出不使用分页截断，并关闭游标。
func TestProductExportScrollBeyond10000(t *testing.T) {
	// total 导出测试的记录总数，超过ES普通分页10000条窗口。
	const total = 10005
	position, cleared := 0, false
	transport := productSearchTransport(func(request *http.Request) (*http.Response, error) {
		var response string
		if request.Method == http.MethodDelete {
			cleared = true
			response = `{"succeeded":true,"num_freed":1}`
		} else {
			if position == 0 {
				var body map[string]json.RawMessage
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if string(body["_source"]) != "false" || body["from"] != nil {
					t.Fatalf("incorrect export source/pagination: %v", body)
				}
			}
			hits := []map[string]string{}
			for count := 0; count < 500 && position < total; count++ {
				position++
				hits = append(hits, map[string]string{"_id": fmt.Sprint(position)})
			}
			encoded, _ := json.Marshal(hits)
			response = fmt.Sprintf(`{"_scroll_id":"test-scroll","hits":{"total":{"value":%d,"relation":"eq"},"hits":%s}}`, total, encoded)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response)), Request: request}, nil
	})
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	previous := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
	page, size := 1, 20
	options := es.ProductSearchOptions{Page: &page, PageSize: &size}
	count := 0
	if err := es.WalkProductIDs(context.Background(), &options, func(ids []int64) error { count += len(ids); return nil }); err != nil {
		t.Fatal(err)
	}
	if count != total || !cleared {
		t.Fatalf("truncated or leaked cursor: count=%d cleared=%v", count, cleared)
	}
}
