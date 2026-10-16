package test

import (
	"context"
	"testing"

	"go-framework-demo/db"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
	"golang.org/x/crypto/bcrypt"
)

// TestUserCreateAndGet 使用真实 MySQL，验证用户持久化、自动生成 ID 及预先生成的密码哈希正确保存。
func TestUserCreateAndGet(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	userDB, err := db.NewUserDb(ctx)
	if err != nil {
		t.Fatalf("初始化UserDb失败: %v", err)
	}

	name := "db_test_" + util.GenerateAlnum(8)
	email, password := util.RandomTestEmail(), util.GenerateAlnum(8)
	created, err := userDB.Create(ctx, fixtureUser(t, name, 30, email, password))
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if created.ID <= 0 {
		t.Fatal("创建用户后未生成有效ID")
	}

	found, err := userDB.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("按ID查询用户失败: %v", err)
	}
	if found.Name != name || found.Email != email {
		t.Fatalf("查询用户数据不一致: got=%+v", found)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(found.Password), []byte(password)); err != nil {
		t.Fatalf("密码未按 bcrypt 存储: %q", found.Password)
	}
}
