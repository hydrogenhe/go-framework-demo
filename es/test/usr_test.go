// 包名必须与目录名一致：test
package test

import (
	"encoding/json"
	"go-framework-demo/util"
	"testing"
	"time"

	// 正确导入父包 es（从项目根目录导入）
	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
)

type userSearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source es_model.User `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// TestMain 全局初始化（所有测试用例前运行）

// TestESUserCRUD 测试User写入+查询
func TestESUserCRUD(t *testing.T) {
	// 1. 构建测试数据
	id := util.GetID()
	err := es.InitES()
	if err != nil {
		return
	}
	testUser := &es_model.User{
		ID:         id,
		Name:       "测试用户",
		Age:        30,
		Email:      "test_user@example.com",
		CreateTime: time.Now(),
	}
	testUser.Name = "test_usr" + util.GenerateAlnum(5)

	// 2. 写入ES
	docID, err := es.IndexUser(testUser)
	if err != nil {
		t.Fatalf("写入User失败: %v", err)
	}
	t.Logf("User写入成功，文档ID：%s", docID)

	// 3. 查询验证
	queryDsl := es.BuildUserElasticQuery(es.UserSearchOptions{ID: testUser.ID})

	result, err := es.SearchDocs(es.ES_USER, queryDsl)
	if err != nil {
		t.Fatalf("查询User失败: %v", err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("序列化查询结果失败: %v", err)
	}
	var response userSearchResponse
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("反序列化查询结果失败: %v", err)
	}
	if response.Hits.Total.Value == 0 || len(response.Hits.Hits) == 0 {
		t.Fatal("查询结果为空，写入的User未找到")
	}
	t.Logf("查询到 %d 条匹配数据", response.Hits.Total.Value)

	source := response.Hits.Hits[0].Source
	if source.Name != testUser.Name {
		t.Errorf("姓名不匹配，预期：%s，实际：%s", testUser.Name, source.Name)
	}
	if source.Email != testUser.Email {
		t.Errorf("邮箱不匹配，预期：%s，实际：%s", testUser.Email, source.Email)
	}
	if source.CreateTimeStamp != testUser.CreateTime.UnixMilli() {
		t.Errorf("创建时间戳不匹配，预期：%d，实际：%d", testUser.CreateTime.UnixMilli(), source.CreateTimeStamp)
	}

	t.Log("✅ User写入+查询测试通过")
}
