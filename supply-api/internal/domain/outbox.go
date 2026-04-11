package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// ==================== P0-06 Outbox模式实现 ====================

// Outbox 事务性说明：
// 1. 在业务事务内：当账户创建、套餐变更、结算发生时，将领域事件写入 outbox 表
// 2. 事务保证：outbox 事件与业务数据在同一个数据库事务中提交，确保原子性
// 3. 异步处理：OutboxProcessor 定期扫描 outbox 表，发布事件到消息队列
// 4. 幂等发布：处理器通过 FetchAndLock 实现分布式锁，确保每条消息只被处理一次
// 5. 失败重试：发布失败的事件会根据退避策略重试，最大重试次数为 DefaultMaxRetries
// 6. 死信处理：超过最大重试次数的事件移入 dead_letter 表，供人工处理

// OutboxEvent Outbox事件
type OutboxEvent struct {
	ID              int64     `json:"id"`
	AggregateType   string    `json:"aggregate_type"`
	AggregateID     string    `json:"aggregate_id"`
	EventType       string    `json:"event_type"`
	EventID         string    `json:"event_id"`
	Payload         json.RawMessage `json:"payload"`
	Status          string    `json:"status"` // pending, processing, completed, failed, dead_letter
	RetryCount      int       `json:"retry_count"`
	MaxRetries      int       `json:"max_retries"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
	NextRetryAt     *time.Time `json:"next_retry_at,omitempty"`
	DeadLetterReason string   `json:"dead_letter_reason,omitempty"`
	Version         int64     `json:"version"`
}

// OutboxStatus Outbox事件状态
const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusCompleted  = "completed"
	OutboxStatusFailed     = "failed"
	OutboxStatusDeadLetter = "dead_letter"
)

// OutboxDeadLetter 死信记录
type OutboxDeadLetter struct {
	ID                 int64     `json:"id"`
	OriginalEventID    string    `json:"original_event_id"`
	OriginalAggregateType string  `json:"original_aggregate_type"`
	OriginalAggregateID string   `json:"original_aggregate_id"`
	EventType          string    `json:"event_type"`
	Payload            json.RawMessage `json:"payload"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	RetryCount         int       `json:"retry_count"`
	FirstFailedAt      time.Time `json:"first_failed_at"`
	DeadLetterAt       time.Time `json:"dead_letter_at"`
	Handled            bool      `json:"handled"`
	HandledAt          *time.Time `json:"handled_at,omitempty"`
	HandlerNotes       string    `json:"handler_notes,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// OutboxProcessor Outbox处理器
type OutboxProcessor struct {
	eventStore  OutboxEventStore
	messageBroker MessageBroker
	stats       OutboxStats
}

// OutboxEventStore Outbox事件存储接口
type OutboxEventStore interface {
	// FetchAndLock 获取并锁定待处理事件
	FetchAndLock(ctx context.Context, limit int) ([]*OutboxEvent, error)
	// MarkCompleted 标记完成
	MarkCompleted(ctx context.Context, eventID string) error
	// MarkFailed 标记失败
	MarkFailed(ctx context.Context, eventID string, errorMsg string) error
	// MoveToDeadLetter 移入死信队列
	MoveToDeadLetter(ctx context.Context, event *OutboxEvent, errorMsg string) error
}

// MessageBroker 消息代理接口
type MessageBroker interface {
	// Publish 发布消息
	Publish(ctx context.Context, event *OutboxEvent) error
}

// OutboxStats Outbox统计
type OutboxStats interface {
	RecordOutboxSuccess(eventType string)
	RecordOutboxFailure(reason string)
	RecordOutboxRetry(eventType string)
	RecordOutboxDLQ(eventType string)
}

// DefaultMaxRetries 默认最大重试次数
const DefaultMaxRetries = 5

// DefaultInitialBackoffSeconds 默认初始退避时间（秒）
const DefaultInitialBackoffSeconds = 1

// DefaultMaxBackoffSeconds 默认最大退避时间（秒）
const DefaultMaxBackoffSeconds = 60

// ProcessOutbox 处理Outbox事件
func (p *OutboxProcessor) ProcessOutbox(ctx context.Context) error {
	// 1. 获取待处理事件
	events, err := p.eventStore.FetchAndLock(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to fetch outbox events: %w", err)
	}

	for _, event := range events {
		// 2. 发布消息
		if err := p.messageBroker.Publish(ctx, event); err != nil {
			// 3. 处理失败
			p.handleFailure(ctx, event, err)
			continue
		}

		// 4. 标记完成
		if err := p.eventStore.MarkCompleted(ctx, event.EventID); err != nil {
			p.stats.RecordOutboxFailure("mark_completed_failed")
			continue
		}

		p.stats.RecordOutboxSuccess(event.EventType)
	}

	return nil
}

// handleFailure 处理失败事件
func (p *OutboxProcessor) handleFailure(ctx context.Context, event *OutboxEvent, publishErr error) {
	event.RetryCount++
	event.ErrorMessage = publishErr.Error()

	if event.RetryCount >= event.MaxRetries {
		// 移入死信队列
		if err := p.eventStore.MoveToDeadLetter(ctx, event, publishErr.Error()); err != nil {
			p.stats.RecordOutboxFailure("move_to_dlq_failed")
		} else {
			p.stats.RecordOutboxDLQ(event.EventType)
		}
	} else {
		// 计算下次重试时间（指数退避）
		backoffSeconds := calculateBackoff(event.RetryCount, event.MaxRetries)
		nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

		// 在存储层更新重试状态（这里简化处理）
		if err := p.eventStore.MarkFailed(ctx, event.EventID, publishErr.Error()); err != nil {
			p.stats.RecordOutboxFailure("mark_failed_failed")
		} else {
			p.stats.RecordOutboxRetry(event.EventType)
		}

		event.NextRetryAt = &nextRetry
	}
}

// calculateBackoff 计算指数退避时间
func calculateBackoff(retryCount, maxRetries int) int {
	backoff := DefaultInitialBackoffSeconds * int(math.Pow(2, float64(retryCount-1)))
	if backoff > DefaultMaxBackoffSeconds {
		backoff = DefaultMaxBackoffSeconds
	}
	return backoff
}

// OutboxProcessorConfig Outbox处理器配置
type OutboxProcessorConfig struct {
	MaxRetries           int
	InitialBackoffSeconds int
	MaxBackoffSeconds    int
	BatchSize            int
}

// DefaultOutboxProcessorConfig 默认配置
func DefaultOutboxProcessorConfig() *OutboxProcessorConfig {
	return &OutboxProcessorConfig{
		MaxRetries:           DefaultMaxRetries,
		InitialBackoffSeconds: DefaultInitialBackoffSeconds,
		MaxBackoffSeconds:    DefaultMaxBackoffSeconds,
		BatchSize:            100,
	}
}
