package strategy

import (
	"context"
	"errors"
	"sort"

	"lijiaoqiao/gateway/internal/adapter"
	gwerror "lijiaoqiao/gateway/pkg/error"
)

// ErrNoAffordableProvider 没有可负担的Provider
var ErrNoAffordableProvider = errors.New("no affordable provider available")

// CostBasedTemplate 成本优先策略模板
// 选择成本最低的provider
type CostBasedTemplate struct {
	name               string
	maxCostPer1KTokens float64
	providers          map[string]adapter.ProviderAdapter
}

// CostParams 成本参数
type CostParams struct {
	// 最大成本 ($/1K tokens)
	MaxCostPer1KTokens float64
}

// NewCostBasedTemplate 创建成本优先策略模板
func NewCostBasedTemplate(name string, params CostParams) *CostBasedTemplate {
	return &CostBasedTemplate{
		name:               name,
		maxCostPer1KTokens: params.MaxCostPer1KTokens,
		providers:          make(map[string]adapter.ProviderAdapter),
	}
}

// RegisterProvider 注册Provider
func (t *CostBasedTemplate) RegisterProvider(name string, provider adapter.ProviderAdapter) {
	t.providers[name] = provider
}

// Name 获取策略名称
func (t *CostBasedTemplate) Name() string {
	return t.name
}

// Type 获取策略类型
func (t *CostBasedTemplate) Type() string {
	return "cost_based"
}

// SelectProvider 选择成本最低的Provider
func (t *CostBasedTemplate) SelectProvider(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error) {
	if len(t.providers) == 0 {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no provider registered")
	}

	// 收集所有可用provider的候选列表
	type candidate struct {
		name string
		cost float64
	}
	var candidates []candidate

	for name, provider := range t.providers {
		// 检查provider是否支持该模型
		supported := false
		for _, m := range provider.SupportedModels() {
			if m == req.Model || m == "*" {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}

		// 检查健康状态
		if !provider.HealthCheck(ctx) {
			continue
		}

		// 获取成本信息 (实际实现需要从provider获取)
		// 这里暂时设置为模拟值
		cost := t.getProviderCost(provider)
		candidates = append(candidates, candidate{name: name, cost: cost})
	}

	if len(candidates) == 0 {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no available provider for model: "+req.Model)
	}

	// 按成本排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].cost < candidates[j].cost
	})

	// 选择成本最低且在预算内的provider
	maxCost := t.maxCostPer1KTokens
	if req.MaxCost > 0 && req.MaxCost < maxCost {
		maxCost = req.MaxCost
	}

	for _, c := range candidates {
		if c.cost <= maxCost {
			return &RoutingDecision{
				Provider:        c.name,
				Strategy:        t.Type(),
				CostPer1KTokens: c.cost,
				TakeoverMark:    true, // M-008: 标记为接管
			}, nil
		}
	}

	return nil, ErrNoAffordableProvider
}

// CostAwareProvider 成本感知Provider接口
type CostAwareProvider interface {
	GetCostPer1KTokens() float64
}

// getProviderCost 获取Provider的成本
func (t *CostBasedTemplate) getProviderCost(provider adapter.ProviderAdapter) float64 {
	// 尝试类型断言获取成本
	if cp, ok := provider.(CostAwareProvider); ok {
		return cp.GetCostPer1KTokens()
	}
	// 默认返回0.5，实际应从provider元数据获取
	return 0.5
}
