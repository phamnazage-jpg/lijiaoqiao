package metrics

import (
	"strconv"
	"sync/atomic"
	"time"
)

// SupplyAPIMetrics supply-api 指标收集器
// P3-C: 统一可观测面，对齐 gateway/platform-token-runtime metrics 风格
type SupplyAPIMetrics struct {
	// HTTP 请求计数
	httpRequests      atomic.Int64
	httpRequestsOK    atomic.Int64
	httpRequestsError atomic.Int64

	// HTTP 延迟（纳秒）
	httpLatencySum   atomic.Int64
	httpLatencyCount atomic.Int64

	// Token 发布计数
	tokenPublishes   atomic.Int64
	tokenPublishFail atomic.Int64

	// Worker queue 指标
	queueSize   atomic.Int64
	workersBusy atomic.Int64

	startAt time.Time
}

var global *SupplyAPIMetrics

func init() {
	global = &SupplyAPIMetrics{startAt: time.Now()}
}

// IncHTTPRequest 记录一次 HTTP 请求
func IncHTTPRequest() { global.httpRequests.Add(1) }

// IncHTTPOK 记录一次成功请求
func IncHTTPOK() { global.httpRequestsOK.Add(1) }

// IncHTTPError 记录一次错误请求
func IncHTTPError() { global.httpRequestsError.Add(1) }

// IncLatency 记录延迟（纳秒）
func IncLatency(ns int64) {
	global.httpLatencySum.Add(ns)
	global.httpLatencyCount.Add(1)
}

// IncTokenPublish 记录一次 token 发布
func IncTokenPublish() { global.tokenPublishes.Add(1) }

// IncTokenPublishFail 记录一次 token 发布失败
func IncTokenPublishFail() { global.tokenPublishes.Add(1); global.tokenPublishFail.Add(1) }

// SetQueueSize 设置当前队列大小
func SetQueueSize(n int64) { global.queueSize.Store(n) }

// SetWorkersBusy 设置忙碌的 worker 数量
func SetWorkersBusy(n int64) { global.workersBusy.Store(n) }

// Export 返回 Prometheus-text 格式指标快照
func Export() string {
	m := global
	uptime := time.Since(m.startAt).Seconds()

	latencyAvg := float64(0)
	if count := m.httpLatencyCount.Load(); count > 0 {
		latencyAvg = float64(m.httpLatencySum.Load()) / float64(count)
	}
	latencyMs := latencyAvg / 1e6

	return `# HELP supply_api_uptime_seconds Time since service start
# TYPE supply_api_uptime_seconds gauge
supply_api_uptime_seconds ` + strconv.FormatFloat(uptime, 'f', 3, 64) + `
# HELP supply_api_http_requests_total Total HTTP requests received
# TYPE supply_api_http_requests_total counter
supply_api_http_requests_total ` + strconv.FormatInt(m.httpRequests.Load(), 10) + `
# HELP supply_api_http_requests_ok_total Successful HTTP requests (2xx/3xx)
# TYPE supply_api_http_requests_ok_total counter
supply_api_http_requests_ok_total ` + strconv.FormatInt(m.httpRequestsOK.Load(), 10) + `
# HELP supply_api_http_requests_error_total Failed HTTP requests (4xx/5xx)
# TYPE supply_api_http_requests_error_total counter
supply_api_http_requests_error_total ` + strconv.FormatInt(m.httpRequestsError.Load(), 10) + `
# HELP supply_api_http_latency_ms_avg Average HTTP request latency in milliseconds
# TYPE supply_api_http_latency_ms_avg gauge
supply_api_http_latency_ms_avg ` + strconv.FormatFloat(latencyMs, 'f', 3, 64) + `
# HELP supply_api_token_publishes_total Total token publish operations
# TYPE supply_api_token_publishes_total counter
supply_api_token_publishes_total ` + strconv.FormatInt(m.tokenPublishes.Load(), 10) + `
# HELP supply_api_token_publish_fail_total Token publish failures
# TYPE supply_api_token_publish_fail_total counter
supply_api_token_publish_fail_total ` + strconv.FormatInt(m.tokenPublishFail.Load(), 10) + `
# HELP supply_api_queue_size Current worker queue size
# TYPE supply_api_queue_size gauge
supply_api_queue_size ` + strconv.FormatInt(m.queueSize.Load(), 10) + `
# HELP supply_api_workers_busy Number of busy workers
# TYPE supply_api_workers_busy gauge
supply_api_workers_busy ` + strconv.FormatInt(m.workersBusy.Load(), 10) + `
`
}
