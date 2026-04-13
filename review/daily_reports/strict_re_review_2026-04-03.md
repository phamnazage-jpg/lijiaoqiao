# 立交桥项目严格复审报告（代码级深度审查）

> 报告日期：2026-04-03
> 评审类型：复审（验证前次问题修复 + 新发现）
> 审查范围：supply-api 全部Go代码 + 设计文档对齐
> 评审标准：生产上线质量门禁 + 设计文档一致性

---

## 一、复审结论

| 维度 | 上次评分 | 本次评分 | 变化 | 说明 |
|------|----------|----------|------|------|
| **总体结论** | CONDITIONAL GO | CONDITIONAL GO | 持平 | 有改进但仍有阻塞项 |
| 代码质量 | 75/100 | **82/100** | +7 | 竞态/硬编码已修复 |
| 设计对齐 | 70/100 | **78/100** | +8 | DB集成已实现 |
| 测试覆盖 | 78/100 | **80/100** | +2 | 事件ID改用UUID |
| 安全合规 | 72/100 | **76/100** | +4 | 幂等响应码修复 |
| 生产就绪 | 35/100 | **55/100** | +20 | 健康检查/优雅关闭/DB已实现 |

---

## 二、前次P0问题验证

### ✅ 已修复

| 编号 | 前次问题 | 修复状态 | 验证证据 |
|------|----------|----------|----------|
| P0-01 | IAM Handler硬编码userID=1 | ✅ 已修复 | `iam_handler.go:380-384` 添加userID==0检查 |
| P0-02 | getUserIDFromContext返回1 | ✅ 已修复 | `iam_handler.go:505-506` 调用`middleware.GetOperatorID(ctx)` |
| P0-03 | 无数据库持久化 | ✅ 已修复 | `main.go:78-98` DB连接+DB-backed存储适配器已实现 |
| P0-04 | 审计服务内存存储 | ⚠️ 部分修复 | 事件ID改用UUID，但main.go仍用内存存储 |

### ✅ 已修复（前次P1）

| 编号 | 前次问题 | 修复状态 | 验证证据 |
|------|----------|----------|----------|
| P1-01 | 审计服务幂等竞态条件 | ✅ 已修复 | `audit_service.go:234-235` 使用`defer s.idempotencyMu.Unlock()` |
| P1-02 | 幂等中间件响应码硬编码200 | ✅ 已修复 | `idempotency.go:174-193` 实现`statusCapturingResponseWriter` |
| P1-03 | 无健康检查端点 | ✅ 已修复 | `main.go:158-160` /actuator/health, /live, /ready |
| P1-04 | 无优雅关闭 | ✅ 已修复 | `main.go:211-225` signal.Notify + srv.Shutdown |

---

## 三、新发现问题

### 3.1 🔴 P0（阻断上线）

#### NEW-P0-01: 审计存储仍使用内存，未对接数据库

**严重程度**：P0（阻断）
**代码位置**：`main.go:70`

```go
// 初始化审计存储
// R-08: DatabaseAuditService 已创建 (audit/service/audit_service_db.go)
// 注意：由于domain层使用audit.AuditStore接口(旧)，而DatabaseAuditService实现的是AuditStoreInterface(新)
// 需要接口适配。暂保持内存存储，后续统一架构时处理。
auditStore := audit.NewMemoryAuditStore()
```

**影响**：
1. 审计事件在服务重启后全部丢失
2. 超过10万条事件会清理旧事件（`cleanupOldEvents`）
3. 不满足合规审计要求（M-013~M-016需要持久化证据）
4. 无法支持跨实例审计查询

**修复建议**：
1. 统一`audit.AuditStore`和`AuditStoreInterface`接口
2. 使用`DatabaseAuditService`替换`MemoryAuditStore`
3. 或创建适配器桥接两个接口

---

#### NEW-P0-02: 幂等中间件未实际使用

**严重程度**：P0（阻断）
**代码位置**：`main.go:132-137`

```go
// 初始化幂等中间件
idempotencyMiddleware := middleware.NewIdempotencyMiddleware(nil, middleware.IdempotencyConfig{
    TTL:     24 * time.Hour,
    Enabled: *env != "dev",
})
_ = idempotencyMiddleware // TODO: 在生产环境中用于幂等处理
```

**影响**：
1. 生产环境幂等保护完全缺失
2. 重复请求可能导致重复扣款/重复创建
3. 提现操作无幂等保护是资金风险

**修复建议**：
1. 将`idempotencyMiddleware`应用到HTTP handler链
2. 或移除中间件，使用`supply_api.go`中的内联幂等逻辑

---

#### NEW-P0-03: 路由注册两次

**严重程度**：P0（阻断）
**代码位置**：`main.go:163, 191`

```go
// 注册API路由（应用鉴权和幂等中间件）
api.Register(mux)  // 第163行

// ...中间件链路...

// 注册API路由
api.Register(mux)  // 第191行 - 重复注册！
```

**影响**：
1. 路由被注册两次，可能导致不可预期行为
2. 中间件应用混乱
3. 性能浪费

---

#### NEW-P0-04: handler变量创建但未使用

**严重程度**：P0（阻断）
**代码位置**：`main.go:175-201`

```go
handler := http.Handler(mux)
handler = middleware.RequestID(handler)
handler = middleware.Recovery(handler)
handler = middleware.Logging(handler)
// ... 更多中间件 ...

srv := &http.Server{
    Addr:              cfg.Server.Addr,
    Handler:           handler,  // ← 等等，handler是mux包装的
    // ...
}
```

**问题**：`api.Register(mux)`在第191行又注册了一次路由到mux，但handler已经是mux的包装。这意味着：
1. 中间件链路应用在mux上
2. 但路由在中间件之后又注册了一次
3. 实际行为取决于注册顺序

---

### 3.2 ⚠️ P1（高优先级）

#### NEW-P1-01: DBEarningStore未实现

**严重程度**：P1（高）
**代码位置**：`main.go:474-481`

```go
func (s *DBEarningStore) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
    // TODO: 实现真实查询
    return nil, 0, nil
}

func (s *DBEarningStore) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
    // TODO: 实现真实查询
    return nil, nil
}
```

**影响**：生产环境下收益查询和账单汇总返回空数据

---

#### NEW-P1-02: DBSettlementStore.GetWithdrawableBalance返回0

**严重程度**：P1（高）
**代码位置**：`main.go:464-467`

```go
func (s *DBSettlementStore) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
    // TODO: 实现真实查询 - 通过 account service 获取
    return 0.0, nil
}
```

**影响**：提现时始终显示余额为0，无法提现

---

#### NEW-P1-03: authMiddleware传入nil后端

**严重程度**：P1（高）
**代码位置**：`main.go:130`

```go
authMiddleware := middleware.NewAuthMiddleware(authConfig, tokenCache, nil, nil)
//                                                            ↑          ↑
//                                                      tokenBackend  auditEmitter
```

**影响**：
1. `tokenBackend=nil` 导致`checkTokenStatus`返回错误（`auth.go:447`）
2. `auditEmitter=nil` 导致所有审计事件不发送
3. Token吊销检查失效

---

#### NEW-P1-04: idempotencyRepo传入nil

**严重程度**：P1（高）
**代码位置**：`main.go:133`

```go
idempotencyMiddleware := middleware.NewIdempotencyMiddleware(nil, ...)
//                                                              ↑
//                                                          repo=nil
```

**影响**：幂等中间件的`Wrap`方法在调用`m.idempotencyRepo.GetByKey`时会panic

---

#### NEW-P1-05: supply_api.go使用内联幂等，与中间件重复

**严重程度**：P1（高）
**代码位置**：`supply_api.go:128-139, 626-637`

```go
// 幂等检查（内联实现）
if idempotencyKey != "" {
    if record, found := a.idempotencyStore.Get(idempotencyKey); found {
        if record.Status == "succeeded" {
            writeJSON(w, http.StatusOK, ...)
            return
        }
    }
    a.idempotencyStore.SetProcessing(idempotencyKey, 24*time.Hour)
}
```

**影响**：
1. 两套幂等逻辑并存，容易不一致
2. `idempotencyStore`是`InMemoryIdempotencyStore`，非DB-backed
3. 生产环境幂等记录在重启后丢失

---

### 3.3 🟡 P2（中优先级）

#### NEW-P2-01: 供应商ID硬编码为1

**代码位置**：`main.go:150`

```go
api := httpapi.NewSupplyAPI(
    // ...
    1, // 默认供应商ID
    time.Now,
)
```

---

#### NEW-P2-02: 审计日志分页total不准确

**代码位置**：`supply_api.go:334`

```go
"pagination": map[string]int{
    "page":      page,
    "page_size": pageSize,
    "total":     len(items),  // ← 这是分页后的数量，不是总数
},
```

---

#### NEW-P2-03: 声明PDF下载链接是硬编码的

**代码位置**：`supply_api.go:764`

```go
"download_url":  fmt.Sprintf("https://example.com/statements/%s.pdf", settlement.SettlementNo),
```

---

## 四、设计文档对齐检查

### 4.1 供应侧技术设计对齐

| 设计要求 | 上次状态 | 本次状态 | 对齐度 |
|----------|----------|----------|--------|
| 双键幂等（request_id + idempotency_key） | ✅ | ✅ | 95% |
| 幂等语义（200/201/202/409） | ⚠️ | ✅ | 95% |
| 乐观锁（version字段） | ✅ | ✅ | 100% |
| 审计事件（CRED-*/AUTH-*） | ✅ | ✅ | 95% |
| 凭证脱敏 | ✅ | ✅ | 95% |
| 数据库持久化 | 🔴 | ⚠️ 部分 | 60% |
| Outbox/Saga | 🔴 | 🔴 | 0% |
| 健康检查 | 🔴 | ✅ | 100% |
| 优雅关闭 | 🔴 | ✅ | 100% |

### 4.2 硬门槛实现状态

| 指标 | 上次 | 本次 | 说明 |
|------|------|------|------|
| M-013 凭证暴露检测 | ⚠️ mock | ⚠️ 内存 | 审计存储未持久化 |
| M-014 凭证入站覆盖率 | ⚠️ mock | ⚠️ 内存 | 审计存储未持久化 |
| M-015 直连检测 | ⚠️ mock | ⚠️ 内存 | 审计存储未持久化 |
| M-016 QueryKey拒绝 | ✅ | ✅ | 中间件已实现 |
| M-017 依赖兼容审计 | ✅ | ✅ | 100%通过 |

---

## 五、代码质量评估

### 5.1 架构评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 分层架构 | 85/100 | domain/service/repository/handler清晰 |
| 接口设计 | 80/100 | 接口定义良好，但有接口不统一问题 |
| 并发安全 | 80/100 | 竞态条件已修复 |
| 错误处理 | 75/100 | 部分错误处理不完整 |
| 可测试性 | 80/100 | 接口设计好 |

### 5.2 生产就绪性

| 维度 | 上次 | 本次 | 说明 |
|------|------|------|------|
| 数据库集成 | 0/100 | 60/100 | DB连接已实现，但审计/幂等/收益未对接 |
| 健康检查 | 0/100 | 100/100 | /health, /live, /ready均已实现 |
| 优雅关闭 | 0/100 | 100/100 | signal + Shutdown已实现 |
| 指标暴露 | 0/100 | 0/100 | 仍未实现 |
| 日志系统 | 50/100 | 60/100 | 基础log.Printf |
| 配置管理 | 60/100 | 70/100 | 配置文件加载已实现 |
| 幂等保护 | 0/100 | 50/100 | 内联实现有，中间件未接入 |
| 审计持久化 | 0/100 | 30/100 | 内存存储，有DB实现但未使用 |

---

## 六、必须整改项

### P0（阻断上线，修复前不可发布）

| 编号 | 问题 | 修复建议 | 预估工作量 |
|------|------|----------|-----------|
| NEW-P0-01 | 审计存储未对接DB | 统一接口或使用DatabaseAuditService | 2天 |
| NEW-P0-02 | 幂等中间件未使用 | 接入handler链或移除中间件 | 1天 |
| NEW-P0-03 | 路由注册两次 | 删除重复的api.Register调用 | 0.5天 |
| NEW-P0-04 | handler/mux链路混乱 | 理清中间件应用顺序 | 1天 |

### P1（本周完成）

| 编号 | 问题 | 修复建议 | 预估工作量 |
|------|------|----------|-----------|
| NEW-P1-01 | DBEarningStore未实现 | 实现真实查询 | 2天 |
| NEW-P1-02 | GetWithdrawableBalance返回0 | 实现真实查询 | 1天 |
| NEW-P1-03 | authMiddleware传入nil后端 | 实现tokenBackend和auditEmitter | 2天 |
| NEW-P1-04 | idempotencyRepo传入nil | 传入真实repo或移除中间件 | 1天 |
| NEW-P1-05 | 两套幂等逻辑并存 | 统一为一套 | 2天 |

### P2（本月完成）

| 编号 | 问题 | 修复建议 |
|------|------|----------|
| NEW-P2-01 | 供应商ID硬编码 | 从配置或认证上下文获取 |
| NEW-P2-02 | 分页total不准确 | 使用Query返回的total |
| NEW-P2-03 | 硬编码PDF链接 | 实现真实文件存储 |

---

## 七、总结

### 7.1 改进项

1. ✅ 4个P0问题已修复（硬编码userID、竞态条件、响应码、健康检查）
2. ✅ 优雅关闭已实现
3. ✅ 数据库连接已实现（有fallback机制）
4. ✅ 事件ID改用UUID
5. ✅ 幂等中间件实现statusCapturingResponseWriter

### 7.2 仍需修复

1. 🔴 审计存储未持久化（合规风险）
2. 🔴 幂等中间件未接入（资金风险）
3. 🔴 路由注册两次（代码质量问题）
4. 🔴 DB-backed存储多处TODO未实现

### 7.3 总体评估

| 项目 | 状态 |
|------|------|
| 代码质量 | 82/100（良好） |
| 设计对齐 | 78/100（大部分对齐） |
| 生产就绪 | 55/100（部分就绪） |
| **结论** | **CONDITIONAL GO**（需修复4个P0） |

### 7.4 修复后预估

修复所有P0+P1后：
- 代码质量：88/100
- 设计对齐：90/100
- 生产就绪：75/100
- 结论：可申请CONDITIONAL GO复审

---

**评审人**：多角色专家联合复审
**评审日期**：2026-04-03
**下次复审**：P0修复后
