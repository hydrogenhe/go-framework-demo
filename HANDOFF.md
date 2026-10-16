# go-framework-demo 项目交接与指导

项目路径：`/Users/hewen/GoProjects/src/go-framework-demo`

最近代码核对：2026-10-14。本次核对覆盖仓库文件、主要业务实现、构建及部分隔离测试，未重新检查本机服务或真实业务数据。下文带日期的数据数量、迁移结果和运行记录属于历史验证，不能作为当前环境状态；涉及启动时，以实际配置、进程和端口检查为准。

这是一个 Go 基础设施整合 Demo，当前包含 Gin HTTP 服务、MySQL、Redis、Elasticsearch、RocketMQ、Canal 和独立前端。目标是演示清晰的业务分层、事务处理、缓存、搜索和消息驱动的数据同步。

## 一、当前项目结构

```text
main.go                 程序入口，初始化日志、MySQL、Redis、ES、图片存储、RocketMQ 和 Gin
http_handler/           HTTP 路由和请求处理，只负责参数解析、校验和响应
server/                 业务服务层，编排事务、数据库、缓存、搜索和消息
db/                     数据访问层，GORM Gen 查询和 CRUD
mysql/                  MySQL 初始化、Gen Query 获取
model/                  GORM Gen 数据模型
query/                  GORM Gen 查询对象
es/                     Elasticsearch 初始化、模型、写入和查询
redis/                  Redis 初始化和缓存封装
rocketmq/               RocketMQ 配置、Producer、Consumer 和业务 Handler
const/                  错误码、统一响应和业务状态枚举
util/                   配置路径、ID、JSON 等通用工具
conf/                   ES、RocketMQ 等配置文件
components_sh/           构建、启动、重启和依赖检查脚本
component/               图片组件、监控和 Canal
frontend/                独立部署的前端页面
cmd/generate/            GORM Gen 数据库代码生成命令
cmd/                    API 文档生成、Demo ES 校准及 MySQL→ES 对账工具
docs/                   OpenAPI YAML、生成的 JSON、内嵌 Swagger UI
db/test/、es/test/       数据库和 ES 测试
server/test/            按业务拆分的服务层测试，含需显式运行的 testdata 包
http_handler/test/      HTTP、认证、权限及接口协议测试
```

## 二、已经完成的功能

### 数据库和事务

- MySQL 已接入 `user`、`product`、`order`、`order_item` 等业务表。
- GORM Gen 已生成 `model/` 和 `query/` 代码。
- `NewUserDb`、`NewOrderDb`、`NewOrderItemDb` 支持 `context.Context` 和可选事务 Query。
- 传入 `txQ` 时直接使用传入的事务 Query；不再从 ctx 中读取事务 Query。
- Server 统一通过 `db.WithTx` 开启事务，封装内绑定请求 Context，回调显式接收 `ctx` 和 `txQ`。可选第三个参数用于沿用外层 Query，保证事务隔离和回滚。
- ID 统一使用 `util.GetID()`。
- 金额使用整数分保存。
- 手写 DB 操作将表查询对象提取成局部变量，避免重复的 `GenQuery.Product` 等长引用；详见 `Project_Handbook.md` 第 2.3、5.4 节。
- 密码入库前进行 bcrypt 处理；注册密码长度为 6–18 位，密码和哈希均不返回前端。
- 订单和订单明细状态使用 `const/status.go` 中的 int 枚举。

### HTTP 和前端

- 使用 Gin 处理 HTTP 请求。
- 路由和 Handler 位于 `http_handler/`，当前包含用户、商品、订单相关框架。
- 前后端分离：前端由独立静态服务器提供，后端只提供 API。
- 前端默认 API 地址是 `http://localhost:8080`，前端开发服务器默认是 `http://localhost:5500`。
- Gin 已处理跨域。
- 前端登录会话使用 localStorage 保存，刷新页面不会直接丢失登录状态。
- 前端 API 已统一处理成功响应和错误响应。

### 统一错误和响应

- `const/error.go` 定义 `MsgError` 和错误码。
- 所有参数校验、业务判断失败必须在对应 Service 中用明确的业务 `err` 包装，携带 `code` 和 `message`（代码字段为 `Code` / `Message`），供前端区分和展示错误。
- 其他错误统一视为系统内部错误；HTTP 响应层只识别业务错误类型，不得通过错误字符串匹配推断错误码或业务含义。DB 保留原始错误链，Service 使用错误类型处理明确的业务结果。
- `const/response.go` 定义统一响应结构。`Project_Handbook.md` 第 4.2 节说明 `MsgError` 字段、错误转换和 Service / HTTP 响应示例，按这些示例实现错误处理。
- 业务校验错误可以返回给前端，例如参数错误、用户不存在、库存不足。
- 系统错误、数据库错误、连接超时等内部错误不应把底层细节直接暴露给前端，应记录日志并返回通用提示。
- 前端约定使用错误码和用户可见消息：实际 JSON 字段为 `code`、`message`、可选 `data`；文中的 msg 仅指消息含义，不是协议字段名。

### Elasticsearch

- ES 使用 `olivere/elastic/v7`。
- 已有用户、订单、商品 ES 模型和操作封装。
- 商品索引为 `product_test`，用户、订单分别有对应索引。
- 商品 CDC Handler 从消息提取主键，回查 MySQL 最新记录（包含软删除状态）后同步 ES 和 Redis；软删除/物理删除时在 ES 写墓碑并清理 Redis，避免延迟事件中的旧字段覆盖事实数据。
- 商品 CDC 对仍有效的商品（新增或更新）均将 MySQL 最新状态写入 `product:<id>`，并刷新 5 小时 TTL；删除时清理该 key。当前商品列表仍以 ES 筛选、MySQL 回查为准，尚无按 ID 读取时的 Redis cache-aside 路径。

### RocketMQ、Canal 和 ES 同步

目标链路：

```text
MySQL Binlog
    ↓
Canal
    ↓ product_cdc_topic
本机 RocketMQ
    ↓
Go product_es_consumer_group
    ↓
Elasticsearch product_test
```

当前使用 macOS 本机 RocketMQ，不使用 Docker RocketMQ：

- NameServer：`127.0.0.1:9876`
- Broker：本机监听端口 `10911`；`restart_mq.sh` 将活动网卡 IPv4 写入 `brokerIP1`，供 Docker 内 Canal/Dashboard 访问，不能把 `127.0.0.1` 当作容器可访问的 Broker 通告地址。
- Canal 使用 Docker，端口：`127.0.0.1:11111`
- Canal 配置位于 `component/canal/canal_conf/`。
- Canal 过滤表：`demo_db.product`、`user`、`order`、`order_item`。
- 业务商品 Topic：`product_topic`（保留业务职责）。
- CDC 商品 Topic：`product_cdc_topic`。
- Go ES 同步消费者：`product_es_consumer_group`。
- 本机 RocketMQ 已确认可以启动，Go 商品消费者也曾成功消费消息并写入 ES。

注意：Canal 启动时如果 `product_cdc_topic` 尚未在 RocketMQ NameServer 注册，可能出现 `No route info for this topic`。稳定启动顺序是：

1. 启动本机 NameServer。
2. 启动本机 Broker。
3. 确认 Topic 已存在。
4. 重启 Canal，让 Canal Producer 重新获取 Topic 路由。
5. 启动后端 Go Consumer。
6. 再写入新的商品进行验证。

历史验证已确认 MySQL Binlog → Canal → RocketMQ → ES 链路可行；本次未重验运行状态。遗漏消息是否能恢复取决于 Canal 位点、MQ 保留及消费位点，不能仅靠等待判断会补发；需要核对位点、日志及数据差异。

## 三、常用启动和检查命令

### 1. 检查所有依赖

```bash
cd /Users/hewen/GoProjects/src/go-framework-demo
./components_sh/ping.sh
```

脚本检查并输出：

- MySQL：`3306`
- Redis：`6379`
- Elasticsearch：`9200`
- RocketMQ NameServer：`9876`
- RocketMQ Broker：`10911`
- Canal：`11111`
- Docker、SeaweedFS：`8333/8888/9333`，以及可选 Prometheus/Grafana：`9090/3000`

检查目标端口集中定义在 `components_sh/ping.sh` 顶部；修改这些变量只改变检查目标，服务本身的监听配置仍需单独同步。

### 2. 启动本机 RocketMQ

```bash
./components_sh/restart_mq.sh
```

在用户自己的终端运行。该脚本更新 Broker 通告地址、重启本机 MQ 并启动 Dashboard；切换网络后需重新运行。多网卡环境可显式设置 `BROKER_ADVERTISE_INTERFACE` 或 `BROKER_ADVERTISE_IP`，详见 `Readme.md`。启动后检查 CDC Topic 路由，再启动 Canal。

停止：

```bash
/usr/local/rocketmq/bin/mqshutdown broker
/usr/local/rocketmq/bin/mqshutdown namesrv
```

如果报 `Lock failed, MQ already started`，先检查 `9876` 和 `10911` 是否已经有进程监听，不要直接删除 store 锁文件。

### 3. 重启 Canal

```bash
cd /Users/hewen/GoProjects/src/go-framework-demo
./components_sh/restart_canal.sh
```

该脚本会重建 `canal-server` 容器，并挂载项目内的 `component/canal/canal_conf`。

### 4. 构建和启动后端

```bash
./components_sh/build.sh
./components_sh/start.sh
```

在用户自己的终端执行并保持前台。`start.sh` 会停止旧的 `8080` 进程、启动后端，并最多等待 30 秒确认端口监听；已有二进制时不会重新构建，代码更新后需先执行 `build.sh`。`restart_go.sh` 使用 nohup，不能据其返回成功认定 Codex 会话结束后仍持续运行。

### 5. 前端

```bash
./components_sh/restart_frontend.sh
```

前端地址：`http://localhost:5500`。

### 6. 商品同步测试

下面的测试会连接真实 MySQL 并保留新增商品，不是只读检查；需依赖就绪后用于明确的链路验证。

```bash
GOCACHE=/tmp/go-framework-demo-cache go test ./db/test -run '^TestProductCreateAndGet$' -count=1 -v
```

记录测试输出中的商品 ID，然后查询：

```bash
curl http://127.0.0.1:9200/product_test/_doc/<商品ID>
```

也可以在浏览器的 ES Dev Tools 中查询 `product_test` 索引。若命令行访问被 Codex 沙箱拦截，以 `ping.sh` 和浏览器实际结果为准；不能把沙箱访问失败直接判断为 ES 停止。

## 四、当前已知问题和后续工作

1. Canal 的 Binlog 位点和 MQ 路由需要持续验证。Topic 必须先注册，再启动或重启 Canal。
2. `user_consumer_group_info` 原配置为广播模式，曾出现就绪探测超时；为保证 Demo 后端完整启动，当前配置已调整为 clustering。
3. `order_consumer_group` 曾因 orderly 消费就绪探测超时，当前代码配置为 `consume_orderly: true`；需要继续核查 Broker 锁和就绪探测。
4. `ProductService.Create/Update` 只写 MySQL；列表及导出由 ES 检索 ID 后批量回查 MySQL。筛选命中和总数仍受 CDC 延迟影响，回查过滤后单页可能不足。
5. 已有消费/ES 操作日志、失败返回重试、按主键覆盖及进程内串行；跨消费者实例和对账并发仍需关注版本与最终收敛。订单业务消息在数据库事务完成后发送，失败只记录日志，尚无持久化补偿。
6. 当前配置中仍有部分本机连接参数和测试密码，生产环境应迁移到配置文件或安全配置系统。
7. 需要继续补充 HTTP Handler 的请求校验、错误码约定和集成测试。
8. `UserService.GetUserByID` 缓存命中后只检查 DB 记录存在，仍返回缓存字段，与手册的 DB 事实源约定有差距；用户更新入口仍使用 map，尚未统一为指针 DTO。
9. `main.go` 退出流程未调用 HTTP `Shutdown`，且 `os.Exit` 会跳过 deferred 日志刷新；不能视为完整的优雅退出。
10. `test_sh/test_db.sh`、`test_sh/test_es.sh` 当前切到脚本目录后使用根目录相对包路径，存在路径错误；暂从项目根目录直接执行 `go test ./db/test` 或 `go test ./es/test`，并先确认测试对真实数据的影响。

## 五、项目设计知识库

项目的分层设计、数据流、错误处理、文件定位规则和可复用工程经验已单独整理到：

[Project_Handbook.md](./Project_Handbook.md)

后续新会话应优先阅读该文件，理解项目结构后再修改代码。

## 六、运行环境注意事项

涉及 `nohup ... &`、RocketMQ、Redis、Canal、Docker 或其他长期运行服务时，Codex 命令执行环境可能在命令结束后回收后台子进程。启动成功日志不代表服务会持续运行。优先在用户自己的终端前台启动；已有 launchd/Homebrew service 或 Docker restart policy 时按其生命周期管理。不能用 Codex 自己的 PTY 替代用户终端并据此承诺持续运行；启动后必须从独立命令检查进程和监听端口。

如果 Codex 沙箱无法访问本机服务、端口、进程或日志，应先说明需要的权限并申请授权，不能把命令环境的访问失败直接判断为服务故障。

AI 执行项目测试前，必须先检查测试入口、TestMain 及依赖初始化，汇总本次所需的全部服务连接和本机端口监听权限，在首次运行测试前一次性向用户申请；不要先运行失败后再逐项申请。申请时说明连接目标、测试范围以及数据写入和清理方式，通过执行工具的权限机制落实。已有授权覆盖且仍有效时复用；新增依赖时提前补充申请。纯隔离测试无需连接权限。完整规则见 `Project_Handbook.md` 第 7.1 节「AI 测试连接权限」。

对应 Skill：

- `background-processes`：`/Users/hewen/.codex/skills/background-processes/SKILL.md`
- `skill-memo-sync`：`/Users/hewen/.codex/skills/skill-memo-sync/SKILL.md`


## 七、2026-10-12 账号与输入校验更新（历史记录，已标注现状）

- 2026-10-12 曾按用户要求删除 78 个测试用户，当时保留管理员：ID `1`、姓名 `hwt`、年龄 `18`、邮箱 `361861860@qq.com`。密码由用户指定，采用 bcrypt 保存，文档不记录明文。
- `user.email` 已添加 `uk_user_email` 唯一索引；注册同时执行邮箱格式、重复邮箱和密码 6–18 位校验。测试用户使用随机数字 QQ 邮箱和 8 位随机密码。
- 登录和注册返回 `role`、`token` 及不含密码的用户信息。会话保存在 Redis，有效期 24 小时；后续请求使用 `Authorization: Bearer <token>`，注销删除服务端会话。
- 管理员由后端同时校验 ID=1、姓名 hwt 和指定邮箱；商品创建/更新和发货接口仅允许管理员，普通用户只能为自己下单、查询自己的订单。
- 数据库 ID 仍为 int64，HTTP 返回及前端请求中的 ID 统一使用十进制字符串；前端旧 localStorage 数据版本已隔离，需要重新登录。
- 当前下单在事务外批量读取商品，核对价格、名称、图片及库存快照；事务内按商品 ID 顺序原子扣库存，并写入订单和明细。校验失败或扣减失败返回冲突错误，事务内失败回滚。事务外校验不保证随后并发改价/改图会被拒绝；当前扣库存条件不包含这些字段的版本。
- 商品更新仅允许 `name/price/stock/image`，整数价格范围 1–100000000 分，整数库存范围 0–1000000；创建沿用相同约束。前端调整库存已调用后端接口。
- 发货业务状态逻辑按用户要求暂未修改；仅为发货接口增加管理员权限检查。
- 订单业务消息与 Canal 消费者协议不匹配已修复，见第八节。
- 验证：权限、密码、商品校验、ID JSON 测试通过；本机 MySQL/Redis 下管理员登录/注销、邮箱唯一性、价格变化回滚及正常订单扣库存测试通过。数据库集成测试在事务中回滚，不保留测试用户。

历史使用过 `cmd/reset-users`，但当前仓库已无该目录，不再提供对应重置或 inspect 命令。新环境邮箱唯一索引脚本仍位于 `migrations/001_user_email.sql`。


## 八、业务消息与 ES 同步链路分离

- 订单业务：`order_topic` → `order_consumer_group` → `order_business_handler.go`，解析 `OrderMessage`，校验后记录业务事件。具体业务动作待定义；不写 ES。
- 订单 CDC：`order_cdc_topic` → `order_es_consumer_group` → `order_es_handler.go`，解析 Canal 事件，从 MySQL 重建订单 ES 文档或删除文档。
- 用户业务：`user_topic` → `user_consumer_group_info/user_consumer_group_notify` → `user_business_handler.go`，解析 `UserMessage`，保留业务日志入口；不写 ES。
- 用户 CDC：`user_cdc_topic` → `user_es_consumer_group` → `user_es_handler.go`。Canal 的用户动态路由已从 `user_topic` 改为 `user_cdc_topic`。
- 商品 CDC：`product_cdc_topic` → `product_es_consumer_group` → `product_es_handler.go`。
- 共享业务消息定义移到 `rocketmq/message/`；`rocketmq/message.go` 保留原类型和构造函数 API，避免生产者和消费 Handler 之间出现循环依赖。
- ES Handler 拒绝缺少 table/data/type 或表名不匹配的消息，返回消费重试；业务 Handler 拒绝 Canal 消息。合法 DDL 不进行文档同步。消费失败有独立日志。
- ES 同步沿用消费 Context，文档 ID 使用业务主键，保留覆盖写入的幂等行为。
- `user_cdc_topic` 已于 2026-10-12 在本机 `broker-a`（127.0.0.1:10911）创建，读写队列各 4 个，并通过 NameServer 的 topicRoute 验证。当时需要重启 Go 和 Canal 应用新配置；本次未核实当前进程加载状态。此前留在 `user_topic` 的 Canal 消息需要单独处理，不能交给新的用户业务 Handler。
- 单元测试直接调用实际注册的消费 Handler，并用 ES HTTP 测试替身验证业务消息不写 ES、CDC 写入正确索引/ID、协议混用和 ES 失败返回重试。

- 测试组织规范已补充到 Project_Handbook.md 第 7 节：测试优先调用真实业务入口，`db/test/` 和 `server/test/` 按业务存放测试。原 `server/server_test.go` 已移除，当前服务层测试按 `server/test/user/`、`product/`、`order/`、`log/` 拆分；公共辅助位于 `support/`，跨业务测试位于 `testdata/`，运行时必须显式包含该包。DB 测试仍位于 `db/test/`。

## 九、商品列表分页与请求 Context

- `PUT /api/products/list?page=1&page_size=20`：默认第一页、每页 20 条；每页支持 1–100 条，分页范围限制在 ES 默认的前 10000 条内，非法参数返回 HTTP 400。
- 成功响应由商品数组改为 `{items: [], total: 0, page: 1, page_size: 20}`，商品 ID 仍为字符串。
- 商品列表通过 HTTP Request Context → ProductService → ES/DB 传递取消和超时；ES 显式设置 from/size，当前按 `create_time DESC, id DESC` 排序并查询精确总数，再批量回查 MySQL 详情。
- 管理端和用户端均支持上一页、下一页、每页条数选择与刷新；批量选择限本页，切页或修改每页条数时清空选择。
- 验证使用 ES HTTP Transport 测试替身，不依赖本机 ES；覆盖分页参数、总数、ID 精度、非法参数和 Context 取消。前端脚本验证请求参数和分页渲染。

## 十、业务 HTTP API 统一 PUT

- 注册、登录、注销：PUT `/api/auth/register`、`/api/auth/login`、`/api/auth/logout`。
- 商品查询：PUT `/api/products/list?page=1&page_size=20`；创建：PUT `/api/products`；更新：PUT `/api/products/:id`。
- 订单创建：PUT `/api/orders`；查询：PUT `/api/orders/list?user_id=<字符串ID>`；发货：PUT `/api/orders/:id/ship`。
- 前端 API 封装强制使用 PUT，监控页面查询 `/api/metrics` 也使用 PUT；请求体及响应格式沿用现有约定。
- CORS 允许 PUT、OPTIONS；浏览器预检仍使用 OPTIONS。静态页面加载和 Prometheus `/metrics` 采集保留 GET。
- 原 GET/POST 业务 API 不再注册；调用方需一起切换方法和列表路径。


## 十一、商品创建时间与新增后刷新

- MySQL `product` 新增 `create_time DATETIME(3)`，ES `product_test.create_time` 使用 `date` 类型。
- 按用户要求，已有 48 条测试商品统一赋当前时间并同步到 ES，不推算历史时间。
- 新商品由模型创建钩子生成创建时间，数据库保存，Canal 商品 Handler 将该时间写入 ES；更新价格、库存不修改创建时间。
- 商品分页按 `create_time DESC, id DESC` 返回，同一创建时间用 ID 保持稳定排序。
- 前端添加商品成功后等待 1 秒执行页面刷新，刷新后回到第一页。
- 历史使用过 `cmd/init-product-time` 初始化测试商品时间；当前仓库已无该目录，不应再执行旧命令。商品时间字段需随新环境表结构准备，不能把 ES 对账工具当作数据库迁移工具。
- 验证：实际 ES 倒序查询、前端新增后 1 秒刷新检查、商品查询排序测试及构建通过。后端需重启后加载新代码。

## 商品图片组件（2026-10-13）

- 新增 `component/Image_component/`：SeaweedFS mini 4.48 Docker Compose、说明及本地数据目录。容器 `demo-seaweedfs`，使用持久化挂载及 `unless-stopped` 策略；S3 8333、Filer 8888、Master 9333、管理 23646，仅本机访问。
- `components_sh/restart_image.sh` 从 `conf/image_storage.yaml` 读取 S3 密钥及 Bucket，重建容器但保留图片数据；`--stop` 仅停止组件。不会操作 Go 后端。
- `storage/image_store.go` 封装 S3 上传，`server/image_server.go` 校验 JPEG/PNG、5MB、分辨率并生成 UUID 文件名；`http_handler/image_handler.go` 处理 multipart 上传。
- PUT `/api/products/image` 只允许管理员，前端添加商品改为文件选择，上传返回 URL 后随商品保存。MySQL 和 ES 沿用 `image` 字段，不改变 CDC 业务链路。
- Filer 读取对象路径为 `/buckets/product-images/products/<文件名>`；本地配置 public_url 包含 `/buckets/product-images`。
- 凭据与图片数据已加入 Git 忽略。组件部署不自动重启 Go；运行后端需加载新代码才能提供新接口。

## 分层修正与订单创建时间（2026-10-13）

- 下单流程位于 `server/order_server.go`：Server 拒绝重复商品，事务外批量读取商品、核对价格/库存及快照并准备金额和明细；事务内按商品 ID 顺序执行 `stock >= quantity` 原子扣减，检查影响行数，并写入订单和批量明细。当前不再先锁行查询；已删除 DB 的 `CreateWithItems/Checked` 和 `matches` 回调。
- `db/product_db.go` 提供事务 Query、批量查询和原子扣库存方法；商品字段校验移到 `server/product_validation.go`。注册字段校验、邮箱重复检查及 bcrypt 处理移到 UserService，DB 创建方法接收准备好的模型；唯一索引冲突由 Server 转换为业务错误。
- 批量订单的校验和事务也移到 Server；DB 批量写入只负责持久化。业务规则测试迁入 `server/test/`，直接调用实际 Service。
- UserService/ProductService/OrderService 支持可选的显式事务 Query。事务内注册不写 Redis 缓存，避免回滚后留下缓存数据。
- `order.create_time` 新增 `DATETIME(3)`，模型创建钩子只在首次创建时生成；ES 订单 CDC 从 MySQL 读取该字段，不再使用同步时的当前时间。Service 禁止通过更新接口修改创建时间。
- 历史迁移工具 `cmd/init-order-time` 当前已不在仓库，旧的 `-apply/-reset` 命令不可用。独立 SQL 仍位于 `migrations/002_order_create_time.sql`，仅用于首次迁移，执行前核对目标表结构。
- 本机已执行迁移。按用户要求，42 条测试订单的创建时间已统一覆盖为 `2026-10-13 18:41:02.389 +08:00`，ES `order_test` 中现有 8 条文档的 `create_time/create_time_stamp` 同步覆盖并验证一致；不恢复历史时间。新环境应先迁移，再启动新后端。
- 数据库验证：`DEMO_INTEGRATION=1 DEMO_TEST_DATABASE_ONLY=1 go test ./server/test/user ./server/test/product ./server/test/order -run '^TestIntegration(OrderValidation|RegistrationValidation|ProductUpdateValidation|OrdersDoNotOversell)$' -count=1`。数据库验证模式不初始化 ES/RocketMQ；回滚测试不保留数据，并发测试清理临时数据。
- 订单同步验证：`DEMO_INTEGRATION=1 go test ./rocketmq/handler/test -run '^TestIntegrationOrderCDCKeepsCreationTime$' -count=1`，使用真实 MySQL 和 ES HTTP 替身，验证重复消费及订单状态变化后创建时间不变，临时记录清理。
- 运行中的 Go 后端需重启后加载这些改动；已有 ES 文档在下一次订单 CDC 同步时使用持久化的创建时间。

### 订单商品图片快照

- 执行 `migrations/003_order_item_image.sql` 添加 `order_item.image`，历史订单留空。
- OrderService 从校验通过的数据库商品保存图片快照；MQ 和订单 ES 同步使用订单明细图片，不读取商品当前图片。
- 历史字段调整时已通过 GORM Gen 重新生成订单明细 model/query 文件；当前生成器使用显式 model 生成 query，以保留软删除标签。

## 软删除（2026-10-13）

- Demo MySQL 四张表 `user/product/order/order_item` 已增加 `is_del TINYINT UNSIGNED NOT NULL DEFAULT 0`；0 正常，1 已删除。不采用历史迁移流程。
- 模型 `IsDel` 使用 GORM 官方 `soft_delete.DeletedAt` 和 `softDelete:flag`。GORM/Gen 模型查询、统计、更新默认附加 `is_del=0`；`Delete()` 更新为 1，不再物理删除。原生 SQL、仅使用 `Table()` 而未绑定模型的维护代码不受此默认过滤约束。
- `GetByIDIncludingDeleted` 显式调用 `Unscoped()`，四类 DB 均提供该方法；订单明细另有 `GetByOrderIDIncludingDeleted`。只查询删除记录可在 Unscoped 查询中加 `IsDel.Eq(1)`；禁止将 Unscoped 查询误用于业务删除（`Unscoped().Delete()` 会物理删除）。
- `cmd/generate/gen.go` 从显式 model 生成 query，保留软删除标签，避免从表结构重新生成时丢失 GORM 插件配置；从项目根目录运行 `go run ./cmd/generate`。
- ES 三个索引增加 `is_del` byte 映射；订单 nested `items` 也保存明细 ID 和 `is_del`。普通用户、商品、订单及通用搜索默认过滤 `is_del=0`；商品条件 nested 查询过滤 `items.is_del=0`；按 ID 读取订单时排除软删除明细。
- 三条 CDC 链路统一按主键读取 MySQL 最新状态（包括软删记录），同步完整记录及 is_del；order_item 事件重建订单。历史物理删除的主键写入 `is_del=1` 墓碑，ES 保留标记但不再作为正常记录展示。
- CDC 同一进程同主键读取/写入串行。这个锁不提供跨进程顺序保证；多个消费者实例或其他直接 ES 写入端仍需独立考虑并发版本控制。
- `cmd/refresh-demo-es` 为 Demo 全量校准入口，添加 ES 映射并复用 CDC 方法按 MySQL 重建副本，将孤立文档标删；不修改 MySQL。
- 数据库/CDC 集成测试通过 `DEMO_INTEGRATION=1` 启用，测试 fixture 在真实 MySQL 事务中回滚。软删除核心测试：`TestSoftDeleteDefaultScope`、`TestCDCSoftDeletedUserOrderAndItem`、`TestProductCDCCreateTime`、`TestCDCMissingRowsMarkDeleted`。
- 模型、查询过滤和 CDC 代码变更后，需重新构建并重启后端才能加载；本次未核实运行中二进制版本。已软删除用户的邮箱仍受现有唯一索引约束，不自动释放邮箱。

- 2026-10-13 历史实测：商品正常文档 MySQL/ES 均 55 条，缺失/多余/字段差异均为 0；user_test 正常 2、软删 20；product_test 正常 55、软删 5；order_test 正常 42、软删 6；三个索引缺失 is_del 的文档均为 0。构建、HTTP/ES 回归及事务回滚的 MySQL/CDC 软删测试通过。

## 日志查询页面（2026-10-14）

- 接口监控重启脚本：`components_sh/restart_api_monitoring.sh`，复用 `component/monitoring/docker-compose.yml`，通过 Docker Compose 重新创建 Prometheus/Grafana（支持首次启动），保留数据卷；检查两个 HTTP 就绪端点，失败输出最近日志。不操作 Go 服务，尚未预置 Grafana 看板。

- `frontend/logs.html` 为管理员日志查询页，与商品前端共用 5500 端口和 `demo-session-v2` 登录会话；管理工作台有跳转入口，日志入口位于管理员工作台导航，与商品管理、用户订单并列；日志页不单独提供登录表单，未登录或普通用户会返回工作台管理员登录，登录成功后自动回到日志页。
- `components_sh/restart_frontend.sh` 一次重启同时提供商品页 `/` 和日志页 `/logs.html`，保持在用户终端前台运行。脚本仅停止该端口的监听进程。
- 管理员接口 PUT `/api/logs/query`，JSON 入参 `keyword/date/level/page/page_size`；默认最新文件日期，每页20条，最多100条。keyword 对解码后的日志内容作不区分大小写的子串匹配，包括部分 log ID、消息、SQL、路径及错误字段。ERR 包含 error/fatal/panic/dpanic。
- 页面按一条日志一行展示时间、级别、摘要/ID、类型、耗时；详情展开完整消息、SQL、错误、文件行号及其他字段。无 log_id 的日志也能通过关键词/级别找到，但不会自动推断其请求归属。
- `server/log_server.go` 从与 logger 共用的日志目录查询，保留大整数 ID 精度，HTTP耗时转毫秒；校验日期及分页，响应不返回服务器绝对路径。原终端 `components_sh/log/query.sh` 仍用于精确匹配 log_id。
- 验证：服务查询、分页/级别/中文模糊条件、权限/日期校验测试，以及页面详情展开/收起与安全文本渲染检查通过；构建通过。后端需重新 Run 注册新接口。

### 组件目录整理

- 原根目录 `Image_component`、`monitoring`、`canal` 统一位于 `component/`。
- `storage/` 保留在项目根目录，Go 导入路径不变。启动脚本和文档沿用新组件路径；本地图片数据及密钥随目录一起移动。

### HTTP 状态码指标

- `http_requests_total` 按 `route` / `status` 累计，支持分别查询 4xx、5xx 和 HTTP 成功率；旧 `http_request_errors_total` 仍只统计 5xx。
- 未匹配路径聚合为 `unmatched`，耗时直方图边界使用数字秒值。查询示例见 `component/monitoring/README.md`；更新后需重启 Go 后端。

### 管理端修改商品

- 商品卡片支持调整库存、修改价格、修改图片。价格输入为元，转换为整数分；图片复用 JPEG/PNG、5MB 上传接口。
- 修改使用管理员商品更新接口；价格和图片保存成功后以接口返回值更新卡片，操作期间禁止重复提交。

### 入参结构体指针约定

- 手写更新 DTO、查询条件和配置参数统一使用指针传递，解引用前检查 nil；具名局部变量组装后传 `&input`。保持更新字段的缺省/null 与显式零值语义，详见项目手册。

### 公开方法必须通过 interface 集中声明

- 四类 DB、五类 Service、UserCache 和 ImageStore 已统一声明对应的 `类型名Interface`，在结构体附近列全公开方法并添加编译期实现断言；通用缓存沿用 `GenericCache[T]`。
- 已补齐旧 DB 接口遗漏的邮箱查询、分页、状态更新及包含软删除记录的查询；ProductService 接口包含跨文件实现的 Export。
- **手写操作类型必须在结构体附近通过 interface 列全公开方法；新增、删除或修改方法时，必须同步更新接口声明，并添加编译期实现断言。**
- **代码审查必须核对接口清单是否完整，不能只看编译通过。** 编译期断言无法发现实现中新加而接口漏列的方法。
- 构造函数、包级函数、私有辅助方法、纯数据结构和自动生成代码不纳入清单；具体规范与示例见项目手册「公开方法必须通过 interface 集中声明」。

### RocketMQ Dashboard

- `component/rocketmq-dashboard/` 保存 Compose 和应用配置；官方镜像固定为 2.1.0，页面仅本机 `8082`。
- `components_sh/restart_mq.sh` 检查/安装镜像后重启 MQ，再启动 Dashboard 并验证页面和集群通信。容器使用 `host.docker.internal:9876`，实际 Broker 由 NameServer 返回。
- 已验证容器到 NameServer / Broker 的 TCP 通信、集群查询、商品 Topic 消息查询与单条详情。

### ES 查询成功日志

- 所有实际 ES Search 入口成功后记录 `ES查询完成`，包含方法、索引、返回数量、总数和耗时；分页查询记录页码和页大小。
- HTTP/Service 使用携带请求 ctx 的查询入口；无 ctx 的兼容入口使用 Background，无法关联 HTTP Log ID。用户、订单、商品按 ID 查询提供 Context 版本。

### 商品列表回查 MySQL

- `ProductService.List` 使用 `es.GetProductIDsPage` 只取 ES ID 与总数，再调用 DB `GetByIDs` 批量回查，返回 MySQL 最新详情并恢复 ES 顺序。日志链路包含 ES 查询及 GORM 批量 SQL。

### 业务查询事实源约定

- 除非需求明确指定查缓存，否则真实业务数据必须最终从 DB 获取。ES 只负责筛选、排序和分页，Service 根据检索 ID 集合批量回查 DB，恢复 ES 顺序后返回。
- 价格、库存、图片、状态等最终字段以 DB 为准；过滤不存在或已删除记录，明确区分 ES 命中总数与回查后的实际条数。完整规范见 `Project_Handbook.md`第 2.6 节「搜索检索与事实源回查」。

### 订单号长度

- 实际 MySQL `order.order_no` 为 VARCHAR(30)。前端订单号为 `OD` + 23位安全随机十六进制字符，共25字符；Service 创建入口校验不超过25字符，低于数据库30字符上限，超长返回参数业务错误。

### Swagger 接口文档

- 接口说明集中维护于 `docs/openapi.yaml`（OpenAPI 3.0.3）；Handler 仅保留简短中文说明，无 Swagger 注释或专用文档 Go 结构体。
- `components_sh/generate_api_docs.sh` 核对实际路由及 schema 引用，将 YAML 转为 JSON；页面直接读取内嵌 YAML。后端提供 `/swagger/index.html`，更新后重新生成、编译和重启。
- 文档包含登录、商品筛选和 Excel 导出、图片、订单、日志及指标接口。

## 2026-10-14 补充核对：商品导出、对账与测试入口

### 商品筛选与导出

- 商品 Excel 生成已改用 `github.com/xuri/excelize/v2 v2.10.1` 的 StreamWriter，移除手写 ZIP/XML 及自定义 productExcelWriter 包装。Service 直接使用 NewFile、StreamWriter.SetRow、Flush、SaveAs，辅助函数仅负责商品表头/格式及数据列转换。保留八列表头、冻结首行、列宽、筛选、大整数 ID 文本及金额两位小数；达到单表行数上限时新建工作表。Excel 元列仅展示时转换为数字，业务金额仍使用整数分。
- 验证：全项目构建及 `DEMO_INTEGRATION=1 DEMO_TEST_DATABASE_ONLY=1 go test ./server/product_server/test -run '^TestIntegrationProductExport$' -count=1` 通过，覆盖多批回查、文件读取、文本/金额/样式和下载清理；MySQL fixture 事务回滚，ES 使用替身。后端需重新构建并重启加载新导出实现。

- 列表与导出共用名称、创建时间、库存及价格范围筛选；范围边界包含端点，价格使用整数分。
- 管理员通过 PUT `/api/products/export` 导出 Excel。ES 分批返回 ID，Service 批量回查 MySQL 最新详情后生成附件；导出不受普通分页的前 10000 条限制。
- `es/product_export.go` 的 `ProductExportUseSearchAfter` 默认 false，导出默认使用 Scroll，可切换为 PIT + search_after；不能将对账的遍历方式误写为导出的默认方式。

### 用户订单页请求修复

- 普通用户进入“我的订单”、点击刷新，以及单个/批量下单成功后，调用 PUT `/api/orders/list?user_id=<当前登录用户ID>`，使用现有 Bearer token。列表使用当前会话的接口结果，转换 `order_no/user_id` 字段，不再读取 localStorage 中的旧订单。
- 区分加载中、查询失败和查询成功但为空；退出或切换账号后忽略旧订单请求结果，避免跨会话写回。
- 当前订单列表接口只返回订单主表，没有 `items`；发货列显示“暂无明细状态”，不根据空数组推断已发货。管理员“用户订单”页仍是原有实现，不属于此次普通用户订单页修复。
- `node --test frontend/test/orders.test.cjs` 验证真实页面脚本的请求、字段映射、刷新/失败/会话隔离及两种下单后的查询；四项通过。测试使用接口替身，未验证真实后端响应。静态页面刷新即可加载改动，无需重启 Go 后端。

### 当前维护工具

| 入口 | 用途与副作用 |
|---|---|
| `cmd/api-docs` | 校验路由和 schema 引用，生成 `docs/openapi.json`；不启动业务服务。 |
| `cmd/refresh-demo-es` | 更新软删除映射，复用 CDC 入口按 MySQL 全量重写 ES，孤立文档写删除墓碑；不修改 MySQL。 |
| `cmd/reconcile-es` | 分批比较 MySQL 与 ES，修复缺失/差异文档，反向扫描删除 DB 不存在或已软删的 ES 文档；不修改 MySQL。 |

对账说明见 [scheduler/reconciliation/README.md](./scheduler/reconciliation/README.md)。`go run ./cmd/reconcile-es` 会直接执行一轮真实 ES 更新和删除，没有 dry-run 开关，本次文档核对未执行它。

- 先完成用户、商品、订单的 DB→ES 扫描，再执行 ES→DB 扫描，每批 100 条。正向用 MySQL 主键游标，反向用 PIT + search_after。
- `source` 表示 ES 当前文档，`target` 表示 MySQL 目标记录。订单主表和明细在短读取事务中查询，字段显式比较。
- ES 更新/删除带扫描时的版本条件，缺失文档使用 create；版本冲突记录 Warn 并跳过，留待后续收敛。数据库错误和 Bulk 失败项返回错误。
- 日志标志为上海日期的 `Reconciliation+YYYY-MM-DD`，记录索引、主键、动作和差异字段名。命令支持 Ctrl+C 取消并释放 PIT；定时执行由调用方另行安排。
- `refresh-demo-es` 保留删除墓碑，`reconcile-es` 反向扫描会物理删除对应 ES 文档，两者不是相同操作；均需连接正确的 MySQL 事实源。

### 测试与本次验证范围

从项目根目录运行以下隔离测试；`DEMO_INTEGRATION=0` 跳过这些包中显式受该开关控制的集成测试：

```bash
DEMO_INTEGRATION=0 GOCACHE=/tmp/go-framework-demo-cache go test \
  ./model ./http_handler/test ./rocketmq/handler/test \
  ./scheduler/reconciliation ./server/test/log \
  ./server/test/testdata -count=1
```

完整服务层入口为 `bash test_sh/test_server.sh`，包含 `testdata` 包。默认会初始化真实依赖，部分测试会保留测试数据；不要把它或 `go test ./...` 当作纯单元测试。

`DEMO_TEST_DATABASE_ONLY=1` 只跳过服务层 TestMain 的统一依赖初始化，不保证测试不连接数据库。需要数据库回滚/并发验证时，使用上文带明确 `-run` 的集成测试命令；`DEMO_INTEGRATION=0` 也不会禁用其他未检查该变量的真实 CRUD 测试。

2026-10-14 本会话已验证：`go build ./...` 通过，上述六个测试包通过。HTTP 测试最初因沙箱不允许监听临时端口失败，放宽执行权限后通过。未运行完整集成测试，未启动/重启服务，未执行迁移、数据重置或真实 ES 对账。此前记录的服务就绪、数据数量及迁移完成状态均未在本次重新验证。

## 2026-10-15 补充：云服务器部署配置

- Go 服务配置统一从项目主目录 `conf/` 向上定位读取，部署包目录结构保持 `bin/`、`conf/`、`frontend/`。可从任意工作目录启动包内可执行文件，只要二进制仍处于部署包目录内。
- `components_sh/Internet/package.sh` 默认交叉编译 Linux amd64 后端、前端静态服务器及配置工具，输出 `dist/go-framework-demo.tar.gz`；云服务器不需要 Go 编译环境。
- 云端只编辑 `conf/*.yaml`，然后重启相关进程。前端静态服务器启动时会从 `app.yaml` 生成浏览器 API 地址配置；修改 API 公网地址后重启前端。
- 配置分别为 `app.yaml`、`mysql.yaml`、`redis.yaml`、`es.yaml`、`rocketmq.yaml`、`image_storage.yaml`。生产环境需填写有效密码、地址和图片存储凭据；`image_storage.yaml` 中 `replace-me` 是模板占位值。
- 启动命令及 ARM64 打包方式见 [DEPLOYMENT.md](./components_sh/Internet/DEPLOYMENT.md)。外部 MySQL、Redis、ES、RocketMQ、对象存储以及可选 Canal 服务仍需单独部署并保证网络可达。
- 本次验证：Linux amd64、`CGO_ENABLED=0` 后端构建成功；打包脚本成功，产物检查为静态 x86-64 ELF，包内包含运行所需二进制、配置和前端文件，没有本地覆盖配置。未连接真实云端依赖验证启动。

## 2026-10-15 配置文件收敛

- 已移除 `.local.yaml` 双文件覆盖方式。MySQL 只读取 `conf/mysql.yaml`；图片存储只读取 `conf/image_storage.yaml`。其他组件继续各自读取单份 `conf/*.yaml`。
- SeaweedFS 启动脚本直接从 `conf/image_storage.yaml` 读取 S3 凭据和 Bucket 传给 Compose，不再生成本机覆盖 YAML 或依赖单独 `.env` 密钥文件。
- 上云时修改部署包中的对应 YAML，然后重启服务；本机与云端配置字段一致。

## 2026-10-15 主程序 ES 定时对账

- 对账实现位于 `scheduler/reconciliation` 公共包：`Run` 执行单轮，`RunScheduler` 立即执行并按间隔重复执行。`cmd/reconcile-es` 独立入口调用 `Run`，项目主程序调用 `RunScheduler`。
- 主程序初始化 MySQL、ES 等依赖后，在独立 goroutine 中立即执行一次对账；每轮完成后按 `conf/app.yaml` 的 `reconciliation.interval_hours` 等待再运行，默认 24 小时。
- 对账中的每次 MySQL 查询结束后等待 100 毫秒再继续；context 取消时等待可提前结束。
- 对账错误只写日志，本轮结束后仍按周期继续；任务不阻塞 HTTP 服务启动或请求处理。主程序收到退出信号时取消对账上下文，不等待其完成。
- 对账过程会读取 MySQL 并更新/删除 ES 数据，需确认连接的是正确主库和 ES。启动及每轮运行会增加后台 MySQL/ES 负载。

## 2026-10-15 订单超时关闭

- 新订单的 `expire_time` 设为 `create_time + 10 分钟`，迁移 `migrations/007_order_expire_time.sql` 为该字段建立 `(status, expire_time)` 复合索引，并按创建时间回填历史订单。
- 订单创建后发送 RocketMQ 10 分钟延迟检查消息（默认 delay level 14）；主程序同时每分钟按复合索引分批扫描到期订单，作为延迟消息丢失、发送失败或停机期间的兜底。Broker 需启用默认 10 分钟 delay level。
- 关闭订单和恢复订单明细中的商品库存处于同一事务；状态只允许待支付订单转换为已取消，重复延迟消息和重复扫描不会重复加库存。支付也检查数据库时间与截止时间。
- 状态 4 为已取消；用户订单列表提供已取消筛选，并返回支付截止时间。

## 2026-10-16 云端 Compose 试玩部署

- `components_sh/Internet/start_set_all.sh` 生成数据库、Redis、Canal 和图片存储随机凭据，写入 `conf/*.yaml` 与 Canal properties，再启动 MySQL、Redis、ES、RocketMQ、SeaweedFS 和 Canal；同一 Compose 网络内通过服务名连接。
- `components_sh/Internet/start_go.sh` 构建并强制重建 Go 后端与前端静态服务器容器；浏览器通过宿主机 5500 端口访问前端，API 通过宿主机 8080 端口访问。Compose 不含 Prometheus/Grafana；服务资源限制针对低流量、4GB试玩机，不代表生产容量保证。
- 两个脚本均可重复运行，固定 Compose 项目名及 `.env`，容器重建后继续挂载原命名卷；脚本不删除卷，并等待服务保持运行、失败时输出容器状态和日志。
- `components_sh/Internet/stack/mysql-init/00-create-tables.sql` 创建当前模型的五张表，`CREATE TABLE IF NOT EXISTS` 可重复执行但不升级已有表；已有库升级仍需备份后按迁移脚本处理。
- 密钥保存在 `components_sh/Internet/stack/.env`（Git 忽略，权限 600）；`start_set_all.sh` 需要 Docker Compose、Python 3，并提示宿主机设置 `vm.max_map_count=262144`。
- 仅完成 Compose 配置校验、脚本语法校验及 Go 构建；未在云端或本机实际拉取镜像、启动容器和验证服务互通。


## 2026-10-16 订单超时关闭分层调整

- 超时关闭业务集中到 `server/order_server/expiration`，通过 `db.WithTx` 编排锁单、截止时间检查、库存返还与状态更新；调度器只负责触发，MQ 超时消息直接复用该服务。
- 数据访问收敛到 `db/order_expiration.go` 及 ProductDb，新增公开方法同步接口声明。保留数据库时间判断、软删除记录处理和重复关闭不重复返库行为；支付校验也复用 DB 的数据库时间方法。
- 新增事务测试位于 `server/order_server/expiration/test`，使用显式 Query 和外层事务整体回滚，覆盖到期、未到期、已支付、重复关闭和返库中途失败回滚。真实 MySQL 下通过。

## 登录过期后的工作台状态

- 商品列表、详情、订单、下单和管理操作均需校验服务端会话；业务接口仅注册、登录允许匿名访问。
- 工作台的 JSON 请求、图片上传及商品导出共用 `apiFetch`。当前会话收到 HTTP 401 时，清除本地登录与订单数据、返回登录页并提示重新登录；登录/注册失败不触发会话清理，403、500 也不按过期处理。
- 清理前核对请求发起时的 token，避免旧请求的 401 退出新登录账号。前端隔离测试覆盖上述行为，不连接真实服务。


## 用户界面统一登录校验

- 新增 `PUT /api/auth/session`，工作台加载时先校验服务端会话，通过后展示用户/管理员页面；失败提供重试，401 返回登录页。
- 商品列表与商品详情纳入 `RequireAuth`，详情页携带 Bearer token，无凭证或会话失效返回登录页。切回商品标签会重新请求商品，触发服务端鉴权。
- OpenAPI YAML 与生成 JSON 已同步。前端隔离测试覆盖启动拦截、详情鉴权及过期清理；后端路由测试验证无凭证返回401。

## 待支付订单倒计时

- 订单响应已包含 MySQL 的 `expire_time`，前端以该字段显示支付截止时间及剩余分秒，每秒仅更新倒计时和支付按钮，不重绘筛选表单。
- 到期显示「已超时，等待关闭」并禁用支付；点击处理再次校验截止时间，缺失或非法时间也禁止支付。订单取消状态仍以服务端返回为准。
- 后端支付流程继续在锁单事务内用数据库时间判断截止时间，前端倒计时为交互提示，最终支付结果以服务端校验为准。
- 前端隔离测试覆盖倒计时推进、到期禁用及请求拦截、无效时间和已支付订单展示。

## 订单商品快照详情

- 订单明细的图片、名称链接至 `order_snapshot.html`，使用订单 ID 和明细 ID 请求 `PUT /api/orders/:id/items/:item_id/snapshot`。
- 普通用户仅可读取本人订单，管理员通过明确授权分支查看全部订单；Service 同时核对订单归属和明细所属订单。
- 返回 MySQL 订单明细中已保存的商品名称、图片、成交单价、数量和商品版本，不查询当前商品、ES 或 Redis，不展示当前库存。历史版本号为0时显示未记录。
- 真实 MySQL 事务测试通过：修改商品名称、价格、图片及软删除后，快照保持原值，跨用户及串用其他订单明细被拒绝；所有测试数据已回滚。前端隔离测试和无登录鉴权测试通过。

## 2026-10-16 订单超时扫描并发与动态延迟级别

- `conf/app.yaml` 新增 `order_expiration.interval_seconds`，默认 3 秒，主程序启动时传给调度器。每轮固定数据库当前时间作为截止时间，按 `(expire_time, id)` 游标前进；每批最多 50 条，先领取容量再查库，ants 独立池上限 200。满载可取消地等待，轮内连续取批次，轮末或查库失败后等待配置间隔。
- MQ 和扫描仍复用 `expiration.Service.CloseIfExpired`。锁单改为 `FOR UPDATE NOWAIT`，MySQL 3572 锁冲突正常跳过；其他错误返回并回滚。每次关单最多 15 秒 Context，库存按商品 ID 顺序返还。失败不阻塞游标，下一轮再尝试。
- 每单只发送一条 EXPIRE 检查；根据订单 `expire_time` 的剩余时间，选择大于等于该时长的最近 Broker 默认级别。10 分钟选 10 分钟，90 秒选 2 分钟，12 分钟选 20 分钟。
- 不修改 Broker 默认18级延迟，不新增消息字段、不分段转投。超过默认最大2小时或发送失败时记录错误，由数据库扫描处理。

## 管理员订单用户详情

- 管理员订单页在用户ID下增加「用户详情」展开按钮，按需加载名字和邮箱，支持收起及重新加载；普通用户不展示入口。
- `PUT /api/admin/orders/:id/user` 经过 RequireAuth 和 RequireAdmin，并在 Handler 再次明确校验管理员身份。Service 从订单确定用户ID，直接读取 MySQL 当前资料，仅返回 id/name/email，不经过用户缓存，不返回密码或凭证。
- 订单或有效用户不存在时返回404。退出、刷新或切换筛选时清除已展开资料，旧请求不得回写新会话。
- 25项前端隔离测试、后端鉴权测试、OpenAPI校验和构建通过；本次未连接真实数据库。

## 我的订单点击无响应修复

- Firefox 实际复现 `resetUserOrders` 报 `ReferenceError: orderUserDetails is not defined`：新 HTML 与旧缓存订单脚本混用导致切换中断。
- 将页面共用的 `orderUserDetails` 状态移到主页面初始化，并为工作台及详情页的外部脚本添加版本参数。后续修改这些脚本时同步更新引用版本，避免混用缓存。
- 26项前端隔离测试通过，增加缺少管理员详情扩展时用户订单仍能切换的回归用例；已刷新用户 Firefox 并实际点击验证进入「我的订单」。


## RocketMQ Handler 测试分目录

- 消费 Handler 的测试统一位于 `rocketmq/handler/test/`，使用独立的 `handler_test` 包，通过 `handler.Handlers()` 调用真实注册入口。
- Canal 输入协议辅助结构位于 `helpers_test.go`；商品副作用失败测试改为验证真实消费者返回重试，业务代码不增加测试专用公开方法。
- 隔离测试：`DEMO_INTEGRATION=0 go test ./rocketmq/handler/test -count=1`；完整集成测试需提前申请 MySQL、Redis 连接权限，ES 使用 HTTP 替身，不连接真实 MQ/ES。


## MQ 基础包与 Handler 解耦，订单超时服务合并

- 根 `rocketmq` 包不再导入 `handler`，初始化函数不自动注册业务 Handler。`main.go` 和 `server/test/support/main.go` 在 MQ 初始化前显式遍历 `handler.Handlers()` 注册回调；MQ 基础测试继续注册自己的测试回调。
- 超时关单代码统一位于 `server/order_server/order_expiration.go`，作为 `OrderService.ProcessExpiredBatch` 和 `OrderService.CloseIfExpired` 方法，接口声明同步更新；延迟消息发送仍按订单实际截止时间选择级别。
- MQ 消费 Handler 与超时扫描调度器均调用 OrderService。保留15秒超时、NOWAIT冲突跳过、同事务返库及取消、按商品ID排序和重复关闭幂等。
- 移除 `server/order_server/expiration/` 子包，其测试迁到 `server/order_server/test/order_expiration_*_test.go`。本文较早记录中的旧路径属于历史状态，以本节为准。
- 构建、隔离测试及真实 MySQL 的 `TestExpirationTransaction` 通过，测试数据整体回滚；未启动或重启服务。

## MQ 只接收外部配置结构体

- 删除 MQ 内部 `LoadConfig` 及 YAML 文件定位、读取、解析逻辑，原 `config.go` 改为 `runtime_config.go`，只保留配置类型、运行时转换和回调注册。
- 初始化统一为 `InitAllRocketmq(config)`，指定组入口为 `InitRocketmqForConsumerGroups(config, groups...)`。main 和测试启动入口负责调用 util.LoadConfig 后传入 config.RocketMQ；不再提供配置路径或内部加载兜底。
- MQ目录Go代码已无YAML、ReadFile或ConfigFile引用。构建和Handler、消息、调度器隔离测试通过；未连接真实MQ或重启服务。


## 关单锁等待由入口显式选择

- `OrderService.CloseIfExpired(ctx, orderID, waitLock)` 与 DB 锁单方法显式接收 waitLock。MQ 延迟消息传 true（FOR UPDATE 等锁），扫描入口传 false（FOR UPDATE NOWAIT，3572跳过）。
- 仅不等锁模式吞掉3572；等锁时数据库错误、锁等待超时及15秒Context超时继续向上传播，MQ消费返回重试。两种模式共用原事务和状态校验。
- sqlmock隔离测试覆盖NOWAIT跳过、普通错误回滚、等锁SQL及错误传播，不连接真实服务。
