// internal/rocketmq/init.go
package rocketmq

import (
	"errors"
	"log"
)

// InitAllRocketmq 统一初始化所有RocketMQ组件（配置加载+生产者+消费者）
// 入参：可选配置文件路径，不传则使用默认路径（config/rocketmq.yaml）
func InitAllRocketmq(configPath ...string) error {
	log.Println("开始初始化RocketMQ组件...")

	// 1. 加载配置文件
	log.Println("步骤1/3：加载RocketMQ配置文件")
	var loadErr error
	if len(configPath) > 0 {
		loadErr = LoadConfig(configPath[0])
	} else {
		loadErr = LoadConfig()
	}
	if loadErr != nil {
		return errors.Join(errors.New("加载配置文件失败"), loadErr)
	}

	// 2. 初始化生产者
	log.Println("步骤2/3：初始化RocketMQ生产者")
	producerErr := InitAllProducers()
	if producerErr != nil {
		return errors.Join(errors.New("初始化生产者失败"), producerErr)
	}

	// 3. 初始化消费者
	log.Println("步骤3/3：初始化RocketMQ消费者")
	consumerErr := InitAllConsumers()
	if consumerErr != nil {
		return errors.Join(errors.New("初始化消费者失败"), consumerErr)
	}

	log.Println("RocketMQ所有组件初始化完成✅")
	return nil
}

// ShutdownAllRocketmq 统一关闭所有RocketMQ组件（生产者+消费者）
func ShutdownAllRocketmq() error {
	log.Println("开始关闭RocketMQ组件...")
	var combinedErr error

	// 1. 关闭消费者
	log.Println("步骤1/2：关闭RocketMQ消费者")
	consumerErr := ShutdownAllConsumers()
	if consumerErr != nil {
		combinedErr = errors.Join(combinedErr, consumerErr)
		log.Printf("关闭消费者失败: %v", consumerErr)
	} else {
		log.Println("消费者关闭成功")
	}

	// 2. 关闭生产者
	log.Println("步骤2/2：关闭RocketMQ生产者")
	producerErr := ShutdownAllProducers()
	if producerErr != nil {
		combinedErr = errors.Join(combinedErr, producerErr)
		log.Printf("关闭生产者失败: %v", producerErr)
	} else {
		log.Println("生产者关闭成功")
	}

	if combinedErr != nil {
		return errors.Join(errors.New("部分RocketMQ组件关闭失败"), combinedErr)
	}

	log.Println("RocketMQ所有组件关闭完成✅")
	return nil
}
