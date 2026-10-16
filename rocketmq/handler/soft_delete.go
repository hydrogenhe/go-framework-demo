package handler

import (
	"context"
	"fmt"
	"sync"

	"go-framework-demo/es"
)

// 固定数量锁避免无限增长；同进程同主键的 CDC 读取和 ES 写入串行执行。
var cdcLocks [256]sync.Mutex

func lockCDC(index string, id int64) func() {
	hash := uint64(id)
	for _, c := range index {
		hash = hash*31 + uint64(c)
	}
	lock := &cdcLocks[hash%uint64(len(cdcLocks))]
	lock.Lock()
	return lock.Unlock
}

// 历史物理删除也写入软删除标记，避免旧文档继续被查询。
func indexDeletedDocument(ctx context.Context, index string, id int64) error {
	doc := map[string]interface{}{"id": id, "is_del": 1}
	if index == es.ES_USER {
		doc["user_id"] = id
	}
	_, err := es.IndexDocContext(ctx, index, fmt.Sprint(id), doc)
	return err
}
