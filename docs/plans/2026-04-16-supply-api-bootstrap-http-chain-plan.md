# Supply API Bootstrap HTTP Chain Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `supply-api/internal/app/bootstrap.go` 中的 HTTP 装配链拆清楚，让 `BuildServer` 只负责校验和组装，路由注册与中间件编排各自收口到独立 helper。

**Architecture:** 保留现有 `BuildServer` 外部接口与运行时语义，新增路由装配 helper 负责 health/supply/alert 路由注册，新增中间件链 helper 负责通用中间件和 prod/staging 鉴权链编排。先写 helper 级失败测试锁住对外可观测行为，再做最小重构并跑整组回归。

**Tech Stack:** Go, Go test

---

### Task 1: 提取路由注册 helper

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRouteMux_RegistersHealthAndSupplyRoutes(t *testing.T) {
	mux := buildRouteMux(buildRouteMuxOptions{...})
	// /actuator/health => 200
	// /api/v1/supply/accounts/verify => 405 on GET, proving route exists
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRouteMux_RegistersHealthAndSupplyRoutes' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type buildRouteMuxOptions struct { ... }
func buildRouteMux(opts buildRouteMuxOptions) *http.ServeMux { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRouteMux_RegistersHealthAndSupplyRoutes' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): extract bootstrap route mux builder"
```

### Task 2: 提取中间件链 helper

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Write the failing test**

```go
func TestBuildMiddlewareChain_ProdRejectsQueryKey(t *testing.T) {
	handler := buildMiddlewareChain(..., http.HandlerFunc(...))
	// /api/v1/supply/accounts?token=bad => 401
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildMiddlewareChain_ProdRejectsQueryKey' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type middlewareChainOptions struct { ... }
func buildMiddlewareChain(opts middlewareChainOptions, next http.Handler) http.Handler { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildMiddlewareChain_ProdRejectsQueryKey' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): extract bootstrap middleware chain"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Verify: `supply-api/internal/app/bootstrap_test.go`
- Verify: `supply-api/internal/app/runtime.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRouteMux_RegistersHealthAndSupplyRoutes|BuildMiddlewareChain_ProdRejectsQueryKey|BuildServer_.*)' -v`
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
git add docs/plans/2026-04-16-supply-api-bootstrap-http-chain-plan.md supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): split bootstrap http assembly"
```
