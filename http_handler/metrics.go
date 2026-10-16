package http_handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-framework-demo/http_handler/http_msg"
	"go-framework-demo/util"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type routeMetric struct {
	Requests atomic.Uint64
	Errors   atomic.Uint64
	Duration atomic.Uint64
	mu       sync.Mutex
	buckets  [6]uint64
	statuses map[int]uint64
}

var durationBounds = [...]time.Duration{10 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 500 * time.Millisecond, time.Second}

type metricsStore struct {
	mu     sync.RWMutex
	routes map[string]*routeMetric
}

var requestMetrics = &metricsStore{routes: make(map[string]*routeMetric)}

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		m := requestMetrics.metric(route)
		m.Requests.Add(1)
		if c.Writer.Status() >= 500 {
			m.Errors.Add(1)
		}
		d := time.Since(start)
		m.Duration.Add(uint64(d.Microseconds()))
		m.mu.Lock()
		m.statuses[c.Writer.Status()]++
		for i, v := range durationBounds {
			if d <= v {
				m.buckets[i]++
			}
		}
		m.buckets[5]++
		m.mu.Unlock()
	}
}
func (s *metricsStore) metric(route string) *routeMetric {
	s.mu.RLock()
	m := s.routes[route]
	s.mu.RUnlock()
	if m != nil {
		return m
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m = s.routes[route]; m == nil {
		m = &routeMetric{statuses: make(map[int]uint64)}
		s.routes[route] = m
	}
	return m
}

type metricView = http_msg.RouteMetricResp

// 查询接口指标摘要。
func metricsHandler(c *gin.Context) {
	v := []metricView{}
	requestMetrics.mu.RLock()
	for r, m := range requestMetrics.routes {
		n := m.Requests.Load()
		avg := 0.0
		if n > 0 {
			avg = float64(m.Duration.Load()) / 1000 / float64(n)
		}
		view := metricView{
			r,
			n,
			m.Errors.Load(),
			avg,
		}
		v = append(v, view)
	}
	requestMetrics.mu.RUnlock()
	sort.Slice(v, func(i, j int) bool { return v[i].Requests > v[j].Requests })
	response := http_msg.MetricsResp{Routes: v}
	writeSuccess(c, http.StatusOK, response)
}

// Prometheus指标。
func prometheusHandler(c *gin.Context) {
	var b strings.Builder
	b.WriteString("# HELP http_requests_total Total HTTP requests.\n# TYPE http_requests_total counter\n# HELP http_request_errors_total Total HTTP 5xx responses.\n# TYPE http_request_errors_total counter\n# HELP http_request_duration_seconds HTTP request duration histogram.\n# TYPE http_request_duration_seconds histogram\n")
	requestMetrics.mu.RLock()
	for r, m := range requestMetrics.routes {
		l := fmt.Sprintf("{route=%q}", r)
		fmt.Fprintf(&b, "http_request_errors_total%s %d\n", l, m.Errors.Load())
		m.mu.Lock()
		for status, count := range m.statuses {
			fmt.Fprintf(&b, "http_requests_total{route=%q,status=%q} %d\n", r, fmt.Sprint(status), count)
		}
		for i, v := range durationBounds {
			fmt.Fprintf(&b, "http_request_duration_seconds_bucket{route=%q,le=%q} %d\n", r, fmt.Sprintf("%g", v.Seconds()), m.buckets[i])
		}
		fmt.Fprintf(&b, "http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", r, m.buckets[5])
		m.mu.Unlock()
		fmt.Fprintf(&b, "http_request_duration_seconds_sum%s %.6f\nhttp_request_duration_seconds_count%s %d\n", l, float64(m.Duration.Load())/1e6, l, m.Requests.Load())
	}
	requestMetrics.mu.RUnlock()
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
}

// runtimeConfigHandler 将应用 YAML 中的 API 地址提供给静态前端页面。
func runtimeConfigHandler(c *gin.Context) {
	config, err := util.LoadAppConfig()
	if err != nil {
		c.String(http.StatusInternalServerError, "配置加载失败")
		return
	}
	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.String(http.StatusOK, "window.DEMO_API_BASE = %q;", config.Frontend.APIBaseURL)
}
