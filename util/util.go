package util

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"
)

// 预定义常用字符集（直接用即可）
const (
	// CharsetNum 纯数字（如验证码）
	CharsetNum = "0123456789"
	// CharsetLower 小写字母
	CharsetLower = "abcdefghijklmnopqrstuvwxyz"
	// CharsetUpper 大写字母
	CharsetUpper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// CharsetAlnum 字母+数字（最常用，如随机ID）
	CharsetAlnum = CharsetLower + CharsetUpper + CharsetNum
	// CharsetAll 字母+数字+常用符号（如密码）
	CharsetAll = CharsetAlnum + "!@#$%^&*()_+-=[]{}|;:,.<>?"
)

// 初始化随机数种子（确保每次运行生成不同结果）
func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

// GenerateRandom 生成指定长度的随机字符串
// length：要生成的字符串长度（必须>0）
// charset：字符集（可传预定义常量，如 CharsetAlnum）
// 返回：生成的字符串 / 错误（长度≤0/字符集为空时返回）
func GenerateRandom(length int, charset string) (string, error) {
	// 参数校验
	if length <= 0 {
		return "", errors.New("长度必须大于0")
	}
	if len(charset) == 0 {
		return "", errors.New("字符集不能为空")
	}

	// 生成随机字符串（高效拼接）
	result := make([]byte, length)
	charsetLen := len(charset)
	for i := 0; i < length; i++ {
		// 随机取字符集中的一个字符
		result[i] = charset[rand.Intn(charsetLen)]
	}
	return string(result), nil
}

// 快捷方法：生成纯数字随机串（简化调用）
func GenerateNum(length int) (string, error) {
	return GenerateRandom(length, CharsetNum)
}

// 快捷方法：生成字母+数字随机串（简化调用）
func GenerateAlnum(length int) string {
	str, _ := GenerateRandom(length, CharsetAlnum)
	return str
}

func Retry(retryFunc func() error, maxRetries int, delay time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = retryFunc()
		if err == nil {
			// 执行成功，直接返回
			return nil
		}
		// 不是最后一次重试，才延迟
		if i < maxRetries-1 {
			log.Printf("[WARN] 操作重试第%d次失败，%dms 后重试：%v", i+1, delay.Milliseconds(), err)
			time.Sleep(delay)
		}
	}
	// 所有重试都失败，返回最后一次的错误
	return fmt.Errorf("重试%d次后仍失败：%w", maxRetries, err)
}
