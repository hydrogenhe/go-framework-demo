-- 用创建时间为历史订单回填固定的10分钟支付期限，并为到期订单批量扫描建立复合索引。
ALTER TABLE `order` ADD COLUMN `expire_time` DATETIME(3) NULL COMMENT '支付截止时间';
UPDATE `order` SET `expire_time` = DATE_ADD(`create_time`, INTERVAL 10 MINUTE) WHERE `expire_time` IS NULL;
ALTER TABLE `order` MODIFY COLUMN `expire_time` DATETIME(3) NOT NULL COMMENT '支付截止时间';
ALTER TABLE `order` MODIFY COLUMN `status` INT NOT NULL DEFAULT 1 COMMENT '订单状态：1待支付，2已支付，3已完成，4已取消';
CREATE INDEX `idx_order_status_expire_time` ON `order` (`status`, `expire_time`);

