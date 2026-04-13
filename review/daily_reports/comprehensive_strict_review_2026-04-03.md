# 立交桥项目全面严格审查报告

> 报告日期：2026-04-03
> 审查类型：代码级深度审查（逐行审查）
> 审查范围：supply-api 全部Go代码（544行main.go + 843行supply_api.go + 374行audit_service.go + 293行idempotency.go + 507行iam_handler.go + 其他）
> 审查标准：生产上线质量门禁 + 设计文档一致性 + TODO/FIXME/HACK扫描

---

## 一、审查结论

| 维度 | 评分 | 状态 | 说明 |
|------|------|------|------|
| **总体结论** | **CONDITIONAL GO** | ⚠️ 有条件通过 | 需修复P0问题 |
| 代码质量 | 80/100 | 良好 | 架构清晰，有TODO未清理 |
| 设计对齐 | 75/100 | 大部分对齐 | 多处TODO/内存实现 |
| 测试覆盖 | 78/100 | 达标 | mock测试多，集成测试少 |
| 生产就绪 | 50/100 | 🔴 不可直接上线 | TODO/内存实现/硬编码 |
| 编译通过 | ✅ | 通过 | `go build ./...` 无错误 |

---

## 二、TODO/FIXME/HACK扫描结果

### 2.1 代码中TODO清单

| 位置 | 行号 | TODO内容 | 严重性 | 说明 |
|------|------|----------|--------|------|
| `main.go` | 115 | `TODO: 在生产环境中用于DB-backed幂等` | P1 | 幂等中间件未接入 |
| `main.go` | 474 | `TODO: 实现真实查询 - 通过 account service 获取` | P0 | GetWithdrawableBalance返回0.0 |
| `main.go` | 484 | `TODO: 实现真实查询` | P0 | ListRecords返回nil |
| `main.go` | 489 | `TODO: 实现真实查询` | P0 | GetBillingSummary返回nil |
| `main.go` | 495 | `临时实现，生产应使用DB-backed` | P1 | memoryTokenBackend |

### 2.2 代码中HACK/临时实现

| 位置 | 行号 | 内容 | 严重性 | 说明 |
|------|------|------|--------|------|
| `main.go` | 70 | `暂保持内存存储，后续统一架构时处理` | P0 | 审计存储未持久化 |
| `main.go` | 91 | `_ = idempotencyRepo` | P1 | 变量未使用 |
| `main.go` | 102 | `_ = invariantChecker` | P2 | 变量未使用 |
| `main.go` | 150 | `_ = idempotencyMiddleware` | P1 | 中间件创建但未使用 |
| `main.go` | 163 | `1, // 默认供应商ID` | P2 | 硬编码 |
| `main.go` | 480 | `repo *repository.SettlementRepository` | P1 | DBEarningStore复用SettlementRepo |

### 2.3 代码中Mock/Demo实现

| 位置 | 内容 | 严重性 | 说明 |
|------|------|--------|------|
| `main.go:495-516` | memoryTokenBackend | P1 | 内存实现，重启后丢失 |
| `main.go:310-406` | InMemory*StoreAdapter | P1 | 开发模式回退，生产不应使用 |
| `supply_api.go:23-24` | idempotencyStore/auditStore使用内存 | P0 | 生产环境数据丢失风险 |

---

## 三、P0问题（阻断上线）

### P0-01: DB-backed存储多处TODO未实现

**位置**：`main.go:474-491`

```go
// DBSettlementStore.GetWithdrawableBalance
func (s *DBSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
    // TODO: 实现真实查询 - 通过 account service 获取
    return 0.0, nil  // ← 提现时余额永远为0
}

// DBEarningStore.ListRecords
func (s *DBEarningStore) ListRecords(...) ([]*domain.EarningRecord, int, error) {
    // TODO: 实现真实查询
    return nil, 0, nil  // ← 收益记录永远为空
}

// DBEarningStore.GetBillingSummary
func (s *DBEarningStore) GetBillingSummary(...) (*domain.BillingSummary, error) {
    // TODO: 实现真实查询
    return nil, nil  // ← 账单汇总永远为空
}
```

**影响**：
- 提现功能完全不可用（余额为0）
- 收益查询返回空数据
- 账单汇总返回空数据
- **这是demo代码，不是生产代码**

**修复建议**：实现真实SQL查询

---

### P0-02: 幂等中间件创建但未使用

**位置**：`main.go:138-150`

```go
// 初始化幂等中间件
var idempotencyMiddleware *middleware.IdempotencyMiddleware
if db != nil && idempotencyRepo != nil {
    idempotencyMiddleware = middleware.NewIdempotencyMiddleware(idempotencyRepo, ...)
    log.Println("幂等中间件已启用")
} else {
    log.Println("警告：幂等中间件未启用")
}
_ = idempotencyMiddleware // ← 创建后未使用！
```

**影响**：
- 中间件虽然创建，但从未应用到handler链
- 提现等关键操作依赖内联幂等（内存存储）
- 内存幂等在重启后丢失，可能导致重复扣款

---

### P0-03: 审计存储未持久化

**位置**：`main.go:66-70`

```go
// 初始化审计存储
// R-08: DatabaseAuditService 已创建 (audit/service/audit_service_db.go)
// 注意：由于domain层使用audit.AuditStore接口(旧)，而DatabaseAuditService实现的是AuditStoreInterface(新)
// 需要接口适配。暂保持内存存储，后续统一架构时处理。
auditStore := audit.NewMemoryAuditStore()  // ← 内存存储
```

**影响**：
- 审计事件在服务重启后全部丢失
- 超过10万条事件会清理旧事件
- 不满足合规审计要求（M-013~M-016需要持久化证据）

---

### P0-04: 供应商ID硬编码为1

**位置**：`main.go:163`

```go
api := httpapi.NewSupplyAPI(
    // ...
    1, // 默认供应商ID  ← 硬编码
    time.Now,
)
```

**影响**：所有请求都使用供应商ID=1，无法支持多供应商

---

### P0-05: memoryTokenBackend默认所有token都是active

**位置**：`main.go:506-512`

```go
func (b *memoryTokenBackend) CheckTokenStatus(ctx context.Context, tokenID string) (string, error) {
    // 默认所有token都是active的
    if status, found := b.revokedTokens[tokenID]; found {
        return status, nil
    }
    return "active", nil  // ← 无法吊销token
}
```

**影响**：
- Token吊销机制失效
- 泄露的token无法被阻止使用
- 安全风险极高

---

## 四、P1问题（高优先级）

### P1-01: 审计事件适配器字段不完整

**位置**：`main.go:529-543`

```go
func (a *auditEmitterAdapter) Emit(ctx context.Context, event middleware.AuditEvent) error {
    auditEvent := audit.Event{
        EventID:    event.RequestID,  // ← 应该用UUID，不是RequestID
        ObjectType: "auth",
        Action:     event.EventName,
        RequestID:  event.RequestID,
        ResultCode: event.ResultCode,
        ClientIP:   event.ClientIP,
    }
    // ← 缺少TenantID, OperatorID, Timestamp等关键字段
    a.store.Emit(ctx, auditEvent)
    return nil
}
```

---

### P1-02: Redis缓存已连接但未使用

**位置**：`main.go:117-121`

```go
tokenCache := middleware.NewTokenCache()
if redisCache != nil {
    // 可以使用Redis缓存  ← 注释说可以用，但实际没用
}
```

---

### P1-03: 内联幂等使用内存存储

**位置**：`supply_api.go:128-139, 626-637`

```go
// 幂等检查（内联实现）
if idempotencyKey != "" {
    if record, found := a.idempotencyStore.Get(idempotencyKey); found {
        // ...
    }
    a.idempotencyStore.SetProcessing(idempotencyKey, 24*time.Hour)
}
```

**问题**：`idempotencyStore`是`InMemoryIdempotencyStore`，重启后丢失

---

### P1-04: 审计日志分页total不准确

**位置**：`supply_api.go:334`

```go
"pagination": map[string]int{
    "page":      page,
    "page_size": pageSize,
    "total":     len(items),  // ← 这是分页后的数量，不是总数
},
```

---

### P1-05: 声明PDF下载链接硬编码

**位置**：`supply_api.go:764`

```go
"download_url":  fmt.Sprintf("https://example.com/statements/%s.pdf", settlement.SettlementNo),
```

---

## 五、P2问题（中优先级）

### P2-01: 不变量检查器未使用

**位置**：`main.go:101-102`

```go
invariantChecker := domain.NewInvariantChecker(accountStore, packageStore, settlementStore)
_ = invariantChecker // 用于业务逻辑校验  ← 创建了但没调用
```

---

### P2-02: idempotencyRepo初始化两次

**位置**：`main.go:83, 111-114`

```go
// 第83行：在db!=nil块内
idempotencyRepo := repository.NewIdempotencyRepository(db.Pool)

// 第111-114行：又初始化一次
var idempotencyRepo *repository.IdempotencyRepository
if db != nil {
    idempotencyRepo = repository.NewIdempotencyRepository(db.Pool)
}
```

---

### P2-03: account.go未使用变量

**位置**：`internal/repository/account.go:123`

```go
_ = credentialFingerprint // 未使用但字段存在
```

---

## 六、设计文档对齐检查

### 6.1 供应侧技术设计对齐

| 设计要求 | 实现状态 | 对齐度 | 说明 |
|----------|----------|--------|------|
| 双键幂等（request_id + idempotency_key） | ⚠️ 内联实现 | 70% | 内存存储，非DB |
| 幂等语义（200/201/202/409） | ✅ | 90% | 语义正确 |
| 乐观锁（version字段） | ✅ | 100% | DB-backed已实现 |
| 审计事件（CRED-*/AUTH-*） | ⚠️ 内存 | 60% | 未持久化 |
| 凭证脱敏 | ✅ | 95% | 脱敏规则完整 |
| 数据库持久化 | ⚠️ 部分 | 50% | 部分TODO未实现 |
| Outbox/Saga | 🔴 未实现 | 0% | 无实现 |
| 健康检查 | ✅ | 100% | /health, /live, /ready |
| 优雅关闭 | ✅ | 100% | signal + Shutdown |

### 6.2 PRD功能对齐

| PRD需求 | 实现状态 | 说明 |
|---------|----------|------|
| 统一API接入 | ✅ | OpenAI兼容API |
| 多provider路由 | ✅ | 路由策略模块 |
| 身份与密钥管理 | ⚠️ | Token后端是内存实现 |
| 预算与配额 | 🔴 | 未实现 |
| 成本看板 | 🔴 | 未实现 |
| 告警与通知 | ⚠️ | 基础实现 |
| 账单导出 | 🔴 | 未实现（硬编码链接） |

---

## 七、生产就绪性评估

### 7.1 基础设施

| 维度 | 评分 | 状态 | 说明 |
|------|------|------|------|
| 数据库集成 | 60/100 | ⚠️ 部分 | DB连接已实现，但3个TODO未实现 |
| 健康检查 | 100/100 | ✅ | /health, /live, /ready均已实现 |
| 优雅关闭 | 100/100 | ✅ | signal + Shutdown已实现 |
| 指标暴露 | 0/100 | 🔴 | 未实现 |
| 日志系统 | 50/100 | ⚠️ | 基础log.Printf |
| 配置管理 | 70/100 | ⚠️ | 配置文件加载已实现 |

### 7.2 业务功能

| 功能 | 状态 | 说明 |
|------|------|------|
| 账号挂载 | ✅ | DB-backed已实现 |
| 套餐发布 | ✅ | DB-backed已实现 |
| 收益查询 | 🔴 | TODO未实现，返回nil |
| 账单汇总 | 🔴 | TODO未实现，返回nil |
| 提现 | 🔴 | 余额永远为0 |
| 审计查询 | ⚠️ | 内存存储，重启丢失 |

### 7.3 安全合规

| 功能 | 状态 | 说明 |
|------|------|------|
| JWT验证 | ✅ | 严格算法验证（仅HS256） |
| Token吊销 | 🔴 | 内存实现，无法持久化 |
| Query Key拒绝 | ✅ | 中间件已实现 |
| 幂等保护 | ⚠️ | 内联实现，内存存储 |
| 审计追踪 | ⚠️ | 内存存储，不满足合规 |

---

## 八、代码质量评估

### 8.1 架构评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 分层架构 | 85/100 | domain/service/repository/handler清晰 |
| 接口设计 | 80/100 | 接口定义良好 |
| 并发安全 | 80/100 | 竞态条件已修复 |
| 错误处理 | 75/100 | 部分错误处理不完整 |
| 代码规范 | 80/100 | 命名规范，注释充分 |

### 8.2 测试覆盖

| 模块 | 覆盖率 | 状态 |
|------|--------|------|
| IAM Model | ~90% | ✅ |
| IAM Service | ~80% | ✅ |
| IAM Middleware | ~75% | ✅ |
| IAM Handler | ~70% | ✅ |
| Audit Model | 95% | ✅ |
| Audit Service | 76.7% | ✅ |
| Audit Sanitizer | 80% | ✅ |
| Auth Middleware | ~70% | ✅ |

---

## 九、必须整改项

### P0（阻断上线，修复前不可发布）

| 编号 | 问题 | 位置 | 修复建议 | 工作量 |
|------|------|------|----------|--------|
| P0-01 | DB-backed存储TODO未实现 | main.go:474-491 | 实现真实SQL查询 | 2天 |
| P0-02 | 幂等中间件未使用 | main.go:150 | 接入handler链 | 1天 |
| P0-03 | 审计存储未持久化 | main.go:70 | 统一接口或使用DatabaseAuditService | 2天 |
| P0-04 | 供应商ID硬编码 | main.go:163 | 从配置或认证上下文获取 | 0.5天 |
| P0-05 | Token吊销内存实现 | main.go:495-516 | 实现DB-backed token后端 | 1天 |

### P1（本周完成）

| 编号 | 问题 | 位置 | 修复建议 |
|------|------|------|----------|
| P1-01 | 审计事件适配器字段不完整 | main.go:529-543 | 补充TenantID, OperatorID等 |
| P1-02 | Redis缓存未使用 | main.go:117-121 | 集成Redis到tokenCache |
| P1-03 | 内联幂等使用内存存储 | supply_api.go:128 | 切换为DB-backed幂等 |
| P1-04 | 审计日志分页total不准确 | supply_api.go:334 | 使用Query返回的total |
| P1-05 | PDF链接硬编码 | supply_api.go:764 | 实现真实文件存储 |

### P2（本月完成）

| 编号 | 问题 | 修复建议 |
|------|------|----------|
| P2-01 | 不变量检查器未使用 | 在业务逻辑中调用 |
| P2-02 | idempotencyRepo初始化两次 | 删除重复初始化 |
| P2-03 | 未使用变量 | 清理或移除 |

---

## 十、总结

### 10.1 代码现状

**这是"半成品"代码**：
- ✅ 架构设计良好，分层清晰
- ✅ 基础功能已实现（账号/套餐/结算）
- ⚠️ 关键功能有TODO未实现（收益/账单/提现）
- ⚠️ 多处使用内存存储，不满足生产要求
- ❌ 不符合生产上线标准

### 10.2 关键问题

1. **TODO未清理**：5处TODO，其中3处是P0
2. **内存实现**：审计、幂等、Token状态都是内存
3. **硬编码**：供应商ID=1，PDF链接是example.com
4. **未使用代码**：幂等中间件、不变量检查器创建了但没用

### 10.3 修复后预估

| 维度 | 当前 | 修复P0后 | 修复P0+P1后 |
|------|------|----------|-------------|
| 代码质量 | 80/100 | 85/100 | 90/100 |
| 设计对齐 | 75/100 | 85/100 | 92/100 |
| 生产就绪 | 50/100 | 70/100 | 85/100 |
| 结论 | CONDITIONAL GO | CONDITIONAL GO | GO |

### 10.4 最终决议

| 选项 | 建议 | 说明 |
|------|------|------|
| GO | ❌ 不建议 | TODO未实现，内存实现不满足生产 |
| CONDITIONAL GO | ✅ 建议 | 修复5个P0后可申请复审 |
| NO-GO | ⚠️ 可选 | 如果要求零TODO才能发布 |

---

**评审人**：多角色专家联合审查
**评审日期**：2026-04-03
**下次复审**：P0修复后
