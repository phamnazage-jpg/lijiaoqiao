# Code Quality Review: platform-token-runtime

**Date**: 2026-04-18
**Module**: `lijiaoqiao/platform-token-runtime`
**Go Version**: 1.22
**Reviewer**: Hermes Agent

---

## 1. 模块边界 (Module Boundaries)

### 1.1 目录结构

```
cmd/platform-token-runtime/      # 入口
internal/app/                    # 装配 / bootstrap
internal/httpapi/                 # HTTP 接口层
internal/auth/
  middleware/                     # HTTP 中间件
  model/                         # 数据模型
  service/                       # 核心业务逻辑 (runtime store, audit store)
internal/token/                  # token 生命周期测试
```

### 1.2 边界评估: 良好

**优点**：
- `internal/` 布局符合 Go 最佳实践，不暴露内部包
- `RuntimeStore` / `AuditStore` 接口契约清晰，上层 (`httpapi`, `middleware`) 仅依赖接口而非实现
- `bootstrap.go` 统一了环境判断 (dev/prod/staging) 和 store 装配逻辑
- `TokenAPI` 依赖注入接口 (`Runtime`, `AuditEmitter`, `AuditEventQuerier`)，而非具体实现

**问题**：
1. **设计边界与 README 描述不一致**：`README.md` 中 TOKEN_RUNTIME_ENV 值被 `***` 遮蔽，无法确认实际支持的环境名，审查者无法验证 prod/staging 判断逻辑是否与 README 描述一致

2. **middleware 与 httpapi 职责重叠**：`httpapi.handleRefresh` 在第 196 行手动 emit 审计事件，但 middleware 链也会 emit 审计事件。`TokenAPI` 同时承担了 HTTP 处理和部分业务编排职责，边界不够清晰

3. **硬编码路径**：`token_api.go` 中 `tokenBasePath = "/api/v1/platform/tokens"` 和 `ScopeRoleAuthorizer` 中 `"/api/v1/supply"`, `"/api/v1/platform"` 硬编码分散，可能导致路径不一致

4. **`internal/token/` 仅为测试文件**：该目录仅含 `*_test.go`，作为测试模板使用，但目录名称不符合 Go package 命名规范（test 包通常用 `_test` 后缀而非独立目录）

---

## 2. 错误处理模式 (Error Handling)

### 2.1 错误定义

`service/errors.go` (实为 `token_verifier.go` 中的 `AuthError`) 提供了结构化错误：

```go
type AuthError struct {
    Code  string
    Cause error
}
```

- 实现了 `Error()` 和 `Unwrap()` 方法
- 提供 `NewAuthError(code, cause)` 工厂函数
- 提供 `IsAuthCode(err, code)` 断言函数

### 2.2 HTTP 层错误映射

`token_api.go` 的 `mapRuntimeError` 使用字符串包含判断：

```go
func mapRuntimeError(err error) (int, string) {
    msg := err.Error()
    switch {
    case strings.Contains(msg, "not found"):
        return http.StatusNotFound, "TOKEN_NOT_FOUND"
    case strings.Contains(msg, "not active"):
        return http.StatusConflict, "TOKEN_NOT_ACTIVE"
    // ...
    }
}
```

**问题**：
1. **脆弱的字符串匹配**：`Refresh` 返回 `"token is not active"` 但判断条件是 `"not active"`（无 "token" 前缀），逻辑不严谨
2. **错误信息无法本地化**：所有错误信息直接透传给客户端，可能暴露内部状态
3. **幂等性检查不统一**：`handleIssue` 用 `strings.Contains(err.Error(), "idempotency key payload mismatch")` 特殊处理，而 `handleRefresh` / `handleRevoke` 使用通用的 `mapRuntimeError`

### 2.3 Store 层 Nil 检查

PostgreSQL store 实现存在 nil receiver 检查：

```go
func (s *PostgresRuntimeStore) Save(...) error {
    if s == nil || s.db == nil {
        return errors.New("postgres runtime store is not configured")
    }
    // ...
}
```

**正面**：防御性 nil 检查，防止 nil receiver panic

**问题**：
- 错误信息不精确：`"not configured"` vs 实际可能是 `"connection pool exhausted"` 等
- 这些检查本应在构造函数或依赖注入阶段完成，运行时检查暴露了初始化顺序问题

### 2.4 错误传播

`emitAudit` 在 `audit_store.go` 中忽略 error：

```go
func emitAudit(emitter AuditEmitter, event AuditEvent, now func() time.Time) {
    _ = emitter.Emit(context.Background(), event)
}
```

**问题**：审计失败被静默忽略，operator 无法感知审计丢失

---

## 3. 命名规范 (Naming Conventions)

### 3.1 包和文件

| 文件 | 包名 | 评估 |
|------|------|------|
| `bootstrap.go` | `app` | 合理，职责明确 |
| `token_api.go` | `httpapi` | 合理 |
| `token_verifier.go` | `service` | 混合了接口定义、常量和错误类型，文件名称不能反映全部内容 |
| `inmemory_runtime.go` | `service` | 包含 `TokenRecord`, `InMemoryTokenRuntime` 等，职责较多 |

### 3.2 接口命名

```
RuntimeStore        # 清晰
AuditStore          # 清晰
AuditEmitter        # 清晰
AuditEventQuerier   # 清晰
TokenVerifier       # 清晰
TokenStatusResolver # 清晰
RouteAuthorizer     # 清晰
```

### 3.3 函数和变量

| 名称 | 评估 |
|------|------|
| `newPostgresStoreBundle` | 函数变量命名，带 `new` 前缀但非构造函数，设计意图模糊 |
| `InMemoryTokenRuntime` vs `InMemoryRuntimeStore` | 两者都管理 token 生命周期，但职责不同（runtime 编排 vs store 持久化），命名相似容易混淆 |
| `CodeAuthMissingBearer` 常量 | 使用全大写但作为错误码暴露给 HTTP API，建议使用更稳定的标识符 |
| `pgxRuntimeStoreDB` / `pgxAuditStoreDB` | 内部包装类型，命名合理 |
| `accessTokenFingerprint` | 良好，清晰表达了计算指纹而非存储原始 token |

### 3.4 缩写一致性

- `URL` 使用全大写（Go 惯例）
- `ID` 在 `TokenID`, `SubjectID` 中为大写，符合 Go 1.22+ 建议

---

## 4. 并发安全 (Concurrency Safety)

### 4.1 In-Memory Store

`InMemoryRuntimeStore` 使用 `sync.RWMutex` 保护所有 map 操作：

```go
type InMemoryRuntimeStore struct {
    mu               sync.RWMutex
    records          map[string]*TokenRecord
    tokenToID        map[string]string
    idempotencyByKey map[string]IdempotencyEntry
}
```

**优点**：
- 读操作使用 `RLock`，写操作使用 `Lock`
- 所有方法都正确使用 mutex 保护
- `cloneRecord` 防止返回内部指针

**问题**：
1. **`cloneRecord` 只复制 Scope**：`TokenRecord` 中的 `TokenID`, `AccessToken` 等 string 字段本身不可变，但 `ExpiresAt` / `IssuedAt` 是值类型，隐式安全
2. **map 不安全**：`sync.RWMutex` 不能保证 map 并发安全（Go 1.21+ 仍要求额外同步），但由于 mutex 覆盖了所有操作，逻辑上安全

### 4.2 InMemoryTokenRuntime

```go
type InMemoryTokenRuntime struct {
    mu    sync.RWMutex
    now   func() time.Time
    store RuntimeStore
}
```

**并发模式**：
- `Issue` 持有 `mu.Lock()` 进行幂等性检查和保存
- `Refresh` / `Revoke` / `Introspect` / `Lookup` 均持有 `mu.Lock()`
- `Verify` / `Resolve` 使用 `mu.RLock()`（读多写少场景优化）

**问题**：
1. **锁粒度**：整个 `Issue` 期间持有锁，包括调用 `r.store.Save()`（可能涉及数据库 I/O），在高并发下会串行化
2. **Refresh 中两次 store.Save**：第 156 行和第 165 行各一次，如果中间失败会导致状态不一致

### 4.3 MemoryAuditStore

```go
type MemoryAuditStore struct {
    mu     sync.RWMutex
    events []AuditEvent
    now    func() time.Time
}
```

- `Emit` 使用 `Lock` 写锁
- `Events()` / `QueryEvents()` 使用 `RLock` 读锁
- `LastEvent()` 使用 `RLock`

**良好实践**：正确使用读写锁分离读写场景

### 4.4 Context 使用

多处存在 `if ctx == nil { ctx = context.Background() }` 模式：

```go
func (r *InMemoryTokenRuntime) Issue(ctx context.Context, input IssueTokenInput) (TokenRecord, error) {
    if ctx == nil {
        ctx = context.Background()
    }
    // ...
}
```

**问题**：
- Go 中约定 `nil context` 是合法的，且应该被信任
- 但项目多处手动替换为 `context.Background()`，增加了代码体积
- 如果调用者忘记传递 context，会获得意外的长生命周期

---

## 5. 测试质量 (Test Quality)

### 5.1 测试覆盖

| 包 | 测试文件 | 覆盖内容 |
|----|----------|----------|
| `cmd/platform-token-runtime` | `main_test.go` | 启动边界 (prod 拒绝内存 store) |
| `internal/app` | `bootstrap_test.go` | BuildRuntime, BuildServer, BuildPostgresStores |
| `internal/httpapi` | `token_api_test.go` | issue/refresh/revoke/introspect/audit-events |
| `internal/auth/service` | `runtime_store_test.go`, `postgres_runtime_store_test.go`, `store_contract_test.go`, `audit_store_test.go`, `postgres_audit_store_test.go` | store 单元测试 |
| `internal/auth/middleware` | `token_auth_middleware_test.go` | 中间件链 |
| `internal/token` | `lifecycle_executable_test.go` | token 生命周期 |
| `internal/auth/model` | `model_test.go` | 模型方法 |

### 5.2 测试设计亮点

1. **Store 契约测试** (`store_contract_test.go`)：验证接口实现
2. **带 Mock 的完整中间件链测试**：`fakeVerifier`, `fakeStatusResolver`, `fakeAuthorizer`, `fakeAuditor`
3. **幂等性测试**：`TestTOKLife003IssueIdempotencyReplay` 覆盖幂等冲突场景
4. **Helper Process 模式**：`main_test.go` 使用子进程测试实际启动行为
5. **时间控制**：使用固定时间函数（`time.Now` 替代）测试时间相关逻辑

### 5.3 测试质量问题

1. **TestTOKLife007ExpiredTokenInactive (第 222-270 行)**：
   ```go
   current := time.Date(2026, 3, 29, 15, 0, 0, 0, time.UTC)
   rt := service.NewInMemoryTokenRuntime(func() time.Time { return current })
   // ...
   current = current.Add(3 * time.Second)  // 修改外部变量
   ```
   闭包捕获 `current` 变量，但修改发生在创建 runtime 之后。注意 runtime 内部持有 `now` 函数副本，`applyExpiry` 使用的是 runtime 创建时的 `now`（即返回 `current` 初始值的函数），所以测试逻辑正确

2. **fakeRuntimeRow.Scan 类型断言**：
   ```go
   case *[]byte:
       *d = append((*d)[:0], r.values[i].([]byte)...)
   ```
   `append` 对 nil slice 的处理是正确的，但断言失败会 panic（测试代码可接受）

3. **测试覆盖缺失**：
   - 没有并发/竞态条件测试（`go test -race` 通过但不意味着逻辑正确）
   - `PostgresRuntimeStore.querySingleRecord` 没有针对扫描失败场景的测试
   - `PostgresAuditStore.QueryEvents` 的 SQL 动态构建（`WHERE 1=1`）没有覆盖 WHERE 子句组合的测试

4. **TestTOKLife006RevokedTokenAccessDenied (第 179-220 行)**：
   测试验证 revoked token 被 middleware 拒绝，但 refresh/revoke 端点没有测试并发 refresh 场景

5. **`internal/token/` 目录命名**：
   - 仅包含 `_test.go` 文件（`lifecycle_executable_test.go`, `audit_test_template_test.go` 等）
   - Go package 名称为 `token_test`（显式导入 `token_test`），不符合标准实践
   - 这些文件应该放在对应的业务包（如 `service`）中，使用 `_test.go` 后缀

6. **测试文件命名**：
   - `audit_test_template_test.go` / `lifecycle_test_template_test.go` 包含 `_template_` 后缀，可能表明这些是测试模板或基类，但与 Go 测试文件命名规范不符

---

## 6. 安全考量 (Security)

### 6.1 Token 安全

- Access token 在内存中存储，但 `InMemoryRuntimeStore.Save` 复制了 `AccessToken` 到 `tokenToID` map
- `PostgresRuntimeStore` 使用 SHA-256 fingerprint 而非存储原始 token（良好）
- Access token 格式：`"ptk_" + hex(16 bytes)`，熵值 128 bit

### 6.2 审计事件中的 Token 安全

`handleAuditEvents` 中审计查询结果不包含 `access_token` 明文（仅 `token_id`），符合 README 承诺

### 6.3 Query Key 拒绝

`QueryKeyRejectMiddleware` 拒绝 `key`, `api_key`, `token` query 参数，设计边界明确

### 6.4 IP 欺骗防护

`extractClientIP` 正确实现了可信代理检查，非可信来源不信任 `X-Forwarded-For`

---

## 7. 总结

### 评分

| 维度 | 评分 (1-5) | 说明 |
|------|------------|------|
| 模块边界 | 4 | 接口契约清晰，但 httpapi 与 middleware 职责有重叠 |
| 错误处理 | 3 | 结构化错误设计良好，但 HTTP 层依赖字符串匹配不严谨 |
| 命名规范 | 4 | 整体良好，少量文件职责过重命名未能反映 |
| 并发安全 | 4 | 正确使用读写锁，但锁粒度可优化 |
| 测试质量 | 4 | 覆盖全面，mock 使用得当，但并发测试和 SQL 组合测试缺失 |

### 主要改进建议

1. **错误处理**：用 `errors.Is` / `errors.As` 替代字符串匹配，或为每种业务错误定义独立 sentinel error
2. **锁粒度**：将 `Issue` 中的幂等性检查与 store 写入分离，避免在锁内进行 I/O
3. **Context 约定**：移除手动 nil check，统一由调用者保证 context 合法性
4. **测试目录**：将 `internal/token/` 中的测试模板文件移入对应业务包
5. **审计失败感知**：审计 Emit 失败应至少记录日志（使用 `log.Printf` 或注入 logger）
