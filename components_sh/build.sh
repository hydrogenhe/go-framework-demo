#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${PROJECT_DIR}/bin"
BACKEND_BIN="${BIN_DIR}/go-framework-demo"

mkdir -p "${BIN_DIR}"

echo "开始构建后端：${BACKEND_BIN}"
(cd "${PROJECT_DIR}" && go build -o "${BACKEND_BIN}" .)

echo "构建完成：${BACKEND_BIN}"
