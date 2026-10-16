package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8" // 正确路径

	require2 "github.com/stretchr/testify/require"
)

// TestRedisKV 使用真实 Redis，验证键值读写、删除、空字符串和键不存在时的返回值。
func TestRedisKV(t *testing.T) {
	Init()

	// 创建断言器，简化所有断言逻辑

	require := require2.New(t)
	// 带超时的上下文（避免测试卡住）
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	ctx := context.Background()

	// 测试数据（加时间戳进一步避免冲突）
	testKey := "test_redis_key_" + time.Now().Format("20060102150405")
	testValue := "测试值"
	emptyValue := "" // 空字符串值（新增测试场景）
	expireTime := 10 * time.Second

	// ========== 前置清理：确保测试key初始不存在 ==========
	_, err := RedisClient.Del(ctx, testKey).Result()
	require.NoError(err, "前置清理测试key失败")

	// ========== 测试0：获取不存在的key（预期返回redis.Nil） ==========
	_, err = RedisClient.Get(ctx, testKey).Result()
	require.ErrorIs(err, redis.Nil, "获取不存在的key应该返回redis.Nil，实际错误：%v", err)

	// ========== 测试1：设置key ==========
	err = RedisClient.Set(ctx, testKey, testValue, expireTime).Err()
	require.NoError(err, "设置key失败") // 断言无错误

	// ========== 测试2：获取key ==========
	val, err := RedisClient.Get(ctx, testKey).Result()
	require.NoError(err, "获取key失败")            // 断言无错误（排除所有错误，包括redis.Nil）
	require.NotNil(val, "获取的key值不应为nil")       // 断言值非nil（补充nil判断）
	require.Equal(testValue, val, "获取的值不符合预期") // 断言值相等

	// ========== 测试3：检查key是否存在 ==========
	exists, err := RedisClient.Exists(ctx, testKey).Result()
	require.NoError(err, "检查key是否存在失败")
	require.Equal(int64(1), exists, "key应该存在") // 断言存在

	// ========== 测试4：查看过期时间 ==========
	ttl, err := RedisClient.TTL(ctx, testKey).Result()
	require.NoError(err, "查看过期时间失败")
	require.True(ttl > 0, "过期时间设置失败，剩余时间：%v", ttl) // 断言过期时间>0

	// ========== 新增测试：设置空字符串值并验证 ==========
	err = RedisClient.Set(ctx, testKey, emptyValue, expireTime).Err()
	require.NoError(err, "设置空字符串值失败")
	valEmpty, err := RedisClient.Get(ctx, testKey).Result()
	require.NoError(err, "获取空字符串值失败")                 // 空字符串不是redis.Nil，无错误
	require.Equal(emptyValue, valEmpty, "空字符串值不符合预期") // 断言值为空字符串
	require.NotEqual(redis.Nil, err, "获取空字符串key不应返回redis.Nil")

	// ========== 测试5：删除key ==========
	delCount, err := RedisClient.Del(ctx, testKey).Result()
	require.NoError(err, "删除key失败")
	require.Equal(int64(1), delCount, "删除key数量不符合预期")

	// ========== 测试6：获取已删除的key（重点验证redis.Nil） ==========
	delVal, err := RedisClient.Get(ctx, testKey).Result()
	require.ErrorIs(err, redis.Nil, "预期key不存在（redis.Nil），实际错误：%v", err)
	require.Empty(delVal, "已删除key的返回值应为空") // 补充断言返回值为空

	// ========== 测试7：批量获取存在/不存在的key（混合nil场景） ==========
	// 先设置一个存在的key
	_ = RedisClient.Set(ctx, "exist_key", "exist_val", 0).Err()
	// 批量获取：存在的key + 不存在的key
	vals, err := RedisClient.MGet(ctx, "exist_key", "non_exist_key").Result()
	require.NoError(err, "批量获取key失败") // MGet不会返回redis.Nil，不存在的key返回nil
	require.Equal("exist_val", vals[0], "存在的key值不符合预期")
	require.Nil(vals[1], "不存在的key值应返回nil") // 断言不存在的key值为nil

	// 后置清理
	RedisClient.Del(ctx, "exist_key").Result()

	t.Log("所有Redis KV测试用例执行成功！")
}

// TestRedisSetNX_WithExpire 验证 Redis 的 NX 只在键不存在时写入，并检查有效期及过期后的再次写入。
func TestRedisSetNX_WithExpire(t *testing.T) {
	Init()
	requ := require2.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	// 改用更紧凑的时间格式，避免特殊字符导致key识别异常
	testKey := "test_setnx_expire_key_" + time.Now().Format("20060102150405")
	testValue := "expire_value"
	expireTime := 3 * time.Second

	// ========== 前置清理（加断言，确保清理成功） ==========
	delCnt, err := RedisClient.Del(ctx, testKey).Result()
	requ.NoError(err, "前置清理key失败")
	t.Logf("前置清理：删除key数量=%d", delCnt)

	// ========== 第一次Set NX（成功） ==========
	setOptions := redis.SetArgs{
		TTL:  expireTime,
		Mode: "NX",
	}
	ok, err := RedisClient.SetArgs(ctx, testKey, testValue, setOptions).Result()
	requ.NoError(err, "第一次Set NX失败")
	requ.Equal(RedisOK, ok, "第一次Set NX应返回OK，实际返回=%s", ok)
	t.Logf("第一次Set NX：ok=%s", ok)

	// ========== 验证key存在+TTL（加日志，确认状态） ==========
	// 1. 检查key存在性
	exists, err := RedisClient.Exists(ctx, testKey).Result()
	requ.NoError(err, "检查key存在性失败")
	requ.Equal(int64(1), exists, "key应存在，实际exists=%d", exists)

	// 2. 验证TTL
	ttl, err := RedisClient.TTL(ctx, testKey).Result()
	requ.NoError(err, "获取TTL失败")
	requ.True(ttl > 0 && ttl <= expireTime, "TTL异常：%v（预期0<%v<=%v）", ttl, ttl, expireTime)
	t.Logf("验证TTL：剩余时间=%v", ttl)

	// 3. 获取key值
	val, err := RedisClient.Get(ctx, testKey).Result()
	requ.NoError(err, "Get key失败（redis: nil说明key不存在）")
	requ.NotEmpty(val, "key值为空")
	requ.Equal(testValue, val, "值不匹配：预期=%s，实际=%s", testValue, val)
	t.Logf("Get key：值=%s", val)

	// ========== 第二次Set NX（key已存在，预期失败） ==========
	updateOptions := redis.SetArgs{
		TTL:  expireTime,
		Mode: "NX",
	}
	ok, err = RedisClient.SetArgs(ctx, testKey, testValue, updateOptions).Result()

	// 核心1：NX失败时err必为nil，断言失败说明有真正的Redis异常
	if errors.Is(err, redis.Nil) {
		//兼容写法
		err = nil
	}
	requ.NoError(err, "第二次Set NX执行异常（非key存在原因）")
	// 核心2：断言ok为空字符串（NX失败的唯一标准），比ok!=RedisOK更精准
	requ.Empty(ok, "第二次Set NX应返回空字符串，实际返回=%s", ok)
	t.Logf("第二次Set NX：ok=%s", ok)

	// ========== 等待过期后Set NX（成功） ==========
	// 加100ms避免Redis过期时间的毫秒级误差
	time.Sleep(expireTime + 100*time.Millisecond)
	t.Log("等待key过期完成")

	keepTTLOptions := redis.SetArgs{
		TTL:  expireTime,
		Mode: "NX",
	}
	ok, err = RedisClient.SetArgs(ctx, testKey, testValue, keepTTLOptions).Result()
	requ.NoError(err, "过期后Set NX失败")
	requ.Equal(RedisOK, ok, "过期后Set NX应返回OK，实际返回=%s", ok)
	t.Logf("过期后Set NX：ok=%s", ok)

	// ========== 后置清理 ==========
	_, err = RedisClient.Del(ctx, testKey).Result()
	requ.NoError(err, "后置清理key失败")
}
