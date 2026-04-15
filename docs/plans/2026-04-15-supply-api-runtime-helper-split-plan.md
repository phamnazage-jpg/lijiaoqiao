# Supply API Runtime Helper Split Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `supply-api/internal/app/runtime.go` 中偏大的 `buildRuntimeWithFactory` 拆成更小的装配 helper，降低单函数复杂度，同时保持现有行为不变。

**Architecture:** 新增存储装配、鉴权装配和 API 装配的内部 helper struct/func，由 `buildRuntimeWithFactory` 只负责串联。继续保留现有外部 API、日志口径和运行时语义，测试先锁 helper 级行为，再复用现有高层回归。

**Tech Stack:** Go, Go test

---

### Task 1: 提取存储装配 helper

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildStoreBundle_UsesInMemoryStoresWithoutDatabase(t *testing.T) {
	bundle := buildStoreBundle(nil, testLogger{})
	if bundle.accountStore == nil {
		t.Fatal("expected account store")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildStoreBundle_(UsesInMemoryStoresWithoutDatabase|UsesDatabaseBackedStoresWithDatabase)' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type runtimeStoreBundle struct { ... }
func buildStoreBundle(db *repository.DB, logger logging.Logger) runtimeStoreBundle { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildStoreBundle_(UsesInMemoryStoresWithoutDatabase|UsesDatabaseBackedStoresWithDatabase)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime store bundle"
```

### Task 2: 提取安全与 API 装配 helper

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildSecurityBundle_UsesMemoryTokenBackendWithoutRepository(t *testing.T) {
	bundle := buildSecurityBundle("dev", cfg, testLogger{}, auditStore, nil, nil)
	if bundle.authMiddleware == nil {
		t.Fatal("expected auth middleware")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildSecurityBundle_UsesMemoryTokenBackendWithoutRepository' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type runtimeSecurityBundle struct { ... }
type runtimeAPIBundle struct { ... }
func buildSecurityBundle(...) runtimeSecurityBundle { ... }
func buildAPIBundle(...) (runtimeAPIBundle, error) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildSecurityBundle_UsesMemoryTokenBackendWithoutRepository' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract runtime security and api bundles"
```

### Task 3: 验证与收尾

**Files:**
- Verify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/bootstrap.go`
- Verify: `supply-api/cmd/supply-api/main.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app ./cmd/supply-api ./internal/httpapi`
Expected: PASS

**Step 2: Run e2e build-tag tests**

Run: `cd "supply-api" && go test -tags=e2e ./e2e`
Expected: PASS

**Step 3: Run repo exit verification**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: PASS

**Step 4: Check formatting**

Run: `git diff --check`
Expected: no output

**Step 5: Commit**

```bash
git add docs/plans/2026-04-15-supply-api-runtime-helper-split-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): split runtime assembly helpers"
```
