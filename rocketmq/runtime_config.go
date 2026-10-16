package rocketmq

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/logger"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/util"
	"golang.org/x/time/rate"
)

// -------------------------- 核心类型定义 --------------------------

const (
	// UserTopic 用户业务事件的Topic名称。
	UserTopic = "user_topic"
	// OrderTopic 订单业务事件的Topic名称。
	OrderTopic = "order_topic"
	// ProductTopic 商品业务事件的Topic名称。
	ProductTopic = "product_topic"
	// ProductCDCTopic 由 Canal 投递，仅供 ES 数据同步链路消费。
	ProductCDCTopic = "product_cdc_topic"
	// ProductESConsumerGroup 商品CDC同步到ES的消费组名称。
	ProductESConsumerGroup = "product_es_consumer_group"
	// OrderCDCTopic 订单及订单明细CDC变更的Topic名称。
	OrderCDCTopic = "order_cdc_topic"
	// OrderESConsumerGroup 订单CDC同步到ES的消费组名称。
	OrderESConsumerGroup = "order_es_consumer_group"
)

// ConsumeHandlerFunc 消费回调函数类型
type ConsumeHandlerFunc func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error)

//  -------------------------- 外部配置结构体 --------------------------

// RocketMQConfig 全局配置结构体
type RocketMQConfig = util.RocketMQConfig

// CommonConfig 公共配置
type CommonConfig = util.RocketMQCommonConfig

// ConsumerConfig 单消费者配置
type ConsumerConfig = util.RocketMQConsumerConfig

// BusinessConfig 单业务配置（生产者+多消费者）
type BusinessConfig = util.RocketMQBusinessConfig

// -------------------------- 运行时配置结构体（新增，直接用于构建实例） --------------------------

// RuntimeProducerConfig 生产者运行时配置（直接映射BusinessConfig）
type RuntimeProducerConfig struct {
	BizName       string        `json:"biz_name"`
	Topic         string        `json:"topic"`
	ProducerGroup string        `json:"producer_group"`
	RetryTimes    int           `json:"retry_times"`
	MaxSendQPS    int           `json:"max_send_qps"`
	MaxSendBurst  int           `json:"max_send_burst"`
	SendTimeout   int           `json:"send_timeout"`
	Limiter       *rate.Limiter `json:"limiter"` // 流量控制器
}

// RuntimeConsumerConfig 消费者运行时配置（直接映射ConsumerConfig）
type RuntimeConsumerConfig struct {
	BizName        string                `json:"biz_name"`
	Topic          string                `json:"topic"`
	ConsumerGroup  string                `json:"consumer_group"`
	ConsumeHandler ConsumeHandlerFunc    `json:"-"`             // 不序列化，关联回调函数
	MessageModel   consumer.MessageModel `json:"message_model"` // 枚举类型
	ThreadNums     int                   `json:"thread_nums"`
	InstanceCount  int                   // 新增：要启动的实例数量（默认1）
	ConsumeOrderly bool
}

// -------------------------- 全局变量 --------------------------
var (
	// 公共配置
	GlobalCommonConfig CommonConfig
	// 生产者运行时配置（bizName -> RuntimeProducerConfig）
	GlobalProducerConfigs = make(map[string]*RuntimeProducerConfig)
	// 消费者运行时配置列表
	GlobalConsumerConfigs = make([]*RuntimeConsumerConfig, 0)
	// 消费回调函数映射（标识 -> 回调函数）
	consumerGroupHandlerMap = make(map[string]ConsumeHandlerFunc)
)

// -------------------------- 外部配置结构体转换为运行时配置 --------------------------

// LoadRuntimeConfig 根据统一配置入口传入的原始配置构造 RocketMQ 运行时配置。
func LoadRuntimeConfig(rawConfig RocketMQConfig) error {

	// 4. 初始化公共配置（兜底默认值）
	GlobalCommonConfig = rawConfig.Common
	if GlobalCommonConfig.NameSrvAddr == "" {
		return errors.New("公共配置name_srv_addr不能为空")
	}
	if GlobalCommonConfig.DefaultRetryTimes <= 0 {
		GlobalCommonConfig.DefaultRetryTimes = 1
	}
	if GlobalCommonConfig.DefaultMaxSendQPS <= 0 {
		GlobalCommonConfig.DefaultMaxSendQPS = 100
	}
	if GlobalCommonConfig.DefaultSendTimeout <= 0 {
		GlobalCommonConfig.DefaultSendTimeout = 3000
	}
	if GlobalCommonConfig.DefaultMessageModel == "" {
		GlobalCommonConfig.DefaultMessageModel = "clustering"
	}
	if GlobalCommonConfig.DefaultThreadNums <= 0 {
		GlobalCommonConfig.DefaultThreadNums = 3
	}

	// 5. 解析业务配置 -> 运行时配置（直接映射，无WithXXX转换）
	for _, bizConf := range rawConfig.Businesses {
		// 校验业务必填项
		if bizConf.BizName == "" {
			return errors.New("业务配置biz_name不能为空")
		}
		if bizConf.Topic == "" {
			return fmt.Errorf("业务[%s]topic不能为空", bizConf.BizName)
		}

		// -------------------------- 构建生产者运行时配置（直接赋值） --------------------------
		producerConfig := &RuntimeProducerConfig{
			BizName:       bizConf.BizName,
			Topic:         bizConf.Topic,
			ProducerGroup: bizConf.ProducerGroup,
			RetryTimes:    bizConf.RetryTimes,
			MaxSendQPS:    bizConf.MaxSendQPS,
			SendTimeout:   bizConf.SendTimeout,
		}

		// 兜底生产者配置（使用公共默认值）
		if producerConfig.ProducerGroup == "" {
			producerConfig.ProducerGroup = fmt.Sprintf("%s_producer_group", bizConf.BizName)
			logger.Sugar.Infof("[%s] 生产者组未配置，自动补全为: %s", bizConf.BizName, producerConfig.ProducerGroup)
		}
		if producerConfig.RetryTimes <= 0 {
			producerConfig.RetryTimes = GlobalCommonConfig.DefaultRetryTimes
		}
		if producerConfig.MaxSendQPS <= 0 {
			producerConfig.MaxSendQPS = GlobalCommonConfig.DefaultMaxSendQPS
		}
		if producerConfig.SendTimeout <= 0 {
			producerConfig.SendTimeout = GlobalCommonConfig.DefaultSendTimeout
		}

		// 初始化流量控制器（直接用MaxSendQPS）
		if producerConfig.MaxSendBurst <= 0 {
			// 默认突发量为QPS的1/10，避免瞬间打满
			producerConfig.MaxSendBurst = producerConfig.MaxSendQPS / 10
			if producerConfig.MaxSendBurst == 0 {
				producerConfig.MaxSendBurst = 10 // 最小突发量兜底
			}
		}
		producerConfig.Limiter = rate.NewLimiter(rate.Limit(producerConfig.MaxSendQPS), producerConfig.MaxSendBurst)
		GlobalProducerConfigs[bizConf.BizName] = producerConfig

		// -------------------------- 构建消费者运行时配置（直接映射） --------------------------
		for _, consumerConf := range bizConf.Consumers {
			// 校验消费者必填项
			if consumerConf.ConsumerGroup == "" {
				consumerConf.ConsumerGroup = fmt.Sprintf("%s_consumer_group", bizConf.BizName)
				logger.Sugar.Infof("[%s] 消费者组未配置，自动补全为: %s", bizConf.BizName, consumerConf.ConsumerGroup)
			}
			// 转换消费模式（字符串 -> 枚举）
			var msgModel consumer.MessageModel
			switch consumerConf.MessageModel {
			case "broadcasting":
				msgModel = consumer.BroadCasting
			case "clustering":
				msgModel = consumer.Clustering
			default:
				msgModel = consumer.Clustering
				logger.Sugar.Infof("[%s] 消费者[%s]模式[%s]无效，使用默认集群模式", bizConf.BizName, consumerConf.ConsumerGroup, consumerConf.MessageModel)
			}

			// 兜底消费者配置
			threadNums := consumerConf.ThreadNums
			if threadNums <= 0 {
				threadNums = GlobalCommonConfig.DefaultThreadNums
			}

			// 获取回调函数（直接从映射中取）
			handler, ok := consumerGroupHandlerMap[consumerConf.ConsumerGroup]
			if !ok {
				return fmt.Errorf("业务[%s]消费者[%s]未注册消费回调", bizConf.BizName, consumerConf.ConsumerGroup)
			}

			// 构建消费者运行时配置（直接赋值）
			runtimeConfig := &RuntimeConsumerConfig{
				BizName:        bizConf.BizName,
				Topic:          bizConf.Topic,
				ConsumerGroup:  consumerConf.ConsumerGroup,
				ConsumeHandler: handler,
				MessageModel:   msgModel,
				ThreadNums:     threadNums,
				InstanceCount:  consumerConf.InstanceCount,
				ConsumeOrderly: consumerConf.ConsumeOrderly,
			}
			GlobalConsumerConfigs = append(GlobalConsumerConfigs, runtimeConfig)
		}
	}

	logger.Sugar.Infof("配置加载完成：%d个生产者，%d个消费者", len(GlobalProducerConfigs), len(GlobalConsumerConfigs))
	return nil
}

// RegisterConsumerGroupHandler 将消费者组直接绑定到消费回调。
// 消费者组属于部署配置，具体处理逻辑由代码指定，不再需要写入 YAML。
func RegisterConsumerGroupHandler(consumerGroup string, handler ConsumeHandlerFunc) {
	if consumerGroup == "" || handler == nil {
		logger.Sugar.Infof("[警告] 消费者组回调注册失败：消费者组为空或函数为nil")
		return
	}
	consumerGroupHandlerMap[consumerGroup] = handler
	logger.Sugar.Infof("注册消费者组回调: %s", consumerGroup)
}

// defaultConsumeHandler 默认回调函数
func defaultConsumeHandler(bizName string) ConsumeHandlerFunc {
	return func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		logger.Sugar.Infof("[%s] 使用默认回调函数", bizName)
		return consumer.ConsumeSuccess, nil
	}
}
