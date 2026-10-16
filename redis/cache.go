package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go-framework-demo/model"

	"github.com/go-redis/redis/v8"
)

// --------------------------- 通用泛型缓存接口（支持指针类型） ---------------------------

type GenericCache[T any] interface {
	// Set 支持传入 T（值/指针），内部自动处理
	Set(ctx context.Context, prefix string, id any, data T, expire time.Duration) error
	// Get 返回 T（指针类型时自动分配内存，无空指针）
	Get(ctx context.Context, prefix string, id any) (obj T, hit bool, err error)
	Del(ctx context.Context, prefix string, id any) error
}

// --------------------------- Redis 实现（零拷贝核心） ---------------------------

type RedisGenericCache[T any] struct {
	rdb *redis.Client
}

func NewRedisGenericCache[T any](rdb *redis.Client) GenericCache[T] {
	if rdb == nil {
		panic("redis client is nil")
	}
	return &RedisGenericCache[T]{
		rdb: rdb,
	}
}

// Set：直接操作入参（值/指针），零拷贝序列化

func (r *RedisGenericCache[T]) Set(ctx context.Context, prefix string, id any, data T, expire time.Duration) error {
	if prefix == "" || id == nil {
		return errors.New("cache prefix/id can't be empty")
	}

	// 序列化：直接传入 data（支持值/指针），json.Marshal 自动解引用指针（零拷贝）
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal %T failed: %w", data, err)
	}

	key := fmt.Sprintf("%s:%v", prefix, id)
	return r.rdb.Set(ctx, key, dataBytes, expire).Err()
}

// Get ：指针类型自动分配内存，零拷贝反序列化
func (r *RedisGenericCache[T]) Get(ctx context.Context, prefix string, id any) (obj T, hit bool, err error) {
	if prefix == "" || id == nil {
		return obj, false, errors.New("cache prefix/id can't be empty")
	}

	key := fmt.Sprintf("%s:%v", prefix, id)
	dataBytes, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return obj, false, nil
		}
		return obj, false, fmt.Errorf("get cache key=%s failed: %w", key, err)
	}

	// 核心：自动为指针类型分配内存（解决空指针问题）
	// 1. 获取 T 的反射类型
	tType := reflect.TypeOf(obj)
	// 2. 如果 T 是指针类型，且当前值为 nil → 分配内存
	if tType.Kind() == reflect.Ptr && reflect.ValueOf(obj).IsNil() {
		// 创建指针指向的实例，再赋值给 obj
		elem := reflect.New(tType.Elem())
		newObj := elem.Interface()
		obj = newObj.(T)
	}

	// 反序列化：直接操作 obj（指针/值），零拷贝
	err = json.Unmarshal(dataBytes, &obj)
	if err != nil {
		return obj, false, fmt.Errorf("unmarshal %T failed: %w", obj, err)
	}

	return obj, true, nil
}

func (r *RedisGenericCache[T]) Del(ctx context.Context, prefix string, id any) error {
	if prefix == "" || id == nil {
		return errors.New("cache prefix/id can't be empty")
	}
	key := fmt.Sprintf("%s:%v", prefix, id)
	_, err := r.rdb.Del(ctx, key).Result()
	return err
}

// --------------------------- User 专属缓存（全程指针，零拷贝） ---------------------------
const (
	// UserCachePrefix 用户缓存键前缀。
	UserCachePrefix    = "user"
	// ProductCachePrefix 商品缓存键前缀。
	ProductCachePrefix = "product"
	// DefaultExpire 通用缓存默认有效期，1小时。
	DefaultExpire      = 1 * time.Hour
	// ProductCacheExpire 商品缓存有效期，5小时。
	ProductCacheExpire = 5 * time.Hour
)

// UserCacheInterface 列出用户缓存的全部公开方法。
type UserCacheInterface interface {
	SetUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, userID int64) (*model.User, bool, error)
	DelUser(ctx context.Context, userID int64) error
}

type UserCache struct {
	// 用户缓存使用独立存储结构。
	genericCache GenericCache[*userRecord]
}

var _ UserCacheInterface = (*UserCache)(nil)

func NewUserCache() *UserCache {
	// 绑定指针类型，全程无值拷贝
	genericCache := NewRedisGenericCache[*userRecord](RedisClient)
	return &UserCache{
		genericCache: genericCache,
	}
}

// SetUser ：入参是 *model.User，直接传入（零拷贝）
func (h *UserCache) SetUser(ctx context.Context, user *model.User) error {
	if user == nil {
		return errors.New("user object is nil")
	}
	// 直接传入指针，无值拷贝
	return h.genericCache.Set(ctx, UserCachePrefix, user.ID, userToRecord(user), DefaultExpire)
}

// GetUser ：返回 *model.User，内部自动分配内存（零拷贝）
func (h *UserCache) GetUser(ctx context.Context, userID int64) (*model.User, bool, error) {
	record, hit, err := h.genericCache.Get(ctx, UserCachePrefix, userID)
	if err != nil || !hit || record == nil {
		return nil, false, err
	}
	user, err := record.model()
	return user, err == nil, err
}

func (h *UserCache) DelUser(ctx context.Context, userID int64) error {
	return h.genericCache.Del(ctx, UserCachePrefix, userID)
}

// ProductCacheInterface 列出商品缓存的全部公开方法。
type ProductCacheInterface interface {
	SetProduct(ctx context.Context, product *model.Product) error
	GetProduct(ctx context.Context, productID int64) (*model.Product, bool, error)
	DelProduct(ctx context.Context, productID int64) error
}

type ProductCache struct {
	genericCache GenericCache[*model.Product]
}

var _ ProductCacheInterface = (*ProductCache)(nil)

func NewProductCache() *ProductCache {
	return &ProductCache{genericCache: NewRedisGenericCache[*model.Product](RedisClient)}
}

func (c *ProductCache) SetProduct(ctx context.Context, product *model.Product) error {
	if product == nil {
		return errors.New("product object is nil")
	}
	return c.genericCache.Set(ctx, ProductCachePrefix, product.ID, product, ProductCacheExpire)
}

func (c *ProductCache) GetProduct(ctx context.Context, productID int64) (*model.Product, bool, error) {
	return c.genericCache.Get(ctx, ProductCachePrefix, productID)
}

func (c *ProductCache) DelProduct(ctx context.Context, productID int64) error {
	return c.genericCache.Del(ctx, ProductCachePrefix, productID)
}
