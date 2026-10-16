package user

import (
	"context"
	appconst "go-framework-demo/const"
	"go-framework-demo/query"
	"go-framework-demo/server/test/support"
	"go-framework-demo/server/user_server"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

// TestIntegrationRegistrationValidation 验证注册密码哈希、邮箱重复拒绝，以及密码长度边界和中文密码。
func TestIntegrationRegistrationValidation(t *testing.T) {
	support.ValidationTransaction(t, func(ctx context.Context, q *query.Query) {
		users := user_server.NewUserService(ctx, q)
		input := support.RandomUserInput()
		user, err := users.CreateUser(ctx, input.Name, input.Age, input.Email, input.Password)
		if err != nil {
			t.Fatal(err)
		}
		if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
			t.Fatal("password not hashed correctly")
		}
		duplicate := support.RandomUserInput()
		duplicate.Email = input.Email
		_, err = users.CreateUser(ctx, duplicate.Name, duplicate.Age, duplicate.Email, duplicate.Password)
		if msg, ok := appconst.AsMsgError(err); !ok || msg.Message != "邮箱已注册" {
			t.Fatalf("unexpected duplicate email result: %v", err)
		}
		for _, password := range []string{"", "12345", strings.Repeat("a", 19)} {
			input := support.RandomUserInput()
			input.Password = password
			_, err = users.CreateUser(ctx, input.Name, input.Age, input.Email, input.Password)
			if msg, ok := appconst.AsMsgError(err); !ok || msg.Code != appconst.CodeInvalidParams {
				t.Fatalf("invalid password accepted: %v", err)
			}
		}
		for _, password := range []string{"123456", strings.Repeat("a", 18), "六位中文密码"} {
			input := support.RandomUserInput()
			input.Password = password
			user, err := users.CreateUser(ctx, input.Name, input.Age, input.Email, input.Password)
			if err != nil {
				t.Fatal(err)
			}
			if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
				t.Fatal(err)
			}
		}
	})
}
