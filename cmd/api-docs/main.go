// api-docs 校验独立 OpenAPI 文档与实际路由，并输出对应 JSON。
package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	handler "go-framework-demo/http_handler"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
	"strings"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate() error {
	raw, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		return err
	}
	var spec map[string]any
	if err = yaml.Unmarshal(raw, &spec); err != nil {
		return err
	}
	if spec["openapi"] != "3.0.3" {
		return fmt.Errorf("文档必须使用 OpenAPI 3.0.3")
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return fmt.Errorf("文档缺少 paths")
	}
	gin.SetMode(gin.ReleaseMode)
	router := handler.InitHTTP(":0").Handler.(*gin.Engine)
	param := regexp.MustCompile(`:([A-Za-z0-9_]+)`)
	routes := map[string]bool{}
	for _, r := range router.Routes() {
		if !strings.HasPrefix(r.Path, "/api/") && r.Path != "/metrics" {
			continue
		}
		path := param.ReplaceAllString(r.Path, "{$1}")
		method := strings.ToLower(r.Method)
		routes[method+" "+path] = true
		operations, ok := paths[path].(map[string]any)
		if !ok || operations[method] == nil {
			return fmt.Errorf("接口文档遗漏: %s %s", r.Method, path)
		}
	}
	for path, value := range paths {
		operations, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("无效路径: %s", path)
		}
		for method := range operations {
			switch method {
			case "get", "put", "post", "delete", "patch", "head", "options":
				if !routes[method+" "+path] {
					return fmt.Errorf("文档存在未注册接口: %s %s", method, path)
				}
			}
		}
	}
	if err = checkReferences(spec, spec); err != nil {
		return err
	}
	raw, err = json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile("docs/openapi.json", append(raw, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("OpenAPI 路由与引用校验通过，已生成 docs/openapi.json（%d 个接口）\n", len(routes))
	return nil
}
func checkReferences(value any, root map[string]any) error {
	switch v := value.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok {
			if !strings.HasPrefix(ref, "#/") {
				return fmt.Errorf("仅支持文档内引用: %s", ref)
			}
			var current any = root
			for _, key := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				object, ok := current.(map[string]any)
				if !ok {
					return fmt.Errorf("无效引用: %s", ref)
				}
				current, ok = object[key]
				if !ok {
					return fmt.Errorf("引用不存在: %s", ref)
				}
			}
		}
		for _, child := range v {
			if err := checkReferences(child, root); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := checkReferences(child, root); err != nil {
				return err
			}
		}
	}
	return nil
}
