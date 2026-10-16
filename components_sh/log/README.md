# 按 Log ID 查看日志

```bash
# 默认查最新日期的 logs/app-YYYY-MM-DD.log（不是按修改时间选择）。
bash components_sh/log/query.sh 561b81a8-d28e-43fd-a5b4-e052bc075f87

# 指定日期。
bash components_sh/log/query.sh 561b81a8-d28e-43fd-a5b4-e052bc075f87 2026-10-14
```

只读、精确匹配 JSON 的 `log_id`。每条日志单独显示表格及原文件行号，SQL 自动换行，中英文按终端字符宽度对齐，HTTP 请求耗时由秒转换为毫秒。未知字段保留展示。

支持从任意工作目录执行绝对路径；依赖 Python 3，无第三方库。`--log-dir /日志目录` 可更换目录。
未匹配返回退出码 1，参数或文件错误返回 2。没有 log_id 的记录不会被归入这条请求。
