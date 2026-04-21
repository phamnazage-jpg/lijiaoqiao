package outbox

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/repository"
)

// mockOutboxRepo implements outboxRepository
type mockOutboxRepo struct {
	fetchAndLockCalled atomic.Int32
	eventsToReturn     int
}

func (m *mockOutboxRepo) FetchAndLock(bc context.Context, limit int) ([]*repository.OutboxEvent, error) {
	m.fetchAndLockCalled.Add(1)
	if m.eventsToReturn > 0 {
		m.eventsToReturn--
		return []*repository.OutboxEvent{{EventID: "test", EventType: "test"}}, nil
	}
	return nil, nil
}

func (m *mockOutboxRepo) MarkCompleted(ctx context.Context, eventID string) error { return nil }
func (m *mockOutboxRepo) MarkFailed(ctx context.Context, eventID, errMsg string, nextRetry *time.Time) error {
	return nil
}
func (m *mockOutboxRepo) MoveToDeadLetter(ctx context.Context, event *repository.OutboxEvent, errMsg string) error {
	return nil
}

// mockBroker implements messaging.MessageBroker
type mockBroker struct {
	publishCalled atomic.Int32
}

func (m *mockBroker) Publish(ctx context.Context, event *repository.OutboxEvent) error {
	m.publishCalled.Add(1)
	return nil
}

// mockStats implements messaging.OutboxStats
type mockStats struct{}

func (m *mockStats) RecordOutboxSuccess(eventType string) {}
func (m *mockStats) RecordOutboxFailure(reason string)    {}
func (m *mockStats) RecordOutboxRetry(eventType string)   {}
func (m *mockStats) RecordOutboxDLQ(eventType string)     {}

// P3-D-02: Stop() 等待当前批次处理完成
func TestOutboxProcessorRunner_Stop_WaitsForCurrentBatch(t *testing.T) {
	repo := &mockOutboxRepo{eventsToReturn: 1}
	broker := &mockBroker{}
	stats := &mockStats{}
	runner := NewOutboxProcessorRunner(repo, broker, stats)
	runner.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	_ = cancel // cancellation handled by runner.Stop()

	go func() {
		runner.Start(ctx)
		close(done)
	}()

	// 等待第一个 tick 开始处理
	time.Sleep(50 * time.Millisecond)
	runner.Stop()

	select {
	case <-done:
		// 正常停止
	case <-time.After(2 * time.Second):
		t.Fatal("OutboxProcessor did not stop in time (drain not working)")
	}

	if repo.fetchAndLockCalled.Load() == 0 {
		t.Error("expected at least one FetchAndLock call")
	}
}

// P3-D-02: drainDone channel 在 Start 返回后关闭
func TestOutboxProcessorRunner_DrainDoneChannel(t *testing.T) {
	repo := &mockOutboxRepo{}
	broker := &mockBroker{}
	stats := &mockStats{}
	runner := NewOutboxProcessorRunner(repo, broker, stats)
	runner.interval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go runner.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	runner.Stop()
	_ = cancel // cancellation handled by runner.Stop()

	select {
	case <-runner.drainDone:
		// drainDone 已关闭
	case <-time.After(1 * time.Second):
		t.Fatal("drainDone should be closed after Stop()")
	}
}

// P3-D-02: context cancellation 触发 drain
func TestOutboxProcessorRunner_CtxCancel_TriggersDrain(t *testing.T) {
	repo := &mockOutboxRepo{eventsToReturn: 1}
	broker := &mockBroker{}
	stats := &mockStats{}
	runner := NewOutboxProcessorRunner(repo, broker, stats)
	runner.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	_ = cancel

	go func() {
		runner.Start(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel() // 发送 context cancellation

	select {
	case <-done:
		// 正常停止
	case <-time.After(2 * time.Second):
		t.Fatal("did not stop after context cancellation")
	}
}
