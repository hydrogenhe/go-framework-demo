#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BACKEND_BIN="${PROJECT_DIR}/bin/go-framework-demo"
APP_CONFIG="${PROJECT_DIR}/conf/app.yaml"
PORT="$(awk -F: '/^[[:space:]]*address:/ { gsub(/[^0-9]/, "", $NF); print $NF; exit }' "${APP_CONFIG}")"
[[ -n "${PORT}" ]] || { echo "无法读取 conf/app.yaml 的 http.address 端口" >&2; exit 1; }

cleanup() {
	trap - INT TERM EXIT
	if [[ -n "${BACKEND_PID:-}" ]]; then
		kill "${BACKEND_PID}" 2>/dev/null || true
	fi
}

trap cleanup INT TERM EXIT
export GOCACHE="${GOCACHE:-/tmp/go-framework-demo-cache}"

if [[ ! -f "${BACKEND_BIN}" ]]; then
	echo "未找到后端执行文件，开始构建：${BACKEND_BIN}"
	mkdir -p "${PROJECT_DIR}/bin"
	(cd "${PROJECT_DIR}" && go build -o "${BACKEND_BIN}" .)
fi

old_pids="$(lsof -ti tcp:${PORT} || true)"
if [[ -n "${old_pids}" ]]; then
	echo "停止旧后端进程：${old_pids}"
	while read -r pid; do
		[[ -n "${pid}" ]] && kill "${pid}" 2>/dev/null || true
	done <<< "${old_pids}"
	sleep 1
fi

echo "启动后端：http://localhost:${PORT}"
"${BACKEND_BIN}" &
BACKEND_PID=$!

MAX_RETRY=30
RETRY_COUNT=0
while ! lsof -ti tcp:${PORT} >/dev/null 2>&1; do
	if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
		echo "后端启动失败：进程已退出" >&2
		exit 1
	fi
	if (( RETRY_COUNT >= MAX_RETRY )); then
		echo "后端启动失败：等待 ${MAX_RETRY} 秒后端口 ${PORT} 仍未监听" >&2
		exit 1
	fi
	RETRY_COUNT=$((RETRY_COUNT + 1))
	sleep 1
done

echo "后端已启动，按 Ctrl+C 停止。"
# macOS 自带 Bash 不支持 wait -n，等待后端进程结束即可。
wait "${BACKEND_PID}"
