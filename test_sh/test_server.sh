#!/usr/bin/env bash
# 按业务运行服务层测试；testdata 不会被 Go 的 ./... 自动发现，必须显式指定。
set -euo pipefail
cd -- "$(dirname -- "$0")/.."
GOCACHE="${GOCACHE:-/tmp/go-framework-demo-cache}" go test -p 1 \
  ./server/user_server/test ./server/product_server/test ./server/order_server/test \
  ./server/test/log ./server/test/testdata -count=1 -v "$@"
