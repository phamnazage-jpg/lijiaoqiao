# Supply API Env Guard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `supply-api/internal/app` 增加统一环境名校验，确保只有 `dev` / `staging` / `prod` 可以进入运行时与 HTTP 装配流程，避免非法 env 静默启动。

**Architecture:** 复用 app 层统一的 env 解析 helper，`BuildRuntime` 和 `BuildServer` 共用同一份校验逻辑。默认空值仍回退 `dev`，但非法值必须显式返回错误。

**Tech Stack:** Go, Go test

---

### Task 1: 为 BuildRuntime 增加非法环境校验

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`

**Step 1: Write the failing test**

```go
func TestBuildRuntime_RejectsUnsupportedEnv(t *testing.T) {
	_, err := buildRuntimeWithFactory(RuntimeOptions{Env: "qa", ...}, runtimeFactory{...})
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_RejectsUnsupportedEnv' -v`
Expected: FAIL，因为当前非法 env 仍会继续构建

**Step 3: Write minimal implementation**

```go
func resolveEnv(env string) (string, error) {
	...
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildRuntime_RejectsUnsupportedEnv' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go
git commit -m "refactor(supply-api): reject unsupported runtime envs"
```

### Task 2: 为 BuildServer 复用非法环境校验

**Files:**
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`

**Step 1: Write the failing test**

```go
func TestBuildServer_RejectsUnsupportedEnv(t *testing.T) {
	_, err := BuildServer(BuildServerOptions{Env: "qa", ...})
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildServer_RejectsUnsupportedEnv' -v`
Expected: FAIL，因为当前非法 env 会继续走非 dev 分支

**Step 3: Write minimal implementation**

```go
env, err := resolveEnv(opts.Env)
if err != nil {
	return nil, err
}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestBuildServer_RejectsUnsupportedEnv' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): guard bootstrap env values"
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
git add docs/plans/2026-04-15-supply-api-env-guard-plan.md supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go supply-api/internal/app/bootstrap.go supply-api/internal/app/bootstrap_test.go
git commit -m "refactor(supply-api): guard unsupported env values"
```
