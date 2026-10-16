#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
STACK_DIR="${SCRIPT_DIR}/stack"
COMPOSE_FILE="${STACK_DIR}/compose.yaml"
ENV_FILE="${STACK_DIR}/.env"

command -v docker >/dev/null 2>&1 || { echo "需要先安装 Docker Engine 和 Docker Compose 插件。" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker 服务未运行。" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "需要 Python 3 来生成安全随机密码和 YAML 配置。" >&2; exit 1; }

if command -v sysctl >/dev/null 2>&1; then
  map_count="$(sysctl -n vm.max_map_count 2>/dev/null || echo 0)"
  if [[ "${map_count}" =~ ^[0-9]+$ ]] && (( map_count < 262144 )); then
    echo "提示：Elasticsearch 建议将宿主机 vm.max_map_count 设为 262144。Ubuntu 可执行：sudo sysctl -w vm.max_map_count=262144"
  fi
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  public_host="${PUBLIC_HOST:-}"
  if [[ -z "${public_host}" ]]; then
    read -r -p "输入云服务器公网 IP 或域名（用于图片访问，留空使用本机地址）: " public_host
  fi
  public_host="${public_host:-127.0.0.1}"
  PUBLIC_HOST="${public_host}" python3 - "${ENV_FILE}" <<'PY'
import os, secrets, sys
path = sys.argv[1]
host = os.environ["PUBLIC_HOST"].strip()
values = {
    "PUBLIC_HOST": host,
    "MYSQL_ROOT_PASSWORD": secrets.token_hex(20),
    "MYSQL_PASSWORD": secrets.token_hex(20),
    "CANAL_PASSWORD": secrets.token_hex(20),
    "REDIS_PASSWORD": secrets.token_hex(20),
    "IMAGE_ACCESS_KEY": "img" + secrets.token_hex(12),
    "IMAGE_SECRET_KEY": secrets.token_hex(32),
}
with open(path, "w", encoding="utf-8") as f:
    for key, value in values.items():
        f.write(f"{key}={value}\n")
os.chmod(path, 0o600)
PY
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

python3 - "${PROJECT_DIR}" "${PUBLIC_HOST}" "${MYSQL_PASSWORD}" "${REDIS_PASSWORD}" "${CANAL_PASSWORD}" "${IMAGE_ACCESS_KEY}" "${IMAGE_SECRET_KEY}" <<'PY'
from pathlib import Path
import re, sys
root = Path(sys.argv[1])
host, mysql_password, redis_password, canal_password, image_access, image_secret = sys.argv[2:]

def replace(path, pattern, replacement):
    file = root / path
    text = file.read_text()
    updated, count = re.subn(pattern, replacement, text, count=1, flags=re.M)
    if count != 1:
        raise SystemExit(f"无法在 {path} 中定位配置项: {pattern}")
    file.write_text(updated)

replace("conf/mysql.yaml", r"^(  host:\s*).*$", r"\g<1>mysql")
replace("conf/mysql.yaml", r"^(  port:\s*).*$", '  port: "3306"')
replace("conf/mysql.yaml", r"^(  user:\s*).*$", r"\g<1>demo_app")
replace("conf/mysql.yaml", r"^(  password:\s*).*$", f"  password: {mysql_password}")
replace("conf/redis.yaml", r"^(  address:\s*).*$", r"\g<1>redis:6379")
replace("conf/redis.yaml", r"^(  password:\s*).*$", f"  password: {redis_password}")
replace("conf/es.yaml", r"^(  addresses:\s*).*$", r'  addresses: ["http://elasticsearch:9200"]')
replace("conf/rocketmq.yaml", r'^(  name_srv_addr:\s*).*$', '  name_srv_addr: "rocketmq-namesrv:9876"')
replace("conf/rocketmq.yaml", r'^(  broker_addr:\s*).*$', '  broker_addr: "rocketmq-broker:10911"')
replace("conf/image_storage.yaml", r"^(  endpoint:\s*).*$", r"\g<1>http://seaweedfs:8333")
replace("conf/image_storage.yaml", r"^(  public_url:\s*).*$", f"  public_url: http://{host}:8888/buckets/product-images")
replace("conf/image_storage.yaml", r"^(  access_key:\s*).*$", f"  access_key: {image_access}")
replace("conf/image_storage.yaml", r"^(  secret_key:\s*).*$", f"  secret_key: {image_secret}")
replace("conf/app.yaml", r"^(  api_base_url:\s*).*$", f"  api_base_url: http://{host}:8080")

# Canal reads MySQL binlog and sends CDC events to the same RocketMQ network.
instance = root / "component/canal/canal_conf/example/instance.properties"
text = instance.read_text()
for key, value in {
    "canal.instance.master.address": "mysql:3306",
    "canal.instance.dbUsername": "canal",
    "canal.instance.dbPassword": canal_password,
}.items():
    text, n = re.subn(rf"^{re.escape(key)}=.*$", f"{key}={value}", text, count=1, flags=re.M)
    if not n:
        raise SystemExit(f"Canal 配置缺少 {key}")
instance.write_text(text)
canal = root / "component/canal/canal_conf/canal.properties"
text = canal.read_text()
text, n = re.subn(r"^rocketmq.namesrv.addr\s*=.*$", "rocketmq.namesrv.addr = rocketmq-namesrv:9876", text, count=1, flags=re.M)
if not n:
    raise SystemExit("Canal 配置缺少 rocketmq.namesrv.addr")
canal.write_text(text)

for name in ("mysql.yaml", "redis.yaml", "image_storage.yaml"):
    (root / "conf" / name).chmod(0o600)
(root / "component/canal/canal_conf/example/instance.properties").chmod(0o600)
(root / "component/canal/canal_conf/canal.properties").chmod(0o600)
PY

chmod +x "${STACK_DIR}/mysql-init/01-create-canal-user.sh"
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" config --quiet
compose=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
"${compose[@]}" up -d mysql redis elasticsearch rocketmq-namesrv rocketmq-broker seaweedfs

ready=false
for ((attempt=1; attempt<=60; attempt++)); do
  if "${compose[@]}" exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot -p"${MYSQL_ROOT_PASSWORD}" --silent >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 2
done
if [[ "${ready}" != true ]]; then
  "${compose[@]}" logs --tail 80 mysql >&2
  echo "MySQL 未能在 120 秒内就绪。" >&2
  exit 1
fi

# 初始化缺失的业务表，再创建或更新业务库账号与 Canal binlog 读取账号。
# SQL 使用 IF NOT EXISTS，重复运行不会覆盖已有表或数据。
"${compose[@]}" exec -T mysql sh -c 'mysql --protocol=socket -uroot -p"$MYSQL_ROOT_PASSWORD"' < "${STACK_DIR}/mysql-init/00-create-tables.sql"
"${compose[@]}" exec -T mysql sh -s < "${STACK_DIR}/mysql-init/01-create-canal-user.sh"
"${compose[@]}" up -d --force-recreate canal

services=(mysql redis elasticsearch rocketmq-namesrv rocketmq-broker seaweedfs canal)
ready=false
for ((attempt=1; attempt<=60; attempt++)); do
  running_services="$("${compose[@]}" ps --status running --services)"
  all_running=true
  for service in "${services[@]}"; do
    if ! grep -qx "${service}" <<<"${running_services}"; then
      all_running=false
      break
    fi
  done
  if [[ "${all_running}" == true ]]; then
    ready=true
    break
  fi
  sleep 2
done
if [[ "${ready}" != true ]]; then
  "${compose[@]}" ps -a >&2
  "${compose[@]}" logs --tail 80 >&2
  echo "有中间件未能保持运行，请根据上方容器状态和日志排查后重新运行本脚本。" >&2
  exit 1
fi

echo "基础组件已启动。配置和随机凭据已写入 conf/*.yaml 与 components_sh/Internet/stack/.env。"
echo "下一步运行：${SCRIPT_DIR}/start_go.sh"
