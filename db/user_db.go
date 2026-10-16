package db

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/mysql"

	"go-framework-demo/model"
	"go-framework-demo/query" // 必须导入根 query 包
)

// UserDbInterface 列出用户数据访问的全部公开方法。
type UserDbInterface interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByName(ctx context.Context, name string) (*model.User, error)
	Update(ctx context.Context, id int64, updateData map[string]interface{}) (*model.User, error)
	Delete(ctx context.Context, id int64) (bool, error)
	BatchCreateUsers(ctx context.Context, users []*model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	PageQuery(ctx context.Context, page int, pageSize int, keyword string) ([]*model.User, int64, error)
	GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.User, error)
}

type UserDb struct {
	GenQuery *query.Query // 核心：Gen 根查询器（绑定普通/事务 DB）
}

var _ UserDbInterface = (*UserDb)(nil)

// NewUserDb 初始化 - 移除原生 Db 字段，仅保留 GenQuery
func NewUserDb(ctx context.Context, txQ ...*query.Query) (*UserDb, error) {
	var genQuery *query.Query
	if len(txQ) > 0 && txQ[0] != nil {
		genQuery = txQ[0]
	}

	// 未传入事务查询器时使用全局 Query。
	if genQuery == nil {
		genQuery = mysql.GetGenQuery()
	}

	if genQuery == nil || !genQuery.Available() {
		return nil, errors.New("Gen 查询器实例为 nil 或未绑定有效 DB")
	}

	return &UserDb{
		GenQuery: genQuery,
	}, nil
}

// NewTxUserDb 创建事务版 UserDb（供 Gen 事务回调使用）
// --------------------------- 核心业务方法（仅调整 DB 依赖为 GenQuery） ---------------------------

// GetByID 查询 - 逻辑不变，仅调整上下文传递
func (u *UserDb) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if id <= 0 {
		return nil, errors.New("查询失败：用户ID必须大于0")
	}

	// 使用 Gen 根包的 User 常量构建条件，绑定上下文
	userQuery := u.GenQuery.User
	userDo := userQuery.WithContext(ctx)
	user, err := userDo.Where(userQuery.ID.Eq(id)).First()
	if err != nil {
		return nil, fmt.Errorf("查询用户失败 [系统错误]：%w（ID=%d）", err, id)
	}

	return user, nil
}

// GetByName 模糊查询 - 逻辑不变
func (u *UserDb) GetByName(ctx context.Context, name string) (*model.User, error) {
	if name == "" {
		return nil, errors.New("查询失败：用户名不能为空")
	}

	userQuery := u.GenQuery.User
	userDo := userQuery.WithContext(ctx)
	user, err := userDo.Where(userQuery.Name.Like("%" + name + "%")).First()
	if err != nil {
		return nil, fmt.Errorf("查询用户失败 [系统错误]：%w（名称=%s）", err, name)
	}

	return user, nil
}

func (u *UserDb) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if email == "" {
		return nil, errors.New("查询失败：邮箱不能为空")
	}
	userQuery := u.GenQuery.User
	user, err := userQuery.WithContext(ctx).Where(userQuery.Email.Eq(email)).First()
	if err != nil {
		return nil, fmt.Errorf("查询用户失败：%w", err)
	}
	return user, nil
}

// Update 更新 - 逻辑不变
func (u *UserDb) Update(ctx context.Context, id int64, updateData map[string]interface{}) (*model.User, error) {
	if id <= 0 {
		return nil, errors.New("更新失败：用户ID必须大于0")
	}
	if len(updateData) == 0 {
		return nil, errors.New("更新失败：请指定要修改的字段")
	}

	if _, err := u.GetByID(ctx, id); err != nil {
		return nil, err
	}

	userQuery := u.GenQuery.User
	userDo := userQuery.WithContext(ctx)
	_, err := userDo.Where(userQuery.ID.Eq(id)).Updates(updateData)
	if err != nil {
		return nil, fmt.Errorf("更新用户失败 [系统错误]：%w（ID=%d）", err, id)
	}

	return u.GetByID(ctx, id)
}

// Delete 删除 - 逻辑不变
func (u *UserDb) Delete(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, errors.New("删除失败：用户ID必须大于0")
	}

	if _, err := u.GetByID(ctx, id); err != nil {
		return false, err
	}

	userQuery := u.GenQuery.User
	userDo := userQuery.WithContext(ctx)
	result, err := userDo.Where(userQuery.ID.Eq(id)).Delete()
	if err != nil {
		return false, fmt.Errorf("删除用户失败 [系统错误]：%w（ID=%d）", err, id)
	}

	return result.RowsAffected > 0, nil
}

// Create 保存由业务层完成字段校验和密码哈希处理的用户模型。
func (u *UserDb) Create(ctx context.Context, user *model.User) (*model.User, error) {
	userQuery := u.GenQuery.User
	if err := userQuery.WithContext(ctx).Create(user); err != nil {
		return nil, fmt.Errorf("新增用户失败: %w", err)
	}
	return user, nil
}

// BatchCreateUsers 仅批量写入用户，校验和事务由调用方负责。
func (u *UserDb) BatchCreateUsers(ctx context.Context, users []*model.User) error {
	userQuery := u.GenQuery.User
	return userQuery.WithContext(ctx).Create(users...)
}

func (u *UserDb) PageQuery(ctx context.Context, page int, pageSize int, keyword string) ([]*model.User, int64, error) {
	// 1. 参数校验
	if page < 1 {
		return nil, 0, errors.New("分页查询失败：页码必须大于等于1")
	}
	if pageSize < 1 || pageSize > 100 { // 限制最大页大小，避免单次查询过多数据
		return nil, 0, errors.New("分页查询失败：页大小必须在1-100之间")
	}

	// 2. 构建查询条件（绑定上下文）
	userQuery := u.GenQuery.User
	userDo := userQuery.WithContext(ctx)
	queryBuilder := userDo

	// 3. 可选：用户名模糊查询
	if keyword != "" {
		queryBuilder = queryBuilder.Where(userQuery.Name.Like("%" + keyword + "%"))
	}

	// 4. 查询总条数（不限制分页）
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, fmt.Errorf("分页查询失败：统计总条数出错 %w", err)
	}

	// 5. 计算偏移量，查询当前页数据
	offset := (page - 1) * pageSize
	var users []*model.User
	users, err = queryBuilder.Limit(pageSize).Offset(offset).Find()
	if err != nil {
		return nil, 0, fmt.Errorf("分页查询失败：获取当前页数据出错 %w", err)
	}

	// 6. 返回结果（空列表也返回，避免nil）
	return users, total, nil
}

// GetByIDIncludingDeleted 仅供 CDC 读取最新状态，包含软删除记录。
func (u *UserDb) GetByIDIncludingDeleted(ctx context.Context, id int64) (*model.User, error) {
	q := u.GenQuery.User
	return q.WithContext(ctx).Unscoped().Where(q.ID.Eq(id)).First()
}
