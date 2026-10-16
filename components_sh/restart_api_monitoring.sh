#!/usr/bin/env bash
# 重启接口监控：Prometheus + Grafana，保留现有数据卷。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${PROJECT_DIR}/component/monitoring/docker-compose.yml"

command -v docker >/dev/null 2>&1 || { echo "未找到 docker，请先安装 Docker Desktop。" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "未找到 curl，无法检查服务就绪状态。" >&2; exit 1; }
docker compose version >/dev/null 2>&1 || { echo "需要 Docker Compose 插件（docker compose）。" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "无法连接 Docker，请先启动 Docker Desktop。" >&2; exit 1; }
[[ -f "${COMPOSE_FILE}" ]] || { echo "缺少配置：${COMPOSE_FILE}" >&2; exit 1; }

docker compose -f "${COMPOSE_FILE}" config --quiet
echo "重启接口监控：Prometheus + Grafana（保留数据）……"
docker compose -f "${COMPOSE_FILE}" up -d --force-recreate prometheus grafana

ready=false
for ((attempt = 1; attempt <= 30; attempt++)); do
    if curl -fsS --max-time 1 http://localhost:9090/-/ready >/dev/null 2>&1 &&
        curl -fsS --max-time 1 http://localhost:3000/api/health >/dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 1
done

docker compose -f "${COMPOSE_FILE}" ps
if [[ "${ready}" != true ]]; then
    echo "接口监控未在等待时间内就绪，最近容器日志如下：" >&2
    docker compose -f "${COMPOSE_FILE}" logs --tail 40 prometheus grafana
    exit 1
fi

echo "接口监控已就绪："
echo "Prometheus：http://localhost:9090"
echo "Grafana：   http://localhost:3000"
echo "Go 服务需单独运行，指标地址：http://localhost:8080/metrics"
echo "服务就绪检查通过；指标采集情况请查看 Prometheus 的 Targets 页面。"
