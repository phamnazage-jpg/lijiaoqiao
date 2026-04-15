# Supply API Finishing Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 消除 `supply-api` 剩余的库级 panic 路径，把构造与配置加载统一收敛为显式错误返回。

**Architecture:** 本轮只做两个收尾点。第一，`AlertAPI` 构造函数不再因依赖缺失直接 `panic`，而是返回明确错误并由启动入口处理。第二，删除未使用的 `config.MustLoad` panic helper，保留 `Load` / `LoadFromPath` 作为唯一配置加载入口，并用测试锁定“缺配置时返回错误或默认值，而不是 panic”的语义。

**Tech Stack:** Go, `go test`, Markdown

---

### Task 1: 移除 AlertAPI 构造期 panic

**Files:**
- Modify: `supply-api/internal/httpapi/alert_api.go`
- Modify: `supply-api/internal/httpapi/alert_api_test.go`
- Modify: `supply-api/cmd/supply-api/main.go`

**Step 1: 写失败测试**

Write:
```go
func TestNewAlertAPI_ReturnsErrorWhenServiceMissing(t *testing.T) {
	api, err := NewAlertAPI(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if api != nil {
		t.Fatal("expected nil api")
	}
}
```

**Step 2: 运行测试，确认当前实现不满足新语义**

Run:
```bash
cd supply-api
go test ./internal/httpapi -run 'TestNewAlertAPI_(ReturnsErrorWhenServiceMissing|UsesInjectedStore)' -v
```

Expected:
- 失败，原因是 `NewAlertAPI` 当前返回单值并在 `nil` 依赖时 `panic`。

**Step 3: 写最小实现**

Write:
- `NewAlertAPI` 改为返回 `(*AlertAPI, error)`。
- `alertSvc == nil` 时返回明确错误，不再 `panic`。
- `cmd/supply-api/main.go` 在构造失败时显式 `Fatalf`，把进程退出责任留在入口层。

**Step 4: 运行验证**

Run:
```bash
cd supply-api
go test ./internal/httpapi ./cmd/supply-api
```

Expected:
- `AlertAPI` 构造期不再有库级 panic。
- 启动入口仍保持显式失败。

### Task 2: 删除未使用的配置 panic helper，收敛到显式错误返回

**Files:**
- Modify: `supply-api/internal/config/config.go`
- Modify: `supply-api/internal/config/config_test.go`

**Step 1: 写保护测试，锁定配置加载的非 panic 路径**

Write:
```go
func TestLoad_UsesDefaultsWithoutConfigFile(t *testing.T) {}
```

Test intent:
- 在临时空目录中调用 `Load("dev")`。
- 期望返回默认配置且不报错。

**Step 2: 运行测试**

Run:
```bash
cd supply-api
go test ./internal/config -run 'TestLoad_(UsesDefaultsWithoutConfigFile|FromPath_MissingFile)' -v
```

Expected:
- `Load("dev")` 走默认值路径。
- `LoadFromPath` 对显式缺失文件返回错误。

**Step 3: 写最小实现**

Write:
- 删除未使用的 `MustLoad` helper。
- 保留 `Load` / `LoadFromPath` 作为唯一公开入口。
- 不引入新的 must/ panic 包装。

**Step 4: 运行验证**

Run:
```bash
cd supply-api
go test ./internal/config ./cmd/supply-api
rg -n 'func MustLoad\\(' internal/config
```

Expected:
- 配置测试通过。
- 仓库中不再存在 `MustLoad` 定义。

### Task 3: 最终收口验证

**Files:**
- Modify: `docs/plans/2026-04-15-supply-api-finishing-refactor-plan.md`

**Step 1: 运行最终验证**

Run:
```bash
cd supply-api
go test ./internal/httpapi ./internal/config ./cmd/supply-api
cd ..
bash scripts/ci/repo_integrity_check.sh
```

Expected:
- 定向测试通过。
- 仓库级基线继续通过。

**Step 2: Commit**

```bash
git add docs/plans/2026-04-15-supply-api-finishing-refactor-plan.md supply-api/internal/httpapi/alert_api.go supply-api/internal/httpapi/alert_api_test.go supply-api/internal/config/config.go supply-api/internal/config/config_test.go supply-api/cmd/supply-api/main.go
git commit -m "refactor(supply-api): remove panic-only helper paths"
```
