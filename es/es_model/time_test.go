package es_model

import (
	"encoding/json"
	"testing"
	"time"
)

// 检查真正发送给 ES 的 JSON：展示时间必须带时区、保留毫秒，并与查询时间戳一致。
func TestDocumentTimeProtocol(t *testing.T) {
	created := time.Date(2026, 10, 15, 12, 21, 7, 135000000, time.FixedZone("Asia/Shanghai", 8*60*60))
	updated := created.Add(2*time.Minute + 321*time.Millisecond)
	for _, explicit := range []bool{false, true} {
		update := time.Time{}
		wantUpdate := created
		if explicit {
			update = updated
			wantUpdate = updated
		}
		user := &User{ID: 1, Name: "test", Age: 18, Email: "test@example.com", CreateTime: created, UpdateTime: update}
		product := &Product{ID: 1, Name: "test", Price: 100, Stock: 1, CreateTime: created, UpdateTime: update}
		order := &Order{ID: 1, UserID: 1, OrderNo: "test", AmountFen: 100, Status: 1, CreateTime: created, UpdateTime: update, Items: []OrderItem{{ID: 1, ProductID: 1, Quantity: 1, UnitPriceFen: 100, Status: 1}, {ID: 2, ProductID: 1, Quantity: 1, UnitPriceFen: 100, Status: 1, UpdateTime: updated.Add(time.Second)}}}
		for name, build := range map[string]func() (map[string]interface{}, error){"user": user.ToMap, "product": product.ToMap, "order": order.ToMap} {
			t.Run(name+"/"+wantUpdate.Format(time.RFC3339Nano), func(t *testing.T) {
				doc, err := build()
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				var wire struct {
					Created      string `json:"create_time"`
					Updated      string `json:"update_time"`
					CreatedStamp int64  `json:"create_time_stamp"`
					UpdatedStamp int64  `json:"update_time_stamp"`
					Items        []struct {
						Updated string `json:"update_time"`
						Stamp   int64  `json:"update_time_stamp"`
					} `json:"items"`
				}
				if err = json.Unmarshal(raw, &wire); err != nil {
					t.Fatal(err)
				}
				check := func(value string, stamp int64, want time.Time) {
					t.Helper()
					parsed, err := time.Parse(time.RFC3339Nano, value)
					if err != nil || !parsed.Equal(want) || stamp != want.UnixMilli() {
						t.Fatalf("invalid ES time %q, stamp=%d, expected=%s: %v", value, stamp, want, err)
					}
				}
				check(wire.Created, wire.CreatedStamp, created)
				check(wire.Updated, wire.UpdatedStamp, wantUpdate)
				if name == "order" {
					if len(wire.Items) != 2 {
						t.Fatal("missing items")
					}
					check(wire.Items[0].Updated, wire.Items[0].Stamp, wantUpdate)
					check(wire.Items[1].Updated, wire.Items[1].Stamp, updated.Add(time.Second))
				}
			})
		}
	}
}
