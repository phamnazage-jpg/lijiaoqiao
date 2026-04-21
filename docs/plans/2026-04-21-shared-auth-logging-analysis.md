# P4-A 分析：共享 auth / logging / audit 能力盘点

生成时间: 2026-04-21
Phase: P4-A

---

## 一、Auth Middleware 差异分析

### 1.1 三服务 Auth 中间件概览

| 维度 | Gateway | Supply-api | Platform-token-runtime |
|------|---------|------------|----------------------|
| 文件位置 | `gateway/internal/middleware/chain.go` | `supply-api/internal/middleware/auth.go` | `platform-token-runtime/internal/auth/middleware/token_auth_middleware.go` |
| 文件行数 | 326 行 | 891 行 | 待盘点 |
| 中间件顺序 | requestID → queryKeyReject → tokenAuth | queryKeyReject → bearerExtract → tokenVerify → scopeRoleAuthz | 待盘点 |
| JWT 依赖 | 无（自定义 claims） | github.com/golang-jwt/jwt/v5 | 待盘点 |

### 1.2 可共享能力（可抽取到共享包）

#### A. 错误响应格式
三服务统一使用相同 JSON 结构：
```json
{"request_id": "...", "error": {"code": "...", "message": "..."}}
```
**Gateway** `writeError()` 定义了 `errorResponse` / `errorPayload` struct。
**Supply-api** `writeAuthError()` 也使用同样结构。
→ **建议**：提取到 `shared/pkg/auth/errors.go`

#### B. Bearer Token 提取
- **Gateway**: `extractBearerToken()` — 检查 "Bearer " 前缀，空 token 返回 false
- **Supply-api**: 相同逻辑在 `BearerExtractMiddleware` 中
→ **建议**：提取到 `shared/pkg/auth/token.go`

#### C. Query Key 拒绝
- **Gateway**: `hasExternalQueryKey()` — 检查 key/api_key/token/access_token
- **Supply-api**: `QueryKeyRejectMiddleware` — 检查 key/api_key/token/secret/password/credential（含长度检测）
→ **Supply-api 覆盖更广，建议以 Supply-api 为基准抽取**

#### D. ClientIP 提取（含可信代理）
- **Gateway**: `extractClientIP()` — X-Forwarded-For 仅在可信代理时使用
- **Supply-api**: `getClientIP()` — 相同逻辑
→ **几乎相同，可直接共享**

#### E. AuditEvent 结构
```go
type AuditEvent struct {
    EventName  string
    RequestID  string
    TokenID    string
    SubjectID  string
    Route      string
    ResultCode string
    ClientIP   string
    CreatedAt  time.Time
}
```
三服务 AuditEvent 字段完全一致。

#### F. RequestID 管理
- **Gateway**: `ensureRequestID()` / `RequestIDFromContext()` / `PrincipalFromContext()`
- **Supply-api**: `getRequestID()` + context key 方式
→ **结构不同：Gateway 用 `contextKey` typed key；Supply-api 用 string key**

### 1.3 必须保留服务差异的能力

#### A. JWT 验证（必须各异）
- **Gateway**: 自定义 `Verifier` 接口（支持多种实现：本地缓存/远程 introspection）
- **Supply-api**: 强耦合 `github.com/golang-jwt/jwt/v5`，直接解析 JWT
→ **JWT 库选择和服务内验证逻辑必须保留**

#### B. Token 状态查询（必须各异）
- **Gateway**: `StatusResolver` 接口，支持缓存层
- **Supply-api**: `TokenStatusBackend` 接口，直接查询后端
→ **接口抽象可共享，实现各异**

#### C. 授权逻辑（必须各异）
- **Gateway**: `Authorizer` 接口，基于 path prefix + method + scope + role
- **Supply-api**: 硬编码路由角色映射 + `model.GetRoleLevelByCode()`
→ **Gateway 的 Authorizer 更灵活；Supply-api 的 routeRoles 映射更静态**

#### D. Brute Force 保护（Supply-api 独有）
- **Supply-api** 有 `BruteForceProtection`（277 行实现）
- Gateway 和 Platform-token-runtime 无此能力
→ **这是 Supply-api 特有安全能力，不需要共享**

#### E. Principal / TokenClaims 结构（服务各异）
- **Gateway Principal**: RequestID, TokenID, SubjectID, Role, Scope
- **Supply-api TokenClaims**: JWT RegisteredClaims + SubjectID, Role, Scope, TenantID
→ **Gateway 有 RequestID；Supply-api 有 TenantID**

#### F. 中间件链组合（服务各异）
- Gateway: `BuildTokenAuthChain` 组合 3 层
- Supply-api: 独立 middleware 方法，可自由组合
- Platform-token-runtime: 待确认
→ **链组合逻辑必须在各服务内**

### 1.4 Audit 事件名称对比

| 事件 | Gateway 常量 | Supply-api 字符串 |
|------|------------|----------------|
| Query key 拒绝 | `EventTokenQueryKeyRejected` | `token.query_key.rejected` |
| 认证失败（无 bearer） | `EventTokenAuthnFail` | `token.authn.fail` |
| 认证失败（invalid token） | `EventTokenAuthnFail` | `token.authn.fail` |
| Token 未激活 | `EventTokenAuthnFail` | `token.authn.fail` |
| 授权拒绝 | `EventTokenAuthzDenied` | `token.authz.denied` |
| 认证成功 | `EventTokenAuthnSuccess` | `token.authn.success` |

**差异**：Gateway 用常量 + `ResultCode` 字段区分；Supply-api 用不同 `EventName` 字符串区分。
**建议**：统一使用常量，差异在 `ResultCode` 中体现。

---

## 二、Logging 差异分析

### 2.1 三服务 Logging 实现对比

| 维度 | Gateway | Supply-api | Platform-token-runtime |
|------|---------|------------|----------------------|
| 文件位置 | `internal/pkg/logging/logger.go` | `internal/pkg/logging/logger.go` | `internal/pkg/logging/logger.go` |
| 行数 | 192 行 | 260 行 | 192 行 |
| LogEntry schema | 相同 | 相同 | 相同 |
| LogLevel 枚举 | 相同 | 相同 | 相同 |
| Logger 类型 | `*Logger` 具体类型 | `Logger` 接口 + `*jsonLogger` | `*Logger` 具体类型 |
| FieldKeyXXX 常量 | 无 | 有 | 无 |
| SensitiveFields | 有 | 有（+ passport） | 有 |
| Fatal 行为 | `os.Exit(1)` | `os.Exit(1)` | `os.Exit(1)` |
| JSON 编码 | `json.NewEncoder` | `json.Marshal` | `json.NewEncoder` |
| Output 方式 | `encoder.Encode` | `Write(append(data,'\n'))` | `encoder.Encode` |

### 2.2 可共享能力

**完全相同（可直接共享）：**
- LogEntry struct（timestamp/level/service/trace_id/span_id/request_id/message/fields）
- LogLevel 枚举（DEBUG/INFO/WARN/ERROR/FATAL）
- shouldLog() 逻辑
- sanitizeFields() 逻辑（supply-api 多一个 "passport" 字段）
- toLower() / contains() 工具函数
- Fatal / Infof / Errorf 等方法签名

**差异点：**
1. **Supply-api 有 Logger 接口**（可测试性更好）；Gateway/Platform-token-runtime 无接口
2. **Supply-api 有 FieldKeyXXX 常量**（防拼写错误）
3. **Supply-api 的 SensitiveFields 多 "passport"**
4. **JSON 编码方式细微差异**（Encoder vs Marshal）

### 2.3 迁移建议

1. **以 Gateway 的 `*Logger` 结构为基础**，吸收 Supply-api 的 `Logger` 接口和 FieldKeyXXX 常量
2. **统一 JSON 编码方式**（Encoder 或 Marshal 二选一）
3. **SensitiveFields 取并集**（加入 "passport"）
4. **抽取到 `internal/shared/logging/`**

---

## 三、Audit Emitter 差异分析

### 3.1 三服务 Audit 实现对比

| 维度 | Gateway | Supply-api | Platform-token-runtime |
|------|---------|------------|----------------------|
| 接口定义 | `AuditEmitter` 接口 | `AuditEmitter` 接口 | 待确认 |
| 事件结构 | `AuditEvent` struct | `AuditEvent` struct | 待确认 |
| 表名 | `audit_events` | `audit_events` | `audit_events` |
| 字段 | EventName, RequestID, TokenID, SubjectID, Route, ResultCode, ClientIP, CreatedAt | 相同 | 待确认 |

**结论**：三服务 AuditEvent 结构相同，表名相同。差异在于：
- Gateway 用 `EventName` 常量，Supply-api 用字符串
- Platform-token-runtime 待确认

---

## 四、共享包边界定义（初稿）

### 4.1 推荐共享包结构

```
internal/shared/
├── auth/
│   ├── errors.go        # 统一错误响应格式 + errorCode 常量
│   ├── token.go         # extractBearerToken, hasExternalQueryKey
│   ├── clientip.go      # extractClientIP（含可信代理逻辑）
│   ├── audit.go         # AuditEvent struct + AuditEmitter 接口
│   ├── context.go       # RequestID/Principal context 工具函数
│   └── README.md
├── logging/
│   ├── logger.go        # LogEntry + Logger 接口 + jsonLogger 实现
│   ├── fieldkeys.go     # FieldKeyXXX 常量
│   └── README.md
└── README.md
```

### 4.2 不适合共享的能力

1. JWT 验证逻辑（各服务实现各异）
2. Token 状态查询实现（各服务后端不同）
3. 授权策略（routeRoles 映射、role level 系统）
4. BruteForceProtection（Supply-api 独有）
5. 中间件链组合方式
6. Service-specific context key 类型

---

## 五、迁移顺序建议

根据 P4-A-05 定义迁移顺序：

**第一步：Logging 共享（风险最低）**
- 理由：三服务 logging 实现几乎相同，只是 supply-api 有额外接口和常量
- 依赖：无
- 改动范围：新建 `internal/shared/logging/`，三服务替换 import 路径

**第二步：Auth 基础能力共享**
- 理由：错误格式、token 提取、clientIP 提取都是工具函数，不涉及业务逻辑
- 依赖：logging 共享包
- 改动范围：新建 `internal/shared/auth/`，三服务替换对应函数调用

**第三步：Audit Event 结构共享**
- 理由：结构相同，但 event name 格式不同，需要统一常量
- 依赖：auth 基础能力
- 改动范围：修改 AuditEvent 定义 + event name 常量

**第四步：契约测试**
- 在共享包稳定后，写 `internal/shared/auth/auth_test.go` 覆盖 missing bearer、invalid token、inactive token

---

## 六、兼容适配层

为避免跨三服务一次性大爆炸改动，采用**追加兼容模式**：

1. **每个共享包先创建在 `internal/shared/`**，不删除原有代码
2. 各服务先修改 import 引用新包，验证通过后删除原有重复代码
3. 如有服务特有行为，通过接口嵌入或适配器模式处理
4. 每次改动后运行 `go build ./...` 和 `go test ./...` 验证

**关键约束：不允许跨三个服务同时大爆炸改动。**
