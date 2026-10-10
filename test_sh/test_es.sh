#!/usr/bin/env bash
set -o pipefail
cd -- "$(dirname -- "$0")" || exit 1
GOCACHE=/tmp/go-build-cache-go-gen go test ./es/test -count=1 -v 2>&1 | grep -E '^=== RUN|^--- (PASS|FAIL|SKIP)|^(PASS|FAIL)$|^(ok|FAIL)[[:space:]]|^# |^.*\.go:[0-9]+:'
