package metrics

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// TokenMetrics token 操作指标收集器
// P3-B: 替代 Prometheus client_golang，提供内嵌计数器避免引入额外依赖
type TokenMetrics struct {
	// 操作计数器（按类型）
	issues      atomic.Int64
	revokes     atomic.Int64
	introspects atomic.Int64
	refreshes   atomic.Int64

	// 活跃 token 数（估算）
	activeTokens atomic.Int64

	// 延迟（纳秒，用于 histogram 近似）
	latencySum   atomic.Int64
	latencyCount atomic.Int64

	// 错误计数
	errors atomic.Int64

	// 错误分类
	errInvalidToken atomic.Int64
	errRevoked      atomic.Int64
	errExpired      atomic.Int64
	errInternal     atomic.Int64

	mu      sync.RWMutex
	startAt time.Time
}

var globalMetrics *TokenMetrics

func init() {
	globalMetrics = &TokenMetrics{startAt: time.Now()}
}

// IncIssue 记录一次 token 发放
func IncIssue() { globalMetrics.issues.Add(1); globalMetrics.activeTokens.Add(1) }

// IncRevoke 记录一次 token 吊销
func IncRevoke() { globalMetrics.revokes.Add(1); globalMetrics.activeTokens.Add(-1) }

// IncIntrospect 记录一次 introspection 调用
func IncIntrospect() { globalMetrics.introspects.Add(1) }

// IncRefresh 记录一次 token 刷新
func IncRefresh() { globalMetrics.refreshes.Add(1) }

// IncLatency 记录一次操作延迟（纳秒）
func IncLatency(ns int64) {
	globalMetrics.latencySum.Add(ns)
	globalMetrics.latencyCount.Add(1)
}

// IncError 记录一次错误，可指定类型
func IncError(typ ErrorType) {
	globalMetrics.errors.Add(1)
	switch typ {
	case ErrInvalidToken:
		globalMetrics.errInvalidToken.Add(1)
	case ErrRevoked:
		globalMetrics.errRevoked.Add(1)
	case ErrExpired:
		globalMetrics.errExpired.Add(1)
	case ErrInternal:
		globalMetrics.errInternal.Add(1)
	}
}

// ErrorType 错误分类
type ErrorType int

const (
	ErrInvalidToken ErrorType = iota
	ErrRevoked
	ErrExpired
	ErrInternal
)

// Export 返回 Prometheus-text 格式的指标快照
func Export() string {
	m := globalMetrics
	uptime := time.Since(m.startAt).Seconds()

	latencyAvg := float64(0)
	if count := m.latencyCount.Load(); count > 0 {
		latencyAvg = float64(m.latencySum.Load()) / float64(count)
	}

	latencyMs := latencyAvg / 1e6

	return `# HELP platform_token_runtime_uptime_seconds Time since service start
# TYPE platform_token_runtime_uptime_seconds gauge
platform_token_runtime_uptime_seconds ` + strconv.FormatFloat(uptime, 'f', 3, 64) + `
# HELP platform_token_issues_total Total number of tokens issued
# TYPE platform_token_issues_total counter
platform_token_issues_total ` + strconv.FormatInt(m.issues.Load(), 10) + `
# HELP platform_token_revokes_total Total number of tokens revoked
# TYPE platform_token_revokes_total counter
platform_token_revokes_total ` + strconv.FormatInt(m.revokes.Load(), 10) + `
# HELP platform_token_introspects_total Total number of introspection calls
# TYPE platform_token_introspects_total counter
platform_token_introspects_total ` + strconv.FormatInt(m.introspects.Load(), 10) + `
# HELP platform_token_refreshes_total Total number of token refreshes
# TYPE platform_token_refreshes_total counter
platform_token_refreshes_total ` + strconv.FormatInt(m.refreshes.Load(), 10) + `
# HELP platform_token_active_gauge Estimated number of active tokens
# TYPE platform_token_active_gauge gauge
platform_token_active_gauge ` + strconv.FormatInt(m.activeTokens.Load(), 10) + `
# HELP platform_token_errors_total Total number of errors
# TYPE platform_token_errors_total counter
platform_token_errors_total ` + strconv.FormatInt(m.errors.Load(), 10) + `
# HELP platform_token_errors_by_type Errors by classification
# TYPE platform_token_errors_by_type counter
platform_token_errors_by_type{type="invalid_token"} ` + strconv.FormatInt(m.errInvalidToken.Load(), 10) + `
platform_token_errors_by_type{type="revoked"} ` + strconv.FormatInt(m.errRevoked.Load(), 10) + `
platform_token_errors_by_type{type="expired"} ` + strconv.FormatInt(m.errExpired.Load(), 10) + `
platform_token_errors_by_type{type="internal"} ` + strconv.FormatInt(m.errInternal.Load(), 10) + `
# HELP platform_token_introspect_latency_ms_avg Average introspection latency in milliseconds
# TYPE platform_token_introspect_latency_ms_avg gauge
platform_token_introspect_latency_ms_avg ` + strconv.FormatFloat(latencyMs, 'f', 3, 64) + `
`
}
