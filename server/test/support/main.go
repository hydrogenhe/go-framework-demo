package support

import (
	"fmt"
	"os"
	"testing"

	"go-framework-demo/es"
	"go-framework-demo/logger"
	"go-framework-demo/mysql"
	"go-framework-demo/redis"
	"go-framework-demo/rocketmq"
	mqhandler "go-framework-demo/rocketmq/handler"
	"go-framework-demo/util"
)

// Run 为业务测试统一初始化依赖，只启动指定的消费组，并在测试结束或初始化失败时回收 MQ。
func Run(m *testing.M, consumerGroups ...string) {
	logger.Init()
	code := run(m, consumerGroups)
	logger.Sync()
	os.Exit(code)
}

func run(m *testing.M, consumerGroups []string) int {
	if os.Getenv("DEMO_TEST_DATABASE_ONLY") == "1" {
		return m.Run()
	}
	mysql.InitMySQL()
	redis.Init()
	if err := es.InitES(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化 ES 失败: %v\n", err)
		return 1
	}
	// 用户测试自行写入 ES，不需要消费者；商品、订单测试分别启动本业务的 CDC 消费组。
	if len(consumerGroups) > 0 {
		defer func() {
			if err := rocketmq.ShutdownAllRocketmq(); err != nil {
				fmt.Fprintf(os.Stderr, "关闭 RocketMQ 失败: %v\n", err)
			}
		}()
		for group, callback := range mqhandler.Handlers() {
			rocketmq.RegisterConsumerGroupHandler(group, callback)
		}
		config, err := util.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取测试配置失败: %v\n", err)
			return 1
		}
		if err := rocketmq.InitRocketmqForConsumerGroups(config.RocketMQ, consumerGroups...); err != nil {
			fmt.Fprintf(os.Stderr, "初始化 RocketMQ 失败: %v\n", err)
			return 1
		}
	}
	return m.Run()
}
