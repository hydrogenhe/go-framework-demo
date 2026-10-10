package main

import (
	"go-framework-demo/mysql"
	"go-framework-demo/redis"
	"go-framework-demo/rocketmq"
	"go-framework-demo/server"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	redis.Init(redis.WithAddr("localhost:6379"))
	mysql.InitMySQL()
	server.TestUser()
	log.Println("🚀 程序启动，开始初始化RocketMQ...")
	if err := rocketmq.InitAllRocketmq(); err != nil { // 不传配置路径则用默认路径
		// 初始化失败直接退出程序
		log.Fatalf("❌ RocketMQ初始化失败: %v", err)
	}

	// ======================== 2. 优雅退出处理 ========================
	// 创建退出信号通道，监听常见的终止信号
	sigChan := make(chan os.Signal, 1)
	// 监听：Ctrl+C (SIGINT)、kill (SIGTERM)、退出 (SIGQUIT)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 定义优雅退出函数
	shutdown := func() {
		log.Println("\n🛑 收到退出信号，开始优雅退出...")

		// 1. 等待1秒，确保正在处理的消息完成（可选，根据业务调整）
		log.Println("⌛ 等待1秒，处理剩余消息...")
		time.Sleep(1 * time.Second)

		// 2. 统一关闭RocketMQ组件
		if err := rocketmq.ShutdownAllRocketmq(); err != nil {
			log.Printf("❌ RocketMQ关闭失败: %v", err)
		}

		// 3. 退出程序
		log.Println("✅ 程序优雅退出完成")
		os.Exit(0)
	}

	// 注册退出处理（两种触发方式：信号监听 + defer兜底）
	defer shutdown()
	go func() {
		<-sigChan // 阻塞等待退出信号
		shutdown()
	}()

	// ======================== 3. 程序常驻运行 ========================
	// 方式1：无限阻塞（最简单，推荐生产环境使用）
	log.Println("✅ 程序初始化完成，进入常驻运行模式（按 Ctrl+C 退出）")
	select {} // 永久阻塞，不会退出

	// 方式2：定时打印心跳日志（可选，便于监控程序是否存活）
	// heartbeatTicker := time.NewTicker(30 * time.Second)
	// defer heartbeatTicker.Stop()
	// log.Println("✅ 程序初始化完成，进入常驻运行模式（心跳间隔30秒）")
	// for {
	// 	select {
	// 	case <-heartbeatTicker.C:
	// 		log.Println("❤️ 程序运行正常，心跳检测通过")
	// 	}
	// }
}
