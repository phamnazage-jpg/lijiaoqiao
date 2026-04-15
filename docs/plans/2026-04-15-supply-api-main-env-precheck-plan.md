# Supply API Main Env Precheck Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 `supply-api/cmd/supply-api/main.go` 在加载配置之前就拦截非法环境名，统一复用 app 层环境解析，避免输出低价值的配置文件错误。

**Architecture:** 提升 app 层环境解析 helper 为可复用入口，`main` 在推导默认配置路径与调用 `config.LoadFromPath` 前先做 env 校验。空值仍回退 `dev`，非法值直接失败。

**Tech Stack:** Go, Go test

---

### Task 1: 为 main 增加非法 env 预校验

**Files:**
- Modify: `supply-api/cmd/supply-api/main.go`
- Modify: `supply-api/cmd/supply-api/main_test.go`

**Step 1: Write the failing test**

```go
func TestMain_RejectsUnsupportedEnvBeforeLoadingConfig(t *testing.T) {
	...
	if !strings.Contains(string(output), "unsupported env") {
		t.Fatalf("expected unsupported env error, got: %s", string(output))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./cmd/supply-api -run 'TestMain_RejectsUnsupportedEnvBeforeLoadingConfig' -v`
Expected: FAIL，因为当前会先报配置文件读取失败

**Step 3: Write minimal implementation**

```go
	envName, err := app.ResolveEnv(*env)
	if err != nil {
		jsonLogger.Fatalf("%v", err)
	}
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./cmd/supply-api -run 'TestMain_RejectsUnsupportedEnvBeforeLoadingConfig' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/cmd/supply-api/main.go supply-api/cmd/supply-api/main_test.go
git commit -m "refactor(supply-api): precheck main env values"
```

### Task 2: 让 app 层公开统一环境解析 helper

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/runtime_test.go`
- Modify: `supply-api/internal/app/bootstrap.go`

**Step 1: Write the failing test**

```go
func TestResolveEnv_RejectsUnsupportedValue(t *testing.T) {
	_, err := ResolveEnv("qa")
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveEnv_RejectsUnsupportedValue' -v`
Expected: FAIL，因为 helper 尚未导出

**Step 3: Write minimal implementation**

```go
func ResolveEnv(env string) (string, error) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd "supply-api" && go test ./internal/app -run 'TestResolveEnv_RejectsUnsupportedValue' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go supply-api/internal/app/bootstrap.go
git commit -m "refactor(supply-api): reuse env resolver across app layer"
```

### Task 3: 验证与收尾

**Files:**
- Verify: `supply-api/cmd/supply-api/main.go`
- Verify: `supply-api/internal/app/runtime.go`
- Verify: `supply-api/internal/app/bootstrap.go`

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
git add docs/plans/2026-04-15-supply-api-main-env-precheck-plan.md supply-api/cmd/supply-api/main.go supply-api/cmd/supply-api/main_test.go supply-api/internal/app/runtime.go supply-api/internal/app/runtime_test.go supply-api/internal/app/bootstrap.go
git commit -m "refactor(supply-api): precheck main env before config load"
```
