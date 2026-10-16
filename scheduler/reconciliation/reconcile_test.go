package reconciliation

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/olivere/elastic/v7"
	"go-framework-demo/es"
	"go-framework-demo/logger"
	"go-framework-demo/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type testDriver struct {
	query func(string, []driver.NamedValue) (driver.Rows, error)
}

func (d testDriver) Open(string) (driver.Conn, error) { return testConn{d}, nil }

type testConn struct{ testDriver }

func (testConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (testConn) Close() error                        { return nil }
func (testConn) Begin() (driver.Tx, error)           { return nil, errors.New("unexpected transaction") }
func (c testConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(q, args)
}

type testRows struct {
	columns []string
	values  [][]driver.Value
	pos     int
}

func (r *testRows) Columns() []string { return r.columns }
func (r *testRows) Close() error      { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.pos == len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.pos])
	r.pos++
	return nil
}

var driverID atomic.Int64

func fakeDB(t *testing.T, query func(string, []driver.NamedValue) (driver.Rows, error)) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("reconciliation_%d", driverID.Add(1))
	sql.Register(name, testDriver{query})
	conn, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fakeES(t *testing.T, fn func(*http.Request) (int, string)) {
	t.Helper()
	c, err := elastic.NewClient(elastic.SetURL("http://es.test"), elastic.SetSniff(false), elastic.SetHealthcheck(false), elastic.SetMaxRetries(0), elastic.SetHttpClient(&http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		status, body := fn(r)
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	prev := es.ElasticClient
	es.ElasticClient = c
	t.Cleanup(func() { c.Stop(); es.ElasticClient = prev })
}

type testTx struct{}

func (testTx) Commit() error                                                  { return nil }
func (testTx) Rollback() error                                                { return nil }
func (testConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) { return testTx{}, nil }

func readBulk(t *testing.T, r *http.Request) (map[string]map[string]any, string) {
	t.Helper()
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	updates := make(map[string]map[string]any)
	var items []map[string]any
	for {
		var meta map[string]map[string]any
		err := decoder.Decode(&meta)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for action, header := range meta {
			id := header["_id"].(string)
			if action != "delete" {
				var doc map[string]any
				if err := decoder.Decode(&doc); err != nil {
					t.Fatal(err)
				}
				updates[id] = doc
			} else {
				updates[id] = nil
			}
			items = append(items, map[string]any{action: map[string]any{"_id": id, "_index": "product_test", "status": 200, "result": action}})
		}
	}
	raw, err := json.Marshal(map[string]any{"errors": false, "items": items})
	if err != nil {
		t.Fatal(err)
	}
	return updates, string(raw)
}

func TestForwardProductsCompensatesWithoutRequery(t *testing.T) {
	calls := 0
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	db := fakeDB(t, func(q string, args []driver.NamedValue) (driver.Rows, error) {
		calls++
		if strings.Contains(q, "id IN") || !strings.Contains(q, "LIMIT ?") {
			t.Fatal("redundant query", q)
		}
		if calls == 1 {
			return &testRows{columns: []string{"id", "is_del", "name", "price", "stock", "image", "create_time"}, values: [][]driver.Value{
				{int64(1), int64(0), "first", int64(100), int64(2), "", now},
				{int64(2), int64(0), "second", int64(200), int64(3), "", now},
				{int64(3), int64(1), "", int64(0), int64(0), "", now},
			}}, nil
		}
		if calls != 2 || args[0].Value != int64(3) {
			t.Fatal(calls, args)
		}
		return &testRows{columns: []string{"id", "is_del"}}, nil
	})
	var updates map[string]map[string]any
	fakeES(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/_mget":
			return 200, `{"docs":[{"_id":"1","found":false},{"_id":"2","found":true,"_seq_no":2,"_primary_term":1,"_source":{"id":2,"is_del":0,"name":"WRONG","price":200,"stock":3,"image":"","create_time_stamp":1791936000000}},{"_id":"3","found":false}]}`
		case "/_bulk":
			var body string
			updates, body = readBulk(t, r)
			return 200, body
		}
		t.Fatal(r.URL)
		return 500, ""
	})
	core, logs := observer.New(zap.ErrorLevel)
	previous := logger.Log
	logger.Log = zap.New(core)
	t.Cleanup(func() { logger.Log = previous })
	if err := forwardProducts(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if len(updates) != 3 || updates["1"]["name"] != "first" || updates["2"]["name"] != "second" || updates["3"]["is_del"] != json.Number("1") {
		t.Fatal(updates)
	}
	if logs.Len() != 3 {
		t.Fatal(logs.All())
	}
}

func TestReverseProductsSeparatesUpdatesAndDeletes(t *testing.T) {
	for _, dbFails := range []bool{false, true} {
		t.Run(fmt.Sprint(dbFails), func(t *testing.T) {
			now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
			db := fakeDB(t, func(q string, args []driver.NamedValue) (driver.Rows, error) {
				if dbFails {
					return nil, errors.New("database unavailable")
				}
				if !strings.Contains(q, "id IN") || len(args) != 4 {
					t.Fatal(q, args)
				}
				return &testRows{columns: []string{"id", "is_del", "name", "price", "stock", "image", "create_time"}, values: [][]driver.Value{
					{int64(1), int64(0), "fixed", int64(100), int64(2), "", now},
					{int64(2), int64(1), "", int64(0), int64(0), "", now},
					{int64(4), int64(0), "same", int64(100), int64(2), "", now},
				}}, nil
			})
			pages, closed := 0, false
			actions := []map[string]map[string]any{}
			fakeES(t, func(r *http.Request) (int, string) {
				switch r.URL.Path {
				case "/product_test/_pit":
					return 200, `{"id":"pit"}`
				case "/_pit":
					closed = true
					return 200, `{"succeeded":true}`
				case "/_search":
					if pages > 0 {
						return 200, `{"hits":{"hits":[]}}`
					}
					pages++
					return 200, `{"hits":{"hits":[{"_id":"1","_seq_no":1,"_primary_term":1,"sort":[1],"_source":{"id":1,"is_del":0,"name":"WRONG","price":100,"stock":2,"image":"","create_time_stamp":1791936000000}},{"_id":"2","_seq_no":2,"_primary_term":1,"sort":[2]},{"_id":"3","_seq_no":3,"_primary_term":1,"sort":[3]},{"_id":"4","_seq_no":4,"_primary_term":1,"sort":[4],"_source":{"id":4,"is_del":0,"name":"same","price":100,"stock":2,"image":"","create_time_stamp":1791936000000}}]}}`
				case "/_bulk":
					updates, body := readBulk(t, r)
					actions = append(actions, updates)
					return 200, body
				}
				t.Fatal(r.URL)
				return 500, ""
			})
			err := reverseProducts(context.Background(), db)
			if dbFails {
				if err == nil || len(actions) != 0 {
					t.Fatal(err, actions)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if len(actions) != 2 || len(actions[0]) != 1 || actions[0]["1"]["name"] != "fixed" || len(actions[1]) != 2 {
					t.Fatal(actions)
				}
				for _, id := range []string{"2", "3"} {
					if value, ok := actions[1][id]; !ok || value != nil {
						t.Fatal(actions)
					}
				}
			}
			if !closed {
				t.Fatal("PIT leaked")
			}
		})
	}
}

func TestRunCompletesForwardBeforeReverse(t *testing.T) {
	var events []string
	db := fakeDB(t, func(q string, args []driver.NamedValue) (driver.Rows, error) {
		for _, table := range []string{"user", "product", "order"} {
			if strings.Contains(q, "`"+table+"`") {
				events = append(events, "forward:"+table)
				return &testRows{columns: []string{"id"}}, nil
			}
		}
		t.Fatal(q)
		return nil, nil
	})
	fakeES(t, func(r *http.Request) (int, string) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/_pit") {
			events = append(events, "reverse:"+strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "_test/_pit"))
			return 200, `{"id":"pit"}`
		}
		if r.URL.Path == "/_pit" {
			return 200, `{"succeeded":true}`
		}
		return 200, `{"hits":{"hits":[]}}`
	})
	if err := Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	expected := []string{"forward:user", "forward:product", "forward:order", "reverse:user", "reverse:product", "reverse:order"}
	if !reflect.DeepEqual(events, expected) {
		t.Fatal(events)
	}
}

func TestProductFieldsAndOrderItems(t *testing.T) {
	target := model.Product{ID: 9007199254740993, Name: "test", Price: 10, Stock: 0, CreateTime: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)}
	var source productSource
	if err := json.Unmarshal([]byte(`{"id":9007199254740993,"is_del":0,"name":"test","price":10,"stock":0,"image":"","create_time_stamp":1791936000000}`), &source); err != nil {
		t.Fatal(err)
	}
	if fields := compareProduct(&source, &target); len(fields) != 0 {
		t.Fatal(fields)
	}
	source.CreateTimeStamp = nil
	if fields := compareProduct(&source, &target); !reflect.DeepEqual(fields, []string{"create_time_stamp"}) {
		t.Fatal(fields)
	}
	stamp := target.CreateTime.UnixMilli()
	source.CreateTimeStamp = &stamp
	source.Stock = nil
	if fields := compareProduct(&source, &target); !reflect.DeepEqual(fields, []string{"stock"}) {
		t.Fatal(fields)
	}
	deleted := int64(1)
	source.IsDel = &deleted
	if fields := compareProduct(&source, &target); !reflect.DeepEqual(fields, []string{"is_del"}) {
		t.Fatal(fields)
	}
	targets := []*model.OrderItem{{ID: 1, ProductID: 10, Quantity: 2, UnitPriceFen: 100, Status: 1}, {ID: 2, ProductID: 20, Quantity: 3, UnitPriceFen: 200, Status: 1}}
	var sources []orderItemSource
	if err := json.Unmarshal([]byte(`[{"id":2,"is_del":0,"product_id":20,"quantity":3,"unit_price_fen":200,"status":1,"image":"","product_name":""},{"id":1,"is_del":0,"product_id":10,"quantity":2,"unit_price_fen":100,"status":1,"image":"","product_name":""}]`), &sources); err != nil {
		t.Fatal(err)
	}
	if fields := compareOrderItems(sources, targets); len(fields) != 0 {
		t.Fatal(fields)
	}
	quantity := int32(4)
	sources[1].Quantity = &quantity
	if fields := compareOrderItems(sources, targets); !reflect.DeepEqual(fields, []string{"items[id=1].quantity"}) {
		t.Fatal(fields)
	}
}
