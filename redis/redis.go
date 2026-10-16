package redis

import (
	"context"
	"fmt"
	"go-framework-demo/util"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

// OrderPaymentTimeoutKey Redis中订单支付时限配置的键，值的单位为分钟。
const OrderPaymentTimeoutKey = "order:payment_timeout_minutes"

// DefaultOrderPaymentTimeoutMinutes 订单支付时限配置不可用时的默认值，单位为分钟。
const DefaultOrderPaymentTimeoutMinutes = 10

// GetOrderPaymentTimeoutMinutes 每次从 Redis 读取订单支付时限。
// Redis 不可用、配置不存在或配置无效时回退到默认值，不能阻断下单。
// Redis 是当前实现；后续可将此读取器替换为 etcd 等配置平台。
func GetOrderPaymentTimeoutMinutes(ctx context.Context) int {
	if RedisClient == nil {
		return DefaultOrderPaymentTimeoutMinutes
	}
	value, err := RedisClient.Get(ctx, OrderPaymentTimeoutKey).Result()
	if err != nil {
		return DefaultOrderPaymentTimeoutMinutes
	}
	minutes, err := strconv.Atoi(value)
	if err != nil || minutes < 1 || minutes > 7*24*60 {
		return DefaultOrderPaymentTimeoutMinutes
	}
	return minutes
}

// 定义Redis常用返回值常量（枚举化）

const (
	RedisOK = "OK" // Redis SET等状态回复命令成功时的返回值
)

type FuncRedisOptions func(opt *redis.Options)

func WithAddr(addr string) FuncRedisOptions {
	return func(opt *redis.Options) {
		opt.Addr = addr
	}
}

type RedisConfig = util.RedisConfig

func Init(conf ...func(option *redis.Options)) error {
	configPath, err := util.ConfigFile("redis.yaml")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var fileConfig struct {
		Redis RedisConfig `yaml:"redis"`
	}
	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("解析 Redis 配置失败: %w", err)
	}
	return initConfig(fileConfig.Redis, conf...)
}

// InitWithConfig 使用启动入口已加载并校验的配置创建 Redis 客户端。
func InitWithConfig(cfg RedisConfig) error { return initConfig(cfg) }

func initConfig(cfg RedisConfig, conf ...func(option *redis.Options)) error {
	if cfg.Address == "" {
		return fmt.Errorf("Redis address 未配置")
	}
	opt := &redis.Options{Addr: cfg.Address, Password: cfg.Password, DB: cfg.Database,
		DialTimeout:  time.Duration(cfg.DialTimeoutSeconds) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeoutSeconds) * time.Second,
	}
	for _, f := range conf {
		f(opt)
	}

	client := redis.NewClient(opt)

	// 2. 验证连接是否成功
	ctx := context.Background() // 上下文，用于控制请求超时
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("连接 Redis 失败: %w", err)
	}
	fmt.Println("Redis连接成功：", pong)
	RedisClient = client
	return nil
}
