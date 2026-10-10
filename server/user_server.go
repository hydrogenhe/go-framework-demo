package server

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go-framework-demo/db"    // 你的DB层包
	"go-framework-demo/model" // 你的模型包
	"go-framework-demo/redis" // 你的Redis缓存包
)

// UserService User业务逻辑层（Server层）
// 核心：依赖DB层 + 模型专属缓存Helper，不直接依赖泛型层
type UserService struct {
	userDb    *db.UserDb       // 原有DB层实例，完全复用
	userCache *redis.UserCache // User专属缓存Helper（封装了通用缓存逻辑）
}

// NewUserService 初始化UserService
// 入参：ctx 上下文（用于DB/缓存初始化）
// 出参：UserService实例 / 错误
func NewUserService(ctx context.Context) *UserService {
	// 1. 初始化原有DB层（完全不动你的DB层代码）
	userDb, _ := db.NewUserDb(ctx)

	// 2. 初始化User专属缓存Helper（内部复用通用缓存+外部RedisClient）
	userCache := redis.NewUserCache()

	return &UserService{
		userDb:    userDb,
		userCache: userCache,
	}
}

// CreateUser 创建用户
// 入参：ctx 上下文，name/age/email 用户信息
// 出参：创建后的用户对象 / 错误
func (s *UserService) CreateUser(ctx context.Context, name string, age int32, email string) (*model.User, error) {
	// 入参校验（业务层基础校验）
	if name == "" || email == "" || age <= 0 {
		return nil, errors.New("invalid user params: name/email/age can't be empty or negative")
	}

	// 1. 调用DB层创建用户（复用原有逻辑，无入侵）
	user, err := s.userDb.Create(name, age, email)
	if err != nil {
		log.Printf("DB创建用户失败: name=%s, err=%v", name, err)
		return nil, fmt.Errorf("create user db failed: %w", err)
	}

	// 2. 调用专属缓存Helper写入缓存（无需关心泛型/反射，只调简单方法）
	if err := s.userCache.SetUser(ctx, user); err != nil {
		// 缓存写入失败不阻塞业务，仅打日志（生产环境建议告警）
		log.Printf("写入User缓存失败: userID=%d, err=%v", user.ID, err)
	}

	return user, nil
}

// GetUserByID 根据ID查询用户
// 核心逻辑：先查缓存 → 缓存未命中查DB → DB查到后回写缓存
func (s *UserService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	// 入参校验
	if userID <= 0 {
		return nil, errors.New("invalid user id: id must be positive")
	}

	// 1. 优先查缓存（调用专属Helper，返回具体*model.User，调试直观）
	cacheUser, hit, err := s.userCache.GetUser(ctx, userID)
	if err != nil {
		// 缓存查询异常（如Redis宕机），降级查DB，打日志告警
		log.Printf("查询User缓存异常，降级查DB: userID=%d, err=%v", userID, err)
	} else if hit {
		// 缓存命中，直接返回（无需走DB，性能最优）
		return cacheUser, nil
	}

	// 2. 缓存未命中/异常，查DB层
	dbUser, err := s.userDb.GetByID(userID)
	if err != nil {
		log.Printf("DB查询用户失败: userID=%d, err=%v", userID, err)
		return nil, fmt.Errorf("get user db failed: %w", err)
	}

	// 3. DB查到后，回写缓存（预热缓存，提升下次查询性能）
	if err := s.userCache.SetUser(ctx, dbUser); err != nil {
		log.Printf("回写User缓存失败: userID=%d, err=%v", dbUser.ID, err)
	}

	return dbUser, nil
}

// UpdateUser 更新用户信息
// 核心逻辑：更新DB → 删除缓存（避免缓存脏数据）
func (s *UserService) UpdateUser(ctx context.Context, userID int64, updateData map[string]interface{}) (*model.User, error) {
	// 入参校验
	if userID <= 0 || len(updateData) == 0 {
		return nil, errors.New("invalid update params: id must be positive, updateData can't be empty")
	}

	// 1. 调用DB层更新用户
	updatedUser, err := s.userDb.Update(userID, updateData)
	if err != nil {
		log.Printf("DB更新用户失败: userID=%d, err=%v", userID, err)
		return nil, fmt.Errorf("update user db failed: %w", err)
	}

	// 2. 删除缓存（更新操作必须删缓存，避免脏数据）
	if err := s.userCache.DelUser(ctx, userID); err != nil {
		log.Printf("删除User缓存失败: userID=%d, err=%v", userID, err)
	}

	return updatedUser, nil
}

// DeleteUser 删除用户
// 核心逻辑：删除DB → 删除缓存
func (s *UserService) DeleteUser(ctx context.Context, userID int64) (bool, error) {
	// 入参校验
	if userID <= 0 {
		return false, errors.New("invalid user id: id must be positive")
	}

	// 1. 调用DB层删除用户
	ok, err := s.userDb.Delete(userID)
	if err != nil || !ok {
		log.Printf("DB删除用户失败: userID=%d, err=%v", userID, err)
		return false, fmt.Errorf("delete user db failed: %w", err)
	}

	// 2. 删除缓存
	if err := s.userCache.DelUser(ctx, userID); err != nil {
		log.Printf("删除User缓存失败: userID=%d, err=%v", userID, err)
	}

	return true, nil
}
