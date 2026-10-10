package rocketmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
)

// -------------------------- 核心类型定义 --------------------------

const (
	UserTopic    = "user_topic"
	OrderTopic   = "order_topic"
	ProductTopic = "product_topic"
)

// ConsumeHandlerFunc 消费回调函数类型
type ConsumeHandlerFunc func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error)

//  -------------------------- 配置文件结构体（与YAML完全对应） --------------------------

// RocketMQConfig 全局配置结构体
type RocketMQConfig struct {
	Common     CommonConfig     `yaml:"common"`     // 公共配置
	Businesses []BusinessConfig `yaml:"businesses"` // 业务配置列表
}

// CommonConfig 公共配置
type CommonConfig struct {
	NameSrvAddr         string `yaml:"name_srv_addr"`
	BrokerAddr          string `yaml:"broker_addr"`
	DefaultRetryTimes   int    `yaml:"default_retry_times"`
	DefaultMaxSendQPS   int    `yaml:"default_max_send_qps"`
	DefaultSendTimeout  int    `yaml:"default_send_timeout"`
	DefaultMessageModel string `yaml:"default_message_model"` // 默认消费模式：clustering/broadcasting
	DefaultThreadNums   int    `yaml:"default_thread_nums"`   // 默认消费线程数
}

// ConsumerConfig 单消费者配置
type ConsumerConfig struct {
	ConsumerGroup  string `yaml:"consumer_group"`  // 消费者组
	ConsumeHandler string `yaml:"consume_handler"` // 回调函数标识
	MessageModel   string `yaml:"message_model"`   // 消费模式
	ThreadNums     int    `yaml:"thread_nums"`     // 消费线程数
	InstanceCount  int    `yaml:"instance_count"`
}

// BusinessConfig 单业务配置（生产者+多消费者）
type BusinessConfig struct {
	BizName       string           `yaml:"biz_name"`       // 业务唯一标识
	Topic         string           `yaml:"topic"`          // 主题名
	ProducerGroup string           `yaml:"producer_group"` // 生产者组
	RetryTimes    int              `yaml:"retry_times"`    // 生产者重试次数
	MaxSendQPS    int              `yaml:"max_send_qps"`   // 生产者最大QPS
	SendTimeout   int              `yaml:"send_timeout"`   // 生产者发送超时（毫秒）
	Consumers     []ConsumerConfig `yaml:"consumers"`      // 多消费者配置
}

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
	consumeHandlerMap = make(map[string]ConsumeHandlerFunc)
)

// -------------------------- 配置加载逻辑（直接映射，无函数式转换） --------------------------

func LoadConfig(configPath ...string) error {
	var path string

	// 1. 优先使用传入的配置路径（修复语法+越界问题）
	if len(configPath) > 0 && strings.TrimSpace(configPath[0]) != "" {
		path = configPath[0]
		// 可选：验证传入的路径是否是绝对路径（增强鲁棒性）
		if !filepath.IsAbs(path) {
			// 如果传入的是相对路径，转为绝对路径
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("转换配置路径为绝对路径失败: %w", err)
			}
			path = absPath
		}
	} else {
		// 2. 未传入路径，使用默认路径（适配你的项目结构：conf/rocketmq.yaml）
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("获取工作目录失败: %w", err)
		}
		// 修正：把 config 改为 conf（匹配你的项目文件夹名）
		path = filepath.Join(wd, "conf", "rocketmq.yaml")
	}

	// 3. 验证配置文件是否存在（提前报错，便于排查）
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s, 错误: %w", path, err)
	}

	// 3. 读取并解析YAML
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var rawConfig RocketMQConfig
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return fmt.Errorf("解析YAML失败: %w", err)
	}

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
			log.Printf("[%s] 生产者组未配置，自动补全为: %s", bizConf.BizName, producerConfig.ProducerGroup)
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
				log.Printf("[%s] 消费者组未配置，自动补全为: %s", bizConf.BizName, consumerConf.ConsumerGroup)
			}
			if consumerConf.ConsumeHandler == "" {
				return fmt.Errorf("业务[%s]消费者[%s]consume_handler不能为空", bizConf.BizName, consumerConf.ConsumerGroup)
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
				log.Printf("[%s] 消费者[%s]模式[%s]无效，使用默认集群模式", bizConf.BizName, consumerConf.ConsumerGroup, consumerConf.MessageModel)
			}

			// 兜底消费者配置
			threadNums := consumerConf.ThreadNums
			if threadNums <= 0 {
				threadNums = GlobalCommonConfig.DefaultThreadNums
			}

			// 获取回调函数（直接从映射中取）
			handler, ok := consumeHandlerMap[consumerConf.ConsumeHandler]
			if !ok {
				return fmt.Errorf("业务[%s]消费者[%s]未找到回调函数[%s]", bizConf.BizName, consumerConf.ConsumerGroup, consumerConf.ConsumeHandler)
			}

			// 构建消费者运行时配置（直接赋值）
			GlobalConsumerConfigs = append(GlobalConsumerConfigs, &RuntimeConsumerConfig{
				BizName:        bizConf.BizName,
				Topic:          bizConf.Topic,
				ConsumerGroup:  consumerConf.ConsumerGroup,
				ConsumeHandler: handler,
				MessageModel:   msgModel,
				ThreadNums:     threadNums,
				InstanceCount:  consumerConf.InstanceCount,
			})
		}
	}

	log.Printf("配置加载完成：%d个生产者，%d个消费者", len(GlobalProducerConfigs), len(GlobalConsumerConfigs))
	return nil
}

// -------------------------- 回调函数注册（保留，无变化） --------------------------
func RegisterConsumeHandler(handlerKey string, handler ConsumeHandlerFunc) {
	if handlerKey == "" || handler == nil {
		log.Printf("[警告] 回调注册失败：标识为空或函数为nil")
		return
	}
	consumeHandlerMap[handlerKey] = handler
	log.Printf("注册回调函数: %s", handlerKey)
}

// defaultConsumeHandler 默认回调函数
func defaultConsumeHandler(bizName string) ConsumeHandlerFunc {
	return func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		log.Printf("[%s] 使用默认回调函数", bizName)
		return consumer.ConsumeSuccess, nil
	}
}
