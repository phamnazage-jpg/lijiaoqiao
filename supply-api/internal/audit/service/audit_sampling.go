package service

import (
	"math/rand"
	"strings"
	"sync"
)

// ==================== P1-04 审计事件采样策略 ====================

// AuditSamplingConfig 审计采样配置
type AuditSamplingConfig struct {
	// 成功事件采样率 (0.0 - 1.0)
	// 0.01 = 1% 采样
	SuccessSampleRate float64

	// 是否启用采样
	Enabled bool

	// 强制全量记录的事件类型（不受采样影响）
	ForceRecordEventTypes []string
}

// DefaultAuditSamplingConfig 默认审计采样配置
func DefaultAuditSamplingConfig() *AuditSamplingConfig {
	return &AuditSamplingConfig{
		Enabled:           true,
		SuccessSampleRate: 0.01, // 1% 采样
		ForceRecordEventTypes: []string{
			"token.authn.fail",
			"token.authz.fail",
			"token.revoked",
			"account.created",
			"account.deleted",
			"settlement.completed",
			"payment.processed",
			"admin.action",
		},
	}
}

// AuditSampler 审计事件采样器
type AuditSampler struct {
	config  *AuditSamplingConfig
	sampled *rand.Rand
	mu      sync.Mutex
}

// NewAuditSampler 创建审计采样器
func NewAuditSampler(config *AuditSamplingConfig) *AuditSampler {
	if config == nil {
		config = DefaultAuditSamplingConfig()
	}

	return &AuditSampler{
		config:  config,
		sampled: rand.New(rand.NewSource(42)), // 确定性采样
	}
}

// ShouldRecord 判断事件是否应该记录
// 返回 true 表示记录，false 表示采样丢弃
func (s *AuditSampler) ShouldRecord(eventName string, success bool) bool {
	if !s.config.Enabled {
		return true
	}

	// 失败事件总是记录
	if !success {
		return true
	}

	// 强制记录类型总是记录（支持前缀匹配）
	for _, forcedType := range s.config.ForceRecordEventTypes {
		if eventName == forcedType || strings.HasPrefix(eventName, forcedType) {
			return true
		}
	}

	// 成功事件按采样率决定
	return s.ShouldSample()
}

// ShouldSample 判断成功事件是否应该采样
func (s *AuditSampler) ShouldSample() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := s.sampled.Float64()
	return r < s.config.SuccessSampleRate
}

// RecordRate 返回当前采样率
func (s *AuditSampler) RecordRate() float64 {
	return 1.0 - s.config.SuccessSampleRate
}

// GetConfig 返回采样配置
func (s *AuditSampler) GetConfig() *AuditSamplingConfig {
	return s.config
}

// AuditEventClassifier 审计事件分类器
type AuditEventClassifier struct{}

// NewAuditEventClassifier 创建事件分类器
func NewAuditEventClassifier() *AuditEventClassifier {
	return &AuditEventClassifier{}
}

// IsHighPriorityEvent 判断是否为高优先级事件（失败事件）
func (c *AuditEventClassifier) IsHighPriorityEvent(eventName string, success bool) bool {
	if !success {
		return true
	}

	highPriorityPrefixes := []string{
		"token.authn.fail",
		"token.authz.fail",
		"token.revoked",
		"account.",
		"settlement.",
		"payment.",
		"admin.",
	}

	lowPriorityPrefixes := []string{
		"token.authn.success",
		"token.access",
		"api.request",
	}

	// 检查是否低优先级
	for _, prefix := range lowPriorityPrefixes {
		if strings.HasPrefix(eventName, prefix) {
			return false
		}
	}

	// 检查是否高优先级
	for _, prefix := range highPriorityPrefixes {
		if strings.HasPrefix(eventName, prefix) {
			return true
		}
	}

	return false
}

// GetSamplingRecommendation 获取采样建议
func (c *AuditEventClassifier) GetSamplingRecommendation(eventName string) string {
	if strings.HasPrefix(eventName, "token.authn.success") {
		return "1% sampling recommended"
	}
	if strings.HasPrefix(eventName, "token.authn.fail") {
		return "100% record required"
	}
	if strings.HasPrefix(eventName, "account.") {
		return "100% record required"
	}
	if strings.HasPrefix(eventName, "settlement.") {
		return "100% record required"
	}
	if strings.HasPrefix(eventName, "payment.") {
		return "100% record required"
	}
	if strings.HasPrefix(eventName, "api.request") {
		return "1% sampling recommended"
	}
	return "10% sampling recommended"
}
