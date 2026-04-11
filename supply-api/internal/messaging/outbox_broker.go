package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"lijiaoqiao/supply-api/internal/repository"
)

// OutboxMessageBroker Outbox消息代理（使用Redis Streams）
type OutboxMessageBroker struct {
	redis       *redis.Client
	streamName  string
	consumerGroup string
}

// NewOutboxMessageBroker 创建Outbox消息代理
func NewOutboxMessageBroker(redisClient *redis.Client, streamName string, consumerGroup string) *OutboxMessageBroker {
	return &OutboxMessageBroker{
		redis:        redisClient,
		streamName:   streamName,
		consumerGroup: consumerGroup,
	}
}

// Publish 发布消息到Redis Stream
func (b *OutboxMessageBroker) Publish(ctx context.Context, event *repository.OutboxEvent) error {
	// 构造消息
	msg := map[string]interface{}{
		"event_id":         event.EventID,
		"aggregate_type":   event.AggregateType,
		"aggregate_id":     event.AggregateID,
		"event_type":       event.EventType,
		"payload":          string(event.Payload),
		"published_at":     time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 发布到Stream
	_, err = b.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: b.streamName,
		ID:     "*", // 自动生成ID
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish to stream: %w", err)
	}

	return nil
}

// EnsureConsumerGroup 确保消费者组存在
func (b *OutboxMessageBroker) EnsureConsumerGroup(ctx context.Context) error {
	err := b.redis.XGroupCreateMkStream(ctx, b.streamName, b.consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}

// MessageBroker 消息代理接口
type MessageBroker interface {
	Publish(ctx context.Context, event *repository.OutboxEvent) error
}

// OutboxStats Outbox统计接口
type OutboxStats interface {
	RecordOutboxSuccess(eventType string)
	RecordOutboxFailure(reason string)
	RecordOutboxRetry(eventType string)
	RecordOutboxDLQ(eventType string)
}

// NoOpOutboxStats 无操作统计（用于默认实现）
type NoOpOutboxStats struct{}

func (s *NoOpOutboxStats) RecordOutboxSuccess(eventType string)        {}
func (s *NoOpOutboxStats) RecordOutboxFailure(reason string)          {}
func (s *NoOpOutboxStats) RecordOutboxRetry(eventType string)         {}
func (s *NoOpOutboxStats) RecordOutboxDLQ(eventType string)            {}
