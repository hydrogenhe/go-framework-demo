package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"io"
	"strconv"
	"time"
)

type canalEvent struct {
	Database  string                   `json:"database"`
	IsDDL     bool                     `json:"isDdl"`
	Data      []map[string]interface{} `json:"data"`
	Table     string                   `json:"table"`
	EventType string                   `json:"type"`
}

func int32Value(v interface{}) int32 { n, _ := int64Value(v); return int32(n) }
func timeValue(v interface{}) (time.Time, error) {
	s := stringValue(v)
	for _, layout := range []string{time.RFC3339Nano, time.DateTime, "2006-01-02 15:04:05.000"} {
		if t, e := time.ParseInLocation(layout, s, time.Local); e == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间 %q", s)
}

func stringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func int64Value(v interface{}) (int64, error) {
	switch value := v.(type) {
	case float64:
		return int64(value), nil
	case json.Number:
		return value.Int64()
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, fmt.Errorf("不支持的数字类型 %T", v)
	}
}

// 拒绝业务消息和表名不匹配的消息，避免将其误认为 CDC 同步成功。
func parseCanalEvent(msg *primitive.MessageExt, tables ...string) (*canalEvent, error) {
	if msg == nil {
		return nil, fmt.Errorf("CDC消息为空")
	}
	var event canalEvent
	decoder := json.NewDecoder(bytes.NewReader(msg.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&event); err != nil {
		return nil, fmt.Errorf("解析CDC消息失败: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("CDC消息包含多余内容")
	}
	allowed := false
	for _, table := range tables {
		if event.Table == table {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("CDC消息表名不匹配: %q", event.Table)
	}
	if event.IsDDL {
		return &event, nil
	}
	switch event.EventType {
	case "INSERT", "UPDATE", "DELETE":
	default:
		return nil, fmt.Errorf("不支持的CDC事件类型: %q", event.EventType)
	}
	if len(event.Data) == 0 {
		return nil, fmt.Errorf("CDC消息缺少data")
	}
	return &event, nil
}
