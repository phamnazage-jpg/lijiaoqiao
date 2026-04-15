package domain

// ==================== P0-06 Outbox共享重试策略 ====================

// DefaultMaxRetries 默认最大重试次数
const DefaultMaxRetries = 5

// DefaultInitialBackoffSeconds 默认初始退避时间（秒）
const DefaultInitialBackoffSeconds = 1

// DefaultMaxBackoffSeconds 默认最大退避时间（秒）
const DefaultMaxBackoffSeconds = 60

// DefaultOutboxBatchSize 默认批量处理大小
const DefaultOutboxBatchSize = 100

// OutboxRetryConfig Outbox重试配置
type OutboxRetryConfig struct {
	MaxRetries            int
	InitialBackoffSeconds int
	MaxBackoffSeconds     int
	BatchSize             int
}

// DefaultOutboxRetryConfig 默认重试配置
func DefaultOutboxRetryConfig() OutboxRetryConfig {
	return OutboxRetryConfig{
		MaxRetries:            DefaultMaxRetries,
		InitialBackoffSeconds: DefaultInitialBackoffSeconds,
		MaxBackoffSeconds:     DefaultMaxBackoffSeconds,
		BatchSize:             DefaultOutboxBatchSize,
	}
}

// CalculateOutboxBackoff 计算指数退避时间
func CalculateOutboxBackoff(retryCount, maxRetries int) int {
	_ = maxRetries

	if retryCount <= 0 {
		retryCount = 1
	}

	backoff := DefaultInitialBackoffSeconds
	for attempt := 1; attempt < retryCount; attempt++ {
		if backoff >= DefaultMaxBackoffSeconds {
			return DefaultMaxBackoffSeconds
		}
		if backoff > DefaultMaxBackoffSeconds/2 {
			return DefaultMaxBackoffSeconds
		}
		backoff *= 2
	}
	return backoff
}
