package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	appconst "go-framework-demo/const"
)

var logFilePattern = regexp.MustCompile(`^app-(\d{4}-\d{2}-\d{2})\.log$`)

// LogServiceInterface 列出日志查询业务的全部公开方法。
type LogServiceInterface interface {
	Query(ctx context.Context, input *LogQuery) (*LogResult, error)
}

type LogService struct{ directory string }

var _ LogServiceInterface = (*LogService)(nil)

func NewLogService(directory string) *LogService { return &LogService{directory: directory} }

type LogField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}
type LogEntry struct {
	LogID   string     `json:"log_id"`
	Line    int        `json:"line"`
	Time    string     `json:"time"`
	Level   string     `json:"level"`
	Message string     `json:"message"`
	Fields  []LogField `json:"fields"`
}
type LogResult struct {
	Keyword  string     `json:"keyword"`
	Level    string     `json:"level"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Date     string     `json:"date"`
	File     string     `json:"file"`
	Total    int        `json:"total"`
	Items    []LogEntry `json:"items"`
}

// LogQuery 是日志关键词、日期、级别及分页条件。
type LogQuery struct {
	Keyword  string `json:"keyword"`
	Date     string `json:"date"`
	Level    string `json:"level"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// Query 按关键词和级别筛选 JSON 日志，不对记录作请求归属推断。
func (s *LogService) Query(ctx context.Context, input *LogQuery) (*LogResult, error) {
	if input == nil {
		return nil, appconst.NewInvalidParamsError("日志查询参数不能为空")
	}
	keyword, date, level := strings.TrimSpace(input.Keyword), input.Date, strings.ToLower(input.Level)
	if len(keyword) > 256 {
		return nil, appconst.NewInvalidParamsError("查询关键词不能超过256字节")
	}
	if level == "err" {
		level = "error"
	}
	switch level {
	case "", "info", "error", "warn", "debug":
	default:
		return nil, appconst.NewInvalidParamsError("日志级别无效")
	}
	page, size := input.Page, input.PageSize
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 || size < 1 || size > 100 || page > 1000000 {
		return nil, appconst.NewInvalidParamsError("分页参数无效")
	}
	if date != "" {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil || parsed.Format("2006-01-02") != date {
			return nil, appconst.NewInvalidParamsError("日期必须为有效的 YYYY-MM-DD")
		}
	} else {
		files, err := os.ReadDir(s.directory)
		if errors.Is(err, os.ErrNotExist) {
			return nil, appconst.NewNotFoundError("暂无日志文件")
		}
		if err != nil {
			return nil, appconst.NewInternalError(err)
		}
		for _, file := range files {
			match := logFilePattern.FindStringSubmatch(file.Name())
			if !file.IsDir() && len(match) == 2 && match[1] > date {
				date = match[1]
			}
		}
		if date == "" {
			return nil, appconst.NewNotFoundError("暂无日志文件")
		}
	}
	name := "app-" + date + ".log"
	file, err := os.Open(filepath.Join(s.directory, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, appconst.NewNotFoundError("该日期没有日志文件")
	}
	if err != nil {
		return nil, appconst.NewInternalError(err)
	}
	defer file.Close()
	result := &LogResult{Keyword: keyword, Level: level, Page: page, PageSize: size, Date: date, File: name, Items: make([]LogEntry, 0)}
	start := (page - 1) * size
	keyword = strings.ToLower(keyword)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var record map[string]interface{}
		decoder := json.NewDecoder(strings.NewReader(scanner.Text()))
		decoder.UseNumber()
		if err := decoder.Decode(&record); err != nil || record == nil {
			continue
		}
		var trailing interface{}
		if decoder.Decode(&trailing) != io.EOF {
			continue
		}
		recordLevel := strings.ToLower(stringField(record, "level"))
		if level == "error" {
			if recordLevel != "error" && recordLevel != "fatal" && recordLevel != "panic" && recordLevel != "dpanic" {
				continue
			}
		} else if level != "" && recordLevel != level {
			continue
		}
		if keyword != "" {
			decoded, _ := json.Marshal(record)
			if !strings.Contains(strings.ToLower(string(decoded)), keyword) {
				continue
			}
		}
		result.Total++
		if result.Total <= start || len(result.Items) >= size {
			continue
		}
		entry := LogEntry{
			LogID: stringField(record, "log_id"), Line: line, Time: stringField(record, "ts"), Level: stringField(record, "level"),
			Message: stringField(record, "msg"), Fields: make([]LogField, 0),
		}
		keys := make([]string, 0, len(record))
		for key := range record {
			if key != "log_id" && key != "ts" && key != "level" && key != "msg" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			field := LogField{Key: key, Label: logFieldLabel(key), Value: logFieldValue(key, record[key])}
			entry.Fields = append(entry.Fields, field)
		}
		result.Items = append(result.Items, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, appconst.NewInternalError(fmt.Errorf("读取日志失败: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func stringField(record map[string]interface{}, key string) string {
	value, _ := record[key].(string)
	return value
}

func logFieldLabel(key string) string {
	labels := map[string]string{
		"sql": "SQL", "source": "来源", "error": "错误", "rows_affected": "影响行数", "elapsed_ms": "SQL耗时(ms)",
		"method": "请求方法", "path": "请求路径", "status": "HTTP状态", "latency": "请求耗时(ms)", "client_ip": "客户端IP",
		"topic": "Topic", "msg_id": "消息ID", "consumer_group": "消费者组", "client_id": "客户端ID", "result": "消费结果",
		"order_id": "订单ID", "order_no": "订单号", "user_id": "用户ID", "product_id": "商品ID",
	}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

func logFieldValue(key string, value interface{}) string {
	if number, ok := value.(json.Number); ok {
		if key == "latency" || key == "elapsed_ms" {
			if n, err := strconv.ParseFloat(string(number), 64); err == nil {
				if key == "latency" {
					n *= 1000
				}
				return fmt.Sprintf("%.3f", n)
			}
		}
		return string(number)
	}
	if text, ok := value.(string); ok {
		return text
	}
	data, _ := json.Marshal(value)
	return string(data)
}
