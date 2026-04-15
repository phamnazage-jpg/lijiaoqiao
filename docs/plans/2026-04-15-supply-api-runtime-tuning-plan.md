# Supply API Runtime Tuning Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 收敛 `supply-api/internal/app` 中残余的启动硬编码常量与 warning 日志口径，使运行时默认值集中定义、日志等级语义一致。

**Architecture:** 保持现有启动行为不变，只把 outbox stream/group、后台轮询周期、补偿周期、幂等 TTL 等启动默认值集中到 app 层默认配置中，并用统一日志 helper 输出 `Warn`/`Info`。测试覆盖 dev 降级和后台任务缺 broker 场景的日志等级与默认值。

**Tech Stack:** Go, Go test, structured logging

---

### Task 1: 收敛运行时默认参数

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntime_SeedsDefaultTuning(t *testing.T) {
	runtime, err := buildRuntimeWithFactory(RuntimeOptions{...}, runtimeFactory{...})
	if err != nil {
		t.Fatalf("build runtime failed: %v", err)
	}
	if runtime.tuning.outboxStreamName != "supply:outbox:stream" {
		t.Fatalf("unexpected outbox stream: %s", runtime.tuning.outboxStreamName)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntime_SeedsDefaultTuning|Runtime_StartBackgroundWorkers_UsesDefaultCompensationInterval)' -v`
Expected: FAIL，因为默认调优配置尚未集中定义

**Step 3: Write minimal implementation**

```go
type runtimeTuning struct {
	outboxStreamName            string
	outboxConsumerGroup         string
	idempotencyTTL              time.Duration
	partitionMaintenanceInterval time.Duration
	compensationCheckInterval   time.Duration
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntime_SeedsDefaultTuning|Runtime_StartBackgroundWorkers_UsesDefaultCompensationInterval)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): centralize runtime tuning defaults"
```

### Task 2: 收敛 warning 日志等级口径

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntime_DevFallbackLogsWarnings(t *testing.T) {
	logger := &captureLogger{}
	_, _ = buildRuntimeWithFactory(RuntimeOptions{Env: "dev", Logger: logger, ...}, runtimeFactory{...})
	if logger.warnCount == 0 {
		t.Fatal("expected warning logs during dev fallback")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntime_DevFallbackLogsWarnings|Runtime_StartBackgroundWorkers_DevMissingOutboxBrokerLogsWarning)' -v`
Expected: FAIL，因为当前 warning 仍走 `Info`

**Step 3: Write minimal implementation**

```go
func warnf(logger logging.Logger, format string, args ...any) {
	logger.Warn(fmt.Sprintf(format, args...), nil)
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'Test(BuildRuntime_DevFallbackLogsWarnings|Runtime_StartBackgroundWorkers_DevMissingOutboxBrokerLogsWarning)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): align startup warning logs"
```

### Task 3: 验证与收尾

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
git add docs/plans/2026-04-15-supply-api-runtime-tuning-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): normalize runtime startup defaults"
```
