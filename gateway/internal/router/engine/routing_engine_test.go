package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/router/strategy"
)

// TestRoutingEngine_SelectProvider 测试路由引擎根据策略选择provider
func TestRoutingEngine_SelectProvider(t *testing.T) {
	engine := NewRoutingEngine()

	// 注册策略
	costBased := strategy.NewCostBasedTemplate("CostBased", strategy.CostParams{
		MaxCostPer1KTokens: 1.0,
	})

	// 注册providers
	costBased.RegisterProvider("ProviderA", &MockProvider{
		name:            "ProviderA",
		costPer1KTokens: 0.5,
		available:       true,
		models:          []string{"gpt-4"},
	})
	costBased.RegisterProvider("ProviderB", &MockProvider{
		name:            "ProviderB",
		costPer1KTokens: 0.3, // 最低成本
		available:       true,
		models:          []string{"gpt-4"},
	})

	engine.RegisterStrategy("cost_based", costBased)

	req := &strategy.RoutingRequest{
		Model:   "gpt-4",
		UserID:  "user123",
		MaxCost: 1.0,
	}

	decision, err := engine.SelectProvider(context.Background(), req, "cost_based")

	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.Equal(t, "ProviderB", decision.Provider, "Should select lowest cost provider")
	assert.True(t, decision.TakeoverMark, "TakeoverMark should be true for M-008")
}

// TestRoutingEngine_DecisionMetrics 测试路由决策记录metrics
func TestRoutingEngine_DecisionMetrics(t *testing.T) {
	engine := NewRoutingEngine()

	// 创建mock metrics collector
	engine.metrics = &MockRoutingMetrics{}

	// 注册策略
	costBased := strategy.NewCostBasedTemplate("CostBased", strategy.CostParams{
		MaxCostPer1KTokens: 1.0,
	})

	costBased.RegisterProvider("ProviderA", &MockProvider{
		name:            "ProviderA",
		costPer1KTokens: 0.5,
		available:       true,
		models:          []string{"gpt-4"},
	})

	engine.RegisterStrategy("cost_based", costBased)

	req := &strategy.RoutingRequest{
		Model:  "gpt-4",
		UserID: "user123",
	}

	decision, err := engine.SelectProvider(context.Background(), req, "cost_based")

	assert.NoError(t, err)
	assert.NotNil(t, decision)

	// 验证metrics被记录
	metrics := engine.metrics.(*MockRoutingMetrics)
	assert.True(t, metrics.recordCalled, "RecordSelection should be called")
	assert.Equal(t, "ProviderA", metrics.lastProvider, "Provider should be recorded")
}

// MockProvider 用于测试的Mock Provider
type MockProvider struct {
	name            string
	costPer1KTokens float64
	qualityScore    float64
	latencyMs       int64
	available       bool
	models          []string
}

func (m *MockProvider) ChatCompletion(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (*adapter.CompletionResponse, error) {
	return nil, nil
}

func (m *MockProvider) ChatCompletionStream(ctx context.Context, model string, messages []adapter.Message, options adapter.CompletionOptions) (<-chan *adapter.StreamChunk, error) {
	return nil, nil
}

func (m *MockProvider) GetUsage(response *adapter.CompletionResponse) adapter.Usage {
	return adapter.Usage{}
}

func (m *MockProvider) MapError(err error) adapter.ProviderError {
	return adapter.ProviderError{}
}

func (m *MockProvider) HealthCheck(ctx context.Context) bool {
	return m.available
}

func (m *MockProvider) ProviderName() string {
	return m.name
}

func (m *MockProvider) SupportedModels() []string {
	return m.models
}

func (m *MockProvider) GetCostPer1KTokens() float64 {
	return m.costPer1KTokens
}

func (m *MockProvider) GetQualityScore() float64 {
	return m.qualityScore
}

func (m *MockProvider) GetLatencyMs() int64 {
	return m.latencyMs
}

// MockRoutingMetrics 用于测试的Mock Metrics
type MockRoutingMetrics struct {
	recordCalled  bool
	lastProvider  string
	lastStrategy  string
	takeoverMark  bool
}

func (m *MockRoutingMetrics) RecordSelection(provider string, strategyName string, decision *strategy.RoutingDecision) {
	m.recordCalled = true
	m.lastProvider = provider
	m.lastStrategy = strategyName
	if decision != nil {
		m.takeoverMark = decision.TakeoverMark
	}
}

// ==================== P0问题测试 ====================

// TestP0_07_RegisterStrategy_ThreadSafety 测试P0-07: 策略注册非线程安全
func TestP0_07_RegisterStrategy_ThreadSafety(t *testing.T) {
	engine := NewRoutingEngine()

	// 并发注册多个策略，启用-race检测器可以发现数据竞争
	done := make(chan bool)
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			name := strategyName(idx)
			tpl := strategy.NewCostBasedTemplate(name, strategy.CostParams{
				MaxCostPer1KTokens: 1.0,
			})
			tpl.RegisterProvider("ProviderA", &MockProvider{
				name:            "ProviderA",
				costPer1KTokens:  0.5,
				available:       true,
				models:          []string{"gpt-4"},
			})
			engine.RegisterStrategy(name, tpl)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// 验证所有策略都已注册
	for i := 0; i < goroutines; i++ {
		name := strategyName(i)
		_, ok := engine.strategies[name]
		assert.True(t, ok, "Strategy %s should be registered", name)
	}
}

func strategyName(idx int) string {
	return "strategy_" + string(rune('a'+idx%26)) + string(rune('0'+idx/26%10))
}

// TestP0_08_DecisionNilPanic 测试P0-08: decision可能为空指针
func TestP0_08_DecisionNilPanic(t *testing.T) {
	engine := NewRoutingEngine()

	// 创建一个返回nil decision但不返回错误的策略
	nilDecisionStrategy := &NilDecisionStrategy{}

	engine.RegisterStrategy("nil_decision", nilDecisionStrategy)

	// 设置metrics
	engine.metrics = &MockRoutingMetrics{}

	req := &strategy.RoutingRequest{
		Model:  "gpt-4",
		UserID: "user123",
	}

	// 验证返回ErrStrategyNotFound而不是panic
	decision, err := engine.SelectProvider(context.Background(), req, "nil_decision")

	assert.Error(t, err, "Should return error when decision is nil")
	assert.Equal(t, ErrStrategyNotFound, err, "Should return ErrStrategyNotFound")
	assert.Nil(t, decision, "Decision should be nil")
}

// NilDecisionStrategy 返回nil decision的测试策略
type NilDecisionStrategy struct{}

func (s *NilDecisionStrategy) SelectProvider(ctx context.Context, req *strategy.RoutingRequest) (*strategy.RoutingDecision, error) {
	// 返回nil decision但不返回错误 - 这模拟了潜在的边界情况
	return nil, nil
}

func (s *NilDecisionStrategy) Name() string {
	return "nil_decision"
}

func (s *NilDecisionStrategy) Type() string {
	return "nil_decision"
}
