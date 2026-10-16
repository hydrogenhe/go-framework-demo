#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTAINER_NAME="canal-server"
IMAGE="canal/canal-server:v1.1.8"
PORT="11111"
CONFIG_DIR="${PROJECT_DIR}/component/canal/canal_conf"

if ! docker image inspect "${IMAGE}" >/dev/null 2>&1; then
	echo "下载 Canal 镜像：${IMAGE}"
	docker pull "${IMAGE}"
fi

if docker container inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
	echo "移除旧 Canal 容器以应用项目配置：${CONTAINER_NAME}"
	docker rm -f "${CONTAINER_NAME}" >/dev/null
fi

echo "创建 Canal 容器：${CONTAINER_NAME}"
docker run -d \
	--name "${CONTAINER_NAME}" \
	--restart unless-stopped \
	-p "${PORT}:${PORT}" \
	-v "${CONFIG_DIR}:/home/admin/canal-server/conf" \
	"${IMAGE}" >/dev/null

sleep 2
if ! docker ps --filter "name=^/${CONTAINER_NAME}$" --filter status=running --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
	echo "Canal 启动失败，请查看日志：docker logs ${CONTAINER_NAME}" >&2
	exit 1
fi

echo "Canal 已启动：127.0.0.1:${PORT}"
