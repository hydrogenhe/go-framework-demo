#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
FRONTEND_DIR="${PROJECT_DIR}/frontend"
cd -- "${PROJECT_DIR}"
export GOCACHE="${GOCACHE:-/tmp/go-framework-demo-cache}"

if [[ -x "${PROJECT_DIR}/bin/frontend-config" ]]; then
  "${PROJECT_DIR}/bin/frontend-config"
else
  go run ./cmd/frontend-config
fi
FRONTEND_PORT="$(sed -n 's/.*DEMO_FRONTEND_PORT = \([0-9]*\);.*/\1/p' "${FRONTEND_DIR}/runtime-config.js")"
[[ -n "${FRONTEND_PORT}" ]] || { echo "无法读取 conf/app.yaml 中的 frontend.address" >&2; exit 1; }
[[ -f "${FRONTEND_DIR}/index.html" && -f "${FRONTEND_DIR}/logs.html" ]] || { echo "缺少商品前端或日志查询页面" >&2; exit 1; }
old_pids="$(lsof -tiTCP:${FRONTEND_PORT} -sTCP:LISTEN || true)"
if [[ -n "${old_pids}" ]]; then
  while read -r pid; do [[ -n "${pid}" ]] && kill "${pid}" 2>/dev/null || true; done <<< "${old_pids}"
  sleep 1
fi
if [[ -x "${PROJECT_DIR}/bin/frontend-server" ]]; then
  exec "${PROJECT_DIR}/bin/frontend-server"
fi
exec go run ./cmd/frontend-server
