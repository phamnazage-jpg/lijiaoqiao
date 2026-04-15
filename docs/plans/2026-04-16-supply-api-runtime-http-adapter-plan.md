# Supply API Runtime HTTP Adapter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `Runtime.BuildServer` 里剩余的 `BuildServerOptions` 构造收成更明确的 adapter，让 runtime 到 HTTP 启动的边界完全声明式。

**Architecture:** 保留 `Runtime.BuildServer` 和 `BuildServer` 的现有外部接口，新增内部 helper 负责从 runtime 提取健康检查和构造 `BuildServerOptions`。`Runtime.BuildServer` 最终只做 `adapt runtime -> BuildServer(opts)`，把边界映射与 HTTP 启动解耦。

**Tech Stack:** Go, Go test

---

### Task 1: 提取 runtime health check adapter

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestResolveRuntimeHealthChecks_OmitsUnavailableDependencies(t *testing.T) {
	checks := resolveRuntimeHealthChecks(&Runtime{})
	if checks.DBHealthCheck != nil || checks.RedisHealthCheck != nil {
		t.Fatal("expected nil health checks without dependencies")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveRuntimeHealthChecks_(OmitsUnavailableDependencies|ExposesAvailableDependencies)' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type runtimeHealthChecks struct { ... }
func resolveRuntimeHealthChecks(runtime *Runtime) runtimeHealthChecks { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveRuntimeHealthChecks_(OmitsUnavailableDependencies|ExposesAvailableDependencies)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime health check adapter"
```

### Task 2: 提取 BuildServerOptions adapter

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestAdaptRuntimeToBuildServerOptions_MapsRuntimeFields(t *testing.T) {
	opts, err := adaptRuntimeToBuildServerOptions(runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.SupplyAPI != runtime.supplyAPI {
		t.Fatal("expected supply api to be preserved")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestAdaptRuntimeToBuildServerOptions_(RequiresRuntime|MapsRuntimeFields)' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func adaptRuntimeToBuildServerOptions(runtime *Runtime) (BuildServerOptions, error) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestAdaptRuntimeToBuildServerOptions_(RequiresRuntime|MapsRuntimeFields)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime server options adapter"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/runtime_test.go`
- Verify: `supply-api/internal/app/bootstrap.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(ResolveRuntimeHealthChecks_(OmitsUnavailableDependencies|ExposesAvailableDependencies)|AdaptRuntimeToBuildServerOptions_(RequiresRuntime|MapsRuntimeFields)|BuildServer_.*|BuildRuntime_.*)' -v`
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
git add docs/plans/2026-04-16-supply-api-runtime-http-adapter-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): declarify runtime http adapter"
```
