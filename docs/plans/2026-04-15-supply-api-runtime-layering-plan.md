# Supply API Runtime Layering Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 `supply-api/cmd/supply-api/main.go` 剩余的基础设施初始化与后台 worker 启动职责下沉到 `internal/app`，让 `main` 只保留进程入口与生命周期控制。

**Architecture:** 在现有 `internal/app/bootstrap.go` 之上新增运行时构建层 `BuildRuntime` 和后台任务启动层 `Runtime.StartBackgroundWorkers`。保持当前 dev/prod 语义、DB/Redis 降级策略和门禁错误口径不变，只把装配逻辑拆分成可测试的 app 层。

**Tech Stack:** Go, net/http, context, Go test

---

### Task 1: 抽取可复用的运行时构建层

**Files:**
- Create: `supply-api/internal/app/runtime.go`
- Create: `supply-api/internal/app/runtime_test.go`
- Modify: `supply-api/cmd/supply-api/main.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntime_ProdRequiresDatabase(t *testing.T) {
	_, err := buildRuntimeWithFactory(RuntimeOptions{Env: "prod", Config: cfg, Logger: testLogger{}}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
	})
	if err == nil {
		t.Fatal("expected prod runtime build to reject database outage")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_(ProdRequiresDatabase|DevFallsBackToInMemoryDependencies)' -v`
Expected: FAIL，因为 `BuildRuntime` 尚不存在

**Step 3: Write minimal implementation**

```go
type Runtime struct {
	...
}

func BuildRuntime(opts RuntimeOptions) (*Runtime, error) {
	return buildRuntimeWithFactory(opts, defaultRuntimeFactory())
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_(ProdRequiresDatabase|DevFallsBackToInMemoryDependencies)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go supply-api/cmd/supply-api/main.go
git commit -m "refactor(supply-api): extract runtime bootstrap"
```

### Task 2: 抽取后台 worker 启动层并收紧 main 入口

**Files:**
- Create: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`
- Modify: `supply-api/cmd/supply-api/main.go`

**Step 1: Write the failing test**

```go
func TestRuntime_StartBackgroundWorkers_ProdRequiresOutboxBroker(t *testing.T) {
	runtime := &Runtime{env: "prod", logger: testLogger{}, db: &repository.DB{}}
	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), runtime, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository { return stubOutboxRepository{} },
		newMessageBroker:    func(*cache.RedisCache) messaging.MessageBroker { return nil },
	})
	if err == nil {
		t.Fatal("expected missing outbox broker to fail in prod")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestRuntime_StartBackgroundWorkers_(WithoutDatabaseIsNoop|ProdRequiresOutboxBroker)' -v`
Expected: FAIL，因为后台启动层尚未抽出

**Step 3: Write minimal implementation**

```go
func (r *Runtime) StartBackgroundWorkers(rootCtx, initCtx context.Context) error {
	return startBackgroundWorkersWithFactory(rootCtx, initCtx, r, defaultBackgroundFactory())
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestRuntime_StartBackgroundWorkers_(WithoutDatabaseIsNoop|ProdRequiresOutboxBroker)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/background.go supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go supply-api/cmd/supply-api/main.go
git commit -m "refactor(supply-api): layer background startup"
```

### Task 3: 全量验证与收尾

**Files:**
- Verify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/background.go`
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
git add docs/plans/2026-04-15-supply-api-runtime-layering-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go supply-api/internal/app/background.go supply-api/cmd/supply-api/main.go
git commit -m "refactor(supply-api): layer runtime startup flow"
```
