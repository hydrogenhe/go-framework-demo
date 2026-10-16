package util

import (
	"fmt"
	"go-framework-demo/logger"

	"github.com/bytedance/sonic"
)

// Marshal 结构体/任意数据转JSON字节数组（高性能sonic版）
// 特性：
// 1. 兼容标准库json tag（omitempty/string等）
// 2. 处理中文/特殊字符无乱码
// 3. 失败返回明确错误信息
func Marshal(v interface{}) ([]byte, error) {
	data, err := sonic.Marshal(v)
	if err != nil {
		errMsg := fmt.Sprintf("JSON序列化失败: %v, 原始数据=%+v", err, v)
		logger.Sugar.Infof("[JSON_UTIL_ERROR] %s", errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}
	return data, nil
}

// Unmarshal JSON字节数组反序列化为指定结构体（高性能sonic版）
// 入参：
//
//	data: JSON字节数组
//	v: 接收结果的结构体指针（如 &model.User{}）
func Unmarshal(data []byte, v interface{}) error {
	err := sonic.Unmarshal(data, v)
	if err != nil {
		errMsg := fmt.Sprintf("JSON反序列化失败: %v, JSON数据=%s", err, string(data))
		logger.Sugar.Infof("[JSON_UTIL_ERROR] %s", errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// MarshalIndent 结构体转格式化JSON字符串（带缩进，便于日志/调试）
// 返回值：格式化后的JSON字符串，失败返回空字符串+错误
func MarshalIndent(v interface{}) (string, error) {
	data, err := sonic.MarshalIndent(v, "", "  ")
	if err != nil {
		errMsg := fmt.Sprintf("JSON格式化失败: %v, 原始数据=%+v", err, v)
		logger.Sugar.Infof("[JSON_UTIL_ERROR] %s", errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}
	return string(data), nil
}

// Format 格式化JSON字节数组为易读字符串（日志打印专用）
// 特性：
// 1. 非JSON数据返回原始字符串+错误提示
// 2. 兼容乱码/特殊字符，保证日志可读
func Format(data []byte) string {
	if len(data) == 0 {
		return "【空JSON】"
	}

	var temp interface{}
	// 尝试解析JSON，失败则返回原始内容+提示
	if err := sonic.Unmarshal(data, &temp); err != nil {
		return fmt.Sprintf("【非JSON格式】%s (解析错误: %v)", string(data), err)
	}

	// 格式化输出（带缩进）
	formatted, err := sonic.MarshalIndent(temp, "", "  ")
	if err != nil {
		return string(data) // 格式化失败返回原始字符串
	}
	return string(formatted)
}

// MustMarshal 结构体转JSON字节数组（忽略错误，panic版）
// 适用场景：确定不会序列化失败的场景（如测试/固定结构体）
func MustMarshal(v interface{}) []byte {
	data, err := Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("JSON序列化panic: %v", err))
	}
	return data
}

// MustUnmarshal JSON字节数组反序列化（忽略错误，panic版）
func MustUnmarshal(data []byte, v interface{}) {
	if err := Unmarshal(data, v); err != nil {
		panic(fmt.Sprintf("JSON反序列化panic: %v", err))
	}
}
