# RocketMQ 终端查看

从项目根目录执行，所有脚本只读查询，不启动服务或修改消息。

| Topic | 命令 | 消费者组 |
|---|---|---|
| 商品 CDC | `bash components_sh/mq/product_cdc_topic.sh` | `product_es_consumer_group` |
| 用户 CDC | `bash components_sh/mq/user_cdc_topic.sh` | `user_es_consumer_group` |
| 订单 CDC | `bash components_sh/mq/order_cdc_topic.sh` | `order_es_consumer_group` |
| 用户业务 | `bash components_sh/mq/user_topic.sh` | info、notify 两个组分别显示 |
| 订单业务 | `bash components_sh/mq/order_topic.sh` | `order_consumer_group` |
| 商品业务 | `bash components_sh/mq/product_topic.sh` | 无消费者，显示存储位点 |

消费进度入口使用同一份 `consumer_progress.sh`，统一表格样式。每个组的业务 Topic 与 `%RETRY%<消费者组>` 分开显示完整名称，队列编号只在各 Topic/Broker 内有效。多个组的积压不能相加当作消息总数。

自定义组：`bash components_sh/mq/consumer_progress.sh <消费者组>`。
原 `bash components_sh/mq_progress.sh` 保留兼容入口。

环境变量：`MQ_NAMESRV` 指定 NameServer，`MQADMIN` 指定 mqadmin 路径，`NO_COLOR=1` 禁用颜色。
`--` 表示未报告时间。没有消费者时不展示待消费或处理中数量。
