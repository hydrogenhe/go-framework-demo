#!/usr/bin/env bash
set -o pipefail

cd -- "$(dirname -- "$0")" || exit 1

run_tests() {
  local packages=("$@")
  GOCACHE=/tmp/go-build-cache-go-gen go test "${packages[@]}" -count=1 -v 2>&1 |
    grep -E '^=== RUN|^--- (PASS|FAIL|SKIP)|^(PASS|FAIL)$|^(ok|FAIL)[[:space:]]|^# |^.*\.go:[0-9]+:'
}

# 运行 DB 测试
run_db_tests() {
  run_tests ./db/test
}

# 运行 ES 测试
run_es_tests() {
  run_tests ./es/test
}

# 运行全部 DB 和 ES 测试
run_all_tests() {
  run_tests ./db/test ./es/test
}

# 直接点击脚本或执行 ./test.sh 时运行全部测试
run_all_tests
