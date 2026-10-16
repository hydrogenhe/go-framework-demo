package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
)

// TestProductSearchAfter 验证开关、PIT续期、int64游标精度以及成功/失败时的PIT释放。
func TestProductSearchAfter(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("callback_failure_%v", fail), func(t *testing.T) {
			page, cleared := 0, false
			transport := productSearchTransport(func(request *http.Request) (*http.Response, error) {
				response := `{"id":"pit-1"}`
				if request.Method == http.MethodDelete {
					var body struct {
						ID string `json:"id"`
					}
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body.ID != fmt.Sprintf("pit-%d", page+1) {
						t.Fatalf("did not close latest PIT: %s", body.ID)
					}
					cleared = true
					response = `{"succeeded":true,"num_freed":1}`
				} else if request.URL.Path == "/_search" {
					var body map[string]json.RawMessage
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["from"] != nil || string(body["_source"]) != "false" {
						t.Fatalf("unexpected query: %v", body)
					}
					if !strings.Contains(string(body["pit"]), fmt.Sprintf("pit-%d", page+1)) {
						t.Fatal("PIT id not refreshed")
					}
					page++
					hits := `[]`
					if page == 1 {
						hits = `[{"_id":"9007199254740995","sort":[1791904109000,9007199254740995,3]},{"_id":"9007199254740993","sort":[1791904109000,9007199254740993,2]}]`
					}
					if page == 2 {
						if !strings.Contains(string(body["search_after"]), "9007199254740993") {
							t.Fatalf("cursor precision lost: %s", body["search_after"])
						}
						hits = `[{"_id":"9007199254740991","sort":[1791904109000,9007199254740991,1]}]`
					}
					response = fmt.Sprintf(`{"pit_id":"pit-%d","hits":{"hits":%s}}`, page+1, hits)
				} else if request.URL.Path != "/product_test/_pit" {
					t.Fatalf("unexpected endpoint: %s", request.URL.Path)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Request: request, Body: io.NopCloser(strings.NewReader(response))}, nil
			})
			client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(&http.Client{Transport: transport}))
			if err != nil {
				t.Fatal(err)
			}
			previous, oldSwitch := es.ElasticClient, es.ProductExportUseSearchAfter
			es.ElasticClient, es.ProductExportUseSearchAfter = client, true
			t.Cleanup(func() { client.Stop(); es.ElasticClient = previous; es.ProductExportUseSearchAfter = oldSwitch })
			count := 0
			failure := errors.New("database failure")
			err = es.WalkProductIDs(context.Background(), &es.ProductSearchOptions{}, func(ids []int64) error {
				count += len(ids)
				if fail {
					return failure
				}
				return nil
			})
			if fail && !errors.Is(err, failure) || !fail && err != nil {
				t.Fatalf("wrong error: %v", err)
			}
			if !cleared || !fail && count != 3 {
				t.Fatalf("lost records or PIT leak: count=%d cleared=%v", count, cleared)
			}
		})
	}
}
