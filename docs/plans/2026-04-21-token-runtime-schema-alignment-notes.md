# 2026-04-21 Token Runtime Schema Alignment Notes

## 1. 输入范围

- `sql/postgresql/token_runtime_schema_v1.sql`
- `platform-token-runtime/internal/auth/service/inmemory_runtime.go`
- `platform-token-runtime/internal/auth/service/postgres_runtime_store.go`
- `platform-token-runtime/internal/auth/service/postgres_audit_store.go`
- `platform-token-runtime/internal/auth/service/token_verifier.go`
- `platform-token-runtime/internal/httpapi/token_api.go`

## 2. P1-B-01 Schema 字段落点

### 2.1 `auth_platform_tokens`

来源：`sql/postgresql/token_runtime_schema_v1.sql:11`

| 字段 | 列位置 | 说明 |
|---|---|---|
| `tenant_id` | `sql/postgresql/token_runtime_schema_v1.sql:16` | token 归属租户 |
| `project_id` | `sql/postgresql/token_runtime_schema_v1.sql:17` | token 归属项目 |
| `operator_id` | 无 | 当前 token 主表没有该列 |
| `metadata` | 无 | 当前 token 主表没有该列 |

### 2.2 `auth_token_audit_events`

来源：`sql/postgresql/token_runtime_schema_v1.sql:47`

| 字段 | 列位置 | 说明 |
|---|---|---|
| `tenant_id` | `sql/postgresql/token_runtime_schema_v1.sql:57` | 审计事件租户 |
| `project_id` | `sql/postgresql/token_runtime_schema_v1.sql:58` | 审计事件项目 |
| `operator_id` | `sql/postgresql/token_runtime_schema_v1.sql:59` | 操作人 |
| `metadata` | `sql/postgresql/token_runtime_schema_v1.sql:61` | 事件附加信息 |

## 3. P1-B-02 当前模型缺口

### 3.1 `TokenRecord`

来源：`platform-token-runtime/internal/auth/service/inmemory_runtime.go:18`

当前字段：

- `TokenID`
- `AccessToken`
- `SubjectID`
- `Role`
- `Scope`
- `IssuedAt`
- `ExpiresAt`
- `Status`
- `RequestID`
- `RevokedReason`

与 schema / canonical principal 的差异：

- 缺 `TenantID`
- 缺 `ProjectID`
- 不承载 `OperatorID`
- 不承载 `Metadata`

### 3.2 `IssueTokenInput`

来源：`platform-token-runtime/internal/auth/service/inmemory_runtime.go:31`

当前仅包含 `SubjectID`、`Role`、`Scope`、`TTL`、`RequestID`、`IdempotencyKey`，没有 `TenantID`、`ProjectID`、`Metadata`。

### 3.3 `VerifiedToken`

来源：`platform-token-runtime/internal/auth/service/token_verifier.go:42`

当前仅包含 `TokenID`、`SubjectID`、`Role`、`Scope`、`IssuedAt`、`ExpiresAt`、`NotBefore`、`Issuer`、`Audience`，缺 `TenantID`，与最新 canonical principal 约束不一致。

### 3.4 `AuditEvent`

来源：`platform-token-runtime/internal/auth/service/token_verifier.go:66`

当前仅包含 `EventID`、`EventName`、`RequestID`、`TokenID`、`SubjectID`、`Route`、`ResultCode`、`ClientIP`、`CreatedAt`，缺：

- `TenantID`
- `ProjectID`
- `OperatorID`
- `Metadata`

## 4. P1-B-03 单一决策

决策：`保留字段并贯穿`

理由：

1. `tenant_id` 已进入最小 canonical principal 契约，不能再通过删除字段回退。
2. `project_id`、`operator_id`、`metadata` 已进入现有 schema，且 Phase 1 目标明确要求补齐证据链与审计闭环，此时删除只会扩大文档、DDL 与代码的漂移面。
3. 当前问题不是“字段多余”，而是“DDL 已定义但模型和 store 没有贯穿”；收缩契约会让后续多租户和审计治理重新返工。

## 5. P1-B-04 字段迁移顺序

按以下单轨顺序执行：

1. `model`
   更新 `TokenRecord`、`IssueTokenInput`、`VerifiedToken`、`AuditEvent`、`AuditEventFilter` 的字段定义。
2. `store`
   先改 `runtime_store.go` / `postgres_runtime_store.go`，再改 `audit_store.go` / `postgres_audit_store.go` 的读写映射。
3. `API`
   更新 `token_api.go` 的 `issue` 请求、`introspect` 响应、`audit-events` 输出与过滤参数。
4. `audit`
   把 middleware、issue / refresh / revoke / introspect 的发射点补齐 `tenant_id`、`project_id`、`operator_id`、`metadata` 来源。
5. `tests`
   补充 runtime store、audit store、HTTP API、middleware 的字段断言与回归验证。

## 6. P1-B-05 删除路线说明

不采用删除路线。

按 `P1-B-03` 的单一决策，当前不编写 shrink SQL，也不定义 existing DB 的收缩策略；后续实施只走“保留字段并贯穿”。

## 7. P1-B-06 `postgres_runtime_store.go` 字段映射检查

### 7.1 `INSERT` / `UPSERT`

来源：`platform-token-runtime/internal/auth/service/postgres_runtime_store.go:84`

当前已写入：

- `token_id`
- `token_fingerprint`
- `hash_algo`
- `subject_id`
- `role_code`
- `scope_json`
- `status`
- `issued_at`
- `expires_at`
- `revoked_reason`
- `revoked_at`
- `issue_request_id`
- `issue_idempotency_key`
- `issue_request_hash`

当前未写入 schema 已存在列：

- `tenant_id`
- `project_id`
- `last_seen_at`
- `created_at` / `updated_at` 依赖 DB 默认值，不是问题

### 7.2 `SELECT`

来源：

- `platform-token-runtime/internal/auth/service/postgres_runtime_store.go:156`
- `platform-token-runtime/internal/auth/service/postgres_runtime_store.go:167`

当前读取：

- `token_id`
- `subject_id`
- `role_code`
- `scope_json`
- `status`
- `issued_at`
- `expires_at`
- `issue_request_id`
- `revoked_reason`

当前未读取 schema 已存在列：

- `tenant_id`
- `project_id`
- `token_fingerprint`
- `revoked_at`
- `last_seen_at`

结论：

1. runtime store 当前只覆盖了最小旧字段集，没有贯穿多租户 / 项目字段。
2. 即使 OpenAPI 已补 `tenant_id`，代码运行态仍无法产出该字段，后续必须先补 model 和 store。

## 8. P1-B-07 `postgres_audit_store.go` 字段映射检查

### 8.1 `Emit`

来源：`platform-token-runtime/internal/auth/service/postgres_audit_store.go:68`

当前写入：

- `event_id`
- `event_name`
- `request_id`
- `token_id`
- `subject_id`
- `route`
- `result_code`
- `client_ip`
- `created_at`

当前未写入 schema 已存在列：

- `tenant_id`
- `project_id`
- `operator_id`
- `metadata`

### 8.2 `QueryEvents`

来源：`platform-token-runtime/internal/auth/service/postgres_audit_store.go:121`

当前读取 / 过滤：

- `event_id`
- `event_name`
- `request_id`
- `token_id`
- `subject_id`
- `route`
- `result_code`
- `client_ip`
- `created_at`

当前未读取 / 未过滤：

- `tenant_id`
- `project_id`
- `operator_id`
- `metadata`

结论：

1. audit schema 已预留审计闭环字段，但 `AuditEvent` / `AuditEventFilter` / Postgres store 三层都没有承接。
2. 这会导致数据即使以后由其他地方补写，也无法通过当前查询接口读出和验证。

## 9. P1-B-08 最小执行清单

### 9.1 改 model

- 给 `TokenRecord` 补 `TenantID`、`ProjectID`
- 给 `IssueTokenInput` 补 `TenantID`、`ProjectID`、`Metadata`
- 给 `VerifiedToken` 补 `TenantID`
- 给 `AuditEvent` 补 `TenantID`、`ProjectID`、`OperatorID`、`Metadata`
- 给 `AuditEventFilter` 补 `TenantID`、`ProjectID`、`OperatorID`

### 9.2 改 SQL / store

- `runtime_store.go` 持久化并返回 `TenantID`、`ProjectID`
- `postgres_runtime_store.go` 的 `INSERT` / `SELECT` / `UPSERT` 同步 `tenant_id`、`project_id`
- `audit_store.go` 与 `postgres_audit_store.go` 同步 `tenant_id`、`project_id`、`operator_id`、`metadata`

### 9.3 改 API

- `token_api.go` 的 `issue` 请求体补字段入口
- `token_api.go` 的 `introspect` 响应补 `tenant_id`
- `audit-events` 查询与返回体补齐审计扩展字段

### 9.4 改 tests

- `postgres_runtime_store_test.go` 增加 `tenant_id` / `project_id` 断言
- `postgres_audit_store_test.go` 增加 `operator_id` / `tenant_id` / `metadata` 断言
- `token_api_test.go` 增加 issue/introspect/audit-events 字段断言
- middleware / executable tests 增加审计字段传递断言

### 9.5 跑验证

- `cd "/home/long/project/立交桥/platform-token-runtime" && GOCACHE=/tmp/lijiaoqiao-go-cache-platform-token-runtime go test ./...`
- `cd "/home/long/project/立交桥" && bash scripts/ci/repo_integrity_check.sh`
