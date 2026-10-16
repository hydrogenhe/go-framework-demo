#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${PROJECT_DIR}/bin"
BACKEND_BIN="${BIN_DIR}/go-framework-demo"
LOG_DIR="${PROJECT_DIR}/logs"
LOG_FILE="${LOG_DIR}/go-framework-demo.log"
PORT="8080"

mkdir -p "${BIN_DIR}" "${LOG_DIR}"
echo "编译最新后端：${BACKEND_BIN}"
(cd "${PROJECT_DIR}" && go build -o "${BACKEND_BIN}" .)

old_pids="$(lsof -ti tcp:${PORT} || true)"
if [[ -n "${old_pids}" ]]; then
  echo "停止旧后端进程：${old_pids}"
  while read -r pid; do
    [[ -n "${pid}" ]] && kill "${pid}" 2>/dev/null || true
  done <<< "${old_pids}"
  for _ in {1..10}; do
    lsof -ti tcp:${PORT} >/dev/null 2>&1 || break
    sleep 1
  done
fi

echo "启动后端：http://127.0.0.1:${PORT}"
nohup "${BACKEND_BIN}" >>"${LOG_FILE}" 2>&1 < /dev/null &
backend_pid=$!
echo "${backend_pid}" > "${LOG_DIR}/go-framework-demo.pid"

for _ in {1..30}; do
  if lsof -ti tcp:${PORT} >/dev/null 2>&1; then
    echo "后端已启动，PID：${backend_pid}"
    echo "日志：${LOG_FILE}"
    exit 0
  fi
  if ! kill -0 "${backend_pid}" 2>/dev/null; then
    echo "后端启动失败，请查看日志：${LOG_FILE}" >&2
    exit 1
  fi
  sleep 1
done

echo "后端启动失败：等待 30 秒后 ${PORT} 仍未监听，请查看日志：${LOG_FILE}" >&2
exit 1
