#!/usr/bin/env bash

# 检查项目依赖服务是否就绪。
# 只读检查，不会启动、停止或修改任何服务。

MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
REDIS_HOST="127.0.0.1"
REDIS_PORT="6379"
ES_HOST="127.0.0.1"
ES_PORT="9200"
ROCKETMQ_NAMESRV_HOST="127.0.0.1"
ROCKETMQ_NAMESRV_PORT="9876"
ROCKETMQ_BROKER_HOST="127.0.0.1"
ROCKETMQ_BROKER_PORT="10911"
CANAL_HOST="127.0.0.1"
CANAL_PORT="11111"
SEAWEEDFS_HOST="127.0.0.1"
SEAWEEDFS_S3_PORT="8333"
SEAWEEDFS_FILER_PORT="8888"
SEAWEEDFS_MASTER_PORT="9333"
SEAWEEDFS_BUCKET="product-images"
PROMETHEUS_HOST="127.0.0.1"
PROMETHEUS_PORT="9090"
GRAFANA_HOST="127.0.0.1"
GRAFANA_PORT="3000"

check_docker() {
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1
}

check_mysql() {
  if command -v mysqladmin >/dev/null 2>&1; then
    mysqladmin ping -h"$MYSQL_HOST" -P"$MYSQL_PORT" -uroot -p12345678 --silent >/dev/null 2>&1
  elif command -v nc >/dev/null 2>&1; then
    nc -z "$MYSQL_HOST" "$MYSQL_PORT" >/dev/null 2>&1
  else
    return 1
  fi
}

check_redis() {
  local cli=""
  if command -v redis-cli >/dev/null 2>&1; then
    cli="redis-cli"
  elif [ -x /opt/homebrew/opt/redis/bin/redis-cli ]; then
    cli="/opt/homebrew/opt/redis/bin/redis-cli"
  fi
  [ -n "$cli" ] && [ "$("$cli" -h "$REDIS_HOST" -p "$REDIS_PORT" ping 2>/dev/null)" = "PONG" ]
}

check_http() {
  curl -fsS --max-time 3 "$1" >/dev/null 2>&1
}

check_tcp() {
  nc -z -w 3 "$1" "$2" >/dev/null 2>&1
}

# 启用认证的 S3 对匿名列举存储桶返回 403，说明服务已响应。
# 同时检查 Filer 存储桶和 Master，不仅检查 TCP 端口。
check_seaweedfs() {
  local status
  status="$(curl --silent --max-time 3 --output /dev/null --write-out '%{http_code}' \
    "http://$SEAWEEDFS_HOST:$SEAWEEDFS_S3_PORT/" 2>/dev/null)" || return 1
  [[ "$status" == "200" || "$status" == "403" ]] && \
    check_http "http://$SEAWEEDFS_HOST:$SEAWEEDFS_FILER_PORT/buckets/$SEAWEEDFS_BUCKET/" && \
    check_http "http://$SEAWEEDFS_HOST:$SEAWEEDFS_MASTER_PORT/cluster/status"
}

check_rocketmq() {
  local admin=""
  if command -v mqadmin >/dev/null 2>&1; then
    admin="mqadmin"
  elif [ -x "${ROCKETMQ_HOME:-}/bin/mqadmin" ]; then
    admin="${ROCKETMQ_HOME}/bin/mqadmin"
  fi
  if [ -n "$admin" ]; then
    "$admin" clusterList -n "$ROCKETMQ_NAMESRV_HOST:$ROCKETMQ_NAMESRV_PORT" 2>/dev/null \
      | grep -q "$ROCKETMQ_BROKER_HOST"
  else
    check_tcp "$ROCKETMQ_NAMESRV_HOST" "$ROCKETMQ_NAMESRV_PORT" && \
      check_tcp "$ROCKETMQ_BROKER_HOST" "$ROCKETMQ_BROKER_PORT"
  fi
}

print_status() {
  local label="$1"
  local check="$2"
  shift 2
  if "$check" "$@"; then
    printf '%s已启动\n' "$label"
  else
    printf '%s已失败\n' "$label"
  fi
}

print_status "Docker" check_docker
print_status "Prometheus($PROMETHEUS_PORT)" check_http "http://$PROMETHEUS_HOST:$PROMETHEUS_PORT/-/ready"
print_status "Grafana($GRAFANA_PORT)" check_http "http://$GRAFANA_HOST:$GRAFANA_PORT/api/health"
print_status "MySQL($MYSQL_PORT)" check_mysql
print_status "Redis($REDIS_PORT)" check_redis
print_status "Elasticsearch($ES_PORT)" check_http "http://$ES_HOST:$ES_PORT"
print_status "SeaweedFS(S3:${SEAWEEDFS_S3_PORT}, Filer:${SEAWEEDFS_FILER_PORT}, Master:${SEAWEEDFS_MASTER_PORT})" check_seaweedfs
# RocketMQ 同时检查 NameServer 和 Broker 两个端口。
if check_rocketmq; then
  printf 'RocketMQ 已正常运行（NameServer(%s)、Broker(%s)）\n' "$ROCKETMQ_NAMESRV_PORT" "$ROCKETMQ_BROKER_PORT"
else
  printf 'RocketMQ 检查失败（请检查 NameServer/Broker 注册地址）\n'
fi
# Canal Server 的 11111 是 TCP 服务端口。
if check_tcp "$CANAL_HOST" "$CANAL_PORT"; then
  printf 'Canal(%s)端口已监听\n' "$CANAL_PORT"
else
  printf 'Canal(%s)检查失败\n' "$CANAL_PORT"
fi
