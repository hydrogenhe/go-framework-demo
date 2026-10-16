package util

import "strconv"

// IDString 将数字 ID 转为 ES 文档 ID 或 keyword 查询值。
func IDString(id int64) string {
	return strconv.FormatInt(id, 10)
}
