package message

import (
	"fmt"
	"time"
)

// ExpirationDelayLevel 返回大于等于所需时长的最近 Broker 默认延迟级别。
// 超过最大2小时明确返回错误，由数据库扫描兜底，避免静默发送提前消息。
func ExpirationDelayLevel(remaining time.Duration) (int, error) {
	delays := [...]time.Duration{
		time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second,
		time.Minute, 2 * time.Minute, 3 * time.Minute, 4 * time.Minute,
		5 * time.Minute, 6 * time.Minute, 7 * time.Minute, 8 * time.Minute,
		9 * time.Minute, 10 * time.Minute, 20 * time.Minute, 30 * time.Minute,
		time.Hour, 2 * time.Hour,
	}
	for index, delay := range delays {
		if remaining <= delay {
			return index + 1, nil
		}
	}
	return 0, fmt.Errorf("订单剩余支付时间 %s 超过 Broker 默认最大延迟2小时", remaining)
}
