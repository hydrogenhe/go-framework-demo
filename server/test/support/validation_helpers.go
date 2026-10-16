package support

import (
	"context"
	"errors"
	"go-framework-demo/mysql"
	"go-framework-demo/query"
	"go-framework-demo/util"
	"os"
	"testing"
)

type UserCreateInput struct {
	Name     string
	Age      int32
	Email    string
	Password string
}

// RandomUserInput 生成随机姓名、邮箱和密码，减少测试数据重复。
func RandomUserInput() UserCreateInput {
	return UserCreateInput{
		Name:     "validation_" + util.GenerateAlnum(8),
		Age:      18,
		Email:    util.RandomTestEmail(),
		Password: util.GenerateAlnum(8),
	}
}

// ValidationTransaction 在外层事务中调用真实业务代码，测试结束后回滚全部测试数据。
func ValidationTransaction(t *testing.T, run func(context.Context, *query.Query)) {
	t.Helper()
	if os.Getenv("DEMO_INTEGRATION") != "1" {
		t.Skip("set DEMO_INTEGRATION=1 to validate local MySQL")
	}
	mysql.InitMySQL()
	ctx := context.Background()
	rollback := errors.New("rollback test fixtures")
	err := mysql.GetGenQuery().Transaction(func(q *query.Query) error { run(ctx, q); return rollback })
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
