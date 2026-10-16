package appconst

import "go-framework-demo/http_handler/http_msg"

// Response 保留旧调用方兼容；HTTP 接口使用 http_msg.Response[具体响应类型]。
type Response = http_msg.Response[any]
