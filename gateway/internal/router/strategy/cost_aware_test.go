package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCostAwareStrategy_Balance 测试成本感知策略的平衡选择
func TestCostAwareStrategy_Balance(t *testing.T) {
	template := NewCostAwareTemplate("CostAware", CostAwareParams{
		MaxCostPer1KTokens: 1.0,
		MaxLatencyMs:      500,
		MinQualityScore:    0.7,
	})

	// 注册多个providers
	// ProviderA: 低成本, 低质量
	template.providers["ProviderA"] = &MockProvider{
		name:            "ProviderA",
		costPer1KTokens: 0.2,
		available:       true,
		models:          []string{"gpt-4"},
		qualityScore:    0.6, // 质量不达标
		latencyMs:       100,
	}

	// ProviderB: 中成本, 高质量, 低延迟
	template.providers["ProviderB"] = &MockProvider{
		name:            "ProviderB",
		costPer1KTokens: 0.5,
		available:       true,
		models:          []string{"gpt-4"},
		qualityScore:    0.9,
		latencyMs:       150,
	}

	// ProviderC: 高成本, 高质量, 高延迟
	template.providers["ProviderC"] = &MockProvider{
		name:            "ProviderC",
		costPer1KTokens: 0.9,
		available:       true,
		models:          []string{"gpt-4"},
		qualityScore:    0.95,
		latencyMs:       400,
	}

	req := &RoutingRequest{
		Model:       "gpt-4",
		UserID:      "user123",
		MaxCost:     1.0,
		MaxLatency:  500,
		MinQuality:  0.7,
	}

	decision, err := template.SelectProvider(context.Background(), req)

	// 验证选择逻辑
	assert.NoError(t, err)
	assert.NotNil(t, decision)

	// ProviderA因质量不达标应被排除
	// ProviderB应在成本/质量/延迟权衡中胜出
	assert.Equal(t, "ProviderB", decision.Provider, "Should select balanced provider")
	assert.GreaterOrEqual(t, decision.QualityScore, 0.7, "Quality should meet minimum")
	assert.LessOrEqual(t, decision.CostPer1KTokens, 1.0, "Cost should be within budget")
	assert.LessOrEqual(t, decision.EstimatedLatency, int64(500), "Latency should be within limit")
}

// TestCostAwareStrategy_QualityThreshold 测试质量阈值过滤
func TestCostAwareStrategy_QualityThreshold(t *testing.T) {
	template := NewCostAwareTemplate("CostAware", CostAwareParams{
		MaxCostPer1KTokens: 1.0,
		MaxLatencyMs:      1000,
		MinQualityScore:    0.9, // 高质量要求
	})

	// 所有provider质量都不达标
	template.providers["ProviderA"] = &MockProvider{
		name:            "ProviderA",
		costPer1KTokens: 0.3,
		available:       true,
		models:          []string{"gpt-4"},
		qualityScore:    0.7,
		latencyMs:       100,
	}
	template.providers["ProviderB"] = &MockProvider{
		name:            "ProviderB",
		costPer1KTokens: 0.4,
		available:       true,
		models:          []string{"gpt-4"},
		qualityScore:    0.8,
		latencyMs:       150,
	}

	req := &RoutingRequest{
		Model:       "gpt-4",
		UserID:      "user123",
		MinQuality:  0.9,
	}

	decision, err := template.SelectProvider(context.Background(), req)

	// 应该返回错误，因为没有满足质量要求的provider
	assert.Error(t, err)
	assert.Nil(t, decision)
}
