# Supply API Bootstrap Options Resolve Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `supply-api/internal/app/bootstrap.go` 中剩余的参数校验和默认 rate-limit 装配收成更小的 helper，让 `BuildServer` 只保留声明式组装流程。

**Architecture:** 保留 `BuildServer` 现有外部接口和行为，新增 `resolveBuildServerOptions` 负责参数校验、环境解析和 server config 归一化，再新增 `resolveRateLimitConfig` 负责默认限流配置装配。`BuildServer` 最终只串联 `resolve -> buildRouteMux -> buildMiddlewareChain -> server` 四步。

**Tech Stack:** Go, Go test

---

### Task 1: 提取 build server options resolve helper

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Write the failing test**

```go
func TestResolveBuildServerOptions_RequiresAuthOutsideDev(t *testing.T) {
	_, err := resolveBuildServerOptions(BuildServerOptions{Env: "prod", ...})
	if err == nil {
		t.Fatal("expected auth middleware requirement")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveBuildServerOptions_RequiresAuthOutsideDev' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
type resolvedBuildServerOptions struct { ... }
func resolveBuildServerOptions(opts BuildServerOptions) (resolvedBuildServerOptions, error) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveBuildServerOptions_RequiresAuthOutsideDev' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): extract bootstrap options resolver"
```

### Task 2: 提取默认 rate-limit helper

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Write the failing test**

```go
func TestResolveRateLimitConfig_DefaultsDisabledInDev(t *testing.T) {
	cfg := resolveRateLimitConfig("dev", nil)
	if cfg.Enabled {
		t.Fatal("expected dev default to disable rate limit")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveRateLimitConfig_(DefaultsDisabledInDev|DefaultsEnabledOutsideDev)' -v`
Expected: FAIL，因为 helper 尚不存在

**Step 3: Write minimal implementation**

```go
func resolveRateLimitConfig(env string, cfg *middleware.RateLimitConfig) *middleware.RateLimitConfig { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveRateLimitConfig_(DefaultsDisabledInDev|DefaultsEnabledOutsideDev)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): extract bootstrap rate limit resolver"
```

### Task 3: 回归验证与收尾

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Verify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Run focused tests**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(ResolveBuildServerOptions_RequiresAuthOutsideDev|ResolveRateLimitConfig_(DefaultsDisabledInDev|DefaultsEnabledOutsideDev)|BuildServer_.*|BuildRouteMux_RegistersHealthAndSupplyRoutes|BuildMiddlewareChain_ProdRejectsQueryKey)' -v`
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
git add docs/plans/2026-04-16-supply-api-bootstrap-options-resolve-plan.md supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): declarify bootstrap server assembly"
```
