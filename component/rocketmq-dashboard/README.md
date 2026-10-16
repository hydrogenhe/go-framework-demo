# RocketMQ Dashboard

官方镜像 `apacherocketmq/rocketmq-dashboard:2.1.0`，通过 Docker Compose 部署。页面仅绑定本机：

- 页面：http://localhost:8082
- 容器访问宿主机 NameServer：`host.docker.internal:9876`
- Broker 使用 NameServer 返回的通告地址；该地址必须能从容器访问。

在自己的终端从项目根目录执行：

```bash
bash components_sh/restart_mq.sh
```

该脚本先检查 Docker 和镜像，再重启本机 NameServer / Broker，随后启动 Dashboard，验证页面及集群查询。MQ 日志和 store 沿用原配置，不清理消息。Dashboard 监控数据保留在独立 Docker 数据卷。

只重启 Dashboard：

```bash
docker compose -f component/rocketmq-dashboard/compose.yaml up -d --force-recreate dashboard
```

可用 `DASHBOARD_PORT` 修改页面端口；用 `DASHBOARD_NAMESRV_ADDR` 覆盖容器内 NameServer 地址。`restart_mq.sh` 默认将本机 `127.0.0.1` / `localhost` 替换为 `host.docker.internal`。

## 查看每条消息

打开 Message（消息）页面，选择 `product_cdc_topic`、`order_cdc_topic` 等 Topic 和时间范围，查询后点击消息 ID 或详情，查看消息正文、属性、队列和偏移量。查询范围要覆盖消息的存储时间。只能查询 Broker 保留期内的消息。

## 通信排查

1. 页面可打开，只证明 Dashboard Web 服务正常。
2. Cluster 页面应显示实际 Broker，证明 NameServer 和 Broker 查询正常。
3. Message 页面能查看正文，证明消息读取正常。

配置中的 `isVIPChannel: false` 使用 Broker 主端口。当前镜像固定使用 `linux/amd64`，Apple Silicon 通过 Docker Desktop 的仿真运行。页面未启用登录，仅绑定本机回环地址。
