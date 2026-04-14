# Repository Integrity Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 以 `gateway`、`platform-token-runtime`、`supply-api` 三个一方服务为范围，补齐当前可见的功能断点、收敛开发态降级实现，并把仓库整理为“文档、代码、DDL、测试结论一致”的可持续状态。

**Architecture:** 先冻结“真实状态”，再按服务拆分重构。`gateway` 负责统一入口、鉴权和上游路由；`platform-token-runtime` 负责 token 生命周期与审计查询；`supply-api` 负责供应链业务、审计、幂等、补偿和 outbox。重构顺序必须遵循“先验证基线，再拔除降级路径，再补持久化和契约测试”，避免再次出现报告先于代码、文档先于事实的漂移。

**Tech Stack:** Go, PostgreSQL, Redis, HTTP, `go test`, `rg`, Bash CI 脚本, Markdown

---

## 当前已验证基线

以下事实已在 2026-04-14 本地复核：

- `git status --branch --short` 干净，当前分支为 `upload/2026-03-26-sync-clean`，相对 `github/upload/2026-03-26-sync` `ahead 47`。
- `gateway` 运行 `go test ./...` 通过。
- `platform-token-runtime` 运行 `go test ./...` 通过。
- `supply-api` 运行 `go test ./...` 通过。
- `supply-api` 运行 `GOCACHE=/tmp/lijiaoqiao-go-cache-plan-e2e go test -tags=e2e ./e2e` 通过。

## 当前主要缺口

### 1. 功能完整性缺口

- `platform-token-runtime/internal/httpapi/token_api.go` 中 `GET /api/v1/platform/tokens/audit-events` 仍可能返回 `501 AUDIT_QUERY_NOT_READY`，说明审计查询仍是半完成能力。
- `platform-token-runtime/cmd/platform-token-runtime/main.go` 只装配内存实现，并在 `prod/staging` 直接拒绝启动，说明它还是开发态运行时，不是可上线组件。
- `supply-api/internal/httpapi/alert_api.go` 将告警 API 固定绑到 `service.NewInMemoryAlertStore()`，与 `cmd/supply-api/main.go` 的 PostgreSQL 启动路径完全脱钩。
- `supply-api/internal/httpapi/supply_api.go` 仍保留“幂等中间件缺失时回退到内联幂等逻辑”的降级分支。
- `supply-api/internal/sms/tencent_sms.go` 与 `supply-api/internal/sms/aliyun_sms.go` 的 `VerifyCode` 仍未实现。
- `supply-api/internal/audit/service/batch_buffer.go` 仍保留“Flush 失败后的错误处理 TODO”，说明审计批处理链路没有完整失败策略。

### 2. 架构完整性缺口

- `gateway/cmd/gateway/main.go` 直接承担配置加载、provider 注册、鉴权运行时选择、限流、审计、HTTP server 装配，组合根过大。
- `gateway/internal/config/config.go` 定义了 `Providers []ProviderConfig`，但 `gateway/cmd/gateway/main.go` 仍硬编码注册 OpenAI provider，配置与运行时脱节。
- `supply-api/cmd/supply-api/main.go` 仍包含大量装配逻辑和生产/开发降级判断，入口职责过重。
- `supply-api` 的仓储集成测试大量 `Skip`，并明确指出 `supply_accounts`、`supply_packages`、`supply_settlements` 缺失 `version`、`idempotency_key`、`available_quota` 等字段，说明 DDL 与仓储契约没有收口。

### 3. 文档与事实源缺口

- `gateway/` 根目录没有 README，模块用途与运行方式没有一份正式入口文档。
- `supply-api/README.md` 引用了仓库中不存在的 `sql/postgresql/platform_core_schema_v1.sql`，说明 README 与真实 DDL 已漂移。
- 当前多份历史报告已经被降级为历史快照，但仓库还缺一条统一的“当前状态以哪些命令/文件为准”的事实源说明。

### 4. 测试覆盖缺口

- `gateway/pkg/model` 无测试文件。
- `platform-token-runtime/internal/auth/model` 与 `platform-token-runtime/internal/auth/service` 无包级测试。
- `supply-api/internal/cache`、`internal/iam/repository`、`internal/messaging`、`internal/storage` 无测试文件。
- `supply-api` 的大量业务能力只在内存路径或 handler 路径下被覆盖，没有持久化路径的契约测试闭环。

## 执行顺序

1. 冻结仓库级真实状态和统一验证脚本。
2. 收口 `gateway` 入口和 provider 装配。
3. 把 `platform-token-runtime` 从开发态运行时推进到可查询、可持久化、可装配的服务。
4. 收口 `supply-api` 的告警、幂等、DDL 契约和外部集成断点。
5. 最后修正文档与 CI，使“代码、测试、报告”共享同一事实源。

---

### Task 1: 建立仓库级真实状态检查脚本

**Files:**
- Create: `scripts/ci/repo_integrity_check.sh`
- Modify: `docs/plans/2026-04-14-repo-integrity-refactor-plan.md`
- Modify: `supply-api/README.md`
- Create: `gateway/README.md`

**Step 1: 写失败用例，锁定当前必须通过的校验矩阵**

Write:
```bash
#!/usr/bin/env bash
set -euo pipefail

echo "[repo] gateway"
(cd gateway && go test ./...)

echo "[repo] platform-token-runtime"
(cd platform-token-runtime && go test ./...)

echo "[repo] supply-api unit"
(cd supply-api && go test ./...)

echo "[repo] supply-api e2e"
(cd supply-api && GOCACHE=/tmp/lijiaoqiao-go-cache-repo-integrity go test -tags=e2e ./e2e)
```

**Step 2: 先运行脚本，确认仓库基线与手工复核一致**

Run:
```bash
bash scripts/ci/repo_integrity_check.sh
```

Expected:
- 四段校验全部通过。
- 输出顺序固定，后续可直接作为“当前仓库健康度”的唯一事实入口。

**Step 3: 补模块入口文档**

Write:
- `gateway/README.md` 说明启动方式、配置来源、provider 注册原则、与 `platform-token-runtime` / `supply-api` 的边界。
- `supply-api/README.md` 删除不存在的 DDL 引用，改成指向真实存在的 `sql/postgresql/*.sql`。

**Step 4: 复跑校验**

Run:
```bash
git diff --check
bash scripts/ci/repo_integrity_check.sh
```

Expected:
- 文档无格式错误。
- 脚本仍通过。

**Step 5: Commit**

```bash
git add scripts/ci/repo_integrity_check.sh gateway/README.md supply-api/README.md docs/plans/2026-04-14-repo-integrity-refactor-plan.md
git commit -m "chore(repo): add integrity baseline check"
```

---

### Task 2: 拆分 gateway 组合根并消除硬编码 provider

**Files:**
- Create: `gateway/internal/app/bootstrap.go`
- Create: `gateway/internal/app/bootstrap_test.go`
- Create: `gateway/internal/app/providers.go`
- Create: `gateway/internal/app/providers_test.go`
- Modify: `gateway/cmd/gateway/main.go`
- Modify: `gateway/internal/config/config.go`
- Modify: `gateway/internal/config/config_test.go`

**Step 1: 写失败测试，固定 bootstrap 输出**

Write:
```go
func TestBuildServer_FromConfigProviders(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.ProviderConfig{{
			Name: "openai",
			Type: "openai",
			BaseURL: "https://api.openai.com",
			APIKey: "secret",
			Models: []string{"gpt-4o"},
		}},
	}

	server, err := app.BuildServer(cfg)
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}
	if server == nil {
		t.Fatal("expected server")
	}
}
```

**Step 2: 运行测试，确认当前实现缺少 bootstrap 层**

Run:
```bash
cd gateway
go test ./internal/app ./cmd/gateway
```

Expected:
- `internal/app` 不存在或 `BuildServer` 未定义导致失败。

**Step 3: 写最小实现**

Write:
- `gateway/internal/app/bootstrap.go`：封装配置校验、runtime 构建、provider 注册、mux 和 `http.Server` 创建。
- `gateway/internal/app/providers.go`：根据 `cfg.Providers` 构建 provider；没有配置时返回明确错误，不再默认偷偷注册 OpenAI。
- `gateway/cmd/gateway/main.go`：仅保留参数解析、信号处理、调用 `app.BuildServer`。

**Step 4: 扩展鉴权模式测试**

Write:
```go
func TestValidateAuthConfig_ProdRequiresRemoteIntrospection(t *testing.T) {
	cfg := config.AuthConfig{Env: "prod", TokenRuntimeMode: "inmemory"}
	if err := config.ValidateAuthConfig(cfg); err == nil {
		t.Fatal("expected error")
	}
}
```

**Step 5: 运行验证**

Run:
```bash
cd gateway
go test ./cmd/gateway ./internal/app ./internal/config ./internal/middleware ./internal/handler
go test ./...
```

Expected:
- `main.go` 只剩薄入口。
- provider 由配置驱动。
- `prod` 下禁止内存 runtime 的约束保持成立。

**Step 6: Commit**

```bash
git add gateway/cmd/gateway/main.go gateway/internal/app gateway/internal/config/config.go gateway/internal/config/config_test.go
git commit -m "refactor(gateway): extract bootstrap and provider registry"
```

---

### Task 3: 让 platform-token-runtime 具备真实审计查询能力

**Files:**
- Create: `platform-token-runtime/internal/auth/service/audit_store.go`
- Create: `platform-token-runtime/internal/auth/service/audit_store_test.go`
- Create: `platform-token-runtime/internal/auth/service/runtime_store.go`
- Create: `platform-token-runtime/internal/auth/service/runtime_store_test.go`
- Modify: `platform-token-runtime/internal/auth/service/inmemory_runtime.go`
- Modify: `platform-token-runtime/internal/httpapi/token_api.go`
- Modify: `platform-token-runtime/internal/httpapi/token_api_test.go`
- Modify: `platform-token-runtime/internal/token/audit_executable_test.go`

**Step 1: 写失败测试，固定 `/audit-events` 不能再返回 501**

Write:
```go
func TestTokenAPIAuditEventsReady(t *testing.T) {
	rt := service.NewInMemoryTokenRuntime(nil)
	auditor := service.NewMemoryAuditStore()
	api := NewTokenAPI(rt, auditor, time.Now)
	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/tokens/audit-events?limit=3", nil)
	req.Header.Set("X-Request-Id", "req-audit-ready")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

**Step 2: 运行测试，确认当前实现返回 501**

Run:
```bash
cd platform-token-runtime
go test ./internal/httpapi -run TestTokenAPIAuditEventsReady -v
```

Expected:
- 失败，当前返回 `501 AUDIT_QUERY_NOT_READY`。

**Step 3: 写最小实现**

Write:
- `audit_store.go`：提供内存审计事件查询实现，满足 `service.AuditEventQuerier`。
- `runtime_store.go`：抽象 token 持久化接口，让 runtime 与 HTTP API 不再直接绑定 map。
- `inmemory_runtime.go`：改为依赖 `RuntimeStore` + `AuditStore`，保留 dev 实现但不锁死后续持久化实现。

**Step 4: 补 direct service tests**

Write:
```go
func TestRuntimeStore_SaveAndLookup(t *testing.T) {}
func TestAuditStore_QueryByTokenID(t *testing.T) {}
```

**Step 5: 运行验证**

Run:
```bash
cd platform-token-runtime
go test ./internal/auth/service ./internal/httpapi ./internal/token ./cmd/platform-token-runtime
go test ./...
```

Expected:
- `audit-events` 返回 `200`。
- `internal/auth/service` 不再是无测试包。
- 审计查询不暴露 `access_token`。

**Step 6: Commit**

```bash
git add platform-token-runtime/internal/auth/service platform-token-runtime/internal/httpapi/token_api.go platform-token-runtime/internal/httpapi/token_api_test.go platform-token-runtime/internal/token/audit_executable_test.go
git commit -m "feat(token-runtime): implement audit query readiness"
```

---

### Task 4: 让 platform-token-runtime 具备可装配的生产运行时

**Files:**
- Create: `platform-token-runtime/internal/app/bootstrap.go`
- Create: `platform-token-runtime/internal/app/bootstrap_test.go`
- Modify: `platform-token-runtime/cmd/platform-token-runtime/main.go`
- Modify: `platform-token-runtime/README.md`

**Step 1: 写失败测试，固定 prod/staging 不是“直接拒绝启动”，而是“缺配置才拒绝启动”**

Write:
```go
func TestBuildRuntime_ProdRequiresConcreteStore(t *testing.T) {
	_, err := app.BuildRuntime(app.Config{Env: "prod"})
	if err == nil {
		t.Fatal("expected missing store error")
	}
	if !strings.Contains(err.Error(), "runtime store") {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

**Step 2: 运行测试，确认当前 main 只会 `log.Fatalf`**

Run:
```bash
cd platform-token-runtime
go test ./cmd/platform-token-runtime ./internal/app
```

Expected:
- 失败，因为当前没有 `internal/app`，也没有可注入的 runtime 构建逻辑。

**Step 3: 写最小实现**

Write:
- `internal/app/bootstrap.go`：把 env、runtime store、audit store、HTTP API 注册解耦。
- `cmd/platform-token-runtime/main.go`：只负责读取环境变量和优雅关闭。
- `README.md`：补充 dev 模式、staging/prod 模式的装配要求。

**Step 4: 运行验证**

Run:
```bash
cd platform-token-runtime
go test ./cmd/platform-token-runtime ./internal/app ./...
```

Expected:
- `dev` 可用内存实现。
- `staging/prod` 的失败原因变成“缺少具体 store 配置”，而不是“这个服务永远不可用于生产”。

**Step 5: Commit**

```bash
git add platform-token-runtime/cmd/platform-token-runtime/main.go platform-token-runtime/internal/app platform-token-runtime/README.md
git commit -m "refactor(token-runtime): extract runtime bootstrap"
```

---

### Task 5: 为 supply-api 告警链路接入持久化存储

**Files:**
- Create: `supply-api/internal/audit/repository/alert_repository.go`
- Create: `supply-api/internal/audit/repository/alert_repository_test.go`
- Modify: `supply-api/internal/httpapi/alert_api.go`
- Modify: `supply-api/cmd/supply-api/main.go`
- Modify: `supply-api/internal/audit/service/alert_service_test.go`

**Step 1: 写失败测试，固定 AlertAPI 由外部注入 store**

Write:
```go
func TestNewAlertAPI_UsesInjectedStore(t *testing.T) {
	store := newMockAlertStore()
	api := NewAlertAPI(service.NewAlertService(store))
	if api == nil {
		t.Fatal("expected api")
	}
}
```

**Step 2: 运行测试，确认当前 `NewAlertAPI()` 固定绑内存实现**

Run:
```bash
cd supply-api
go test ./internal/httpapi -run TestNewAlertAPI_UsesInjectedStore -v
```

Expected:
- 失败，因为当前构造函数不接收依赖。

**Step 3: 写最小实现**

Write:
- `alert_repository.go`：实现 PostgreSQL-backed `AlertStoreInterface`。
- `alert_api.go`：改为接收 `*service.AlertService` 或 `AlertStoreInterface` 注入。
- `cmd/supply-api/main.go`：`db != nil` 时装配 PostgreSQL 告警仓储；`db == nil` 时才允许显式内存告警仓储。

**Step 4: 运行验证**

Run:
```bash
cd supply-api
go test ./internal/audit/repository ./internal/audit/service ./internal/httpapi ./cmd/supply-api
```

Expected:
- 告警 API 不再偷偷使用内存实现。
- 入口装配和 HTTP 层边界清晰。

**Step 5: Commit**

```bash
git add supply-api/internal/audit/repository/alert_repository.go supply-api/internal/audit/repository/alert_repository_test.go supply-api/internal/httpapi/alert_api.go supply-api/cmd/supply-api/main.go supply-api/internal/audit/service/alert_service_test.go
git commit -m "feat(supply-api): persist audit alerts"
```

---

### Task 6: 消除 supply-api 的内联幂等降级路径

**Files:**
- Modify: `supply-api/internal/httpapi/supply_api.go`
- Modify: `supply-api/internal/httpapi/supply_api_test.go`
- Modify: `supply-api/cmd/supply-api/main.go`
- Modify: `supply-api/internal/middleware/idempotency.go`
- Modify: `supply-api/internal/middleware/idempotency_test.go`

**Step 1: 写失败测试，固定创建账户和提现只能走统一幂等中间件**

Write:
```go
func TestHandleCreateAccount_RequiresIdempotencyMiddleware(t *testing.T) {
	api := newSupplyAPIWithoutIdempotencyForTest()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", bytes.NewReader(validBody))
	rec := httptest.NewRecorder()

	api.handleCreateAccount(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when idempotency middleware is missing, got=%d", rec.Code)
	}
}
```

**Step 2: 运行测试，确认当前逻辑会回退到内联处理**

Run:
```bash
cd supply-api
go test ./internal/httpapi -run TestHandleCreateAccount_RequiresIdempotencyMiddleware -v
```

Expected:
- 失败，因为当前代码会继续执行业务逻辑。

**Step 3: 写最小实现**

Write:
- 删除 `handleCreateAccount` 和其他写操作中的内联幂等降级分支。
- `cmd/supply-api/main.go` 中明确：生产模式缺少幂等仓储时启动失败；开发模式可显式关闭相关写接口或返回明确错误。

**Step 4: 运行验证**

Run:
```bash
cd supply-api
go test ./internal/httpapi ./internal/middleware ./cmd/supply-api
go test ./...
```

Expected:
- 写接口的幂等路径单一。
- 不再存在“中间件缺失时静默换逻辑”的分叉。

**Step 5: Commit**

```bash
git add supply-api/internal/httpapi/supply_api.go supply-api/internal/httpapi/supply_api_test.go supply-api/cmd/supply-api/main.go supply-api/internal/middleware/idempotency.go supply-api/internal/middleware/idempotency_test.go
git commit -m "refactor(supply-api): remove inline idempotency fallback"
```

---

### Task 7: 对齐 supply-api 的 DDL 与仓储契约

**Files:**
- Create: `supply-api/sql/postgresql/supply_core_schema_v2.sql`
- Modify: `supply-api/internal/repository/account_integration_test.go`
- Modify: `supply-api/internal/repository/package_integration_test.go`
- Modify: `supply-api/internal/repository/settlement_integration_test.go`
- Modify: `supply-api/scripts/run_integration_tests.sh`
- Modify: `supply-api/README.md`

**Step 1: 写失败测试，禁止“字段缺失就 Skip”继续长期存在**

Write:
```go
func TestAccountRepositorySchemaContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}
	requireColumn(t, db, "supply_accounts", "version")
}
```

**Step 2: 运行集成测试，确认当前 schema contract 不完整**

Run:
```bash
cd supply-api
GOCACHE=/tmp/lijiaoqiao-go-cache-schema go test -tags=integration ./internal/repository -run 'SchemaContract|AccountRepository|PackageRepository|SettlementRepository' -v
```

Expected:
- 当前会出现字段缺失、脚本不完整或 `Skip`。

**Step 3: 写最小实现**

Write:
- `supply_core_schema_v2.sql`：把 `supply_accounts`、`supply_packages`、`supply_settlements`、幂等、版本字段和关键索引放到统一事实源。
- `run_integration_tests.sh`：优先应用统一 schema，再跑仓储集成测试。
- `README.md`：只指向实际存在的 DDL 文件。

**Step 4: 运行验证**

Run:
```bash
cd supply-api
bash scripts/run_integration_tests.sh ./internal/repository
```

Expected:
- 仓储集成测试以真实 schema 通过，不再依赖“字段缺失 -> Skip”。

**Step 5: Commit**

```bash
git add supply-api/sql/postgresql/supply_core_schema_v2.sql supply-api/internal/repository/account_integration_test.go supply-api/internal/repository/package_integration_test.go supply-api/internal/repository/settlement_integration_test.go supply-api/scripts/run_integration_tests.sh supply-api/README.md
git commit -m "feat(supply-api): align schema with repository contract"
```

---

### Task 8: 修复 supply-api 的外部集成断点和审计失败处理

**Files:**
- Modify: `supply-api/internal/sms/tencent_sms.go`
- Modify: `supply-api/internal/sms/aliyun_sms.go`
- Modify: `supply-api/internal/sms/factory.go`
- Modify: `supply-api/internal/sms/sms_test.go`
- Modify: `supply-api/internal/audit/service/batch_buffer.go`
- Modify: `supply-api/internal/audit/service/batch_buffer_test.go`

**Step 1: 写失败测试，固定 SMS verify 与 batch flush 失败语义**

Write:
```go
func TestTencentSMSService_VerifyCode_UsesCodeStore(t *testing.T) {}
func TestBatchBuffer_FlushHandlerFailureIsReported(t *testing.T) {}
```

**Step 2: 运行测试，确认当前分别是“未实现”和“吞错误”**

Run:
```bash
cd supply-api
go test ./internal/sms ./internal/audit/service -run 'VerifyCode|FlushHandlerFailure' -v
```

Expected:
- `VerifyCode` 返回 not implemented。
- `BatchBuffer` 没有可断言的失败策略。

**Step 3: 写最小实现**

Write:
- `tencent_sms.go` / `aliyun_sms.go`：统一走 code store 验证接口，至少实现与当前内存 code store 兼容的 verify 路径。
- `batch_buffer.go`：增加 flush error hook、计数器或显式返回，禁止静默吞错。

**Step 4: 运行验证**

Run:
```bash
cd supply-api
go test ./internal/sms ./internal/audit/service
go test ./...
```

Expected:
- 外部短信服务不再只有“发送能调、校验不能用”的半实现状态。
- 审计批处理失败具备可观测信号。

**Step 5: Commit**

```bash
git add supply-api/internal/sms/tencent_sms.go supply-api/internal/sms/aliyun_sms.go supply-api/internal/sms/factory.go supply-api/internal/sms/sms_test.go supply-api/internal/audit/service/batch_buffer.go supply-api/internal/audit/service/batch_buffer_test.go
git commit -m "fix(supply-api): close sms and audit flush gaps"
```

---

### Task 9: 最终收口文档与 CI，建立单一事实源

**Files:**
- Modify: `gateway/README.md`
- Modify: `platform-token-runtime/README.md`
- Modify: `supply-api/README.md`
- Modify: `scripts/ci/repo_integrity_check.sh`
- Modify: `docs/plans/2026-04-14-repo-integrity-refactor-plan.md`

**Step 1: 写失败检查，固定 README 必须引用真实文件和真实命令**

Run:
```bash
rg -n "platform_core_schema_v1\\.sql|TODO|not implemented|AUDIT_QUERY_NOT_READY" gateway platform-token-runtime supply-api
```

Expected:
- 仍能看到待清理的历史痕迹。

**Step 2: 清理文档和剩余误导性描述**

Write:
- 每个模块 README 都只写当前真实运行方式、当前真实依赖、当前真实验证命令。
- 将 `scripts/ci/repo_integrity_check.sh` 作为所有报告和发布前检查的统一入口。

**Step 3: 运行最终验证**

Run:
```bash
git diff --check
bash scripts/ci/repo_integrity_check.sh
cd supply-api && bash scripts/run_integration_tests.sh ./internal/repository
```

Expected:
- 仓库格式通过。
- 三个服务的单元与 E2E 基线通过。
- `supply-api` 仓储集成测试通过。

**Step 4: Commit**

```bash
git add gateway/README.md platform-token-runtime/README.md supply-api/README.md scripts/ci/repo_integrity_check.sh docs/plans/2026-04-14-repo-integrity-refactor-plan.md
git commit -m "docs(repo): align readmes and integrity workflow"
```

---

## 收尾标准

- `gateway` 不再有硬编码 provider 注册，组合根拆分完成。
- `platform-token-runtime` 的审计查询可用，运行时可装配，不再是“只能 dev 运行”的组件。
- `supply-api` 的告警、幂等、DDL、短信校验和审计批处理全部有单一实现路径。
- `scripts/ci/repo_integrity_check.sh` 成为仓库统一验证入口。
- README、DDL、测试、代码对同一事实达成一致。

## 明确不纳入本轮

- `llm-gateway-competitors/` 下的竞品快照。
- 任何新的产品功能扩展。
- UI 或接口风格优化。
- 非一方服务的历史归档清理。

## 风险提示

- `supply-api` 的 DDL 收口会触及集成测试和迁移脚本，必须在独立小提交中推进，不能与业务重构混提。
- `platform-token-runtime` 一旦引入持久化抽象，`gateway` 的 remote introspection 契约也要一起回归验证。
- 任何“开发模式降级”都必须改成显式、可观察、可测试，而不是静默回退。

Plan complete and saved to `docs/plans/2026-04-14-repo-integrity-refactor-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** - 我在当前会话按任务逐个实现、逐个验证、逐个提交。

**2. Parallel Session (separate)** - 新开会话，按 `executing-plans` 工作流批量执行。
