package test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
)

// TestDeleteDocContextIdempotence 验证重复删除成功，同时保留索引缺失和服务端错误。
func TestDeleteDocContextIdempotence(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"删除成功", 200, `{"_index":"product_test","_id":"1","result":"deleted"}`, false},
		{"文档已不存在", 404, `{"_index":"product_test","_id":"1","result":"not_found"}`, false},
		{"索引不存在", 404, `{"error":{"type":"index_not_found_exception","reason":"no such index"},"status":404}`, true},
		{"服务端错误", 500, `{"error":{"type":"exception","reason":"failed"},"status":500}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			httpClient := &http.Client{Transport: productSearchTransport(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodDelete || request.URL.Path != "/product_test/_doc/1" {
					t.Fatalf("删除请求不正确: %s %s", request.Method, request.URL.Path)
				}
				return &http.Response{
					StatusCode: tc.status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Request:    request,
				}, nil
			})}
			client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetMaxRetries(0), elastic.SetHttpClient(httpClient))
			if err != nil {
				t.Fatal(err)
			}
			previous := es.ElasticClient
			es.ElasticClient = client
			t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
			err = es.DeleteDocContext(context.Background(), es.ES_PRODUCT, "1")
			if (err != nil) != tc.wantErr {
				t.Fatalf("错误结果不符合预期: err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
