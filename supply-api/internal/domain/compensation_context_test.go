package domain

import (
	"context"
	"testing"
	"time"
)

// TestCompensationProcessor_StartBackgroundWorker_Cancel
// TDD: 验证 StopBackgroundWorker 能够正确取消后台 worker
func TestCompensationProcessor_StartBackgroundWorker_Cancel(t *testing.T) {
	store := &mockCompensationStore{}
	executor := &mockOperationExecutor{}
	stats := &mockCompensationStats{}
	processor := NewCompensationProcessor(store, executor, stats)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 worker
	workerCtx := processor.StartBackgroundWorker(ctx, 100*time.Millisecond)

	// 验证 worker context 不是已经取消的
	select {
	case <-workerCtx.Done():
		t.Fatal("workerCtx should not be cancelled immediately")
	default:
	}

	// 停止 worker
	processor.StopBackgroundWorker()

	// 验证 worker context 已被取消
	select {
	case <-workerCtx.Done():
		// 预期：context 应该在 Stop 后被取消
	default:
		t.Fatal("workerCtx should be cancelled after StopBackgroundWorker")
	}
}

// TestCompensationProcessor_WorkerStopsOnParentContext
// TDD: 验证当父 context 取消时，worker 应该停止
func TestCompensationProcessor_WorkerStopsOnParentContext(t *testing.T) {
	store := &mockCompensationStore{}
	executor := &mockOperationExecutor{}
	stats := &mockCompensationStats{}
	processor := NewCompensationProcessor(store, executor, stats)

	ctx, cancel := context.WithCancel(context.Background())

	// 启动 worker
	workerCtx := processor.StartBackgroundWorker(ctx, 100*time.Millisecond)

	// 取消父 context
	cancel()

	// 验证 worker context 已被取消
	select {
	case <-workerCtx.Done():
		// 预期：父 context 取消后 worker 应该停止
	default:
		t.Fatal("workerCtx should be cancelled after parent context is cancelled")
	}
}
