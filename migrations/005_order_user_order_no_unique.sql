-- Enforce one order number per user for concurrent-safe order retries.
ALTER TABLE `order`
    ADD UNIQUE INDEX `uk_order_user_order_no` (`user_id`, `order_no`);
