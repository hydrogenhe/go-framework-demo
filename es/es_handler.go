package es

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/olivere/elastic/v7"
)

const (
	ES_USER  = "user_test"
	ES_ORDER = "order_test"
)

func clientReady() (*elastic.Client, error) {
	if ElasticClient == nil {
		return nil, errors.New("ES客户端未初始化，请先调用InitES()")
	}
	return ElasticClient, nil
}

func IndexDoc(index, docID string, doc interface{}) (string, error) {
	c, err := clientReady()
	if err != nil {
		return "", err
	}
	req := c.Index().Index(index).BodyJson(doc).Refresh("false")
	if docID != "" {
		req = req.Id(docID)
	}
	res, err := req.Do(context.Background())
	if err != nil {
		return "", fmt.Errorf("写入ES失败: %w", err)
	}
	return res.Id, nil
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
	c, err := clientReady()
	if err != nil {
		return nil, err
	}
	res, err := c.Search(index).Query(query).Do(context.Background())
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
	return result, nil
}

func SearchAfterDocs(index string, dsl map[string]interface{}, sort []map[string]interface{}, searchAfter []interface{}) (map[string]interface{}, error) {
	c, err := clientReady()
	if err != nil {
		return nil, err
	}
	queryJSON, _ := json.Marshal(dsl)
	search := c.Search(index).Query(elastic.NewRawStringQuery(string(queryJSON))).Size(10)
	for _, item := range sort {
		for field, direction := range item {
			desc, _ := direction.(string)
			search = search.Sort(field, desc != "desc")
		}
	}
	if len(searchAfter) > 0 {
		search = search.SearchAfter(searchAfter...)
	}
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(res)
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	return result, nil
}
