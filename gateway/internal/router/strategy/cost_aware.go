package strategy

import (
	"context"
	"errors"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/router/scoring"
	gwerror "lijiaoqiao/gateway/pkg/error"
)

// ErrNoQualifiedProvider 没有符合条件的Provider
var ErrNoQualifiedProvider = errors.New("no qualified provider available")

// CostAwareTemplate 成本感知策略模板
// 综合考虑成本、质量、延迟进行权衡
type CostAwareTemplate struct {
	name               string
	maxCostPer1KTokens float64
	maxLatencyMs      int64
	minQualityScore    float64
	providers          map[string]adapter.ProviderAdapter
	scoringModel       *scoring.ScoringModel
}

// CostAwareParams 成本感知参数
type CostAwareParams struct {
	MaxCostPer1KTokens float64
	MaxLatencyMs      int64
	MinQualityScore    float64
}

// NewCostAwareTemplate 创建成本感知策略模板
func NewCostAwareTemplate(name string, params CostAwareParams) *CostAwareTemplate {
	return &CostAwareTemplate{
		name:               name,
		maxCostPer1KTokens: params.MaxCostPer1KTokens,
		maxLatencyMs:      params.MaxLatencyMs,
		minQualityScore:    params.MinQualityScore,
		providers:          make(map[string]adapter.ProviderAdapter),
		scoringModel:       scoring.NewScoringModel(scoring.DefaultWeights),
	}
}

// RegisterProvider 注册Provider
func (t *CostAwareTemplate) RegisterProvider(name string, provider adapter.ProviderAdapter) {
	t.providers[name] = provider
}

// Name 获取策略名称
func (t *CostAwareTemplate) Name() string {
	return t.name
}

// Type 获取策略类型
func (t *CostAwareTemplate) Type() string {
	return "cost_aware"
}

// SelectProvider 选择最佳平衡的Provider
func (t *CostAwareTemplate) SelectProvider(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error) {
	if len(t.providers) == 0 {
		return nil, gwerror.NewGatewayError(gwerror.ROUTER_NO_PROVIDER_AVAILABLE, "no provider registered")
	}

	type candidate struct {
		name           string
		cost           float64
		quality        float64
		latency        int64
		score          float64
	}

	var candidates []candidate
	maxCost := t.maxCostPer1KTokens
	if req.MaxCost > 0 && req.MaxCost < maxCost {
		maxCost = req.MaxCost
	}
	maxLatency := t.maxLatencyMs
	if req.MaxLatency > 0 && req.MaxLatency < maxLatency {
		maxLatency = req.MaxLatency
	}
	minQuality := t.minQualityScore
	if req.MinQuality > 0 && req.MinQuality > minQuality {
		minQuality = req.MinQuality
	}

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

		// 获取provider指标
		cost := t.getProviderCost(provider)
		quality := t.getProviderQuality(provider)
		latency := t.getProviderLatency(provider)

		// 过滤不满足基本条件的provider
		if cost > maxCost || latency > maxLatency || quality < minQuality {
			continue
		}

		// 计算综合评分
		metrics := scoring.ProviderMetrics{
			Name:            name,
			LatencyMs:       latency,
			Availability:    1.0, // 假设可用
			CostPer1KTokens: cost,
			QualityScore:    quality,
		}
		score := t.scoringModel.CalculateScore(metrics)

		candidates = append(candidates, candidate{
			name:   name,
			cost:   cost,
			quality: quality,
			latency: latency,
			score:  score,
		})
	}

	if len(candidates) == 0 {
		return nil, ErrNoQualifiedProvider
	}

	// 选择评分最高的provider
	best := &candidates[0]
	for i := 1; i < len(candidates); i++ {
		if candidates[i].score > best.score {
			best = &candidates[i]
		}
	}

	return &RoutingDecision{
		Provider:         best.name,
		Strategy:         t.Type(),
		CostPer1KTokens:  best.cost,
		EstimatedLatency: best.latency,
		QualityScore:     best.quality,
		TakeoverMark:     true, // M-008: 标记为接管
	}, nil
}

// getProviderCost 获取Provider的成本
func (t *CostAwareTemplate) getProviderCost(provider adapter.ProviderAdapter) float64 {
	if cp, ok := provider.(CostAwareProvider); ok {
		return cp.GetCostPer1KTokens()
	}
	return 0.5
}

// getProviderQuality 获取Provider的质量分数
func (t *CostAwareTemplate) getProviderQuality(provider adapter.ProviderAdapter) float64 {
	if qp, ok := provider.(QualityProvider); ok {
		return qp.GetQualityScore()
	}
	return 0.8 // 默认质量分数
}

// getProviderLatency 获取Provider的延迟
func (t *CostAwareTemplate) getProviderLatency(provider adapter.ProviderAdapter) int64 {
	if lp, ok := provider.(LatencyProvider); ok {
		return lp.GetLatencyMs()
	}
	return 100 // 默认延迟100ms
}

// QualityProvider 质量感知Provider接口
type QualityProvider interface {
	GetQualityScore() float64
}

// LatencyProvider 延迟感知Provider接口
type LatencyProvider interface {
	GetLatencyMs() int64
}
