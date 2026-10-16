# go-framework-demo

这是一个 Go + Gin + MySQL + Redis + RocketMQ + Canal + Elasticsearch + SeaweedFS 的示例项目。

## 各组件的业务作用

| 组件 | 在项目中负责什么 |
|---|---|
| Go / Gin 后端 | 提供注册、登录、商品管理和订单接口，校验身份与业务参数，编排订单事务，并运行 MQ 消费者。 |
| MySQL | 保存用户、商品、库存、订单及订单明细，是业务数据的最终依据；下单时核对商品数据并在事务中更新库存。 |
| Redis | 保存登录会话和用户查询缓存，让后端能够校验登录状态、减少重复查询数据库。 |
| Canal | 监听 MySQL Binlog，将用户、商品和订单等表的变更转换为 CDC 消息，投递到对应 MQ Topic。 |
| RocketMQ | 传递业务事件和 CDC 消息，支持异步消费与失败重试；两类消息使用独立 Topic 和 Handler。NameServer 提供路由，Broker 保存和投递消息。 |
| Elasticsearch | 保存 MySQL 数据的搜索副本，支持商品列表、筛选、分页及排序，也提供用户和订单的 ES 查询能力；数据通过 CDC 异步同步。 |
| SeaweedFS mini | 保存商品图片文件，提供 S3 上传和 HTTP 图片读取；MySQL、ES 只保存图片地址。 |
| Docker Desktop | 提供容器运行环境，运行 Canal、SeaweedFS 及可选监控组件。 |
| Prometheus | 采集后端 `/metrics` 指标，用于观察请求量、错误和耗时等运行情况。 |
| Grafana | 将 Prometheus 指标展示为监控看板，方便查看服务运行情况。 |
| 前端 | 提供管理员商品管理、图片上传以及用户选购和订单页面，通过 HTTP 接口调用后端。 |

## 数据同步目标

业务数据以 MySQL 为最终事实来源。商品、订单和用户发生变化后，通过 MySQL Binlog 捕获变更，再投递到 RocketMQ，最后由 Go 消费者同步到 Elasticsearch：

```text
MySQL Binlog → Canal → RocketMQ → Go ES Consumer → Elasticsearch
```

这样 HTTP 业务接口只负责写 MySQL，不直接承担 ES 同步；ES 同步失败时可以依靠消息重试和消费位点恢复。

## 云服务器部署：运行时只需修改 YAML

在开发机执行 `./components_sh/Internet/package.sh`，默认生成 Linux x86_64 静态可执行部署包 `dist/go-framework-demo.tar.gz`。将压缩包上传服务器并解压后，按服务器环境修改部署目录 `conf/` 中的 YAML 配置；不需要在服务器修改或编译 Go 代码，也不需要安装 Go。每类配置只有一份 YAML，本机和云端使用相同文件。

至少检查并按实际环境填写：`app.yaml`（后端监听、前端 API 公网地址和前端监听、日志目录、Snowflake machine_id）、`mysql.yaml`、`redis.yaml`、`es.yaml`、`rocketmq.yaml` 和 `image_storage.yaml`。特别是数据库密码、ES/MQ 地址及图片存储凭据需填写真实部署值；图片配置模板中的 `replace-me` 不能直接用于生产。前端静态服务启动时会从 `app.yaml` 重新生成 API 运行配置，因此改完 YAML 重启服务即可。

运行后端和前端静态服务（分别启动；可交由 systemd、Supervisor 等进程管理器守护）：

```bash
cd go-framework-demo
./bin/go-framework-demo
# 另一个进程
./bin/frontend-server
```

项目会从当前目录或二进制所在目录逐级向上查找 `conf/`，因此无需固定安装路径。若服务器是 ARM64，可在打包机执行 `GOARCH=arm64 ./components_sh/Internet/package.sh` 后上传对应包。MySQL、Redis、ES、RocketMQ 和对象存储等外部服务仍需先行部署并可达；Canal 若用于 CDC 同步，也须在服务器单独部署和配置。

## 项目启动依赖与就绪检查

以下地址对应当前本机 Demo 配置。运行项目时先让依赖服务就绪，再启动 Go 后端，最后启动前端。长期运行的本机进程在自己的终端运行；Docker 容器由 Docker 管理。

| 组件 | 用途 | 本机端口 | 就绪检查 |
|---|---|---|---|
| MySQL 8.0 | 用户、商品、订单事实数据 | 3306 | `mysqladmin ping`；业务还需 `demo_db` 表结构与 Binlog 配置 |
| Redis | 登录会话、缓存 | 6379 | `redis-cli ping` 返回 `PONG` |
| Elasticsearch 7.x | 商品搜索及用户、订单查询副本 | 9200 | HTTP 返回正常，索引和集群健康可查询 |
| RocketMQ NameServer / Broker | 业务事件和 CDC 消息 | 9876 / 10911 | 两端口可达，Broker 注册到 NameServer，Topic 路由存在 |
| Canal | 读取 MySQL Binlog，投递 CDC 到 MQ | 11111 | 容器运行、端口监听，日志确认连接 MySQL/MQ、无持续投递错误 |
| SeaweedFS mini | 商品图片对象存储 | 8333 / 8888 / 9333 | S3 响应、Filer 的 `product-images` Bucket 可读、Master 响应 |
| Docker Desktop | 运行 Canal、SeaweedFS 和监控容器 | — | `docker info` 正常 |
| Prometheus / Grafana（可选） | 指标采集与监控看板 | 9090 / 3000 | `/-/ready` 与 `/api/health` 正常 |
| Go 后端 | HTTP API 与 MQ 消费者 | 8080 | 服务初始化成功、端口监听，业务请求正常 |
| 前端 | 管理端和用户端页面 | 5500 | 页面可访问、能够调用后端 |

MySQL、Redis、ES、RocketMQ 是后端启动依赖；Canal 决定 MySQL 到 ES 的异步同步能否工作；SeaweedFS 决定上传和展示图片能否工作；Prometheus、Grafana 不影响业务服务启动。

### 1. 启动 Docker 与基础数据库

先打开 Docker Desktop，确认：

```bash
docker info
```

当前本机安装的 MySQL 可使用下面的命令启动；Redis、ES 可分别在独立终端前台运行，已经启动的服务不必重复启动：

```bash
mysql.server start
redis-server
elasticsearch
```

上面三个命令分别执行；`redis-server` 和 `elasticsearch` 会持续占用各自终端。数据库连接配置见 `conf/mysql.yaml`，ES 配置见 `conf/es.yaml`。MySQL 除启动进程外，还需业务表和 Canal 所需的 ROW 格式 Binlog、server-id、读取账号等配置，不能仅凭端口开放判断同步可用。

### 2. 启动 RocketMQ 并确认 Topic

在自己的终端执行项目脚本：

```bash
./components_sh/restart_mq.sh
```

脚本每次启动时会从当前活动的 Wi-Fi/以太网网卡获取 IPv4 地址，并更新 `/usr/local/rocketmq/conf/broker.conf` 中的 `brokerIP1`。Canal 运行在 Docker 容器内，不能通过 `127.0.0.1` 访问宿主机 Broker；NameServer 返回的 Broker 地址必须是 Canal 容器可达的宿主机地址。若 Canal 容器正在运行，脚本会额外检查它能否连接该 Broker 地址的 `10911` 端口。

如果 Mac 同时连接多个有线或无线网络，可明确指定用于 Broker 通告的网卡或地址：

```bash
BROKER_ADVERTISE_INTERFACE=en0 ./components_sh/restart_mq.sh
# 也可直接指定本次通告的 IPv4 地址
BROKER_ADVERTISE_IP=192.168.11.72 ./components_sh/restart_mq.sh
```

自动获取只在脚本启动时生效。若 Broker 运行期间切换网络或 IP 改变，NameServer 中仍可能保留旧地址；需要重新运行脚本，让 Broker 用新地址注册。VPN 若阻断 Docker 到宿主机的局域网访问，也会导致脚本中的 Canal 连通性检查失败。

MQ 配置见 `conf/rocketmq.yaml`，当前使用以下 Topic：

```text
user_topic、order_topic、product_topic
user_cdc_topic、order_cdc_topic、product_cdc_topic
```

Canal 启动前必须确认 CDC Topic 已注册；Go 消费者启动前也需要对应 Topic 路由。例如检查商品同步 Topic：

```bash
/usr/local/rocketmq/bin/mqadmin clusterList -n 127.0.0.1:9876
/usr/local/rocketmq/bin/mqadmin topicRoute -n 127.0.0.1:9876 -t product_cdc_topic
```

### 3. 启动 Canal 和图片组件

```bash
./components_sh/restart_canal.sh
./components_sh/restart_image.sh
```

Canal 脚本会重建容器并挂载 `component/canal/canal_conf/` 项目配置；不要使用未挂载配置的裸 `docker run` 代替。

SeaweedFS 脚本首次生成随机密钥和后端配置，重建容器但保留 `component/Image_component/data/`。S3 上传端口为 8333，图片读取端口为 8888，管理界面为 http://localhost:23646。详细说明见 [图片组件文档](component/Image_component/README.md)。

检查运行状态和日志：

```bash
docker ps --filter name=canal-server
docker ps --filter name=demo-seaweedfs
docker logs --tail 100 canal-server
docker logs --tail 50 demo-seaweedfs
```

### 4. 统一检查依赖就绪

```bash
./components_sh/ping.sh
```

脚本只读检测 Docker、MySQL、Redis、ES、RocketMQ、Canal、SeaweedFS，以及可选的 Prometheus、Grafana，不会启动或停止服务。SeaweedFS 检查 S3 响应、Filer Bucket 和 Master；S3 匿名请求返回 403 是启用访问密钥后的正常响应，不按故障处理。

RocketMQ 优先通过 `mqadmin` 检查 Broker 注册；找不到该工具时降级检查 NameServer/Broker 端口。Canal 只检查 TCP 监听，所以仍需结合日志及实际创建商品后的 ES 同步测试确认业务链路。脚本输出各项状态，不以退出码表示所有依赖都健康。

若业务依赖检查失败，先修复对应组件再启动后端；监控组件未启动可以暂时忽略。在 Codex 沙箱内出现连接权限限制不代表服务故障，应在自己的终端检查或放行只读检测权限。

### 5. 可选监控组件

```bash
./component/monitoring/manage.sh
```

监控可以在后端启动前或后启动；只有后端的 `/metrics` 能访问后，Prometheus 才能成功采集项目指标。详见 [监控文档](component/monitoring/README.md)。

## 启动后端和前端

构建后端：

```bash
./components_sh/build.sh
```

启动后端（端口 `8080`）：

```bash
./components_sh/start.sh
```

启动前端（端口 `5500`）：

```bash
./components_sh/restart_frontend.sh
```

访问前端：

```text
http://localhost:5500
```

后端 API 地址：

```text
http://localhost:8080
```

## 常用 Docker 指令

```bash
# 查看所有容器
docker ps -a

# 查看 Canal 最近日志
docker logs --tail 100 canal-server

# 重启 Canal
docker restart canal-server

# 停止并删除 Canal 容器
docker rm -f canal-server

# 查看镜像
docker images canal/canal-server
```

Canal 当前使用项目内的 MySQL Binlog 与 MQ 路由配置。容器处于 running 状态只说明进程存活，同步链路仍需结合日志和实际业务数据验证。

### 订单创建时间迁移

运行本次更新的后端前，先执行：

```bash
go run ./cmd/init-order-time -apply
```

该工具新增订单 `create_time`，只初始化缺失时间，重复执行保留已有值。旧表没有历史创建时间，因此旧订单一次性使用迁移时间；新订单在创建时保存时间，后续状态更新和 ES 同步保持不变。本机数据库已完成迁移，新环境仍需执行。业务校验及下单事务位于 `server/`，DB 创建用户和订单的方法接收准备好的模型。

测试数据需要统一覆盖为当前时间并同步 ES 时，执行 `go run ./cmd/init-order-time -apply -reset`。本机已覆盖 42 条 MySQL 订单和 ES 中现有的 8 条订单文档，并核对时间一致。

### RocketMQ 消息页面

`components_sh/restart_mq.sh` 同时启动 RocketMQ Dashboard，访问 http://localhost:8082，在 Message 页面按 Topic 和时间查询每条消息。配置与说明位于 [component/rocketmq-dashboard](component/rocketmq-dashboard/README.md)。

### 商品筛选与 Excel 导出

- 商品列表支持名称包含匹配、创建时间起止、库存上下限和价格上下限。前端价格输入使用元，接口 `price_min`、`price_max` 使用整数分；时间参数 `created_from`、`created_to` 使用带时区的 RFC3339 格式，范围包含端点。空条件不参与筛选。
- `PUT /api/products/list` 返回筛选后的分页结果；管理员通过 `PUT /api/products/export` 导出相同条件的全部结果，导出忽略 `page`、`page_size`。两者都先通过 ES 筛选 ID，再批量读取 MySQL 最新详情；数据库中已删除或不存在的商品不导出。
- Excel 包含商品 ID、名称、价格（元/分）、库存、图片 URL、创建时间、删除标记。ID 按文本写入避免精度丢失；导出分批写入临时文件，下载结束后删除。超过 Excel 单表行数时自动分表。
- 遍历方案开关位于 `es/product_export.go`：`ProductExportUseSearchAfter = false` 默认使用 Scroll；改为 `true` 后使用 `search_after + PIT`。开关在启动前设置，运行时不修改。两种方案都不受普通分页 10000 条窗口限制，并在结束时清理 Scroll/PIT。

```bash
# 名称包含 hwt，库存 0–200，价格 10–200 元
curl -X PUT 'http://localhost:8080/api/products/list?name=hwt&stock_min=0&stock_max=200&price_min=1000&price_max=20000&page=1&page_size=20'

# 导出使用相同筛选参数，另带管理员 Bearer token；前端按钮会自动下载 .xlsx。
```

### 服务层测试运行方式

订单、商品、用户测试分别位于 `server/order_server/test/`、`server/product_server/test/`、`server/user_server/test/`；日志测试位于 `server/test/log/`，跨业务测试和共享辅助分别位于 `server/test/testdata/`、`server/test/support/`。业务包分别初始化依赖，商品和订单测试只启动各自的 CDC 消费组，测试结束后关闭 MQ。

```bash
# 全部服务层测试（包含 Go 的 ./... 默认跳过的 testdata 包）
bash test_sh/test_server.sh

# 只运行商品同步测试
go test ./server/product_server/test -run '^TestProductCreateES$' -count=1 -v

# 事务回滚模式验证商品部分更新，不启动 ES/MQ
DEMO_INTEGRATION=1 DEMO_TEST_DATABASE_ONLY=1 bash test_sh/test_server.sh \
  -run '^TestIntegrationProductUpdateValidation$'
```
