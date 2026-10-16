// frontend-server 使用 app.yaml 的监听地址提供独立静态前端。
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"go-framework-demo/util"
)

func main() {
	config, err := util.LoadAppConfig()
	if err != nil {
		log.Fatal(err)
	}
	root, err := util.ProjectPath("frontend")
	if err != nil {
		log.Fatal(err)
	}
	_, portText, err := net.SplitHostPort(config.Frontend.Address)
	if err != nil {
		log.Fatal("frontend.address 无效: ", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		log.Fatal("frontend.address 端口无效")
	}
	runtimeConfig := fmt.Sprintf("window.DEMO_API_BASE = %q;\nwindow.DEMO_FRONTEND_PORT = %d;\n", config.Frontend.APIBaseURL, port)
	if err := os.WriteFile(filepath.Join(root, "runtime-config.js"), []byte(runtimeConfig), 0644); err != nil {
		log.Fatal("生成前端运行配置失败: ", err)
	}
	server := &http.Server{Addr: config.Frontend.Address, Handler: http.FileServer(http.Dir(filepath.Clean(root)))}
	log.Printf("前端静态服务启动: http://%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
