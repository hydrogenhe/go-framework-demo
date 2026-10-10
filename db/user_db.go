package db

import (
	"context"
	"errors"
	"fmt"
	"go-framework-demo/mysql"
	"time"

	"go-framework-demo/model"
	"go-framework-demo/query" // 必须导入根 query 包

	"gorm.io/gorm"
)

// UserDbInterface 调整事务方法签名，适配 Gen 内置事务
type UserDbInterface interface {
	Create(name string, age int32, email string) (*model.User, error)
	GetByID(id int64) (*model.User, error)
	GetByName(name string) (*model.User, error)
	Update(id int64, updateData map[string]interface{}) (*model.User, error)
	Delete(id int64) (bool, error)
	BatchCreateUsers(users []*model.User) error
}

type UserDb struct {
	GenQuery *query.Query // 核心：Gen 根查询器（绑定普通/事务 DB）
}

// NewUserDb 初始化 - 移除原生 Db 字段，仅保留 GenQuery
func NewUserDb(ctx context.Context) (*UserDb, error) {
	// 1. 先从上下文取事务 txQ
	var genQuery *query.Query
	if v := ctx.Value(TxContextKey{}); v != nil {
		genQuery = v.(*query.Query)
	}

	// 2. 无则用全局 Query
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
func NewTxUserDb(txQ *query.Query) (*UserDb, error) {
	if txQ == nil || !txQ.Available() {
		return nil, errors.New("事务版 Gen 查询器无效")
	}
	return &UserDb{
		GenQuery: txQ,
	}, nil
}

// --------------------------- 核心业务方法（仅调整 DB 依赖为 GenQuery） ---------------------------

// GetByID 查询 - 逻辑不变，仅调整上下文传递
func (u *UserDb) GetByID(id int64) (*model.User, error) {
	if id <= 0 {
		return nil, errors.New("查询失败：用户ID必须大于0")
	}

	// 使用 Gen 根包的 User 常量构建条件，绑定上下文
	userDo := u.GenQuery.WithContext(context.Background()).User
	user, err := userDo.Where(u.GenQuery.User.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询失败：用户不存在（ID=%d）", id)
		}
		return nil, fmt.Errorf("查询用户失败 [系统错误]：%v（ID=%d）", err, id)
	}

	return user, nil
}

// GetByName 模糊查询 - 逻辑不变
func (u *UserDb) GetByName(name string) (*model.User, error) {
	if name == "" {
		return nil, errors.New("查询失败：用户名不能为空")
	}

	userDo := u.GenQuery.WithContext(context.Background()).User
	user, err := userDo.Where(u.GenQuery.User.Name.Like("%" + name + "%")).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询失败：未找到匹配用户（名称=%s）", name)
		}
		return nil, fmt.Errorf("查询用户失败 [系统错误]：%v（名称=%s）", err, name)
	}

	return user, nil
}

// Update 更新 - 逻辑不变
func (u *UserDb) Update(id int64, updateData map[string]interface{}) (*model.User, error) {
	if id <= 0 {
		return nil, errors.New("更新失败：用户ID必须大于0")
	}
	if len(updateData) == 0 {
		return nil, errors.New("更新失败：请指定要修改的字段")
	}

	if _, err := u.GetByID(id); err != nil {
		return nil, err
	}

	userDo := u.GenQuery.WithContext(context.Background()).User
	resultInfo, err := userDo.Where(u.GenQuery.User.ID.Eq(id)).Updates(updateData)
	if err != nil {
		return nil, fmt.Errorf("更新用户失败 [系统错误]：%v（ID=%d）", err, id)
	}

	if resultInfo.RowsAffected == 0 {
		return nil, fmt.Errorf("更新失败：无数据被修改（ID=%d，可能字段无变化或用户不存在）", id)
	}

	return u.GetByID(id)
}

// Delete 删除 - 逻辑不变
func (u *UserDb) Delete(id int64) (bool, error) {
	if id <= 0 {
		return false, errors.New("删除失败：用户ID必须大于0")
	}

	if _, err := u.GetByID(id); err != nil {
		return false, err
	}

	userDo := u.GenQuery.WithContext(context.Background()).User
	_, err := userDo.Where(u.GenQuery.User.ID.Eq(id)).Delete()
	if err != nil {
		return false, fmt.Errorf("删除用户失败 [系统错误]：%v（ID=%d）", err, id)
	}

	return true, nil
}

// Create 新增 - 逻辑不变
func (u *UserDb) Create(name string, age int32, email string) (*model.User, error) {
	if name == "" {
		return nil, errors.New("新增失败：用户名不能为空")
	}
	if age <= 0 {
		return nil, errors.New("新增失败：年龄必须大于0")
	}

	newUser := &model.User{
		Name:       name,
		Age:        age,
		Email:      email,
		CreateTime: time.Now(),
	}

	userDo := u.GenQuery.WithContext(context.Background()).User
	if err := userDo.Create(newUser); err != nil {
		return nil, fmt.Errorf("新增用户失败 [系统错误]：%v（用户信息：%+v）", err, newUser)
	}

	return newUser, nil
}

// BatchCreateUsers 批量新增 - 改用 Gen 内置事务
func (u *UserDb) BatchCreateUsers(users []*model.User) error {
	if len(users) == 0 {
		return errors.New("批量新增失败：用户列表为空")
	}

	// 调用 Gen 内置 Transaction 方法（核心改造点）
	err := u.GenQuery.Transaction(func(txQ *query.Query) error {
		// 创建事务版 UserDb
		txUserDb, err := NewTxUserDb(txQ)
		if err != nil {
			return fmt.Errorf("创建事务版 UserDb 失败：%v", err)
		}

		// 事务内批量插入
		for idx, user := range users {
			if user.Name == "" || user.Age <= 0 {
				return fmt.Errorf("第%d个用户信息无效：%+v", idx+1, user)
			}

			userDo := txUserDb.GenQuery.WithContext(context.Background()).User
			if err := userDo.Create(user); err != nil {
				return fmt.Errorf("第%d个用户新增失败：%v（用户信息：%+v）", idx+1, err, user)
			}
		}
		return nil // 无错误则自动提交事务
	})

	if err != nil {
		return fmt.Errorf("批量新增用户失败：%v（事务已回滚）", err)
	}
	return nil
}

func (u *UserDb) PageQuery(page int, pageSize int, keyword string) ([]*model.User, int64, error) {
	// 1. 参数校验
	if page < 1 {
		return nil, 0, errors.New("分页查询失败：页码必须大于等于1")
	}
	if pageSize < 1 || pageSize > 100 { // 限制最大页大小，避免单次查询过多数据
		return nil, 0, errors.New("分页查询失败：页大小必须在1-100之间")
	}

	// 2. 构建查询条件（绑定上下文）
	userDo := u.GenQuery.User.WithContext(context.Background())
	queryBuilder := userDo

	// 3. 可选：用户名模糊查询
	if keyword != "" {
		queryBuilder = queryBuilder.Where(u.GenQuery.User.Name.Like("%" + keyword + "%"))
	}

	// 4. 查询总条数（不限制分页）
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, fmt.Errorf("分页查询失败：统计总条数出错 %v", err)
	}

	// 5. 计算偏移量，查询当前页数据
	offset := (page - 1) * pageSize
	var users []*model.User
	users, err = queryBuilder.Limit(pageSize).Offset(offset).Find()
	if err != nil {
		return nil, 0, fmt.Errorf("分页查询失败：获取当前页数据出错 %v", err)
	}

	// 6. 返回结果（空列表也返回，避免nil）
	return users, total, nil
}
