package es_model

import (
	"errors"
	"time"
)

// User ES user索引对应的结构体（严格对齐ES映射）
// 字段标签说明：
// - json: 对应ES文档的字段名（必须与ES映射一致）
// - validate: 校验规则（必填/格式等）
type User struct {
	ID              int64     `json:"id" validate:"required,min=1"`           // 用户ID（必填，最小值1）
	Name            string    `json:"name" validate:"required,min=1,max=50"`  // 姓名（必填，1-50字符）
	Age             int32     `json:"age" validate:"omitempty,min=0,max=150"` // 年龄（可选，0-150）
	Email           string    `json:"email" validate:"required,email"`        // 邮箱（必填，格式校验）
	CreateTime      time.Time `json:"-" validate:"required"`                  // 创建时间（写入时使用）
	CreateTimeStamp int64     `json:"create_time_stamp"`                      // ES读取时使用时间戳
}

// Validate 结构体数据校验（保障完整性+格式准确性）
func (u *User) Validate() error {
	// 1. 基础必填字段校验（兜底，即使validate标签失效也能拦截）
	if u.ID <= 0 {
		return errors.New("用户ID不能为空且必须大于0")
	}
	if u.Name == "" || len(u.Name) > 50 {
		return errors.New("姓名不能为空且长度需1-50字符")
	}
	if u.Email == "" {
		return errors.New("邮箱不能为空")
	}
	// 2. 邮箱格式校验（简单版，也可使用第三方校验库）
	if !isValidEmail(u.Email) {
		return errors.New("邮箱格式错误")
	}
	// 3. 年龄范围校验
	if u.Age < 0 || u.Age > 150 {
		return errors.New("年龄需在0-150之间")
	}
	// 4. 创建时间校验
	if u.CreateTime.IsZero() {
		return errors.New("创建时间不能为空")
	}
	return nil
}

// ToMap 将结构体转为map[string]interface{}（兼容ES写入）
// 保障字段类型与ES映射完全一致（避免json序列化的类型隐式转换）
func (u *User) ToMap() (map[string]interface{}, error) {
	if err := u.Validate(); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"user_id":           u.ID,                               // int64 → ES long（精准匹配）
		"name":              u.Name,                             // string → ES keyword（精准匹配）
		"age":               u.Age,                              // int32 → ES integer（精准匹配）
		"email":             u.Email,                            // string → ES keyword（精准匹配）
		"create_time":       u.CreateTime.Format(time.DateTime), // time.Time → ES date（指定格式）
		"create_time_stamp": u.CreateTime.UnixMilli(),
	}, nil
}

// 辅助函数：简单邮箱格式校验
func isValidEmail(email string) bool {
	// 生产环境建议使用 github.com/go-playground/validator 等专业校验库
	atPos := -1
	dotPos := -1
	lastAtPos := len(email) - 1
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			atPos = i
		} else if email[i] == '.' && atPos != -1 && i > atPos && i < lastAtPos {
			dotPos = i
		}
	}
	return atPos != -1 && dotPos != -1
}
