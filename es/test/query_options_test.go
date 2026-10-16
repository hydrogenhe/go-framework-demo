package test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
)

// TestUserOrderSearchQueries 验证用户和订单查询包含指定过滤条件、分页及创建时间排序。
func TestUserOrderSearchQueries(t *testing.T) {
	id, userID, page, size, asc := int64(9007199254740993), int64(1), 2, 20, true
	name, email, orderNo := "用户", "123@qq.com", "order-1"
	productIDs := []int64{9007199254740995}
	for _, input := range []struct {
		Name   string
		Build  func() (*elastic.SearchSource, error)
		Fields []string
	}{
		{Name: "user", Build: func() (*elastic.SearchSource, error) {
			userOptions := es.UserSearchOptions{
				ID:        &id,
				Name:      &name,
				Email:     &email,
				Page:      &page,
				PageSize:  &size,
				AscCreate: &asc,
			}
			return es.BuildUserElasticQuery(&userOptions)
		}, Fields: []string{"is_del", "user_id", "name", "email.keyword"}},
		{Name: "order", Build: func() (*elastic.SearchSource, error) {
			orderOptions := es.OrderSearchOptions{
				ID:         &id,
				OrderNo:    &orderNo,
				UserID:     &userID,
				ProductIDs: &productIDs,
				Page:       &page,
				PageSize:   &size,
				AscCreate:  &asc,
			}
			return es.BuildOrderElasticQuery(&orderOptions)
		}, Fields: []string{"is_del", "items.is_del", "ids", "order_no", "user_id", "items.product_id"}},
	} {
		t.Run(input.Name, func(t *testing.T) {
			source, err := input.Build()
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
			for _, field := range input.Fields {
				if !strings.Contains(string(raw), `"`+field+`"`) {
					t.Fatalf("missing %s: %s", field, raw)
				}
			}
			for _, part := range []string{`"from":20`, `"size":20`, `"update_time_stamp":{"order":"asc"}`} {
				if !strings.Contains(string(raw), part) {
					t.Fatalf("missing %s: %s", part, raw)
				}
			}
		})
	}
}

// TestUserOrderLookupUsesSearchSource 通过 ES HTTP 替身，验证按 ID 查询使用统一条件和排序，并正确解析结果。
func TestUserOrderLookupUsesSearchSource(t *testing.T) {
	httpClient := &http.Client{
		Transport: productSearchTransport(func(r *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			field := "id"
			if strings.Contains(r.URL.Path, es.ES_USER) {
				field = "user_id"
			}
			for _, part := range []string{`"size":1`, `"update_time_stamp":{"order":"desc"}`, `"` + field + `":{"order":"desc"}`, "9007199254740993"} {
				if !strings.Contains(string(raw), part) {
					t.Fatalf("missing %s: %s", part, raw)
				}
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_source":{"id":9007199254740993,"user_id":9007199254740993}}]}}`)), Request: r}, nil
		}),
	}
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	previous := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
	user, err := es.GetUserByID(9007199254740993)
	if err != nil || user.ID != 9007199254740993 {
		t.Fatalf("user lookup: %v %+v", err, user)
	}
	order, err := es.GetOrderByID(9007199254740993)
	if err != nil || order.ID != 9007199254740993 {
		t.Fatalf("order lookup: %v %+v", err, order)
	}
}

// TestNilSearchOptions 验证空查询条件指针被明确拒绝，不发生空指针异常。
func TestNilSearchOptions(t *testing.T) {
	for _, build := range []func() (*elastic.SearchSource, error){
		func() (*elastic.SearchSource, error) { return es.BuildUserElasticQuery(nil) },
		func() (*elastic.SearchSource, error) { return es.BuildOrderElasticQuery(nil) },
		func() (*elastic.SearchSource, error) { return es.BuildProductElasticQuery(nil) },
	} {
		if _, err := build(); err == nil {
			t.Fatal("空查询参数未被拒绝")
		}
	}
}
