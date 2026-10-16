// internal/rocketmq/init.go
package rocketmq

import (
	"errors"
	"go-framework-demo/logger"
)

// InitAllRocketmq 接收调用方已加载的配置，初始化基础组件；须事先注册消费回调。
func InitAllRocketmq(config RocketMQConfig) error {
	return initRocketmq(config, nil)
}

// InitRocketmqForConsumerGroups 使用外部配置，仅启动指定消费组及生产者。
func InitRocketmqForConsumerGroups(config RocketMQConfig, consumerGroups ...string) error {
	return initRocketmq(config, consumerGroups)
}

func initRocketmq(config RocketMQConfig, consumerGroups []string) error {
	logger.Sugar.Info("开始初始化RocketMQ组件...")
	if err := LoadRuntimeConfig(config); err != nil {
		return errors.Join(errors.New("构造RocketMQ运行时配置失败"), err)
	}

	// 2. 初始化生产者
	logger.Sugar.Info("步骤2/3：初始化RocketMQ生产者")
	producerErr := InitAllProducers()
	if producerErr != nil {
		return errors.Join(errors.New("初始化生产者失败"), producerErr)
	}

	// 3. 初始化消费者
	logger.Sugar.Info("步骤3/3：初始化RocketMQ消费者")
	consumerErr := InitConsumers(consumerGroups...)
	if consumerErr != nil {
		return errors.Join(errors.New("初始化消费者失败"), consumerErr)
	}

	logger.Sugar.Info("RocketMQ所有组件初始化完成✅")
	return nil
}

// ShutdownAllRocketmq 统一关闭所有RocketMQ组件（生产者+消费者）
func ShutdownAllRocketmq() error {
	logger.Sugar.Info("开始关闭RocketMQ组件...")
	var combinedErr error

	// 1. 关闭消费者
	logger.Sugar.Info("步骤1/2：关闭RocketMQ消费者")
	consumerErr := ShutdownAllConsumers()
	if consumerErr != nil {
		combinedErr = errors.Join(combinedErr, consumerErr)
		logger.Sugar.Infof("关闭消费者失败: %v", consumerErr)
	} else {
		logger.Sugar.Info("消费者关闭成功")
	}

	// 2. 关闭生产者
	logger.Sugar.Info("步骤2/2：关闭RocketMQ生产者")
	producerErr := ShutdownAllProducers()
	if producerErr != nil {
		combinedErr = errors.Join(combinedErr, producerErr)
		logger.Sugar.Infof("关闭生产者失败: %v", producerErr)
	} else {
		logger.Sugar.Info("生产者关闭成功")
	}

	if combinedErr != nil {
		return errors.Join(errors.New("部分RocketMQ组件关闭失败"), combinedErr)
	}

	logger.Sugar.Info("RocketMQ所有组件关闭完成✅")
	return nil
}
