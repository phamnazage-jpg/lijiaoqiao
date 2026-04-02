package strategy

import (
	"fmt"
	"hash/fnv"
	"sync"
)

// RolloutStrategy 灰度发布策略
type RolloutStrategy struct {
	percentage int    // 当前灰度百分比 (0-100)
	bucketKey  string // 分桶key
	mu         sync.RWMutex
}

// NewRolloutStrategy 创建灰度发布策略
func NewRolloutStrategy(percentage int, bucketKey string) *RolloutStrategy {
	return &RolloutStrategy{
		percentage: percentage,
		bucketKey:  bucketKey,
	}
}

// SetPercentage 设置灰度百分比
func (r *RolloutStrategy) SetPercentage(percentage int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	r.percentage = percentage
}

// GetPercentage 获取当前灰度百分比
func (r *RolloutStrategy) GetPercentage() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.percentage
}

// ShouldApply 判断请求是否应该在灰度范围内
func (r *RolloutStrategy) ShouldApply(req *RoutingRequest) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.percentage >= 100 {
		return true
	}
	if r.percentage <= 0 {
		return false
	}

	// 一致性哈希分桶
	bucket := r.hashString(fmt.Sprintf("%s:%s", r.bucketKey, req.UserID)) % 100
	return bucket < r.percentage
}

// hashString 计算字符串哈希值 (用于一致性分桶)
func (r *RolloutStrategy) hashString(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}

// IncrementPercentage 增加灰度百分比
func (r *RolloutStrategy) IncrementPercentage(delta int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.percentage += delta
	if r.percentage > 100 {
		r.percentage = 100
	}
}
