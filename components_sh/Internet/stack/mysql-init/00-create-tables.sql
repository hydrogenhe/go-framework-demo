-- Fresh-install schema for the current application model.
-- Safe to re-run: existing tables and data are left untouched.
CREATE DATABASE IF NOT EXISTS `demo_db`
  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `demo_db`;

CREATE TABLE IF NOT EXISTS `user` (
  `id` BIGINT NOT NULL,
  `name` VARCHAR(255) NOT NULL COMMENT '姓名',
  `age` INT NULL COMMENT '年龄',
  `email` VARCHAR(255) NOT NULL COMMENT '邮箱',
  `create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `password` VARCHAR(255) NOT NULL COMMENT '密码MD5',
  `is_del` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '软删除：0正常，1已删除',
  `update_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户';

CREATE TABLE IF NOT EXISTS `product` (
  `id` BIGINT NOT NULL,
  `name` VARCHAR(255) NOT NULL COMMENT '商品名称',
  `price` BIGINT NOT NULL COMMENT '商品价格（单位：分）',
  `stock` INT NOT NULL DEFAULT 0 COMMENT '库存',
  `image` TEXT NOT NULL,
  `create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `is_del` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '软删除：0正常，1已删除',
  `update_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `current_version` INT NOT NULL DEFAULT 1 COMMENT '当前商品资料版本',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品';

CREATE TABLE IF NOT EXISTS `order` (
  `id` BIGINT NOT NULL,
  `order_no` VARCHAR(255) NOT NULL COMMENT '订单号',
  `user_id` BIGINT NOT NULL,
  `amount` BIGINT NOT NULL COMMENT '订单金额（单位：分）',
  `status` INT NOT NULL DEFAULT 1 COMMENT '订单状态：1待支付，2已支付，3已完成，4已取消',
  `create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `expire_time` DATETIME(3) NOT NULL COMMENT '支付截止时间',
  `is_del` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '软删除：0正常，1已删除',
  `update_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_user_order_no` (`user_id`, `order_no`),
  KEY `idx_order_status_expire_time` (`status`, `expire_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单';

CREATE TABLE IF NOT EXISTS `order_item` (
  `id` BIGINT NOT NULL,
  `order_id` BIGINT NOT NULL,
  `product_id` BIGINT NOT NULL,
  `quantity` INT NOT NULL,
  `unit_price_fen` BIGINT NOT NULL COMMENT '下单时单价（单位：分）',
  `status` INT NOT NULL DEFAULT 1 COMMENT '明细状态：1待发货，2已发货',
  `image` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '下单时商品图片快照',
  `is_del` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '软删除：0正常，1已删除',
  `product_name` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '下单时商品名称快照',
  `update_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  `product_version` INT NOT NULL DEFAULT 0 COMMENT '下单时商品资料版本，0表示历史订单未记录',
  PRIMARY KEY (`id`),
  KEY `idx_order_item_order_id` (`order_id`),
  KEY `idx_order_item_product_id` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单明细';

CREATE TABLE IF NOT EXISTS `product_version` (
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
