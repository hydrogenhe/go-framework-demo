#!/usr/bin/env bash
# 只读查看消费进度：bash components_sh/mq/consumer_progress.sh [消费者组]
set -euo pipefail

group="${1:-product_es_consumer_group}"
namesrv="${MQ_NAMESRV:-127.0.0.1:9876}"
admin="${MQADMIN:-${ROCKETMQ_HOME:-/usr/local/rocketmq}/bin/mqadmin}"
if [[ ! -x "$admin" ]]; then
  printf '找不到 mqadmin：%s\n请用 MQADMIN 指定路径。\n' "$admin" >&2
  exit 1
fi
output="$(mktemp)"
trap 'rm -f "$output"' EXIT
if ! "$admin" consumerProgress -n "$namesrv" -g "$group" >"$output" 2>&1; then
  printf '消费进度查询失败：\n' >&2
  cat "$output" >&2
  exit 1
fi

color=0
[[ -t 1 && -z "${NO_COLOR:-}" ]] && color=1
awk -v group="$group" -v color="$color" '
  BEGIN {
    if (color) { bold="\033[1m"; reset="\033[0m"; yellow="\033[33m"; green="\033[32m" }
  }
  /^#Topic/ { header=1; next }
  header && NF >= 8 && $3 ~ /^[0-9]+$/ && $4 ~ /^[0-9]+$/ && $5 ~ /^[0-9]+$/ && $6 ~ /^[0-9]+$/ {
    if (!seen[$1]++) topics[++count]=$1
    topic=$1; n=++rows[topic]
    details[topic,n]=sprintf("  | %4s | %10s | %10s | %-19s |", $3, $6, $7, ($8 == "N/A" ? "--" : $8 " " $9))
    if (!seenBroker[topic,$2]++) brokers[topic]=brokers[topic] (brokers[topic] == "" ? "" : ", ") $2
    backlog[topic]+=$6; total+=$6; inflight+=$7
  }
  /^Consume TPS:/ { tps=$3 }
  END {
    if (!count) { print "未识别到队列数据，请检查消费者组或 mqadmin 输出。"; exit 1 }
    printf "\n%sRocketMQ 消费进度%s\n", bold, reset
    printf "消费者组：%s\n", group
    printf "\n%s待消费 %d%s  |  处理中 %d  |  速率 %s 条/秒\n", (total ? yellow : green), total, reset, inflight, (tps == "" ? "未知" : tps)
    for (i=1; i<=count; i++) {
      topic=topics[i]
      printf "\n%s%s%s\n", bold, topic, reset
      printf "  Broker：%s  |  待消费合计：%d\n", brokers[topic], backlog[topic]
      print "  +------+------------+------------+---------------------+"
      print "  | 队列 |     待消费 |     处理中 | 最近消费            |"
      print "  +------+------------+------------+---------------------+"
      for (j=1; j<=rows[topic]; j++) {
        print details[topic,j]
      }
      print "  +------+------------+------------+---------------------+"
    }
    print "\n待消费 = Broker 与消费者位点之差；处理中来自 Broker 上报。"
  }
' "$output"
