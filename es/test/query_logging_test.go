package test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestESQuerySuccessLogs 验证实际查询入口的成功日志包含 Log ID、数量、分页与耗时，失败时不误报成功。
func TestESQuerySuccessLogs(t *testing.T) {
	var logs bytes.Buffer
	previousLog := logger.Log
	logger.Log = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&logs), zap.InfoLevel))
	t.Cleanup(func() { logger.Log = previousLog })
	fail := false
	transport := productSearchTransport(func(r *http.Request) (*http.Response, error) {
		status, body := 200, `{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_source":{"id":1,"name":"商品"}}]}}`
		if fail {
			status, body = 400, `{"error":{"type":"bad_request","reason":"test"},"status":400}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	previousClient := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previousClient })
	ctx := logger.WithLogID(context.Background(), "es-query-test")
	page, size := 2, 20
	if _, _, err = es.GetProductsPage(ctx, &es.ProductSearchOptions{Page: &page, PageSize: &size}); err != nil {
		t.Fatal(err)
	}
	var entry map[string]interface{}
	if err = json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["log_id"] != "es-query-test" || entry["page"] != float64(2) || entry["page_size"] != float64(20) || entry["returned_count"] != float64(1) || entry["elapsed_ms"] == nil {
		t.Fatalf("missing query fields: %+v", entry)
	}
	logs.Reset()
	if _, err = es.SearchDocsContext(ctx, es.ES_PRODUCT, elastic.NewMatchAllQuery()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"method":"SearchDocs"`) {
		t.Fatal("missing SearchDocs log")
	}
	logs.Reset()
	var target map[string]interface{}
	if err = es.SearchOneContext(ctx, es.ES_PRODUCT, elastic.NewSearchSource(), &target); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"method":"SearchOne"`) {
		t.Fatal("missing SearchOne log")
	}
	logs.Reset()
	fail = true
	if _, _, err = es.GetProductsPage(ctx, &es.ProductSearchOptions{}); err == nil {
		t.Fatal("failure accepted")
	}
	if strings.Contains(logs.String(), "ES查询完成") {
		t.Fatal("failed query logged success")
	}
}
