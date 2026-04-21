package metrics

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// GatewayMetrics gateway 指标收集器
// P3-C: 提供内嵌计数器，支持 Prometheus-text 格式导出，无需额外依赖
type GatewayMetrics struct {
	providerRequests map[string]*providerMetrics
	providerMu       sync.RWMutex
	uptime           time.Time

	tokenRuntimeLatencyNs atomic.Int64
	tokenRuntimeRequests  atomic.Int64
	tokenRuntimeErrors    atomic.Int64
}

type providerMetrics struct {
	requests atomic.Int64
	success  atomic.Int64
	failure  atomic.Int64
	latency  atomic.Int64
}

var gwGlobal *GatewayMetrics

func init() {
	gwGlobal = &GatewayMetrics{
		providerRequests: make(map[string]*providerMetrics),
		uptime:           time.Now(),
	}
}

// RecordProviderResult 记录 provider 调用结果（从 router.RecordResult 同步调用）
func RecordProviderResult(providerName string, success bool, latencyMs int64) {
	m := getProviderMetrics(providerName)
	m.requests.Add(1)
	if success {
		m.success.Add(1)
	} else {
		m.failure.Add(1)
	}
	if latencyMs > 0 {
		m.latency.Add(latencyMs)
	}
}

// RecordTokenRuntime 记录 token-runtime introspection 调用
func RecordTokenRuntime(latencyNs int64, err bool) {
	gwGlobal.tokenRuntimeRequests.Add(1)
	if err {
		gwGlobal.tokenRuntimeErrors.Add(1)
	}
	if latencyNs > 0 {
		gwGlobal.tokenRuntimeLatencyNs.Add(latencyNs)
	}
}

// P3-A-05: Cache metrics
var cacheHits atomic.Int64
var cacheMisses atomic.Int64
var cacheEvictions atomic.Int64

// RecordCacheHit 记录缓存命中
func RecordCacheHit() {
	cacheHits.Add(1)
}

// RecordCacheMiss 记录缓存未命中（需要调用上游）
func RecordCacheMiss() {
	cacheMisses.Add(1)
}

// RecordCacheEviction 记录缓存淘汰
func RecordCacheEviction() {
	cacheEvictions.Add(1)
}

// GetCacheHits 返回缓存命中总数
func GetCacheHits() int64 {
	return cacheHits.Load()
}

// GetCacheMisses 返回缓存未命中总数
func GetCacheMisses() int64 {
	return cacheMisses.Load()
}

// GetCacheEvictions 返回缓存淘汰总数
func GetCacheEvictions() int64 {
	return cacheEvictions.Load()
}

// GetCacheHitRate 返回缓存命中率 (0.0 ~ 1.0)，未命中数为0时返回0
func GetCacheHitRate() float64 {
	hits := cacheHits.Load()
	misses := cacheMisses.Load()
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

func getProviderMetrics(name string) *providerMetrics {
	gwGlobal.providerMu.RLock()
	m, ok := gwGlobal.providerRequests[name]
	gwGlobal.providerMu.RUnlock()
	if ok {
		return m
	}
	gwGlobal.providerMu.Lock()
	defer gwGlobal.providerMu.Unlock()
	if m, ok = gwGlobal.providerRequests[name]; !ok {
		m = &providerMetrics{}
		gwGlobal.providerRequests[name] = m
	}
	return m
}

// Export 返回 Prometheus-text 格式
func Export() string {
	m := gwGlobal
	uptime := time.Since(m.uptime).Seconds()
	avgLatencyNs := float64(0)
	if n := m.tokenRuntimeRequests.Load(); n > 0 {
		avgLatencyNs = float64(m.tokenRuntimeLatencyNs.Load()) / float64(n)
	}

	lines := []string{
		"# HELP gateway_uptime_seconds Time since gateway start",
		"# TYPE gateway_uptime_seconds gauge",
		formatFloat("gateway_uptime_seconds", uptime),
		"# HELP gateway_token_runtime_requests_total Token-runtime introspection requests",
		"# TYPE gateway_token_runtime_requests_total counter",
		formatInt("gateway_token_runtime_requests_total", m.tokenRuntimeRequests.Load()),
		"# HELP gateway_token_runtime_errors_total Token-runtime introspection errors",
		"# TYPE gateway_token_runtime_errors_total counter",
		formatInt("gateway_token_runtime_errors_total", m.tokenRuntimeErrors.Load()),
		"# HELP gateway_token_runtime_latency_ms_avg Average token-runtime introspection latency in ms",
		"# TYPE gateway_token_runtime_latency_ms_avg gauge",
		formatFloat("gateway_token_runtime_latency_ms_avg", avgLatencyNs/1e6),
	}

	m.providerMu.RLock()
	defer m.providerMu.RUnlock()
	for name, pm := range m.providerRequests {
		prefix := "gateway_provider_" + sanitizeLabel(name) + "_"
		total := pm.requests.Load()
		succ := pm.success.Load()
		fail := pm.failure.Load()
		avgLat := float64(0)
		if total > 0 {
			avgLat = float64(pm.latency.Load()) / float64(total)
		}
		lines = append(lines,
			"# HELP "+prefix+"requests_total Total requests to provider",
			"# TYPE "+prefix+"requests_total counter",
			formatInt(prefix+"requests_total", total),
			"# HELP "+prefix+"success_total Successful requests",
			"# TYPE "+prefix+"success_total counter",
			formatInt(prefix+"success_total", succ),
			"# HELP "+prefix+"failure_total Failed requests",
			"# TYPE "+prefix+"failure_total counter",
			formatInt(prefix+"failure_total", fail),
			"# HELP "+prefix+"latency_ms_avg Average request latency in ms",
			"# TYPE "+prefix+"latency_ms_avg gauge",
			formatFloat(prefix+"latency_ms_avg", avgLat),
		)
	}

	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func formatInt(name string, v int64) string { return name + " " + strconv.FormatInt(v, 10) }
func formatFloat(name string, v float64) string {
	return name + " " + strconv.FormatFloat(v, 'f', 3, 64)
}

// Prometheus label 值白名单：[A-Za-z0-9_]
func sanitizeLabel(s string) string {
	var out []byte
	for i := 0; i < len(s) && i < 64; i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
