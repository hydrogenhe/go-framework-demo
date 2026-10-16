package http_msg

// Response 统一 JSON 协议。成功 code=0；Data 由每个接口指定具体响应类型。
type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}
type EmptyResp struct{}
