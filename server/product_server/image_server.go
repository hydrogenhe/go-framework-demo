package product_server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	appconst "go-framework-demo/const"
	"go-framework-demo/storage"
)

// MaxProductImageSize 单张商品图片允许的最大字节数，5 MiB。
const MaxProductImageSize = 5 << 20

// ImageServiceInterface 列出图片上传业务的全部公开方法。
type ImageServiceInterface interface {
	Upload(ctx context.Context, body io.Reader) (string, error)
}

// 商品图片单独上传，MySQL 和 ES 仅保存图片地址。
type ImageService struct{ store *storage.ImageStore }

var _ ImageServiceInterface = (*ImageService)(nil)

// NewImageService 默认复用启动时初始化的全局客户端，也支持测试注入存储客户端。
func NewImageService(stores ...*storage.ImageStore) *ImageService {
	store := storage.ImageStoreClient
	if len(stores) > 0 && stores[0] != nil {
		store = stores[0]
	}
	service := &ImageService{store: store}
	return service
}

func (s *ImageService) Upload(ctx context.Context, body io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxProductImageSize+1))
	if err != nil {
		return "", appconst.NewInternalError(err)
	}
	if len(data) == 0 || len(data) > MaxProductImageSize {
		return "", appconst.NewInvalidParamsError("图片不能为空且不能超过 5MB")
	}
	contentType := http.DetectContentType(data)
	extension := ""
	switch contentType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	default:
		return "", appconst.NewInvalidParamsError("只支持 JPEG、PNG 图片")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40000000 {
		return "", appconst.NewInvalidParamsError("图片无效或分辨率过大")
	}
	store := s.store
	if store == nil {
		return "", appconst.NewInternalError(fmt.Errorf("图片存储客户端未初始化"))
	}
	key := "products/" + uuid.NewString() + extension
	uploadCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	imageURL, err := store.Put(uploadCtx, key, bytes.NewReader(data), int64(len(data)), contentType)
	if err != nil {
		return "", appconst.NewInternalError(fmt.Errorf("商品图片上传失败: %w", err))
	}
	return imageURL, nil
}
