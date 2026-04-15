# Supply API Server Defaults Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `supply-api/internal/app` 增加统一的 server 零值默认配置，避免 `BuildServer` 在零值超时下直接生成无超时保护的 `http.Server`。

**Architecture:** 在 app 层集中实现 `normalizeServerConfig`，由 `BuildServer` 与 `BuildRuntime` 共用。保持现有默认值与 `config` 层一致：地址 `:18082`、读超时 `10s`、写超时 `15s`、空闲超时 `30s`、关闭超时 `5s`。

**Tech Stack:** Go, net/http, Go test

---

### Task 1: 为 BuildServer 补齐零值超时默认值

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Write the failing test**

```go
func TestBuildServer_DefaultsTimeoutsWhenUnset(t *testing.T) {
	srv, err := BuildServer(BuildServerOptions{Env: "dev", Logger: testLogger{}, SupplyAPI: supplyAPI, AlertAPI: alertAPI})
	...
	if srv.ReadTimeout == 0 {
		t.Fatal("expected read timeout default")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildServer_DefaultsTimeoutsWhenUnset' -v`
Expected: FAIL，因为当前零值超时会原样进入 `http.Server`

**Step 3: Write minimal implementation**

```go
func normalizeServerConfig(cfg config.ServerConfig) config.ServerConfig { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildServer_DefaultsTimeoutsWhenUnset' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): default zero-value server timeouts"
```

### Task 2: 让 Runtime 统一复用规范化后的 server 配置

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntime_NormalizesServerConfigDefaults(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.Server = config.ServerConfig{}
	runtime, err := buildRuntimeWithFactory(..., cfg)
	...
	if runtime.ShutdownTimeout() == 0 {
		t.Fatal("expected shutdown timeout default")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_NormalizesServerConfigDefaults' -v`
Expected: FAIL，因为当前 runtime 直接保留原始零值配置

**Step 3: Write minimal implementation**

```go
	serverConfig: normalizeServerConfig(opts.Config.Server),
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_NormalizesServerConfigDefaults' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): normalize runtime server config"
```

### Task 3: 验证与收尾

**Files:**
- Verify: `supply-api/internal/app/bootstrap.go`
- Verify: `supply-api/internal/app/runtime.go`
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
git add docs/plans/2026-04-15-supply-api-server-defaults-plan.md supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): normalize app server defaults"
```
