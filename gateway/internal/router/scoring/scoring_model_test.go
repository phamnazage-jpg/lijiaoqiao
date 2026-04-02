package scoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScoringModel_CalculateScore_Latency(t *testing.T) {
	// 低延迟应该得高分
	model := NewScoringModel(DefaultWeights)

	// Provider A: 延迟100ms
	providerA := ProviderMetrics{
		Name:    "ProviderA",
		LatencyMs: 100,
	}

	// Provider B: 延迟200ms
	providerB := ProviderMetrics{
		Name:    "ProviderB",
		LatencyMs: 200,
	}

	scoreA := model.CalculateScore(providerA)
	scoreB := model.CalculateScore(providerB)

	// 延迟低的应该分数高
	assert.Greater(t, scoreA, scoreB, "Lower latency should result in higher score")
}

func TestScoringModel_CalculateScore_Availability(t *testing.T) {
	// 高可用应该得高分
	model := NewScoringModel(DefaultWeights)

	// Provider A: 可用性 99%
	providerA := ProviderMetrics{
		Name:         "ProviderA",
		Availability: 0.99,
	}

	// Provider B: 可用性 90%
	providerB := ProviderMetrics{
		Name:         "ProviderB",
		Availability: 0.90,
	}

	scoreA := model.CalculateScore(providerA)
	scoreB := model.CalculateScore(providerB)

	// 可用性高的应该分数高
	assert.Greater(t, scoreA, scoreB, "Higher availability should result in higher score")
}

func TestScoringModel_CalculateScore_Cost(t *testing.T) {
	// 低成本应该得高分
	model := NewScoringModel(DefaultWeights)

	// Provider A: 成本 $0.5/1K tokens
	providerA := ProviderMetrics{
		Name:            "ProviderA",
		CostPer1KTokens: 0.5,
	}

	// Provider B: 成本 $1.0/1K tokens
	providerB := ProviderMetrics{
		Name:            "ProviderB",
		CostPer1KTokens: 1.0,
	}

	scoreA := model.CalculateScore(providerA)
	scoreB := model.CalculateScore(providerB)

	// 成本低的应该分数高
	assert.Greater(t, scoreA, scoreB, "Lower cost should result in higher score")
}

func TestScoringModel_CalculateScore_Quality(t *testing.T) {
	// 高质量应该得高分
	model := NewScoringModel(DefaultWeights)

	// Provider A: 质量 0.95
	providerA := ProviderMetrics{
		Name:         "ProviderA",
		QualityScore: 0.95,
	}

	// Provider B: 质量 0.80
	providerB := ProviderMetrics{
		Name:         "ProviderB",
		QualityScore: 0.80,
	}

	scoreA := model.CalculateScore(providerA)
	scoreB := model.CalculateScore(providerB)

	// 质量高的应该分数高
	assert.Greater(t, scoreA, scoreB, "Higher quality should result in higher score")
}

func TestScoringModel_CalculateScore_Combined(t *testing.T) {
	// 综合评分正确
	model := NewScoringModel(DefaultWeights)

	// 完美provider: 延迟0ms, 可用性100%, 成本0$/1K, 质量1.0
	perfect := ProviderMetrics{
		Name:            "Perfect",
		LatencyMs:       0,
		Availability:    1.0,
		CostPer1KTokens: 0,
		QualityScore:    1.0,
	}

	// 最差provider: 延迟1000ms, 可用性0%, 成本10$/1K, 质量0
	worst := ProviderMetrics{
		Name:            "Worst",
		LatencyMs:       1000,
		Availability:    0.0,
		CostPer1KTokens: 10.0,
		QualityScore:    0.0,
	}

	scorePerfect := model.CalculateScore(perfect)
	scoreWorst := model.CalculateScore(worst)

	// 完美的应该分数高
	assert.Greater(t, scorePerfect, scoreWorst, "Perfect provider should score higher than worst")

	// 完美分数应该在合理范围内 (接近1.0)
	assert.LessOrEqual(t, scorePerfect, 1.0, "Perfect score should be <= 1.0")
	assert.Greater(t, scorePerfect, 0.9, "Perfect score should be > 0.9")
}

func TestScoringModel_SelectBestProvider(t *testing.T) {
	// 选择最佳provider
	model := NewScoringModel(DefaultWeights)

	providers := []ProviderMetrics{
		{Name: "ProviderA", LatencyMs: 100, Availability: 0.99, CostPer1KTokens: 0.5, QualityScore: 0.9},
		{Name: "ProviderB", LatencyMs: 50, Availability: 0.95, CostPer1KTokens: 0.8, QualityScore: 0.85},
		{Name: "ProviderC", LatencyMs: 200, Availability: 0.99, CostPer1KTokens: 0.3, QualityScore: 0.8},
	}

	best := model.SelectBestProvider(providers)

	// 验证返回了provider
	assert.NotNil(t, best, "Should return a provider")
	assert.Equal(t, "ProviderB", best.Name, "ProviderB should be selected (low latency with good balance)")
}
