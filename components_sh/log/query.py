#!/usr/bin/env python3
"""只读查询 JSON 行日志，以终端显示宽度换行，精确匹配 log_id。"""
import argparse
import datetime
import json
import re
import shutil
import sys
import unicodedata
from pathlib import Path


LABELS = {
    "ts": "时间", "level": "级别", "msg": "消息", "source": "来源",
    "sql": "SQL", "rows_affected": "影响行数", "elapsed_ms": "SQL耗时(ms)",
    "error": "错误", "method": "请求方法", "path": "请求路径",
    "status": "HTTP状态", "latency": "请求耗时(ms)", "client_ip": "客户端IP",
    "topic": "Topic", "msg_id": "消息ID", "consumer_group": "消费者组",
    "client_id": "客户端ID", "result": "消费结果", "order_id": "订单ID",
    "order_no": "订单号", "user_id": "用户ID", "product_id": "商品ID",
}


def width(text):
    return sum(0 if unicodedata.combining(c) else
               2 if unicodedata.east_asian_width(c) in "WF" else 1 for c in text)


def safe(text):
    # 日志中的控制字符不得改变终端状态，保留换行用于展示。
    return "".join(c if c == "\n" or not unicodedata.category(c).startswith("C")
                   else " " for c in str(text))


def wrap(text, limit):
    lines = []
    for paragraph in safe(text).split("\n"):
        line, used = "", 0
        for c in paragraph:
            n = width(c)
            if line and used + n > limit:
                lines.append(line)
                line, used = "", 0
            line += c
            used += n
        lines.append(line)
    return lines


def pad(text, size):
    return text + " " * max(0, size - width(text))


def table(rows, columns):
    key_size = min(18, max(12, max(width(k) for k, _ in rows)))
    value_size = max(12, columns - key_size - 7)
    edge = "+" + "-" * (key_size + 2) + "+" + "-" * (value_size + 2) + "+"
    print(edge)
    for key, value in rows:
        keys, values = wrap(key, key_size), wrap(value, value_size)
        for i in range(max(len(keys), len(values))):
            print("| " + pad(keys[i] if i < len(keys) else "", key_size) + " | " +
                  pad(values[i] if i < len(values) else "", value_size) + " |")
        print(edge)


def value_text(key, value):
    if key in ("latency", "elapsed_ms") and isinstance(value, (float, int)):
        return f"{value * 1000 if key == 'latency' else value:.3f}"
    if isinstance(value, (dict, list)):
        return json.dumps(value, ensure_ascii=False)
    if value is None:
        return "null"
    if isinstance(value, bool):
        return str(value).lower()
    return safe(value)


def main():
    parser = argparse.ArgumentParser(description="按 log_id 查询，默认选择最新日期的 app 日志")
    parser.add_argument("log_id", help="精确匹配的 log ID")
    parser.add_argument("date", nargs="?", help="指定日期 YYYY-MM-DD")
    parser.add_argument("--log-dir", type=Path,
                        default=Path(__file__).resolve().parents[2] / "logs")
    args = parser.parse_args()
    if args.date:
        try:
            if not re.fullmatch(r"\d{4}-\d{2}-\d{2}", args.date):
                raise ValueError()
            datetime.date.fromisoformat(args.date)
        except ValueError:
            parser.error("日期格式应为 YYYY-MM-DD，且必须是有效日期")
        path = args.log_dir / f"app-{args.date}.log"
    else:
        candidates = sorted(p for p in args.log_dir.glob("app-*.log")
                            if re.fullmatch(r"app-\d{4}-\d{2}-\d{2}\.log", p.name))
        if not candidates:
            parser.error(f"未找到 app 日志：{args.log_dir}")
        path = candidates[-1]
    matches, malformed = [], 0
    try:
        with path.open(encoding="utf-8", errors="replace") as log:
            for number, line in enumerate(log, 1):
                try:
                    entry = json.loads(line)
                except json.JSONDecodeError:
                    if args.log_id in line:
                        malformed += 1
                    continue
                if isinstance(entry, dict) and entry.get("log_id") == args.log_id:
                    matches.append((number, entry))
    except OSError as err:
        parser.error(f"无法读取日志：{err}")
    print(f"\n日志：{path}\nLog ID：{safe(args.log_id)}\n匹配：{len(matches)} 条")
    columns = min(110, max(36, shutil.get_terminal_size(fallback=(100, 24)).columns))
    for index, (number, entry) in enumerate(matches, 1):
        print(f"\n记录 {index} / {len(matches)} · 原文件第 {number} 行")
        keys = [k for k in ("ts", "level", "msg") if k in entry]
        keys.extend(k for k in entry if k not in ("ts", "level", "msg", "log_id"))
        rows = [(LABELS.get(k, safe(k)), value_text(k, entry[k])) for k in keys]
        table(rows, columns)
    if malformed:
        print(f"\n另有 {malformed} 条包含该 ID 的非完整 JSON 行，未计入精确匹配。")
    if not matches:
        print("\n该文件未找到此 log_id，可指定其他日期查询。")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
