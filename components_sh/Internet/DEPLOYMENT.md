# 云服务器部署

## Docker Compose 试玩部署

项目根目录提供两条云服务器脚本：

```bash
./components_sh/Internet/start_set_all.sh
./components_sh/Internet/start_go.sh
```

`start_set_all.sh` 首次运行会询问图片访问用的公网 IP/域名，生成随机数据库、Redis、Canal 和图片存储凭据，写入 `conf/*.yaml` 与 Canal 配置，再启动 MySQL、Redis、Elasticsearch、RocketMQ、SeaweedFS 和 Canal。MySQL 就绪后会执行 `components_sh/Internet/stack/mysql-init/00-create-tables.sql` 创建当前五张业务表；所有建表语句均为 `CREATE TABLE IF NOT EXISTS`，已有表和数据不会被覆盖。随后脚本创建或更新业务数据库账号及 Canal binlog 账号。秘密保存在权限为 `600` 且被 Git 忽略的 `components_sh/Internet/stack/.env`。重复运行会复用相同凭据和 Docker 数据卷。

`start_go.sh` 构建并重启 Go 后端和前端静态服务器容器。浏览器访问 `http://<服务器公网IP>:5500`，前端通过 `app.yaml` 中的 `frontend.api_base_url` 请求 Go API（默认由初始化脚本设置为 `http://<服务器公网IP>:8080`）。容器通过 Compose 服务名访问中间件。部署栈不包含 Prometheus、Grafana 等监控服务；映射前端 5500、Go API 的 8080 和图片读取的 8888 到宿主机，数据库、Redis、ES、RocketMQ 端口仅在 Compose 内网开放。

这两个脚本可重复运行：Compose 项目名固定，凭据复用同一个 `.env`，数据库及服务数据保存在命名卷中。脚本使用 `compose up` 拉起已停止的容器，配置变化时重建容器，并检查服务是否保持运行；脚本不执行 `down -v` 或删除命名卷。不要手动运行 `docker compose down -v`、`docker volume rm` 或带 `--volumes` 的清理命令，否则会删除持久化数据。

该配置按低流量试玩压低内存参数，4GB 主机仍可能因服务启动峰值触发 OOM；需要在 Ubuntu 上先设置 `vm.max_map_count=262144`。MySQL 启用 ROW binlog。这个初始化 SQL 适用于全新安装；在已有项目数据库上运行时，它只会创建缺失表，不会升级已有表结构。升级已有表请先备份，再按顺序检查并执行 `migrations/` 中尚未应用的脚本。

部署包默认面向 Linux x86_64。开发机在项目根目录运行：

```bash
./components_sh/Internet/package.sh
```

上传 `dist/go-framework-demo.tar.gz` 到服务器后解压。部署包包含静态编译的后端与前端服务，不要求云服务器安装 Go。进入解压目录，只修改 `conf/*.yaml`：

- `app.yaml`：后端监听地址、前端 API 公网地址/监听地址、日志目录、Snowflake 机器号及 ES 对账周期（`reconciliation.interval_hours`）。默认启动后立即对账，之后每 24 小时执行一次。
- `mysql.yaml`、`redis.yaml`：数据库和缓存连接信息。
- `es.yaml`、`rocketmq.yaml`：搜索和消息队列连接信息。
- `image_storage.yaml`：对象存储地址、Bucket、公网图片地址及凭据。模板中的 `replace-me` 必须替换。

所有环境共用上述 YAML 文件，不存在 `.local.yaml` 覆盖文件。MySQL 密码、生产凭据、服务地址和端口都直接填写在对应 YAML 中。

分别启动后端和前端静态服务器，由 systemd、Supervisor 等工具负责后台守护：

```bash
./bin/go-framework-demo
./bin/frontend-server
```

前端服务启动时会根据 `app.yaml` 生成 API 地址运行配置。因此修改 YAML 后重启对应服务即可，无需改代码或在云端编译。程序会从当前工作目录或可执行文件所在目录逐级向上查找 `conf/`，部署目录可以放在任意路径。

默认包为 Linux amd64。需要 Linux ARM64 包时，在打包机运行 `GOARCH=arm64 ./components_sh/Internet/package.sh`。MySQL、Redis、Elasticsearch、RocketMQ、对象存储等外部服务仍需事先部署且网络可达。使用 CDC 时，Canal 需单独部署和配置。

升级已有数据库时，先核对并执行 `migrations/007_order_expire_time.sql`，再启动新版本；该迁移会按订单创建时间回填支付截止时间，并创建超时扫描索引。
订单支付时限从 Redis 的 `order:payment_timeout_minutes` 读取并保存到订单 `expire_time`，默认 10 分钟。每单只发送一条超时检查消息：根据剩余支付时间，选择大于等于该时长的最近默认延迟级别。10 分钟选 10 分钟，90 秒选 2 分钟，12 分钟选 20 分钟，不修改 Broker、不分段转投。超出默认最大级别 2 小时或发送失败时记录错误，由数据库扫描兜底。消息到达时间仍受 MQ 延迟影响。

`conf/app.yaml` 的 `order_expiration.interval_seconds` 配置扫描间隔，默认 3 秒。启动后立即扫描；一轮固定数据库截止时间，按 `(expire_time, id)` 游标持续取数，每批最多 50 条。独立 ants 池最多并发 200 个扫描关单任务，满载时停止查库并等待容量；本轮结束或查询失败后再等待配置间隔，不为每批额外等待。MQ 消费不占用该池。

两个入口复用同一个关单事务，订单锁使用 `FOR UPDATE NOWAIT`；被锁订单立即跳过，其他错误回滚并退出本次任务，下一轮重新检查。单次关单 Context 超时 15 秒。上线前需按数据库连接容量与库存行锁竞争压测，200 为池上限，不代表数据库一定能承载同等并发。
