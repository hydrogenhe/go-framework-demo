package model

import (
	"encoding/json"
	"testing"
)

// 内部模型的 ID 保持数字，HTTP 字符串协议由 http_msg 负责。
func TestModelIDsRemainIntegers(t *testing.T) {
	for _, v := range []any{User{ID: 9007199254740993}, Product{ID: 9007199254740993}, Order{ID: 9007199254740993}, OrderItem{ID: 9007199254740993}} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(b, &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields["id"]) != "9007199254740993" {
			t.Fatalf("unexpected ID: %s", b)
		}
	}
}
