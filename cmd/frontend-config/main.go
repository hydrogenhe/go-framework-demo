// frontend-config 从 conf/app.yaml 生成独立静态页面使用的运行时 API 地址配置。
package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"go-framework-demo/util"
)

func main() {
	config, err := util.LoadAppConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output, err := util.ProjectPath(filepath.Join("frontend", "runtime-config.js"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, portText, err := net.SplitHostPort(config.Frontend.Address)
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend.address 无效:", err)
		os.Exit(1)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprintln(os.Stderr, "frontend.address 端口无效")
		os.Exit(1)
	}
	content := fmt.Sprintf("window.DEMO_API_BASE = %q;\nwindow.DEMO_FRONTEND_PORT = %d;\n", config.Frontend.APIBaseURL, port)
	if err := os.WriteFile(output, []byte(content), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
