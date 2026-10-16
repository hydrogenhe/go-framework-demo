package model

import (
	"testing"
	"time"
)

func TestOrderBeforeCreateSetsPaymentDeadline(t *testing.T) {
	created := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	order := Order{CreateTime: created}
	if err := order.BeforeCreate(nil); err != nil {
		t.Fatal(err)
	}
	if want := created.Add(OrderPaymentWindow); !order.ExpireTime.Equal(want) {
		t.Fatalf("expire_time=%s, want %s", order.ExpireTime, want)
	}
}
