#!/usr/bin/env bash
# 校验集中维护的 OpenAPI YAML 及真实路由，生成对应 JSON，不解析业务注释。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "$PROJECT_DIR"
go run ./cmd/api-docs
