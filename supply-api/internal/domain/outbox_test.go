package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== P0-06 Outbox模式测试 ====================

// mockOutboxEventStore Mock Outbox事件存储
type mockOutboxEventStore struct {
	events     map[string]*OutboxEvent
	processed  []*OutboxEvent
	failed     []*OutboxEvent
	deadLetter []*OutboxEvent
}

func newMockOutboxEventStore() *mockOutboxEventStore {
	return &mockOutboxEventStore{
		events: make(map[string]*OutboxEvent),
	}
}

func (m *mockOutboxEventStore) FetchAndLock(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	var result []*OutboxEvent
	for _, e := range m.events {
		if e.Status == OutboxStatusPending || e.Status == OutboxStatusFailed {
			if e.NextRetryAt == nil || e.NextRetryAt.Before(time.Now()) {
				e.Status = OutboxStatusProcessing
				result = append(result, e)
				if len(result) >= limit {
					break
				}
			}
		}
	}
	return result, nil
}

func (m *mockOutboxEventStore) MarkCompleted(ctx context.Context, eventID string) error {
	if e, ok := m.events[eventID]; ok {
		e.Status = OutboxStatusCompleted
		now := time.Now()
		e.ProcessedAt = &now
		m.processed = append(m.processed, e)
	}
	return nil
}

func (m *mockOutboxEventStore) MarkFailed(ctx context.Context, eventID string, errorMsg string) error {
	if e, ok := m.events[eventID]; ok {
		e.Status = OutboxStatusFailed
		e.ErrorMessage = errorMsg
		backoff := CalculateOutboxBackoff(e.RetryCount, e.MaxRetries)
		nextRetry := time.Now().Add(time.Duration(backoff) * time.Second)
		e.NextRetryAt = &nextRetry
		m.failed = append(m.failed, e)
	}
	return nil
}

func (m *mockOutboxEventStore) MoveToDeadLetter(ctx context.Context, event *OutboxEvent, errorMsg string) error {
	event.Status = OutboxStatusDeadLetter
	event.DeadLetterReason = errorMsg
	m.deadLetter = append(m.deadLetter, event)
	return nil
}

// mockMessageBroker Mock消息代理
type mockMessageBroker struct {
	published []*OutboxEvent
	shouldFail bool
	failError error
}

func newMockMessageBroker() *mockMessageBroker {
	return &mockMessageBroker{
		published: make([]*OutboxEvent, 0),
	}
}

func (m *mockMessageBroker) Publish(ctx context.Context, event *OutboxEvent) error {
	if m.shouldFail {
		return m.failError
	}
	m.published = append(m.published, event)
	return nil
}

// mockOutboxStats Mock统计
type mockOutboxStats struct {
	successCount int
	failureCount int
	retryCount   int
	dlqCount    int
}

func (m *mockOutboxStats) RecordOutboxSuccess(eventType string) {
	m.successCount++
}

func (m *mockOutboxStats) RecordOutboxFailure(reason string) {
	m.failureCount++
}

func (m *mockOutboxStats) RecordOutboxRetry(eventType string) {
	m.retryCount++
}

func (m *mockOutboxStats) RecordOutboxDLQ(eventType string) {
	m.dlqCount++
}

// TestP006_OutboxEventPublishing 验证Outbox事件发布
func TestP006_OutboxEventPublishing(t *testing.T) {
	store := newMockOutboxEventStore()
	broker := newMockMessageBroker()
	stats := &mockOutboxStats{}

	processor := &OutboxProcessor{
		eventStore:   store,
		messageBroker: broker,
		stats:        stats,
	}

	// 添加测试事件
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	event := &OutboxEvent{
		EventID:       "evt_123",
		AggregateType: "supply_account",
		AggregateID:   "acc_456",
		EventType:     "created",
		Payload:       payload,
		Status:        OutboxStatusPending,
		MaxRetries:    5,
		RetryCount:    0,
	}
	store.events[event.EventID] = event

	// 处理
	err := processor.ProcessOutbox(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证事件已发布
	if len(broker.published) != 1 {
		t.Errorf("expected 1 published event, got %d", len(broker.published))
	}

	// 验证统计
	if stats.successCount != 1 {
		t.Errorf("expected 1 success, got %d", stats.successCount)
	}
}

// TestP006_OutboxRetryOnFailure 验证失败重试
func TestP006_OutboxRetryOnFailure(t *testing.T) {
	store := newMockOutboxEventStore()
	broker := newMockMessageBroker()
	stats := &mockOutboxStats{}

	processor := &OutboxProcessor{
		eventStore:   store,
		messageBroker: broker,
		stats:        stats,
	}

	// 模拟发布失败
	broker.shouldFail = true
	broker.failError = errors.New("connection refused")

	// 添加测试事件
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	event := &OutboxEvent{
		EventID:       "evt_123",
		AggregateType: "supply_account",
		AggregateID:   "acc_456",
		EventType:     "created",
		Payload:       payload,
		Status:        OutboxStatusPending,
		MaxRetries:    5,
		RetryCount:    0,
	}
	store.events[event.EventID] = event

	// 处理
	err := processor.ProcessOutbox(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证统计
	if stats.retryCount != 1 {
		t.Errorf("expected 1 retry, got %d", stats.retryCount)
	}

	// 验证失败记录
	if len(store.failed) != 1 {
		t.Errorf("expected 1 failed event, got %d", len(store.failed))
	}
}

// TestP006_MoveToDeadLetter 验证超过最大重试后移入死信队列
func TestP006_MoveToDeadLetter(t *testing.T) {
	store := newMockOutboxEventStore()
	broker := newMockMessageBroker()
	stats := &mockOutboxStats{}

	processor := &OutboxProcessor{
		eventStore:   store,
		messageBroker: broker,
		stats:        stats,
	}

	// 模拟持续失败
	broker.shouldFail = true
	broker.failError = errors.New("persistent failure")

	// 添加已重试4次的事件（第5次失败后应移入DLQ）
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	event := &OutboxEvent{
		EventID:       "evt_dlq_test",
		AggregateType: "supply_account",
		AggregateID:   "acc_456",
		EventType:     "created",
		Payload:       payload,
		Status:        OutboxStatusPending,
		MaxRetries:    5,
		RetryCount:    4, // 第5次重试后达到上限
	}
	store.events[event.EventID] = event

	// 处理
	err := processor.ProcessOutbox(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证DLQ统计
	if stats.dlqCount != 1 {
		t.Errorf("expected 1 DLQ, got %d", stats.dlqCount)
	}

	// 验证死信记录
	if len(store.deadLetter) != 1 {
		t.Errorf("expected 1 dead letter event, got %d", len(store.deadLetter))
	}
}

// TestP006_ExponentialBackoff 验证指数退避计算
func TestP006_ExponentialBackoff(t *testing.T) {
	tests := []struct {
		retryCount int
		maxRetries int
		expectedMin int
		expectedMax int
	}{
		{1, 5, 1, 2},   // 第1次重试: 1-2秒
		{2, 5, 2, 4},   // 第2次重试: 2-4秒
		{3, 5, 4, 8},   // 第3次重试: 4-8秒
		{4, 5, 8, 16},  // 第4次重试: 8-16秒
		{5, 5, 16, 32}, // 第5次重试: 16-32秒（接近上限）
	}

	for _, tt := range tests {
		backoff := CalculateOutboxBackoff(tt.retryCount, tt.maxRetries)
		if backoff < tt.expectedMin || backoff > tt.expectedMax {
			t.Errorf("retry %d: expected backoff %d-%d, got %d",
				tt.retryCount, tt.expectedMin, tt.expectedMax, backoff)
		}
	}
}

// TestP006_MaxBackoffCap 验证退避时间上限
func TestP006_MaxBackoffCap(t *testing.T) {
	// 即使重试很多次，退避时间也不应超过60秒
	backoff := CalculateOutboxBackoff(100, 100)
	if backoff > DefaultMaxBackoffSeconds {
		t.Errorf("backoff should be capped at %d, got %d", DefaultMaxBackoffSeconds, backoff)
	}
}

// TestP006_Summary 测试总结
func TestP006_Summary(t *testing.T) {
	t.Log("=== P0-06 Outbox模式测试总结 ===")
	t.Log("问题: Outbox事件 至少一次投递 未定义重试策略和DLQ处理")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - Outbox事件表结构定义")
	t.Log("  - 死信队列表结构定义")
	t.Log("  - 重试策略: 指数退避 (1s, 2s, 4s, 8s, 16s)")
	t.Log("  - 最大重试次数: 5次")
	t.Log("  - 超过最大重试后移入DLQ")
	t.Log("")
	t.Log("SQL脚本: sql/postgresql/outbox_pattern_v1.sql")
}

// TestDefaultOutboxProcessorConfig 测试默认配置
func TestDefaultOutboxProcessorConfig(t *testing.T) {
	config := DefaultOutboxProcessorConfig()

	assert.NotNil(t, config)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultInitialBackoffSeconds, config.InitialBackoffSeconds)
	assert.Equal(t, DefaultMaxBackoffSeconds, config.MaxBackoffSeconds)
	assert.Equal(t, 100, config.BatchSize)
}

// TestOutboxConstants 测试outbox常量
func TestOutboxConstants(t *testing.T) {
	assert.Equal(t, 5, DefaultMaxRetries)
	assert.Equal(t, 1, DefaultInitialBackoffSeconds)
	assert.Equal(t, 60, DefaultMaxBackoffSeconds)
}

// TestOutboxProcessorConfig 处理器配置测试
func TestOutboxProcessorConfig(t *testing.T) {
	config := &OutboxProcessorConfig{
		MaxRetries:           10,
		InitialBackoffSeconds: 2,
		MaxBackoffSeconds:    120,
		BatchSize:            50,
	}

	assert.Equal(t, 10, config.MaxRetries)
	assert.Equal(t, 2, config.InitialBackoffSeconds)
	assert.Equal(t, 120, config.MaxBackoffSeconds)
	assert.Equal(t, 50, config.BatchSize)
}

// TestOutboxEventStruct 测试OutboxEvent结构体
func TestOutboxEventStruct(t *testing.T) {
	event := &OutboxEvent{
		ID:              1,
		AggregateType:   "test-aggregate",
		AggregateID:    "123",
		EventType:       "TestEvent",
		EventID:        "evt-001",
		Payload:        json.RawMessage(`{"key":"value"}`),
		Status:         OutboxStatusPending,
		RetryCount:     0,
		MaxRetries:     5,
		CreatedAt:      time.Now(),
		Version:        1,
	}

	assert.Equal(t, int64(1), event.ID)
	assert.Equal(t, "test-aggregate", event.AggregateType)
	assert.Equal(t, "123", event.AggregateID)
	assert.Equal(t, "TestEvent", event.EventType)
	assert.Equal(t, "evt-001", event.EventID)
	assert.Equal(t, OutboxStatusPending, event.Status)
	assert.Equal(t, 0, event.RetryCount)
	assert.Equal(t, 5, event.MaxRetries)
}

// TestOutboxDeadLetterStruct 测试OutboxDeadLetter结构体
func TestOutboxDeadLetterStruct(t *testing.T) {
	now := time.Now()
	dl := &OutboxDeadLetter{
		ID:                   1,
		OriginalEventID:     "evt-001",
		OriginalAggregateType: "test-aggregate",
		OriginalAggregateID:  "123",
		EventType:           "TestEvent",
		Payload:             json.RawMessage(`{"key":"value"}`),
		ErrorMessage:        "max retries exceeded",
		RetryCount:          5,
		FirstFailedAt:       now,
		DeadLetterAt:        now,
		Handled:             false,
		CreatedAt:           now,
	}

	assert.Equal(t, int64(1), dl.ID)
	assert.Equal(t, "evt-001", dl.OriginalEventID)
	assert.Equal(t, "test-aggregate", dl.OriginalAggregateType)
	assert.Equal(t, "123", dl.OriginalAggregateID)
	assert.Equal(t, "TestEvent", dl.EventType)
	assert.Equal(t, "max retries exceeded", dl.ErrorMessage)
	assert.Equal(t, 5, dl.RetryCount)
	assert.False(t, dl.Handled)
}
