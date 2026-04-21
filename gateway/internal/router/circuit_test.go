package router

import (
	"context"
	"testing"
	"time"

	"lijiaoqiao/gateway/internal/adapter"
)

// P3-B: 熔断器测试矩阵

// circuitTestProvider 实现 adapter.ProviderAdapter（使用真实 adapter 类型）
type circuitTestProvider struct {
	models       []string
	healthResult bool
}

func (p *circuitTestProvider) ChatCompletion(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (*adapter.CompletionResponse, error) {
	return nil, nil
}
func (p *circuitTestProvider) ChatCompletionStream(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (<-chan *adapter.StreamChunk, error) {
	return nil, nil
}
func (p *circuitTestProvider) GetUsage(response *adapter.CompletionResponse) adapter.Usage {
	return adapter.Usage{}
}
func (p *circuitTestProvider) MapError(err error) adapter.ProviderError {
	return adapter.ProviderError{}
}
func (p *circuitTestProvider) HealthCheck(ctx context.Context) bool {
	return p.healthResult
}
func (p *circuitTestProvider) ProviderName() string {
	return "circuit-test"
}
func (p *circuitTestProvider) SupportedModels() []string {
	return p.models
}

func TestCircuitBreaker_ClosedToOpen_FailureRateThreshold(t *testing.T) {
	r := NewRouter(StrategyLatency)
	health := &ProviderHealth{
		Name:                "test",
		Available:           true,
		FailureRate:         0,
		CircuitState:        CircuitClosed,
		ConsecutiveFailures: 0,
	}

	// 模拟 6 次失败，失败率超过 0.5
	for i := 0; i < 6; i++ {
		r.transitionCircuitLocked(health, false)
	}

	if health.CircuitState != CircuitOpen {
		t.Errorf("expected CircuitOpen after failure rate > 0.5, got %v", health.CircuitState)
	}
	if health.Available != false {
		t.Error("expected Available=false when circuit opens")
	}
}

func TestCircuitBreaker_ClosedToOpen_ConsecutiveFailures(t *testing.T) {
	r := NewRouter(StrategyLatency)
	health := &ProviderHealth{
		Name:                "test",
		Available:           true,
		FailureRate:         0,
		CircuitState:        CircuitClosed,
		ConsecutiveFailures: 0,
	}

	// 4次失败，不应触发
	for i := 0; i < 4; i++ {
		r.transitionCircuitLocked(health, false)
	}
	if health.CircuitState != CircuitClosed {
		t.Errorf("expected CircuitClosed after 4 failures (limit=5), got %v", health.CircuitState)
	}

	// 第5次失败，触发熔断
	r.transitionCircuitLocked(health, false)
	if health.CircuitState != CircuitOpen {
		t.Errorf("expected CircuitOpen after 5 consecutive failures, got %v", health.CircuitState)
	}
}

func TestCircuitBreaker_ClosedSuccess_ResetsCounters(t *testing.T) {
	r := NewRouter(StrategyLatency)
	health := &ProviderHealth{
		Name:                "test",
		Available:           true,
		FailureRate:         0,
		CircuitState:        CircuitClosed,
		ConsecutiveFailures: 3,
	}

	r.transitionCircuitLocked(health, true)

	if health.ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures=0 after success, got %d", health.ConsecutiveFailures)
	}
	if health.ConsecutiveSuccesses != 1 {
		t.Errorf("expected ConsecutiveSuccesses=1 after success, got %d", health.ConsecutiveSuccesses)
	}
}

func TestCircuitBreaker_HalfOpenToClosed_SuccessThreshold(t *testing.T) {
	r := NewRouter(StrategyLatency)
	health := &ProviderHealth{
		Name:                 "test",
		Available:            false,
		FailureRate:          0.9,
		CircuitState:         CircuitHalfOpen,
		ConsecutiveFailures:  0,
		ConsecutiveSuccesses: 0,
	}

	// 2次成功，不应关闭
	r.transitionCircuitLocked(health, true)
	r.transitionCircuitLocked(health, true)
	if health.CircuitState != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen after 2 successes, got %v", health.CircuitState)
	}

	// 第3次成功，应切换到 Closed
	r.transitionCircuitLocked(health, true)
	if health.CircuitState != CircuitClosed {
		t.Errorf("expected CircuitClosed after 3 consecutive successes, got %v", health.CircuitState)
	}
	if health.Available != true {
		t.Error("expected Available=true when circuit closes")
	}
}

func TestCircuitBreaker_HalfOpenToOpen_ProbeFailed(t *testing.T) {
	r := NewRouter(StrategyLatency)
	health := &ProviderHealth{
		Name:                 "test",
		Available:            false,
		CircuitState:         CircuitHalfOpen,
		ConsecutiveFailures:  0,
		ConsecutiveSuccesses: 2,
	}

	r.transitionCircuitLocked(health, false)

	if health.CircuitState != CircuitOpen {
		t.Errorf("expected CircuitOpen after half-open probe failure, got %v", health.CircuitState)
	}
	if health.OpenReason != "half_open_probe_failed" {
		t.Errorf("expected OpenReason='half_open_probe_failed', got %s", health.OpenReason)
	}
}

func TestIsProviderAvailable_CircuitOpen_ReturnsFalse(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{models: []string{"gpt-4"}}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		Available:    true, // 但熔断器开着
		CircuitState: CircuitOpen,
	}

	if r.isProviderAvailable("test", "gpt-4") {
		t.Error("expected isProviderAvailable=false when CircuitOpen")
	}
}

func TestIsProviderAvailable_CircuitHalfOpen_ReturnsTrue(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{models: []string{"gpt-4"}}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		Available:    false, // HalfOpen 时 Available 可以是 false
		CircuitState: CircuitHalfOpen,
	}

	// HalfOpen 允许试探请求通过
	if !r.isProviderAvailable("test", "gpt-4") {
		t.Error("expected isProviderAvailable=true when CircuitHalfOpen (probe allowed)")
	}
}

func TestIsProviderAvailable_CircuitClosed_NormalCheck(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{models: []string{"gpt-4"}}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		Available:    false, // 不可用
		CircuitState: CircuitClosed,
	}

	// CircuitClosed 时应该走原有的 Available 检查
	if r.isProviderAvailable("test", "gpt-4") {
		t.Error("expected isProviderAvailable=false when CircuitClosed and Available=false")
	}
}

func TestGetCircuitState(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		CircuitState: CircuitHalfOpen,
	}

	if r.GetCircuitState("test") != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen, got %v", r.GetCircuitState("test"))
	}

	if r.GetCircuitState("unknown") != CircuitClosed {
		t.Error("expected CircuitClosed for unknown provider")
	}
}

func TestSetAndGetCircuitConfig(t *testing.T) {
	r := NewRouter(StrategyLatency)
	cfg := CircuitBreakerConfig{
		FailureRateThreshold:     0.3,
		ConsecutiveFailureLimit:  10,
		HalfOpenSuccessThreshold: 5,
		OpenTimeout:              60 * time.Second,
	}

	r.SetCircuitConfig(cfg)
	got := r.GetCircuitConfig()

	if got.FailureRateThreshold != 0.3 {
		t.Errorf("expected FailureRateThreshold=0.3, got %f", got.FailureRateThreshold)
	}
	if got.OpenTimeout != 60*time.Second {
		t.Errorf("expected OpenTimeout=60s, got %v", got.OpenTimeout)
	}
}

func TestStartStopHealthChecker(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{}
	r.health["test"] = &ProviderHealth{Name: "test"}

	// 启动
	r.StartHealthChecker(100 * time.Millisecond)
	if r.healthChecker == nil {
		t.Error("expected healthChecker to be non-nil after Start")
	}

	// 重复启动不应该 panic 或创建多个
	r.StartHealthChecker(100 * time.Millisecond)

	// 停止
	r.StopHealthChecker()
	if r.healthChecker != nil {
		t.Error("expected healthChecker to be nil after Stop")
	}
}

func TestCheckAndTransitionToHalfOpen(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		CircuitState: CircuitOpen,
	}

	// Provider 不健康，不应转换
	changed := r.CheckAndTransitionToHalfOpen("test", false)
	if changed {
		t.Error("expected no transition when provider unhealthy")
	}

	// Provider 健康，应转换到 HalfOpen
	changed = r.CheckAndTransitionToHalfOpen("test", true)
	if !changed {
		t.Error("expected transition to HalfOpen when provider healthy")
	}
	if r.health["test"].CircuitState != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen, got %v", r.health["test"].CircuitState)
	}
}

func TestCheckAndTransitionToHalfOpen_NotOpenState(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		CircuitState: CircuitClosed, // 非 Open 状态
	}

	changed := r.CheckAndTransitionToHalfOpen("test", true)
	if changed {
		t.Error("expected no transition when not in Open state")
	}
}

func TestRecordResult_Integration(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.circuitConfig = CircuitBreakerConfig{
		FailureRateThreshold:     0.5,
		ConsecutiveFailureLimit:  5,
		HalfOpenSuccessThreshold: 3,
		OpenTimeout:              30 * time.Second,
	}
	r.providers["test"] = &circuitTestProvider{models: []string{"gpt-4"}}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		Available:    true,
		FailureRate:  0,
		CircuitState: CircuitClosed,
	}

	ctx := context.Background()

	// 模拟多次失败触发熔断
	for i := 0; i < 5; i++ {
		r.RecordResult(ctx, "test", false, 100)
	}

	if r.health["test"].CircuitState != CircuitOpen {
		t.Errorf("expected CircuitOpen after 5 consecutive failures via RecordResult, got %v", r.health["test"].CircuitState)
	}

	// CircuitOpen 后 isProviderAvailable 应返回 false
	if r.isProviderAvailable("test", "gpt-4") {
		t.Error("expected isProviderAvailable=false when CircuitOpen")
	}
}

func TestHealthChecker_ChecksProviders(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{models: []string{"gpt-4"}, healthResult: true}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		CircuitState: CircuitOpen, // 从 Open 开始
	}

	hc := NewHealthChecker(r, 50*time.Millisecond, r.circuitConfig)
	hc.Start()

	// 等待健康检查执行
	time.Sleep(150 * time.Millisecond)
	hc.Stop()

	// provider 健康，应该已经转换到 HalfOpen
	if r.health["test"].CircuitState != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen after health check passed, got %v", r.health["test"].CircuitState)
	}
}

func TestHealthChecker_UnhealthyProvider(t *testing.T) {
	r := NewRouter(StrategyLatency)
	r.providers["test"] = &circuitTestProvider{models: []string{"gpt-4"}, healthResult: false}
	r.health["test"] = &ProviderHealth{
		Name:         "test",
		CircuitState: CircuitOpen,
	}

	hc := NewHealthChecker(r, 50*time.Millisecond, r.circuitConfig)
	hc.Start()

	time.Sleep(150 * time.Millisecond)
	hc.Stop()

	// provider 不健康，应该保持在 Open 状态
	if r.health["test"].CircuitState != CircuitOpen {
		t.Errorf("expected CircuitOpen when provider unhealthy, got %v", r.health["test"].CircuitState)
	}
}

func TestCircuitState_String(t *testing.T) {
	states := []CircuitState{CircuitClosed, CircuitOpen, CircuitHalfOpen}
	names := []string{"CircuitClosed", "CircuitOpen", "CircuitHalfOpen"}
	for i, s := range states {
		if s.String() != names[i] {
			t.Errorf("expected %s, got %s", names[i], s.String())
		}
	}
}
