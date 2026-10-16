// internal/rocketmq/producer.go
package rocketmq

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/logger"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// BizProducer 生产者实例
type BizProducer struct {
	Instance rocketmq.Producer      // RocketMQ生产者实例
	Config   *RuntimeProducerConfig // 直接关联运行时配置
}

var (
	producerMap  = make(map[string]*BizProducer)
	producerOnce sync.Once
	producerErr  error
)

// InitAllProducers 初始化所有生产者（直接用RuntimeProducerConfig）
func InitAllProducers() error {
	producerOnce.Do(func() {
		// 遍历运行时配置，直接构建生产者
		for bizName, config := range GlobalProducerConfigs {
			// 直接用配置结构体的字段构建生产者
			producerOptions := []producer.Option{
				producer.WithGroupName(config.ProducerGroup),
				producer.WithNsResolver(primitive.NewPassthroughResolver([]string{GlobalCommonConfig.NameSrvAddr})),
				producer.WithRetry(config.RetryTimes),
				producer.WithSendMsgTimeout(time.Duration(config.SendTimeout) * time.Millisecond),
			}
			// 订单顺序消息按分片键固定选择队列；默认轮询选择器不会使用该键。
			if bizName == "order" {
				producerOptions = append(producerOptions, producer.WithQueueSelector(producer.NewHashQueueSelector()))
			}
			p, err := rocketmq.NewProducer(
				producerOptions...,
			)
			if err != nil {
				producerErr = fmt.Errorf("业务[%s]创建生产者失败: %w", bizName, err)
				logger.Sugar.Infof("[ERROR] %v", producerErr)
				return
			}

			// 启动生产者
			if err = p.Start(); err != nil {
				producerErr = fmt.Errorf("业务[%s]启动生产者失败: %w", bizName, err)
				logger.Sugar.Infof("[ERROR] %v", producerErr)
				return
			}
			// 发送空消息（触发Topic自动创建，失败不影响，只要触发路由注册即可）
			//msg := primitive.NewMessage(config.Topic, []byte("init_topic"))
			//_, err = p.SendSync(context.Background(), msg)
			//if err != nil {
			//	producerErr = fmt.Errorf("业务[%s]发送测试信息失败: %w", bizName, err)
			//	logger.Sugar.Infof("[ERROR] %v", producerErr)
			//	return
			//}

			// 保存生产者实例（关联配置）
			producerMap[bizName] = &BizProducer{
				Instance: p,
				Config:   config,
			}
			logger.Sugar.Infof("业务[%s]生产者初始化成功（QPS:%d, 超时:%dms）", bizName, config.MaxSendQPS, config.SendTimeout)
		}
	})
	return producerErr
}

// SendDelayMsg 发送带自定义属性的RocketMQ消息（含延时、标签等）
func SendDelayMsg(ctx context.Context, bizName string, msg *primitive.Message) (*primitive.SendResult, error) {
	if msg == nil {
		return nil, errors.New("RocketMQ消息不能为空")
	}
	ctx = ensureLogID(ctx)
	// 1. 获取你原有初始化的生产者实例（复用你已有的生产者逻辑）
	producer, ok := producerMap[bizName]
	if !ok {
		return nil, fmt.Errorf("未找到业务[%s]生产者", bizName)
	}
	if logID := logger.LogID(ctx); logID != "" {
		msg.WithProperty("log_id", logID)
	}
	// 先获取令牌（带超时）
	limiter := producer.Config.Limiter
	if limiter != nil {
		// 2.1 创建带超时的子上下文（避免无限阻塞）
		timeout := time.Duration(producer.Config.SendTimeout) * time.Millisecond
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel() // 确保上下文资源释放

		// 2.2 获取1个令牌（第二个参数传1，修复参数错误）
		if err := limiter.WaitN(timeoutCtx, 1); err != nil {
			// 2.3 返回值修正：第一个返回值为nil，第二个为error
			return nil, fmt.Errorf("业务[%s]获取发送令牌超时（限流）: %w", bizName, err)
		}
	}

	// 发送消息
	res, err := producer.Instance.SendSync(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("业务[%s]发送消息失败: %w", bizName, err)
	}

	logger.Info(ctx, "RocketMQ消息发送成功", "biz_name", bizName, "msg_id", res.MsgID)
	return res, nil
}

// SendMsg 发送消息（直接用配置）
func SendMsg(ctx context.Context, bizName string, body []byte) (*primitive.SendResult, error) {
	ctx = ensureLogID(ctx)
	producer, ok := producerMap[bizName]
	if !ok {
		return nil, fmt.Errorf("未找到业务[%s]生产者", bizName)
	}
	// 先获取令牌（带超时）
	limiter := producer.Config.Limiter
	if limiter != nil {
		// 2.1 创建带超时的子上下文（避免无限阻塞）
		timeout := time.Duration(producer.Config.SendTimeout) * time.Millisecond
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel() // 确保上下文资源释放

		// 2.2 获取1个令牌（第二个参数传1，修复参数错误）
		if err := limiter.WaitN(timeoutCtx, 1); err != nil {
			// 2.3 返回值修正：第一个返回值为nil，第二个为error
			return nil, fmt.Errorf("业务[%s]获取发送令牌超时（限流）: %w", bizName, err)
		}
	}

	// 构建消息（直接用配置的Topic）
	msg := &primitive.Message{
		Topic: producer.Config.Topic,
		Body:  body,
	}
	if logID := logger.LogID(ctx); logID != "" {
		msg.WithProperty("log_id", logID)
	}

	// 发送消息
	res, err := producer.Instance.SendSync(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("业务[%s]发送消息失败: %w", bizName, err)
	}

	logger.Info(ctx, "RocketMQ消息发送成功", "biz_name", bizName, "msg_id", res.MsgID)
	return res, nil
}

func ensureLogID(ctx context.Context) context.Context {
	if logger.LogID(ctx) == "" {
		ctx = logger.WithLogID(ctx, logger.NewLogID())
	}
	return ctx
}

// ShutdownAllProducers 关闭所有生产者
func ShutdownAllProducers() error {
	var err error
	for bizName, p := range producerMap {
		if e := p.Instance.Shutdown(); e != nil {
			logger.Sugar.Infof("业务[%s]关闭生产者失败: %v", bizName, e)
			err = errors.Join(err, e)
		} else {
			logger.Sugar.Infof("业务[%s]生产者已关闭", bizName)
		}
	}
	return err
}

func UserProducerSendMsg(ctx context.Context, body []byte) (*primitive.SendResult, error) {
	return SendMsg(ctx, "user", body)
}

// OrderProducerSendMsg 发送订单业务消息。
func OrderProducerSendMsg(ctx context.Context, body []byte) (*primitive.SendResult, error) {
	return SendMsg(ctx, "order", body)
}

// OrderProducerSendDelayMsg 发送订单延迟消息。传入的 level 需匹配 Broker 配置的延迟级别。
func OrderProducerSendDelayMsg(ctx context.Context, body []byte, level int) (*primitive.SendResult, error) {
	producer, ok := producerMap["order"]
	if !ok {
		return nil, fmt.Errorf("未找到订单生产者")
	}
	msg := primitive.NewMessage(producer.Config.Topic, body).WithDelayTimeLevel(level)
	return SendDelayMsg(ctx, "order", msg)
}
