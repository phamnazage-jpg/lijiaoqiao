# Supply API Runtime Layered Surfaces Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `Runtime` 自身继续降噪为“外部资源句柄 + 业务启动视图”两层结构，进一步压低 `Runtime` 顶层字段密度。

**Architecture:** 保留 `BuildRuntime`、`Runtime.BuildServer`、`Runtime.StartBackgroundWorkers` 等对外接口不变，但把 `Runtime` 顶层的 `db/redis/api/auth/tuning/serverConfig/env/logger` 等字段收进更有语义的聚合体。具体做法是新增 `runtimeExternalResources` 和 `runtimeStartupViews`，其中启动视图再分为 HTTP 与 background 两支；现有 `runtimeHTTPView`/`runtimeBackgroundView` builder 改为从分层后的 `Runtime` 聚合中派生。

**Tech Stack:** Go, Go test

---

### Task 1: 为 Runtime 引入分层聚合

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntime_GroupsResourcesAndStartupViews(t *testing.T) {
	runtime, err := buildRuntimeWithFactory(...)
	if err != nil {
		t.Fatalf("expected runtime build to succeed: %v", err)
	}
	if runtime.startupViews.http.env != "dev" {
		t.Fatal("expected http startup view env")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_GroupsResourcesAndStartupViews' -v`
Expected: FAIL，因为新聚合字段尚不存在

**Step 3: Write minimal implementation**

```go
type runtimeExternalResources struct { ... }
type runtimeStartupViews struct { ... }
type runtimeHTTPStartupView struct { ... }
type runtimeBackgroundStartupView struct { ... }
type Runtime struct { resources runtimeExternalResources; startupViews runtimeStartupViews }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_GroupsResourcesAndStartupViews' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): layer runtime resources and startup views"
```

### Task 2: 改造现有 HTTP/background view builder 读取分层 Runtime

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntimeHTTPView_MapsLayeredRuntimeFields(t *testing.T) {
	view, err := buildRuntimeHTTPView(runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.supplyAPI == nil {
		t.Fatal("expected supply api")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntimeHTTPView_MapsHTTPFields|BuildRuntimeBackgroundView_MapsBackgroundFields|AdaptRuntimeToBuildServerOptions_MapsRuntimeFields)' -v`
Expected: FAIL，旧 builder 仍读取旧顶层字段

**Step 3: Write minimal implementation**

```go
func resolveRuntimeHealthChecks(runtime *Runtime) runtimeHealthChecks { ... } // 改为读取 runtime.resources
func buildRuntimeHTTPView(runtime *Runtime) (runtimeHTTPView, error) { ... } // 改为读取 runtime.startupViews.http
func buildRuntimeBackgroundView(runtime *Runtime) (runtimeBackgroundView, error) { ... } // 改为读取 runtime.startupViews.background + runtime.resources
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntimeHTTPView_MapsHTTPFields|BuildRuntimeBackgroundView_MapsBackgroundFields|AdaptRuntimeToBuildServerOptions_MapsRuntimeFields)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): route runtime views through layered surfaces"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/background.go`
- Verify: `supply-api/internal/app/runtime_test.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntime_GroupsResourcesAndStartupViews|BuildRuntimeHTTPView_.*|BuildRuntimeBackgroundView_.*|AdaptRuntime(ToBuildServerOptions|HTTPViewToBuildServerOptions)_.*|Runtime_StartBackgroundWorkers_.*|BuildRuntime_.*)' -v`
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
git add docs/plans/2026-04-16-supply-api-runtime-layered-surfaces-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): reduce runtime aggregation density"
```
