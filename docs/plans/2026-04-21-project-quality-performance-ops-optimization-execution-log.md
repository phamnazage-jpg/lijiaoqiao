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

待执行。

