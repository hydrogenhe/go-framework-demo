package es

import (
	"context"
	"fmt"
)

// EnsureCreateTimeStampMapping 将现有业务索引的业务时间戳映射为 long、展示时间映射为 date。
// 未创建的索引不提前创建；后续写入的 int64 字段会动态映射为 long。
func EnsureCreateTimeStampMapping(ctx context.Context) error {
	client, err := clientReady()
	if err != nil {
		return err
	}
	for _, index := range []string{ES_USER, ES_PRODUCT, ES_ORDER} {
		exists, err := client.IndexExists(index).Do(ctx)
		if err != nil {
			return fmt.Errorf("检查索引 %s: %w", index, err)
		}
		if !exists {
			continue
		}
		mapping := `{"properties":{"create_time_stamp":{"type":"long"},"update_time_stamp":{"type":"long"},"update_time":{"type":"date"}}}`
		if index == ES_ORDER {
			mapping = `{"properties":{"create_time_stamp":{"type":"long"},"update_time_stamp":{"type":"long"},"update_time":{"type":"date"},"items":{"type":"nested","properties":{"update_time_stamp":{"type":"long"},"update_time":{"type":"date"}}}}}`
		}
		if _, err := client.PutMapping().Index(index).BodyString(mapping).Do(ctx); err != nil {
			return fmt.Errorf("设置 %s 时间戳映射: %w", index, err)
		}
	}
	return nil
}
