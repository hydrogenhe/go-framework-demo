#!/usr/bin/env bash
# 用法：bash components_sh/log/query.sh <log_id> [YYYY-MM-DD]
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$script_dir/query.py" "$@"
