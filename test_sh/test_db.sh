#!/usr/bin/env bash
# 运行数据库测试，检查持久化及查询结果，并输出用例结果与错误位置。
set -o pipefail
cd -- "$(dirname -- "$0")" || exit 1
GOCACHE=/tmp/go-build-cache-go-gen go test ./db/test -count=1 -v 2>&1 | grep -E '^=== RUN|^--- (PASS|FAIL|SKIP)|^(PASS|FAIL)$|^(ok|FAIL)[[:space:]]|^# |^.*\.go:[0-9]+:'
