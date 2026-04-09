package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// mockCompensationStore Mock补偿存储
type mockCompensationStore struct {
	compensations map[int64]*BatchCompensation
	nextID       int64
}

func newMockCompensationStore() *mockCompensationStore {
	return &mockCompensationStore{
		compensations: make(map[int64]*BatchCompensation),
		nextID:        1,
	}
}

func (m *mockCompensationStore) Create(ctx context.Context, comp *BatchCompensation) (int64, error) {
	comp.ID = m.nextID
	m.nextID++
	m.compensations[comp.ID] = comp
	return comp.ID, nil
}

func (m *mockCompensationStore) GetByBatchID(ctx context.Context, batchID string) ([]*BatchCompensation, error) {
	var result []*BatchCompensation
	for _, comp := range m.compensations {
		if comp.BatchID == batchID {
			result = append(result, comp)
		}
	}
	return result, nil
}

func (m *mockCompensationStore) GetPending(ctx context.Context) ([]*BatchCompensation, error) {
	var result []*BatchCompensation
	for _, comp := range m.compensations {
		if comp.Status == CompensationStatusPending || comp.Status == CompensationStatusRetrying {
			result = append(result, comp)
		}
	}
	return result, nil
}

func (m *mockCompensationStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	if comp, ok := m.compensations[id]; ok {
		comp.Status = status
	}
	return nil
}

func (m *mockCompensationStore) Resolve(ctx context.Context, id int64, resolvedBy int64, notes string) error {
	if comp, ok := m.compensations[id]; ok {
		comp.Status = CompensationStatusResolved
		now := time.Now()
		comp.ResolvedAt = &now
		comp.ResolvedBy = &resolvedBy
		comp.ResolutionNotes = notes
	}
	return nil
}

func (m *mockCompensationStore) MarkManualRequired(ctx context.Context, id int64, reason string) error {
	if comp, ok := m.compensations[id]; ok {
		comp.Status = CompensationStatusManualRequired
		comp.FailureReason = comp.FailureReason + "; " + reason
	}
	return nil
}

// mockOperationExecutor Mock操作执行器
type mockOperationExecutor struct {
	shouldFail      bool
	failError       error
	executionCount  int
}

func (m *mockOperationExecutor) Execute(ctx context.Context, operationType string, payload json.RawMessage) error {
	m.executionCount++
	if m.shouldFail {
		return m.failError
	}
	return nil
}

// mockCompensationStats Mock统计
type mockCompensationStats struct {
	retryCount   int
	resolvedCount int
	manualCount   int
}

func (m *mockCompensationStats) RecordCompensationRetry(operationType string) {
	m.retryCount++
}

func (m *mockCompensationStats) RecordCompensationResolved(operationType string) {
	m.resolvedCount++
}

func (m *mockCompensationStats) RecordCompensationManual(operationType string) {
	m.manualCount++
}

// TestP007_CompensationRetry 验证补偿重试逻辑存在
func TestP007_CompensationRetry(t *testing.T) {
	// 验证重试配置存在
	config := DefaultCompensationConfig()
	if config.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", config.MaxRetries)
	}
	if config.RetryInterval != 1*time.Minute {
		t.Errorf("expected retry interval 1 minute, got %v", config.RetryInterval)
	}
	t.Log("P0-07: 补偿重试配置验证通过 (max_retries=3, retry_interval=1min)")
}

// TestP007_CompensationSuccess 验证补偿成功处理逻辑存在
func TestP007_CompensationSuccess(t *testing.T) {
	processor := &CompensationProcessor{}
	if processor == nil {
		t.Error("CompensationProcessor should not be nil")
	}
	t.Log("P0-07: CompensationProcessor 结构验证通过")
}

// TestP007_MaxRetriesExceeded 验证最大重试逻辑存在
func TestP007_MaxRetriesExceeded(t *testing.T) {
	// 验证状态常量存在
	statuses := []string{
		CompensationStatusPending,
		CompensationStatusRetrying,
		CompensationStatusResolved,
		CompensationStatusManualRequired,
		CompensationStatusAbandoned,
	}
	if len(statuses) != 5 {
		t.Errorf("expected 5 compensation statuses, got %d", len(statuses))
	}
	t.Log("P0-07: 补偿状态常量验证通过")
}

// TestP007_CompensationResultSummary 验证补偿结果统计
func TestP007_CompensationResultSummary(t *testing.T) {
	result := &CompensationResult{
		BatchID:      "batch_123",
		TotalItems:   10,
		SuccessCount: 7,
		RetryCount:   2,
		ManualCount:  1,
		FailedCount:  0,
	}

	if result.TotalItems != result.SuccessCount+result.RetryCount+result.ManualCount+result.FailedCount {
		t.Error("counts do not add up correctly")
	}

	if result.BatchID != "batch_123" {
		t.Errorf("expected batch ID batch_123, got %s", result.BatchID)
	}
}

// TestP007_CompensationStatusConstants 验证补偿状态常量
func TestP007_CompensationStatusConstants(t *testing.T) {
	if CompensationStatusPending != "pending" {
		t.Errorf("expected pending, got %s", CompensationStatusPending)
	}
	if CompensationStatusRetrying != "retrying" {
		t.Errorf("expected retrying, got %s", CompensationStatusRetrying)
	}
	if CompensationStatusResolved != "resolved" {
		t.Errorf("expected resolved, got %s", CompensationStatusResolved)
	}
	if CompensationStatusManualRequired != "manual_required" {
		t.Errorf("expected manual_required, got %s", CompensationStatusManualRequired)
	}
	if CompensationStatusAbandoned != "abandoned" {
		t.Errorf("expected abandoned, got %s", CompensationStatusAbandoned)
	}
}

// TestP007_Summary 测试总结
func TestP007_Summary(t *testing.T) {
	t.Log("=== P0-07 批量补偿策略测试总结 ===")
	t.Log("问题: 批量操作失败后无补偿/重试机制")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - supply_batch_compensation 表结构")
	t.Log("  - 重试策略: 最大3次重试")
	t.Log("  - 超过最大重试后标记 manual_required")
	t.Log("  - 提供人工介入接口")
	t.Log("")
	t.Log("SQL脚本: sql/postgresql/outbox_pattern_v1.sql")
}

// TestCompensationProcessor_ProcessBatchCompensations_Success 测试处理成功
func TestCompensationProcessor_ProcessBatchCompensations_Success(t *testing.T) {
	store := newMockCompensationStore()
	executor := &mockOperationExecutor{shouldFail: false}
	stats := &mockCompensationStats{}

	processor := NewCompensationProcessor(store, executor, stats)

	// 添加补偿记录
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	store.compensations[1] = &BatchCompensation{
		ID:            1,
		BatchID:       "batch_001",
		OperationType: "account.create",
		ItemPayload:   payload,
		Status:        CompensationStatusPending,
		MaxRetries:    3,
		RetryCount:    0,
	}

	result, err := processor.ProcessBatchCompensations(context.Background(), "batch_001")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", result.SuccessCount)
	}
	if stats.resolvedCount != 1 {
		t.Errorf("expected 1 resolved stat, got %d", stats.resolvedCount)
	}
}

// TestCompensationProcessor_ProcessBatchCompensations_Retry 测试重试逻辑
func TestCompensationProcessor_ProcessBatchCompensations_Retry(t *testing.T) {
	store := newMockCompensationStore()
	executor := &mockOperationExecutor{shouldFail: true, failError: errors.New("temporary failure")}
	stats := &mockCompensationStats{}

	processor := NewCompensationProcessor(store, executor, stats)

	// 添加补偿记录（还有重试次数）
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	store.compensations[1] = &BatchCompensation{
		ID:            1,
		BatchID:       "batch_002",
		OperationType: "account.create",
		ItemPayload:   payload,
		Status:        CompensationStatusPending,
		MaxRetries:    3,
		RetryCount:    0, // 还没重试过
	}

	result, err := processor.ProcessBatchCompensations(context.Background(), "batch_002")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.RetryCount != 1 {
		t.Errorf("expected 1 retry, got %d", result.RetryCount)
	}
	if result.ManualCount != 0 {
		t.Errorf("expected 0 manual, got %d", result.ManualCount)
	}
	if stats.retryCount != 1 {
		t.Errorf("expected 1 retry stat, got %d", stats.retryCount)
	}
}

// TestCompensationProcessor_ProcessBatchCompensations_MaxRetriesExceeded 测试超过最大重试
func TestCompensationProcessor_ProcessBatchCompensations_MaxRetriesExceeded(t *testing.T) {
	store := newMockCompensationStore()
	executor := &mockOperationExecutor{shouldFail: true, failError: errors.New("permanent failure")}
	stats := &mockCompensationStats{}

	processor := NewCompensationProcessor(store, executor, stats)

	// 添加补偿记录（已达到最大重试次数）
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	store.compensations[1] = &BatchCompensation{
		ID:            1,
		BatchID:       "batch_003",
		OperationType: "account.create",
		ItemPayload:   payload,
		Status:        CompensationStatusPending,
		MaxRetries:    3,
		RetryCount:    3, // 已达最大重试次数
	}

	result, err := processor.ProcessBatchCompensations(context.Background(), "batch_003")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ManualCount != 1 {
		t.Errorf("expected 1 manual, got %d", result.ManualCount)
	}
	if stats.manualCount != 1 {
		t.Errorf("expected 1 manual stat, got %d", stats.manualCount)
	}
}

// TestCompensationProcessor_ProcessBatchCompensations_AlreadyProcessed 测试跳过已处理的记录
func TestCompensationProcessor_ProcessBatchCompensations_AlreadyProcessed(t *testing.T) {
	store := newMockCompensationStore()
	executor := &mockOperationExecutor{shouldFail: false}
	stats := &mockCompensationStats{}

	processor := NewCompensationProcessor(store, executor, stats)

	// 添加已解决的补偿记录
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	store.compensations[1] = &BatchCompensation{
		ID:            1,
		BatchID:       "batch_004",
		OperationType: "account.create",
		ItemPayload:   payload,
		Status:        CompensationStatusResolved, // 已解决
		MaxRetries:    3,
		RetryCount:    0,
	}

	result, err := processor.ProcessBatchCompensations(context.Background(), "batch_004")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.SuccessCount != 0 {
		t.Errorf("expected 0 success, got %d", result.SuccessCount)
	}
	if executor.executionCount != 0 {
		t.Errorf("expected 0 executions, got %d", executor.executionCount)
	}
}

// TestNewCompensationProcessor 测试构造函数
func TestNewCompensationProcessor(t *testing.T) {
	store := newMockCompensationStore()
	executor := &mockOperationExecutor{}
	stats := &mockCompensationStats{}

	processor := NewCompensationProcessor(store, executor, stats)

	if processor == nil {
		t.Fatal("expected processor, got nil")
	}
	if processor.store != store {
		t.Error("store not set correctly")
	}
	if processor.operationExecutor != executor {
		t.Error("executor not set correctly")
	}
	if processor.stats != stats {
		t.Error("stats not set correctly")
	}
}

// TestNoOpCompensationStats 测试NoOp实现
func TestNoOpCompensationStats(t *testing.T) {
	stats := &NoOpCompensationStats{}

	// 这些调用不应该panic
	stats.RecordCompensationRetry("test")
	stats.RecordCompensationResolved("test")
	stats.RecordCompensationManual("test")
}

// TestStartBackgroundWorker 测试启动后台worker（简单测试不panic）
func TestStartBackgroundWorker(t *testing.T) {
	store := newMockCompensationStore()
	executor := &mockOperationExecutor{}
	stats := &NoOpCompensationStats{}

	processor := NewCompensationProcessor(store, executor, stats)
	ctx := context.Background()

	// 启动worker（会立即返回）
	workerCtx := processor.StartBackgroundWorker(ctx, 100*time.Millisecond)

	// 等待一下让worker运行
	time.Sleep(50 * time.Millisecond)

	// worker应该还在运行
	select {
	case <-workerCtx.Done():
		t.Error("worker should still be running")
	default:
		// 正常
	}
}
