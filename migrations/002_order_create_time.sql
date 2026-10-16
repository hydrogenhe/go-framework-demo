-- 启动新后端前执行；需要重复运行时，使用以下迁移工具：
-- go run ./cmd/init-order-time -apply
-- 旧表未保存订单创建时间，现有记录统一使用迁移时间。
ALTER TABLE `order` ADD COLUMN create_time DATETIME(3) NULL COMMENT '创建时间';
UPDATE `order` SET create_time = CURRENT_TIMESTAMP(3) WHERE create_time IS NULL;
ALTER TABLE `order` MODIFY COLUMN create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间';
