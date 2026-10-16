// refresh-demo-es 根据 MySQL 当前状态校准 Demo ES 副本，不修改 MySQL。
package main

import (
	"context"
	"fmt"
	"go-framework-demo/es"
	"go-framework-demo/logger"
	"go-framework-demo/mysql"
	"go-framework-demo/rocketmq/handler"
	"go-framework-demo/util"
	"io"
	"os"
	"strconv"
	"time"
)

func main() {
	logger.Init()
	defer logger.Sync()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	config, err := util.LoadConfig()
	if err != nil {
		return err
	}
	if err := mysql.InitMySQL(config.MySQL); err != nil {
		return err
	}
	if err := es.InitES(config.ES); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	for _, entry := range []struct {
		Table, Index string
		Sync         func(context.Context, int64) error
	}{
		{"user", es.ES_USER, handler.SyncUserFromMySQL},
		{"product", es.ES_PRODUCT, handler.SyncProductFromMySQL},
		{"order", es.ES_ORDER, handler.SyncOrderFromMySQL},
	} {
		mapping := `{"properties":{"create_time_stamp":{"type":"long"},"is_del":{"type":"byte"}}}`
		if entry.Table == "order" {
			mapping = `{"properties":{"create_time_stamp":{"type":"long"},"is_del":{"type":"byte"},"items":{"type":"nested","properties":{"is_del":{"type":"byte"},"id":{"type":"long"}}}}}`
		}
		if _, err := es.ElasticClient.PutMapping().Index(entry.Index).BodyString(mapping).Do(ctx); err != nil {
			return err
		}
		var ids []int64
		if err := mysql.GetDB().WithContext(ctx).Table(entry.Table).Pluck("id", &ids).Error; err != nil {
			return err
		}
		known := map[int64]bool{}
		for _, id := range ids {
			known[id] = true
			if err := entry.Sync(ctx, id); err != nil {
				return fmt.Errorf("%s id=%d: %w", entry.Table, id, err)
			}
		}
		// 孤立旧文档也通过同一个 CDC 入口写入 is_del=1，而不是直接物理删除。
		scroll := es.ElasticClient.Scroll(entry.Index).Size(500)
		extra := 0
		for {
			result, err := scroll.Do(ctx)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			for _, hit := range result.Hits.Hits {
				id, err := strconv.ParseInt(hit.Id, 10, 64)
				if err != nil {
					return err
				}
				if !known[id] {
					if err := entry.Sync(ctx, id); err != nil {
						return err
					}
					extra++
				}
			}
		}
		if err := scroll.Clear(ctx); err != nil {
			return err
		}
		fmt.Printf("%s: refreshed=%d orphan_marked_deleted=%d\n", entry.Index, len(ids), extra)
	}
	return nil
}
