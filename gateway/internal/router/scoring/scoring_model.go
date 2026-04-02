package scoring

import (
	"math"
)

// ProviderMetrics Provider评分指标
type ProviderMetrics struct {
	Name            string
	LatencyMs       int64
	Availability    float64
	CostPer1KTokens float64
	QualityScore    float64
}

// ScoringModel 评分模型
type ScoringModel struct {
	weights ScoreWeights
}

// NewScoringModel 创建评分模型
func NewScoringModel(weights ScoreWeights) *ScoringModel {
	return &ScoringModel{
		weights: weights,
	}
}

// CalculateScore 计算单个Provider的综合评分
// 评分范围: 0.0 - 1.0, 越高越好
func (m *ScoringModel) CalculateScore(provider ProviderMetrics) float64 {
	// 计算各维度得分

	// 延迟得分: 使用指数衰减，越低越好
	// 基准延迟100ms，得分0.5；延迟0ms得分1.0
	latencyScore := math.Exp(-float64(provider.LatencyMs) / 200.0)

	// 可用性得分: 直接使用可用性值
	availabilityScore := provider.Availability

	// 成本得分: 使用指数衰减，越低越好
	// 基准成本$1/1K tokens，得分0.5；成本0得分1.0
	costScore := math.Exp(-provider.CostPer1KTokens)

	// 质量得分: 直接使用质量分数
	qualityScore := provider.QualityScore

	// 综合评分 = 延迟权重*延迟得分 + 可用性权重*可用性得分 + 成本权重*成本得分 + 质量权重*质量得分
	totalScore := m.weights.LatencyWeight*latencyScore +
		m.weights.AvailabilityWeight*availabilityScore +
		m.weights.CostWeight*costScore +
		m.weights.QualityWeight*qualityScore

	return math.Max(0, math.Min(1, totalScore))
}

// SelectBestProvider 从候选列表中选择最佳Provider
func (m *ScoringModel) SelectBestProvider(providers []ProviderMetrics) *ProviderMetrics {
	if len(providers) == 0 {
		return nil
	}

	best := &providers[0]
	bestScore := m.CalculateScore(*best)

	for i := 1; i < len(providers); i++ {
		score := m.CalculateScore(providers[i])
		if score > bestScore {
			best = &providers[i]
			bestScore = score
		}
	}

	return best
}
