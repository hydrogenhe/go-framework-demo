#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${PROJECT_DIR}"
export GOCACHE="${GOCACHE:-/tmp/go-framework-demo-cache}"
# 生成静态页面运行配置，并默认打包 Linux x86_64 云服务器可执行文件。
go run ./cmd/frontend-config

PACKAGE_DIR="${PROJECT_DIR}/dist/go-framework-demo"
rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}/bin"
GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" CGO_ENABLED="${CGO_ENABLED:-0}" go build -o "${PACKAGE_DIR}/bin/go-framework-demo" .
GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" CGO_ENABLED="${CGO_ENABLED:-0}" go build -o "${PACKAGE_DIR}/bin/frontend-config" ./cmd/frontend-config
GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" CGO_ENABLED="${CGO_ENABLED:-0}" go build -o "${PACKAGE_DIR}/bin/frontend-server" ./cmd/frontend-server
cp -R conf frontend "${PACKAGE_DIR}/"
cp "${SCRIPT_DIR}/DEPLOYMENT.md" "${PACKAGE_DIR}/"
cp components_sh/restart_frontend.sh components_sh/start.sh "${PACKAGE_DIR}/bin/"
mkdir -p "${PACKAGE_DIR}/logs"
(
  cd "${PROJECT_DIR}/dist"
  tar -czf go-framework-demo.tar.gz go-framework-demo
)
echo "部署包已生成：${PROJECT_DIR}/dist/go-framework-demo.tar.gz"
echo "服务器解压后阅读 DEPLOYMENT.md 并编辑 conf/*.yaml，然后启动 bin/go-framework-demo 和 bin/frontend-server。"
