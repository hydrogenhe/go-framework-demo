package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

// 定义Redis常用返回值常量（枚举化）

const (
	RedisOK = "OK" // Set/Del等命令成功返回值
)

type FuncRedisOptions func(opt *redis.Options)

func WithAddr(addr string) FuncRedisOptions {
	return func(opt *redis.Options) {
		opt.Addr = addr
	}
}

func Init(conf ...func(option *redis.Options)) {
	// 1. 创建Redis客户端（连接本地Redis，默认端口6379，无密码）

	opt := &redis.Options{
		Addr:     "localhost:6379", // Redis地址（本地默认）
		Password: "",               // Redis密码（未设置则为空）
		DB:       0,
	}
	for _, f := range conf {
		f(opt)
	}

	client := redis.NewClient(opt)

	// 2. 验证连接是否成功
	ctx := context.Background() // 上下文，用于控制请求超时
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("连接Redis失败：%v", err))
	}
	fmt.Println("Redis连接成功：", pong) // 输出 PONG
	RedisClient = client
}
