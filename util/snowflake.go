package util

import (
	"os"
	"strconv"
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
	epoch        int64 = 1704067200000 // 2024-01-01 00:00:00 UTC
	sequenceBits       = 12
	machineBits        = 10
	maxSequence        = -1 ^ (-1 << sequenceBits)
	maxMachineID       = -1 ^ (-1 << machineBits)
)

var defaultSnowflake = NewSnowflake(machineIDFromEnv())

func NewSnowflake(machineID int64) *Snowflake {
	if machineID < 0 || machineID > maxMachineID {
		machineID = 1
	}
	return &Snowflake{machineID: machineID, lastMS: -1}
}

// GetID 获取全局唯一 ID。所有数据库新增操作都应使用该方法。
func GetID() int64 {
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

func machineIDFromEnv() int64 {
	id, err := strconv.ParseInt(os.Getenv("SNOWFLAKE_MACHINE_ID"), 10, 64)
	if err != nil {
		return 1
	}
	return id
}
