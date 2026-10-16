package user_server

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/logger"
	"strings"

	mysqlerr "github.com/go-sql-driver/mysql"
	appconst "go-framework-demo/const"
	"go-framework-demo/db"    // 你的DB层包
	"go-framework-demo/model" // 你的模型包
	"go-framework-demo/query"
	"go-framework-demo/redis" // 你的Redis缓存包
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/mail"
	"time"
	"unicode/utf8"
)

// UserServiceInterface 列出用户业务的全部公开方法。
type UserServiceInterface interface {
	CreateModel(ctx context.Context, profile *model.User, plainPassword string) (*model.User, error)
	Authenticate(ctx context.Context, email, password string) (*model.User, error)
	CreateUser(ctx context.Context, name string, age int32, email string, password ...string) (*model.User, error)
	GetUserByID(ctx context.Context, userID int64) (*model.User, error)
	UpdateUser(ctx context.Context, userID int64, updateData map[string]interface{}) (*model.User, error)
	DeleteUser(ctx context.Context, userID int64) (bool, error)
}

// UserService User业务逻辑层（Server层）
// 核心：依赖DB层 + 模型专属缓存Helper，不直接依赖泛型层
type UserService struct {
	userDb    *db.UserDb       // 原有DB层实例，完全复用
	userCache *redis.UserCache // User专属缓存Helper（封装了通用缓存逻辑）
}

var _ UserServiceInterface = (*UserService)(nil)

func (s *UserService) Authenticate(ctx context.Context, email, password string) (*model.User, error) {
	if s == nil || s.userDb == nil {
		return nil, appconst.NewInternalError(errors.New("user service is not initialized"))
	}
	if strings.TrimSpace(email) == "" || password == "" {
		return nil, appconst.NewUnauthorizedError("邮箱或密码错误")
	}
	u, err := s.userDb.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appconst.NewUnauthorizedError("邮箱或密码错误")
		}
		logger.Error(ctx, "查询登录用户失败", "email", email, "error", err)
		return nil, appconst.NewInternalError(fmt.Errorf("authenticate user: %w", err))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, appconst.NewUnauthorizedError("邮箱或密码错误")
	}
	return u, nil
}

// NewUserService 初始化UserService
// 入参：ctx 上下文（用于DB/缓存初始化）
// 出参：UserService实例 / 错误
func NewUserService(ctx context.Context, txQ ...*query.Query) *UserService {
	// 1. 初始化原有DB层（完全不动你的DB层代码）
	userDb, _ := db.NewUserDb(ctx, txQ...)

	// 2. 初始化User专属缓存Helper（内部复用通用缓存+外部RedisClient）
	var userCache *redis.UserCache
	if redis.RedisClient != nil && (len(txQ) == 0 || txQ[0] == nil) {
		userCache = redis.NewUserCache()
	}

	return &UserService{
		userDb:    userDb,
		userCache: userCache,
	}
}

// CreateUser 创建用户
// 入参：ctx 上下文，name/age/email 用户信息
// 出参：创建后的用户对象 / 错误
func (s *UserService) CreateUser(ctx context.Context, name string, age int32, email string, password ...string) (*model.User, error) {
	plainPassword := ""
	if len(password) > 0 {
		plainPassword = password[0]
	}
	profile := &model.User{
		Name:  name,
		Age:   age,
		Email: email,
	}
	return s.CreateModel(ctx, profile, plainPassword)
}

// CreateModel 接收用户资料模型；明文密码不写入模型，校验后生成哈希。
func (s *UserService) CreateModel(ctx context.Context, profile *model.User, plainPassword string) (*model.User, error) {
	if profile == nil {
		return nil, appconst.NewInvalidParamsError("用户资料不能为空")
	}
	name, age, email := profile.Name, profile.Age, profile.Email
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 50 {
		return nil, appconst.NewInvalidParamsError("用户名不能为空且不能超过50位")
	}
	if age <= 0 {
		return nil, appconst.NewInvalidParamsError("姓名、邮箱和年龄不能为空且必须有效")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if address, err := mail.ParseAddress(email); err != nil || address.Address != email || len(email) > 100 {
		return nil, appconst.NewInvalidParamsError("邮箱格式不正确")
	}
	length := utf8.RuneCountInString(plainPassword)
	if length < 6 || length > 18 {
		return nil, appconst.NewInvalidParamsError("密码长度必须为6-18位")
	}
	if s == nil || s.userDb == nil {
		return nil, appconst.NewInternalError(errors.New("user service is not initialized"))
	}
	_, err := s.userDb.GetByEmail(ctx, email)
	if err == nil {
		return nil, appconst.NewConflictError("邮箱已注册")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appconst.NewInternalError(err)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	input := &model.User{
		Name:       name,
		Age:        age,
		Email:      email,
		Password:   string(hashed),
		CreateTime: time.Now(),
	}
	user, err := s.userDb.Create(ctx, input)
	if err != nil {
		logger.Error(ctx, "DB创建用户失败", "name", name, "error", err)
		return nil, userServiceError(err)
	}

	// 2. 调用专属缓存Helper写入缓存（无需关心泛型/反射，只调简单方法）
	if s.userCache != nil {
		if err := s.userCache.SetUser(ctx, user); err != nil {
			// 缓存写入失败不阻塞业务，仅打日志（生产环境建议告警）
			logger.Error(ctx, "写入User缓存失败", "user_id", user.ID, "error", err)
		}

	}
	return user, nil
}

// GetUserByID 根据ID查询用户
// 核心逻辑：先查缓存 → 缓存未命中查DB → DB查到后回写缓存
func (s *UserService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	if s == nil || s.userDb == nil {
		return nil, appconst.NewInternalError(errors.New("user service is not initialized"))
	}
	// 入参校验
	if userID <= 0 {
		return nil, appconst.NewInvalidParamsError("用户ID必须为正整数")
	}

	// 1. 优先查缓存（调用专属Helper，返回具体*model.User，调试直观）
	if s.userCache != nil {
		cacheUser, hit, err := s.userCache.GetUser(ctx, userID)
		if err != nil {
			// 缓存查询异常（如Redis宕机），降级查DB，打日志告警
			logger.Error(ctx, "查询User缓存异常，降级查DB", "user_id", userID, "error", err)
		} else if hit && cacheUser != nil && cacheUser.IsDel == 0 {
			// 缓存可能先于 CDC 失效；校验数据库状态，软删除用户不得继续返回。
			if _, err := s.userDb.GetByID(ctx, userID); err != nil {
				return nil, userServiceError(err)
			}
			return cacheUser, nil
		}

	}
	// 2. 缓存未命中/异常，查DB层
	dbUser, err := s.userDb.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "DB查询用户失败", "user_id", userID, "error", err)
		return nil, userServiceError(err)
	}

	// 3. DB查到后，回写缓存（预热缓存，提升下次查询性能）
	if s.userCache != nil {
		if err := s.userCache.SetUser(ctx, dbUser); err != nil {
			logger.Error(ctx, "回写User缓存失败", "user_id", dbUser.ID, "error", err)
		}

	}
	return dbUser, nil
}

// UpdateUser 更新用户信息
// 核心逻辑：更新DB → 删除缓存（避免缓存脏数据）
func (s *UserService) UpdateUser(ctx context.Context, userID int64, updateData map[string]interface{}) (*model.User, error) {
	if s == nil || s.userDb == nil {
		return nil, appconst.NewInternalError(errors.New("user service is not initialized"))
	}
	// 入参校验
	if userID <= 0 || len(updateData) == 0 {
		return nil, appconst.NewInvalidParamsError("用户ID必须为正整数且更新字段不能为空")
	}

	// 1. 调用DB层更新用户
	updatedUser, err := s.userDb.Update(ctx, userID, updateData)
	if err != nil {
		logger.Error(ctx, "DB更新用户失败", "user_id", userID, "error", err)
		return nil, userServiceError(err)
	}

	// 2. 删除缓存（更新操作必须删缓存，避免脏数据）
	if s.userCache != nil {
		if err := s.userCache.DelUser(ctx, userID); err != nil {
			logger.Error(ctx, "删除User缓存失败", "user_id", userID, "error", err)
		}

	}
	return updatedUser, nil
}

// DeleteUser 删除用户
// 核心逻辑：删除DB → 删除缓存
func (s *UserService) DeleteUser(ctx context.Context, userID int64) (bool, error) {
	if s == nil || s.userDb == nil {
		return false, appconst.NewInternalError(errors.New("user service is not initialized"))
	}
	// 入参校验
	if userID <= 0 {
		return false, appconst.NewInvalidParamsError("用户ID必须为正整数")
	}

	// 1. 调用DB层删除用户
	ok, err := s.userDb.Delete(ctx, userID)
	if err != nil {
		logger.Error(ctx, "DB删除用户失败", "user_id", userID, "error", err)
		return false, userServiceError(err)
	}

	if !ok {
		return false, appconst.NewNotFoundError("用户不存在")
	}

	// 2. 删除缓存
	if s.userCache != nil {
		if err := s.userCache.DelUser(ctx, userID); err != nil {
			logger.Error(ctx, "删除User缓存失败", "user_id", userID, "error", err)
		}

	}
	return true, nil
}

// userServiceError 根据数据库错误类型明确表达用户业务结果。
func userServiceError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appconst.NewNotFoundError("用户不存在")
	}
	var duplicate *mysqlerr.MySQLError
	if errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062) {
		return appconst.NewConflictError("用户信息已存在")
	}
	return appconst.NewInternalError(err)
}
