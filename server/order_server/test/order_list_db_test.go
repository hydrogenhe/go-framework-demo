package order

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/olivere/elastic/v7"
	appconst "go-framework-demo/const"
	"go-framework-demo/es"
	"go-framework-demo/model"
	"go-framework-demo/query"
	"go-framework-demo/server/order_server"
	"go-framework-demo/server/test/support"
)

type orderIDTransport func(*http.Request) (*http.Response, error)

func (f orderIDTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestIntegrationOrderListReadsMySQL 调用真实 Service：详情来自 MySQL，批量回查保持 ES 顺序，过滤删除、其他用户和已变更状态。
func TestIntegrationOrderListReadsMySQL(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		userInput := support.RandomUserInput()
		user := &model.User{
			Name:     userInput.Name,
			Email:    userInput.Email,
			Age:      userInput.Age,
			Password: userInput.Password,
		}
		if err := q.User.WithContext(ctx).Create(user); err != nil {
			t.Fatal(err)
		}
		targets := []*model.Order{
			{OrderNo: "list_first", UserID: user.ID, Amount: 100, Status: 2},
			{OrderNo: "list_second", UserID: user.ID, Amount: 200, Status: 2},
			{OrderNo: "list_deleted", UserID: user.ID, Amount: 100, Status: 2},
			{OrderNo: "list_changed", UserID: user.ID, Amount: 100, Status: 3},
			{OrderNo: "list_other", UserID: user.ID + 1, Amount: 100, Status: 2},
		}
		if err := q.Order.WithContext(ctx).Create(targets...); err != nil {
			t.Fatal(err)
		}
		if _, err := q.Order.WithContext(ctx).Where(q.Order.ID.Eq(targets[2].ID)).Delete(); err != nil {
			t.Fatal(err)
		}
		product := &model.Product{Name: "order_list_product", Price: 100, Stock: 1}
		if err := q.Product.WithContext(ctx).Create(product); err != nil {
			t.Fatal(err)
		}
		item := &model.OrderItem{
			OrderID:      targets[1].ID,
			ProductID:    product.ID,
			Quantity:     2,
			UnitPriceFen: 100,
			Status:       1,
			Image:        "db-image",
		}
		if err := q.OrderItem.WithContext(ctx).Create(item); err != nil {
			t.Fatal(err)
		}
		httpClient := &http.Client{Transport: orderIDTransport(func(r *http.Request) (*http.Response, error) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["_source"] != false {
				t.Fatal("ES must return IDs only")
			}
			response := fmt.Sprintf(`{"hits":{"total":{"value":5,"relation":"eq"},"hits":[{"_id":"%d"},{"_id":"%d"},{"_id":"%d"},{"_id":"%d"},{"_id":"%d"}]}}`, targets[1].ID, targets[2].ID, targets[3].ID, targets[4].ID, targets[0].ID)
			return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(response))}, nil
		})}
		client, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetHttpClient(httpClient))
		if err != nil {
			t.Fatal(err)
		}
		previous := es.ElasticClient
		es.ElasticClient = client
		defer func() { client.Stop(); es.ElasticClient = previous }()
		status := appconst.OrderStatusPaid
		input := order_server.OrderQueryInput{
			UserID:   user.ID,
			Status:   &status,
			Page:     1,
			PageSize: 20,
		}
		service := order_server.NewOrderService(ctx, q)
		result, err := service.List(ctx, &input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 5 || len(result.Items) != 2 || result.Items[0].ID != targets[1].ID || result.Items[1].ID != targets[0].ID {
			t.Fatalf("wrong order or filtering: %+v", result)
		}
		if result.Items[0].Amount != 200 || len(result.Items[0].Items) != 1 || result.Items[0].Items[0].Image != "db-image" {
			t.Fatal("MySQL details not returned", result.Items[0])
		}
		adminInput := order_server.OrderQueryInput{
			Status:   &status,
			Page:     1,
			PageSize: 20,
		}
		adminResult, err := service.ListAdmin(ctx, &adminInput)
		if err != nil || len(adminResult.Items) != 3 {
			t.Fatalf("admin list: %+v %v", adminResult, err)
		}
	})
}
