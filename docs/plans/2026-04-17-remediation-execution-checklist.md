# Remediation Execution Checklist Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 基于 2026-04-17 的纠偏结果，按 `P0 -> P1 -> P2` 顺序恢复三块核心服务的可信构建与测试链路，并补齐当前真正阻塞上线的能力缺口。

**Architecture:** 先修“证据链”再修“能力链”。`P0` 只做构建、测试、持久化这类阻塞项，不混入新功能；`P1` 处理补偿、IAM、提现、默认配置等已实现但未闭环的能力；`P2` 收口模型清单、路由策略、SQL 基线和命名一致性，避免后续继续误判完成度。

**Tech Stack:** Go, Bash, PostgreSQL, go test, integration scripts

---

## 执行规则

1. `P0` 未全部完成前，不开始任何 `P1/P2` 任务。
2. 每个任务都要先复现问题，再做最小修复，再跑无缓存回归。
3. 每个任务单独提交，不跨服务混改。
4. 所有验证默认使用 `go test -count=1`，禁止依赖缓存绿灯。
5. 任何“已有代码但未接入”的能力，都必须在本轮被收敛为“明确上线”或“明确关闭”，不再维持模糊状态。

## 交付顺序

1. `P0-1` 恢复 `supply-api` 编译
2. `P0-2` 恢复 `gateway` 无缓存测试
3. `P0-3` 恢复 `platform-token-runtime` 无缓存测试
4. `P0-4` 把仓库验证脚本升级为可信基线
5. `P0-5` 抽象 `platform-token-runtime` store 接口
6. `P0-6` 落地 `platform-token-runtime` 最小 PostgreSQL 持久化实现
7. `P1-1` 收敛 `supply-api` 补偿执行器，先禁止假成功，再补真实回滚
8. `P1-2` 将 IAM 收敛为显式能力：默认关闭，可配置启用并接入主服务
9. `P1-3` 明确提现/SMS 的正式接入路径
10. `P1-4` 收紧 `gateway` 默认密钥和默认 CORS
11. `P2-1` 让 `gateway /v1/models` 反映真实 provider 配置
12. `P2-2` 明确高级路由策略是否进入主链路
13. `P2-3` 收敛 SQL 基线与命名一致性

### Task 1: 修复 `supply-api` KMS 编译阻塞 [P0]

**Files:**
- Modify: `supply-api/internal/security/kms_service.go`
- Modify: `supply-api/internal/security/kms_service_test.go`

**Step 1: 先复现当前阻塞**

Run: `cd "supply-api" && go test -count=1 ./internal/security -run 'Test(DeriveDEK|KMSService_EncryptDecrypt)' -v`
Expected: FAIL，报错包含 `package crypto/hkdf is not in std`

**Step 2: 改成当前工具链可编译的 HKDF 实现**

- 保持 `deriveDEK` 的 `HKDF-SHA256` 语义不变
- 不回退到旧的固定盐 + 直接哈希实现
- 优先改为 `golang.org/x/crypto/hkdf`，仓库已存在该依赖

**Step 3: 运行安全包回归**

Run: `cd "supply-api" && go test -count=1 ./internal/security -v`
Expected: PASS

**Step 4: 运行整仓单元回归**

Run: `cd "supply-api" && go test -count=1 ./...`
Expected: PASS；若出现新的真实失败，继续在本任务内清零

**Step 5: Commit**

```bash
git add supply-api/internal/security/kms_service.go supply-api/internal/security/kms_service_test.go
git commit -m "fix(supply-api): restore hkdf compatibility on current toolchain"
```

### Task 2: 同步 `gateway` 测试到当前 `BuildMux` 和错误脱敏语义 [P0]

**Files:**
- Modify: `gateway/cmd/gateway/main_test.go`
- Modify: `gateway/internal/handler/handler_test.go`
- Verify: `gateway/internal/app/bootstrap.go`
- Verify: `gateway/internal/middleware/cors.go`

**Step 1: 复现当前测试失配**

Run: `cd "gateway" && go test -count=1 ./cmd/gateway ./internal/handler -v`
Expected:
- `cmd/gateway` 因 `BuildMux` 参数数量不匹配而失败
- `internal/handler` 因旧断言仍期待原始错误文案而失败

**Step 2: 修正 `BuildMux` 调用方**

- 在 `gateway/cmd/gateway/main_test.go` 中为 `app.BuildMux(...)` 传入显式 `corsConfig`
- 使用 `middleware.DefaultCORSConfig()`，不要在测试里复制一份默认值

**Step 3: 修正错误响应断言**

- `COMMON_INVALID_REQUEST` 应断言为对外消息 `invalid request`
- 保留 `X-Request-ID`、`gateway_error`、错误码的断言
- 为 `sanitizeRequestID` 增加覆盖非法字符的断言，防止旧问题回归

**Step 4: 跑 focused 回归**

Run: `cd "gateway" && go test -count=1 ./cmd/gateway ./internal/handler ./internal/app -v`
Expected: PASS

**Step 5: 跑整仓无缓存回归**

Run: `cd "gateway" && go test -count=1 ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add gateway/cmd/gateway/main_test.go gateway/internal/handler/handler_test.go
git commit -m "test(gateway): realign mux and error response assertions"
```

### Task 3: 同步 `platform-token-runtime` 测试到当前鉴权与中间件签名 [P0]

**Files:**
- Modify: `platform-token-runtime/internal/auth/middleware/token_auth_middleware_test.go`
- Modify: `platform-token-runtime/internal/httpapi/token_api_test.go`
- Verify: `platform-token-runtime/internal/auth/middleware/query_key_reject_middleware.go`
- Verify: `platform-token-runtime/internal/httpapi/token_api.go`

**Step 1: 复现当前失败**

Run: `cd "platform-token-runtime" && go test -count=1 ./internal/auth/middleware ./internal/httpapi -v`
Expected:
- `QueryKeyRejectMiddleware` 调用参数不匹配
- `audit-events` 相关测试因未带 `Authorization` 失败

**Step 2: 同步中间件签名**

- 所有 `QueryKeyRejectMiddleware(...)` 测试调用都补齐 `trustedProxies`
- 默认传 `nil` 或空切片，不要在测试中引入新的代理策略

**Step 3: 同步 `audit-events` 语义**

- 先通过 `/issue` 获取真实 `access_token`
- 查询 `/api/v1/platform/tokens/audit-events` 时补 `Authorization: Bearer <token>`
- 额外新增一个负例，明确“无 Bearer 头返回 401”是当前正确行为

**Step 4: 跑 focused 回归**

Run: `cd "platform-token-runtime" && go test -count=1 ./internal/auth/middleware ./internal/httpapi -v`
Expected: PASS

**Step 5: 跑整仓无缓存回归**

Run: `cd "platform-token-runtime" && go test -count=1 ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add platform-token-runtime/internal/auth/middleware/token_auth_middleware_test.go platform-token-runtime/internal/httpapi/token_api_test.go
git commit -m "test(token-runtime): align auth tests with current http behavior"
```

### Task 4: 升级仓库验证脚本，禁止缓存绿灯掩盖问题 [P0]

**Files:**
- Modify: `scripts/ci/repo_integrity_check.sh`
- Verify: `scripts/ci/lib/verification_common.sh`

**Step 1: 先确认脚本盲区**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: 当前脚本无法稳定暴露已知问题，且没有包含 `integration` 仓储基线

**Step 2: 升级 Go 测试命令**

- 把三块核心服务统一切到 `go test -count=1 ./...`
- 不新增复杂 helper，优先直接改命令参数，保持脚本简单

**Step 3: 纳入 `supply-api` 仓储集成基线**

- 在 `supply-api` 单元测试后加入：
  `bash scripts/run_integration_tests.sh ./internal/repository`
- 保留现有 `-tags=e2e ./e2e`，但不要把它误称为完整上线验证

**Step 4: 运行完整脚本**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: PASS；后续任何一块服务回归都必须在这里稳定暴露

**Step 5: Commit**

```bash
git add scripts/ci/repo_integrity_check.sh
git commit -m "ci: make repo integrity check uncached and integration-aware"
```

### Task 5: 为 `platform-token-runtime` 引入 store 抽象，解除对内存实现的硬编码 [P0]

**Files:**
- Modify: `platform-token-runtime/internal/auth/service/inmemory_runtime.go`
- Modify: `platform-token-runtime/internal/auth/service/runtime_store.go`
- Modify: `platform-token-runtime/internal/auth/service/audit_store.go`
- Modify: `platform-token-runtime/internal/app/bootstrap.go`
- Modify: `platform-token-runtime/internal/app/bootstrap_test.go`
- Create: `platform-token-runtime/internal/auth/service/store_contract_test.go`

**Step 1: 先锁住接口化目标**

- 新增 contract test，要求：
  - 内存 runtime store 满足统一 `RuntimeStore` 接口
  - 内存 audit store 满足统一 `AuditStore` / `AuditQuerier` 接口
  - `BuildRuntime(Config{Env:"prod"})` 必须接收抽象 store，而不是具体内存类型

**Step 2: 提炼最小接口**

- `RuntimeStore` 至少覆盖：`Save`、`GetByTokenID`、`GetByAccessToken`、`LookupIdempotency`
- `AuditStore` 至少覆盖：`Emit`
- `AuditQuerier` 覆盖：`QueryEvents`
- `InMemoryTokenRuntime` 只依赖抽象，不再写死 `*InMemoryRuntimeStore`

**Step 3: 调整 bootstrap 输入**

- `app.Config` 从具体指针改为抽象接口
- `dev` 下仍自动注入内存实现
- `staging/prod` 继续要求显式注入，但错误信息不变

**Step 4: 跑 focused 回归**

Run: `cd "platform-token-runtime" && go test -count=1 ./internal/auth/service ./internal/app -v`
Expected: PASS

**Step 5: Commit**

```bash
git add platform-token-runtime/internal/auth/service/inmemory_runtime.go platform-token-runtime/internal/auth/service/runtime_store.go platform-token-runtime/internal/auth/service/audit_store.go platform-token-runtime/internal/app/bootstrap.go platform-token-runtime/internal/app/bootstrap_test.go platform-token-runtime/internal/auth/service/store_contract_test.go
git commit -m "refactor(token-runtime): abstract runtime and audit stores"
```

### Task 6: 落地 `platform-token-runtime` 最小 PostgreSQL 持久化实现 [P0]

**Files:**
- Create: `platform-token-runtime/internal/auth/service/postgres_runtime_store.go`
- Create: `platform-token-runtime/internal/auth/service/postgres_runtime_store_test.go`
- Create: `platform-token-runtime/internal/auth/service/postgres_audit_store.go`
- Create: `platform-token-runtime/internal/auth/service/postgres_audit_store_test.go`
- Modify: `platform-token-runtime/internal/app/bootstrap.go`
- Modify: `platform-token-runtime/cmd/platform-token-runtime/main.go`
- Modify: `platform-token-runtime/cmd/platform-token-runtime/main_test.go`
- Modify: `sql/postgresql/token_runtime_schema_v1.sql`
- Modify: `platform-token-runtime/README.md`

**Step 1: 先写 prod/staging 可启动的失败测试**

- 在 `main_test.go` / `bootstrap_test.go` 中增加：
  - 提供 `TOKEN_RUNTIME_DATABASE_URL` 时，`prod` 不再因“缺少 store”直接失败
  - 缺少 DSN 时仍快速失败

**Step 2: 实现 PostgreSQL runtime store**

- 表结构复用 `auth_token_runtime` / `auth_token_audit_events`
- `Save` 要覆盖刷新后的 `expires_at` 与吊销状态
- `LookupIdempotency` 必须保持和内存实现一致的幂等语义

**Step 3: 实现 PostgreSQL audit store**

- `Emit` 写入 `auth_token_audit_events`
- `QueryEvents` 保持当前 `request_id` / `token_id` / `subject_id` / `event_name` / `result_code` 过滤语义
- 默认 `limit`、最大 `limit` 与当前内存实现保持一致

**Step 4: 接入启动链路**

- `cmd/platform-token-runtime/main.go` 增加 DSN 解析
- `staging/prod` 下优先装配 PostgreSQL store
- `dev` 仍保持零配置内存启动

**Step 5: 跑 focused 回归**

Run: `cd "platform-token-runtime" && go test -count=1 ./internal/auth/service ./internal/app ./cmd/platform-token-runtime -v`
Expected: PASS

**Step 6: 跑整仓回归**

Run: `cd "platform-token-runtime" && go test -count=1 ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add platform-token-runtime/internal/auth/service/postgres_runtime_store.go platform-token-runtime/internal/auth/service/postgres_runtime_store_test.go platform-token-runtime/internal/auth/service/postgres_audit_store.go platform-token-runtime/internal/auth/service/postgres_audit_store_test.go platform-token-runtime/internal/app/bootstrap.go platform-token-runtime/cmd/platform-token-runtime/main.go platform-token-runtime/cmd/platform-token-runtime/main_test.go sql/postgresql/token_runtime_schema_v1.sql platform-token-runtime/README.md
git commit -m "feat(token-runtime): add postgres-backed runtime and audit stores"
```

### Task 7: 收敛 `supply-api` 补偿执行器，先禁止假成功，再补真实回滚 [P1]

**Files:**
- Modify: `supply-api/internal/compensation/compensation.go`
- Modify: `supply-api/internal/compensation/compensation_test.go`
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime_test.go`
- Create: `supply-api/internal/compensation/payload_test.go`

**Step 1: 先阻断“记录日志后返回 nil”**

- 为四类补偿操作新增测试：
  - payload 不完整时返回显式错误
  - handler 未注入依赖时返回显式错误
  - 绝不允许 no-op 成功

**Step 2: 引入显式 payload 契约**

- 给 `account.create`、`package.publish`、`settlement.withdraw`、`quota.deduct` 定义最小 payload 解析规则
- 解析失败必须返回错误，让 `CompensationProcessor` 进入 `retrying/manual_required`

**Step 3: 调整 executor 构造方式**

- `NewDefaultCompensationExecutor()` 改为接收依赖，不再硬编码无状态空实现
- `background.go` 负责把真实依赖传入 executor

**Step 4: 第一阶段只做 fail-fast**

- 如果当前仓储层尚不具备安全回滚能力，先返回 `not implemented for production rollback`
- 目标是杜绝假成功，而不是在一个任务里强行补齐所有逆向操作

**Step 5: 跑 focused 回归**

Run: `cd "supply-api" && go test -count=1 ./internal/compensation ./internal/domain ./internal/app -run 'Test(DefaultCompensationExecutor|NewCompensationProcessor|BuildRuntime)' -v`
Expected: PASS

**Step 6: Commit**

```bash
git add supply-api/internal/compensation/compensation.go supply-api/internal/compensation/compensation_test.go supply-api/internal/app/background.go supply-api/internal/app/runtime_test.go supply-api/internal/compensation/payload_test.go
git commit -m "fix(supply-api): make compensation executor fail closed"
```

### Task 8: 将 IAM 收敛为显式能力，默认关闭，可配置启用并接入主服务 [P1]

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/runtime_test.go`
- Modify: `supply-api/internal/app/bootstrap_test.go`
- Modify: `supply-api/internal/config/config.go`
- Modify: `supply-api/README.md`
- Verify: `supply-api/internal/iam/repository/iam_repository.go`
- Verify: `supply-api/internal/iam/service/iam_service_db.go`
- Verify: `supply-api/internal/iam/handler/iam_handler.go`

**Step 1: 明确产品边界**

- 新增 `server.iam_enabled` 配置，默认 `false`
- README 明确说明：未启用时 IAM 不属于对外交付面

**Step 2: 启用态接入 DB-backed IAM**

- `runtime.go` 在 DB 可用且 `iam_enabled=true` 时，装配：
  - `iamrepo.NewPostgresIAMRepository(...)`
  - `iamservice.NewDatabaseIAMService(...)`
  - `iamhandler.NewIAMHandler(...)`
- `bootstrap.go` 在 handler 非空时注册 IAM 路由

**Step 3: 加双态测试**

- `iam_enabled=false` 时，请求 IAM 路由返回 `404`
- `iam_enabled=true` 且 DB 可用时，路由注册成功
- `prod` 下若显式启用 IAM 但 DB 不可用，启动必须失败

**Step 4: 跑 focused 回归**

Run: `cd "supply-api" && go test -count=1 ./internal/app ./internal/iam/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/app/bootstrap.go supply-api/internal/app/runtime_test.go supply-api/internal/app/bootstrap_test.go supply-api/internal/config/config.go supply-api/README.md
git commit -m "feat(supply-api): gate and wire iam routes explicitly"
```

### Task 9: 明确提现/SMS 正式接入路径，避免长期门禁悬置 [P1]

**Files:**
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/domain/settlement.go`
- Modify: `supply-api/internal/config/config.go`
- Modify: `supply-api/internal/httpapi/supply_api_test.go`
- Modify: `supply-api/README.md`

**Step 1: 先锁住当前边界**

- 为以下行为补测试：
  - 未配置真实 `SMSVerifier` 时，提现始终不可启用
  - `prod` 下打开 `settlement.withdraw_enabled` 但缺少 SMS 配置必须启动失败

**Step 2: 引入显式注入点**

- `runtime.go` 改为根据配置选择：
  - 无 SMS 配置：继续使用关闭态
  - 有 SMS 配置：装配 `NewSettlementServiceWithSMS(...)`

**Step 3: 让配置语义和 README 对齐**

- 保留当前 `prod` 保护
- README 写清楚“接口存在 != 能力已上线”
- 配置项命名和错误提示统一到“SMS 未就绪，提现不可用”

**Step 4: 跑 focused 回归**

Run: `cd "supply-api" && go test -count=1 ./internal/domain ./internal/app ./internal/httpapi -run 'Test(NewSettlementService|BuildRuntime|Withdraw)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/internal/app/runtime.go supply-api/internal/domain/settlement.go supply-api/internal/config/config.go supply-api/internal/httpapi/supply_api_test.go supply-api/README.md
git commit -m "feat(supply-api): make withdraw readiness depend on sms wiring"
```

### Task 10: 收紧 `gateway` 默认密钥和默认 CORS [P1]

**Files:**
- Modify: `gateway/internal/config/config.go`
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `gateway/internal/middleware/cors_test.go`
- Modify: `gateway/internal/app/bootstrap_test.go`
- Modify: `gateway/README.md`

**Step 1: 先补回归测试**

- `prod` 下未显式设置 `PASSWORD_ENCRYPTION_KEY` 必须失败
- `prod` 下 `CORS_ALLOW_ORIGINS` 为空或 `*` 必须失败
- `dev` 下仍允许默认值启动

**Step 2: 删除不安全默认行为**

- `config.go` 不再在全局层面静默回退到默认密钥
- `bootstrap.go` 负责基于环境做显式校验
- CORS 白名单只在 `dev` 保留 `*`

**Step 3: 跑 focused 回归**

Run: `cd "gateway" && go test -count=1 ./internal/config ./internal/app ./internal/middleware -v`
Expected: PASS

**Step 4: 跑整仓回归**

Run: `cd "gateway" && go test -count=1 ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add gateway/internal/config/config.go gateway/internal/app/bootstrap.go gateway/internal/middleware/cors_test.go gateway/internal/app/bootstrap_test.go gateway/README.md
git commit -m "fix(gateway): fail closed on secret and cors defaults"
```

### Task 11: 让 `gateway /v1/models` 反映真实 provider 配置 [P2]

**Files:**
- Modify: `gateway/internal/handler/handler.go`
- Modify: `gateway/internal/handler/handler_test.go`
- Verify: `gateway/internal/router/router.go`
- Verify: `gateway/internal/adapter/openai_adapter.go`

**Step 1: 先补行为测试**

- 当路由器注册两个 provider 且支持模型不同，`/v1/models` 返回合并后的真实模型列表
- 去重、稳定排序、空 provider 时返回空数组

**Step 2: 把硬编码列表替换为动态枚举**

- 从已注册 provider 的 `SupportedModels()` 生成输出
- 不在 handler 中维持任何静态模型常量

**Step 3: 跑 focused 回归**

Run: `cd "gateway" && go test -count=1 ./internal/handler ./internal/router ./internal/adapter -v`
Expected: PASS

**Step 4: Commit**

```bash
git add gateway/internal/handler/handler.go gateway/internal/handler/handler_test.go
git commit -m "feat(gateway): serve models from registered providers"
```

### Task 12: 明确高级路由策略是否进入主链路 [P2]

**Files:**
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `gateway/internal/app/bootstrap_test.go`
- Modify: `gateway/README.md`
- Verify: `gateway/internal/router/strategy/cost_based.go`
- Verify: `gateway/internal/router/strategy/cost_aware.go`
- Verify: `gateway/internal/router/strategy/fallback.go`

**Step 1: 先做边界决策**

- 如果本期不交付高级策略：
  - README 明确写成实验能力
  - `resolveStrategy` 只暴露当前主链路支持的策略
- 如果本期交付：
  - 必须把策略接入 `bootstrap`
  - 必须补覆盖真实生效路径的测试

**Step 2: 推荐做法**

- 本轮先收敛为“实验能力未接入主链路”
- 删除对外文档中的完成态表述
- 保证启动链只暴露已验证策略

**Step 3: 跑 focused 回归**

Run: `cd "gateway" && go test -count=1 ./internal/app ./internal/router/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add gateway/internal/app/bootstrap.go gateway/internal/app/bootstrap_test.go gateway/README.md
git commit -m "docs(gateway): clarify advanced routing strategy status"
```

### Task 13: 收敛 SQL 基线与命名一致性 [P2]

**Files:**
- Modify: `supply-api/sql/postgresql/supply_core_schema_v2.sql`
- Modify: `sql/postgresql/platform_core_schema_v1.sql`
- Modify: `sql/postgresql/token_runtime_schema_v1.sql`
- Modify: `review/REPORT_CORRECTION_2026-04-17.md`
- Create: `docs/plans/2026-04-17-schema-boundary-notes.md`

**Step 1: 修正 `supply_core_schema_v2.sql` 的状态约束**

- 给 `supply_accounts`、`supply_packages`、`supplier_settlements` 的 `status` 补 `CHECK`
- 约束值必须来自当前代码使用的真实状态集合，禁止拍脑袋扩枚举

**Step 2: 明确 `audit_events` 边界**

- `platform_core_schema_v1.sql` 中的 `audit_events` 属平台侧
- `token_runtime_schema_v1.sql` 中的 `auth_token_audit_events` 属 token runtime
- 说明文档明确“谁是当前基线，谁是历史/并列 schema”

**Step 3: 收敛命名一致性**

- 统一新代码中的 `ClientIP` / `SourceIP` 选择
- 若无法一次性全改，至少在文档中明确后续统一方向

**Step 4: 运行 SQL 相关验证**

Run: `cd "supply-api" && bash scripts/run_integration_tests.sh ./internal/repository`
Expected: PASS

**Step 5: Commit**

```bash
git add supply-api/sql/postgresql/supply_core_schema_v2.sql sql/postgresql/platform_core_schema_v1.sql sql/postgresql/token_runtime_schema_v1.sql review/REPORT_CORRECTION_2026-04-17.md docs/plans/2026-04-17-schema-boundary-notes.md
git commit -m "docs(sql): clarify active schema boundaries and status constraints"
```

## 总验收

**Step 1: 核心服务无缓存回归**

Run:

```bash
cd "gateway" && go test -count=1 ./...
cd "platform-token-runtime" && go test -count=1 ./...
cd "supply-api" && go test -count=1 ./...
```

Expected: PASS

**Step 2: 仓储基线**

Run: `cd "supply-api" && bash scripts/run_integration_tests.sh ./internal/repository`
Expected: PASS

**Step 3: 仓库统一验证**

Run: `bash "scripts/ci/repo_integrity_check.sh"`
Expected: PASS

**Step 4: 差异检查**

Run: `git diff --check`
Expected: no output
