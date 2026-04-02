package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestABStrategy_TrafficSplit 测试A/B测试流量分配
func TestABStrategy_TrafficSplit(t *testing.T) {
	ab := &ABStrategy{
		controlStrategy:    &RoutingStrategyTemplate{ID: "control"},
		experimentStrategy: &RoutingStrategyTemplate{ID: "experiment"},
		trafficSplit:       20, // 20%实验组
		bucketKey:          "user_id",
	}

	// 验证流量分配
	// 一致性哈希：同一user_id始终分配到同一组
	controlCount := 0
	experimentCount := 0

	for i := 0; i < 100; i++ {
		userID := string(rune('0' + i))
		isExperiment := ab.ShouldApplyToRequest(&RoutingRequest{UserID: userID})

		if isExperiment {
			experimentCount++
		} else {
			controlCount++
		}
	}

	// 验证一致性：同一user_id应该始终在同一组
	for i := 0; i < 10; i++ {
		userID := "test_user_123"
		first := ab.ShouldApplyToRequest(&RoutingRequest{UserID: userID})
		for j := 0; j < 10; j++ {
			second := ab.ShouldApplyToRequest(&RoutingRequest{UserID: userID})
			assert.Equal(t, first, second, "Same user_id should always be in same group")
		}
	}

	// 验证分配比例大约是80:20
	assert.InDelta(t, 80, controlCount, 15, "Control should be around 80%%")
	assert.InDelta(t, 20, experimentCount, 15, "Experiment should be around 20%%")
}

// TestRollout_Percentage 测试灰度发布百分比递增
func TestRollout_Percentage(t *testing.T) {
	rollout := &RolloutStrategy{
		percentage: 10,
		bucketKey: "user_id",
	}

	// 统计10%时的用户数
	count10 := 0
	for i := 0; i < 100; i++ {
		userID := string(rune('0' + i))
		if rollout.ShouldApply(&RoutingRequest{UserID: userID}) {
			count10++
		}
	}
	assert.InDelta(t, 10, count10, 5, "10%% rollout should have around 10 users")

	// 增加百分比到20%
	rollout.SetPercentage(20)

	// 统计20%时的用户数
	count20 := 0
	for i := 0; i < 100; i++ {
		userID := string(rune('0' + i))
		if rollout.ShouldApply(&RoutingRequest{UserID: userID}) {
			count20++
		}
	}
	assert.InDelta(t, 20, count20, 5, "20%% rollout should have around 20 users")

	// 增加百分比到50%
	rollout.SetPercentage(50)

	// 统计50%时的用户数
	count50 := 0
	for i := 0; i < 100; i++ {
		userID := string(rune('0' + i))
		if rollout.ShouldApply(&RoutingRequest{UserID: userID}) {
			count50++
		}
	}
	assert.InDelta(t, 50, count50, 10, "50%% rollout should have around 50 users")

	// 增加百分比到100%
	rollout.SetPercentage(100)

	// 验证100%时所有用户都在
	for i := 0; i < 100; i++ {
		userID := string(rune('0' + i))
		assert.True(t, rollout.ShouldApply(&RoutingRequest{UserID: userID}), "All users should be in 100% rollout")
	}
}

// TestRollout_Consistency 测试灰度发布一致性
func TestRollout_Consistency(t *testing.T) {
	rollout := &RolloutStrategy{
		percentage: 30,
		bucketKey: "user_id",
	}

	// 同一用户应该始终被同样对待
	userID := "consistent_user"
	firstResult := rollout.ShouldApply(&RoutingRequest{UserID: userID})

	for i := 0; i < 100; i++ {
		result := rollout.ShouldApply(&RoutingRequest{UserID: userID})
		assert.Equal(t, firstResult, result, "Same user should always have same result")
	}
}

// TestRollout_PercentageIncrease 测试百分比递增
func TestRollout_PercentageIncrease(t *testing.T) {
	rollout := &RolloutStrategy{
		percentage: 10,
		bucketKey: "user_id",
	}

	// 收集10%时的用户
	var in10Percent []string
	for i := 0; i < 100; i++ {
		userID := string(rune('a' + i))
		if rollout.ShouldApply(&RoutingRequest{UserID: userID}) {
			in10Percent = append(in10Percent, userID)
		}
	}

	// 增加百分比到50%
	rollout.SetPercentage(50)

	// 收集50%时的用户
	var in50Percent []string
	for i := 0; i < 100; i++ {
		userID := string(rune('a' + i))
		if rollout.ShouldApply(&RoutingRequest{UserID: userID}) {
			in50Percent = append(in50Percent, userID)
		}
	}

	// 50%的用户应该包含10%的用户（一致性）
	for _, userID := range in10Percent {
		found := false
		for _, id := range in50Percent {
			if userID == id {
				found = true
				break
			}
		}
		assert.True(t, found, "10%% users should be included in 50%% rollout")
	}

	// 50%应该包含更多用户
	assert.Greater(t, len(in50Percent), len(in10Percent), "50%% should have more users than 10%%")
}
