package strategy

import (
	"context"
)

// RoutingRequest 路由请求
type RoutingRequest struct {
	Model         string
	UserID        string
	TenantID      string
	Region        string
	Messages      []string
	MaxCost       float64
	MaxLatency    int64
	MinQuality    float64
}

// RoutingDecision 路由决策
type RoutingDecision struct {
	Provider         string
	Strategy         string
	CostPer1KTokens  float64
	EstimatedLatency int64
	QualityScore     float64
	TakeoverMark     bool // M-008: 是否标记为接管
}

// StrategyTemplate 策略模板接口
// 所有路由策略都必须实现此接口
type StrategyTemplate interface {
	// SelectProvider 选择最佳Provider
	SelectProvider(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error)

	// Name 获取策略名称
	Name() string

	// Type 获取策略类型
	Type() string
}
