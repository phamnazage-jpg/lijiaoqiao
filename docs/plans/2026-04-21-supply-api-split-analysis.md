# P4-B 分析：supply-api 大文件拆分与未接入能力

生成时间: 2026-04-21
Phase: P4-B

---

## 一、supply_api.go 分区记录（P4-B-01 完成）

文件：`supply-api/internal/httpapi/supply_api.go`，1048 行

### 分区概览

| 分区 | 起始行 | 主要 handler | 说明 |
|------|--------|-------------|------|
| Account | 163 | handleVerifyAccount, handleCreateAccount, handleAccountActions, handleActivateAccount, handleSuspendAccount, handleDeleteAccount, handleAccountAuditLogs | 账号生命周期管理 |
| Package | 453 | handleCreatePackageDraft, handlePackageActions, handlePublishPackage, handlePausePackage, handleUnlistPackage, handleClonePackage, handleBatchUpdatePrice | 套餐生命周期管理 |
| Billing | 731 | handleGetBilling | 账单查询 |
| Settlement | 761 | handleWithdraw, handleSettlementActions, handleCancelSettlement, handleGetStatement | 结算与提现 |
| Earning | 918 | handleGetEarningRecords | 收益记录 |
| Helpers | 975 | writeJSON, writeError, getRequestID, getQueryInt, handleAuditEvent | 通用工具 |

### 拆分建议

```
supply-api/internal/httpapi/
├── supply_api.go          # 保留 Register + NewSupplyAPI（聚合入口）
├── account.go             # Account 分区 handler（~150行）
├── package.go             # Package 分区 handler（~230行）
├── billing.go             # Billing 分区 handler（~30行）
├── settlement.go          # Settlement 分区 handler（~160行）
├── earning.go             # Earning 分区 handler（~60行）
├── errors.go              # Helpers: writeJSON/writeError/writeSupplyActionError（~30行）
└── request.go             # Helpers: getRequestID/getQueryInt/resolveSupplierID（~30行）
```

**拆分原则**：每个分区有独立的 registerXXXRoutes(mux) 函数，主 supply_api.go 的 Register 调用各子包的注册函数。

---

## 二、runtime.go 分区记录（P4-B-02 完成）

文件：`supply-api/internal/app/runtime.go`，589 行

### 分区概览

| 分区 | 起始行 | 函数 | 说明 |
|------|--------|------|------|
| 输入解析 | 27 | RuntimeOptions 结构体 | 所有外部依赖注入 |
| 资源初始化 | 140 | BuildRuntime, buildRuntimeWithFactory, resolveRuntimeBuildInputs | 构建逻辑入口 |
| 外部资源初始化 | 222 | initializeRuntimeExternalResources, buildRuntimeResources | DB/Redis 连接初始化 |
| Store Bundle | 286 | buildStoreBundle, buildDBStoreBundle, buildMemoryStoreBundle | 所有 repository 聚合 |
| Security Bundle | 332 | buildSecurityBundle | 鉴权/JWT key/验证码等安全依赖 |
| API Bundle | 369 | buildAPIBundle | HTTP handler/API 路由依赖 |
| Server Bundle | 485 | BuildServer, resolveRuntimeHealthChecks, buildRuntimeHTTPView | HTTP server 构建 |
| 生命周期 | 549 | Close, ShutdownTimeout | 资源释放 |

### 拆分建议

```
supply-api/internal/app/
├── runtime.go             # RuntimeOptions + Runtime struct + BuildRuntime 入口
├── runtime_stores.go       # buildStoreBundle / buildDBStoreBundle / buildMemoryStoreBundle
├── runtime_security.go     # buildSecurityBundle
├── runtime_server.go       # BuildServer + buildRuntimeHTTPView
└── runtime_options.go     # RuntimeOptions 定义
```

---

## 三、auth.go 分区记录（P4-B-03 完成）

文件：`supply-api/internal/middleware/auth.go`，891 行

### 分区概览

| 分区 | 起始行 | 内容 | 说明 |
|------|--------|------|------|
| 配置与结构 | 25 | TokenClaims, AuthConfig, AuthMiddleware, AuditEmitter, AuditEvent | 类型定义 |
| 工厂方法 | 79 | NewAuthMiddleware | 构造器 |
| BruteForce 保护 | 98 | BruteForceProtection 结构 + 所有方法（277行） | MED-12: 防暴力破解 |
| Query Key Reject | 211 | QueryKeyRejectMiddleware | M-016: query key 拒绝 |
| Bearer Extract | 274 | BearerExtractMiddleware | Bearer token 提取 |
| Token Verify | 320 | TokenVerifyMiddleware + verifyToken + signingKey + parseRSAPublicKey | JWT 校验 |
| Scope/Role Authz | 449 | ScopeRoleAuthzMiddleware + containsScope | 权限校验 |
| Token Status | 595 | checkTokenStatus + GetTokenClaims | 状态查询 |
| Helpers | 589 | shouldBypassAuth + writeAuthError | 工具函数 |

### 拆分建议

```
supply-api/internal/middleware/auth/
├── auth.go                 # AuthConfig, AuthMiddleware, NewAuthMiddleware
├── auth_bearer.go          # BearerExtractMiddleware
├── auth_verify.go          # TokenVerifyMiddleware, verifyToken, signingKey
├── auth_authz.go           # ScopeRoleAuthzMiddleware, containsScope
├── auth_bruteforce.go      # BruteForceProtection
├── auth_errors.go          # writeAuthError, shouldBypassAuth
├── auth_status.go          # checkTokenStatus, GetTokenClaims
└── auth_test.go            # 测试
```

---

## 四、gateway 未接入能力清单（P4-B-04 完成）

已接入模块：Token Auth Middleware, Router, Circuit Breaker, Health Checker, Metrics, Remote Runtime Cache, Rate Limiter, Fallback Router

### 未接入 / 实验模块

| 模块 | 位置 | 状态 | 建议 |
|------|------|------|------|
| compliance/rules | `internal/compliance/rules/` | 已实现但未注册到路由 | 接入或移入 experimental/ |

### P4-B-05 决策

**待补充**：compliance/rules 模块接入路径或 experimental 转移决策。

---

## 五、InvariantChecker 去留决策（P4-B-06）

### 搜索结果

```
$ grep -rn "InvariantChecker" supply-api/internal/ --include="*.go" | grep -v "_test.go"
supply-api/internal/app/runtime.go:380:
    _ = domain.NewInvariantChecker(storeBundle.accountStore, ...)
supply-api/internal/domain/invariants.go:52: type InvariantChecker struct {
```

### 分析

- `InvariantChecker` 在 `buildSecurityBundle` 中创建（第380行）
- 创建后立即丢弃（`_ = domain.NewInvariantChecker(...)`），**从未被任何写路径调用**
- 检查方法有实际业务价值：
  - `CheckAccountDelete`: 删除账号前校验
  - `CheckAccountActivate`: 激活账号前校验
  - `CheckPackagePublish`: 发布套餐前校验
  - `CheckPackagePrice`: 调价前校验
  - `CheckSettlementCancel`: 取消结算前校验
  - `CheckWithdrawBalance`: 提现余额校验（防透支）

### P4-B-06 决策：**接入真实写路径**

理由：
1. InvariantChecker 已有完整测试覆盖（invariants_test.go）
2. 业务校验逻辑有价值，不应丢弃
3. 当前状态（创建但不用）比删除更危险——给人虚假信心

### P4-B-07 接入点顺序

```
1. settlement.go: Withdraw() — 接入 CheckWithdrawBalance（最关键，防透支）
2. settlement.go: CancelSettlement() — 接入 CheckSettlementCancel
3. account.go: DeleteAccount() — 接入 CheckAccountDelete
4. account.go: ActivateAccount() — 接入 CheckAccountActivate
5. package.go: PublishPackage() — 接入 CheckPackagePublish
6. package.go: UpdatePrice() — 接入 CheckPackagePrice
```

### 接入方式

在 `runtime.go` 的 `buildSecurityBundle` 中，将 `_ = domain.NewInvariantChecker(...)` 改为传入 `runtimeAPIBundle`，各 domain service 构造函数接收 `*InvariantChecker` 参数。

### 回归测试

- `go test ./internal/domain/...` — invariants_test.go 已有覆盖
- `go test ./internal/app/...` — 集成测试
- supply-api 全量 `go test ./...`

---

## 六、拆分执行计划（P4-B-07）

**第一步**：supply_api.go 分区（按 Account → Package → Billing → Settlement → Earning → Helpers 顺序，每次改一个分区，验证 `go build ./...` + `go test ./...`）

**第二步**：auth.go 分区（按 Errors → Bearer → Verify → Authz → BruteForce 顺序）

**第三步**：runtime.go 分区（Stores → Security → Server → Options）

**第四步**：删除 InvariantChecker（确认无引用后执行）

**关键约束**：每步改动后全量测试通过才能提交，不跨多个分区同时大爆炸改动。
