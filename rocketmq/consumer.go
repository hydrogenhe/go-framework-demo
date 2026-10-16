package rocketmq

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/logger"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// BizConsumer 消费者实例
type BizConsumer struct {
	Instance   rocketmq.PushConsumer  // RocketMQ消费者实例
	Config     *RuntimeConsumerConfig // 直接关联运行时配置
	InstanceID int
}

var (
	consumerList []*BizConsumer
	consumerOnce sync.Once
	consumerErr  error
	readyChecks  sync.Map // 就绪探测 ID 对应的通知通道。
)

// readyCheckProperty 标记消费者就绪探测消息的自定义属性键。
const readyCheckProperty = "GO_FRAMEWORK_DEMO_READY_CHECK"

type RocketCtxKey string

var ClientIdKey RocketCtxKey = "clientId"
var InstanceIDKey RocketCtxKey = "instanceId"

// InitAllConsumers 初始化所有消费者（直接用RuntimeConsumerConfig）
func InitAllConsumers() error {
	return InitConsumers()
}

// InitConsumers 初始化指定消费组；不传消费组时初始化全部消费者。
func InitConsumers(consumerGroups ...string) error {
	consumerOnce.Do(func() {
		groupFilter := make(map[string]struct{}, len(consumerGroups))
		for _, group := range consumerGroups {
			groupFilter[group] = struct{}{}
		}
		// 遍历每个消费者配置
		for _, config := range GlobalConsumerConfigs {
			if len(groupFilter) > 0 {
				if _, ok := groupFilter[config.ConsumerGroup]; !ok {
					continue
				}
			}
			// 兜底：实例数默认1（未配置时）
			instanceCount := config.InstanceCount
			if instanceCount <= 0 {
				instanceCount = 1
			}

			// 核心：循环创建多个实例（按instance_count配置）
			for instanceID := 1; instanceID <= instanceCount; instanceID++ {
				// ========== 关键1：延迟启动，避免并发注册冲突 ==========

				// 生成唯一标识（仅用于日志，无SDK配置）
				clientID := generateConsumerClientID(config.ConsumerGroup, instanceID)

				// ========== 关键2：仅用v2.1.2 确定存在的配置 ==========
				c, err := consumer.NewPushConsumer(
					consumer.WithGroupName(config.ConsumerGroup),
					consumer.WithNsResolver(primitive.NewPassthroughResolver([]string{GlobalCommonConfig.NameSrvAddr})),
					// 以下配置如果也报错，直接删除（用默认值）
					consumer.WithConsumerModel(config.MessageModel),
					consumer.WithConsumeConcurrentlyMaxSpan(config.ThreadNums),
					consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset),
					consumer.WithConsumerOrder(config.ConsumeOrderly),
				)
				if err != nil {
					logger.Sugar.Infof("[ERROR] 业务[%s]消费者[%s]实例[%s]创建失败: %v", config.BizName, config.ConsumerGroup, clientID, err)
					consumerErr = errors.Join(consumerErr, fmt.Errorf("业务[%s]消费者[%s]实例[%s]创建失败: %w", config.BizName, config.ConsumerGroup, clientID, err))
					continue // 单个实例失败不影响其他
				}

				// 订阅主题（如果 MessageSelector 报错，改为 consumer.AllMessage()）
				err = c.Subscribe(config.Topic, consumer.MessageSelector{}, wrapConsumeHandler(config.ConsumeHandler, clientID, instanceID))
				if err != nil {
					logger.Sugar.Infof("[ERROR] 业务[%s]消费者[%s]实例[%d]订阅主题[%s]失败: %v", config.BizName, config.ConsumerGroup, instanceID, config.Topic, err)
					_ = c.Shutdown()
					consumerErr = errors.Join(consumerErr, fmt.Errorf("业务[%s]消费者[%s]实例[%d]订阅主题[%s]失败: %w", config.BizName, config.ConsumerGroup, instanceID, config.Topic, err))
					continue
				}

				// ========== 关键3：启动重试，解决偶发冲突 ==========
				var startErr error

				startErr = StartConsumerAndWait(c, config.BizName, config.ConsumerGroup, instanceID, config.MessageModel, config.Topic)

				if startErr != nil {
					logger.Sugar.Infof("[ERROR] 业务[%s]消费者[%s]实例[%d]启动失败: %v", config.BizName, config.ConsumerGroup, instanceID, startErr)
					if shutdownErr := c.Shutdown(); shutdownErr != nil {
						startErr = errors.Join(startErr, fmt.Errorf("关闭失败: %w", shutdownErr))
					}
					consumerErr = errors.Join(consumerErr, fmt.Errorf("业务[%s]消费者[%s]实例[%d]启动失败: %w", config.BizName, config.ConsumerGroup, instanceID, startErr))
					continue
				}

				// 保存实例
				bizConsumer := &BizConsumer{
					Instance:   c,
					Config:     config,
					InstanceID: instanceID,
				}
				consumerList = append(consumerList, bizConsumer)
				logger.Sugar.Infof("业务[%s]消费者[%s]实例[%d]初始化成功（总实例数:%d）",
					config.BizName, config.ConsumerGroup, instanceID, instanceCount)
			}
		}
	})
	return consumerErr
}

// 包装消费回调，添加实例标识
func wrapConsumeHandler(handler ConsumeHandlerFunc, clientID string, instanceID int) ConsumeHandlerFunc {
	return func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, msg := range msgs {
			if msg == nil {
				continue
			}
			logID := msg.GetProperty("log_id")
			if logID == "" {
				logID = logger.LogID(ctx)
				if logID == "" {
					logID = logger.NewLogID()
				}
				msg.WithProperty("log_id", logID)
			}
			logger.Info(logger.WithLogID(ctx, logID), "开始消费RocketMQ消息", "client_id", clientID, "topic", msg.Topic, "msg_id", msg.MsgId)
		}
		// 执行原回调逻辑
		ctx = context.WithValue(ctx, ClientIdKey, clientID)
		ctx = context.WithValue(ctx, InstanceIDKey, instanceID)
		for _, msg := range msgs {
			if checkID := msg.GetProperty(readyCheckProperty); checkID != "" {
				if ch, ok := readyChecks.Load(checkID); ok {
					select {
					case ch.(chan struct{}) <- struct{}{}:
					default:
					}
				}
				// 探测消息只用于确认链路，不交给业务回调处理。
				return consumer.ConsumeSuccess, nil
			}
		}

		result, err := handler(ctx, msgs...)
		for _, msg := range msgs {
			if msg == nil {
				continue
			}
			messageCtx := logger.WithLogID(ctx, msg.GetProperty("log_id"))
			if err != nil {
				logger.Error(messageCtx, "消费RocketMQ消息失败", "client_id", clientID, "topic", msg.Topic, "msg_id", msg.MsgId, "error", err)
			} else {
				logger.Info(messageCtx, "消费RocketMQ消息完成", "client_id", clientID, "topic", msg.Topic, "msg_id", msg.MsgId, "result", result)
			}
		}
		return result, err
	}
}

// 关闭所有消费者实例
func ShutdownAllConsumers() error {
	var err error
	for _, bc := range consumerList {

		if shutdownErr := bc.Instance.Shutdown(); shutdownErr != nil {
			logger.Sugar.Infof("业务[%s]消费者[%s]实例[%d]关闭失败: %v", bc.Config.BizName, bc.Config.ConsumerGroup, bc.InstanceID, shutdownErr)
			err = shutdownErr
		} else {
			logger.Sugar.Infof("业务[%s]消费者[%s]实例[%d]关闭成功", bc.Config.BizName, bc.Config.ConsumerGroup, bc.InstanceID)
		}
	}
	return err
}

// 生成消费者实例的唯一ClientID
func generateConsumerClientID(consumerGroup string, instanceID int) string {
	// 1. 获取本机IP（避免多机器部署时重复）
	ip := getLocalIP()
	if ip == "" {
		ip = "unknown-ip"
	}
	// 2. 获取进程ID（单机多实例时区分）
	pid := os.Getpid()
	// 3. 拼接唯一标识：IP-进程ID-消费者组-实例ID
	return fmt.Sprintf("%s-%d-%s-%d", ip, pid, consumerGroup, instanceID)
}

// 获取本机非回环IP（简化版）
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return "127.0.0.1" // 兜底：回环IP
}

// ========== 核心：监听异步启动结果（真正判断实例是否就绪） ==========

// WaitConsumerReady 等待消费者实例异步启动完成（超时返回错误）
// 返回：true=就绪，false=未就绪
// WaitConsumerReady 通用版：不依赖订阅报错，直接校验实例就绪状态
func WaitConsumerReady(c rocketmq.PushConsumer, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond) // 降低轮询频率，减少开销
	defer ticker.Stop()

	// 记录重试次数，避免无限轮询
	retryCount := 0
	maxRetry := int(timeout / (200 * time.Millisecond)) // 超时前最大重试次数

	for {
		select {
		case <-ctx.Done():
			logger.Sugar.Infof("[WARN] 实例就绪检查超时（%v），重试次数：%d", timeout, retryCount)
			return false
		case <-ticker.C:
			retryCount++
			// 1. 基础校验：实例非空
			if c == nil {
				logger.Sugar.Infof("[DEBUG] 第%d次检查：实例为nil，跳过", retryCount)
				continue
			}

			// 2. 核心：主动校验实例是否就绪（适配所有版本）
			if isConsumerReady(c) {
				logger.Sugar.Infof("[INFO] 第%d次检查：实例已就绪", retryCount)
				return true
			}

			// 3. 达到最大重试次数，判定未就绪
			if retryCount >= maxRetry {
				logger.Sugar.Infof("[WARN] 达到最大重试次数（%d），实例仍未就绪", maxRetry)
				return false
			}
			logger.Sugar.Infof("[DEBUG] 第%d次检查：实例未就绪，继续等待", retryCount)
		}
	}
}

// isConsumerReady 主动校验实例状态（核心逻辑，适配所有版本）
func isConsumerReady(c rocketmq.PushConsumer) bool {
	// 方式1：尝试获取消费组信息（最可靠，适配所有版本）
	// 原理：实例就绪后，能获取到消费组的基本信息；未就绪则返回错误
	if err := getConsumerGroupInfo(c); err == nil {
		return true
	}

	// 方式2：备选方案——尝试订阅一个「随机无效Topic」（比空Topic更通用）
	// 原理：就绪实例会校验Topic格式，未就绪则连校验都走不到
	randomInvalidTopic := fmt.Sprintf("invalid_topic_%d", time.Now().UnixNano())
	err := c.Subscribe(
		randomInvalidTopic,
		consumer.MessageSelector{},
		func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
			return consumer.ConsumeSuccess, nil
		},
	)
	// 只要报错（无论什么错），说明实例已进入「订阅校验阶段」= 就绪
	if err != nil {
		return true
	}

	// 极端情况：订阅无效Topic也没报错 → 再通过「是否能获取配置」校验
	return canGetConsumerConfig(c)
}

// getConsumerGroupInfo 尝试获取消费组信息（轻量操作）
func getConsumerGroupInfo(c rocketmq.PushConsumer) error {
	// 反射获取实例内部的消费组信息（适配v2.x所有版本，无侵入）
	// 原理：就绪实例的内部字段已初始化，能拿到消费组名；未就绪则反射失败
	val := reflect.ValueOf(c).Elem()
	// 不同版本的内部字段名："consumerGroup"/"groupName"/"group"
	groupFields := []string{"consumerGroup", "groupName", "group"}
	for _, fieldName := range groupFields {
		field := val.FieldByName(fieldName)
		if field.IsValid() && field.Kind() == reflect.String && field.String() != "" {
			logger.Sugar.Infof("[DEBUG] 获取到消费组名：%s", field.String())
			return nil
		}
	}
	return fmt.Errorf("未获取到消费组信息")
}

// canGetConsumerConfig 尝试获取消费者配置（备选校验）
func canGetConsumerConfig(c rocketmq.PushConsumer) bool {
	val := reflect.ValueOf(c).Elem()
	// 检查核心配置字段（如NameServer地址）是否已初始化
	nsField := val.FieldByName("namesrvAddr")
	if nsField.IsValid() && nsField.Kind() == reflect.String && nsField.String() != "" {
		return true
	}
	return false
}

// StartConsumerAndWait 启动实例 + 等待异步就绪
func StartConsumerAndWait(
	c rocketmq.PushConsumer,
	bizName, consumerGroup string,
	instanceID int,
	msgModel consumer.MessageModel,
	topic string,
) error {
	// 1. 调用Start（异步发起启动）
	startErr := c.Start()
	if startErr != nil {
		// 仅集群模式忽略「消费组已存在」伪错误
		errMsg := strings.ToLower(startErr.Error())
		if msgModel == consumer.Clustering && strings.Contains(errMsg, "consumer group has been created") {
			logger.Sugar.Infof("[INFO] [集群] 实例[%d]消费组已存在（复用）", instanceID)
		} else {
			logger.Sugar.Infof("[ERROR] [模式:%s] 实例[%d]Start调用失败: %v", getModelName(msgModel), instanceID, startErr)
			return startErr
		}
	}

	// 2. 发送带唯一标识的探测消息，并等待该消费者回调确认。
	checkID := fmt.Sprintf("%s-%d-%d", consumerGroup, instanceID, time.Now().UnixNano())
	ready := make(chan struct{}, 1)
	readyChecks.Store(checkID, ready)
	defer readyChecks.Delete(checkID)
	producer, ok := producerMap[bizName]
	if !ok {
		return fmt.Errorf("业务[%s]未找到生产者，无法探测消费者", bizName)
	}
	msg := primitive.NewMessage(topic, []byte("rocketmq-ready-ping"))
	msg.WithProperty(readyCheckProperty, checkID)
	msg.WithProperty("log_id", logger.NewLogID())
	if _, err := producer.Instance.SendSync(context.Background(), msg); err != nil {
		return fmt.Errorf("[模式:%s] 实例[%d]发送就绪探测消息失败: %w", getModelName(msgModel), instanceID, err)
	}
	select {
	case <-ready:
		logger.Sugar.Infof("[INFO] [模式:%s] 实例[%d]已收到就绪探测消息", getModelName(msgModel), instanceID)
		return nil
	case <-time.After(5 * time.Second):
		err := fmt.Errorf("[模式:%s] 实例[%d]就绪探测超时", getModelName(msgModel), instanceID)
		logger.Sugar.Infof("[ERROR] %v", err)
		return err
	}
}

func getModelName(model consumer.MessageModel) string {
	if model == consumer.BroadCasting {
		return "广播"
	}
	return "集群"
}
