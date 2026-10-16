package es

import (
	"context"
	"errors"
	"go-framework-demo/es/es_model"
	"go-framework-demo/util"
)

// GetUserByID 按 ID 从 ES 读取文档。
func GetUserByID(id int64) (*es_model.User, error) {
	return GetUserByIDContext(context.Background(), id)
}

// GetUserByIDContext 使用请求上下文查询文档。
func GetUserByIDContext(ctx context.Context, id int64) (*es_model.User, error) {
	var user es_model.User
	searchOptions := UserSearchOptions{
		ID: &id,
	}
	source, err := BuildUserElasticQuery(&searchOptions)
	if err != nil {
		return nil, err
	}
	source.Sort("user_id", false)
	if err := SearchOneContext(ctx, ES_USER, source, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func IndexUser(user *es_model.User) (string, error) {
	return IndexUserContext(context.Background(), user)
}

func IndexUserContext(ctx context.Context, user *es_model.User) (string, error) {
	if user == nil {
		return "", errors.New("User对象不能为空")
	}
	doc, err := user.ToMap()
	if err != nil {
		return "", err
	}
	return IndexDocContext(ctx, ES_USER, util.IDString(user.ID), doc)
}

// UpdateUser 更新用户 ES 文档。
func UpdateUser(user *es_model.User) error {
	if user == nil {
		return errors.New("User对象不能为空")
	}
	doc, err := user.ToMap()
	if err != nil {
		return err
	}
	return UpdateDoc(ES_USER, util.IDString(user.ID), doc)
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
