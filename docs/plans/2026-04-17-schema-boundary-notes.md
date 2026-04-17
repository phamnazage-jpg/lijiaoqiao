# 2026-04-17 Schema Boundary Notes

## 目的

澄清仓库里几套 PostgreSQL DDL 的边界，避免把“并列 schema”“历史迁移脚本”“当前 fresh setup 基线”混成一个问题。

## 当前基线

### 1. `supply-api`

`supply-api` 的 fresh setup 基线由本服务目录下的脚本维护，仓储集成测试实际应用的是：

- `supply-api/sql/postgresql/supply_core_schema_v2.sql`
- `supply-api/sql/postgresql/partition_strategy_v1.sql`
- `supply-api/sql/postgresql/outbox_pattern_v1.sql`
- `supply-api/sql/postgresql/token_status_registry_v1.sql`
- `supply-api/sql/postgresql/audit_alerts_v1.sql`

其中：

- `supply_core_schema_v2.sql` 负责账户、套餐、结算三张核心业务表。
- `partition_strategy_v1.sql` 负责 supply-side `audit_events`、`billing_ledger_entries` 以及相关分区策略。
- `audit_events_migration_v1_to_v2.sql` 是历史迁移脚本，不属于 fresh setup 基线。

### 2. 平台侧根目录 SQL

仓库根目录 `sql/postgresql/platform_core_schema_v1.sql` 是平台核心域的 fresh setup 基线，覆盖：

- `core_tenants`
- `core_projects`
- `iam_users`
- `auth_platform_api_keys`
- `billing_accounts`
- `billing_ledger_entries`
- `routing_policies`
- `security_kms_key_registry`
- `audit_events`

这里的 `audit_events` 属于平台核心域，不等同于 `supply-api/sql/postgresql/partition_strategy_v1.sql` 中的 supply-side `audit_events`。

### 3. `platform-token-runtime`

仓库根目录 `sql/postgresql/token_runtime_schema_v1.sql` 是 token runtime 的独立基线，覆盖：

- `auth_platform_tokens`
- `auth_token_audit_events`

其中 `auth_token_audit_events` 只服务 `platform-token-runtime`，不是平台侧 `audit_events` 的别名或替代表。

## 边界结论

1. 仓库里存在多套审计相关表定义，并不自动等于“同一张表被错误重复定义”。
2. 真正需要区分的是服务边界：
   - 平台侧：`sql/postgresql/platform_core_schema_v1.sql::audit_events`
   - 供应侧：`supply-api/sql/postgresql/partition_strategy_v1.sql::audit_events`
   - token runtime：`sql/postgresql/token_runtime_schema_v1.sql::auth_token_audit_events`
3. 这些文件不应在同一个 fresh setup 数据库里被无差别串行执行，除非先明确数据库或 schema 级隔离。

## 状态约束结论

`supply-api/sql/postgresql/supply_core_schema_v2.sql` 的 `status` 约束已经按当前代码真实集合收敛：

- `supply_accounts.status`
  - `pending`
  - `active`
  - `suspended`
  - `disabled`
- `supply_packages.status`
  - `draft`
  - `active`
  - `paused`
  - `sold_out`
  - `expired`
- `supply_settlements.status`
  - `pending`
  - `processing`
  - `completed`
  - `failed`

后续如果领域层扩展状态，必须同步修改：

1. `internal/domain/*` 常量
2. 仓储测试/集成测试
3. 对应 DDL 的 `CHECK`

## 命名一致性方向

当前仓库仍同时存在 `ClientIP` 与 `SourceIP`：

- `ClientIP`
  - 保留在 HTTP / middleware / request context 这类“直接来自入口请求”的局部上下文
- `SourceIP`
  - 作为审计事件、持久化模型、对外 JSON 字段的统一命名

因此，后续新增代码遵循以下规则：

1. 进入审计域或持久化域后，统一写 `SourceIP`
2. 中间件和入口请求提取函数继续使用 `ClientIP` 作为临时变量是允许的
3. 不再新增 `ClientIP` 字段进入审计模型或数据库模型
