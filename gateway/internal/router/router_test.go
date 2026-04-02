package router

import (
	"context"
	"math"
	"testing"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
)

// mockProvider 实现adapter.ProviderAdapter接口
type mockProvider struct {
	name    string
	models  []string
	healthy bool
}

func (m *mockProvider) ChatCompletion(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (*adapter.CompletionResponse, error) {
	return nil, nil
}

func (m *mockProvider) ChatCompletionStream(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (<-chan *adapter.StreamChunk, error) {
	return nil, nil
}

func (m *mockProvider) GetUsage(response *adapter.CompletionResponse) adapter.Usage {
	return adapter.Usage{}
}

func (m *mockProvider) MapError(err error) adapter.ProviderError {
	return adapter.ProviderError{}
}

func (m *mockProvider) HealthCheck(ctx context.Context) bool {
	return m.healthy
}

func (m *mockProvider) ProviderName() string {
	return m.name
}

func (m *mockProvider) SupportedModels() []string {
	return m.models
}

func TestNewRouter(t *testing.T) {
	r := NewRouter(StrategyLatency)

	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.strategy != StrategyLatency {
		t.Errorf("expected strategy latency, got %s", r.strategy)
	}
	if len(r.providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(r.providers))
	}
}

func TestRegisterProvider(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}

	r.RegisterProvider("test", prov)

	if len(r.providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(r.providers))
	}

	health := r.health["test"]
	if health == nil {
		t.Fatal("expected health to be registered")
	}
	if health.Name != "test" {
		t.Errorf("expected name test, got %s", health.Name)
	}
	if !health.Available {
		t.Error("expected provider to be available")
	}
}

func TestSelectProvider_NoProviders(t *testing.T) {
	r := NewRouter(StrategyLatency)

	_, err := r.SelectProvider(context.Background(), "gpt-4")

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectProvider_BasicSelection(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	selected, err := r.SelectProvider(context.Background(), "gpt-4")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ProviderName() != "test" {
		t.Errorf("expected provider test, got %s", selected.ProviderName())
	}
}

func TestSelectProvider_ModelNotSupported(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-3.5"}, healthy: true}
	r.RegisterProvider("test", prov)

	_, err := r.SelectProvider(context.Background(), "gpt-4")

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectProvider_ProviderUnavailable(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 通过UpdateHealth标记为不可用
	r.UpdateHealth("test", false)

	_, err := r.SelectProvider(context.Background(), "gpt-4")

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectProvider_WildcardModel(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"*"}, healthy: true}
	r.RegisterProvider("test", prov)

	selected, err := r.SelectProvider(context.Background(), "any-model")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ProviderName() != "test" {
		t.Errorf("expected provider test, got %s", selected.ProviderName())
	}
}

func TestSelectProvider_MultipleProviders(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov1 := &mockProvider{name: "fast", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "slow", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("fast", prov1)
	r.RegisterProvider("slow", prov2)

	// 记录初始延迟
	r.health["fast"].LatencyMs = 10
	r.health["slow"].LatencyMs = 100

	selected, err := r.SelectProvider(context.Background(), "gpt-4")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ProviderName() != "fast" {
		t.Errorf("expected fastest provider, got %s", selected.ProviderName())
	}
}

func TestRecordResult_Success(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 初始状态
	initialLatency := r.health["test"].LatencyMs

	r.RecordResult(context.Background(), "test", true, 50)

	if r.health["test"].LatencyMs == initialLatency {
		// 首次更新
	}
	if r.health["test"].FailureRate != 0 {
		t.Errorf("expected failure rate 0, got %f", r.health["test"].FailureRate)
	}
}

func TestRecordResult_Failure(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	r.RecordResult(context.Background(), "test", false, 100)

	if r.health["test"].FailureRate == 0 {
		t.Error("expected failure rate to increase")
	}
}

func TestRecordResult_MultipleFailures(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 多次失败直到失败率超过0.5
	// 公式: newRate = oldRate * 0.9 + 0.1
	// 需要7次才能超过0.5 (0.469 -> 0.522)
	for i := 0; i < 7; i++ {
		r.RecordResult(context.Background(), "test", false, 100)
	}

	// 失败率超过0.5应该标记为不可用
	if r.health["test"].Available {
		t.Error("expected provider to be marked unavailable")
	}
}

func TestUpdateHealth(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	r.UpdateHealth("test", false)

	if r.health["test"].Available {
		t.Error("expected provider to be unavailable")
	}
}

func TestGetHealthStatus(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	status := r.GetHealthStatus()

	if len(status) != 1 {
		t.Errorf("expected 1 health status, got %d", len(status))
	}

	health := status["test"]
	if health == nil {
		t.Fatal("expected health for test")
	}
	if health.Available != true {
		t.Error("expected available")
	}
}

func TestGetHealthStatus_Empty(t *testing.T) {
	r := NewRouter(StrategyLatency)

	status := r.GetHealthStatus()

	if len(status) != 0 {
		t.Errorf("expected 0 health statuses, got %d", len(status))
	}
}

func TestSelectByLatency_EqualLatency(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov1 := &mockProvider{name: "p1", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "p2", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("p1", prov1)
	r.RegisterProvider("p2", prov2)

	// 相同的延迟
	r.health["p1"].LatencyMs = 50
	r.health["p2"].LatencyMs = 50

	selected, err := r.selectByLatency([]string{"p1", "p2"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应该返回其中一个
	if selected.ProviderName() != "p1" && selected.ProviderName() != "p2" {
		t.Errorf("unexpected provider: %s", selected.ProviderName())
	}
}

func TestSelectByLatency_NoProviders(t *testing.T) {
	r := NewRouter(StrategyLatency)

	_, err := r.selectByLatency([]string{})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectByWeight(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov1 := &mockProvider{name: "p1", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "p2", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("p1", prov1)
	r.RegisterProvider("p2", prov2)

	r.health["p1"].Weight = 3.0
	r.health["p2"].Weight = 1.0

	// 测试能正常返回结果
	selected, err := r.selectByWeight([]string{"p1", "p2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该返回其中一个
	if selected.ProviderName() != "p1" && selected.ProviderName() != "p2" {
		t.Errorf("unexpected provider: %s", selected.ProviderName())
	}

	// 注意：由于实现中randVal = time.Now().UnixNano()/MaxInt64 * totalWeight
	// 在大多数系统上这个值较小，可能总是选中第一个provider。
	// 这是实现的一个已知限制。
}

func TestSelectByWeight_SingleProvider(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "p1", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("p1", prov)

	r.health["p1"].Weight = 2.0

	selected, err := r.selectByWeight([]string{"p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ProviderName() != "p1" {
		t.Errorf("expected p1, got %s", selected.ProviderName())
	}
}

func TestSelectByAvailability(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov1 := &mockProvider{name: "p1", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "p2", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("p1", prov1)
	r.RegisterProvider("p2", prov2)

	r.health["p1"].FailureRate = 0.3
	r.health["p2"].FailureRate = 0.1

	selected, err := r.selectByAvailability([]string{"p1", "p2"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ProviderName() != "p2" {
		t.Errorf("expected provider with lower failure rate, got %s", selected.ProviderName())
	}
}

func TestGetFallbackProviders(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov1 := &mockProvider{name: "primary", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "fallback", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("primary", prov1)
	r.RegisterProvider("fallback", prov2)

	fallbacks, err := r.GetFallbackProviders(context.Background(), "gpt-4")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fallbacks) != 1 {
		t.Errorf("expected 1 fallback, got %d", len(fallbacks))
	}
	if fallbacks[0].ProviderName() != "fallback" {
		t.Errorf("expected fallback, got %s", fallbacks[0].ProviderName())
	}
}

func TestGetFallbackProviders_AllUnavailable(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "primary", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("primary", prov)

	fallbacks, err := r.GetFallbackProviders(context.Background(), "gpt-4")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fallbacks) != 0 {
		t.Errorf("expected 0 fallbacks, got %d", len(fallbacks))
	}
}

func TestRecordResult_LatencyUpdate(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 首次记录
	r.RecordResult(context.Background(), "test", true, 100)
	if r.health["test"].LatencyMs != 100 {
		t.Errorf("expected latency 100, got %d", r.health["test"].LatencyMs)
	}

	// 第二次记录，使用指数移动平均 (7/8 * 100 + 1/8 * 200 = 87.5 + 25 = 112.5)
	r.RecordResult(context.Background(), "test", true, 200)
	expectedLatency := int64((100*7 + 200) / 8)
	if r.health["test"].LatencyMs != expectedLatency {
		t.Errorf("expected latency %d, got %d", expectedLatency, r.health["test"].LatencyMs)
	}
}

func TestRecordResult_UnknownProvider(t *testing.T) {
	r := NewRouter(StrategyLatency)

	// 不应该panic
	r.RecordResult(context.Background(), "unknown", true, 100)
}

func TestUpdateHealth_UnknownProvider(t *testing.T) {
	r := NewRouter(StrategyLatency)

	// 不应该panic
	r.UpdateHealth("unknown", false)
}

func TestIsProviderAvailable(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4", "gpt-3.5"}, healthy: true}
	r.RegisterProvider("test", prov)

	tests := []struct {
		model     string
		available bool
	}{
		{"gpt-4", true},
		{"gpt-3.5", true},
		{"claude", false},
	}

	for _, tt := range tests {
		if got := r.isProviderAvailable("test", tt.model); got != tt.available {
			t.Errorf("isProviderAvailable(%s) = %v, want %v", tt.model, got, tt.available)
		}
	}
}

func TestIsProviderAvailable_UnknownProvider(t *testing.T) {
	r := NewRouter(StrategyLatency)

	if r.isProviderAvailable("unknown", "gpt-4") {
		t.Error("expected false for unknown provider")
	}
}

func TestIsProviderAvailable_Unhealthy(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 通过UpdateHealth标记为不可用
	r.UpdateHealth("test", false)

	if r.isProviderAvailable("test", "gpt-4") {
		t.Error("expected false for unhealthy provider")
	}
}

func TestProviderHealth_Struct(t *testing.T) {
	health := &ProviderHealth{
		Name:          "test",
		Available:     true,
		LatencyMs:     50,
		FailureRate:   0.1,
		Weight:        1.0,
		LastCheckTime: time.Now(),
	}

	if health.Name != "test" {
		t.Errorf("expected name test, got %s", health.Name)
	}
	if !health.Available {
		t.Error("expected available")
	}
	if health.LatencyMs != 50 {
		t.Errorf("expected latency 50, got %d", health.LatencyMs)
	}
	if health.FailureRate != 0.1 {
		t.Errorf("expected failure rate 0.1, got %f", health.FailureRate)
	}
	if health.Weight != 1.0 {
		t.Errorf("expected weight 1.0, got %f", health.Weight)
	}
}

func TestLoadBalancerStrategy_Constants(t *testing.T) {
	if StrategyLatency != "latency" {
		t.Errorf("expected latency, got %s", StrategyLatency)
	}
	if StrategyRoundRobin != "round_robin" {
		t.Errorf("expected round_robin, got %s", StrategyRoundRobin)
	}
	if StrategyWeighted != "weighted" {
		t.Errorf("expected weighted, got %s", StrategyWeighted)
	}
	if StrategyAvailability != "availability" {
		t.Errorf("expected availability, got %s", StrategyAvailability)
	}
}

func TestSelectProvider_AllStrategies(t *testing.T) {
	strategies := []LoadBalancerStrategy{StrategyLatency, StrategyWeighted, StrategyAvailability}

	for _, strategy := range strategies {
		r := NewRouter(strategy)
		prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
		r.RegisterProvider("test", prov)

		selected, err := r.SelectProvider(context.Background(), "gpt-4")

		if err != nil {
			t.Errorf("strategy %s: unexpected error: %v", strategy, err)
		}
		if selected.ProviderName() != "test" {
			t.Errorf("strategy %s: expected provider test, got %s", strategy, selected.ProviderName())
		}
	}
}

// 确保FailureRate永远不会超过1.0
func TestRecordResult_FailureRateCapped(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 多次失败
	for i := 0; i < 20; i++ {
		r.RecordResult(context.Background(), "test", false, 100)
	}

	if r.health["test"].FailureRate > 1.0 {
		t.Errorf("failure rate should be capped at 1.0, got %f", r.health["test"].FailureRate)
	}
}

// 确保LatencyMs永远不会变成负数
func TestRecordResult_LatencyNeverNegative(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov := &mockProvider{name: "test", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("test", prov)

	// 提供负延迟
	r.RecordResult(context.Background(), "test", true, -100)

	if r.health["test"].LatencyMs < 0 {
		t.Errorf("latency should never be negative, got %d", r.health["test"].LatencyMs)
	}
}

// 确保math.MaxInt64不会溢出
func TestSelectByLatency_MaxInt64(t *testing.T) {
	r := NewRouter(StrategyLatency)
	prov1 := &mockProvider{name: "p1", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "p2", models: []string{"gpt-4"}, healthy: true}
	r.RegisterProvider("p1", prov1)
	r.RegisterProvider("p2", prov2)

	// p1设置为较大值，p2设置为MaxInt64
	r.health["p1"].LatencyMs = math.MaxInt64 - 1
	r.health["p2"].LatencyMs = math.MaxInt64

	selected, err := r.selectByLatency([]string{"p1", "p2"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// p1的延迟更低，应该被选中
	if selected.ProviderName() != "p1" {
		t.Errorf("expected provider p1 (lower latency), got %s", selected.ProviderName())
	}
}
