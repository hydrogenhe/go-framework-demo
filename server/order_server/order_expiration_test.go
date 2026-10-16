package order_server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/model"
	"go-framework-demo/rocketmq/message"
)

func TestScheduleSingleExpirationCheck(t *testing.T) {
	cases := []struct {
		window time.Duration
		level  int
		fail   bool
		calls  int
	}{
		{10 * time.Minute, 14, false, 1},
		{90 * time.Second, 6, false, 1},
		{12 * time.Minute, 15, false, 1},
		{time.Minute, 5, true, 1},
		{3 * time.Hour, 0, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.window.String(), func(t *testing.T) {
			created := time.Now()
			order := &model.Order{ID: 42, CreateTime: created, ExpireTime: created.Add(tc.window)}
			calls := 0
			sender := func(ctx context.Context, body []byte, level int) (*primitive.SendResult, error) {
				calls++
				event, err := message.ParseOrderMessage(string(body))
				if err != nil {
					t.Fatal(err)
				}
				if event.OrderID != "42" || event.Action != message.ActionExpire || level != tc.level {
					t.Errorf("event=%+v level=%d", event, level)
				}
				if tc.fail {
					return nil, errors.New("send failed")
				}
				return &primitive.SendResult{}, nil
			}
			sendOrderExpirationCheck(context.Background(), order, sender)
			if calls != tc.calls {
				t.Errorf("sent %d messages; want %d", calls, tc.calls)
			}
		})
	}
}
