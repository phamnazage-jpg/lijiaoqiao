package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// RoutingMetrics 路由指标收集器 (M-008)
type RoutingMetrics struct {
	// 计数器
	totalRequests    int64
	totalTakeovers   int64
	primaryTakeovers int64
	fallbackTakeovers int64
	noMarkCount      int64

	// 按provider统计
	providerStats map[string]*ProviderStat
	providerMu    sync.RWMutex

	// 按策略统计
	strategyStats map[string]*StrategyStat
	strategyMu   sync.RWMutex

	// 时间窗口
	windowStart time.Time
}

// ProviderStat Provider统计
type ProviderStat struct {
	Count      int64
	LatencySum int64
	Errors     int64
}

// StrategyStat 策略统计
type StrategyStat struct {
	Count       int64
	Takeovers   int64
	LatencySum  int64
}

// RoutingStats 路由统计
type RoutingStats struct {
	TotalRequests    int64
	TotalTakeovers   int64
	PrimaryTakeovers int64
	FallbackTakeovers int64
	NoMarkCount      int64
	TakeoverRate     float64
	M008Coverage     float64 // 路由标记覆盖率 >= 99.9%
	ProviderStats    map[string]*ProviderStat
	StrategyStats    map[string]*StrategyStat
}

// NewRoutingMetrics 创建路由指标收集器
func NewRoutingMetrics() *RoutingMetrics {
	return &RoutingMetrics{
		providerStats: make(map[string]*ProviderStat),
		strategyStats: make(map[string]*StrategyStat),
		windowStart:   time.Now(),
	}
}

// RecordTakeoverMark 记录接管标记
// pathType: "primary" 或 "fallback"
// strategy: 使用的策略名称
func (m *RoutingMetrics) RecordTakeoverMark(provider string, tier int, pathType string, strategy string) {
	atomic.AddInt64(&m.totalTakeovers, 1)

	// 更新路径类型计数
	switch pathType {
	case "primary":
		atomic.AddInt64(&m.primaryTakeovers, 1)
	case "fallback":
		atomic.AddInt64(&m.fallbackTakeovers, 1)
	}

	// 更新Provider统计
	m.providerMu.Lock()
	if _, ok := m.providerStats[provider]; !ok {
		m.providerStats[provider] = &ProviderStat{}
	}
	m.providerStats[provider].Count++
	m.providerMu.Unlock()

	// 更新策略统计
	m.strategyMu.Lock()
	if _, ok := m.strategyStats[strategy]; !ok {
		m.strategyStats[strategy] = &StrategyStat{}
	}
	m.strategyStats[strategy].Count++
	m.strategyStats[strategy].Takeovers++
	m.strategyMu.Unlock()
}

// RecordNoMark 记录未标记的请求（用于计算覆盖率）
func (m *RoutingMetrics) RecordNoMark(reason string) {
	atomic.AddInt64(&m.noMarkCount, 1)
}

// RecordRequest 记录请求
func (m *RoutingMetrics) RecordRequest() {
	atomic.AddInt64(&m.totalRequests, 1)
}

// GetStats 获取统计信息
func (m *RoutingMetrics) GetStats() *RoutingStats {
	total := atomic.LoadInt64(&m.totalRequests)
	takeovers := atomic.LoadInt64(&m.totalTakeovers)
	primary := atomic.LoadInt64(&m.primaryTakeovers)
	fallback := atomic.LoadInt64(&m.fallbackTakeovers)
	noMark := atomic.LoadInt64(&m.noMarkCount)

	// 计算接管率 (有标记的请求 / 总请求)
	var takeoverRate float64
	if total > 0 {
		takeoverRate = float64(takeovers) / float64(total) * 100
	}

	// 计算M-008覆盖率 (有标记的请求 / 总请求)
	var coverage float64
	if total > 0 {
		coverage = float64(takeovers) / float64(total) * 100
	}

	// 复制Provider统计
	m.providerMu.RLock()
	providerStats := make(map[string]*ProviderStat)
	for k, v := range m.providerStats {
		providerStats[k] = &ProviderStat{
			Count:      v.Count,
			LatencySum: v.LatencySum,
			Errors:     v.Errors,
		}
	}
	m.providerMu.RUnlock()

	// 复制策略统计
	m.strategyMu.RLock()
	strategyStats := make(map[string]*StrategyStat)
	for k, v := range m.strategyStats {
		strategyStats[k] = &StrategyStat{
			Count:      v.Count,
			Takeovers:  v.Takeovers,
			LatencySum: v.LatencySum,
		}
	}
	m.strategyMu.RUnlock()

	return &RoutingStats{
		TotalRequests:     total,
		TotalTakeovers:    takeovers,
		PrimaryTakeovers:  primary,
		FallbackTakeovers: fallback,
		NoMarkCount:       noMark,
		TakeoverRate:      takeoverRate,
		M008Coverage:      coverage,
		ProviderStats:     providerStats,
		StrategyStats:     strategyStats,
	}
}

// Reset 重置统计
func (m *RoutingMetrics) Reset() {
	atomic.StoreInt64(&m.totalRequests, 0)
	atomic.StoreInt64(&m.totalTakeovers, 0)
	atomic.StoreInt64(&m.primaryTakeovers, 0)
	atomic.StoreInt64(&m.fallbackTakeovers, 0)
	atomic.StoreInt64(&m.noMarkCount, 0)

	m.providerMu.Lock()
	m.providerStats = make(map[string]*ProviderStat)
	m.providerMu.Unlock()

	m.strategyMu.Lock()
	m.strategyStats = make(map[string]*StrategyStat)
	m.strategyMu.Unlock()

	m.windowStart = time.Now()
}
