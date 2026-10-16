package mysql

import (
	"errors"
	"fmt"
	"go-framework-demo/util"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"sync"

	"go-framework-demo/query" // 导入gen生成的query包

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// MySQLConfig MySQL 连接配置结构体（保留写法，内部赋值）
type MySQLConfig = util.MySQLConfig

// 单例模式：确保全局只有一个 DB 实例 + gen 只初始化一次
var (
	genQuery      *query.Query // 替换原 DB/SetDefault 相关
	once          sync.Once
	db            *gorm.DB
	dbErr         error
	genInited     bool // 标记gen是否已初始化
	defaultConfig MySQLConfig
)

// InitMySQL 初始化 MySQL + 初始化 GORM Gen 查询器（统一入口）

func InitMySQL(configs ...MySQLConfig) error {
	once.Do(func() {
		if len(configs) > 0 {
			defaultConfig = configs[0]
		} else {
			configPath, err := util.ConfigFile("mysql.yaml")
			if err != nil {
				dbErr = err
				return
			}
			data, err := os.ReadFile(configPath)
			if err != nil {
				dbErr = err
				return
			}
			var fileConfig struct {
				MySQL MySQLConfig `yaml:"mysql"`
			}
			if err = yaml.Unmarshal(data, &fileConfig); err != nil {
				dbErr = fmt.Errorf("解析 MySQL 配置失败: %w", err)
				return
			}
			defaultConfig = fileConfig.MySQL
		}
		if defaultConfig.Host == "" || defaultConfig.Port == "" || defaultConfig.User == "" || defaultConfig.Password == "" || defaultConfig.DBName == "" {
			dbErr = errors.New("MySQL 配置必须提供 host、port、user、password 和 database")
			return
		}
		if _, err := strconv.Atoi(defaultConfig.Port); err != nil {
			dbErr = fmt.Errorf("MySQL port 无效: %w", err)
			return
		}
		if defaultConfig.Charset == "" {
			defaultConfig.Charset = "utf8mb4"
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local", defaultConfig.User, defaultConfig.Password, defaultConfig.Host, defaultConfig.Port, defaultConfig.DBName, defaultConfig.Charset)
		gormConfig := &gorm.Config{Logger: newZapGormLogger(gormlogger.Info)}
		db, dbErr = gorm.Open(mysql.Open(dsn), gormConfig)
		if dbErr != nil {
			dbErr = fmt.Errorf("连接 MySQL 失败: %w", dbErr)
			return
		}
		initGen(db)
		if !genInited || genQuery == nil {
			dbErr = errors.New("Gen 查询器初始化失败")
		}
	})
	return dbErr
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
