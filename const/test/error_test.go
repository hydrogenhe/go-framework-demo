package appconst_test

import (
	"errors"
	"fmt"
	appconst "go-framework-demo/const"
	"testing"
)

// TestWrappedBusinessError 验证业务错误可以被正确识别，且保留原始错误对象。
func TestWrappedBusinessError(t *testing.T) {
	original := appconst.NewConflictError("邮箱已注册")
	actual, ok := appconst.AsMsgError(original)
	if !ok || actual != original {
		t.Fatal("lost business error")
	}
}

func TestNewInternalErrorPreservesWrappedErrors(t *testing.T) {
	original := appconst.NewConflictError("库存不足")
	for _, input := range []error{original, fmt.Errorf("创建订单失败: %w", original)} {
		actual := appconst.NewInternalError(input)
		if actual != original {
			t.Fatalf("业务错误被重复包装: %+v", actual)
		}
		if appconst.NewInternalError(actual) != actual {
			t.Fatal("重复调用改变业务错误")
		}
	}
	raw := errors.New("database connection failed")
	actual := appconst.NewInternalError(raw)
	if actual.Code != appconst.CodeInternal || actual.HTTPStatus != 500 || actual.Message != "系统繁忙，请稍后重试" || !errors.Is(actual, raw) {
		t.Fatalf("系统错误包装异常: %+v", actual)
	}
	if appconst.NewInternalError(actual) != actual {
		t.Fatal("系统错误被重复包装")
	}
	var nilBusiness *appconst.MsgError
	if actual, ok := appconst.AsMsgError(nilBusiness); ok || actual != nil {
		t.Fatal("空业务错误不应识别为有效包装")
	}
}
