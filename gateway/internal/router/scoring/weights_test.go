package scoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScoreWeights_DefaultValues(t *testing.T) {
	// 验证默认权重
	// LatencyWeight = 0.4 (40%)
	// AvailabilityWeight = 0.3 (30%)
	// CostWeight = 0.2 (20%)
	// QualityWeight = 0.1 (10%)

	assert.Equal(t, 0.4, DefaultWeights.LatencyWeight, "LatencyWeight should be 0.4 (40%%)")
	assert.Equal(t, 0.3, DefaultWeights.AvailabilityWeight, "AvailabilityWeight should be 0.3 (30%%)")
	assert.Equal(t, 0.2, DefaultWeights.CostWeight, "CostWeight should be 0.2 (20%%)")
	assert.Equal(t, 0.1, DefaultWeights.QualityWeight, "QualityWeight should be 0.1 (10%%)")
}

func TestScoreWeights_Sum(t *testing.T) {
	// 验证权重总和为1.0
	total := DefaultWeights.LatencyWeight +
		DefaultWeights.AvailabilityWeight +
		DefaultWeights.CostWeight +
		DefaultWeights.QualityWeight

	assert.InDelta(t, 1.0, total, 0.001, "Weights sum should be 1.0")
}
