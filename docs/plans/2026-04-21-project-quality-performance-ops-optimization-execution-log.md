# 2026-04-21 Project Quality / Performance / Ops Optimization Execution Log

## P1-A-01 当前身份入口扫描结果

执行命令：

```bash
rg -n "inmemory|remote_introspection|token runtime|JWT|Bearer" gateway supply-api platform-token-runtime
```

结论：

1. `gateway` 当前同时保留两种 token runtime 装配路径：
   - `gateway/internal/app/bootstrap.go`
   - `gateway/internal/config/config.go`
   - `gateway/internal/middleware/chain.go`
   - `gateway/README.md`

2. `platform-token-runtime` 当前承载独立的 token issue / refresh / revoke / introspect / audit-events 语义：
   - `platform-token-runtime/internal/app/bootstrap.go`
   - `platform-token-runtime/internal/httpapi/token_api.go`
   - `platform-token-runtime/internal/auth/middleware/token_auth_middleware.go`
   - `platform-token-runtime/README.md`

3. `supply-api` 当前仍保留独立 JWT 校验、Bearer 提取和 token 状态后端：
   - `supply-api/internal/middleware/auth.go`
   - `supply-api/internal/app/bootstrap.go`
   - `supply-api/internal/middleware/ratelimit.go`
   - `supply-api/internal/middleware/token_format_test.go`

4. 当前身份链路的关键事实：
   - `gateway` 的远程模式依赖 `platform-token-runtime` 做 introspection。
   - `supply-api` 仍以 JWT claims 为认证入口。
   - 三个服务对“谁是 authority”仍未收敛为单一真源。

## P1-A-04 OpenAPI 与 canonical principal 差异记录

执行命令：

```bash
rg -n "IntrospectTokenResponse|tenant_id|project_id|operator_id|metadata|IssueTokenRequest" docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml
```

结论：

1. `IssueTokenRequest` 已定义 `metadata`，但 `IntrospectTokenResponse.data` 尚未暴露 `tenant_id`。
2. 现有 introspection 响应字段只有 `token_id`、`subject_id`、`role`、`status`、`scope`、`issued_at`、`expires_at`，与 canonical principal 最小字段清单相比缺少 `tenant_id`。
3. `project_id`、`operator_id`、`metadata` 目前也未出现在 introspection 响应中，但它们尚未进入最小 canonical principal 强制字段，保留为 Phase 1 后续 schema / 审计收敛项。

## P1-A-07 supply-api README 统一 principal 说明

结论：

1. 已在 `supply-api/README.md` 明确 `supply-api` 只消费 canonical principal，不自持独立 token authority。
2. 文档现在把 token 状态与权限判断的唯一来源收束到 `platform-token-runtime` 的 introspection 结果。

## P1-A-08 platform-token-runtime README 唯一 authority 与字段边界

结论：

1. 已在 `platform-token-runtime/README.md` 明确 `platform-token-runtime` 是唯一 token authority。
2. 文档现在把 canonical principal 的最小字段边界写死为 `token_id`、`subject_id`、`tenant_id`、`role`、`scope`、`issued_at`、`expires_at`、`status`。
3. 文档同时要求未来扩展字段必须同步更新 DDL、OpenAPI、存储模型和审计字段，避免边界漂移。
