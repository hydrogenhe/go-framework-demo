package mysql

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"go-framework-demo/query" // 导入gen生成的query包

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MySQLConfig MySQL 连接配置结构体（保留写法，内部赋值）
type MySQLConfig struct {
	Host     string // 数据库地址
	Port     string // 端口
	User     string // 用户名
	Password string // 密码
	DBName   string // 数据库名
	Charset  string // 字符集
}

// 单例模式：确保全局只有一个 DB 实例 + gen 只初始化一次
var (
	genQuery  *query.Query // 替换原 DB/SetDefault 相关
	once      sync.Once
	db        *gorm.DB
	dbErr     error
	genInited bool // 标记gen是否已初始化
	// 内部写死配置：初始化时自动赋值给 MySQLConfig
	defaultConfig = MySQLConfig{
		Host:     "127.0.0.1", // 固定地址
		Port:     "3306",      // 固定端口
		User:     "root",      // 固定用户名
		Password: "12345678",  // 你的 MySQL 密码（仅需修改这里！）
		DBName:   "demo_db",   // 你的数据库名（仅需修改这里！）
		Charset:  "utf8mb4",   // 固定字符集
	}
)

// InitMySQL 初始化 MySQL + 初始化 GORM Gen 查询器（统一入口）

func InitMySQL() {
	once.Do(func() {
		// 1. 校验配置
		if defaultConfig.Password == "" {
			log.Fatal("MySQL 初始化失败：密码未配置，请修改 defaultConfig 中的 Password 字段")
		}
		if defaultConfig.DBName == "" {
			log.Fatal("MySQL 初始化失败：数据库名未配置，请修改 defaultConfig 中的 DBName 字段")
		}

		// 2. 构建 DSN 并连接 MySQL
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
			defaultConfig.User, defaultConfig.Password,
			defaultConfig.Host, defaultConfig.Port,
			defaultConfig.DBName, defaultConfig.Charset,
		)
		var err error
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			log.Fatalf("MySQL 连接失败：%v", err) // 直接终止
		}

		// 3. 初始化 Gen 查询器
		initGen(db)
		if !genInited || genQuery == nil {
			log.Fatal("Gen 查询器初始化失败")
		}
	})
}

// initGen 初始化 GORM Gen 查询器（内部方法）
func initGen(db *gorm.DB) {
	if genInited {
		return // 避免重复初始化
	}
	// 新版 Gen 核心：调用 Use 方法创建 Query 实例
	genQuery = query.Use(db)
	if genQuery == nil {
		dbErr = errors.New("gen 查询器初始化失败：query.Use(db) 返回 nil")
		return
	}
	genInited = true
}

// GetGenQuery 【新增】获取全局 Gen Query 实例
func GetGenQuery() *query.Query {
	return genQuery
}

func GetDB() *gorm.DB {
	return db
}

// ResetDB 【新增】重置 genQuery
func ResetDB() {
	once = sync.Once{}
	db = nil
	dbErr = nil
	genQuery = nil    // 新增
	genInited = false // 新增
}

// GetDefaultConfig 获取内部写死的默认配置（可选，便于调试）
func GetDefaultConfig() MySQLConfig {
	return defaultConfig
}
