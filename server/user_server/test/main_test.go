package user

import (
	"go-framework-demo/server/test/support"
	"testing"
)

// TestMain 初始化本业务的测试依赖，并在退出时回收 MQ 资源。
func TestMain(m *testing.M) { support.Run(m) }
