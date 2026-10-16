package http_msg

type RouteMetricResp struct {
	Route    string  `json:"route"`
	Requests uint64  `json:"requests"`
	Errors   uint64  `json:"errors"`
	AvgMS    float64 `json:"avg_ms"`
}
type MetricsResp struct {
	Routes []RouteMetricResp `json:"routes"`
}
