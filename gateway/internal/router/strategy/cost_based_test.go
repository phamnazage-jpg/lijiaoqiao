package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/gateway/internal/adapter"
)

// TestCostBasedStrategy_SelectProvider 测试成本优先策略选择Provider
func TestCostBasedStrategy_SelectProvider(t *testing.T) {
	template := &CostBasedTemplate{
		name:               "CostBased",
		maxCostPer1KTokens: 1.0,
		providers:          make(map[string]adapter.ProviderAdapter),
	}

	// 注册mock providers
	template.providers["ProviderA"] = &MockProvider{
		name:            "ProviderA",
		costPer1KTokens: 0.5,
		available:       true,
		models:          []string{"gpt-4"},
	}
	template.providers["ProviderB"] = &MockProvider{
		name:            "ProviderB",
		costPer1KTokens: 0.3, // 最低成本
		available:       true,
		models:          []string{"gpt-4"},
	}
	template.providers["ProviderC"] = &MockProvider{
		name:            "ProviderC",
		costPer1KTokens: 0.8,
		available:       true,
		models:          []string{"gpt-4"},
	}

	req := &RoutingRequest{
		Model:   "gpt-4",
		UserID:  "user123",
		MaxCost: 1.0,
	}

	decision, err := template.SelectProvider(context.Background(), req)

	// 验证选择了最低成本的Provider
	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.Equal(t, "ProviderB", decision.Provider, "Should select lowest cost provider")
	assert.LessOrEqual(t, decision.CostPer1KTokens, 1.0, "Cost should be within budget")
}

func TestCostBasedStrategy_Fallback(t *testing.T) {
	// 成本超出阈值时fallback
	template := &CostBasedTemplate{
		name:               "CostBased",
		maxCostPer1KTokens: 0.5, // 设置低成本上限
		providers:          make(map[string]adapter.ProviderAdapter),
	}

	// 注册成本较高的providers
	template.providers["ProviderA"] = &MockProvider{
		name:            "ProviderA",
		costPer1KTokens: 0.8,
		available:       true,
		models:          []string{"gpt-4"},
	}
	template.providers["ProviderB"] = &MockProvider{
		name:            "ProviderB",
		costPer1KTokens: 1.0,
		available:       true,
		models:          []string{"gpt-4"},
	}

	req := &RoutingRequest{
		Model:   "gpt-4",
		UserID:  "user123",
		MaxCost: 0.5,
	}

	decision, err := template.SelectProvider(context.Background(), req)

	// 应该返回错误
	assert.Error(t, err, "Should return error when no affordable provider")
	assert.Nil(t, decision, "Should not return decision when cost exceeds threshold")
	assert.Equal(t, ErrNoAffordableProvider, err, "Should return ErrNoAffordableProvider")
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

// Verify MockProvider implements adapter.ProviderAdapter
var _ adapter.ProviderAdapter = (*MockProvider)(nil)
