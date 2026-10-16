package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectRoot 从当前工作目录或可执行文件所在目录逐级向上查找 conf 目录。
func ProjectRoot() (string, error) {
	starts := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			info, err := os.Stat(filepath.Join(dir, "conf"))
			if err == nil && info.IsDir() {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return "", fmt.Errorf("未找到项目 conf 目录：请从项目目录运行，或将二进制放在项目目录内")
}

// ConfigPath 查找主目录下的 conf/name，保持旧调用兼容；新代码优先使用 ConfigFile。
func ConfigPath(name string) string {
	path, err := ConfigFile(name)
	if err != nil {
		return filepath.Join("conf", name)
	}
	return path
}

// ConfigFile 返回项目主目录 conf 下的配置文件，并明确报告文件缺失。
func ConfigFile(name string) (string, error) {
	root, err := ProjectRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "conf", name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("配置文件 %s 不可用: %w", path, err)
	}
	return path, nil
}

// ProjectPath 将项目内资源路径解析为绝对路径。
func ProjectPath(name string) (string, error) {
	root, err := ProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}
