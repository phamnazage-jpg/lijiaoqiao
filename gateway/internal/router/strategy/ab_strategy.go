package strategy

import (
	"fmt"
	"hash/fnv"
	"time"
)

// ABStrategy A/B测试策略
type ABStrategy struct {
	controlStrategy     *RoutingStrategyTemplate
	experimentStrategy  *RoutingStrategyTemplate
	trafficSplit        int    // 实验组流量百分比 (0-100)
	bucketKey           string // 分桶key
	experimentID        string
	startTime           *time.Time
	endTime             *time.Time
}

// NewABStrategy 创建A/B测试策略
func NewABStrategy(control, experiment *RoutingStrategyTemplate, split int, bucketKey string) *ABStrategy {
	return &ABStrategy{
		controlStrategy:    control,
		experimentStrategy: experiment,
		trafficSplit:       split,
		bucketKey:          bucketKey,
	}
}

// ShouldApplyToRequest 判断请求是否应该使用实验组策略
func (a *ABStrategy) ShouldApplyToRequest(req *RoutingRequest) bool {
	// 检查时间范围
	now := time.Now()
	if a.startTime != nil && now.Before(*a.startTime) {
		return false
	}
	if a.endTime != nil && now.After(*a.endTime) {
		return false
	}

	// 一致性哈希分桶
	bucket := a.hashString(fmt.Sprintf("%s:%s", a.bucketKey, req.UserID)) % 100
	return bucket < a.trafficSplit
}

// hashString 计算字符串哈希值 (用于一致性分桶)
func (a *ABStrategy) hashString(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}

// GetControlStrategy 获取对照组策略
func (a *ABStrategy) GetControlStrategy() *RoutingStrategyTemplate {
	return a.controlStrategy
}

// GetExperimentStrategy 获取实验组策略
func (a *ABStrategy) GetExperimentStrategy() *RoutingStrategyTemplate {
	return a.experimentStrategy
}

// RoutingStrategyTemplate 路由策略模板
type RoutingStrategyTemplate struct {
	ID          string
	Name        string
	Type        string
	Priority    int
	Enabled     bool
	Description string
}
