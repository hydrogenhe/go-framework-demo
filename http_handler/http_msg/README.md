# HTTP 协议

本目录是前后端通信协议的定义入口，不引用数据库模型或服务层类型。

- `product.go`：商品创建、更新、列表/导出筛选、图片上传。
- `order.go`：下单、订单查询、订单和明细响应。
- `user.go`：注册、登录和会话响应。
- `log.go`：日志查询请求及分页响应。
- `metrics.go`：指标查询响应。
- `response.go`：统一外层 `Response[T]`，`T` 是明确的业务响应类型。

JSON 接口成功返回 `{ "code": 0, "message": "success", "data": ... }`；失败返回非零业务 code 和 message，并使用对应 HTTP 错误状态码。退出登录返回 HTTP 200，data 为 `{}`。前端检查 HTTP 状态和 code 后，只读取 data。

所有 ID（包括 user_id、product_id、order_id）使用十进制字符串；内部模型保持 int64。金额使用整数分，时间使用带时区的 RFC3339，空列表返回 []。数据库字段 is_del 和密码不进入响应协议。商品更新字段缺省或 null 表示不修改，零值参与更新。

路由绑定 Req 后，显式构建 model 或服务输入；调用服务后在 HTTP 层映射为 Resp。`response.go`（父目录）集中维护返回字段映射，不嵌入整个数据库模型。Redis 有独立存储协议，不复用 HTTP DTO。

列表与导出仍通过 URL query 传递筛选参数；JSON 创建/更新请求和 multipart 图片上传保持现有请求方式。Excel 下载和 Prometheus 文本不包装为 JSON。

完整路径、状态码和 data schema 见 `docs/openapi.yaml`；执行 `go run ./cmd/api-docs` 校验路由并更新 Swagger JSON。
