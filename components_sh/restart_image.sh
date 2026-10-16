#!/usr/bin/env bash
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_DIR="$PROJECT_ROOT/component/Image_component"
cd "$IMAGE_DIR"
if [[ $# -gt 1 || ( $# -eq 1 && "$1" != "--stop" ) ]]; then
  echo "用法：$0 [--stop]" >&2
  exit 1
fi
umask 077
mkdir -p data
IMAGE_CONFIG="$PROJECT_ROOT/conf/image_storage.yaml"
[[ -f "$IMAGE_CONFIG" ]] || { echo "缺少配置文件：$IMAGE_CONFIG" >&2; exit 1; }
image_access_key="$(awk '$1 == "access_key:" {print $2; exit}' "$IMAGE_CONFIG")"
image_secret_key="$(awk '$1 == "secret_key:" {print $2; exit}' "$IMAGE_CONFIG")"
image_bucket="$(awk '$1 == "bucket:" {print $2; exit}' "$IMAGE_CONFIG")"
if [[ ! "$image_access_key" =~ ^[a-zA-Z0-9-]+$ || ! "$image_secret_key" =~ ^[a-zA-Z0-9-]+$ ]]; then
  echo 'conf/image_storage.yaml 中图片存储凭据格式无效' >&2
  exit 1
fi
[[ -n "$image_bucket" ]] || { echo 'conf/image_storage.yaml 中 bucket 未配置' >&2; exit 1; }
export AWS_ACCESS_KEY_ID="$image_access_key"
export AWS_SECRET_ACCESS_KEY="$image_secret_key"
export S3_BUCKET="$image_bucket"
docker info >/dev/null
docker compose down --timeout 10
if [[ "${1:-}" == '--stop' ]]; then
  echo 'SeaweedFS 已停止，图片数据保留在 component/Image_component/data'
  exit 0
fi
docker compose up -d
for ((attempt=0; attempt<60; attempt++)); do
  s3_status="$(curl --silent --max-time 2 --output /dev/null --write-out '%{http_code}' http://127.0.0.1:8333/ || true)"
  if [[ "$s3_status" == 200 || "$s3_status" == 403 ]] && curl --silent --fail --max-time 2 http://127.0.0.1:9333/cluster/status >/dev/null && curl --silent --fail --max-time 2 http://127.0.0.1:8888/ >/dev/null; then
    echo 'SeaweedFS 已就绪：S3 http://localhost:8333，文件 http://localhost:8888，管理 http://localhost:23646'
    exit 0
  fi
  sleep 1
done
docker compose logs --tail 30 >&2
echo 'SeaweedFS 未在时限内就绪' >&2
exit 1
