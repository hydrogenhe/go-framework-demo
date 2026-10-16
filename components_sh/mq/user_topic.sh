#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bash "$script_dir/consumer_progress.sh" user_consumer_group_info
bash "$script_dir/consumer_progress.sh" user_consumer_group_notify
