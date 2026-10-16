# 监控中间件

Go 业务服务由项目本身启动；本目录脚本只负责启动 Prometheus 和 Grafana，不会启动或停止 Go 服务。

一键启动：

```bash
./manage.sh
```

启动前请确保 Docker Desktop 已运行。

从项目根目录重启接口监控（同时支持首次启动，保留数据卷）：

```bash
bash components_sh/restart_api_monitoring.sh
```


脚本复用本目录 Compose 配置，重新创建 Prometheus 和 Grafana 容器，并检查服务就绪状态；不会重启 Go 服务。服务就绪不代表指标采集正常，请在 Prometheus 的 Targets 页面确认 Go 服务为 UP。当前仅配置 Grafana 数据源，尚未预置接口监控看板。

地址：

- Prometheus：http://localhost:9090
- Grafana：http://localhost:3000
- Go 指标：http://localhost:8080/metrics


## 按 HTTP 状态码监控接口

`http_requests_total` 使用 `route` 和 `status` 标签。`http_request_errors_total` 仍只统计 5xx；未匹配的路由统一记为 `unmatched`，避免任意 URL 产生无限标签。修改 Go 指标代码后需要重启后端，监控组件无需重启。

以下查询排除 `/metrics` 自身采集请求。首次启动或没有流量时，可能暂时无数据。

各接口 4xx 每秒请求数：

```promql
sum by (route) (rate(http_requests_total{job="go-framework-demo",route!="/metrics",status=~"4.."}[5m]))
```

各接口 5xx 每秒请求数：

```promql
sum by (route) (rate(http_requests_total{job="go-framework-demo",route!="/metrics",status=~"5.."}[5m]))
```

各接口成功率（此处将 2xx、3xx 定义为成功，结果为 0–100；无流量时不显示百分比）：

```promql
100 *
(sum by (route) (rate(http_requests_total{job="go-framework-demo",route!="/metrics",status=~"[23].."}[5m]))
 or on (route)
 (0 * sum by (route) (rate(http_requests_total{job="go-framework-demo",route!="/metrics",status!=""}[5m]))))
/
(sum by (route) (rate(http_requests_total{job="go-framework-demo",route!="/metrics",status!=""}[5m])) > 0)
```

成功率查询中的 `status!=""` 排除代码升级前无状态码标签的旧时间序列。该成功率按 HTTP 状态定义；如果业务失败也返回 HTTP 200，需要额外的业务结果指标。
