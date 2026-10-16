package main

import (
	"fmt"
	"go-framework-demo/logger"
	"go-framework-demo/util"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

// 需要生成代码的表名（按需修改）
var needGenTables = []string{"user", "order", "product", "order_item", "product_version"}

func main() {

	// 步骤1：打印运行环境（仅排查用）
	logger.Sugar.Info("===== 运行环境信息 =====")
	wd, _ := os.Getwd()
	logger.Sugar.Infof("当前工作目录（项目根目录）：%s", wd)

	// 步骤2：连接数据库
	logger.Sugar.Info("\n===== 开始连接数据库 =====")
	configPath, err := util.ConfigFile("mysql.yaml")
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	var fileConfig struct {
		MySQL struct {
			Host     string `yaml:"host"`
			Port     string `yaml:"port"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			Database string `yaml:"database"`
			Charset  string `yaml:"charset"`
		} `yaml:"mysql"`
	}
	if err := yaml.Unmarshal(configData, &fileConfig); err != nil {
		logger.Sugar.Fatal(err)
	}
	cfg := fileConfig.MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Charset)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Sugar.Fatalf("❌ 数据库连接失败：%v", err)
	}
	logger.Sugar.Info("✅ 数据库连接成功")

	// 步骤3：手动指定项目根目录（核心！替换为你的真实路径）
	// 示例：如果你的项目在 /Users/hewen/GoProjects/my-demo，就写这个路径
	// 也可以直接用当前工作目录（推荐，无需硬编码）：rootDir = wd
	rootDir := wd // 🌟 推荐：直接用当前运行目录作为项目根目录（最稳）
	queryPath := filepath.Join(rootDir, "query")
	modelPath := filepath.Join(rootDir, "model")
	logger.Sugar.Infof("✅ 项目根目录：%s", rootDir)
	logger.Sugar.Infof("✅ query 生成路径：%s", queryPath)
	logger.Sugar.Infof("✅ model 生成路径：%s", modelPath)

	// 步骤4：创建目录
	logger.Sugar.Info("\n===== 开始创建生成目录 =====")
	if err := os.MkdirAll(queryPath, 0755); err != nil {
		logger.Sugar.Fatalf("❌ 创建 query 目录失败：%v", err)
	}
	logger.Sugar.Infof("✅ query 目录创建成功：%s", queryPath)

	if err := os.MkdirAll(modelPath, 0755); err != nil {
		logger.Sugar.Fatalf("❌ 创建 model 目录失败：%v", err)
	}
	logger.Sugar.Infof("✅ model 目录创建成功：%s", modelPath)

	// 步骤5：初始化 Gen
	logger.Sugar.Info("\n===== 初始化 GORM Gen =====")
	config := gen.Config{
		OutPath:      queryPath,
		ModelPkgPath: modelPath,
		Mode:         gen.WithQueryInterface,
	}
	g := gen.NewGenerator(config)
	//g := gen.NewGenerator(gen.Config{
	//	OutPath:      "../query",
	//	ModelPkgPath: "../model",
	//	Mode:         gen.WithQueryInterface, // 适配你的 v0.3.x 版本
	//})

	g.UseDB(db)
	logger.Sugar.Info("✅ Gen 初始化成功")

	// 步骤6：加载表结构
	logger.Sugar.Info("\n===== 加载表结构 =====")
	// 所有业务表都从数据库结构生成 Model 和 Query。is_del 的类型及软删除标签通过
	// Gen 字段选项配置；model.DeletedAt 是对插件类型的别名，避免手改生成文件。
	models := make([]interface{}, 0, len(needGenTables))
	for _, tableName := range needGenTables {
		models = append(models, g.GenerateModel(tableName,
			// 将数据库 is_del 列生成为软删除插件类型的别名 model.DeletedAt。
			gen.FieldType("is_del", "DeletedAt"),
			// 使用 flag 模式：0 表示未删除，非 0 表示已删除；插件会据此过滤查询并软删除。
			gen.FieldGORMTag("is_del", func(tag field.GormTag) field.GormTag {
				return tag.Set("softDelete", "flag")
			}),
		))
	}
	g.ApplyBasic(models...)
	// 方式2：生成所有表（注释方式1，取消下面注释）
	// g.ApplyBasic(g.GenerateAllTable()...)

	// 步骤7：执行生成（无返回值版本）
	logger.Sugar.Info("\n===== 执行代码生成 =====")
	g.Execute()
	logger.Sugar.Info("✅ Gen 执行生成完成")

	// 步骤8：验证结果
	logger.Sugar.Info("\n===== 验证生成结果 =====")
	queryFile := filepath.Join(queryPath, "gen.go")
	if _, err := os.Stat(queryFile); os.IsNotExist(err) {
		logger.Sugar.Fatalf(`❌ query.go 未生成！请检查：
		1. 数据库 demo_db 中是否有表：%v
		2. 生成路径：%s`, needGenTables, queryPath)
	}

	logger.Sugar.Infof("🎉 所有操作成功！query.go 已生成到：%s", queryFile)
}
