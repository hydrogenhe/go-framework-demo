# 项目设计手册

> 本文记录可复用的项目设计与开发规范。环境地址、运行状态、验证结果和待办事项统一记录在 `HANDOFF.md`、`Readme.md` 或部署文档中。

## 规范维护要求（后续 AI 必须遵守）

- 新增或修改规范前，先通读相关章节并检查已有规则；内容重复时合并到原条目，不在文末反复追加。
- 按主题组织文档，使用“大类 → 子节 → 具体规则”的层级。优先归入已有类别；现有分类无法清晰承载新内容时，可新增大类或子节，并同步调整目录和编号。
- 保留明确、可执行的约束，合并同义表述，精简重复说明和过长示例。去重不得削弱或遗漏原有要求；规则之间存在实质冲突时，结合用户最新指示核实，不擅自删除其中一条。
- 规范应直观、便于理解。架构、依赖和业务流程可用 Mermaid、目录树或对照表辅助说明；图文围绕同一规则，简单约束不为形式增加图表。
- 示例只保留理解规则所需的关键代码；明确标出旧写法与推荐写法。用户要求保留的对比示例应继续保留。
- 修改后核对规则、代码示例、图表和跨文档引用是否一致，修正过时字段、包路径、调用签名及章节引用；避免重复编号、失效链接和前后矛盾。

## 目录

1. 架构与目录组织
2. 数据访问与搜索同步
3. 事务与业务一致性
4. 接口、身份与错误处理
5. 代码风格与接口声明
6. 配置与组件初始化
7. 测试规范
8. 脚本与运行管理


## 1. 架构与目录组织


### 1.1 分层职责

```text
HTTP Handler → Server → DB
                    ├→ Cache
                    ├→ Search
                    └→ Message Queue
```

#### HTTP Handler 层

负责：

- 注册路由
- 解析请求参数
- 做基础参数校验
- 调用 Server 层
- 返回统一 HTTP 响应

不负责：

- 直接写数据库
- 直接操作缓存、搜索或消息队列
- 编排事务和复杂业务流程

#### Server 层

负责：

- 编排完整业务流程
- 校验业务字段、重新读取数据库核对商品价格和库存、计算订单金额及检查状态流转
- 决定事务边界
- 调用 DB、Cache、Search 和 Message Queue
- 把底层错误转换成业务语义

Server 层是业务规则的主要承载位置，避免业务逻辑散落在 Handler、DB 或基础设施代码中。

#### DB 层

负责：

- 封装数据库 CRUD
- 使用统一的查询对象和模型
- 接收 `context.Context`
- 支持事务 Query

DB 层不应该知道 HTTP 请求格式，也不应该返回前端响应。业务参数校验、业务判断和事务流程编排统一放在 Server 层，DB 层保持简单的数据访问职责。


### 1.2 业务目录与文件定位

- 订单、商品、用户分别放在 `server/order_server/`、`server/product_server/`、`server/user_server/`，目录内文件使用对应业务包名。
- 同一业务的 Service、查询、状态操作、校验、更新参数和业务测试集中在对应目录；公共测试辅助仍放在 `server/test/support/`。


推荐使用“业务名 + 层名”的命名方式：

```text
http_handler/product_handler.go
server/product_server/product_server.go
db/product_db.go
search/product_search.go
message/handler/product_handler.go
```

按问题定位文件：

```text
路由和参数问题       → http_handler/
业务流程问题         → server/
数据库问题           → db/
缓存问题             → cache/ 或 redis/
搜索问题             → es/ 或 search/
消息发送问题         → producer/ 或 message/
消息消费问题         → consumer/ 或 message/handler/
错误码问题           → const/ 或 errors/
启动和依赖问题       → scripts/、config/ 或 conf/
```

清晰的文件名可以让新成员和 AI 快速找到业务代码，减少全项目搜索。


### 1.3 MQ 基础能力与业务消费处理

- `rocketmq` 根包只提供配置、客户端初始化、消息发送、消费回调注册和生命周期管理，不导入 `rocketmq/handler` 或业务 Service 包。
- `rocketmq/handler` 解析业务消息、调用对应 Service 并返回消费结果；订单事务与业务判断集中在 `server/order_server/`，不因消费入口再拆独立业务子包。
- 应用启动入口先注册具体 Handler，再初始化消费者；测试启动入口也须显式注册所需 Handler。业务 Service 可以调用 MQ 发送接口，根 MQ 包只执行已注册回调，避免循环依赖。

```text
main → handler → order_server → rocketmq
main → rocketmq（先注册回调，再初始化）
```


## 2. 数据访问与搜索同步


### 2.1 基础数据约定

- ID 统一由一个工具生成，避免各业务自行生成不同格式。
- 金额使用整数最小单位保存，避免浮点误差。
- 状态使用 int 枚举和集中定义的常量。
- 密码等敏感字段禁止明文保存。
- 数据库模型、查询对象和业务 DTO 分开，避免把数据库结构直接暴露给前端。


### 2.2 Schema 变更与代码生成

- 开发阶段开始实现前，先检查需求是否依赖 MySQL 表/列/索引或 Elasticsearch 索引/字段映射变更。把所需变更、迁移内容和目标开发环境说明清楚；未经用户明确授权，不得执行数据库 DDL/DML 迁移、修改 ES 索引或映射。
- 一旦确认需要 schema 变更，先申请授权并完成变更，再验证目标 MySQL 表结构和 ES mapping 已生效；MySQL 变更随后按本节流程重新生成 Model 与 Query。只有数据库和 ES 结构均可供代码使用后，才开始编写依赖这些字段的业务代码，避免先写代码、最后才发现运行环境缺列或缺映射。
- 未获授权或目标环境不可用时，可以先整理迁移 SQL、ES mapping 和影响说明，但不得实现依赖尚未应用字段的代码；明确告知阻塞点并等待授权或环境就绪。
- 本项目所有纳入 `cmd/generate/gen.go` 的表统一使用 `g.GenerateModel` 从已应用的数据库结构生成 Model 和 Query，再通过 `g.ApplyBasic(models...)` 一起输出。不要把手写 Model 作为 `ApplyBasic` 输入来代替 Model 生成，也不要在生成前让业务代码引用尚不存在的 Model/Query。
- 任何数据库表结构变更都必须先写并应用 migration，再从项目根目录运行 `go run ./cmd/generate`，将对应的 `model/*.gen.go` 与 `query/*.gen.go` 一并更新并提交。生成文件是生成器的产物，禁止手工编辑；需要适配软删除类型、自定义字段类型或 GORM 标签时，在 `cmd/generate/gen.go` 配置 `GenerateModel` 字段选项，并在普通业务代码（例如 `model/db_types.go` 的类型别名、独立 hooks）中提供兼容，不要改生成结果。
- 生成后检查 `query/gen.go` 是否注册新表、生成 Model 与实际列/主键/类型一致、Query 包含新增列；随后构建并运行覆盖变更的数据库集成用例。当前 `is_del` 的 `DeletedAt` 类型和 `softDelete:flag` 标签由生成配置处理，`model.DeletedAt` 只是软删除插件类型的别名。

标准步骤：

```text
编写数据库 migration
  → 应用 migration 到生成器连接的数据库
  → 更新 cmd/generate/gen.go 的 needGenTables 和 GenerateModel 字段适配选项
  → 运行 go run ./cmd/generate
  → 检查并提交生成出的 Model 与 Query
  → 构建并运行对应数据库集成测试
```


### 2.3 GORM Gen 查询与更新

- DB 层统一先从当前 `GenQuery` 取得表查询句柄，再通过该句柄的字段及字段方法（如 `productQuery.ID.Eq(id)`、`productQuery.Stock.Gte(quantity)`）构造过滤条件。查询、更新、删除均沿用此方式；禁止为普通单表操作改用 `UnderlyingDB().Model(...)` 和字符串 `Where("id = ?", id)`。只有 Gen 无法表达的专用 SQL 片段才使用 GORM 表达式，并保留 Gen 字段条件和事务 Query。
- 多字段更新使用当前表查询句柄构造 `field.AssignExpr`，非 nil 的字段逐项加入后，通过一次 `UpdateColumnSimple(columns...)` 执行。不要把字段更新组装成 `map`，也不要逐字段调用 `UpdateColumn`，以免产生多条 UPDATE 并造成部分成功。指针字段的 nil 表示不更新，非 nil 的零值仍须写入；所有字段均未提供时按接口约定返回参数错误。`UpdateColumnSimple` 不触发 GORM 自动更新时间，需要更新时间时将 `UpdateTime.Value(time.Now())` 显式加入更新列。

- 手写 DB 代码中的列赋值、加减运算统一使用当前表查询句柄提供的 GORM Gen 字段表达式，如 `Stock.Sub(quantity)`、`Stock.Add(quantity)`、`Price.Value(price)`，通过 `UpdateColumnSimple` 执行。Gen 能表达的操作不再手写列名或使用 `gorm.Expr` 拼接 SQL；只有 Gen 无法表达的专用 SQL 才使用原生表达式。
- 加减直接在数据库内执行，不能改成先查询、在 Go 中计算后覆盖写回，以免并发更新丢失。原有主键条件、库存充足条件、版本校验、软删除作用域和事务 Query 必须保留。

原写法：

```go
result, err := productQuery.WithContext(ctx).Where(
    productQuery.ID.Eq(id),
    productQuery.Stock.Gte(quantity),
    productQuery.CurrentVersion.Eq(version),
).UpdateColumn(productQuery.Stock, gorm.Expr("stock - ?", quantity))
```

统一后的写法：

```go
result, err := productQuery.WithContext(ctx).Where(
    productQuery.ID.Eq(id),
    productQuery.Stock.Gte(quantity),
    productQuery.CurrentVersion.Eq(version),
).UpdateColumnSimple(productQuery.Stock.Sub(quantity))
```

两种写法都在数据库中执行 `SET stock = stock - ?`；统一后的写法使用 Gen 字段表达式，避免手写字段名。返还库存对应使用：

```go
result, err := productQuery.WithContext(ctx).Unscoped().
    Where(productQuery.ID.Eq(id)).
    UpdateColumnSimple(productQuery.Stock.Add(quantity))
```

上面的 `Unscoped()` 仅沿用超时关单返库需要处理软删除商品的业务约定；普通库存更新保留默认软删除作用域。


### 2.4 批量数据库读写

- 当一次业务操作已持有多条记录的 ID，且数据层提供批量查询接口时，先收集 ID 并批量读取，避免在循环中逐条访问数据库造成重复往返。
- 将查询结果按主键整理为 `map[ID]*Model`，后续按原始请求中的 ID 查找记录并执行校验、计算或组装。不要依赖批量查询结果的返回顺序与请求顺序一致。
- 对映射中不存在的 ID 显式处理，例如返回记录不存在或数据已变化的业务错误；不要静默跳过。
- 只有确实存在多条记录的批量场景才使用此模式。单条读取无需为了形式创建 ID 切片和映射。
- 同一业务操作需要写入多条同类记录，且数据层支持批量接口时，应先准备好记录切片，再调用一次批量写入；避免在循环中逐条执行 INSERT、UPDATE 或 DELETE。
- 批量写入属于事务业务时，应与关联的扣减、主记录及其他明细共用同一个事务；若数据库或驱动有单次批量大小限制，再按限制分批执行，并保持事务边界。

```go
productIDs := make([]int64, 0, len(inputs))
for _, input := range inputs {
	productIDs = append(productIDs, input.ProductID)
}

products, err := productDB.GetByIDs(ctx, productIDs)
if err != nil {
	return err
}
productsByID := make(map[int64]*model.Product, len(products))
for _, product := range products {
	productsByID[product.ID] = product
}

for _, input := range inputs {
	product, ok := productsByID[input.ProductID]
	if !ok {
		return appconst.NewConflictError("商品不存在或数据已变化")
	}
	// 使用 product 执行业务校验或组装数据。
}
```

批量创建订单明细时，先补齐共同的订单 ID，再一次写入整个切片：

```go
for _, item := range items {
	item.OrderID = order.ID
}
if err := orderItemDB.BatchCreate(ctx, items); err != nil {
	return err
}
```


### 2.5 CDC 同步与幂等

当 MySQL 是业务事实数据源、搜索引擎只是查询副本时，可以采用：

```text
MySQL
  → Binlog
  → CDC 工具
  → Message Queue
  → Search Consumer
  → Elasticsearch
```

设计约束：

- MySQL 是事实数据源。
- Elasticsearch 是可重建的查询副本。
- 消费者必须支持幂等，通常使用业务主键作为文档 ID。
- Binlog 事件需要记录来源、表名、事件类型和主键。
- 业务事件和 CDC 数据变更事件使用独立的 Topic、消费者组、消息结构和 Handler；业务 Handler 处理业务事件，ES 同步 Handler 只处理 CDC，避免混用协议或重复写入搜索副本。
- 需要记录投递失败、消费失败和 ES 写入失败的日志。
- 不要在业务接口中无条件同时写 MySQL 和 ES，避免双写不一致。

#### CDC 删除的幂等处理与重试边界

- CDC 消息可能重复投递，删除操作必须支持幂等：目标文档已经不存在时，删除目标已达成，应返回消费成功，避免将同一删除事件反复送入 MQ 重试队列。
- Elasticsearch 删除接口返回 HTTP 404 时，必须检查响应语义。只有响应中的 `result` 为 `not_found`（目标文档不存在）才按成功处理；不能无条件忽略所有 404。索引不存在、路由错误等异常，以及权限、连接、超时和服务端错误，仍应返回错误并交由消费层处理重试或告警。
- 在统一 ES 删除封装中落实该规则，使所有 CDC 消费者使用同一处理方式。正常的“文档已不存在”不应记录为消费失败。

```go
result, err := client.Delete().Index(index).Id(docID).Do(ctx)
if elastic.IsNotFound(err) && result != nil && result.Result == "not_found" {
    return nil // 文档已不存在，删除成功。
}
if err != nil {
    return fmt.Errorf("删除ES文档失败: %w", err)
}
return nil
```


### 2.6 搜索检索与事实源回查

- 除非需求明确指定从缓存读取，否则返回前端的真实业务数据必须最终从 DB 获取。不得直接将 ES 索引或其他查询副本中的业务字段作为最终详情返回。
- ES 负责复杂条件筛选、排序、分页等检索任务，返回匹配的 ID 集合及命中总数；Service 使用这些 ID 调用 DB 批量查询，再组装响应。价格、库存、图片、状态等字段以 DB 当前数据为准。
- 回查必须使用同一个请求 Context，并采用批量查询，避免逐条查询导致 N+1 问题。DB 的 IN 查询不保证顺序，Service 应按 ES 检索顺序恢复结果。
- DB 中不存在或已删除的记录不返回。ES 异步同步可能导致回查后的单页数量少于页大小；命中总数与实际返回条数应明确区分，不得用单页数量替代检索总数。

标准链路：

```text
请求条件 → ES 检索 → ID 集合 → DB 批量查询 → Service 按检索顺序组装 → 返回前端
```


### 2.7 ES 查询组装

- 同一业务的 ES 查询条件统一由对应的 `Build...ElasticQuery` 方法组装，集中管理过滤条件、匹配条件、分页、排序及总数统计选项，避免规则散落在执行方法中。
- 组装方法只接收业务查询参数和分页参数，负责参数校验，将查询条件、分页和排序放入同一个 `SearchSource`，只返回该句柄及错误；不依赖 ES Client，不创建或执行 `SearchService`。
- 查询执行方法负责取得 Client、选择索引，并通过 `c.Search(index).SearchSource(source).Sort("id", false).TrackTotalHits(true).Do(ctx)` 执行查询和解析结果；请求 Context 沿用调用方传入的 Context。
- 修改查询规则时优先调整组装方法，并通过真实查询入口验证最终发送的 ES 请求和返回结果。

- 可选查询参数统一放在 Options 结构体中，使用指针区分未传入与明确传入的零值；非 nil 才添加对应条件。分页的 Page、PageSize 必须同时提供，均为 nil 时不显式设置分页（仍受 ES 默认返回条数限制）。
- 更新时间排序默认倒序，AscUpdate 指针为 true 时升序；ID 等固定的兜底排序放在查询执行处。


### 2.8 软删除与查询边界

- 软删除标记统一使用 `is_del`，数据库默认值为 0，删除后为 1。
- GORM 模型使用官方软删除插件配置默认查询和更新作用域，避免在每个 DB 方法重复拼接条件。必须通过真实 CRUD 测试确认 Delete 变为 UPDATE、查询/统计排除删除记录、库存更新不命中已删除商品。
- 普通业务查询使用默认作用域；CDC、维护和审计入口通过名称明确的 IncludingDeleted 方法显式解除作用域。原生 SQL 不自动继承模型软删除规则。
- ES 文档和 nested 明细保留软删除状态，正常查询显式过滤 `is_del=0`。对 nested 明细的匹配及响应展示也应排除已删除明细。
- CDC 从事实源读取包含软删除状态的最新记录，避免直接用延迟事件中的旧字段复活记录；无法找到主键时写删除墓碑。进程内串行不等于跨实例顺序保证。

```go
// 普通查询自动过滤 is_del=0。
user, err := q.User.WithContext(ctx).Where(q.User.ID.Eq(id)).First()

// 维护查询显式只查已删除记录。
deleted, err := q.User.WithContext(ctx).Unscoped().
    Where(q.User.IsDel.Eq(1)).Find()
```


## 3. 事务与业务一致性


### 3.1 事务入口与 Context

- 所有业务事务统一通过 `db.WithTx` 开启，不在 Server、DB 或其他业务代码中直接调用 GORM `Transaction`。只有 `db.WithTx` 的实现本身调用 GORM `Transaction`。
- `db.WithTx` 默认使用全局 Query；需要沿用显式传入的 Query（例如外层事务）时，必须将它作为第三个参数传入，避免切回全局连接。
- 服务编排层需要保存或传递根 Query 时，使用语义中立的 `genQuery` 字段；事务可能跨多个 DB 对象，不要借用某个单表 DAO 的 `GenQuery` 字段表达服务级事务连接。
- DB 方法显式接收 `context.Context`。
- 事务回调必须使用回调参数 `txQ` 创建 DB 对象或执行查询；事务内禁止调用默认构造器切回全局 Query。
- 事务内部沿用业务 Context，不要在中途使用 `context.Background()` 替换。
- Context 用于超时、取消、链路追踪和请求生命周期传递。

典型 Service 写法：

```go
err := db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
    userDB, err := db.NewUserDb(ctx, txQ)
    if err != nil {
        return err
    }
    if _, err := userDB.Create(ctx, user); err != nil {
        return err
    }
    return nil
}, s.genQuery)
if err != nil {
    return appconst.NewInternalError(err)
}
```


业务判断失败从回调返回明确的业务错误；DB 或基础设施错误保留原始错误链，由 Service 在事务结束后统一包装。


### 3.2 库存、状态流转与超时关闭

尽量在开启事务前完成商品读取、请求快照核对和业务校验，缩短事务及行锁持有时间。单独的“先查库存再扣库存”不能防止并发超卖，因此最终库存校验和扣减必须合并为一条原子条件更新。

- 通用原则：事务内只执行保证数据一致性所必需的数据库操作；能在事务外完成的业务校验、数据准备和计算应尽量提前完成，避免在事务持锁期间执行无关逻辑。
- 事务外完成普通入参校验、权限校验、数据准备和可独立完成的业务判断。例外是依赖被锁定记录当前状态的并发一致性判断：必须在事务内读取并锁定记录后再校验状态，并在同一事务中完成状态变更。例如发货前确认订单仍为已支付；事务外预检查不能替代锁内复核，否则并发请求可能基于同一旧状态重复执行。
- Server 在事务外读取商品并完成价格、商品快照、商品数据合法性及请求参数校验，准备订单金额和明细。
- 事务由 Server 层编排；订单创建、明细写入和库存扣减使用同一个事务 Query 和请求 Context。
- DB 扣减方法使用类似 `WHERE id = ? AND stock >= quantity` 的条件，在一条更新中原子扣减库存；返回影响行数，由 Server 判断是否成功。
- 如果原子更新影响行数为 0，说明商品不存在或当前库存不足；Server 应返回明确的冲突错误，事务回滚，不得继续创建订单。
- 价格、名称和图片等业务校验在事务外完成。若业务要求并发修改这些字段时也必须拒绝下单，应另行引入版本号或把对应的预期值纳入条件更新；不能认为事务外校验自动保证之后的值未变化。
- 不要仅依赖事务外库存预检，也不要为了重复校验而在事务中先锁行查询；数据库原子更新负责并发安全，事务保证扣减、订单和明细一起提交或回滚。
- 唯一索引、非空约束和原子更新属于持久化一致性保障，可以保留在数据库；它们不替代 Server 的业务校验。
- 待支付订单的截止时间保存在 `order.expire_time`，固定支付窗口由订单创建流程写入；超时关闭和支付必须在事务中锁定订单并以“仍待支付”作为状态前提。关闭订单与恢复预扣库存必须处于同一事务，重复触发不得重复返还库存。
- 超时任务使用 `(status, expire_time)` 复合索引分批扫描；延迟消息用于及时触发，索引扫描用于补偿消息丢失、发送失败或服务停机。

推荐下单流程：

```text
Server 校验入参、重复商品
  → Server 查询商品并核对价格、商品快照、库存及商品状态
  → Server 计算金额并准备订单和明细
  → Server 开启事务
  → DB 原子校验关键商品数据并扣减库存，返回影响行数
  → Server 判断扣减结果
  → DB 写入订单及明细
  → Server 提交事务；任一步失败则回滚
```

测试也按职责组织：字段和业务规则测试调用 Server 入口；DB 测试验证查询、锁定、条件更新和持久化结果；订单集成测试验证并发库存约束及事务回滚。

- 不通过业务回调把 Service 的判断传入 DB；DB 返回查询结果、影响行数和原始错误，由 Service 判断业务结果。


## 4. 接口、身份与错误处理


### 4.1 HTTP 身份与权限

- 需要用户身份的路由先通过登录中间件校验请求头 Bearer token 的服务端会话，再从请求 Context 获取真实用户，不以请求中的 ID、姓名或角色确定登录身份。
- Handler 使用前端传入的用户 ID 前，先检查登录用户存在、请求 ID 格式正确且为正整数，并核对该 ID 是否等于当前登录用户 ID。核对通过后可以将该请求 ID 传入 Server。
- 跨用户访问必须由路由明确授权，例如管理员查询指定用户订单；管理员身份由服务端校验，不允许通用校验默认放行所有管理员跨用户操作。
- 校验在调用业务服务前完成；缺少登录态返回未登录，非法 ID 返回参数错误，无权访问其他用户返回权限错误。新增使用用户 ID 的路由应复用统一校验并补充越权测试。


### 4.2 统一响应与错误转换

统一响应建议至少包含：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

错误应分成两类：

#### 业务错误

例如：

- 参数校验失败
- 用户不存在
- 商品不存在
- 库存不足
- 状态不允许当前操作

所有校验、业务判断不通过都必须在对应 Service 中明确用 `err` 包装成业务错误（`MsgError`），通过 `code` 区分错误类型、`message` 提供用户可读的报错信息（代码字段为 `Code` / `Message`），并设置对应 HTTP 状态。DB 保留原始错误类型，Service 使用 `errors.Is` / `errors.As` 判断记录不存在、唯一键冲突等明确结果；禁止通过错误文本推断业务类型。响应层仅输出明确的业务错误，其余错误统一作为系统内部错误。

#### 系统错误

例如：

- 数据库连接失败
- Redis 超时
- 消息队列不可用
- Elasticsearch 请求失败

这类错误应该记录完整日志，但不要把数据库、网络或底层组件的原始错误直接返回给前端。前端只需要收到通用提示和错误码。

- 返回错误时统一调用 `NewInternalError(err)`：函数内部识别错误链中的已有 `MsgError` 并直接返回，只有未包装的错误才转换成系统内部错误。调用方不重复写 `AsMsgError` 判断；明确的数据库业务结果仍由 Service 转为对应业务错误。

```go
if err != nil {
    return false, appconst.NewInternalError(err)
}
```

`MsgError` 定义于 `const/error.go`，包含 `Code`、`Message`、`HTTPStatus` 和仅供内部追踪的 `Err`，通过 `Unwrap` 保留错误链。追加上下文使用 `%w`，禁止按错误文本推断业务类型。

```go
// Service 中明确校验，不交给响应层推断。
if userID <= 0 {
    return nil, appconst.NewInvalidParamsError("用户ID必须为正整数")
}
user, err := s.userDb.GetByID(ctx, userID)
if errors.Is(err, gorm.ErrRecordNotFound) {
    return nil, appconst.NewNotFoundError("用户不存在")
}
if err != nil {
    return nil, appconst.NewInternalError(fmt.Errorf("查询用户失败: %w", err))
}
return user, nil
```

响应层只识别已经明确包装的 `MsgError`；未包装的错误也统一兜底为内部错误，不检查错误文本。

```go
msgErr := appconst.NewInternalError(err)
response := appconst.Response{
    Code:    msgErr.Code,
    Message: msgErr.Message,
}
c.JSON(msgErr.HTTPStatus, response)
```


### 4.3 关键错误日志

- 对数据库写入、消息发送、Elasticsearch 写入/删除等关键外部操作，如果调用失败会导致数据未持久化、未投递或副本不一致，必须在该操作的调用点打印一条详细错误日志，然后继续向上返回错误。
- 不要给每个 `if err != nil` 都加日志。参数校验、格式转换等错误通常只需返回带上下文的错误；只有在这些错误需要独立排查、且上层没有记录时，才单独打印。
- 调用点日志必须能回答“哪个方法、对什么资源执行什么操作、处理哪条业务数据、失败原因是什么”。按场景包含：方法名、操作名、目标表/索引/Topic、业务主键、消息 ID，以及原始错误。字段不可用时省略，不要填造值。
- 示例：`[handleUserCDC] ES用户文档写入失败: msgId=... userID=... index=... err=...`。
- 上层 Consumer 可以另外记录消费失败和重试状态；不要在每层重复打印同一条详细错误。调用点负责操作细节，上层负责消费结果（例如重试）。
- 日志不得包含密码、访问令牌等敏感数据，也不要输出整条消息正文，除非排查确实需要且已确认不含敏感信息。


### 4.4 接口文档维护

- 接口文档统一维护于 `docs/openapi.yaml`，使用 OpenAPI 3。业务 Handler 仅保留简短中文说明，不写大段 Swagger 格式注释，不创建仅服务于注释生成器的 Go DTO。
- 新增或修改接口后同步文档的请求参数、响应 schema、认证和业务错误；运行 `components_sh/generate_api_docs.sh` 校验路由与内部引用并生成 JSON。
- YAML 是唯一编辑源，JSON 是生成物；页面直接读取 YAML，更新后需重新编译、重启后端。路由校验不能替代字段和业务语义的人工核对。


## 5. 代码风格与接口声明


### 5.1 函数参数与指针语义

- 手写函数接收更新 DTO、查询条件、配置等入参结构体时，统一使用 `*T`，例如 `input *ProductUpdateInput`、`opt *ProductSearchOptions`。
- 外部 HTTP 查询协议中的所有查询条件字段必须使用指针类型，包括字符串、数字、时间和布尔条件。`nil` 表示调用方未提供该条件、不应用过滤；非 `nil` 表示调用方明确提供并要求按该值处理，因此 `0`、`false` 等零值也必须保留。对显式空字符串按字段规则校验或规范化，不能仅因值为空就与“未提供”混为一谈。解析后再映射到内部查询 Input/Options；前端无筛选时省略对应 query 参数。
- 调用方先用具名变量组装入参，再传入 `&input`；已经是指针的参数继续直接传递，避免多余解引用和复制。
- 接收指针后，在解引用前明确处理 `nil`。Service 将空的必要入参包装为参数业务错误；DB 和基础设施入口返回明确错误，不能因空指针 panic。
- 整体入参指针和字段指针职责不同：整体指针统一传参风格；更新字段中的 `nil` 表示不修改，非 nil 的零值表示显式更新，不能丢失该语义。
- 不因改成指针而修改调用方对象。切片、map、接口及基础类型沿用其原有传参语义；不修改自动生成代码或第三方要求的接口签名。

```go
input := product_server.ProductUpdateInput{
    Price: &price,
    Image: &image,
}
product, err := service.Update(ctx, productID, &input)
```

- 函数签名只保留调用方确实需要提供或控制的参数。轻量业务对象及可以在函数内部取得或推导的依赖，不必让调用方先构造再传入；中间件客户端须遵循第 6 节的统一初始化规则，不能在业务函数中反复创建。
- 例如路由注册函数内部统一创建业务 Service，只接收需要注册路由的 Router；不单独要求外部传入某一个 Service。初始化方法也不保留已经不使用的参数。
- 同类依赖采用一致的创建方式，避免一部分在内部创建、一部分无实际需要却由外部传入。
- 请求 Context、事务 Query、调用方配置及确实需要共享生命周期或替换的依赖仍应显式传入；不能为减少参数而丢失请求取消、事务边界或必要的可配置能力。


### 5.2 部分更新协议

- 前后端明确约定可修改字段；HTTP 请求及 Service 更新入参使用字段为指针的结构体，不使用任意字段的 `map[string]interface{}` 作为公开更新协议。
- 未传字段或 JSON `null` 解析为 nil，表示不修改；非 nil 表示明确要求修改，包括 `0` 和空字符串。非 nil 字段仍须通过业务校验，例如库存 0 合法，商品价格 0 不合法。普通指针不能区分未传与 null；需要“null 表示清空”的业务应单独定义可区分这两种状态的输入类型。
- 校验函数只做字段范围、权限等业务校验并返回 error，不负责返回更新 map 或组装 SQL。Service 校验通过后，把原始指针结构体传给 DB；共享参数类型放在双方可依赖的公共模型包，DB 不反向依赖 Service。
- DB 更新方式遵循第 2.3 节，Service 不拼接数据库字段。主键、归属、创建时间等禁止修改的字段不放入更新结构体；HTTP 应拒绝未知字段。
- 参数转换、更新列收集等逐字段组装逻辑应提取到职责单一的子函数，避免字段增加后主流程堆积大量分支。DB 更新列组装函数负责处理 nil、收集赋值表达式及拒绝空更新，不执行 SQL、不承担业务校验；主更新方法只调用组装函数、执行更新并返回结果。同类规则适用于批量参数和请求/响应对象的复杂组装，新增字段在对应组装函数中维护。
- 前端只发送本次要修改的字段，不发送完整旧记录；不要用真假判断过滤字段，否则会丢失 0 和空字符串。空更新明确返回参数错误。专用业务动作（例如订单发货）继续由服务端确定更新内容与执行业务校验，不能让前端任意指定订单状态。


### 5.3 结构体组装与返回值

调用函数时，包含多个字段的结构体入参应在调用前声明为具名变量，每个字段单独一行，再将变量传入函数；不要在函数调用括号内内联组装。循环中不变的参数应在循环外组装一次，方便直接查看参数字段及其取值。业务代码和测试代码均遵守这一写法。

```go
page, pageSize := 1, 1
searchOptions := es.ProductSearchOptions{
    ID:       &product.ID,
    Page:     &page,
    PageSize: &pageSize,
}
items, _, err := es.GetProductsPage(ctx, &searchOptions)
```

测试调用业务接口时，多个相关参数应优先使用结构体集中定义，再在调用处传入字段。结构体可以放在测试文件顶部或独立的测试辅助文件中，避免在函数调用处重复拼接大量字面量参数，也便于修改和复用测试数据。结构体字面量包含多个字段时，每个字段单独换行，保持代码清晰。

例如商品创建测试可以定义：

```go
type productCreateInput struct {
    Name  string
    Price int64
    Stock int32
    Image string
}
```

调用时只负责生成动态名称并传递字段：

```go
input := productCreateInput{
    Name:  "server_product_" + util.GenerateAlnum(8),
    Price: 19900,
    Stock: 10,
}
product, err := service.Create(ctx, input.Name, input.Price, input.Stock, input.Image)
```

- 函数返回由多个依赖或字段组成的结构体时，先在函数体内创建依赖，再用具名局部变量组装返回结构体，最后单独 `return` 该变量；不要把结构体字面量直接写在 `return` 后面。
- 这样便于逐步阅读依赖的创建和结构体的组装过程，属于可读性约定，不应宣称它能减少一次分配或带来性能提升。性能判断应以编译器优化结果和实际测量为准。

```go
func newRouteHandlers(ctx context.Context) routeHandlers {
	userService := user_server.NewUserService(ctx)
	productService := product_server.NewProductService(ctx)

	handlers := routeHandlers{
		userService:    userService,
		productService: productService,
	}
	return handlers
}
```


### 5.4 局部变量与辅助函数

- 同一函数内多次使用同一个较长的取值表达式时，在首次使用前提取为具名局部变量，再复用该变量。HTTP Handler 和中间件多次使用请求上下文时，在开头写 `ctx := c.Request.Context()`，后续业务、数据库、Redis 和日志调用统一传入 `ctx`。不能用 Gin 的 `c` 替代请求上下文，以免丢失 `log_id` 等上下文值；如果函数中替换了请求上下文，后续应使用更新后的上下文。
- 手写 DB 操作先将表查询对象保存到具名局部变量，再用于构建查询条件和执行操作，避免反复写 `p.GenQuery.Product` 等长引用。
- 使用明确的名称，例如 `product`、`productQuery`、`orderQuery`；与模型参数或查询结果重名时使用 `Query` 后缀。
- 从当前 DB 对象的 `GenQuery` 获取表查询对象，保留调用方传入的事务 Query；每次执行查询仍显式绑定 `WithContext(ctx)`。
- 这是可读性约定，保持原有 SQL、错误处理和返回结果；不修改自动生成的 model、query 文件。

- 辅助逻辑应定义为函数外部的具名函数，不要在函数内部定义局部辅助函数。所需数据通过参数显式传入，避免闭包隐式捕获变量。
- 对账、校验等涉及两份数据对比的代码，双方变量统一使用 `source` 和 `target`：`source` 表示当前待校验的数据，`target` 表示应达到的目标状态或事实数据。集合及索引可使用 `sourceRows`、`targetRows`、`sourceMap`、`targetMap` 等名称，保持含义一致。


例如：`source := esDocument.Name`，`target := mysqlRow.Name`，将 ES 当前值与 MySQL 事实值比较。


### 5.5 常量注释

- 所有手写 Go `const` 常量必须逐项添加中文注释，包括公开常量、私有常量、函数内局部常量、测试常量和兼容别名；常量块的总体注释不能替代每个值的说明。自动生成文件不手工修改，应通过生成器维护。
- 注释说明用途或业务含义，涉及时间、金额、容量、数量等值时写明单位和边界；枚举类型及每个枚举值都要有说明。公开常量的注释以常量名称开头；已有清楚的行尾注释可保留。
- 新增或修改常量时同步更新注释，不只重复字面值，也不保留与当前用途无关的历史描述。

```go
// OrderStatus 表示订单支付及完成状态。
type OrderStatus int32

const (
    // OrderStatusPendingPayment 订单待支付。
    OrderStatusPendingPayment OrderStatus = 1
    // OrderStatusPaid 订单已支付。
    OrderStatusPaid OrderStatus = 2
)
```

### 5.6 公开方法与接口声明

手写的 DB、Service、缓存、存储等操作类型，**必须在对应结构体附近定义 `类型名Interface`，完整列出该类型对外提供的方法**。阅读接口即可了解该类型具备哪些能力，无需逐个文件查找方法实现。

- **新增、删除或修改公开方法时，必须同步更新接口声明**，包括分散在其他文件中的方法，避免接口清单与实现脱节。
- 接口签名必须与实现一致，包括参数类型、Context、指针、可变参数和返回值。
- 每个实现必须添加编译期断言，确保其满足接口：

```go
// ProductServiceInterface 集中声明商品服务对外提供的全部方法。
type ProductServiceInterface interface {
    List(ctx context.Context, input *ProductQueryInput) (*ProductPage, error)
    Create(ctx context.Context, name string, price int64, stock int32, image string) (*model.Product, error)
    Update(ctx context.Context, id int64, input *ProductUpdateInput) (*model.Product, error)
    Export(ctx context.Context, input *ProductQueryInput) (*ProductExport, error)
}

type ProductService struct{ productDb *db.ProductDb }

var _ ProductServiceInterface = (*ProductService)(nil)
```

**代码审查必须检查接口是否完整，不能只看编译是否通过。** 编译期断言能检查接口中声明的方法是否已实现，但不能发现实现中新加的方法是否漏写进接口。

该规范适用于公开接收者方法；构造函数、包级函数、私有辅助方法、纯数据结构和自动生成代码不纳入清单。已有接口名称如 `GenericCache[T]` 可以保留。


## 6. 配置与组件初始化


### 6.1 配置入口与客户端生命周期

- 应用启动入口统一读取 `app.yaml` 与各组件 YAML，组装为应用配置并在启动前完成校验；禁止中间件包在生产启动链路中自行定位、读取或解析配置文件。
- MySQL、Redis、Elasticsearch、消息队列和对象存储初始化函数接收已解析的组件配置，只负责创建和检查客户端。配置路径、默认值及必填校验集中在配置加载层。
- 各服务命令复用同一配置加载与校验入口；只初始化该进程实际需要的组件，但不能绕过统一解析和校验。
- 初始化完成后仍按当前项目约定由组件包复用全局客户端；退出时由应用生命周期负责关闭客户端。
- 测试可以使用专用配置或替身客户端，不得让生产配置读取逻辑成为业务代码的隐式依赖。
- 初始化必须在注册路由、启动消费者和接收业务请求之前完成；必须检查初始化错误，失败时停止启动并记录具体原因，避免拖到业务请求中才发现配置或客户端缺失。
- Service 构造函数默认从全局变量取得已初始化的客户端；Handler、Service 和每次请求的执行路径不得重复读取客户端配置或创建中间件客户端。客户端未初始化时明确返回错误，不允许在业务操作中临时创建作为兜底。
- 全局客户端在启动后保持稳定，业务调用复用其连接池和线程安全接口；请求级 Context、数据库事务和其他操作级状态仍由每次调用独立传入，不得放入共享客户端的全局状态。
- 测试可以显式注入替身客户端，以隔离外部服务；注入能力不改变生产环境的启动初始化方式。需要关闭的客户端由应用退出流程统一回收，不能在单次业务请求结束时关闭共享客户端。

```go
// main.go：统一加载并校验配置，再创建客户端。
cfg, err := util.LoadConfig()
if err != nil { return err }
if err := storage.InitImageStoreWithConfig(cfg.ImageStorage); err != nil {
    log.Fatal(err)
}

// Service 构造函数：复用全局客户端，支持测试注入。
func NewImageService(stores ...*storage.ImageStore) *ImageService {
    store := storage.ImageStoreClient
    if len(stores) > 0 && stores[0] != nil {
        store = stores[0]
    }
    service := &ImageService{store: store}
    return service
}

imageService := product_server.NewImageService()
```

MQ 同样只接收外部已加载的配置结构体，不提供配置路径、YAML 解析或自行读取文件的兜底；测试也由测试入口加载后传入。


## 7. 测试规范


### 7.1 AI 测试连接权限

- AI 在执行项目测试前，必须先检查测试入口、TestMain 和初始化逻辑，梳理本次测试需要的全部连接权限，包括 MySQL、Redis、Elasticsearch、RocketMQ、对象存储、Docker，以及测试 HTTP 服务需要的本机端口监听权限等。
- 首次运行测试前，一次性向用户说明所需服务、连接目标、测试范围及数据写入和清理方式，并统一申请本次测试所需的全部连接权限；不得先在受限环境中运行，等连接失败后再逐项申请。
- 使用执行工具的权限申请机制落实授权。已有明确授权覆盖本次测试且权限仍有效时直接复用，不重复询问；新增依赖或超出原授权范围时，运行相关测试前补充申请。
- 连接权限仅覆盖已说明的测试用途，不等于授权执行迁移、重置数据或其他破坏性操作。完全不需要外部连接或端口监听的隔离测试无需申请连接权限。


### 7.2 验证真实业务行为

- 测试应优先调用实际使用的业务入口，例如 Service 方法、HTTP Handler 或消息消费 Handler，验证真实代码的行为。
- 不要在测试中重新实现业务流程、计算公式或校验规则，再用测试自己的实现证明结果正确。
- 优先断言用户可观察的业务结果和关键副作用，例如返回值、业务错误、数据库变化、库存扣减、事务回滚、消息处理结果及失败重试；避免只验证内部函数调用或代码结构。
- 外部依赖可以使用测试替身或隔离的测试环境，但应保留被测业务流程；不要把需要验证的核心业务方法本身替换掉。
- 对本机已运行、项目已有测试配置且本次验证目标包含其读写行为的依赖（例如 MySQL、Redis），在测试初始化中使用组件已有的配置初始化方法和业务封装 API（例如 Redis 客户端初始化方法、`NewProductCache`），让真实业务入口沿用生产中的组件逻辑；不要在测试里另写连接、序列化或缓存逻辑，也不要用 fake/mock 替代本应验证的组件后把测试当作集成验证。
- 集成测试应在测试辅助初始化中连接所需依赖，检查初始化和连接错误，并清理测试写入的数据、回滚事务及恢复全局状态；测试应断言真实持久化结果，而不只是记录某个方法曾被调用。替身仅用于明确隔离外部系统的单元测试，不能替代真实依赖的集成覆盖。
- 辅助函数可以有针对性的单元测试，但不能替代关键业务入口的测试。涉及事务、数据一致性或多个组件协作时，应补充相应的集成测试。
- 新增和修改测试应遵守上述原则，并沿用第 5.3 节的结构体入参组织规范。


### 7.3 测试目录与运行入口

- 手写测试与业务实现分目录存放，使用对应业务目录下的 `test/` 文件夹，例如 `rocketmq/handler/test/`；不要将 `*_test.go` 与业务实现混放。迁移时同步测试命令和脚本路径。
- 独立测试包通过公开业务入口或已注册的 Handler 验证行为；不为测试私有方法添加公开导出桥接，也不复制业务实现。测试消息结构及环境辅助统一放在 `test/` 内。

- 订单、商品、用户 Service 测试分别放在 `server/order_server/test/`、`server/product_server/test/`、`server/user_server/test/`；`db/` 层测试放在 `db/test/`，避免测试文件与业务实现混放。
- 各业务的 `test/` 子目录是独立 Go 测试包。日志测试仍放在 `server/test/log/`，共用初始化和测试数据辅助函数放在 `server/test/support/`；每个业务包单独管理依赖与资源回收，不能依赖其他包的 TestMain。
- 本项目其余跨业务测试放在 `server/test/testdata/`。Go 的 `./...` 默认跳过名为 `testdata` 的目录，运行全部服务层测试时必须显式增加该包路径；统一脚本必须覆盖它，避免测试迁移后漏跑。
- 测试按业务拆分为独立文件，例如 `user_test.go`、`product_test.go`、`order_test.go`、`order_item_test.go`；内容较多时可增加表达测试主题的后缀，例如 `order_validation_test.go`。
- 测试文件名应能对应到被测业务，避免把用户、商品、订单等不同业务的测试集中到一个泛化的测试文件中。
- 共用的测试数据构造和环境辅助代码可放在对应测试目录的 `helpers_test.go` 等辅助文件中；辅助代码不应承载或复制业务逻辑。
- 测试目录通过导入业务包调用公开业务入口，不为访问私有实现而把测试移回业务目录。

```text
server/
  user_server/
    user_server.go
    test/user_test.go
  product_server/
    product_server.go
    test/product_test.go
  order_server/
    order_server.go
    test/order_test.go
  log_server.go
  test/
    log/log_test.go
    testdata/business_error_test.go
    support/validation_helpers.go

db/
  user_db.go
  product_db.go
  order_db.go
  order_item_db.go
  test/
    user_test.go
    product_test.go
    order_test.go
    order_item_test.go
```


## 8. 脚本与运行管理


### 8.1 脚本职责与服务验证

项目应提供统一脚本处理：

- 构建
- 启动
- 重启
- 依赖检查
- 数据初始化
- 测试

长期运行服务启动后必须验证真实进程和监听端口，不能只根据“启动命令返回成功”判断服务正常。脚本中的地址和端口应该集中定义，避免散落在多个命令中。

#### 本机长期服务的启动方式

- 启动需要持续运行的本机服务时，优先在用户自己的终端（通常是 iTerm2）前台执行启动命令，并让服务进程保持在该终端会话中。
- 不要从 Codex 命令会话或 Codex 自己的 PTY 启动服务后，就声称服务会持续运行；命令会话结束时，后台子进程可能被清理。
- 如果无法操作用户的终端，应说明这一限制，并提供可直接复制到用户终端执行的准确命令；不要悄悄改用 Codex 后台进程或 PTY。
- 启动后再独立检查进程和监听端口。启动脚本的成功提示不能代替运行状态验证。
