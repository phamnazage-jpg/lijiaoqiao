# Supply API Bootstrap Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 收敛 `supply-api` 的构造期显式错误处理，并抽离可复用的 HTTP 启动装配层，消除 `main` 与 `e2e` 的重复装配逻辑。

**Architecture:** 保持现有领域初始化与后台 worker 启动不动，只把 HTTP server、health route、API route 和 middleware chain 抽到 `internal/app`。`httpapi.NewSupplyAPI` 继续承担处理器级依赖校验，`internal/app` 负责装配期依赖校验和环境分支约束。

**Tech Stack:** Go, net/http, httptest, Go test, structured logging

---

### Task 1: 收紧 SupplyAPI 构造器错误口径

**Files:**
- Modify: `supply-api/internal/httpapi/supply_api.go`
- Modify: `supply-api/internal/httpapi/supply_api_test.go`
- Modify: `supply-api/cmd/supply-api/main.go`
- Modify: `supply-api/e2e/e2e_test.go`

**Step 1: Write the failing test**

```go
func TestNewSupplyAPI_ReturnsErrorWhenAccountServiceMissing(t *testing.T) {
	api, err := NewSupplyAPI(nil, pkgSvc, settlementSvc, earningSvc, idempotencyMw, auditStore, nil, 1, "", time.Now)
	if err == nil {
		t.Fatal("expected constructor to reject missing account service")
	}
	if api != nil {
		t.Fatal("expected nil api when constructor fails")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/httpapi -run 'TestNewSupplyAPI_(ReturnsErrorWhenAccountServiceMissing|DefaultsClockWhenNil)' -v`
Expected: FAIL，因为构造器尚未返回 `error`

**Step 3: Write minimal implementation**

```go
func NewSupplyAPI(...) (*SupplyAPI, error) {
	if accountService == nil {
		return nil, errors.New("account service is required")
	}
	if now == nil {
		now = time.Now
	}
	return &SupplyAPI{...}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/httpapi -run 'TestNewSupplyAPI_(ReturnsErrorWhenAccountServiceMissing|DefaultsClockWhenNil)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/httpapi/supply_api.go supply-api/internal/httpapi/supply_api_test.go supply-api/cmd/supply-api/main.go supply-api/e2e/e2e_test.go
git commit -m "refactor(supply-api): make supply api constructor explicit"
```

### Task 2: 抽离可复用的 HTTP 启动装配层

**Files:**
- Create: `supply-api/internal/app/bootstrap.go`
- Create: `supply-api/internal/app/bootstrap_test.go`
- Modify: `supply-api/cmd/supply-api/main.go`
- Modify: `supply-api/e2e/e2e_test.go`

**Step 1: Write the failing test**

```go
func TestBuildServer_ReturnsErrorWhenSupplyAPIMissing(t *testing.T) {
	server, err := BuildServer(BuildServerOptions{Logger: testLogger{}, AlertAPI: alertAPI})
	if err == nil {
		t.Fatal("expected missing supply api to fail")
	}
	if server != nil {
		t.Fatal("expected nil server on invalid bootstrap options")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildServer_(ReturnsErrorWhenSupplyAPIMissing|ProdRequiresAuthMiddleware|RegistersHealthRoute)' -v`
Expected: FAIL，因为 `internal/app` 尚不存在

**Step 3: Write minimal implementation**

```go
type BuildServerOptions struct {
	Env             string
	ServerConfig    config.ServerConfig
	Logger          logging.Logger
	SupplyAPI       *httpapi.SupplyAPI
	AlertAPI        *httpapi.AlertAPI
	AuthMiddleware  *middleware.AuthMiddleware
	RateLimitConfig *middleware.RateLimitConfig
	DBHealthCheck   func(context.Context) error
	RedisHealthCheck func(context.Context) error
}
```

```go
func BuildServer(opts BuildServerOptions) (*http.Server, error) {
	if opts.SupplyAPI == nil {
		return nil, errors.New("supply api is required")
	}
	...
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildServer_(ReturnsErrorWhenSupplyAPIMissing|ProdRequiresAuthMiddleware|RegistersHealthRoute)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go supply-api/cmd/supply-api/main.go supply-api/e2e/e2e_test.go
git commit -m "refactor(supply-api): extract reusable http bootstrap"
```

### Task 3: 完整验证与收尾

**Files:**
- Verify: `supply-api/internal/httpapi/supply_api.go`
- Verify: `supply-api/internal/app/bootstrap.go`
- Verify: `supply-api/cmd/supply-api/main.go`
- Verify: `supply-api/e2e/e2e_test.go`

**Step 1: Run focused package tests**

Run: `cd "supply-api" && go test ./internal/httpapi ./internal/app ./cmd/supply-api`
Expected: PASS

**Step 2: Run e2e build-tag tests**

Run: `cd "supply-api" && go test -tags=e2e ./e2e`
Expected: PASS

**Step 3: Run repo exit verification**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: PASS

**Step 4: Check formatting and workspace**

Run: `git diff --check`
Expected: no output

**Step 5: Commit**

```bash
git add docs/plans/2026-04-15-supply-api-bootstrap-refactor-plan.md supply-api/internal/httpapi/supply_api.go supply-api/internal/httpapi/supply_api_test.go supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go supply-api/cmd/supply-api/main.go supply-api/e2e/e2e_test.go
git commit -m "refactor(supply-api): deduplicate bootstrap assembly"
```
