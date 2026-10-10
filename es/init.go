package es

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/olivere/elastic/v7" // 正确导入 olivere/elastic/v7
	"gopkg.in/yaml.v3"
)

// -------------------------- 全局客户端（olivere/elastic/v7） --------------------------
var (
	ElasticClient *elastic.Client // 核心：全局 olivere 客户端
)

// -------------------------- 配置结构体（兼容原有 es.yaml） --------------------------
type ESConfig struct {
	Addresses  []string `yaml:"addresses"`   // ES 节点地址
	Username   string   `yaml:"username"`    // 认证用户名
	Password   string   `yaml:"password"`    // 认证密码
	Timeout    int      `yaml:"timeout"`     // 超时时间（秒）
	MaxRetries int      `yaml:"max_retries"` // 最大重试次数
	Sniff      bool     `yaml:"sniff"`       // 节点嗅探（单节点建议 false）
}

type ESYamlConfig struct {
	Elasticsearch ESConfig `yaml:"elasticsearch"`
}

// InitES -------------------------- 初始化方法（适配 olivere/elastic/v7 正确 API） --------------------------
func InitES() error {
	// 1. 获取配置文件路径
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(currentFile)) // 项目根目录
	configPath := filepath.Join(projectRoot, "conf", "es.yaml")

	// 2. 加载配置
	cfg, err := loadESConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载 ES 配置失败: %w", err)
	}

	// 3. 构建 olivere/elastic 客户端选项（核心：修复 API 调用）
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(cfg.Addresses...),                 // 设置节点地址
		elastic.SetSniff(cfg.Sniff),                      // 关闭节点嗅探（单节点必选）
		elastic.SetHealthcheckInterval(10 * time.Second), // 健康检查
	}

	// 3.1 认证配置
	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts, elastic.SetBasicAuth(cfg.Username, cfg.Password))
	}

	// 3.2 超时配置（olivere 正确 API：SetRequestTimeout）
	timeout := time.Duration(cfg.Timeout) * time.Second
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second, // 全局请求超时
		Transport: &http.Transport{
			ResponseHeaderTimeout: time.Duration(cfg.Timeout) * time.Second, // 响应头超时
			IdleConnTimeout:       30 * time.Second,                         // 空闲连接超时
			MaxIdleConnsPerHost:   10,                                       // 每个主机最大空闲连接
		},
	}
	// 2. 将自定义HTTP客户端注入ES客户端
	opts = append(opts, elastic.SetHttpClient(httpClient))

	// 3.3 重试配置（olivere 正确 API）
	opts = append(opts, elastic.SetMaxRetries(cfg.MaxRetries))

	// 3.4 日志配置（调试用）
	opts = append(opts, elastic.SetErrorLog(log.New(os.Stderr, "ES-ERROR: ", log.LstdFlags)))
	opts = append(opts, elastic.SetInfoLog(log.New(os.Stdout, "ES-INFO: ", log.LstdFlags)))

	// 4. 创建客户端（olivere 正确 API）
	client, err := elastic.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("创建 ES 客户端失败: %w", err)
	}

	// 5. 验证连接（olive 正确 API：Info().Do(ctx)）
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	info, _, err := client.Ping(cfg.Addresses[0]).Do(ctx)
	if err != nil {
		return fmt.Errorf("连接 ES 失败: %w", err)
	}
	log.Printf("✅ 连接 ES 成功（olivere/elastic/v7）：%+v", info)

	// 6. 赋值全局客户端
	ElasticClient = client
	return nil
}

// -------------------------- 配置加载方法 --------------------------
func loadESConfig(configPath string) (ESConfig, error) {
	// 1. 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ESConfig{}, fmt.Errorf("读取配置文件 [%s] 失败: %w", configPath, err)
	}

	// 2. 解析 YAML
	var yamlConfig ESYamlConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return ESConfig{}, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 3. 设置默认值
	cfg := yamlConfig.Elasticsearch
	if len(cfg.Addresses) == 0 {
		cfg.Addresses = []string{"http://127.0.0.1:9200"}
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	cfg.Sniff = false // 单节点强制关闭嗅探

	return cfg, nil
}
