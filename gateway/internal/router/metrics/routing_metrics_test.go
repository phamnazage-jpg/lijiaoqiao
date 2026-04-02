package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRoutingMetrics_M008_TakeoverMarkCoverage 测试M-008指标采集的完整覆盖
func TestRoutingMetrics_M008_TakeoverMarkCoverage(t *testing.T) {
	metrics := NewRoutingMetrics()

	// 模拟主路径调用
	metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")

	// 模拟Fallback路径调用
	metrics.RecordTakeoverMark("ProviderB", 2, "fallback", "cost_based")

	// 验证主路径和Fallback路径都记录了TakeoverMark
	stats := metrics.GetStats()

	// 验证总接管次数
	assert.Equal(t, int64(2), stats.TotalTakeovers, "Should have 2 takeovers")

	// 验证主路径和Fallback路径分开统计
	assert.Equal(t, int64(1), stats.PrimaryTakeovers, "Should have 1 primary takeover")
	assert.Equal(t, int64(1), stats.FallbackTakeovers, "Should have 1 fallback takeover")
}

// TestRoutingMetrics_PrimaryPath 测试主路径M-008采集
func TestRoutingMetrics_PrimaryPath(t *testing.T) {
	metrics := NewRoutingMetrics()

	metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")

	stats := metrics.GetStats()
	assert.Equal(t, int64(1), stats.PrimaryTakeovers)
	assert.Equal(t, int64(1), stats.TotalTakeovers)
}

// TestRoutingMetrics_FallbackPath 测试Fallback路径M-008采集
func TestRoutingMetrics_FallbackPath(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Tier1失败，Tier2成功
	metrics.RecordTakeoverMark("ProviderA", 1, "fallback", "cost_based")
	metrics.RecordTakeoverMark("ProviderB", 2, "fallback", "cost_based")

	stats := metrics.GetStats()
	assert.Equal(t, int64(2), stats.FallbackTakeovers)
	assert.Equal(t, int64(2), stats.TotalTakeovers)
}

// TestRoutingMetrics_TakeoverRate 测试接管率计算
func TestRoutingMetrics_TakeoverRate(t *testing.T) {
	metrics := NewRoutingMetrics()

	// 模拟100次请求，60次主路径接管，40次无接管
	for i := 0; i < 100; i++ {
		metrics.RecordRequest()
	}
	// 60次接管
	for i := 0; i < 60; i++ {
		metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")
	}
	// 40次无接管 - 记录noMark
	for i := 0; i < 40; i++ {
		metrics.RecordNoMark("no provider available")
	}

	stats := metrics.GetStats()

	// 验证接管率 60/(60+40) = 60%
	expectedRate := 60.0 / 100.0 * 100 // 60%
	assert.InDelta(t, expectedRate, stats.TakeoverRate, 0.1, "Takeover rate should be around 60%%")
}

// TestRoutingMetrics_M008Coverage 测试M-008覆盖率
func TestRoutingMetrics_M008Coverage(t *testing.T) {
	metrics := NewRoutingMetrics()

	// 模拟所有请求都标记了TakeoverMark
	for i := 0; i < 1000; i++ {
		metrics.RecordRequest()
	}
	for i := 0; i < 1000; i++ {
		metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")
	}

	stats := metrics.GetStats()

	// M-008要求覆盖率 >= 99.9%
	assert.GreaterOrEqual(t, stats.M008Coverage, 99.9, "M-008 coverage should be >= 99.9%%")
}

// TestRoutingMetrics_Concurrent 测试并发安全
func TestRoutingMetrics_Concurrent(t *testing.T) {
	metrics := NewRoutingMetrics()

	// 并发记录
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 100; i++ {
		<-done
	}

	stats := metrics.GetStats()
	assert.Equal(t, int64(100), stats.TotalTakeovers, "Should handle concurrent recordings")
}

// TestRoutingMetrics_RouteMarkCoverage 测试路由标记覆盖率
func TestRoutingMetrics_RouteMarkCoverage(t *testing.T) {
	metrics := NewRoutingMetrics()

	// 模拟所有请求都有标记
	for i := 0; i < 1000; i++ {
		metrics.RecordRequest()
		metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")
	}

	// 没有未标记的请求
	metrics.RecordNoMark("reason")

	stats := metrics.GetStats()

	// 覆盖率应该很高
	assert.GreaterOrEqual(t, stats.M008Coverage, 99.9, "Coverage should be >= 99.9%%")
}

// TestRoutingMetrics_ProviderStats 测试按provider统计
func TestRoutingMetrics_ProviderStats(t *testing.T) {
	metrics := NewRoutingMetrics()

	metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")
	metrics.RecordTakeoverMark("ProviderA", 1, "primary", "cost_based")
	metrics.RecordTakeoverMark("ProviderB", 1, "primary", "cost_aware")

	stats := metrics.GetStats()

	// 验证按provider统计
	providerA, ok := stats.ProviderStats["ProviderA"]
	assert.True(t, ok, "ProviderA should be in stats")
	assert.Equal(t, int64(2), providerA.Count, "ProviderA should have 2 takeovers")

	providerB, ok := stats.ProviderStats["ProviderB"]
	assert.True(t, ok, "ProviderB should be in stats")
	assert.Equal(t, int64(1), providerB.Count, "ProviderB should have 1 takeover")
}
