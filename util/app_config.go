package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	OrderExpiration struct {
		IntervalSeconds int `yaml:"interval_seconds"`
	} `yaml:"order_expiration"`
	HTTP struct {
		Address string `yaml:"address"`
	} `yaml:"http"`
	Snowflake struct {
		MachineID int64 `yaml:"machine_id"`
	} `yaml:"snowflake"`
	Logs struct {
		Directory string `yaml:"directory"`
	} `yaml:"logs"`
	Frontend struct {
		APIBaseURL string `yaml:"api_base_url"`
		Address    string `yaml:"address"`
	} `yaml:"frontend"`
	Reconciliation struct {
		IntervalHours int `yaml:"interval_hours"`
	} `yaml:"reconciliation"`
	MySQL        MySQLConfig    `yaml:"-"`
	Redis        RedisConfig    `yaml:"-"`
	ES           ESConfig       `yaml:"-"`
	ImageStorage ImageConfig    `yaml:"-"`
	RocketMQ     RocketMQConfig `yaml:"-"`
}

type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"database"`
	Charset  string `yaml:"charset"`
}
type RedisConfig struct {
	Address             string `yaml:"address"`
	Password            string `yaml:"password"`
	Database            int    `yaml:"database"`
	DialTimeoutSeconds  int    `yaml:"dial_timeout_seconds"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}
type ESConfig struct {
	Addresses  []string `yaml:"addresses"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	Timeout    int      `yaml:"timeout"`
	MaxRetries int      `yaml:"max_retries"`
	Sniff      bool     `yaml:"sniff"`
}
type ImageConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	PublicURL string `yaml:"public_url"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}
type RocketMQConfig struct {
	Common     RocketMQCommonConfig     `yaml:"common"`
	Businesses []RocketMQBusinessConfig `yaml:"businesses"`
}
type RocketMQCommonConfig struct {
	NameSrvAddr         string `yaml:"name_srv_addr"`
	BrokerAddr          string `yaml:"broker_addr"`
	DefaultRetryTimes   int    `yaml:"default_retry_times"`
	DefaultMaxSendQPS   int    `yaml:"default_max_send_qps"`
	DefaultSendTimeout  int    `yaml:"default_send_timeout"`
	DefaultMessageModel string `yaml:"default_message_model"`
	DefaultThreadNums   int    `yaml:"default_thread_nums"`
}
type RocketMQBusinessConfig struct {
	BizName       string                   `yaml:"biz_name"`
	Topic         string                   `yaml:"topic"`
	ProducerGroup string                   `yaml:"producer_group"`
	RetryTimes    int                      `yaml:"retry_times"`
	MaxSendQPS    int                      `yaml:"max_send_qps"`
	SendTimeout   int                      `yaml:"send_timeout"`
	Consumers     []RocketMQConsumerConfig `yaml:"consumers"`
}
type RocketMQConsumerConfig struct {
	ConsumerGroup  string `yaml:"consumer_group"`
	MessageModel   string `yaml:"message_model"`
	ThreadNums     int    `yaml:"thread_nums"`
	InstanceCount  int    `yaml:"instance_count"`
	ConsumeOrderly bool   `yaml:"consume_orderly"`
}

// LoadAppConfig 只加载 HTTP、前端等应用进程配置，供不依赖后端中间件的工具使用。
func LoadAppConfig() (AppConfig, error) {
	var config AppConfig
	path, err := ConfigFile("app.yaml")
	if err != nil {
		return config, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("解析应用配置失败: %w", err)
	}
	if config.HTTP.Address == "" {
		return config, fmt.Errorf("app.yaml 中 http.address 未配置")
	}
	if config.Logs.Directory == "" {
		config.Logs.Directory = "logs"
	}
	if config.Frontend.APIBaseURL == "" {
		return config, fmt.Errorf("app.yaml 中 frontend.api_base_url 未配置")
	}
	if config.Frontend.Address == "" {
		return config, fmt.Errorf("app.yaml 中 frontend.address 未配置")
	}
	if config.OrderExpiration.IntervalSeconds < 0 {
		return config, fmt.Errorf("order_expiration.interval_seconds 不能为负数")
	}
	if config.OrderExpiration.IntervalSeconds == 0 {
		config.OrderExpiration.IntervalSeconds = 3
	}
	if config.Reconciliation.IntervalHours <= 0 {
		config.Reconciliation.IntervalHours = 24
	}
	return config, nil
}

// LoadConfig 是后端进程统一的配置入口：加载全部组件配置并在启动前校验。
func LoadConfig() (AppConfig, error) {
	config, err := LoadAppConfig()
	if err != nil {
		return config, err
	}
	if err := loadComponentConfig("mysql.yaml", "mysql", &config.MySQL); err != nil {
		return config, err
	}
	if err := loadComponentConfig("redis.yaml", "redis", &config.Redis); err != nil {
		return config, err
	}
	if err := loadComponentConfig("es.yaml", "elasticsearch", &config.ES); err != nil {
		return config, err
	}
	if err := loadComponentConfig("image_storage.yaml", "image_storage", &config.ImageStorage); err != nil {
		return config, err
	}
	if err := loadComponentConfig("rocketmq.yaml", "", &config.RocketMQ); err != nil {
		return config, err
	}
	if config.MySQL.Host == "" || config.MySQL.Port == "" || config.MySQL.User == "" || config.MySQL.Password == "" || config.MySQL.DBName == "" {
		return config, fmt.Errorf("MySQL 配置不完整")
	}
	if config.Redis.Address == "" {
		return config, fmt.Errorf("Redis address 未配置")
	}
	if len(config.ES.Addresses) == 0 {
		return config, fmt.Errorf("elasticsearch.addresses 未配置")
	}
	if config.ImageStorage.Endpoint == "" || config.ImageStorage.Bucket == "" || config.ImageStorage.AccessKey == "" || config.ImageStorage.SecretKey == "" {
		return config, fmt.Errorf("图片存储配置不完整")
	}
	if config.RocketMQ.Common.NameSrvAddr == "" {
		return config, fmt.Errorf("RocketMQ common.name_srv_addr 未配置")
	}
	if _, err := strconv.Atoi(config.MySQL.Port); err != nil {
		return config, fmt.Errorf("MySQL port 无效")
	}
	if config.ES.Timeout <= 0 {
		config.ES.Timeout = 10
	}
	if config.ES.MaxRetries <= 0 {
		config.ES.MaxRetries = 3
	}
	config.ES.Sniff = false
	if config.MySQL.Charset == "" {
		config.MySQL.Charset = "utf8mb4"
	}
	if config.RocketMQ.Common.DefaultRetryTimes <= 0 {
		config.RocketMQ.Common.DefaultRetryTimes = 1
	}
	if config.RocketMQ.Common.DefaultMaxSendQPS <= 0 {
		config.RocketMQ.Common.DefaultMaxSendQPS = 100
	}
	if config.RocketMQ.Common.DefaultSendTimeout <= 0 {
		config.RocketMQ.Common.DefaultSendTimeout = 3000
	}
	if config.RocketMQ.Common.DefaultThreadNums <= 0 {
		config.RocketMQ.Common.DefaultThreadNums = 3
	}
	if config.RocketMQ.Common.DefaultMessageModel == "" {
		config.RocketMQ.Common.DefaultMessageModel = "clustering"
	}
	for i, business := range config.RocketMQ.Businesses {
		if business.BizName == "" || business.Topic == "" {
			return config, fmt.Errorf("RocketMQ businesses[%d] 的 biz_name 和 topic 必须配置", i)
		}
	}
	return config, nil
}

func loadComponentConfig(file, key string, target any) error {
	path, err := ConfigFile(file)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", file, err)
	}
	if key == "" {
		if err := yaml.Unmarshal(data, target); err != nil {
			return fmt.Errorf("解析 %s 失败: %w", file, err)
		}
		return nil
	}
	var wrapped map[string]any
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", file, err)
	}
	value, ok := wrapped[key]
	if !ok {
		return fmt.Errorf("%s 中缺少 %s 配置", file, key)
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(encoded, target); err != nil {
		return fmt.Errorf("解析 %s 配置失败: %w", file, err)
	}
	return nil
}

func ResolveProjectPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	root, err := ProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, path), nil
}
