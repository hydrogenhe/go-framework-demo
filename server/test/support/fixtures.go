package support

import (
	"go-framework-demo/model"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

// FixtureUser 构造带 bcrypt 密码哈希的用户模型，供数据库测试准备数据。
func FixtureUser(t *testing.T, name string, age int32, email, password string) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &model.User{
		Name:       name,
		Age:        age,
		Email:      email,
		Password:   string(hash),
		CreateTime: time.Now(),
	}
}
