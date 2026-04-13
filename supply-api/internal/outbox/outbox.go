package outbox

import (
	"context"
	"fmt"
	"math"
	"time"

	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/repository"
)

// OutboxProcessorRunner Outbox处理器运行器
type OutboxProcessorRunner struct {
	repo      outboxRepository
	msgBroker messaging.MessageBroker
	stats     messaging.OutboxStats
	stopCh    chan struct{}
	batchSize int
	interval  time.Duration
}

type outboxRepository interface {
	FetchAndLock(ctx context.Context, limit int) ([]*repository.OutboxEvent, error)
	MarkCompleted(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID string, errorMsg string, nextRetryAt *time.Time) error
	MoveToDeadLetter(ctx context.Context, event *repository.OutboxEvent, errorMsg string) error
}

// NewOutboxProcessorRunner 创建Outbox处理器运行器
func NewOutboxProcessorRunner(
	repo outboxRepository,
	msgBroker messaging.MessageBroker,
	stats messaging.OutboxStats,
) *OutboxProcessorRunner {
	return &OutboxProcessorRunner{
		repo:      repo,
		msgBroker: msgBroker,
		stats:     stats,
		stopCh:    make(chan struct{}),
		batchSize: 100,
		interval:  1 * time.Second,
	}
}

// Start 启动Outbox处理器
func (r *OutboxProcessorRunner) Start(ctx context.Context) {
	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("OutboxProcessor started", nil)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
			logger.Info("OutboxProcessor stopping due to context cancellation", nil)
			return
		case <-r.stopCh:
			logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
			logger.Info("OutboxProcessor stopping", nil)
			return
		case <-ticker.C:
			if err := r.process(ctx); err != nil {
				logger := logging.NewLogger("supply-api", logging.LogLevelError)
				logger.Error("OutboxProcessor error", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// Stop 停止Outbox处理器
func (r *OutboxProcessorRunner) Stop() {
	close(r.stopCh)
}

// process 处理一批Outbox事件
func (r *OutboxProcessorRunner) process(ctx context.Context) error {
	if r.msgBroker == nil {
		return fmt.Errorf("outbox message broker is unavailable")
	}

	// 获取待处理事件
	events, err := r.repo.FetchAndLock(ctx, r.batchSize)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		// 转换为domain.OutboxEvent
		domainEvent := &domain.OutboxEvent{
			ID:            event.ID,
			AggregateType: event.AggregateType,
			AggregateID:   event.AggregateID,
			EventType:     event.EventType,
			EventID:       event.EventID,
			Payload:       event.Payload,
			Status:        string(event.Status),
			RetryCount:    event.RetryCount,
			MaxRetries:    event.MaxRetries,
			ErrorMessage:  event.ErrorMessage,
			Version:       event.Version,
		}

		// 发布消息
		if err := r.msgBroker.Publish(ctx, event); err != nil {
			r.handleFailure(ctx, domainEvent, err)
			continue
		}

		// 标记完成
		if err := r.repo.MarkCompleted(ctx, event.EventID); err != nil {
			r.stats.RecordOutboxFailure("mark_completed_failed")
			continue
		}

		r.stats.RecordOutboxSuccess(event.EventType)
	}

	return nil
}

// handleFailure 处理失败事件
func (r *OutboxProcessorRunner) handleFailure(ctx context.Context, event *domain.OutboxEvent, publishErr error) {
	event.RetryCount++

	if event.RetryCount >= event.MaxRetries {
		// 移入死信队列
		domainEvent := &repository.OutboxEvent{
			ID:         event.ID,
			EventID:    event.EventID,
			Payload:    event.Payload,
			RetryCount: event.RetryCount,
		}
		if err := r.repo.MoveToDeadLetter(ctx, domainEvent, publishErr.Error()); err != nil {
			r.stats.RecordOutboxFailure("move_to_dlq_failed")
		} else {
			r.stats.RecordOutboxDLQ(event.EventType)
		}
	} else {
		// 计算下次重试时间（指数退避）
		backoffSeconds := CalculateOutboxBackoff(event.RetryCount, event.MaxRetries)
		nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

		if err := r.repo.MarkFailed(ctx, event.EventID, publishErr.Error(), &nextRetry); err != nil {
			r.stats.RecordOutboxFailure("mark_failed_failed")
		} else {
			r.stats.RecordOutboxRetry(event.EventType)
		}
	}
}

// CalculateOutboxBackoff 计算指数退避时间
func CalculateOutboxBackoff(retryCount, maxRetries int) int {
	initialBackoff := 1.0
	maxBackoff := 60.0
	backoff := initialBackoff * math.Pow(2, float64(retryCount-1))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return int(backoff)
}
