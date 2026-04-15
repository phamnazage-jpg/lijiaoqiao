# Supply API Background Helper Split Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `supply-api/internal/app/background.go` 中串行堆叠的后台启动逻辑拆成更小的 helper，降低单函数复杂度，同时保持现有启动语义不变。

**Architecture:** 保留 `StartBackgroundWorkers` 和 `startBackgroundWorkersWithFactory` 作为编排入口，把主动吊销订阅、Outbox 启动、分区维护和补偿 worker 启动拆到独立 helper。先补 helper 级失败测试锁住关键行为，再在现有高层测试基础上做回归验证。

**Tech Stack:** Go, Go test

---

### Task 1: 提取 Outbox 启动 helper

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestStartOutboxProcessor_ProdRequiresBroker(t *testing.T) {
	err := startOutboxProcessor(context.Background(), runtime, factory)
	if err == nil {
		t.Fatal("expected missing broker to fail in prod")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestStartOutboxProcessor_ProdRequiresBroker' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func startOutboxProcessor(...) error { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestStartOutboxProcessor_ProdRequiresBroker' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract background outbox startup"
```

### Task 2: 提取补偿与订阅启动 helper

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestStartCompensationWorker_UsesConfiguredInterval(t *testing.T) {
	var gotInterval time.Duration
	startCompensationWorker(context.Background(), runtime, factory)
	if gotInterval != 5*time.Minute {
		t.Fatalf("unexpected compensation interval: %s", gotInterval)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestStartCompensationWorker_UsesConfiguredInterval' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func startRevocationSubscriber(...) { ... }
func startCompensationWorker(...) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestStartCompensationWorker_UsesConfiguredInterval' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract background startup helpers"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Verify: `supply-api/internal/app/runtime_test.go`
- Verify: `supply-api/internal/app/runtime.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(Runtime_StartBackgroundWorkers_.*|StartOutboxProcessor_ProdRequiresBroker|StartCompensationWorker_UsesConfiguredInterval)' -v`
Expected: PASS

**Step 2: Run package regression**

Run: `cd "supply-api" && go test ./internal/app ./cmd/supply-api ./internal/httpapi`
Expected: PASS

**Step 3: Run repo exit verification**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: PASS

**Step 4: Check formatting**

Run: `git diff --check`
Expected: no output

**Step 5: Commit**

```bash
git add docs/plans/2026-04-15-supply-api-background-helper-split-plan.md supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): split background startup helpers"
```
