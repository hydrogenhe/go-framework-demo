-- 订单明细保存下单时商品图片快照，无法还原原图的历史订单留空。
ALTER TABLE `order_item` ADD COLUMN `image` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '下单时商品图片快照';
