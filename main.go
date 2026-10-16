package main

import (
	"context"
	"fmt"
	"go-framework-demo/es"
	"go-framework-demo/http_handler"
	"go-framework-demo/logger"
	"go-framework-demo/mysql"
	"go-framework-demo/redis"
	"go-framework-demo/rocketmq"
	mqhandler "go-framework-demo/rocketmq/handler"
	"go-framework-demo/scheduler/order_expiration"
	"go-framework-demo/scheduler/reconciliation"
	"go-framework-demo/storage"
	"go-framework-demo/util"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	appConfig, err := util.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logger.InitWithDirectory(appConfig.Logs.Directory)
	defer logger.Sync()
	if err := redis.InitWithConfig(appConfig.Redis); err != nil {
		logger.Log.Fatal("Redis初始化失败", zap.Error(err))
	}
	if err := mysql.InitMySQL(appConfig.MySQL); err != nil {
		logger.Log.Fatal("MySQL初始化失败", zap.Error(err))
	}
	err = es.InitES(appConfig.ES)
	if err != nil {
		logger.Log.Fatal("ES初始化失败", zap.Error(err))
	}
	if err := storage.InitImageStoreWithConfig(appConfig.ImageStorage); err != nil {
		logger.Log.Fatal("图片存储初始化失败", zap.Error(err))
	}

	logger.Log.Info("程序启动，开始初始化RocketMQ")
	for group, callback := range mqhandler.Handlers() {
		rocketmq.RegisterConsumerGroupHandler(group, callback)
	}
	if err := rocketmq.InitAllRocketmq(appConfig.RocketMQ); err != nil {
		// 初始化失败直接退出程序
		logger.Log.Fatal("RocketMQ初始化失败", zap.Error(err))
	}

	// ES 对账启动时立即执行，之后按 app.yaml 中配置的间隔调度。
	reconcileCtx, cancelReconciliation := context.WithCancel(context.Background())
	go func() {
		reconciliation.RunScheduler(reconcileCtx, time.Duration(appConfig.Reconciliation.IntervalHours)*time.Hour, mysql.GetDB())
	}()
	expirationCtx, cancelOrderExpiration := context.WithCancel(context.Background())
	go order_expiration.RunScheduler(expirationCtx, mysql.GetDB(), time.Duration(appConfig.OrderExpiration.IntervalSeconds)*time.Second)

	// ======================== 2. HTTP 服务初始化 ========================
	httpServer := http_handler.InitHTTP(appConfig.HTTP.Address)
	go func() {
		logger.Log.Info("HTTP服务启动", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("HTTP服务异常退出", zap.Error(err))
		}
	}()

	// ======================== 3. 优雅退出处理 ========================
	// 创建退出信号通道，监听常见的终止信号
	sigChan := make(chan os.Signal, 1)
	// 监听：Ctrl+C (SIGINT)、kill (SIGTERM)、退出 (SIGQUIT)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 定义优雅退出函数
	shutdown := func() {
		logger.Log.Info("收到退出信号，开始优雅退出")
		cancelReconciliation()
		cancelOrderExpiration()

		// 1. 等待1秒，确保正在处理的消息完成（可选，根据业务调整）
		logger.Log.Info("等待1秒，处理剩余消息")
		time.Sleep(1 * time.Second)

		// 2. 统一关闭RocketMQ组件
		if err := rocketmq.ShutdownAllRocketmq(); err != nil {
			logger.Log.Error("RocketMQ关闭失败", zap.Error(err))
		}

		// 3. 退出程序
		logger.Log.Info("程序优雅退出完成")
		os.Exit(0)
	}

	// 注册退出处理（两种触发方式：信号监听 + defer兜底）
	defer shutdown()
	go func() {
		<-sigChan // 阻塞等待退出信号
		shutdown()
	}()

	// ======================== 4. 程序常驻运行 ========================
	// 方式1：无限阻塞（最简单，推荐生产环境使用）
	logger.Log.Info("程序初始化完成，进入常驻运行模式")
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
