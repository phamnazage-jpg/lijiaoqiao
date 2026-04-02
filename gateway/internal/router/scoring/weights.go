package scoring

// ScoreWeights 评分权重配置
type ScoreWeights struct {
	// LatencyWeight 延迟权重 (40%)
	LatencyWeight float64
	// AvailabilityWeight 可用性权重 (30%)
	AvailabilityWeight float64
	// CostWeight 成本权重 (20%)
	CostWeight float64
	// QualityWeight 质量权重 (10%)
	QualityWeight float64
}

// DefaultWeights 默认权重配置
// LatencyWeight = 0.4 (40%)
// AvailabilityWeight = 0.3 (30%)
// CostWeight = 0.2 (20%)
// QualityWeight = 0.1 (10%)
var DefaultWeights = ScoreWeights{
	LatencyWeight:      0.4,
	AvailabilityWeight: 0.3,
	CostWeight:          0.2,
	QualityWeight:       0.1,
}
