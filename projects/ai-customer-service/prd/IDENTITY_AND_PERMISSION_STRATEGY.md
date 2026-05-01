# 身份核验与数据权限策略

> 版本：v1.0 | 状态：已生效
> 关联：tech/INTERFACE.md、PRODUCTION_PHASE1_STATUS.md

---

## 1. 身份核验

### 1.1 核验场景

客服系统需要处理两类身份核验：

| 场景 | 说明 |
|------|------|
| 用户身份核验 | 验证用户提供的邮箱/手机与注册信息匹配（用于敏感操作如退款查询） |
| 客服身份核验 | 验证运营后台操作者的身份（防止越权操作） |

### 1.2 用户身份核验

**接口**（`tech/INTERFACE.md` 定义）：

| 接口 | 路径 | 说明 |
|------|------|------|
| 身份校验 | `GET /internal/supply/users/verify?email={email}` | 校验用户身份是否匹配 |
| 配额查询 | `GET /internal/runtime/quota?user_id={uid}` | 查询用户配额 |
| Token 消耗查询 | `GET /internal/runtime/token-usage?user_id={uid}&window=1d` | 查询 Token 消耗 |
| 错误日志 | `GET /internal/runtime/error-logs?user_id={uid}&limit=5` | 查询错误日志 |

**当前状态**：上述接口**已定义但外部依赖（supply-api / token-runtime）尚未联调**，实际调用可能失败。

**核验流程**：
1. 用户发起敏感操作（如查询退款状态）
2. 系统要求用户输入邮箱 + 验证码
3. 调用 supply-api 校验邮箱是否匹配用户 ID
4. 匹配成功后执行操作，否则拒绝

### 1.3 身份核验失败处理

| 失败次数 | 处理方式 |
|----------|----------|
| 1-2 次 | 返回 `CS_IDT_4002`（验证码错误），允许重试 |
| 3 次 | 返回 `CS_SES_4003`（身份校验已锁定），锁定 15 分钟 |
| 锁定期间 | 所有身份核验请求返回 403，持续 15min 后自动解锁 |

> **注**：失败计数和锁定机制**当前未落地**（P0 缺口），身份校验只返回匹配结果，不做计数锁定。

---

## 2. 数据权限策略

### 2.1 权限基本原则

- 用户**只能查询自己的**会话、工单、Token 消耗数据
- 客服**只能操作被分配的**工单
- 管理员可以查看所有数据，但不得泄露给未授权第三方
- 审计日志**不可篡改**，所有敏感操作均需记录

### 2.2 客服操作权限

| 操作 | agent | supervisor | admin |
|------|-------|------------|-------|
| 查看自己被分配的工单 | ✅ | ✅ | ✅ |
| 查看所有工单 | ❌ | ✅ | ✅ |
| assign 工单 | 仅自己的 | ✅ | ✅ |
| resolve 工单 | 仅自己的 | ✅ | ✅ |
| 查看转人工统计 | ❌ | ✅ | ✅ |
| 查看运营大盘 | ❌ | ✅ | ✅ |
| 敏感操作（退款） | ❌ | ✅ | ✅ |

> **注**：权限模型**当前未落地**（无 RBAC 实现），所有接口均为平权访问。Phase 4 运营后台需补充完整权限校验。

### 2.3 跨用户数据隔离

**当前状态**：`tech/INTERFACE.md` 中各接口的 user_id 隔离**依赖调用方传入正确的 user_id**，后端不做强制校验。

**缺失项（P0）**：
- 所有查询类接口（sessions、tickets、quota 等）应强制要求带上 `user_id`，后端校验 `user_id` 归属，不允许跨用户查询
- 客服操作工单时，后端应校验工单的 `user_id` 与当前操作者的权限范围

**建议方案**（待 TechLead 评审）：
```
// 中间件层增强
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := getJWTClaims(r)
        ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
        ctx = context.WithValue(ctx, "role", claims.Role)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// 处理器层校验
func (h *TicketHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id")
    ticketID := mux.Vars(r)["id"]
    ticket := h.store.GetTicket(ticketID)
    
    role := r.Context().Value("role")
    if role != "admin" && role != "supervisor" && ticket.UserID != userID {
        writeError(w, "CS_AUTH_4001", 403) // 越权访问
        return
    }
}
```

---

## 3. Webhook 身份校验

### 3.1 已落地

- **HMAC 签名校验**（`webhook_security.go`）：验证请求来自合法渠道
- **时间戳防重放**（`webhook_security.go`）：防止 replay attack
- **幂等去重**（`dedup_store.go`）：防止重复消息

### 3.2 待补充

| 项目 | 优先级 | 说明 |
|------|--------|------|
| webhook 速率限制 | P1 | 防止恶意刷请求 |
| 渠道级独立 webhook 路由 | P0 | INTERFACE 定义 `/webhook/{channel}`，当前统一入口 |

---

## 4. 敏感数据处理

### 4.1 敏感字段

| 字段 | 处理方式 |
|------|----------|
| 用户邮箱 | 脱敏展示（后三位 + `@` 前的后三位），如 `t***@gmail.com` |
| 用户手机 | 脱敏展示（后四位），如 `***-****-1234` |
| API Key | 仅返回前缀后四字符，如 `sk-****-abcd` |
| 退款金额 | 日志脱敏，接口明文返回（须登录态） |

### 4.2 当前状态

敏感数据脱敏**当前未落地**，所有字段明文返回。

---

## 5. 审计日志与权限审计

### 5.1 已落地

- **审计日志持久化**（`audit_store.go`）：写入 PostgreSQL `cs_audit_logs` 表
- **fail-closed**：审计写入失败时整体请求返回错误
- **source_ip / actor_id**：记录操作来源（actor_id 当前有默认值 fallback）

### 5.2 待补充

| 项目 | 优先级 | 说明 |
|------|--------|------|
| 安全拒绝事件审计 | P0 | 签名失败、时间戳失败不记审计 |
| 工单状态流转审计 | P0 | assign/resolve 未写审计 |
| source_ip 字段缺失 | P0 | audit_store 当前未写 source_ip |

---

## 6. 当前版本状态

- **本文档版本**：v1.0
- **生效日期**：2026-04-30
- **下次审查**：RBAC 权限模型落地后
