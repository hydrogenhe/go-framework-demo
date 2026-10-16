-- 商品库存继续保存在 product；商品名称、价格、图片的历史版本保存在 product_version。
ALTER TABLE `product`
    ADD COLUMN `current_version` INT NOT NULL DEFAULT 1 COMMENT '当前商品资料版本';

CREATE TABLE `product_version` (
    `product_id` BIGINT NOT NULL COMMENT '稳定商品ID',
    `version` INT NOT NULL COMMENT '商品资料版本号',
    `name` VARCHAR(255) NOT NULL COMMENT '商品名称',
    `price` BIGINT NOT NULL COMMENT '商品价格（单位：分）',
    `image` TEXT NOT NULL COMMENT '商品图片地址',
    `create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '版本创建时间',
    PRIMARY KEY (`product_id`, `version`),
    CONSTRAINT `fk_product_version_product`
        FOREIGN KEY (`product_id`) REFERENCES `product` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='不可变商品资料版本';

-- 既有商品以迁移时的资料作为版本1。既有订单无法可靠推断商品版本，保持0并继续展示订单项内的历史快照。
INSERT INTO `product_version` (`product_id`, `version`, `name`, `price`, `image`, `create_time`)
SELECT `id`, 1, `name`, `price`, COALESCE(`image`, ''), `create_time`
FROM `product`;

ALTER TABLE `order_item`
    ADD COLUMN `product_version` INT NOT NULL DEFAULT 0 COMMENT '下单时商品资料版本，0表示历史订单未记录';
