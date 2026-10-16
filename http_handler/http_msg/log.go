package http_msg

type LogFieldResp struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}
type LogEntryResp struct {
	LogID   string         `json:"log_id"`
	Line    int            `json:"line"`
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  []LogFieldResp `json:"fields"`
}
type LogQueryResp struct {
	Keyword  string         `json:"keyword"`
	Level    string         `json:"level"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Date     string         `json:"date"`
	File     string         `json:"file"`
	Total    int            `json:"total"`
	Items    []LogEntryResp `json:"items"`
}

// LogQuery 是日志关键词、日期、级别及分页条件。
type LogQueryReq struct {
	Keyword  string `json:"keyword"`
	Date     string `json:"date"`
	Level    string `json:"level"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}
