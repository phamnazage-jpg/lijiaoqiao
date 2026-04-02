package fallback

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/gateway/internal/router/strategy"
)

// TestFallback_Tier1_Success 测试Tier1可用时直接返回
func TestFallback_Tier1_Success(t *testing.T) {
	fb := NewFallbackHandler()

	// 设置Tier1 provider
	fb.tiers = []TierConfig{
		{
			Tier: 1,
			Providers: []string{"ProviderA"},
		},
	}

	// 创建mock router
	fb.router = &MockFallbackRouter{
		providers: map[string]*MockFallbackProvider{
			"ProviderA": {
				name:      "ProviderA",
				available: true,
			},
		},
	}

	// 设置metrics
	fb.metrics = &MockFallbackMetrics{}

	req := &strategy.RoutingRequest{
		Model:  "gpt-4",
		UserID: "user123",
	}

	decision, err := fb.Handle(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.Equal(t, "ProviderA", decision.Provider, "Should select Tier1 provider")
	assert.True(t, decision.TakeoverMark, "TakeoverMark should be true")
}

// TestFallback_Tier1_Fail_Tier2 测试Tier1失败时降级到Tier2
func TestFallback_Tier1_Fail_Tier2(t *testing.T) {
	fb := NewFallbackHandler()

	// 设置多级tier
	fb.tiers = []TierConfig{
		{Tier: 1, Providers: []string{"ProviderA"}},
		{Tier: 2, Providers: []string{"ProviderB"}},
	}

	// Tier1不可用，Tier2可用
	fb.router = &MockFallbackRouter{
		providers: map[string]*MockFallbackProvider{
			"ProviderA": {
				name:      "ProviderA",
				available: false, // Tier1 不可用
			},
			"ProviderB": {
				name:      "ProviderB",
				available: true, // Tier2 可用
			},
		},
	}

	fb.metrics = &MockFallbackMetrics{}

	req := &strategy.RoutingRequest{
		Model:  "gpt-4",
		UserID: "user123",
	}

	decision, err := fb.Handle(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.Equal(t, "ProviderB", decision.Provider, "Should fallback to Tier2")
}

// TestFallback_AllFail 测试全部失败返回错误
func TestFallback_AllFail(t *testing.T) {
	fb := NewFallbackHandler()

	fb.tiers = []TierConfig{
		{Tier: 1, Providers: []string{"ProviderA"}},
		{Tier: 2, Providers: []string{"ProviderB"}},
	}

	// 所有provider都不可用
	fb.router = &MockFallbackRouter{
		providers: map[string]*MockFallbackProvider{
			"ProviderA": {name: "ProviderA", available: false},
			"ProviderB": {name: "ProviderB", available: false},
		},
	}

	fb.metrics = &MockFallbackMetrics{}

	req := &strategy.RoutingRequest{
		Model:  "gpt-4",
		UserID: "user123",
	}

	decision, err := fb.Handle(context.Background(), req)

	assert.Error(t, err, "Should return error when all tiers fail")
	assert.Nil(t, decision)
}

// TestFallback_RatelimitIntegration 测试Fallback与ratelimit集成
func TestFallback_RatelimitIntegration(t *testing.T) {
	fb := NewFallbackHandler()

	fb.tiers = []TierConfig{
		{Tier: 1, Providers: []string{"ProviderA"}},
	}

	fb.router = &MockFallbackRouter{
		providers: map[string]*MockFallbackProvider{
			"ProviderA": {
				name:           "ProviderA",
				available:      true,
				rateLimitError: errors.New("rate limit exceeded"), // 触发ratelimit
			},
		},
	}

	fb.metrics = &MockFallbackMetrics{}

	req := &strategy.RoutingRequest{
		Model:  "gpt-4",
		UserID: "user123",
	}

	_, err := fb.Handle(context.Background(), req)

	// 应该检测到ratelimit错误并返回
	assert.Error(t, err, "Should return error on rate limit")
	assert.Contains(t, err.Error(), "rate limit", "Error should mention rate limit")
}

// MockFallbackRouter 用于测试的Mock Router
type MockFallbackRouter struct {
	providers map[string]*MockFallbackProvider
}

func (r *MockFallbackRouter) SelectProvider(ctx context.Context, req *strategy.RoutingRequest, providerName string) (*strategy.RoutingDecision, error) {
	provider, ok := r.providers[providerName]
	if !ok {
		return nil, errors.New("provider not found")
	}

	if !provider.available {
		return nil, errors.New("provider not available")
	}

	if provider.rateLimitError != nil {
		return nil, provider.rateLimitError
	}

	return &strategy.RoutingDecision{
		Provider:     providerName,
		TakeoverMark: true,
	}, nil
}

// MockFallbackProvider 用于测试的Mock Provider
type MockFallbackProvider struct {
	name           string
	available      bool
	rateLimitError error
}

// MockFallbackMetrics 用于测试的Mock Metrics
type MockFallbackMetrics struct {
	recordCalled bool
	tier         int
}

func (m *MockFallbackMetrics) RecordTakeoverMark(provider string, tier int) {
	m.recordCalled = true
	m.tier = tier
}
