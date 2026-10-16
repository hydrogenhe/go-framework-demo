#!/usr/bin/env bash
# 商品业务 Topic 当前没有消费者，展示存储位点，不推算消费积压。
set -euo pipefail
namesrv="${MQ_NAMESRV:-127.0.0.1:9876}"
admin="${MQADMIN:-${ROCKETMQ_HOME:-/usr/local/rocketmq}/bin/mqadmin}"
output="$(mktemp)"
trap 'rm -f "$output"' EXIT
if ! "$admin" topicStatus -n "$namesrv" -t product_topic >"$output" 2>&1; then
  printf 'product_topic 查询失败（请确认 Topic 已注册）：\n' >&2
  cat "$output" >&2
  exit 1
fi
awk '
  BEGIN { print "\nproduct_topic\n未配置消费者；以下为存储位点，不能据此判断待消费数量。" }
  /^#Broker/ { header=1; next }
  header && $2 ~ /^[0-9]+$/ && $3 ~ /^[0-9]+$/ && $4 ~ /^[0-9]+$/ {
    if ($1 != broker) {
      if (broker != "") print "  +------+------------+------------+---------------------+"
      broker=$1
      printf "\n  Broker：%s\n", broker
      print "  +------+------------+------------+---------------------+"
      print "  | 队列 |   最小位点 |   最大位点 | 最近写入            |"
      print "  +------+------------+------------+---------------------+"
    }
    printf "  | %4s | %10s | %10s | %-19s |\n", $2, $3, $4, (NF >= 6 ? $5 " " substr($6,1,8) : "--")
    count++
  }
  END {
    if (!count) { print "未识别到队列数据。"; exit 1 }
    print "  +------+------------+------------+---------------------+"
  }
' "$output"
