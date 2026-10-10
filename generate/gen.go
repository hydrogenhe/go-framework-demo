package main

import (
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// 需要生成代码的表名（按需修改）
var needGenTables = []string{"user", "order", "product", "order_item"}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 步骤1：打印运行环境（仅排查用）
	log.Println("===== 运行环境信息 =====")
	wd, _ := os.Getwd()
	log.Printf("当前工作目录（项目根目录）：%s", wd)

	// 步骤2：连接数据库
	log.Println("\n===== 开始连接数据库 =====")
	dsn := "root:12345678@tcp(127.0.0.1:3306)/demo_db?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ 数据库连接失败：%v", err)
	}
	log.Println("✅ 数据库连接成功")

	// 步骤3：手动指定项目根目录（核心！替换为你的真实路径）
	// 示例：如果你的项目在 /Users/hewen/GoProjects/my-demo，就写这个路径
	// 也可以直接用当前工作目录（推荐，无需硬编码）：rootDir = wd
	rootDir := wd // 🌟 推荐：直接用当前运行目录作为项目根目录（最稳）
	queryPath := filepath.Join(rootDir, "query")
	modelPath := filepath.Join(rootDir, "model")
	log.Printf("✅ 项目根目录：%s", rootDir)
	log.Printf("✅ query 生成路径：%s", queryPath)
	log.Printf("✅ model 生成路径：%s", modelPath)

	// 步骤4：创建目录
	log.Println("\n===== 开始创建生成目录 =====")
	if err := os.MkdirAll(queryPath, 0755); err != nil {
		log.Fatalf("❌ 创建 query 目录失败：%v", err)
	}
	log.Printf("✅ query 目录创建成功：%s", queryPath)

	if err := os.MkdirAll(modelPath, 0755); err != nil {
		log.Fatalf("❌ 创建 model 目录失败：%v", err)
	}
	log.Printf("✅ model 目录创建成功：%s", modelPath)

	// 步骤5：初始化 Gen
	log.Println("\n===== 初始化 GORM Gen =====")
	g := gen.NewGenerator(gen.Config{
		OutPath:      queryPath,
		ModelPkgPath: modelPath,
		Mode:         gen.WithQueryInterface, // 适配你的 v0.3.x 版本
	})
	//g := gen.NewGenerator(gen.Config{
	//	OutPath:      "../query",
	//	ModelPkgPath: "../model",
	//	Mode:         gen.WithQueryInterface, // 适配你的 v0.3.x 版本
	//})

	g.UseDB(db)
	log.Println("✅ Gen 初始化成功")

	// 步骤6：加载表结构
	log.Println("\n===== 加载表结构 =====")
	var models []interface{}
	for _, table := range needGenTables {
		models = append(models, g.GenerateModel(table))
		log.Printf("✅ 已加载表：%s", table)
	}
	g.ApplyBasic(models...)
	// 方式2：生成所有表（注释方式1，取消下面注释）
	// g.ApplyBasic(g.GenerateAllTable()...)

	// 步骤7：执行生成（无返回值版本）
	log.Println("\n===== 执行代码生成 =====")
	g.Execute()
	log.Println("✅ Gen 执行生成完成")

	// 步骤8：验证结果
	log.Println("\n===== 验证生成结果 =====")
	queryFile := filepath.Join(queryPath, "gen.go")
	if _, err := os.Stat(queryFile); os.IsNotExist(err) {
		log.Fatalf(`❌ query.go 未生成！请检查：
		1. 数据库 demo_db 中是否有表：%v
		2. 生成路径：%s`, needGenTables, queryPath)
	}

	log.Printf("🎉 所有操作成功！query.go 已生成到：%s", queryFile)
}
