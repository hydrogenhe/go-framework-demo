# 接口文档

正式页面：http://localhost:8080/swagger/index.html（后端重启后生效）。

## 文件位置

- `openapi.yaml`：集中维护的 OpenAPI 3.0.3 文档，是唯一编辑源。
- `openapi.json`：由 YAML 转换生成，供接口工具导入，不手工编辑。
- `index.html`：Swagger UI 页面，直接读取 `openapi.yaml`。
- `embed.go`：把文档和页面内嵌到后端，部署时不依赖启动目录。
- `cmd/api-docs/`：核对实际路由、检查文档内部引用并生成 JSON。

Handler 仅保留简短中文说明，不使用 `@Summary` 等注释，也不需要专用 Go 文档结构体。

## 更新文档

新增或修改接口时直接编辑 `docs/openapi.yaml`：请求参数在 `parameters` / `requestBody`，响应在 `responses`，共享结构在 `components.schemas`。

从项目根目录执行：

```bash
bash components_sh/generate_api_docs.sh
```

脚本不扫描业务注释，也不启动服务。它核对真实业务路由是否遗漏、文档是否包含未注册的接口、内部 schema 引用是否存在，然后生成 JSON。它不自动判断业务规则或字段变更，接口实现变更后仍需同步 YAML。

文档内嵌在编译产物中，更新后重新编译并重启后端。Swagger UI 静态资源随 Go 包提供，不依赖外部 CDN。

## 使用页面

1. 展开登录接口，在 Try it out 输入邮箱和密码，再 Execute。
2. 复制返回的 token，点击 Authorize，填入 token 本身；HTTP Bearer 认证会自动添加 `Bearer ` 前缀。
3. 管理员接口需要管理员账户；文档页面不会绕过后端权限。
4. 创建、修改、下单、上传、发货和注销操作会真实执行。仅阅读文档不会修改数据。

业务接口使用 PUT；ID 是字符串、金额是整数分；错误返回 `code/message`，成功响应按接口定义。部分更新未传/null 不改，显式零值参与更新。订单号最多25字符。Excel 导出返回附件。

普通网页不属于业务接口文档；`/metrics` 和 `/api/metrics` 包含在运维标签下。
