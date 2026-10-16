package util

import (
	"fmt"
	"sync"
	"time"
)

// Snowflake 生成 64 位、趋势递增且并发安全的 ID。
type Snowflake struct {
	mu        sync.Mutex
	lastMS    int64
	sequence  int64
	machineID int64
}

const (
	epoch int64 = 1704067200000 // 雪花ID的起始时间戳，单位为毫秒，对应2024-01-01 00:00:00 UTC
	// sequenceBits 雪花ID中毫秒内序列号占用的位数。
	sequenceBits = 12
	// machineBits 雪花ID中机器号占用的位数。
	machineBits = 10
	// maxSequence 雪花ID每毫秒序列号的最大值。
	maxSequence = -1 ^ (-1 << sequenceBits)
	// maxMachineID 雪花ID允许的最大机器号。
	maxMachineID = -1 ^ (-1 << machineBits)
)

var defaultSnowflake *Snowflake
var defaultSnowflakeOnce sync.Once

func NewSnowflake(machineID int64) *Snowflake {
	if machineID < 0 || machineID > maxMachineID {
		machineID = 1
	}
	return &Snowflake{machineID: machineID, lastMS: -1}
}

// GetID 获取全局唯一 ID。所有数据库新增操作都应使用该方法。
func GetID() int64 {
	defaultSnowflakeOnce.Do(func() {
		config, err := LoadAppConfig()
		if err != nil {
			panic(fmt.Errorf("加载 Snowflake 配置失败: %w", err))
		}
		defaultSnowflake = NewSnowflake(config.Snowflake.MachineID)
	})
	return defaultSnowflake.GetID()
}

func (s *Snowflake) GetID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	if now < s.lastMS {
		now = s.lastMS
	}
	if now == s.lastMS {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			for now <= s.lastMS {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}
	s.lastMS = now
	return ((now - epoch) << (machineBits + sequenceBits)) | (s.machineID << sequenceBits) | s.sequence
}
