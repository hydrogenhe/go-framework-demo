package redis

import (
	"encoding/json"
	"go-framework-demo/model"
	"gorm.io/plugin/soft_delete"
	"strconv"
	"time"
)

// userRecord 是 Redis 存储协议，与 HTTP 协议独立。Number 可读取旧字符串 ID 和数字 ID。
// 不缓存密码哈希；鉴权从数据库读取密码。
type userRecord struct {
	ID         json.Number           `json:"id"`
	IsDel      soft_delete.DeletedAt `json:"is_del"`
	Name       string                `json:"name"`
	Age        int32                 `json:"age"`
	Email      string                `json:"email"`
	CreateTime time.Time             `json:"create_time"`
	UpdateTime time.Time             `json:"update_time"`
}

func userToRecord(u *model.User) *userRecord {
	return &userRecord{ID: json.Number(strconv.FormatInt(u.ID, 10)), IsDel: u.IsDel, Name: u.Name, Age: u.Age, Email: u.Email, CreateTime: u.CreateTime, UpdateTime: u.UpdateTime}
}
func (r *userRecord) model() (*model.User, error) {
	id, err := r.ID.Int64()
	if err != nil {
		return nil, err
	}
	return &model.User{ID: id, IsDel: r.IsDel, Name: r.Name, Age: r.Age, Email: r.Email, CreateTime: r.CreateTime, UpdateTime: r.UpdateTime}, nil
}
