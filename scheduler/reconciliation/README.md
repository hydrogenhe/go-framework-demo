# MySQL → ES 对账与补偿

对账实现位于公共包 `go-framework-demo/scheduler/reconciliation`。初始化 MySQL 和 ES 后，可调用 `reconciliation.Run(ctx, mysql.GetDB())` 执行一轮；主程序可调用 `reconciliation.RunScheduler(ctx, interval, mysql.GetDB())` 在独立 goroutine 中启动定时任务。

独立命令入口也复用该公共包，从项目根目录运行：

```sh
go run ./cmd/reconcile-es
```

命令会真实更新、删除 ES 文档，不修改 MySQL。连接必须指向 MySQL 主库。

执行顺序：先完成用户、商品、订单的所有 DB→ES 扫描与补偿，再执行 ES→DB 扫描。

- DB→ES：完整记录每批 100 条，用 `id > lastID ORDER BY id ASC` 分页；拿 ID 批量读取 ES，构建 `sourceMap`。遍历 DB 的 `targets`，优先比较 is_del，再逐字段比较；缺失或不一致时存入 `errorMap[id] = target`，记录差异并批量补偿 ES。DB 已软删除时补偿删除标记。
- ES→DB：复用现有 PIT + search_after，每批 100 条获取完整 ES 文档，按 ID 批量读取完整 DB 记录并构建 `targetMap`。DB 不存在或 is_del=1 时进入 `deleteIDs`；其余字段不一致时进入 `updateMap[id] = target`。每批先执行更新补偿，再执行删除。
- 创建时间比较 `create_time_stamp`，更新时间比较 `update_time_stamp`（分别对应 MySQL 的 `CreateTime`、`UpdateTime` 毫秒值）；ES 的 `create_time`、`update_time` 只便于查看，不参与业务判断。
- `source` 表示 ES 当前数据，`target` 表示 MySQL 目标数据。用户、商品、订单各自显式比较字段，没有递归 diff、反射或 DB JSON 转换。ES 解码字段用指针区分缺失值与零值。
- 订单在一个短读取事务里查询主表和批量查询明细；明细以 ID 匹配，顺序变化不算差异，缺失、多余及重复明细均记录。
- 每次 MySQL 查询结束后等待 100 毫秒再继续，降低连续对账查询的压力；等待期间收到取消信号会立即退出。
- Zap 日志标志是上海时区当天的 `Reconciliation+YYYY-MM-DD`，包含 index、id、action、fields 或原因。仅打印差异字段名，不打印完整用户数据。
- 更新、删除带扫描时的 ES 版本条件；缺失文档使用 create。并发冲突跳过并记 Warn，留给下轮对账；数据库报错会停止，Bulk 中的失败项也会返回错误。
- `Run` 只执行一轮；`RunScheduler` 启动后立即执行，完成后按给定间隔继续执行，可通过 context 取消。独立命令使用 `Run`，项目主程序使用 `RunScheduler`。PIT 会释放。

MySQL 与 ES 没有跨库事务，并发恢复或更新可能留待后续 CDC/对账收敛。CDC 写入的新删除墓碑会在反向扫描中清理。

模拟验证，不连接真实数据库：

```sh
go test ./scheduler/reconciliation
go test ./es/test -run TestProductSearchAfter
```
