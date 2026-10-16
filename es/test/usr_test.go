package test

import (
	"testing"
	"time"

	"go-framework-demo/es"
	"go-framework-demo/es/es_model"
	"go-framework-demo/util"
)

// TestESUserCRUD 使用真实 ES，验证用户文档创建、查询、姓名更新及创建时间戳。
func TestESUserCRUD(t *testing.T) {
	if err := es.InitES(); err != nil {
		t.Skipf("ES不可用，跳过测试: %v", err)
	}
	testUser := &es_model.User{
		ID:         util.GetID(),
		Name:       "test_usr" + util.GenerateAlnum(5),
		Age:        30,
		Email:      util.RandomTestEmail(),
		CreateTime: time.Now()}
	docID, err := es.IndexUser(testUser)
	if err != nil {
		t.Fatalf("写入User失败: %v", err)
	}
	t.Logf("User写入成功，文档ID：%s", docID)

	actualUser, err := es.GetUserByID(testUser.ID)
	if err != nil {
		t.Fatalf("查询User失败: %v", err)
	}
	if actualUser.Name != testUser.Name || actualUser.Email != testUser.Email {
		t.Fatalf("查询用户数据不匹配: %+v", actualUser)
	}
	if actualUser.CreateTimeStamp != testUser.CreateTime.UnixMilli() {
		t.Errorf("创建时间戳不匹配")
	}
	testUser.Name = "updated_usr" + util.GenerateAlnum(5)
	if err := es.UpdateUser(testUser); err != nil {
		t.Fatalf("更新User失败: %v", err)
	}
	updatedUser, err := es.GetUserByID(testUser.ID)
	if err != nil {
		t.Fatalf("查询更新后的User失败: %v", err)
	}
	if updatedUser.Name != testUser.Name {
		t.Errorf("User更新失败，预期：%s，实际：%s", testUser.Name, updatedUser.Name)
	}
}
