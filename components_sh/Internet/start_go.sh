#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
STACK_DIR="${SCRIPT_DIR}/stack"
ENV_FILE="${STACK_DIR}/.env"
COMPOSE_FILE="${STACK_DIR}/compose.yaml"

[[ -f "${ENV_FILE}" ]] || { echo "先运行 components_sh/Internet/start_set_all.sh 生成配置和凭据。" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker 服务未运行。" >&2; exit 1; }

docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" --profile app up -d --build --force-recreate go frontend
ready=false
for ((attempt=1; attempt<=30; attempt++)); do
  running_services="$(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" --profile app ps --status running --services)"
  if grep -qx go <<<"${running_services}" && grep -qx frontend <<<"${running_services}"; then
    ready=true
    break
  fi
  sleep 2
done
if [[ "${ready}" != true ]]; then
  docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" --profile app ps -a >&2
  docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" --profile app logs --tail 80 go frontend >&2
  echo "Go 后端或前端未能保持运行，请根据容器日志排查后重新运行本脚本。" >&2
  exit 1
fi
echo "Go 后端与前端已构建并启动。前端：http://<服务器公网IP>:5500；API：http://<服务器公网IP>:8080"
echo "查看状态：docker compose --env-file '${ENV_FILE}' -f '${COMPOSE_FILE}' --profile app ps go frontend"
echo "查看日志：docker compose --env-file '${ENV_FILE}' -f '${COMPOSE_FILE}' --profile app logs -f go frontend"
