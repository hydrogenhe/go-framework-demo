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
	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
)

func TestProductWriteUsesMillisecondStamp(t *testing.T) {
	created := time.Date(2026, 10, 13, 22, 27, 20, 135000000, time.FixedZone("Asia/Shanghai", 8*60*60))
	httpClient := &http.Client{Transport: productSearchTransport(func(r *http.Request) (*http.Response, error) {
		var doc struct {
			Stamp   int64  `json:"create_time_stamp"`
			Display string `json:"create_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
			t.Fatal(err)
		}
		if doc.Stamp != created.UnixMilli() || doc.Display != "2026-10-13T22:27:20.135+08:00" {
			t.Fatalf("unexpected creation time: %+v", doc)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(`{"_id":"1","result":"created"}`))}, nil
	})}
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	previous := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
	product := es_model.Product{
		ID:         1,
		Name:       "商品",
		Price:      100,
		Stock:      1,
		CreateTime: created,
	}
	if _, err := es.IndexProductContext(context.Background(), &product); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTimeStampMapping(t *testing.T) {
	updated := make(map[string]bool)
	httpClient := &http.Client{Transport: productSearchTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			var body struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Properties["create_time_stamp"].Type != "long" {
				t.Fatal(body)
			}
			updated[r.URL.Path] = true
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(`{"acknowledged":true}`))}, nil
	})}
	client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	previous := es.ElasticClient
	es.ElasticClient = client
	t.Cleanup(func() { client.Stop(); es.ElasticClient = previous })
	if err := es.EnsureCreateTimeStampMapping(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, index := range []string{es.ES_USER, es.ES_PRODUCT, es.ES_ORDER} {
		if !updated["/"+index+"/_mapping"] {
			t.Fatal("missing mapping", index)
		}
	}
}
