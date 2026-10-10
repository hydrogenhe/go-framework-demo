package server

import (
	"context"
	"fmt"
	"go-framework-demo/db"
	"go-framework-demo/model"
	"go-framework-demo/query"
)

func TestUser() {

	TestTx2()
	//TestTx()
	//TestBatchTx()

}

func TestTx2() {
	ctx := context.Background()

	// 调用全局事务方法
	err := db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		// 核心修正：传入 ctx，自动从 ctx 取 txQ
		userDB, err := db.NewUserDb(ctx)
		if err != nil {
			return fmt.Errorf("创建 UserDb 失败：%v", err)
		}
		// 1. 事务内创建用户（基于 txQ）
		_, err = userDB.Create("hwt3", 30, "huapin")
		if err != nil {
			return fmt.Errorf("创建用户失败：%v", err)
		}

		// 2. 事务内创建订单（基于 txQ）
		orderDB, err := db.NewOrderDb(ctx)
		if err != nil {
			return fmt.Errorf("创建 orderDB 失败：%v", err)
		}
		_, err = orderDB.Create("o11111", 1, 66666)
		if err != nil {
			return fmt.Errorf("创建订单失败：%v", err)
		}

		return nil // 无错误则自动提交事务
	})

	// 错误处理
	if err != nil {
		fmt.Printf("事务执行失败：%s\n", err)
		return
	}
	fmt.Println("事务成功：创建+更新均已持久化")
}

func TestTx() {
	ctx := context.Background()

	// 调用全局事务方法
	err := db.WithTx(ctx, func(ctx context.Context, txQ *query.Query) error {
		// 核心修正：传入 ctx，自动从 ctx 取 txQ
		userDB, err := db.NewUserDb(ctx)
		if err != nil {
			return fmt.Errorf("创建 UserDb 失败：%v", err)
		}

		// 1. 事务内创建用户（基于 txQ）
		user, err := userDB.Create("hwt", 30, "huapin")
		if err != nil {
			return fmt.Errorf("创建用户失败：%v", err)
		}

		// 2. 事务内更新用户（基于 txQ）
		mp := map[string]interface{}{"name": "hwt2"}
		_, err = userDB.Update(user.ID, mp)
		if err != nil {
			return fmt.Errorf("更新用户失败：%v", err)
		}

		return nil // 无错误则自动提交事务
	})

	// 错误处理
	if err != nil {
		fmt.Printf("事务执行失败：%s\n", err)
		return
	}
	fmt.Println("事务成功：创建+更新均已持久化")
}

func TestBatchTx() {
	ctx := context.Background()

	userDB, _ := db.NewUserDb(ctx)
	userSli := []*model.User{
		&model.User{
			Name: "hwt2",
			Age:  30,
		},
		&model.User{
			Name: "hwt3",
			Age:  30,
		},
		&model.User{
			Name: "",
			Age:  30,
		},
	}
	err := userDB.BatchCreateUsers(userSli)
	if err != nil {
		fmt.Println(err)
		fmt.Sprintf("err=%s", err)
	}
}
