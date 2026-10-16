-- 新增业务更新时间。历史数据以各记录当前创建时间作为初始更新时间。
-- 订单明细同时保存商品名称快照；历史行从商品表回填。
ALTER TABLE `order_item` ADD COLUMN `product_name` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '下单时商品名称快照';
UPDATE `order_item` i LEFT JOIN `product` p ON p.id = i.product_id
SET i.product_name = COALESCE(p.name, '商品已删除') WHERE i.product_name = '';

ALTER TABLE `user` ADD COLUMN update_time DATETIME(3) NULL COMMENT '更新时间';
UPDATE `user` SET update_time = create_time WHERE update_time IS NULL;
ALTER TABLE `user` MODIFY COLUMN update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间';

ALTER TABLE `product` ADD COLUMN update_time DATETIME(3) NULL COMMENT '更新时间';
UPDATE `product` SET update_time = create_time WHERE update_time IS NULL;
ALTER TABLE `product` MODIFY COLUMN update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间';

ALTER TABLE `order` ADD COLUMN update_time DATETIME(3) NULL COMMENT '更新时间';
UPDATE `order` SET update_time = create_time WHERE update_time IS NULL;
ALTER TABLE `order` MODIFY COLUMN update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间';

ALTER TABLE `order_item` ADD COLUMN update_time DATETIME(3) NULL COMMENT '更新时间';
UPDATE `order_item` SET update_time = CURRENT_TIMESTAMP(3) WHERE update_time IS NULL;
ALTER TABLE `order_item` MODIFY COLUMN update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间';
