package handler_test

// canalEvent 仅描述测试输入的 Canal JSON 协议，业务解析由真实消费 Handler 完成。
type canalEvent struct {
	Database  string                   `json:"database"`
	IsDDL     bool                     `json:"isDdl"`
	Data      []map[string]interface{} `json:"data"`
	Table     string                   `json:"table"`
	EventType string                   `json:"type"`
}
