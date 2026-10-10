package test

import (
	"context"
	"testing"

	"go-framework-demo/db"
	"go-framework-demo/mysql"
	"go-framework-demo/util"
)

func TestUserCreateAndGet(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	userDB, err := db.NewUserDb(ctx)
	if err != nil {
		t.Fatalf("初始化UserDb失败: %v", err)
	}

	name := "db_test_" + util.GenerateAlnum(8)
	created, err := userDB.Create(name, 30, "huapin")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if created.ID <= 0 {
		t.Fatal("创建用户后未生成有效ID")
	}

	found, err := userDB.GetByID(created.ID)
	if err != nil {
		t.Fatalf("按ID查询用户失败: %v", err)
	}
	if found.Name != name || found.Email != "huapin" {
		t.Fatalf("查询用户数据不一致: got=%+v", found)
	}
}
