package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"
)

// TestBatchBuffer_BatchSize 测试50条/批刷新
func TestBatchBuffer_BatchSize(t *testing.T) {
	const batchSize = 50

	buffer := NewBatchBuffer(batchSize, 100*time.Millisecond) // 100ms超时防止测试卡住
	ctx := context.Background()

	err := buffer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer buffer.Close()

	// 收集器：接收批量事件
	var receivedBatches [][]*model.AuditEvent
	var mu sync.Mutex

	buffer.SetFlushHandler(func(events []*model.AuditEvent) error {
		mu.Lock()
		receivedBatches = append(receivedBatches, events)
		mu.Unlock()
		return nil
	})

	// 添加50条事件，应该触发一次批量刷新
	for i := 0; i < batchSize; i++ {
		event := &model.AuditEvent{
			EventID:   "batch-test-001",
			EventName: "TEST-EVENT",
		}
		if err := buffer.Add(event); err != nil {
			t.Errorf("Add failed: %v", err)
		}
	}

	// 等待刷新完成
	time.Sleep(50 * time.Millisecond)

	// 验证：应该收到恰好一个批次
	mu.Lock()
	if len(receivedBatches) != 1 {
		t.Errorf("expected 1 batch, got %d", len(receivedBatches))
	}
	if len(receivedBatches) > 0 && len(receivedBatches[0]) != batchSize {
		t.Errorf("expected batch size %d, got %d", batchSize, len(receivedBatches[0]))
	}
	mu.Unlock()
}

// TestBatchBuffer_TimeoutFlush 测试5ms超时刷新
func TestBatchBuffer_TimeoutFlush(t *testing.T) {
	const batchSize = 100 // 大于我们添加的数量
	const flushInterval = 5 * time.Millisecond

	buffer := NewBatchBuffer(batchSize, flushInterval)
	ctx := context.Background()

	err := buffer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer buffer.Close()

	// 收集器
	var receivedBatches [][]*model.AuditEvent
	var mu sync.Mutex

	buffer.SetFlushHandler(func(events []*model.AuditEvent) error {
		mu.Lock()
		receivedBatches = append(receivedBatches, events)
		mu.Unlock()
		return nil
	})

	// 只添加3条事件，不满50条
	for i := 0; i < 3; i++ {
		event := &model.AuditEvent{
			EventID:   "batch-test-002",
			EventName: "TEST-TIMEOUT",
		}
		if err := buffer.Add(event); err != nil {
			t.Errorf("Add failed: %v", err)
		}
	}

	// 等待5ms超时刷新
	time.Sleep(20 * time.Millisecond)

	// 验证：应该收到一个批次，包含3条事件
	mu.Lock()
	defer mu.Unlock()
	if len(receivedBatches) != 1 {
		t.Errorf("expected 1 batch (timeout flush), got %d", len(receivedBatches))
	}
	if len(receivedBatches) > 0 && len(receivedBatches[0]) != 3 {
		t.Errorf("expected 3 events in batch, got %d", len(receivedBatches[0]))
	}
}

// TestBatchBuffer_ConcurrentAccess 测试并发安全性
func TestBatchBuffer_ConcurrentAccess(t *testing.T) {
	const batchSize = 50
	const numGoroutines = 10
	const eventsPerGoroutine = 100

	buffer := NewBatchBuffer(batchSize, 10*time.Millisecond)
	ctx := context.Background()

	err := buffer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer buffer.Close()

	var totalReceived int
	var mu sync.Mutex

	buffer.SetFlushHandler(func(events []*model.AuditEvent) error {
		mu.Lock()
		totalReceived += len(events)
		mu.Unlock()
		return nil
	})

	// 并发添加事件
	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				event := &model.AuditEvent{
					EventID:   "batch-test-concurrent",
					EventName: "TEST-CONCURRENT",
				}
				if err := buffer.Add(event); err != nil {
					t.Errorf("Add failed: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond) // 等待所有刷新完成

	mu.Lock()
	defer mu.Unlock()
	expectedTotal := numGoroutines * eventsPerGoroutine
	if totalReceived != expectedTotal {
		t.Errorf("expected %d total events, got %d", expectedTotal, totalReceived)
	}
}

// TestBatchBuffer_Close 测试关闭
func TestBatchBuffer_Close(t *testing.T) {
	buffer := NewBatchBuffer(50, 10*time.Millisecond)
	ctx := context.Background()

	err := buffer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 添加一些事件
	for i := 0; i < 5; i++ {
		event := &model.AuditEvent{
			EventID:   "batch-test-close",
			EventName: "TEST-CLOSE",
		}
		if err := buffer.Add(event); err != nil {
			t.Errorf("Add failed: %v", err)
		}
	}

	// 关闭缓冲区
	err = buffer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 关闭后添加应该失败
	event := &model.AuditEvent{
		EventID:   "batch-test-after-close",
		EventName: "TEST-AFTER-CLOSE",
	}
	if err := buffer.Add(event); err == nil {
		t.Errorf("Add after Close should fail")
	}
}

// TestBatchBuffer_FlushNow 测试手动刷新
func TestBatchBuffer_FlushNow(t *testing.T) {
	const batchSize = 100 // 足够大，不会自动触发

	buffer := NewBatchBuffer(batchSize, 100*time.Millisecond) // 100ms才自动刷新
	ctx := context.Background()

	err := buffer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer buffer.Close()

	var receivedBatches [][]*model.AuditEvent
	var mu sync.Mutex

	buffer.SetFlushHandler(func(events []*model.AuditEvent) error {
		mu.Lock()
		receivedBatches = append(receivedBatches, events)
		mu.Unlock()
		return nil
	})

	// 添加少量事件
	for i := 0; i < 3; i++ {
		event := &model.AuditEvent{
			EventID:   "batch-test-manual",
			EventName: "TEST-MANUAL",
		}
		if err := buffer.Add(event); err != nil {
			t.Errorf("Add failed: %v", err)
		}
	}

	// 立即手动刷新
	err = buffer.FlushNow()
	if err != nil {
		t.Errorf("FlushNow failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(receivedBatches) != 1 {
		t.Errorf("expected 1 batch after FlushNow, got %d", len(receivedBatches))
	}
}

func TestBatchBuffer_FlushHandlerFailureIsReported(t *testing.T) {
	buffer := NewBatchBuffer(10, 100*time.Millisecond)
	ctx := context.Background()

	err := buffer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer buffer.Close()

	expectedErr := errors.New("flush failed")
	var hookCalls int

	buffer.SetFlushHandler(func(events []*model.AuditEvent) error {
		return expectedErr
	})
	buffer.SetFlushErrorHandler(func(err error, events []*model.AuditEvent) {
		hookCalls++
	})

	if err := buffer.Add(&model.AuditEvent{EventID: "evt-flush-fail", EventName: "TEST-FLUSH-FAIL"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err = buffer.FlushNow()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected FlushNow error %v, got %v", expectedErr, err)
	}
	if !errors.Is(buffer.LastFlushError(), expectedErr) {
		t.Fatalf("expected last flush error %v, got %v", expectedErr, buffer.LastFlushError())
	}
	if buffer.FlushErrorCount() != 1 {
		t.Fatalf("expected flush error count 1, got %d", buffer.FlushErrorCount())
	}
	if hookCalls != 1 {
		t.Fatalf("expected flush error hook to be called once, got %d", hookCalls)
	}
}
