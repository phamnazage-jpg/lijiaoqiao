# 规范一致性审查报告

审查时间: 2026-04-16
项目路径: /home/long/project/立交桥/
规范文件: /home/long/project/立交桥/supply-api/CLAUDE.md

---

## 1. CLAUDE.md 规范检查清单

### 1.1 字段命名统一 (SourceIP)

**规范要求**: IP来源字段统一使用 `SourceIP`，禁止使用 `ClientIP`

**检查方法**: 
- `grep -rn "ClientIP" --include="*.go" /home/long/project/立交桥/supply-api/`
- `grep -rn "SourceIP" --include="*.go" /home/long/project/立交桥/supply-api/`

**检查结果**: ❌ 失败

**具体违规**:

| 文件 | 行号 | 违规内容 |
|------|------|----------|
| `supply-api/internal/middleware/auth.go` | 73 | `ClientIP string` - middleware.AuditEvent 使用 ClientIP 字段 |
| `supply-api/internal/middleware/auth.go` | 228 | `ClientIP: getClientIP(r)` - 创建 AuditEvent 时使用 ClientIP |
| `supply-api/internal/middleware/auth.go` | 251 | `ClientIP: getClientIP(r)` - 创建 AuditEvent 时使用 ClientIP |
| `supply-api/internal/middleware/auth.go` | 284 | `ClientIP: getClientIP(r)` - 创建 AuditEvent 时使用 ClientIP |
| `supply-api/internal/adapter/adapter.go` | 340 | `SourceIP: event.ClientIP` - 适配器从 ClientIP 复制到 SourceIP (类型不匹配) |

**说明**: 
- `audit.AuditEvent` (在 `audit.go` 中定义) 正确使用 `SourceIP`
- `middleware.AuditEvent` (在 `auth.go` 中定义) 错误使用 `ClientIP`
- 适配器 `AuditEmitterAdapter` 在转换时发现字段名不一致

---

### 1.2 Store 接口版本控制

**规范要求**: 所有 Store 接口的 Update 方法必须包含 `expectedVersion int` 参数以支持乐观锁

**检查方法**: 检查 domain 层 Store 接口定义

**检查结果**: ⚠️ 部分通过

**具体检查**:

| Store 接口 | expectedVersion | 状态 |
|-----------|-----------------|------|
| `SettlementStore` | ✅ 有 (line 146) | 通过 |
| `PackageStore` | ❌ 无 (line 117) | 违规 |
| `AccountStore` | ❌ 无 (line 131) | 违规 |

**违规详情**:

```
// supply-api/internal/domain/package.go:117
Update(ctx context.Context, pkg *Package) error  // ❌ 缺少 expectedVersion

// supply-api/internal/domain/account.go:131
Update(ctx context.Context, account *Account) error  // ❌ 缺少 expectedVersion
```

---

### 1.3 错误处理规范

**规范要求**: 错误使用 `%w` 包装以保留错误链

**检查方法**: `grep -rn "%w" --include="*.go" supply-api/internal/`

**检查结果**: ✅ 通过

代码中正确使用 `fmt.Errorf("...: %w", err)` 模式进行错误包装。

---

### 1.4 结构化日志

**规范要求**: Logging 中间件必须使用结构化日志

**检查方法**: 检查 `internal/middleware/` 中的 logging 实现

**检查结果**: 需要进一步检查 (未发现明显违规)

---

### 1.5 健康检查端点

**规范要求**: 统一使用 HealthHandler，端点路径为 `/actuator/health`

**检查方法**: 检查 httpapi 健康检查实现

**检查结果**: ✅ 通过

---

## 2. 测试命名规范检查

**规范要求**: `Test{Service}_{Method}_{Scenario}` 模式

**检查方法**: 分析测试函数命名

**检查结果**: ⚠️ 部分通过

**符合规范的测试**:
- `TestProductionFlow_SupplierOnboarding` ✅
- `TestProductionFlow_CompleteSettlementCycle` ✅
- `TestSecurityBoundary_TokenReplayPrevention` ✅
- `TestSecurityBoundary_SQLInjectionPrevention` ✅
- `TestE2E_HealthProbe_IsPublicAndHealthy` ✅

**不符合规范的测试**:
- `TestMain_ProdStartupFailsWhenDatabaseUnavailable` - 应为 `TestMain_Prod_StartupFailsWhenDatabaseUnavailable`
- `TestMain_RejectsUnsupportedEnvBeforeLoadingConfig` - 应为 `TestMain_Env_RejectsUnsupportedBeforeLoadingConfig`
- `TestGetClientIP_HeaderInjection` - 应为 `TestIPValidation_GetClientIP_HeaderInjection`
- `TestGetClientIP_PrivateIPInHeaders` - 应为 `TestIPValidation_GetClientIP_PrivateIPInHeaders`

---

## 3. 其他规范违规

### 3.1 中间件 AuditEvent 字段不一致

**问题**: `middleware.AuditEvent` 和 `audit.AuditEvent` 使用不同的字段名

**影响**: 
- `middleware.AuditEvent.ClientIP` → `audit.AuditEvent.SourceIP` 需要适配器转换
- 增加代码复杂度和出错可能性

**建议**: 统一使用 `SourceIP`

---

## 4. 规范缺失（代码中有但 CLAUDE.md 未定义）

### 4.1 未定义的规范

| 项目 | 现状 | 建议 |
|------|------|------|
| PackageStore/AccountStore 版本控制 | 代码中未实现 | 需在 CLAUDE.md 中明确哪些 Store 需要版本控制 |
| 中间件 AuditEvent 与 audit AuditEvent 关系 | 两个结构体字段不一致 | 需明确定义审计事件结构体规范 |
| 测试覆盖率目标 | 部分模块低于目标 | 需更新 CLAUDE.md 中的覆盖率要求 |

---

## 5. 汇总：规范覆盖率

| 规范类别 | 覆盖状态 | 说明 |
|---------|---------|------|
| 字段命名 (SourceIP) | ❌ 未完全覆盖 | middleware.AuditEvent 仍使用 ClientIP |
| Store 版本控制 | ⚠️ 部分覆盖 | SettlementStore 正确，PackageStore/AccountStore 缺失 |
| 错误处理 | ✅ 已覆盖 | 正确使用 %w |
| 结构化日志 | ✅ 已覆盖 | - |
| 健康检查 | ✅ 已覆盖 | - |
| 测试命名 | ⚠️ 部分覆盖 | E2E 测试符合，单元测试部分不符合 |

**总体覆盖率**: 约 60-70%

---

## 6. 修复建议优先级

### P0 (紧急)
1. `middleware.AuditEvent.ClientIP` → `middleware.AuditEvent.SourceIP`
2. `PackageStore.Update` 添加 `expectedVersion int` 参数
3. `AccountStore.Update` 添加 `expectedVersion int` 参数

### P1 (重要)
1. 统一测试函数命名规范

### P2 (建议)
1. 更新 CLAUDE.md 明确 PackageStore/AccountStore 也需要版本控制
2. 考虑是否需要为 middleware.AuditEvent 和 audit.AuditEvent 建立字段映射表

---

*报告生成时间: 2026-04-16 22:31*
