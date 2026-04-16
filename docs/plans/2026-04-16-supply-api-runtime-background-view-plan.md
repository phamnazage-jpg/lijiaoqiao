# Supply API Runtime Background View Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `Runtime` 暴露给后台 worker 启动层的字段收成更窄的 `runtimeBackgroundView`，与现有 `runtimeHTTPView` 对称，进一步缩小启动层直接读取 `Runtime` 的表面积。

**Architecture:** 保留 `Runtime.StartBackgroundWorkers` 和 `startBackgroundWorkersWithFactory` 的对外/测试入口，新增 `buildRuntimeBackgroundView` 负责从 runtime 提取后台启动所需字段并补默认 tuning。后台启动 helper 统一改为读取 `runtimeBackgroundView`，不再直接读完整 `Runtime`。

**Tech Stack:** Go, Go test

---

### Task 1: 提取 runtimeBackgroundView

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntimeBackgroundView_RequiresRuntime(t *testing.T) {
	_, err := buildRuntimeBackgroundView(nil)
	if err == nil {
		t.Fatal("expected nil runtime to fail")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntimeBackgroundView_(RequiresRuntime|MapsBackgroundFields)' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type runtimeBackgroundView struct { ... }
func buildRuntimeBackgroundView(runtime *Runtime) (runtimeBackgroundView, error) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntimeBackgroundView_(RequiresRuntime|MapsBackgroundFields)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/background.go supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime background view"
```

### Task 2: 改造后台 helper 使用 background view

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestStartOutboxProcessor_ProdRequiresBroker(t *testing.T) {
	err := startOutboxProcessor(context.Background(), runtimeBackgroundView{...}, backgroundFactory{...})
	if err == nil {
		t.Fatal("expected missing outbox broker to fail in prod")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(StartOutboxProcessor_ProdRequiresBroker|Runtime_StartBackgroundWorkers_UsesDefaultCompensationInterval)' -v`
Expected: FAIL，因为 helper 签名/实现尚未切到 view

**Step 3: Write minimal implementation**

```go
func startBackgroundWorkersWithViewAndFactory(...) error { ... }
func startRevocationSubscriber(ctx context.Context, view runtimeBackgroundView) { ... }
func startOutboxProcessor(ctx context.Context, view runtimeBackgroundView, factory backgroundFactory) error { ... }
func startPartitionMaintenanceWorker(..., view runtimeBackgroundView, ...) { ... }
func startCompensationWorker(ctx context.Context, view runtimeBackgroundView, factory backgroundFactory) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(StartOutboxProcessor_ProdRequiresBroker|Runtime_StartBackgroundWorkers_UsesDefaultCompensationInterval)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): route background workers through view"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/runtime_test.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntimeBackgroundView_(RequiresRuntime|MapsBackgroundFields)|Runtime_StartBackgroundWorkers_.*|StartOutboxProcessor_ProdRequiresBroker|StartCompensationWorker_UsesConfiguredInterval|BuildRuntime_.*)' -v`
Expected: PASS

**Step 2: Run package regression**

Run: `cd "supply-api" && go test ./internal/app ./cmd/supply-api ./internal/httpapi`
Expected: PASS

**Step 3: Run e2e tests**

Run: `cd "supply-api" && go test -tags=e2e ./e2e`
Expected: PASS

**Step 4: Run repo exit verification**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: PASS

**Step 5: Check formatting**

Run: `git diff --check`
Expected: no output

**Step 6: Commit**

```bash
git add docs/plans/2026-04-16-supply-api-runtime-background-view-plan.md supply-api/internal/app/background.go supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): narrow runtime background surface"
```
