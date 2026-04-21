package outbox

import (
	"context"
	"fmt"
	"time"

	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/repository"
)

// OutboxProcessorRunner Outbox处理器运行器
// P3-D-02: 增加 drainDone channel 支持优雅停止
type OutboxProcessorRunner struct {
	repo       outboxRepository
	msgBroker  messaging.MessageBroker
	stats      messaging.OutboxStats
	stopCh     chan struct{}
	drainDone  chan struct{}
	batchSize  int
	interval   time.Duration
	processing bool // 当前是否在处理中（用于 drain 等待）
}

type outboxRepository interface {
	FetchAndLock(bc context.Context, limit int) ([]*repository.OutboxEvent, error)
	MarkCompleted(bc context.Context, eventID string) error
	MarkFailed(bc context.Context, eventID string, errorMsg string, nextRetryAt *time.Time) error
	MoveToDeadLetter(bc context.Context, event *repository.OutboxEvent, errorMsg string) error
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
		drainDone: make(chan struct{}),
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
	defer close(r.drainDone) // P3-D-02: 通知所有等待方处理已完成

	for {
		select {
		case <-ctx.Done():
			logger.Info("OutboxProcessor: context cancelled, waiting for current batch to finish...", nil)
			r.waitForProcessingDone()
			logger.Info("OutboxProcessor: stopped (context cancelled)", nil)
			return
		case <-r.stopCh:
			logger.Info("OutboxProcessor: stop requested, waiting for current batch to finish...", nil)
			r.waitForProcessingDone()
			logger.Info("OutboxProcessor: stopped (stopCh)", nil)
			return
		case <-ticker.C:
			r.processing = true
			if err := r.process(ctx); err != nil {
				logger.Error("OutboxProcessor error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			r.processing = false
		}
	}
}

// waitForProcessingDone 等待当前处理批次完成（如果有）
func (r *OutboxProcessorRunner) waitForProcessingDone() {
	if r.processing {
		logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
		logger.Info("OutboxProcessor: waiting for in-flight batch to complete...", nil)
	}
	// processing 为 true 时等待一个 tick 让当前 process 完成
	// 由于 processing 在 process() 返回后才会被设为 false，
	// 这里轮询等待
	for r.processing {
		time.Sleep(10 * time.Millisecond)
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
		// 发布消息
		if err := r.msgBroker.Publish(ctx, event); err != nil {
			r.handleFailure(ctx, event, err)
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
func (r *OutboxProcessorRunner) handleFailure(bc context.Context, event *repository.OutboxEvent, publishErr error) {
	event.RetryCount++

	if event.RetryCount >= event.MaxRetries {
		// 移入死信队列
		if err := r.repo.MoveToDeadLetter(bc, event, publishErr.Error()); err != nil {
			r.stats.RecordOutboxFailure("move_to_dlq_failed")
		} else {
			r.stats.RecordOutboxDLQ(event.EventType)
		}
	} else {
		// 计算下次重试时间（指数退避）
		backoffSeconds := domain.CalculateOutboxBackoff(event.RetryCount, event.MaxRetries)
		nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

		if err := r.repo.MarkFailed(bc, event.EventID, publishErr.Error(), &nextRetry); err != nil {
			r.stats.RecordOutboxFailure("mark_failed_failed")
		} else {
			r.stats.RecordOutboxRetry(event.EventType)
		}
	}
}
