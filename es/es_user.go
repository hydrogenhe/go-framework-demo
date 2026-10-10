package es

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-framework-demo/es/es_model"
)

type userSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source es_model.User `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// GetUserByID 按用户 ID 从 ES 读取用户。
func GetUserByID(id int64) (*es_model.User, error) {
	result, err := SearchDocs(ES_USER, BuildUserElasticQuery(UserSearchOptions{ID: id}))
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var response userSearchResponse
	if err := json.Unmarshal(b, &response); err != nil {
		return nil, err
	}
	if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("用户不存在: %d", id)
	}
	return &response.Hits.Hits[0].Source, nil
}

func IndexUser(user *es_model.User) (string, error) {
	if user == nil {
		return "", errors.New("User对象不能为空")
	}
	doc, err := user.ToMap()
	if err != nil {
		return "", err
	}
	return IndexDoc(ES_USER, fmt.Sprintf("%d", user.ID), doc)
}
func BulkIndexUsers(users []*es_model.User) error {
	if len(users) == 0 {
		return errors.New("批量写入的User列表为空")
	}
	docs := make([]map[string]interface{}, 0, len(users))
	for _, user := range users {
		if user == nil {
			return errors.New("User对象不能为空")
		}
		doc, err := user.ToMap()
		if err != nil {
			return err
		}
		docs = append(docs, doc)
	}
	return BulkIndexDocs("user", docs)
}
