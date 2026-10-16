#!/usr/bin/env bash
# 一键启动监控中间件，不启动、不停止 Go 业务服务。
set -e
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"
docker compose up -d
echo "监控中间件已启动："
echo "Prometheus: http://localhost:9090"
echo "Grafana:    http://localhost:3000"
echo "Go 指标地址需由业务服务提供：http://localhost:8080/metrics"
