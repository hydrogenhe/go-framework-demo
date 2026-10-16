package redis

import (
	"encoding/json"
	"go-framework-demo/model"
	"strings"
	"testing"
)

func TestUserRecordCompatibility(t *testing.T) {
	for _, raw := range []string{`{"id":9007199254740993,"name":"test"}`, `{"id":"9007199254740993","name":"test"}`} {
		var record userRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			t.Fatal(err)
		}
		user, err := record.model()
		if err != nil || user.ID != 9007199254740993 {
			t.Fatalf("lost ID: %+v %v", user, err)
		}
	}
	raw, err := json.Marshal(userToRecord(&model.User{ID: 9007199254740993, Password: "secret-hash"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"id":9007199254740993`) || strings.Contains(string(raw), "password") || strings.Contains(string(raw), "secret-hash") {
		t.Fatalf("invalid cache: %s", raw)
	}
}
