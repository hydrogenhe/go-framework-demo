package es

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-framework-demo/logger"
	"time"

	"github.com/olivere/elastic/v7"
)

const (
	// ES_USER 用户搜索副本的索引名称。
	ES_USER = "user_test"
	// ES_ORDER 订单搜索副本的索引名称。
	ES_ORDER = "order_test"
)

func clientReady() (*elastic.Client, error) {
	if ElasticClient == nil {
		return nil, errors.New("ES客户端未初始化，请先调用InitES()")
	}
	return ElasticClient, nil
}

func IndexDoc(index, docID string, doc interface{}) (string, error) {
	return IndexDocContext(context.Background(), index, docID, doc)
}

func IndexDocContext(ctx context.Context, index, docID string, doc interface{}) (string, error) {
	c, err := clientReady()
	if err != nil {
		return "", err
	}
	// wait_for 确保写入后立即查询时文档已经可见。
	req := c.Index().Index(index).BodyJson(doc).Refresh("wait_for")
	if docID != "" {
		req = req.Id(docID)
	}
	res, err := req.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("写入ES失败: %w", err)
	}
	return res.Id, nil
}

// UpdateDoc 更新 ES 文档的 source 字段。
func UpdateDoc(index, docID string, doc interface{}) error {
	c, err := clientReady()
	if err != nil {
		return err
	}
	if _, err := c.Update().Index(index).Id(docID).Doc(doc).Refresh("wait_for").Do(context.Background()); err != nil {
		return fmt.Errorf("更新ES文档失败: %w", err)
	}
	return nil
}

func DeleteDoc(index, docID string) error {
	return DeleteDocContext(context.Background(), index, docID)
}

func DeleteDocContext(ctx context.Context, index, docID string) error {
	c, err := clientReady()
	if err != nil {
		return err
	}
	res, err := c.Delete().Index(index).Id(docID).Refresh("wait_for").Do(ctx)
	// CDC 删除必须幂等：仅文档不存在（404 + not_found）按成功处理。
	// 索引不存在等其他 404 仍返回错误，避免掩盖配置问题。
	if elastic.IsNotFound(err) && res != nil && res.Result == "not_found" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除ES文档失败: %w", err)
	}
	return nil
}

func BulkIndexDocs(index string, docs []map[string]interface{}) error {
	c, err := clientReady()
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return errors.New("批量写入的文档列表为空")
	}
	bulk := c.Bulk().Index(index).Refresh("false")
	for _, doc := range docs {
		bulk.Add(elastic.NewBulkIndexRequest().Doc(doc))
	}
	res, err := bulk.Do(context.Background())
	if err != nil {
		return fmt.Errorf("批量写入失败: %w", err)
	}
	if res.Errors {
		return errors.New("ES批量写入包含失败项")
	}
	return nil
}

func SearchDocs(index string, query elastic.Query) (map[string]interface{}, error) {
	return SearchDocsContext(context.Background(), index, query)
}

// SearchDocsContext 查询文档并保留请求上下文中的 Log ID。
func SearchDocsContext(ctx context.Context, index string, query elastic.Query) (map[string]interface{}, error) {
	c, err := clientReady()
	if err != nil {
		return nil, err
	}
	started := time.Now()
	res, err := c.Search(index).Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("is_del", 0)).Must(query)).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("ES查询失败: %w", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	logger.Info(ctx, "ES查询完成", "method", "SearchDocs", "index", index, "returned_count", len(res.Hits.Hits), "total", res.TotalHits(), "elapsed_ms", float64(time.Since(started))/float64(time.Millisecond))
	return result, nil
}

func SearchOne(index string, source *elastic.SearchSource, target interface{}) error {
	return SearchOneContext(context.Background(), index, source, target)
}

// SearchOneContext 查询单个文档，成功时记录数量和耗时。
func SearchOneContext(ctx context.Context, index string, source *elastic.SearchSource, target interface{}) error {
	c, err := clientReady()
	if err != nil {
		return err
	}
	started := time.Now()
	res, err := c.Search(index).SearchSource(source).Size(1).Do(ctx)
	if err != nil {
		return fmt.Errorf("ES查询失败: %w", err)
	}
	if len(res.Hits.Hits) == 0 {
		logger.Info(ctx, "ES查询完成", "method", "SearchOne", "index", index, "returned_count", 0, "total", res.TotalHits(), "elapsed_ms", float64(time.Since(started))/float64(time.Millisecond))
		return errors.New("ES查询结果为空")
	}
	if err := json.Unmarshal(res.Hits.Hits[0].Source, target); err != nil {
		return fmt.Errorf("ES结果反序列化失败: %w", err)
	}
	logger.Info(ctx, "ES查询完成", "method", "SearchOne", "index", index, "returned_count", 1, "total", res.TotalHits(), "elapsed_ms", float64(time.Since(started))/float64(time.Millisecond))
	return nil
}
