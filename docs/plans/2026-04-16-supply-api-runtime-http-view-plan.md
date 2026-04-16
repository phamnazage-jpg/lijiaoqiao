# Supply API Runtime HTTP View Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `Runtime` 暴露给 HTTP 启动层的字段再收成更窄的 view/struct，进一步缩小 `Runtime.BuildServer` 直接读取的表面积。

**Architecture:** 在 `supply-api/internal/app/runtime.go` 内新增 `runtimeHTTPView`，只保留 HTTP 启动所需字段和健康检查，然后再由独立 adapter 把 view 映射成 `BuildServerOptions`。`Runtime.BuildServer` 最终只做 `buildRuntimeHTTPView -> adaptRuntimeHTTPViewToBuildServerOptions -> BuildServer` 三步串联。

**Tech Stack:** Go, Go test

---

### Task 1: 提取 runtime HTTP view

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntimeHTTPView_MapsHTTPFields(t *testing.T) {
	view, err := buildRuntimeHTTPView(runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.supplyAPI != runtime.supplyAPI {
		t.Fatal("expected supply api to be preserved")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntimeHTTPView_(RequiresRuntime|MapsHTTPFields)' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type runtimeHTTPView struct { ... }
func buildRuntimeHTTPView(runtime *Runtime) (runtimeHTTPView, error) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntimeHTTPView_(RequiresRuntime|MapsHTTPFields)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime http view"
```

### Task 2: 提取 view 到 BuildServerOptions adapter

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestAdaptRuntimeHTTPViewToBuildServerOptions_MapsHealthChecks(t *testing.T) {
	opts := adaptRuntimeHTTPViewToBuildServerOptions(view)
	if opts.DBHealthCheck == nil {
		t.Fatal("expected db health check")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestAdaptRuntimeHTTPViewToBuildServerOptions_MapsHealthChecks' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func adaptRuntimeHTTPViewToBuildServerOptions(view runtimeHTTPView) BuildServerOptions { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestAdaptRuntimeHTTPViewToBuildServerOptions_MapsHealthChecks' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime http view adapter"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/runtime_test.go`
- Verify: `supply-api/internal/app/bootstrap.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntimeHTTPView_(RequiresRuntime|MapsHTTPFields)|AdaptRuntimeHTTPViewToBuildServerOptions_MapsHealthChecks|ResolveRuntimeHealthChecks_(OmitsUnavailableDependencies|ExposesAvailableDependencies)|AdaptRuntimeToBuildServerOptions_(RequiresRuntime|MapsRuntimeFields)|BuildServer_.*|BuildRuntime_.*)' -v`
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
git add docs/plans/2026-04-16-supply-api-runtime-http-view-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): narrow runtime http surface"
```
