# PostgreSQL 数据库全面评审报告

> **评审日期**: 2026-04-18
> **项目**: 立交桥 / supply-api
> **评审范围**: SQL 质量、索引设计、连接池配置、事务边界、迁移规范、字段设计

---

## 1. SQL 质量分析

### 1.1 优点

**参数化查询**
- 所有 Repository 层方法均使用 `$1, $2, $3` 参数占位符
- 有效防止 SQL 注入攻击
- 示例: `account.go:58-70` 的 INSERT 语句

**错误处理**
- 正确区分 `pgx.ErrNoRows` 与其他错误
- 使用 `errors.Is()` 进行错误检查
- 自定义错误类型 `ErrNotFound`, `ErrConcurrencyConflict`

**幂等性设计**
- 幂等记录表使用 tenant/operator/path/key 组合唯一约束
- Payload 使用 SHA256 摘要防止参数Replay攻击
- 24小时过期机制（提现72小时）

### 1.2 问题

**[P2-01] supply_idempotency_records 唯一约束定义不一致**

`supply_idempotency_record_v1.sql` 定义:
```sql
UNIQUE (tenant_id, operator_id, api_path, idempotency_key)
```

但 `idempotency.go:217-226` 的 ON CONFLICT 处理逻辑假设过期记录可被覆盖:
```go
ON CONFLICT (tenant_id, operator_id, api_path, idempotency_key)
DO UPDATE SET
    ...
WHERE supply_idempotency_records.expires_at <= $8
```

**问题**: 唯一约束缺少过期时间维度，可能导致同一幂等键在过期前被重复使用

**[P2-02] 枚举值硬编码风险**

多处 CHECK 约束硬编码状态值:
```sql
-- supply_core_schema_v2.sql:21-22
CHECK (status IN ('pending', 'active', 'suspended', 'disabled'))

-- settlement.go 中使用 'processing' 状态
-- 但 supply_settlements 表 CHECK 约束未包含 'processing'
```

**建议**: 状态枚举应集中管理，建议创建 ENUM 类型

**[P2-03] account.go:123 未使用变量**

```go
var credentialFingerprint *string
// ...
_ = credentialFingerprint // 未使用但字段存在
```

---

## 2. 索引设计分析

### 2.1 优点

**[良好实践] 部分索引 (Partial Index)**

```sql
-- supply_core_schema_v2.sql:58-60
CREATE INDEX idx_supply_accounts_request_id
    ON supply_accounts (request_id)
    WHERE request_id <> '';
```

针对可选字段使用部分索引，减少索引体积。

**[良好实践] 复合索引覆盖常见查询模式**

```sql
-- supply_core_schema_v2.sql:55-56
CREATE INDEX idx_supply_accounts_user_status_created_at
    ON supply_accounts (user_id, status, created_at DESC);
```

支持 `WHERE user_id = ? AND status = ? ORDER BY created_at DESC` 查询。

### 2.2 问题

**[P1-001] supply_usage_records 缺少关键复合索引**

`usage.go:47-65` 查询模式:
```go
WHERE supplier_user_id = $1
AND started_at >= $2
AND started_at < $3
ORDER BY started_at DESC
```

**现状索引**:
- `idx_supply_usage_records_request_id`
- `idx_supply_usage_records_order_id`
- `idx_supply_usage_records_supply_account_id`
- `idx_supply_usage_records_platform_model`
- `idx_supply_usage_records_started_at`

**问题**: 缺少 `(supplier_user_id, started_at)` 复合索引，导致 `idx_supply_usage_records_started_at` 被用于 supplier_user_id 过滤时效率低

**[P1-002] supply_earnings 缺少 user_id + status 索引**

`earnings` 表高频查询:
```sql
WHERE user_id = $1 AND status IN ('available', 'pending')
```

仅存在单列索引 `idx_supply_earnings_user_id` 和 `idx_supply_earnings_status`

**[P2-04] supply_settlements period 字段类型不一致**

```sql
-- supply_core_schema_v2.sql:123-124
period_start TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
period_end TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
```

但 `data_dictionary_v1.md:208-209` 标注为 DATE 类型。schema 定义为 TIMESTAMPTZ 更合理，但需确认业务层日期边界处理逻辑。

---

## 3. 连接池配置分析

### 3.1 优点

**[良好实践] DSN 脱敏**

`config.go:116-126`:
```go
func (d *DatabaseConfig) SafeDSN() string {
    return fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable",
        d.User, d.Host, d.Port, d.Database)
}
```

避免在日志中泄露数据库密码 (P2-05)。

**[良好实践] 连接池参数完整**

`config.go:259-268`:
```go
v.SetDefault("database.max_open_conns", 25)
v.SetDefault("database.max_idle_conns", 5)
v.SetDefault("database.conn_max_lifetime", 1*time.Hour)
v.SetDefault("database.conn_max_idle_time", 10*time.Minute)
```

健康检查间隔配置正确。

### 3.2 问题

**[P1-003] ConnMaxIdleTime 配置缺失**

默认值 `10*time.Minute` 偏短。对于高频服务，可能导致连接频繁断开重建。

**建议**: 根据实际负载调整，或通过环境变量 `SUPPLY_DB_CONN_MAX_IDLE_TIME` 配置。

**[P2-05] DSN() 方法密码泄露**

```go
// config.go:112-113
return fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable",
    d.User, d.Password, d.Host, d.Port, d.Database)
```

**问题**: 格式化字符串中 `d.Password` 未被 `***` 替换，密码会被打印到日志

**正确写法**:
```go
return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
    d.User, "***", d.Host, d.Port, d.Database)
```

---

## 4. 事务边界分析

### 4.1 优点

**[良好实践] 悲观锁与乐观锁结合**

`settlement.go:153-210`:
```go
// 悲观锁 - 等待获取
func (r *SettlementRepository) GetForUpdate(...)

// 悲观锁 - 不等待
func (r *SettlementRepository) GetForUpdateNoWait(...)
```

提供两种锁策略适应不同并发场景。

**[良好实践] FOR UPDATE SKIP LOCKED 实现分布式锁**

`settlement.go:297-311`:
```go
lockQuery := `
    SELECT id FROM supply_settlements
    WHERE user_id = $1 AND status IN ('pending', 'processing')
    FOR UPDATE SKIP LOCKED
`
```

避免多个 worker 竞争同一资源的阻塞问题。

**[良好实践] Outbox 模式实现可靠消息**

`outbox.go:101-146`:
```go
func (r *OutboxRepository) FetchAndLock(ctx context.Context, limit int) ([]*OutboxEvent, error) {
    query := `
        WITH claimed AS (
            SELECT id FROM supply_outbox
            WHERE status IN ('pending', 'failed')
            ...
            FOR UPDATE SKIP LOCKED
        )
        UPDATE supply_outbox AS o SET status = 'processing', ...
    `
}
```

使用 CTE + UPDATE 实现原子的 "claim and update" 操作。

### 4.2 问题

**[P2-06] 幂等锁获取逻辑存在竞争窗口**

`idempotency.go:197-246`:
```go
func (r *IdempotencyRepository) AcquireLock(...) {
    // 1. 先尝试插入
    err := r.pool.QueryRow(ctx, query, ...)
    
    // 2. 失败后查询
    if err != nil {
        existing, getErr := r.GetByKey(...)
    }
}
```

**问题**: 在 INSERT 失败和 GetByKey 之间存在竞争窗口，另一个请求可能插入新记录

**建议**: 使用单个事务完成 insert-on-conflict 操作

**[P2-07] GetWithdrawableBalance 缺少事务保护**

`account.go:277-291`:
```go
func (r *AccountRepository) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
    query := `
        SELECT COALESCE(SUM(available_quota), 0)
        FROM supply_accounts
        WHERE user_id = $1 AND status = 'active'
    `
    // 直接使用 pool，无事务保护
}
```

在并发场景下可能出现余额计算不准确

---

## 5. 迁移规范分析

### 5.1 优点

**[良好实践] 版本化 Schema 文件**

```
sql/postgresql/
├── iam_schema_v1.sql
├── platform_core_schema_v1.sql
├── supply_schema_v1.sql
├── supply_core_schema_v2.sql          # 版本递增
└── partition_strategy_v1.sql
```

命名规范: `{module}_{aspect}_v{version}.sql`

**[良好实践] 数据字典文档**

`supply-api/sql/postgresql/data_dictionary_v1.md` 详细记录:
- 字段定义
- 索引列表
- 枚举类型
- 数据类型说明

**[良好实践] 索引维护文档**

`supply-api/sql/postgresql/index_maintenance_v1.md` 定义:
- VACUUM 策略
- REINDEX 触发条件
- 自动化脚本

### 5.2 问题

**[P1-004] supply_accounts 表缺少 created_by/updated_by 字段**

`supply_core_schema_v2.sql` 定义:
```sql
-- 仅有 audit_trace_id 追溯
audit_trace_id VARCHAR(128) NOT NULL DEFAULT '',
```

但审计追踪通常需要:
- 创建人/更新人 ID (created_by, updated_by)
- 当前代码中这两个字段存在但未在 schema 中定义

**[P1-006] supply_usage_records 缺少 tenant_id 字段**

多租户系统中，使用记录按 `buyer_user_id` 和 `supplier_user_id` 区分，但缺少直接的 `tenant_id` 字段用于全局查询优化

**[P2-08] 迁移脚本未使用事务包装**

部分迁移脚本（如 `audit_events_migration_v1_to_v2.sql`）未使用 `BEGIN...COMMIT` 包装，失败时可能导致部分执行

---

## 6. 字段设计分析

### 6.1 优点

**[良好实践] TIMESTAMPTZ 统一时区处理**

所有时间戳字段使用 `TIMESTAMPTZ`:
```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
```

避免时区歧义。

**[良好实践] NUMERIC 用于货币金额**

```sql
total_amount NUMERIC(20, 6) NOT NULL DEFAULT 0,
fee_amount NUMERIC(20, 6) NOT NULL DEFAULT 0,
net_amount NUMERIC(20, 6) NOT NULL DEFAULT 0,
```

浮点数不应用于货币计算。

**[良好实践] 乐观锁版本号**

```sql
version INTEGER NOT NULL DEFAULT 0,
```

所有核心业务表都包含 version 字段用于并发控制。

### 6.2 问题

**[P1-007] supply_accounts.risk_level 缺少 CHECK 约束**

```sql
risk_level VARCHAR(32) NOT NULL DEFAULT '',
```

无 CHECK 约束限制允许的值（low/normal/high）

**[P2-09] supply_packages.model VARCHAR(128) 可能不足**

AI 模型名称日益增长，128 字符可能不够（如 `o3-pro`, `claude-sonnet-4-20250514`）

**建议**: 扩展到 VARCHAR(255)

**[P2-10] 审计字段分散**

- `request_id` - 请求追踪
- `idempotency_key` - 幂等键
- `audit_trace_id` - 审计追踪

建议统一审计上下文，或考虑 JSONB 字段聚合

---

## 7. 优先级修复建议

### P1 (高优先级 - 影响生产稳定性)

| ID | 问题 | 建议 |
|----|------|------|
| P1-001 | supply_usage_records 缺少复合索引 | 添加 `(supplier_user_id, started_at)` 复合索引 |
| P1-002 | supply_earnings 缺少复合索引 | 添加 `(user_id, status)` 复合索引 |
| P1-003 | ConnMaxIdleTime 可能过短 | 调整默认值或开放环境变量配置 |
| P1-004 | 缺少 created_by/updated_by | 补充字段定义 |
| P1-006 | 缺少 tenant_id 字段 | 评估并补充多租户支持 |
| P1-007 | risk_level 缺少 CHECK 约束 | 添加 CHECK 约束 |

### P2 (中优先级 - 改进建议)

| ID | 问题 | 建议 |
|----|------|------|
| P2-01 | 幂等唯一约束需包含过期时间 | 重构约束或调整过期策略 |
| P2-04 | period 字段类型不一致 | 统一并更新文档 |
| P2-05 | DSN() 方法密码泄露 | 修复格式化字符串 |
| P2-06 | 幂等锁竞争窗口 | 重构为单事务操作 |
| P2-07 | GetWithdrawableBalance 无事务 | 添加事务保护 |
| P2-08 | 迁移脚本未事务包装 | 统一使用 BEGIN...COMMIT |
| P2-09 | model 字段长度可能不足 | 扩展到 VARCHAR(255) |
| P2-10 | 审计字段分散 | 考虑统一审计上下文 |

---

## 8. 总结

### 整体评价

| 维度 | 评分 (1-5) | 说明 |
|------|------------|------|
| SQL 质量 | 4 | 参数化查询完善，错误处理规范 |
| 索引设计 | 3.5 | 基础索引合理，高频查询缺少关键复合索引 |
| 连接池 | 4 | 配置完整，但有密码泄露Bug |
| 事务边界 | 4.5 | 悲观锁/乐观锁结合，Outbox模式优秀 |
| 迁移规范 | 4 | 版本化管理完善，文档齐全 |
| 字段设计 | 4 | 类型选择合理，多租户支持待完善 |

### 关键优势

1. **Outbox 模式实现完整**: 支持死信队列、重试机制、补偿记录
2. **幂等性设计**: SHA256 payload 摘要 + 过期机制
3. **连接池配置**: pgxpool 正确使用，DSN 脱敏（Bug 除外）
4. **事务策略**: FOR UPDATE SKIP LOCKED 避免竞争，NOWAIT 变体支持高并发

### 关键风险

1. **索引缺失**: usage/earnings 高频查询缺少复合索引，可能影响查询性能
2. **多租户支持**: 缺少 tenant_id 统一字段
3. **密码泄露**: DSN() 方法格式化错误

---

> **评审结论**: 数据库设计整体良好，核心模式（Outbox、幂等、连接池）设计成熟。建议优先修复 P1 级问题，特别是索引缺失和密码泄露问题。
