# 平台鉴权与 Token 校验中间件设计（TOK-002）

- 版本：v1.0
- 日期：2026-03-29
- 状态：开发实施设计基线
- 依赖：`docs/token_runtime_minimal_spec_v1.md`
- 目标：实现“仅平台凭证入站”，并为 M-014/M-016/M-021 提供可验证链路。

## 1. 设计目标

1. 所有北向请求必须通过平台凭证校验。
2. 外部 `query key` 入站一律拒绝并记录审计事件。
3. 鉴权结果可追踪到 `request_id + subject_id + token_id`。
4. 在不泄露上游凭证的前提下返回标准错误码。

## 2. 适用范围

1. 路由范围：`/api/v1/supply/*`、`/api/v1/platform/*`。
2. 鉴权头：仅支持 `Authorization: Bearer <token>`。
3. 排除范围：健康检查、内部探针、公开静态资源。

## 3. 中间件链路

## 3.1 处理顺序

1. `RequestIdMiddleware`
2. `QueryKeyRejectMiddleware`
3. `BearerExtractMiddleware`
4. `TokenVerifyMiddleware`
5. `TokenStatusCheckMiddleware`
6. `ScopeRoleAuthzMiddleware`
7. `AuditEmitMiddleware`

## 3.2 关键规则

1. `QueryKeyRejectMiddleware`
   - 拒绝任意 `?key=`、`?api_key=`、`?token=` 形式外部参数。
   - 返回 `401 QUERY_KEY_NOT_ALLOWED`。
2. `BearerExtractMiddleware`
   - 无 `Authorization` 直接 `401 AUTH_MISSING_BEARER`。
3. `TokenVerifyMiddleware`
   - 校验签名、`iss`、`aud`、`exp`、`nbf`、`jti`。
   - 签名失败返回 `401 AUTH_INVALID_TOKEN`。
4. `TokenStatusCheckMiddleware`
   - 查询 token 状态缓存（`active/revoked/expired`）。
   - `revoked/expired` 返回 `401 AUTH_TOKEN_INACTIVE`。
5. `ScopeRoleAuthzMiddleware`
   - 按路由匹配 scope；不足返回 `403 AUTH_SCOPE_DENIED`。

## 4. 数据与缓存策略

1. 状态源：`platform_token_registry`（运行态主表）。
2. 热缓存：`token_status_cache`（TTL 30s）。
3. 吊销传播：
   - 吊销事件写入总线后，1~5 秒内刷新缓存。
   - 验收阈值：吊销生效延迟 `<= 5s`。

## 5. 错误语义

| 场景 | HTTP | error.code | 说明 |
|---|---|---|---|
| 缺失 Bearer | 401 | AUTH_MISSING_BEARER | 请求头缺失 |
| query key 外部入站 | 401 | QUERY_KEY_NOT_ALLOWED | 边界拒绝 |
| token 无效/签名失败 | 401 | AUTH_INVALID_TOKEN | 校验失败 |
| token 已吊销/过期 | 401 | AUTH_TOKEN_INACTIVE | 状态不可用 |
| scope 不足 | 403 | AUTH_SCOPE_DENIED | 权限不足 |

## 6. 审计事件（TOK-004 依赖）

1. `token.authn.success`
2. `token.authn.fail`
3. `token.authz.denied`
4. `token.query_key.rejected`

最小字段：
1. `event_id`
2. `request_id`
3. `token_id`（可空，提取失败时为空）
4. `subject_id`（可空）
5. `route`
6. `result_code`
7. `client_ip`
8. `created_at`

## 7. 伪代码（实现参考）

```text
onRequest(req):
  reqId = ensureRequestId(req)
  if hasExternalQueryKey(req):
    emitAudit("token.query_key.rejected", reqId, route, clientIp)
    return 401 QUERY_KEY_NOT_ALLOWED

  bearer = parseBearer(req.headers.Authorization)
  if bearer is null:
    emitAudit("token.authn.fail", reqId, route, "AUTH_MISSING_BEARER")
    return 401 AUTH_MISSING_BEARER

  claims = verifyToken(bearer)
  if verify failed:
    emitAudit("token.authn.fail", reqId, route, "AUTH_INVALID_TOKEN")
    return 401 AUTH_INVALID_TOKEN

  status = getTokenStatus(claims.jti)
  if status != active:
    emitAudit("token.authn.fail", reqId, route, "AUTH_TOKEN_INACTIVE")
    return 401 AUTH_TOKEN_INACTIVE

  if !checkScopeRole(claims.scope, claims.role, route):
    emitAudit("token.authz.denied", reqId, route, "AUTH_SCOPE_DENIED")
    return 403 AUTH_SCOPE_DENIED

  attachPrincipal(req, claims)
  emitAudit("token.authn.success", reqId, route, "OK")
  pass
```

## 8. 开发阶段验收（设计级）

1. 与 `TOK-001` 角色、状态机、审计字段一致。
2. 与 `M-014/M-016` 指标定义一致。
3. 与 OpenAPI token 契约草案字段一致。
