# Supply-API 代码质量评审报告

**项目路径**: `/home/long/project/立交桥/supply-api`  
**评审日期**: 2026-04-18  
**评审范围**: 模块边界、错误处理模式、命名规范、并发安全、测试质量

---

## 1. 模块边界分析

### 1.1 分层架构概览

项目采用标准分层架构：

```
internal/
├── domain/        # 领域层（实体、服务、不变量）
├── repository/    # 数据访问层（DB操作）
├── adapter/       # 适配器层（内存存储 vs DB-backed存储）
├── httpapi/       # HTTP处理层（路由、handler）
├── middleware/    # 中间件（幂等、审计）
├── audit/         # 审计模块
├── config/        # 配置管理
└── storage/       # 内存存储实现
```

### 1.2 模块边界问题

**问题 1.2.1: Repository层的transaction类型不一致**

`SettlementRepository`存在三个创建方法，接收不同的事务类型：

| 方法 | 参数类型 | 用途 |
|------|----------|------|
| `Create` | `*pgxpool.Pool` | 非事务创建 |
| `CreateTx` | `pgx.Tx` | 事务内创建 |
| `CreateInTx` | `pgxpool.Tx` | 事务内创建 |

**严重性**: 中

`pgx.Tx`和`pgxpool.Tx`是两种不同的接口，这会导致：
- 调用方需要区分使用场景
- `adapter/adapter.go`中的`CreateInTx`方法接收`pgxpool.Tx`但调用`repo.CreateTx(ctx, tx, ...)`可能存在类型不匹配

```go
// adapter/adapter.go:231
if err := s.repo.CreateTx(ctx, tx, settlement, "", "", ""); err != nil { ... }
```

此处`tx`类型为`pgxpool.Tx`，但调用的是`CreateTx`而非`CreateInTx`，逻辑上是正确的但命名混乱。

**问题 1.2.2: Domain层包含Mock存储**

`internal/domain/settlement_test.go`包含了mock存储实现，这些mock类型本应属于测试辅助代码：

```go
// internal/domain/settlement_test.go
type mockSettlementStore struct {...}
type mockEarningStore struct {...}
type mockAuditStoreForSettlement struct {...}
```

**建议**: 将mock实现移至`internal/testutil/mock/`目录，与生产代码分离。

---

## 2. 错误处理模式分析

### 2.1 错误码规范

项目定义了完善的错误码规范（`pkg/error/errors.go`）：

```
{DOMAIN}_{CODE}
- 4xxx: 业务逻辑错误
- 5xxx: 系统/服务器错误
- 9xxx: 内部/未知错误
```

领域层定义了模块化错误（如`ErrSettlementCannotCancel`、`ErrWithdrawExceedsBalance`），并使用错误码前缀：

```go
// internal/domain/invariants.go
var ErrSettlementCannotCancel = errors.New("SUP_SET_4092: cannot cancel processing or completed settlements")
```

### 2.2 错误处理问题

**问题 2.2.1: 字符串比对获取错误码**

HTTP handler中使用字符串比对判断错误类型：

```go
// internal/httpapi/supply_api.go:321
if strings.Contains(err.Error(), "SUP_ACC") {
    writeError(w, http.StatusConflict, CodeConflict, err.Error())
}
```

**严重性**: 低

这种做法虽然可用，但不够健壮。建议使用`pkg/error/errors.go`中的`As()`函数或错误码前缀常量进行类型断言。

**问题 2.2.2: 错误未充分Wrapping**

某些repository方法返回的错误未充分Wrapping：

```go
// internal/repository/settlement.go:48
return fmt.Errorf("failed to create settlement: %w", err)
```

缺少原始错误的cause context，如`s.SettlementNo`等关键业务字段。

---

## 3. 命名规范分析

### 3.1 整体命名评估

| 位置 | 命名 | 评估 |
|------|------|------|
| `domain.Settlement` | ✅ 清晰 | 实体名称符合DDD |
| `domain.SettlementStatus` | ✅ 清晰 | 枚举类型明确 |
| `SettlementRepository` | ✅ 清晰 | 仓储命名规范 |
| `InvariantChecker` | ✅ 清晰 | 职责明确 |
| `OutboxRepository` | ✅ 清晰 | 模式命名正确 |

### 3.2 命名问题

**问题 3.2.1: 字段名不一致 - SupplierID vs user_id**

数据库表`supply_settlements`的列名为`user_id`，但Go结构体使用`SupplierID`：

```go
// domain/Settlement struct
type Settlement struct {
    SupplierID int64  // 数据库列: user_id
}

// repository/settlement.go:40 - INSERT语句使用SupplierID
s.SettlementNo, s.SupplierID, s.TotalAmount, s.FeeAmount, s.NetAmount,
```

**严重性**: 中

代码能正常工作（pgx按位置映射），但语义不一致：
- 数据库字段：`user_id`
- Go字段：`SupplierID`

建议：确认业务语义后统一命名，或在SQL中使用命名参数避免歧义。

**问题 3.2.2: CreateWithdrawTx vs CreateInTx 命名混淆**

两个方法名过于相似：
- `CreateWithdrawTx`: 原子化创建提现（带锁）
- `CreateInTx`: 在事务中创建结算单

调用方容易混淆。`CreateInTx`的具体语义不明确。

---

## 4. 并发安全分析

### 4.1 已采用的并发控制机制

**4.1.1 乐观锁**

```go
// internal/repository/settlement.go:119-149
func (r *SettlementRepository) Update(...) error {
    cmdTag, err := r.pool.Exec(ctx, query,
        ...s.ID, s.SupplierID, expectedVersion,  // WHERE条件包含version
    )
    if cmdTag.RowsAffected() == 0 {
        return ErrConcurrencyConflict  // 乐观锁冲突
    }
}
```

**4.1.2 悲观锁 (FOR UPDATE SKIP LOCKED)**

```go
// internal/repository/settlement.go:296-340
func (r *SettlementRepository) CreateWithdrawTx(...) error {
    // 锁定现有pending/processing的提现记录
    lockQuery := `SELECT id FROM supply_settlements
                  WHERE user_id = $1 AND status IN ('pending', 'processing')
                  FOR UPDATE SKIP LOCKED`
    ...
}
```

**4.1.3 Outbox模式的分布式锁**

```go
// internal/repository/outbox.go:101-146
func (r *OutboxRepository) FetchAndLock(...) {
    // FOR UPDATE SKIP LOCKED实现分布式锁
}
```

### 4.2 并发安全问题

**问题 4.2.1: 提现操作的Check-And-Create竞态条件**

测试文件`settlement_race_test.go`明确标注了已知竞态条件问题：

```go
// internal/domain/settlement_race_test.go:13-14
// 问题: HasPendingOrProcessingWithdraw 和 CreateInTx 不在同一事务中
```

`Withdraw`服务方法的实现：
1. 调用`HasPendingOrProcessingWithdraw`检查
2. 调用`CreateWithdrawTx`创建

两个操作之间存在时间窗口，并发请求可能同时通过检查。

**严重性**: 高

虽然`CreateWithdrawTx`内部使用`FOR UPDATE SKIP LOCKED`，但`HasPendingOrProcessingWithdraw`是在事务外单独调用的。

**问题 4.2.2: MemoryAuditStore的锁粒度问题**

```go
// internal/audit/audit.go:58-65
func (s *MemoryAuditStore) Emit(ctx context.Context, event Event) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    // 整个操作持有锁
    event.EventID = generateEventID()
    event.CreatedAt = time.Now()
    s.events = append(s.events, event)
    return nil
}
```

在高并发写入场景下，`append`可能导致底层数组重新分配，但整体锁粒度可接受。

---

## 5. 测试质量分析

### 5.1 测试覆盖率

```
internal/domain        56.9%
internal/adapter       0.0%
internal/repository   3.1%
internal/middleware   68.9%
pkg/error            93.1%
internal/config       88.7%
```

### 5.2 测试问题

**问题 5.2.1: Mock存储未隔离**

部分mock实现过于简化，无法反映真实数据库行为：

```go
// internal/domain/settlement_test.go:32-37
func (m *mockSettlementStore) Create(ctx context.Context, s *Settlement) error {
    s.ID = m.nextID
    m.nextID++
    m.settlements[s.ID] = s
    return nil  // 无并发保护
}
```

这导致`settlement_race_test.go`需要单独的`mockConcurrentSettlementStore`来测试竞态条件。

**问题 5.2.2: 缺少Integration Test**

`internal/repository/`仅有3.1%覆盖率，缺乏数据库集成测试。

```bash
$ find internal/repository -name "*_integration_test.go" -o -name "*_test.go" | head
internal/repository/settlement_test.go
internal/repository/settlement_lock_test.go
```

建议补充真实数据库的集成测试。

**问题 5.2.3: 测试断言消息缺失**

部分测试使用`assert.NoError`但未提供上下文：

```go
// internal/domain/settlement_test.go:379
assert.NoError(t, err)
assert.NotNil(t, result)
```

当测试失败时，缺乏足够的诊断信息。

---

## 6. 其他发现

### 6.1 配置安全

**优点**: `config.go`实现了`SafeDSN()`方法，避免在日志中泄露密码：

```go
// internal/config/config.go:118
func (d *DatabaseConfig) SafeDSN() string {
    return fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable", ...)
}
```

### 6.2 生产环境校验

`config.go`在生产环境检查中禁止使用HMAC算法（HS256/HS384/HS512）：

```go
// internal/config/config.go:372-374
switch cfg.Token.Algorithm {
case "HS256", "HS384", "HS512":
    return fmt.Errorf("invalid prod config: token.algorithm %q is not allowed in production...")
}
```

这是良好的安全实践。

### 6.3 IdempotencyMiddleware设计

幂等中间件设计合理，支持：
- 处理中状态超时重试
- Payload hash验证（防止异参重放）
- 锁获取机制

---

## 7. 总结与建议

### 7.1 高优先级问题

| # | 问题 | 严重性 | 模块 |
|---|------|--------|------|
| 1 | 提现操作Check-And-Create竞态条件 | 高 | domain |
| 2 | CreateWithdrawTx vs CreateInTx命名混淆 | 中 | repository |
| 3 | 字段名不一致(SupplierID vs user_id) | 中 | repository |

### 7.2 中优先级问题

| # | 问题 | 严重性 | 模块 |
|---|------|--------|------|
| 4 | Mock存储与生产代码混杂 | 中 | domain |
| 5 | Repository层测试覆盖率低 | 中 | repository |
| 6 | 错误处理使用字符串比对 | 低 | httpapi |

### 7.3 低优先级问题

| # | 问题 | 严重性 | 模块 |
|---|------|--------|------|
| 7 | 错误Wrapping缺少业务上下文 | 低 | repository |
| 8 | 测试断言缺少上下文消息 | 低 | domain |

### 7.4 改进建议

1. **重构提现逻辑**: 将`HasPendingOrProcessingWithdraw`检查纳入`CreateWithdrawTx`事务内
2. **统一事务接口**: 考虑定义统一的`Tx`接口适配`pgx.Tx`和`pgxpool.Tx`
3. **分离测试代码**: 将mock实现移至`internal/testutil/mock/`
4. **补充集成测试**: 为repository层添加真实的数据库集成测试
5. **优化错误Wrapping**: 在返回错误时包含关键业务字段便于调试
