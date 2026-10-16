package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go-framework-demo/util"
	"gopkg.in/yaml.v3"
)

type ImageConfig = util.ImageConfig

// ImageStoreInterface 列出图片存储的全部公开方法。
type ImageStoreInterface interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (string, error)
}

type ImageStore struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

var _ ImageStoreInterface = (*ImageStore)(nil)

// ImageStoreClient 由 main 在启动阶段初始化，供图片服务复用；运行期间不重新赋值。
var ImageStoreClient *ImageStore

// InitImageStore 读取配置并创建全局图片存储客户端，应在注册 HTTP 路由前调用。
func InitImageStore() error {
	config, err := LoadImageConfig()
	if err != nil {
		return err
	}
	return InitImageStoreWithConfig(config)
}

// InitImageStoreWithConfig 使用启动入口统一加载的图片存储配置。
func InitImageStoreWithConfig(config ImageConfig) error {
	store, err := NewImageStore(&config)
	if err != nil {
		return err
	}
	ImageStoreClient = store
	return nil
}

func LoadImageConfig() (ImageConfig, error) {
	var config ImageConfig
	path, err := util.ConfigFile("image_storage.yaml")
	if err != nil {
		return config, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("读取图片存储配置失败: %w", err)
	}
	var fileConfig struct {
		ImageStorage ImageConfig `yaml:"image_storage"`
	}
	if err := yaml.Unmarshal(raw, &fileConfig); err != nil {
		return config, fmt.Errorf("解析图片存储配置失败: %w", err)
	}
	config = fileConfig.ImageStorage
	return config, nil
}

func NewImageStore(config *ImageConfig) (*ImageStore, error) {
	if config == nil {
		return nil, fmt.Errorf("图片存储配置不能为空")
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Path != "" {
		return nil, fmt.Errorf("图片存储 endpoint 无效")
	}
	publicURL, err := url.Parse(config.PublicURL)
	if err != nil || publicURL.Host == "" || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("图片存储配置不完整")
	}
	options := &minio.Options{
		Creds:        credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure:       endpoint.Scheme == "https",
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
	}
	client, err := minio.New(endpoint.Host, options)
	if err != nil {
		return nil, err
	}
	store := &ImageStore{
		client:    client,
		bucket:    config.Bucket,
		publicURL: strings.TrimRight(config.PublicURL, "/"),
	}
	return store, nil
}

func (s *ImageStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (string, error) {
	options := minio.PutObjectOptions{
		ContentType:      contentType,
		DisableMultipart: true,
	}
	if _, err := s.client.PutObject(ctx, s.bucket, key, body, size, options); err != nil {
		return "", fmt.Errorf("上传图片到对象存储失败: %w", err)
	}
	return s.publicURL + "/" + key, nil
}
