package engine

import (
	"context"
	"errors"

	"lijiaoqiao/gateway/internal/router/strategy"
)

// ErrStrategyNotFound 策略未找到
var ErrStrategyNotFound = errors.New("strategy not found")

// RoutingMetrics 路由指标接口
type RoutingMetrics interface {
	// RecordSelection 记录路由选择
	RecordSelection(provider string, strategyName string, decision *strategy.RoutingDecision)
}

// RoutingEngine 路由引擎
type RoutingEngine struct {
	strategies map[string]strategy.StrategyTemplate
	metrics    RoutingMetrics
}

// NewRoutingEngine 创建路由引擎
func NewRoutingEngine() *RoutingEngine {
	return &RoutingEngine{
		strategies: make(map[string]strategy.StrategyTemplate),
		metrics:    nil,
	}
}

// RegisterStrategy 注册路由策略
func (e *RoutingEngine) RegisterStrategy(name string, template strategy.StrategyTemplate) {
	e.strategies[name] = template
}

// SetMetrics 设置指标收集器
func (e *RoutingEngine) SetMetrics(metrics RoutingMetrics) {
	e.metrics = metrics
}

// SelectProvider 根据策略选择Provider
func (e *RoutingEngine) SelectProvider(ctx context.Context, req *strategy.RoutingRequest, strategyName string) (*strategy.RoutingDecision, error) {
	// 查找策略
	tpl, ok := e.strategies[strategyName]
	if !ok {
		return nil, ErrStrategyNotFound
	}

	// 执行策略选择
	decision, err := tpl.SelectProvider(ctx, req)
	if err != nil {
		return nil, err
	}

	// 记录指标
	if e.metrics != nil && decision != nil {
		e.metrics.RecordSelection(decision.Provider, decision.Strategy, decision)
	}

	return decision, nil
}
