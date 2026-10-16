#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DASHBOARD_COMPOSE="${PROJECT_DIR}/component/rocketmq-dashboard/compose.yaml"
DASHBOARD_PORT="${DASHBOARD_PORT:-8082}"
# Docker 中的 localhost 指容器本身，将本机 NameServer 地址替换为宿主机入口。
DASHBOARD_NAMESRV_ADDR="${DASHBOARD_NAMESRV_ADDR:-${NAMESRV_ADDR:-127.0.0.1:9876}}"
DASHBOARD_NAMESRV_ADDR="${DASHBOARD_NAMESRV_ADDR//127.0.0.1/host.docker.internal}"
DASHBOARD_NAMESRV_ADDR="${DASHBOARD_NAMESRV_ADDR//localhost/host.docker.internal}"
export DASHBOARD_PORT DASHBOARD_NAMESRV_ADDR

ROCKETMQ_HOME="${ROCKETMQ_HOME:-/usr/local/rocketmq}"
NAMESRV_ADDR="${NAMESRV_ADDR:-127.0.0.1:9876}"
NAMESRV_PORT="${NAMESRV_PORT:-9876}"
BROKER_PORT="${BROKER_PORT:-10911}"
NAMESRV_LOG="${NAMESRV_LOG:-$HOME/namesrv.log}"
BROKER_LOG="${BROKER_LOG:-$HOME/broker.log}"
BROKER_CONF="${BROKER_CONF:-$ROCKETMQ_HOME/conf/broker.conf}"
BROKER_STORE_PATH="${BROKER_STORE_PATH:-$HOME/store}"
BROKER_LOCK_FILE="$BROKER_STORE_PATH/lock"
BROKER_ADVERTISE_IP="${BROKER_ADVERTISE_IP:-}"
BROKER_ADVERTISE_INTERFACE="${BROKER_ADVERTISE_INTERFACE:-}"

if [[ ! -x "$ROCKETMQ_HOME/bin/mqshutdown" ]]; then
  echo "RocketMQ 未找到：$ROCKETMQ_HOME" >&2
  echo "可通过 ROCKETMQ_HOME=/path/to/rocketmq 覆盖默认路径。" >&2
  exit 1
fi
if [[ ! -f "$BROKER_CONF" ]]; then
  echo "RocketMQ Broker 配置不存在：$BROKER_CONF" >&2
  exit 1
fi

is_usable_ipv4() {
  local ip="$1"
  [[ "$ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || return 1
  [[ "$ip" != 127.* && "$ip" != 169.254.* && "$ip" != 0.* ]]
}

detect_broker_advertise_ip() {
  local interface ip

  if [[ -n "$BROKER_ADVERTISE_IP" ]]; then
    if ! is_usable_ipv4 "$BROKER_ADVERTISE_IP"; then
      echo "BROKER_ADVERTISE_IP 不是可用的 IPv4 地址：$BROKER_ADVERTISE_IP" >&2
      return 1
    fi
    printf '%s\n' "$BROKER_ADVERTISE_IP"
    return 0
  fi

  if [[ -n "$BROKER_ADVERTISE_INTERFACE" ]]; then
    interface="$BROKER_ADVERTISE_INTERFACE"
    ip="$(ipconfig getifaddr "$interface" 2>/dev/null || true)"
    if is_usable_ipv4 "$ip"; then
      printf '%s\n' "$ip"
      return 0
    fi
    echo "网卡 $interface 没有可用的 IPv4 地址。" >&2
    return 1
  fi

  # 优先从 macOS 的 Wi-Fi/以太网硬件网卡取地址，避开 VPN 的 utun 和 Docker 虚拟网卡。
  while IFS= read -r interface; do
    [[ -n "$interface" ]] || continue
    ip="$(ipconfig getifaddr "$interface" 2>/dev/null || true)"
    if is_usable_ipv4 "$ip"; then
      printf '%s\n' "$ip"
      return 0
    fi
  done < <(networksetup -listallhardwareports | awk '
    /^Hardware Port:/ { port = $0; next }
    /^Device:/ {
      if (port ~ /Wi-Fi|Ethernet|LAN/) print $2
      port = ""
    }
  ')

  echo "无法从活动的 Wi-Fi/以太网网卡获取 Broker 通告 IP。可设置 BROKER_ADVERTISE_INTERFACE 或 BROKER_ADVERTISE_IP 后重试。" >&2
  return 1
}

write_broker_advertise_ip() {
  local ip="$1"
  if grep -Eq '^[[:space:]]*brokerIP1[[:space:]]*=' "$BROKER_CONF"; then
    sed -i '' -E "s/^[[:space:]]*brokerIP1[[:space:]]*=.*/brokerIP1 = $ip/" "$BROKER_CONF"
  else
    printf '\nbrokerIP1 = %s\n' "$ip" >> "$BROKER_CONF"
  fi
}

verify_canal_can_reach_broker() {
  local running
  command -v docker >/dev/null 2>&1 || return 0
  running="$(docker inspect -f '{{.State.Running}}' canal-server 2>/dev/null || true)"
  [[ "$running" == "true" ]] || return 0

  echo "从 Canal 容器验证 Broker 地址 $BROKER_ADVERTISE_IP:$BROKER_PORT..."
  if ! docker exec canal-server sh -c "nc -z -w 3 '$BROKER_ADVERTISE_IP' '$BROKER_PORT'"; then
    echo "Canal 容器无法连接 Broker 地址 ${BROKER_ADVERTISE_IP}:${BROKER_PORT}。请检查 VPN、本机防火墙和 Docker 网络。" >&2
    return 1
  fi
}

port_in_use() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
}

wait_port_state() {
  local port="$1"
  local expected="$2"
  local attempts=30

  while (( attempts > 0 )); do
    if [[ "$expected" == "up" ]] && port_in_use "$port"; then
      return 0
    fi
    if [[ "$expected" == "down" ]] && ! port_in_use "$port"; then
      return 0
    fi
    sleep 1
    attempts=$((attempts - 1))
  done
  return 1
}

wait_broker_lock_release() {
  local attempts=30

  while (( attempts > 0 )); do
    if [[ ! -e "$BROKER_LOCK_FILE" ]] || [[ -z "$(lsof -t "$BROKER_LOCK_FILE" 2>/dev/null || true)" ]]; then
      return 0
    fi
    sleep 1
    attempts=$((attempts - 1))
  done
  return 1
}

# 在停止 MQ 前检查 Dashboard 依赖并下载镜像，下载失败不影响现有 MQ。
command -v docker >/dev/null 2>&1 || { echo "Dashboard 需要 Docker Desktop。" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "Dashboard 就绪检查需要 curl。" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "请先启动 Docker Desktop。" >&2; exit 1; }
docker compose -f "$DASHBOARD_COMPOSE" config --quiet
if ! docker image inspect apacherocketmq/rocketmq-dashboard:2.1.0 >/dev/null 2>&1; then
  docker compose -f "$DASHBOARD_COMPOSE" pull dashboard
fi

BROKER_ADVERTISE_IP="$(detect_broker_advertise_ip)"

echo "停止 RocketMQ Broker..."
"$ROCKETMQ_HOME/bin/mqshutdown" broker || true
echo "停止 RocketMQ NameServer..."
"$ROCKETMQ_HOME/bin/mqshutdown" namesrv || true

# RocketMQ Broker 还有内部 HA/Store 线程需要异步释放端口，不能只看 10911。
sleep 5

if ! wait_port_state "$BROKER_PORT" down; then
  echo "Broker 端口 $BROKER_PORT 仍被占用，停止失败。" >&2
  exit 1
fi
if ! wait_port_state "$NAMESRV_PORT" down; then
  echo "NameServer 端口 $NAMESRV_PORT 仍被占用，停止失败。" >&2
  exit 1
fi
if ! wait_broker_lock_release; then
  echo "Broker store 锁仍被占用：$BROKER_LOCK_FILE。请确认没有其他 Broker 使用该 store。" >&2
  exit 1
fi

cd "$ROCKETMQ_HOME"
write_broker_advertise_ip "$BROKER_ADVERTISE_IP"
echo "Broker 通告地址：${BROKER_ADVERTISE_IP}:${BROKER_PORT}（已写入 ${BROKER_CONF}）"
echo "Broker 配置文件：$BROKER_CONF"
echo "启动 RocketMQ NameServer..."
nohup sh bin/mqnamesrv >"$NAMESRV_LOG" 2>&1 &
if ! wait_port_state "$NAMESRV_PORT" up; then
  echo "NameServer 启动失败，请检查：$NAMESRV_LOG" >&2
  exit 1
fi

echo "启动 RocketMQ Broker..."
nohup sh bin/mqbroker \
  -n "$NAMESRV_ADDR" \
  -c "$BROKER_CONF" \
  >"$BROKER_LOG" 2>&1 &
if ! wait_port_state "$BROKER_PORT" up; then
  echo "Broker 启动失败，请检查：$BROKER_LOG" >&2
  echo "--- Broker 日志末尾 ---" >&2
  tail -30 "$BROKER_LOG" >&2 || true
  exit 1
fi
verify_canal_can_reach_broker

echo "启动 RocketMQ Dashboard..."
docker compose -f "$DASHBOARD_COMPOSE" up -d --force-recreate dashboard
ready=false
for ((attempt=1; attempt<=60; attempt++)); do
  if curl -fsS --max-time 2 "http://127.0.0.1:${DASHBOARD_PORT}/" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  echo "Dashboard 未就绪，MQ 已启动；请检查以下日志。" >&2
  docker compose -f "$DASHBOARD_COMPOSE" logs --tail 40 dashboard
  exit 1
fi
# 页面可打开还不代表 MQ 通信正常，继续验证集群查询。
cluster_response="$(curl -fsS --max-time 15 "http://127.0.0.1:${DASHBOARD_PORT}/cluster/list.query")" || {
  echo "Dashboard 页面已启动，但集群查询失败。" >&2
  exit 1
}
if ! printf '%s' "$cluster_response" | grep -Eq '"status"[[:space:]]*:[[:space:]]*0' ||
   ! printf '%s' "$cluster_response" | grep -q '"brokerAddrTable"'; then
  echo "Dashboard 无法读取 MQ 集群，请检查 NameServer 配置和 Broker 通告地址。" >&2
  docker compose -f "$DASHBOARD_COMPOSE" logs --tail 30 dashboard
  exit 1
fi
echo "Dashboard：http://localhost:${DASHBOARD_PORT}（NameServer：${DASHBOARD_NAMESRV_ADDR}）"

echo "RocketMQ 重启成功：NameServer ${NAMESRV_PORT}，Broker ${BROKER_PORT}"
echo "NameServer 日志：${NAMESRV_LOG}"
echo "Broker 日志：${BROKER_LOG}"
