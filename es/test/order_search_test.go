package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/olivere/elastic/v7"
	appconst "go-framework-demo/const"
	"go-framework-demo/es"
)

func TestOrderIDsFiltersAndSort(t *testing.T) {
	userID := int64(9007199254740993)
	status := appconst.OrderStatusPaid
	min, max := int64(0), int64(1299)
	from := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	page, size := 2, 20
	for _, sort := range []string{"newest", "price_asc", "price_desc"} {
		t.Run(sort, func(t *testing.T) {
			httpClient := &http.Client{Transport: productSearchTransport(func(r *http.Request) (*http.Response, error) {
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if string(body["_source"]) != "false" || string(body["track_total_hits"]) != "true" || string(body["from"]) != "20" || string(body["size"]) != "20" {
					t.Fatal(body)
				}
				query := string(body["query"])
				for _, fragment := range []string{`"user_id":"9007199254740993"`, `"status":2`, `"is_del":0`, `"amount_fen"`, `"from":0`, `"to":1299`, `"create_time_stamp"`} {
					if !strings.Contains(query, fragment) {
						t.Fatalf("missing %s: %s", fragment, query)
					}
				}
				sorts := string(body["sort"])
				if sort == "newest" && !strings.HasPrefix(sorts, `[{"update_time_stamp":{"order":"desc"}}`) {
					t.Fatal(sorts)
				}
				if sort != "newest" {
					order := "desc"
					if sort == "price_asc" {
						order = "asc"
					}
					if !strings.HasPrefix(sorts, `[{"amount_fen":{"order":"`+order+`"}}`) {
						t.Fatal(sorts)
					}
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(`{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_id":"9007199254740995"}]}}`))}, nil
			})}
			client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(httpClient))
			if err != nil {
				t.Fatal(err)
			}
			previous := es.ElasticClient
			es.ElasticClient = client
			defer func() { client.Stop(); es.ElasticClient = previous }()
			input := es.OrderSearchOptions{
				UserID:      &userID,
				Status:      &status,
				MinPrice:    &min,
				MaxPrice:    &max,
				CreatedFrom: &from,
				CreatedTo:   &to,
				Sort:        sort,
				Page:        &page,
				PageSize:    &size,
			}
			ids, total, err := es.GetOrderIDsPage(context.Background(), &input)
			if err != nil || total != 1 || len(ids) != 1 || ids[0] != 9007199254740995 {
				t.Fatal(ids, total, err)
			}
		})
	}
}
