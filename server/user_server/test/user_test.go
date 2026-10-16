package user

import (
	"context"
	"go-framework-demo/db"
	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"go-framework-demo/redis"
	"go-framework-demo/server/test/support"
	"go-framework-demo/server/user_server"
	"go-framework-demo/util"
	"testing"
	"time"

	require2 "github.com/stretchr/testify/require"
)

// TestUserTx 验证同一显式事务 Query 内创建和更新用户，提交后能够查询到新姓名。
func TestUserTx(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	totalStart := time.Now()
	defer func() {
		t.Logf("📊 测试总耗时：%v", time.Since(totalStart))
	}()
	createName := "hwt_" + util.GenerateAlnum(5)
	updateName := "hwt_" + util.GenerateAlnum(5)

	// 执行事务逻辑
	err := db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		// 1. 创建 UserDB（精简错误处理）
		dbStart := time.Now()
		userDB, err := db.NewUserDb(ctx, txQ)
		t.Logf("创建 UserDb 耗时：%v", time.Since(dbStart))
		if err != nil {
			t.Fatalf("创建 UserDb 失败：%v", err) // 终止测试，无需 return
		}

		// 2. 创建用户（精简错误处理）
		createStart := time.Now()
		user, err := userDB.Create(ctx, support.FixtureUser(t, createName, 30, util.RandomTestEmail(), util.GenerateAlnum(8)))
		if err != nil {
			return err
		}
		t.Logf("创建用户（ID：%d）耗时：%v", user.ID, time.Since(createStart))

		// 3. 更新用户（精简错误处理）
		updateStart := time.Now()
		mp := map[string]interface{}{"name": updateName}
		_, err = userDB.Update(ctx, user.ID, mp)
		if err != nil {
			t.Fatalf("更新用户失败：%v", err) // 终止测试，无需 return
		}
		t.Logf("更新用户（ID：%d）耗时：%v", user.ID, time.Since(updateStart))

		return nil // 仅无错误时执行，触发事务提交
	})

	// 全局错误处理（捕获事务回调外的错误，比如 WithTx 本身的错误）
	if err != nil {
		t.Errorf("事务执行失败：%s", err)
		return
	}

	// 验证结果
	verifyStart := time.Now()
	userDB, _ := db.NewUserDb(context.Background())
	user, err := userDB.GetByName(context.Background(), updateName)
	t.Logf("✅ 验证结果耗时：%v", time.Since(verifyStart))
	if err != nil {
		t.Errorf("查询更新后的用户失败：%v", err)
	} else if user == nil || user.Name != updateName {
		t.Errorf("用户更新失败，名称未改为 %s", updateName)
	}

	t.Log("✅ 事务测试成功：创建+更新均已持久化")
}

// TestUserCache 验证用户服务创建用户后可从缓存读取，并将同一用户写入 ES。
func TestUserCache(t *testing.T) {
	mysql.InitMySQL()
	ctx := context.Background()
	redis.Init()
	requ := require2.New(t)

	ser := user_server.NewUserService(ctx)
	name := "hwt_" + util.GenerateAlnum(5)
	usr, err := ser.CreateUser(ctx, name, 12, util.RandomTestEmail(), util.GenerateAlnum(8))
	requ.Nil(err)
	requ.NotNil(usr)

	// CreateUser 已同步写入 Redis；这里验证缓存读取，并将同一用户写入 ES。
	usr2, err := ser.GetUserByID(ctx, usr.ID)
	requ.Nil(err)
	requ.NotNil(usr2)
	requ.Equal(usr2.Name, name, "名称不一致")

	if err := es.InitES(); err != nil {
		t.Fatalf("初始化ES失败：%v", err)
	}
	esUser := &es_model.User{
		ID:         usr.ID,
		Name:       usr.Name,
		Age:        usr.Age,
		Email:      usr.Email,
		CreateTime: usr.CreateTime,
	}
	_, err = es.IndexUser(esUser)
	requ.Nil(err)

}
