# 2026-04-20 真实验证整改任务单

## 目的

把以下两份真实验证报告合并为一份可执行整改任务单，并按 `P0 / P1 / P2`、模块负责人、依赖顺序、验证命令、完成标准拆解成可以直接执行的任务：

- [REAL_ENV_REVIEW_AND_VALIDATION_REPORT_2026-04-20.md](/home/long/project/立交桥/review/REAL_ENV_REVIEW_AND_VALIDATION_REPORT_2026-04-20.md)
- [API_MATRIX_VALIDATION_REPORT_2026-04-20.md](/home/long/project/立交桥/review/API_MATRIX_VALIDATION_REPORT_2026-04-20.md)

本任务单只覆盖**已证实缺陷**。不把以下事项误写成整改目标：

- `gateway` 当前主链路通过的接口
- `supply-api` 告警链路
- 提现因 SMS 未就绪而 `503` 的 fail-closed 行为
- 不属于仓库业务代码的问题，例如临时种子数据中的空格格式问题

## 执行收口状态

截至 2026-04-20 本轮整改执行收口时，本任务单列出的 `13` 个任务已全部完成，并已分别提交到当前分支。

对应提交如下：

- `414ecbb` `P0-TR-01`
- `9dba094` `P0-SA-01`
- `50f0cc8` `P0-SA-02`
- `00ff636` `P0-SA-03`
- `1c088e2` `P0-SA-04`
- `319d9e1` `P1-AUD-01`
- `5661696` `P1-IAM-01`
- `a109a68` `P1-IAM-02`
- `79d9b87` `P1-IAM-03`
- `a1555c0` `P1-IAM-04`
- `eab029a` `P2-API-01`
- `b879906` `P2-QA-01`

总验收已实际通过：

- `bash scripts/ci/repo_integrity_check.sh`
- `bash scripts/ci/supply_domain_stability_check.sh 20`
- `git diff --check`

说明：

- 本文保留“任务拆解”原始结构，作为整改执行基线。
- 当前是否已修复，应以以上提交和最终验收结果为准。

## 2026-04-20 复核确认

截至 2026-04-20 本次全面复核结束时，本任务单覆盖的 `13` 个已证实问题已再次按“整仓基线 + focused 回归 + 仓储集成”口径核验，当前未发现残留未修复项。

本次复核实际执行并通过的关键命令如下：

- `bash scripts/ci/repo_integrity_check.sh`
- `bash scripts/ci/supply_domain_stability_check.sh 20`
- `cd "/home/long/project/立交桥/platform-token-runtime" && go test -count=1 ./internal/auth/service -run 'Test(PostgresRuntimeStore_SavePreservesExistingFingerprintWhenAccessTokenMissing|InMemoryTokenRuntimeWithPostgresStore_RefreshAndRevokePersistLifecycle)$' -v`
- `cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/httpapi -run 'TestSupplyAPI_(ActivateAccount_ConcurrencyConflict|PublishPackage_ConcurrencyConflict|ClonePackage_UnexpectedCreateFailureReturnsInternalServerError|CancelSettlement_ConcurrencyConflict|ActivateAccount_NotFound|PublishPackage_NotFound|ClonePackage_NotFound|CancelSettlement_NotFound)$' -v`
- `cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/iam/... -run 'Test.*(AssignRole|RevokeRole|ListRoles|GetUserRoles|UpdateRole)' -v`
- `cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/domain ./internal/httpapi -run 'Test.*(Activate|Suspend|Delete|Publish|Pause|Unlist|Clone|Cancel)' -v`
- `cd "/home/long/project/立交桥/supply-api" && bash scripts/run_integration_tests.sh ./internal/iam/repository`
- `cd "/home/long/project/立交桥/supply-api" && bash scripts/run_integration_tests.sh ./internal/audit/...`

复核结论：

- 任务单内 `P0 / P1 / P2` 的 `13` 个问题已全部真实解决。
- 当前未复现 `platform-token-runtime` PostgreSQL-backed `refresh/revoke` 失败。
- 当前未复现 `supply-api` 幂等锁、套餐创建、账号/套餐状态流转、审计契约、IAM DDL 与 DB-backed IAM 缺陷。
- `supply-api/internal/domain` 连续 `20` 轮无缓存复跑通过，之前一次性波动信号本轮未复现。

任务单外的后续治理补充：

- 结构化日志统一已在三套服务入口收口：`supply-api`、[gateway main.go](/home/long/project/立交桥/gateway/cmd/gateway/main.go) 与 [platform-token-runtime main.go](/home/long/project/立交桥/platform-token-runtime/cmd/platform-token-runtime/main.go) 现在都输出统一 JSON 结构化日志。该事项不属于本文 `13` 个缺陷，但已作为项目统一性治理一并完成。

## 范围结论

截至本任务单生成时，真实验证已确认的问题可归纳为 13 项：

### P0

1. `platform-token-runtime` PostgreSQL-backed `refresh` / `revoke` 失败
2. `supply-api` 幂等锁 `ON CONFLICT` 与表结构不匹配
3. `supply-api` 套餐创建 SQL 占位符数量错误
4. `supply-api` 账号激活 / 暂停存在确定性乐观锁错误
5. `supply-api` 套餐发布 / 暂停 / 下架存在确定性乐观锁错误
6. `supply-api` 套餐读取字段映射错误，放大后续更新失败

### P1

7. `supply-api` 审计仓储与 `audit_events` 表结构不一致，读写都失败
8. `IAM` 原始 DDL 在干净数据库上无法落地
9. `IAM` 角色列表与用户角色查询对可空字段扫描不安全
10. `IAM` 角色更新把空字符串写入 `INET` 列
11. `IAM` 角色分配未传 `granted_by`，触发外键失败

### P2

12. 多个 `supply-api` handler 对内部错误返回错误的 HTTP 语义，导致真实冲突被包装成 `404`
13. `supply-api/internal/domain` 存在一次未复现的不稳定失败信号，需要专项复查和压测封口

## 覆盖校验

以下映射用于保证两份原始报告中的**全部已证实缺陷**都已经落进整改任务，不留“报告里写了、任务单里没拆”的空档：

| 已证实缺陷 | 对应任务 |
| --- | --- |
| `platform-token-runtime` PostgreSQL-backed `refresh` / `revoke` 失败 | `P0-TR-01` |
| `supply-api` 账号创建被幂等锁 DDL/SQL 契约阻断 | `P0-SA-01` |
| `supply-api` 套餐创建 / clone 被 `INSERT` 列数不匹配阻断 | `P0-SA-02` |
| `supply-api` 账号 `activate` / `suspend` 的乐观锁错误 | `P0-SA-03` |
| `supply-api` 套餐 `publish` / `pause` / `unlist` 的乐观锁错误 | `P0-SA-04` |
| `supply-api` 套餐读取字段映射错误 | `P0-SA-04` |
| `supply-api` 审计写入与查询受 `audit_events` 契约失配影响 | `P1-AUD-01` |
| `IAM` 原始 DDL 无法在干净库初始化 | `P1-IAM-01` |
| `IAM` 角色列表 / 用户角色查询的 null scan | `P1-IAM-02` |
| `IAM` 角色更新把空字符串写入 `INET` | `P1-IAM-03` |
| `IAM` 角色分配缺失 `granted_by` 触发外键失败 | `P1-IAM-04` |
| `supply-api` 多个 handler 错误语义错误 | `P2-API-01` |
| `supply-api/internal/domain` 一次性波动失败信号 | `P2-QA-01` |

说明：

- 两份报告中没有其他已证实的 `gateway` 缺陷进入整改范围；`gateway` 当前角色是总验收联调方。
- `gateway` 当前没有直接整改任务，但必须纳入最终总验收。
- `platform-token-runtime` 当前没有新增查询链路缺陷，整改重点只在变更链路。
- 提现 `503` 属于条件关闭，不属于待修复缺陷，因此未拆入任务。

## 负责人视角

| 负责人视角 | 覆盖模块 | 负责内容 |
| --- | --- | --- |
| `Token Runtime 负责人` | `platform-token-runtime` | Token 生命周期写路径、PostgreSQL store、一致性回归 |
| `Supply Domain/Repository 负责人` | `supply-api` | 幂等、账号、套餐、结算读写、仓储与领域一致性 |
| `Audit/IAM/SQL 负责人` | `supply-api/sql`、`internal/audit`、`internal/iam` | DDL、审计表契约、IAM schema、DB-backed IAM |
| `QA/CI 负责人` | 验证脚本、矩阵回归 | 真实复现、无缓存回归、环境验收、波动性复查 |
| `Gateway 负责人` | `gateway` | 最终联调验收，无直接缺陷修复任务 |

## 执行规则

1. `P0` 未全部完成前，不开始任何 `P1` 代码修复。
2. 每个任务都必须先写或补“能稳定复现缺陷”的测试，再修代码。
3. 每个任务都必须跑 focused 回归和至少一条跨模块回归。
4. 所有 Go 测试默认使用 `go test -count=1`。
5. 涉及 PostgreSQL 契约的问题，必须同时修：
   - 仓库代码
   - 基线 schema
   - 干净库初始化路径
   - 已有库升级路径
6. 任何接口级缺陷修复后，都要补一条真实 HTTP 或集成测试，避免只在单元层“看起来正确”。

## 依赖顺序

1. `P0-TR-01` 修复 token runtime `refresh/revoke`
2. `P0-SA-01` 修复 idempotency DDL/仓储契约
3. `P0-SA-02` 修复 package create SQL
4. `P0-SA-03` 修复 account lifecycle 乐观锁
5. `P0-SA-04` 修复 package lifecycle 乐观锁与字段映射
6. `P1-AUD-01` 统一 `audit_events` 契约
7. `P1-IAM-01` 修复 IAM DDL 初始化
8. `P1-IAM-02` 修复 IAM null scan
9. `P1-IAM-03` 修复 IAM update role `INET` 写入
10. `P1-IAM-04` 修复 IAM assign role `granted_by`
11. `P2-API-01` 统一 handler 错误语义
12. `P2-QA-01` 处理 `internal/domain` 波动性信号
13. 执行总验收

---

## P0 任务

### P0-TR-01 修复 `platform-token-runtime` 的 PostgreSQL `refresh` / `revoke`

**负责人：** `Token Runtime 负责人`

**目标：**

- 让 `refresh` 与 `revoke` 在 PostgreSQL-backed 模式下返回 `200`
- 让 token 状态真实落库
- 让 `gateway` 在 revoke 后拒绝已吊销 token

**根因：**

- [postgres_runtime_store.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/postgres_runtime_store.go#L73) 在 `INSERT` 阶段使用 `NULLIF($2, '')`
- [inmemory_runtime.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/inmemory_runtime.go#L164) 和 [inmemory_runtime.go](/home/long/project/立交桥/platform-token-runtime/internal/auth/service/inmemory_runtime.go#L192) 在刷新/撤销时调用 `Save`，但没有重新携带 access token

**建议改动：**

1. 先补 `postgres_runtime_store` 的 focused 测试：
   - 刷新时未重传 access token，仍能保留旧 `token_fingerprint`
   - 撤销时未重传 access token，仍能落库成功
2. 二选一，但必须只保留一套语义：
   - 方案 A：`Save` 在 PostgreSQL 路径上先查询旧记录并补齐 `token_fingerprint`
   - 方案 B：`Refresh` / `Revoke` 路径显式传回原 token 或原 fingerprint
3. 补 HTTP 集成测试：
   - `issue -> refresh -> introspect`
   - `issue -> revoke -> introspect`
   - `issue -> revoke -> gateway /v1/chat/completions` 应返回 `401`

**涉及文件：**

- `platform-token-runtime/internal/auth/service/postgres_runtime_store.go`
- `platform-token-runtime/internal/auth/service/postgres_runtime_store_test.go`
- `platform-token-runtime/internal/auth/service/inmemory_runtime.go`
- `platform-token-runtime/internal/httpapi/token_api_test.go`
- 如有必要：`platform-token-runtime/internal/app/bootstrap_test.go`

**完成标准：**

- `/api/v1/platform/tokens/{token_id}/refresh` 返回 `200`
- `/api/v1/platform/tokens/{token_id}/revoke` 返回 `200`
- PostgreSQL 中 token 状态真实变更
- `gateway` 对已撤销 token 拒绝访问

**验证命令：**

```bash
cd "/home/long/project/立交桥/platform-token-runtime" && go test -count=1 ./internal/auth/service ./internal/httpapi ./internal/app -v
cd "/home/long/project/立交桥/platform-token-runtime" && go test -count=1 ./...
```

### P0-SA-01 修复 `supply-api` 幂等锁 DDL/仓储契约

**负责人：** `Supply Domain/Repository 负责人`

**目标：**

- 让 `POST /api/v1/supply/accounts` 不再因幂等锁初始化失败而直接 `500`

**根因：**

- [idempotency.go](/home/long/project/立交桥/supply-api/internal/repository/idempotency.go#L217) 依赖 `(tenant_id, operator_id, api_path, idempotency_key)` 的 `ON CONFLICT`
- [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L110) 当前没有对应唯一约束

**建议改动：**

1. 先补 repository 集成测试：
   - 首次 `AcquireLock` 成功
   - 未过期重复 key 走冲突语义
   - 过期后可重入
2. 补齐数据库契约，至少包括：
   - 基线 schema 上的唯一索引/唯一约束
   - 已有库升级脚本
   - 分区场景下的兼容实现
3. 校正 `AcquireLock` 与 `SaveResponse` 的 SQL，确认幂等记录不会因分区/过期逻辑失效
4. 补 HTTP 集成测试：
   - 同一个 `Idempotency-Key` 连续两次创建账号

**涉及文件：**

- `supply-api/internal/repository/idempotency.go`
- `supply-api/internal/repository/idempotency_test.go`
- `supply-api/sql/postgresql/partition_strategy_v1.sql`
- 新增一个 forward-only 升级脚本，放到当前 SQL 基线管理目录
- 如有需要：`supply-api/internal/middleware/idempotency_middleware_test.go`

**完成标准：**

- 账号创建不再报 `IDEMPOTENCY_LOCK_FAILED`
- 幂等锁在干净库和升级后数据库都能工作

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/repository -run 'Test.*Idempotency' -v
cd "/home/long/project/立交桥/supply-api" && bash scripts/run_integration_tests.sh ./internal/repository
```

### P0-SA-02 修复 `supply-api` 套餐创建 SQL

**负责人：** `Supply Domain/Repository 负责人`

**目标：**

- 让 `POST /api/v1/supply/packages/draft` 恢复可用
- 让 clone 依赖的内部创建链路恢复可用

**根因：**

- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L27) 目标列为 29 个
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L37) 占位符只有 28 个

**建议改动：**

1. 先补 `PackageRepository.Create` 集成测试：
   - 直接 create 成功
   - clone 间接走 create 也成功
2. 修正列数/占位符/参数顺序
3. 复查 `request_id`、`audit_trace_id`、`created_at/updated_at` 的落库列是否和查询层一致
4. 补 HTTP 测试：
   - `POST /api/v1/supply/packages/draft`
   - `POST /api/v1/supply/packages/{package_id}/clone`

**涉及文件：**

- `supply-api/internal/repository/package.go`
- `supply-api/internal/repository/package_test.go`
- `supply-api/internal/domain/package_test.go`
- `supply-api/internal/httpapi/supply_api_test.go`

**完成标准：**

- draft 创建返回 `201`
- clone 返回 `201`

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/repository ./internal/domain ./internal/httpapi -run 'Test.*Package' -v
```

### P0-SA-03 修复 `supply-api` 账号状态流转的乐观锁错误

**负责人：** `Supply Domain/Repository 负责人`

**目标：**

- 让 `activate` / `suspend` 在正确样本下返回 `200`

**根因：**

- 领域层在 [account.go](/home/long/project/立交桥/supply-api/internal/domain/account.go#L221) 先 `Version++`
- DB adapter 在 [adapter.go](/home/long/project/立交桥/supply-api/internal/adapter/adapter.go#L150) 再把已递增版本作为 `expectedVersion`
- Repository 在 [account.go](/home/long/project/立交桥/supply-api/internal/repository/account.go#L143) 又做 `expectedVersion + 1`

**建议改动：**

1. 明确版本职责，只保留一处递增：
   - 推荐：领域层只表达状态变化，不直接改 `Version`
   - repository 负责 `expectedVersion -> newVersion`
2. 为 DB-backed AccountStore 增加集成测试：
   - 激活 pending/suspended
   - 暂停 active
   - 冲突场景下仍返回真实并发错误
3. 补 HTTP 集成测试：
   - `POST /activate`
   - `POST /suspend`

**涉及文件：**

- `supply-api/internal/domain/account.go`
- `supply-api/internal/domain/account_test.go`
- `supply-api/internal/adapter/adapter.go`
- `supply-api/internal/repository/account.go`
- `supply-api/internal/httpapi/supply_api_test.go`

**完成标准：**

- 正常状态流转返回 `200`
- 冲突场景不再在正常路径误报

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/domain ./internal/repository ./internal/httpapi -run 'Test.*Account' -v
```

### P0-SA-04 修复 `supply-api` 套餐状态流转与字段映射

**负责人：** `Supply Domain/Repository 负责人`

**目标：**

- 让 `publish` / `pause` / `unlist` 返回 `200`
- 修正 `PackageRepository.GetByID` 的字段映射

**根因：**

- 与账号相同的双重 `Version++`
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L73) 查询列是 `supply_account_id, user_id`
- [package.go](/home/long/project/立交桥/supply-api/internal/repository/package.go#L88) 扫描到 `pkg.SupplierID, pkg.AccountID`，发生反转

**建议改动：**

1. 先补 `GetByID` 仓储测试，锁定：
   - `SupplierID == user_id`
   - `AccountID == supply_account_id`
2. 去掉领域层和 adapter 之间的双重版本递增
3. 补状态流转测试：
   - draft -> publish
   - active -> pause
   - paused -> unlist

**涉及文件：**

- `supply-api/internal/repository/package.go`
- `supply-api/internal/repository/package_test.go`
- `supply-api/internal/domain/package.go`
- `supply-api/internal/domain/package_test.go`
- `supply-api/internal/adapter/adapter.go`
- `supply-api/internal/httpapi/supply_api_test.go`

**完成标准：**

- 三个状态流转接口全部 `200`
- `GetByID` 返回的领域对象字段语义正确

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/domain ./internal/repository ./internal/httpapi -run 'Test.*Package' -v
```

---

## P1 任务

### P1-AUD-01 统一 `audit_events` 表结构与审计仓储契约

**负责人：** `Audit/IAM/SQL 负责人`

**目标：**

- 修复审计写入失败
- 修复审计事件查询失败
- 修复账号审计日志查询失败

**根因：**

- 仓储实现按 [audit_repository.go](/home/long/project/立交桥/supply-api/internal/audit/repository/audit_repository.go#L333) 的完整字段集读写
- 当前 [partition_strategy_v1.sql](/home/long/project/立交桥/supply-api/sql/postgresql/partition_strategy_v1.sql#L7) 的 `audit_events` 父表只定义了极简列

**决策要求：**

此任务开始前先做一个明确决策，只能二选一：

1. **以仓储契约为准扩展 schema**
2. **以最小 schema 为准收缩仓储读写字段**

不允许继续维持“两边都不是”的状态。

**推荐方向：**

- 以仓储契约为准扩展 schema。因为当前代码已经在多个路径里读写更多字段，收缩仓储的改动面更广。

**建议改动：**

1. 列出 `audit_repository.go` 实际依赖的全部列
2. 更新 `audit_events` 父表定义
3. 更新分区生成函数，确保新分区继承完整列
4. 为已有库增加迁移脚本
5. 补两类测试：
   - 审计写入成功
   - `GetByEventID` / `QueryWithTotal` 成功

**涉及文件：**

- `supply-api/internal/audit/repository/audit_repository.go`
- `supply-api/internal/audit/repository/audit_repository_test.go`
- `supply-api/internal/audit/postgres_audit_store.go`
- `supply-api/sql/postgresql/partition_strategy_v1.sql`
- 新增 forward-only 迁移脚本
- 如有需要：`supply-api/internal/httpapi/supply_api_test.go`

**完成标准：**

- 审计写入不再报 `trace_id` 缺列
- `GET /api/v1/audit/events/{event_id}` 返回 `200`
- `GET /api/v1/supply/accounts/{account_id}/audit-logs` 返回 `200`

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/audit/... ./internal/httpapi -run 'Test.*Audit' -v
```

### P1-IAM-01 修复 IAM 原始 DDL 初始化失败

**负责人：** `Audit/IAM/SQL 负责人`

**目标：**

- 让 [iam_schema_v1.sql](/home/long/project/立交桥/sql/postgresql/iam_schema_v1.sql) 能在干净数据库直接落库

**根因：**

- `'*'` 默认 Scope 与 `chk_scope_code_format` 不兼容

**建议改动：**

1. 明确 `'*'` 是否要作为保留合法值
2. 二选一：
   - 放宽约束，允许 `'*'`
   - 替换默认种子值，改成合规代码，例如 `all`
3. 同步修正：
   - schema
   - seed 数据
   - 相关测试与文档

**涉及文件：**

- `sql/postgresql/iam_schema_v1.sql`
- 相关 IAM schema 测试
- 相关文档

**完成标准：**

- 干净库执行 `iam_schema_v1.sql` 成功

**验证命令：**

```bash
psql "<test_dsn>" -v ON_ERROR_STOP=1 -f "/home/long/project/立交桥/sql/postgresql/iam_schema_v1.sql"
```

### P1-IAM-02 修复 IAM null scan 问题

**负责人：** `Audit/IAM/SQL 负责人`

**目标：**

- 让 `GET /api/v1/iam/roles`
- 让 `GET /api/v1/iam/users/{user_id}/roles`

在真实数据下正常返回

**根因：**

- `request_id`、`granted_by` 等可空字段被直接扫描进非空基础类型

**建议改动：**

1. 把 repository 扫描改成 `sql.Null*` / 指针 / 中间变量
2. 明确 model 中哪些字段应允许空值
3. 补集成测试：
   - `request_id IS NULL` 的角色记录
   - `granted_by IS NULL` 的用户角色记录

**涉及文件：**

- `supply-api/internal/iam/repository/iam_repository.go`
- `supply-api/internal/iam/repository/iam_repository_test.go`
- 如有需要：`supply-api/internal/iam/model/*.go`

**完成标准：**

- 角色列表和用户角色列表都返回 `200`

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/iam/... -run 'Test.*(ListRoles|GetUserRoles)' -v
```

### P1-IAM-03 修复 IAM 角色更新的 `INET` 写入错误

**负责人：** `Audit/IAM/SQL 负责人`

**目标：**

- 让 `PUT /api/v1/iam/roles/{role_code}` 返回 `200`

**根因：**

- [iam_repository.go](/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go#L158) 直接把空字符串写入 `updated_ip`

**建议改动：**

1. 把 `UpdatedIP` 在 repository 层转换为 `NULL` 而不是 `""`
2. 统一 `CreatedIP / UpdatedIP` 的可空处理
3. 补 update 集成测试：
   - 未提供更新 IP
   - 提供合法 IP

**涉及文件：**

- `supply-api/internal/iam/repository/iam_repository.go`
- `supply-api/internal/iam/repository/iam_repository_test.go`
- `supply-api/internal/iam/model/role.go`

**完成标准：**

- 角色更新成功
- 不再出现 `invalid input syntax for type inet: ""`

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/iam/... -run 'Test.*UpdateRole' -v
```

### P1-IAM-04 修复 IAM 角色分配的 `granted_by` 外键错误

**负责人：** `Audit/IAM/SQL 负责人`

**目标：**

- 让 `POST /api/v1/iam/users/{user_id}/roles` 返回 `201`
- 让对应 revoke 流程可用

**根因：**

- handler 不传 `GrantedBy`
- service 直接把 `0` 落库
- foreign key 拒绝

**建议改动：**

1. 先确定语义：
   - `granted_by` 是必填审计字段
   - 还是允许 `NULL`
2. 推荐修法：
   - 从认证上下文读取操作人 userID
   - handler 构造 `AssignRoleRequest` 时写入 `GrantedBy`
   - 若上下文缺失则明确返回 `401/400`，不要写 `0`
3. 如果产品允许系统分配，才考虑把 `granted_by` 设为可空，并同步模型/DDL
4. 补两条 HTTP 集成测试：
   - assign success
   - revoke success

**涉及文件：**

- `supply-api/internal/iam/handler/iam_handler.go`
- `supply-api/internal/iam/service/iam_service_db.go`
- `supply-api/internal/iam/repository/iam_repository.go`
- 如有需要：`sql/postgresql/iam_schema_v1.sql`
- `supply-api/internal/iam/handler/iam_handler_real_test.go`

**完成标准：**

- assign 返回 `201`
- revoke 在 assign 成功后返回 `200`

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./internal/iam/... -run 'Test.*(AssignRole|RevokeRole)' -v
```

---

## P2 任务

### P2-API-01 统一 `supply-api` handler 错误语义

**负责人：** `Supply Domain/Repository 负责人`

**目标：**

- 避免内部冲突、SQL 错误、创建失败被错误包装成 `404`

**本轮已观察到的错误语义：**

- 账号状态流转内部并发冲突对外是 `404`
- 套餐状态流转内部并发冲突对外是 `404`
- clone 内部创建失败对外是 `404`

**说明：**

这个任务不替代 `P0` 根因修复，但必须在根因修复后补上，防止后续再次出现“逻辑修好了、错误语义仍然误导”的情况。

**建议改动：**

1. 统一 domain/repository 错误类型，不再依赖字符串 contains
2. handler 按错误类别映射：
   - `ErrNotFound -> 404`
   - `ErrConcurrencyConflict / business conflict -> 409`
   - `validation -> 400/422`
   - `unexpected storage error -> 500`
3. 补 handler 层测试，锁定返回码

**涉及文件：**

- `supply-api/internal/httpapi/supply_api.go`
- `supply-api/internal/httpapi/supply_api_test.go`
- 如有需要：`supply-api/internal/repository/errors.go`、`internal/domain/errors.go`

**完成标准：**

- 所有状态流转和 clone 相关接口在错误情况下返回正确状态码

### P2-QA-01 复查 `supply-api/internal/domain` 的波动性失败信号

**负责人：** `QA/CI 负责人`

**目标：**

- 把之前只出现一次、后续未复现的 domain 测试失败定性为：
  - 已消失
  - 或可复现并已修复

**建议改动：**

1. 锁定上次失败覆盖的测试集合
2. 无缓存循环执行至少 `20` 次
3. 记录是否存在：
   - 时序问题
   - 共享状态污染
   - 数据依赖顺序问题
4. 若仍无法复现，保留复查记录并在 CI 中加入高频重跑 job

**涉及文件：**

- `supply-api/internal/domain/*_test.go`
- 可选：CI 脚本

**完成标准：**

- 给出“已稳定 / 已复现并修复 / 仍待观察”的明确结论

**验证命令：**

```bash
cd "/home/long/project/立交桥/supply-api" && GOCACHE=/tmp/lijiaoqiao-go-cache-flake go test -count=20 ./internal/domain -v
```

---

## 总验收

以下门槛全部通过，才允许关闭本任务单：

### 1. 单模块回归

```bash
cd "/home/long/project/立交桥/gateway" && go test -count=1 ./...
cd "/home/long/project/立交桥/platform-token-runtime" && go test -count=1 ./...
cd "/home/long/project/立交桥/supply-api" && go test -count=1 ./...
cd "/home/long/project/立交桥/supply-api" && bash scripts/run_integration_tests.sh ./internal/repository
```

### 2. 干净数据库初始化

必须在全新 PostgreSQL 中成功执行：

- `supply-api/sql/postgresql/*.sql` 当前基线
- `sql/postgresql/iam_schema_v1.sql`
- `sql/postgresql/token_runtime_schema_v1.sql`

### 3. 真实接口矩阵复跑

至少重跑以下接口并全部满足预期：

- `gateway`
  - `/v1/models`
  - `/v1/chat/completions`
  - `/v1/completions`
- `platform-token-runtime`
  - `issue`
  - `introspect`
  - `audit-events`
  - `refresh`
  - `revoke`
- `supply-api`
  - `accounts.verify`
  - `accounts.create`
  - `accounts.activate`
  - `accounts.suspend`
  - `accounts.delete`
  - `accounts.audit-logs`
  - `packages.draft`
  - `packages.batch-price`
  - `packages.publish`
  - `packages.pause`
  - `packages.unlist`
  - `packages.clone`
  - `billing`
  - `settlements.cancel`
  - `settlements.statement`
  - `earnings.records`
  - `audit.events.get`
  - `audit.alerts` 全 CRUD
  - `iam.roles` list/create/get/update/delete
  - `iam.users.roles` get/assign/revoke
  - `iam.scopes`
  - `iam.check-scope`

### 4. 关闭标准

当且仅当满足以下条件，任务单可关闭：

1. 两份原始报告中的全部已证实缺陷都有对应提交和验证记录
2. 不再存在 “真实接口失败但单元测试全绿” 的已知缺口
3. 新一轮接口矩阵中只允许保留“条件关闭”的提现项
4. `gateway / platform-token-runtime / supply-api` 三个模块都能在真实 PostgreSQL 下完成主链路联调

## 建议执行批次

### 批次 A：必须先完成

- `P0-TR-01`
- `P0-SA-01`
- `P0-SA-02`
- `P0-SA-03`
- `P0-SA-04`

### 批次 B：完成后才能宣称 DB-backed 能力闭环

- `P1-AUD-01`
- `P1-IAM-01`
- `P1-IAM-02`
- `P1-IAM-03`
- `P1-IAM-04`

### 批次 C：收口与抗回归

- `P2-API-01`
- `P2-QA-01`
