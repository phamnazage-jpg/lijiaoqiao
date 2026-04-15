# Supply API Store Bundle Split Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `supply-api/internal/app/runtime.go` 中 `buildStoreBundle` 的 DB/in-memory 双分支拆成两个更小的 helper，降低单函数分支复杂度并保持现有语义不变。

**Architecture:** 保留 `buildStoreBundle` 作为总入口，只负责根据 `db` 是否存在分发到 `buildDBStoreBundle` 与 `buildMemoryStoreBundle`。先补 helper 级失败测试，再复用现有 `buildStoreBundle` 回归测试验证委派后的行为不变。

**Tech Stack:** Go, Go test

---

### Task 1: 提取 DB-backed store helper

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildDBStoreBundle_InitializesDatabaseOnlyDependencies(t *testing.T) {
	bundle := buildDBStoreBundle(&repository.DB{})
	if bundle.tokenStatusRepo == nil {
		t.Fatal("expected token status repo")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildDBStoreBundle_InitializesDatabaseOnlyDependencies' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func buildDBStoreBundle(db *repository.DB) runtimeStoreBundle { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildDBStoreBundle_InitializesDatabaseOnlyDependencies' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract db store bundle builder"
```

### Task 2: 提取内存 store helper

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildMemoryStoreBundle_DisablesDatabaseOnlyDependencies(t *testing.T) {
	bundle := buildMemoryStoreBundle()
	if bundle.idempotencyRepo != nil {
		t.Fatal("expected nil idempotency repo")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildMemoryStoreBundle_DisablesDatabaseOnlyDependencies' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func buildMemoryStoreBundle() runtimeStoreBundle { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildMemoryStoreBundle_DisablesDatabaseOnlyDependencies' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): extract memory store bundle builder"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/runtime_test.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(Build(DB|Memory)StoreBundle_.*|BuildStoreBundle_.*)' -v`
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
git add docs/plans/2026-04-15-supply-api-store-bundle-split-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): split runtime store bundle builders"
```
