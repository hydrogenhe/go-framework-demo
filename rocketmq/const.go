package rocketmq

const (
	// RocketMQDelayLevel1 延时级别1 → 1秒
	RocketMQDelayLevel1 = 1
	// RocketMQDelayLevel2 延时级别2 → 5秒
	RocketMQDelayLevel2 = 2
	// RocketMQDelayLevel3 延时级别3 → 10秒
	RocketMQDelayLevel3 = 3
	// RocketMQDelayLevel4 延时级别4 → 30秒
	RocketMQDelayLevel4 = 4
	// RocketMQDelayLevel5 延时级别5 → 1分钟
	RocketMQDelayLevel5 = 5
	// RocketMQDelayLevel6 延时级别6 → 2分钟
	RocketMQDelayLevel6 = 6
	// RocketMQDelayLevel7 延时级别7 → 3分钟
	RocketMQDelayLevel7 = 7
	// RocketMQDelayLevel8 延时级别8 → 4分钟
	RocketMQDelayLevel8 = 8
	// RocketMQDelayLevel9 延时级别9 → 5分钟
	RocketMQDelayLevel9 = 9
	// RocketMQDelayLevel10 延时级别10 → 6分钟
	RocketMQDelayLevel10 = 10
	// RocketMQDelayLevel11 延时级别11 → 7分钟
	RocketMQDelayLevel11 = 11
	// RocketMQDelayLevel12 延时级别12 → 8分钟
	RocketMQDelayLevel12 = 12
	// RocketMQDelayLevel13 延时级别13 → 9分钟
	RocketMQDelayLevel13 = 13
	// RocketMQDelayLevel14 延时级别14 → 10分钟
	RocketMQDelayLevel14 = 14
	// RocketMQDelayLevel15 延时级别15 → 20分钟
	RocketMQDelayLevel15 = 15
	// RocketMQDelayLevel16 延时级别16 → 30分钟
	RocketMQDelayLevel16 = 16
	// RocketMQDelayLevel17 延时级别17 → 1小时
	RocketMQDelayLevel17 = 17
	// RocketMQDelayLevel18 延时级别18 → 2小时
	RocketMQDelayLevel18 = 18
	// OrderExpirationDelayLevel 保留默认10分钟级别；实际订单检查按目标时间动态选择级别。
	OrderExpirationDelayLevel = RocketMQDelayLevel14
)
