package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/gateway/internal/adapter"
)

// TestStrategyTemplate_Interface 验证策略模板接口
func TestStrategyTemplate_Interface(t *testing.T) {
	// 所有策略实现必须实现SelectProvider, Name, Type方法

	// 创建策略实现示例
	costBased := &CostBasedTemplate{
		name: "CostBased",
	}

	aware := &CostAwareTemplate{
		name: "CostAware",
	}

	// 验证实现了StrategyTemplate接口
	var _ StrategyTemplate = costBased
	var _ StrategyTemplate = aware

	// 验证方法
	assert.Equal(t, "CostBased", costBased.Name())
	assert.Equal(t, "cost_based", costBased.Type())

	assert.Equal(t, "CostAware", aware.Name())
	assert.Equal(t, "cost_aware", aware.Type())
}

// TestStrategyTemplate_SelectProvider_Signature 验证SelectProvider方法签名
func TestStrategyTemplate_SelectProvider_Signature(t *testing.T) {
	req := &RoutingRequest{
		Model:      "gpt-4",
		UserID:     "user123",
		TenantID:  "tenant1",
		MaxCost:   1.0,
		MaxLatency: 1000,
	}

	// 验证返回值 - 创建一个有providers的模板
	template := &CostBasedTemplate{
		name:               "test",
		maxCostPer1KTokens: 1.0,
		providers:          make(map[string]adapter.ProviderAdapter),
	}
	template.providers["test"] = &MockProvider{
		name:            "test",
		costPer1KTokens: 0.5,
		available:       true,
		models:          []string{"gpt-4"},
	}

	decision, err := template.SelectProvider(context.Background(), req)

	// 接口实现应该返回决策或错误
	assert.NotNil(t, decision)
	assert.Nil(t, err)
}
