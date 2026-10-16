package log

import (
	"context"
	"errors"
	appconst "go-framework-demo/const"
	"go-framework-demo/server"
	"os"
	"path/filepath"
	"testing"
)

func TestLogQuery(t *testing.T) {
	dir := t.TempDir()
	old := `{"log_id":"abc-old","level":"info","msg":"旧日志"}` + "\n"
	lines := `{"ts":"2026-10-14T00:04:10.011+0800","log_id":"abc-123","level":"info","msg":"SQL执行完成","sql":"SELECT 368077737904050176","product_id":368077737904050176,"elapsed_ms":0.711167}` + "\n" + `{"log_id":"abc-456","level":"error","msg":"创建订单失败","error":"Data too long","path":"/api/orders"}` + "\n" + `{"level":"fatal","msg":"连接失败"}` + "\n" + `not JSON` + "\n" + `{"log_id":"abc-789","level":"info","msg":"HTTP请求完成","latency":0.003422792}` + "\n"
	for name, data := range map[string]string{"app-2026-10-13.log": old, "app-2026-10-14.log": lines} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := server.NewLogService(dir)
	input := server.LogQuery{Keyword: "AbC-", Level: "info", Page: 2, PageSize: 1}
	result, err := service.Query(context.Background(), &input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Date != "2026-10-14" || result.Total != 2 || len(result.Items) != 1 || result.Items[0].Line != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Items[0].Fields[0].Value != "3.423" {
		t.Fatalf("latency not in milliseconds: %+v", result.Items[0].Fields)
	}
	input = server.LogQuery{Keyword: "368077737", Level: "info"}
	result, err = service.Query(context.Background(), &input)
	if err != nil {
		t.Fatal(err)
	}
	foundID := false
	for _, field := range result.Items[0].Fields {
		if field.Key == "product_id" {
			foundID = field.Value == "368077737904050176"
		}
	}
	if !foundID {
		t.Fatal("large ID lost precision")
	}
	input = server.LogQuery{Level: "err"}
	result, err = service.Query(context.Background(), &input)
	if err != nil || result.Total != 2 {
		t.Fatalf("ERR filter failed: %+v %v", result, err)
	}
	input = server.LogQuery{Keyword: "订单"}
	result, err = service.Query(context.Background(), &input)
	if err != nil || result.Total != 1 {
		t.Fatalf("Chinese substring search failed: %+v %v", result, err)
	}
	input = server.LogQuery{Date: "2026-10-13"}
	result, err = service.Query(context.Background(), &input)
	if err != nil || result.Total != 1 {
		t.Fatalf("date selection failed: %+v %v", result, err)
	}
	for _, input := range []server.LogQuery{{Date: "../../etc/passwd"}, {Date: "2026-02-30"}, {Level: "invalid"}, {Page: -1}, {PageSize: 101}} {
		if _, err := service.Query(context.Background(), &input); err == nil {
			t.Fatalf("invalid query accepted: %+v", input)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Query(ctx, &server.LogQuery{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if _, err := server.NewLogService(t.TempDir()).Query(context.Background(), &server.LogQuery{}); err == nil {
		t.Fatal("empty directory accepted")
	} else if e, ok := appconst.AsMsgError(err); !ok || e.Code != appconst.CodeNotFound {
		t.Fatal(err)
	}
}
