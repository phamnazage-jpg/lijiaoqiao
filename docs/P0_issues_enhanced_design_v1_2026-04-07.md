# P0问题系统性修复设计方案

- 版本：v1.0
- 日期：2026-04-07
- 状态：实施基线
- 目标：系统性修复设计文档审查中发现的11个P0问题
- 关联文档：
  - `llm_gateway_prd_v1_2026-03-25.md`
  - `supply_technical_design_enhanced_v1_2026-03-25.md`
  - `database_domain_model_and_governance_v1_2026-03-27.md`
  - `token_runtime_minimal_spec_v1.md`
  - `token_auth_middleware_design_v1_2026-03-29.md`

---

## 一、问题修复总览

| # | 问题ID | 问题描述 | 影响 | 修复方案 | 优先级 |
|---|---|---|---|---|---|
| P0-01 | SEC-001 | Token格式未定义 | 鉴权链路基础缺失 | 明确JWT+RS256方案 | P1 |
| P0-02 | SEC-002 | 加密方案未定义 | 安全合规风险 | 补充KMS集成方案 | P1 |
| P0-03 | SEC-003 | 缓存吊销传播矛盾 | 安全漏洞 | 主动失效机制 | P0 |
| P0-04 | SEC-004 | Query Key检测不完整 | 安全风险 | 增强检测+白名单 | P2 |
| P0-05 | ARC-001 | 缺少限流策略 | 可用性风险 | 实现令牌桶+滑动窗口 | P1 |
| P0-006 | ARC-002 | Outbox无重试策略 | 可靠性 | 完整Outbox+DLQ | P1 |
| P0-007 | ARC-003 | 批量无补偿策略 | 数据一致性 | 补偿表+人工介入 | P2 |
| P0-008 | DB-001 | 大表无分区策略 | 性能退化 | 月分区策略 | P0 |
| P0-009 | DB-002 | 外键策略未定义 | 数据一致性 | 应用层外键+校验 | P1 |
| P0-010 | TST-001 | 需求追溯不完整 | 可追溯性 | 补充映射矩阵 | P2 |
| P0-011 | OPS-001 | 数据保留策略缺失 | 合规风险 | 多级保留策略 | P0 |

---

## 二、Token体系增强设计（P0-01）

### 2.1 Token格式规范

**选定方案：JWT + RS256（非对称签名）**

| 字段 | 规范 |
|------|------|
| 格式 | JWT (RFC 7519) |
| 签名算法 | RS256 (RSA-SHA256) / ES256 (可选) |
| Token类型 | Access Token + Refresh Token |
| Access Token有效期 | 15分钟 |
| Refresh Token有效期 | 7天 |

### 2.2 JWT Claims定义

```json
{
  "iss": "llm-gateway-platform",          // Issuer: 平台签发
  "sub": "user:12345",                     // Subject: 用户标识
  "aud": "llm-gateway-supply-api",        // Audience: 目标服务
  "exp": 1749366000,                       // Expiration: 15min
  "iat": 1749365300,                       // Issued At
  "nbf": 1749365300,                       // Not Before
  "jti": "tok_abc123def456",               // JWT ID: 唯一标识
  "tenant_id": 10001,                       // 租户ID
  "role": "owner",                          // 角色: owner/viewer/admin
  "scope": ["supply:accounts:read"],        // 授权范围
  "token_type": "access"                    // access/refresh
}
```

### 2.3 Token状态机

```
active -> expired (时间到期自动转换)
         -> revoked (主动吊销，不可恢复)
         -> invalid (验证失败)
```

### 2.4 与现有设计对比

| 维度 | 原设计 | 修复后 |
|------|--------|--------|
| Token格式 | 未定义 | JWT (RFC 7519) |
| 签名算法 | 未定义 | RS256 |
| 有效期 | 未定义 | 15min + 7d |
| Token类型 | 未定义 | access + refresh |

---

## 三、缓存吊销传播策略（P0-03）

### 3.1 问题分析

原设计存在逻辑矛盾：
- 缓存TTL = 30秒
- 要求吊销传播延迟 <= 5秒
- 矛盾点：30秒内吊销的token可能被使用最长30秒

### 3.2 修复方案：主动失效 + 短TTL

**核心策略：主动失效机制（Active Invalidation）**

```
架构：
[Token Revoke] -> [Message Queue/Pub-Sub] -> [Cache Invalidation] -> [Redis Del]
                                       |
                                       +-> [In-Memory Cache Del]
```

**实现要点：**

1. **吊销事件发布**
   - 吊销操作时，发布 `token.revoked` 事件到 Pub/Sub
   - 事件包含：`token_id`, `revoked_at`, `reason`

2. **缓存主动失效**
   - 订阅服务接收事件，立即删除对应缓存key
   - 延迟要求：<= 100ms（远小于5s目标）

3. **TTL兜底**
   - 将TTL从30s缩短至10s
   - 作为主动失效失败时的兜底

### 3.3 验收标准

| 指标 | 目标值 | 测量方法 |
|------|--------|---------|
| 吊销传播延迟 | <= 5s | 吊销操作到缓存删除的时间 |
| 主动失效成功率 | >= 99.9% | 主动失效次数/总吊销次数 |
| 缓存命中率 | >= 80% | 缓存命中/总查询 |

### 3.4 Redis Pub/Sub实现

```go
// 吊销时发布事件
func (s *TokenRevocationService) RevokeAndPublish(ctx context.Context, tokenID string) error {
    // 1. 更新数据库状态
    if err := s.db.RevokeToken(ctx, tokenID); err != nil {
        return err
    }

    // 2. 发布吊销事件
    event := TokenRevokedEvent{
        TokenID:    tokenID,
        RevokedAt:  time.Now(),
        Reason:     "user_requested",
    }

    // 3. 发布到Redis Pub/Sub
    return s.redis.Publish(ctx, "token:revoked", event)
}

// 订阅服务处理
func (s *RevocationSubscriber) Subscribe(ctx context.Context) {
    pubsub := s.redis.Subscribe(ctx, "token:revoked")
    defer pubsub.Close()

    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-pubsub.Channel():
            var event TokenRevokedEvent
            json.Unmarshal([]byte(msg.Payload), &event)

            // 立即失效缓存
            s.cache.Invalidate(ctx, event.TokenID)
        }
    }
}
```

---

## 四、Outbox模式实现（P0-006）

### 4.1 Outbox表结构

```sql
CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL,     -- 聚合类型: supply_account, package, settlement
    aggregate_id VARCHAR(128) NOT NULL,      -- 聚合ID
    event_type VARCHAR(128) NOT NULL,        -- 事件类型: created, updated, revoked
    event_id VARCHAR(64) NOT NULL UNIQUE,    -- 事件全局唯一ID (UUID)
    payload JSONB NOT NULL,                  -- 事件载荷
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_letter')),
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    dead_letter_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1         -- 乐观锁版本
);

-- 高频查询索引
CREATE INDEX idx_outbox_events_status_next_retry ON outbox_events (status, next_retry_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_outbox_events_aggregate ON outbox_events (aggregate_type, aggregate_id);
CREATE INDEX idx_outbox_events_created_at ON outbox_events (created_at);
```

### 4.2 死信队列（DLQ）表

```sql
CREATE TABLE outbox_dead_letter (
    id BIGSERIAL PRIMARY KEY,
    original_event_id VARCHAR(64) NOT NULL,
    original_aggregate_type VARCHAR(64) NOT NULL,
    original_aggregate_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    error_message TEXT,
    retry_count INT NOT NULL,
    first_failed_at TIMESTAMPTZ NOT NULL,
    dead_letter_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    handled BOOLEAN NOT NULL DEFAULT FALSE,
    handled_at TIMESTAMPTZ,
    handler_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_outbox_dead_letter_unhandled ON outbox_dead_letter (handled, dead_letter_at)
    WHERE handled = FALSE;
```

### 4.3 重试策略

| 参数 | 值 | 说明 |
|------|-----|------|
| 最大重试次数 | 5 | 超过后移入DLQ |
| 初始重试间隔 | 1s | 指数退避起始 |
| 最大重试间隔 | 60s | 退避上限 |
| 退避公式 | min(60s, 1s * 2^retry_count) | 指数退避 |

**重试时间线：**
- 第1次重试：1s
- 第2次重试：2s
- 第3次重试：4s
- 第4次重试：8s
- 第5次重试：16s
- 之后：移入DLQ

### 4.4 事件消费保障

```go
// Outbox处理器
type OutboxProcessor struct {
    db     *pgxpool.Pool
    broker MessageBroker
    stats  *MetricsService
}

// ProcessOutbox 扫描并处理待发送事件
func (p *OutboxProcessor) ProcessOutbox(ctx context.Context) error {
    // 1. 原子获取待处理事件（带悲观锁）
    events, err := p.db.FetchAndLockOutboxEvents(ctx, 100)
    if err != nil {
        return err
    }

    for _, event := range events {
        // 2. 更新状态为processing
        if err := p.db.UpdateOutboxStatus(ctx, event.ID, "processing"); err != nil {
            continue
        }

        // 3. 发送到消息队列
        if err := p.broker.Publish(ctx, event); err != nil {
            // 4. 处理失败，更新重试状态
            p.handleFailure(ctx, event, err)
            continue
        }

        // 5. 标记完成
        if err := p.db.UpdateOutboxStatus(ctx, event.ID, "completed", time.Now()); err != nil {
            p.stats.RecordOutboxFailure("update_completed_failed")
        }
    }

    return nil
}

// handleFailure 处理失败事件
func (p *OutboxProcessor) handleFailure(ctx context.Context, event *OutboxEvent, err error) {
    event.RetryCount++
    event.ErrorMessage = err.Error()

    if event.RetryCount >= event.MaxRetries {
        // 移入死信队列
        p.db.MoveToDeadLetter(ctx, event)
        p.stats.RecordOutboxDLQ(event.EventType)
    } else {
        // 计算下次重试时间（指数退避）
        backoff := time.Duration(math.Min(60, 1*math.Pow(2, float64(event.RetryCount)))) * time.Second
        event.NextRetryAt = time.Now().Add(backoff)
        p.db.UpdateOutboxRetry(ctx, event)
        p.stats.RecordOutboxRetry(event.EventType)
    }
}
```

---

## 五、批量操作补偿策略（P0-007）

### 5.1 补偿表结构

```sql
CREATE TABLE supply_batch_compensation (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(64) NOT NULL,           -- 批量任务ID
    operation_type VARCHAR(32) NOT NULL,      -- batch_price, batch_update
    item_index INT NOT NULL,                  -- 失败项在批量中的索引
    item_payload JSONB NOT NULL,              -- 失败项的原始请求
    failure_reason TEXT,                       -- 失败原因
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
        CHECK (status IN ('pending', 'retrying', 'resolved', 'manual_required', 'abandoned')),
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    resolved_at TIMESTAMPTZ,
    resolved_by BIGINT,                        -- 操作者ID
    resolution_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT,
    version BIGINT NOT NULL DEFAULT 1
);

CREATE INDEX idx_compensation_batch ON supply_batch_compensation (batch_id, status);
CREATE INDEX idx_compensation_status ON supply_batch_compensation (status, created_at);
```

### 5.2 补偿流程

```
批量调价请求
     |
     v
+-----------------+
| 分片事务处理    |
+-----------------+
     |
     +---> 成功项 ---> 返回成功响应
     |
     +---> 失败项 ---> 记录compensation表
     |                    |
     |                    v
     |            +-------------------+
     |            | 自动重试 (3次)     |
     |            +-------------------+
     |                    |
     |            +-------+-------+
     |            |成功    |失败    |
     |            v        v        v
     |         [resolved] [人工介入] [放弃]
     |
     v
返回批量结果（含成功/失败明细）
```

### 5.3 补偿接口

| 接口 | 方法 | 说明 |
|------|------|------|
| GET /api/v1/supply/batches/{batch_id}/compensation | 查询 | 获取批量补偿项列表 |
| POST /api/v1/supply/batches/{batch_id}/retry | 重试 | 重试失败项 |
| POST /api/v1/supply/compensation/{id}/resolve | 解决 | 人工确认解决 |
| POST /api/v1/supply/compensation/{id}/abandon | 放弃 | 放弃补偿 |

---

## 六、数据库分区策略（P0-008）

### 6.1 分区表定义

**audit_events 按月分区**

```sql
CREATE TABLE audit_events (
    id BIGINT NOT NULL,
    tenant_id BIGINT,
    project_id BIGINT,
    actor_user_id BIGINT,
    actor_type VARCHAR(32) NOT NULL,
    domain_code VARCHAR(32) NOT NULL,
    object_type VARCHAR(64) NOT NULL,
    object_id VARCHAR(128),
    action_code VARCHAR(64) NOT NULL,
    result_code VARCHAR(32) NOT NULL,
    severity VARCHAR(16) NOT NULL DEFAULT 'info',
    request_id VARCHAR(64),
    trace_id VARCHAR(64),
    idempotency_key VARCHAR(128),
    client_ip INET,
    user_agent VARCHAR(256),
    before_data JSONB,
    after_data JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 2026年分区
CREATE TABLE audit_events_2026_01 PARTITION OF audit_events
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_events_2026_02 PARTITION OF audit_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE audit_events_2026_03 PARTITION OF audit_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_events_2026_04 PARTITION OF audit_events
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_events_2026_05 PARTITION OF audit_events
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE audit_events_2026_06 PARTITION OF audit_events
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- 默认分区（捕获未预期的数据）
CREATE TABLE audit_events_default PARTITION OF audit_events DEFAULT;
```

**billing_ledger_entries 按月分区**

```sql
CREATE TABLE billing_ledger_entries (
    id BIGINT NOT NULL,
    billing_account_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    project_id BIGINT,
    user_id BIGINT,
    request_id VARCHAR(64) NOT NULL,
    trace_id VARCHAR(64),
    entry_type VARCHAR(32) NOT NULL,
    direction VARCHAR(2) NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    amount_unit VARCHAR(16) NOT NULL DEFAULT 'minor',
    balance_after_minor BIGINT,
    ref_type VARCHAR(32),
    ref_id BIGINT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    idempotency_key VARCHAR(128),
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- 2026年月度分区（示例）
CREATE TABLE billing_ledger_2026_04 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE billing_ledger_2026_05 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
```

### 6.2 分区维护

```sql
-- 自动创建新分区的存储过程（每日执行）
CREATE OR REPLACE FUNCTION create_monthly_partition(
    table_name TEXT,
    partition_date DATE
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := date_trunc('month', partition_date);
    end_date := start_date + INTERVAL '1 month';
    partition_name := table_name || '_' || to_char(start_date, 'YYYY_MM');

    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
        partition_name, table_name, start_date, end_date
    );
END;
$$ LANGUAGE plpgsql;

-- 分区清理策略（保留24个月）
CREATE OR REPLACE FUNCTION drop_old_partitions(
    table_name TEXT,
    retention_months INT DEFAULT 24
) RETURNS VOID AS $$
DECLARE
    partition_record RECORD;
    cutoff_date DATE;
BEGIN
    cutoff_date := date_trunc('month', CURRENT_DATE) - (retention_months || ' months')::INTERVAL;

    FOR partition_record IN
        SELECT inhrelid::regclass::text AS partition_name
        FROM pg_inherits
        WHERE inhparent = table_name::regclass
    LOOP
        IF partition_record.partition_name ~ '_[0-9]{4}_[0-9]{2}$' THEN
            IF to_date(substring(partition_record.partition_name from '_[0-9]{4}_[0-9]{2}$'), 'YYYY_MM') < cutoff_date THEN
                EXECUTE format('DROP TABLE IF EXISTS %I', partition_record.partition_name);
            END IF;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;
```

---

## 七、外键约束策略（P0-009）

### 7.1 策略决策

| 表类型 | 策略 | 理由 |
|--------|------|------|
| 核心实体表 | 物理外键 | 数据完整性关键 |
| 高频写入表 | 应用层外键 | 性能考量 |
| 审计/日志表 | 无外键 | 数据量大，历史数据可能孤立 |

### 7.2 外键策略定义

**保留物理外键的表（核心实体）：**
- `core_tenants` (主实体)
- `core_projects` (依赖 tenants)
- `iam_users` (依赖 tenants)
- `billing_accounts` (依赖 tenants, projects)

**使用应用层外键的表（高频写入）：**
- `supply_accounts` -> `iam_users`
- `supply_packages` -> `supply_accounts`
- `supply_orders` -> `supply_accounts`, `supply_packages`
- `supply_usage_records` -> `supply_orders`, `supply_accounts`

**无外键表（审计/日志）：**
- `audit_events`
- `outbox_events`
- `outbox_dead_letter`
- `supply_idempotency_record`

### 7.3 应用层外键校验服务

```go
// ForeignKeyValidator 应用层外键校验器
type ForeignKeyValidator struct {
    db *pgxpool.Pool
}

// ValidateSupplyAccountOwner 校验供应账号所属用户存在
func (v *ForeignKeyValidator) ValidateSupplyAccountOwner(ctx context.Context, userID int64) error {
    var exists bool
    err := v.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM iam_users WHERE id = $1)",
        userID,
    ).Scan(&exists)

    if err != nil {
        return fmt.Errorf("failed to validate user: %w", err)
    }

    if !exists {
        return ErrReferencedEntityNotFound
    }
    return nil
}

// ValidatePackageSupplyAccount 校验套餐所属供应账号存在
func (v *ForeignKeyValidator) ValidatePackageSupplyAccount(ctx context.Context, accountID int64) error {
    var exists bool
    err := v.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM supply_accounts WHERE id = $1)",
        accountID,
    ).Scan(&exists)

    if err != nil {
        return fmt.Errorf("failed to validate supply account: %w", err)
    }

    if !exists {
        return ErrReferencedEntityNotFound
    }
    return nil
}
```

### 7.4 一致性校验任务

```sql
-- 每日一致性校验任务
CREATE OR REPLACE FUNCTION check_orphan_records() RETURNS VOID AS $$
BEGIN
    -- 检查孤立的supply_accounts
    PERFORM COUNT(*) FROM supply_accounts sa
    WHERE NOT EXISTS (SELECT 1 FROM iam_users WHERE id = sa.user_id);

    -- 检查孤立的supply_packages
    PERFORM COUNT(*) FROM supply_packages sp
    WHERE NOT EXISTS (SELECT 1 FROM supply_accounts WHERE id = sp.supply_account_id);

    -- 检查孤立的supply_orders
    PERFORM COUNT(*) FROM supply_orders so
    WHERE NOT EXISTS (SELECT 1 FROM supply_accounts WHERE id = so.supply_account_id)
       OR NOT EXISTS (SELECT 1 FROM supply_packages WHERE id = so.supply_package_id);

    -- 如发现孤立记录，插入警告审计事件
    -- 实际处理由应用层完成
END;
$$ LANGUAGE plpgsql;
```

---

## 八、数据保留与归档策略（P0-011）

### 8.1 保留策略定义

| 数据类别 | 表名 | 保留期限 | 归档策略 | 清理策略 |
|---------|------|---------|---------|---------|
| 审计日志 | audit_events | 1年 | 压缩归档到OSS | 1年后DELETE |
| 凭证操作日志 | auth_credential_events | 1年 | 压缩归档 | 1年后DELETE |
| 调用日志 | supply_usage_records | 90天 | 压缩归档到OSS | 90天后DELETE |
| 结算数据 | billing_ledger_entries | 永久 | 不归档 | 不清理 |
| 订单数据 | supply_orders | 永久 | 不归档 | 不清理 |
| 套餐数据 | supply_packages | 永久(历史) | 不归档 | 不清理 |
| 账号数据 | supply_accounts | 永久(历史) | 不归档 | 不清理 |
| Outbox事件 | outbox_events | 30天 | 不归档 | 处理后DELETE |
| 补偿记录 | supply_batch_compensation | 1年 | 压缩归档 | 1年后DELETE |

### 8.2 分区保留策略

```sql
-- 审计日志分区保留（按分区删除）
CREATE OR REPLACE FUNCTION cleanup_audit_events_partitions(
    retention_months INT DEFAULT 12
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    partition_date DATE;
    cutoff_date DATE;
BEGIN
    cutoff_date := date_trunc('month', CURRENT_DATE) - (retention_months || ' months')::INTERVAL;

    FOR partition_name IN
        SELECT inhrelid::regclass::text
        FROM pg_inherits
        WHERE inhparent = 'audit_events'::regclass
    LOOP
        -- 提取分区日期 (格式: audit_events_YYYY_MM)
        IF partition_name ~ '^audit_events_[0-9]{4}_[0-9]{2}$' THEN
            partition_date := to_date(
                substring(partition_name from 'audit_events_(.*)'), 'YYYY_MM'
            );

            IF partition_date < cutoff_date THEN
                RAISE NOTICE 'Dropping partition: %', partition_name;
                EXECUTE format('DROP TABLE IF EXISTS %I', partition_name);
            END IF;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;
```

### 8.3 合规标签

根据GDPR、等保等合规要求：

```json
{
  "audit_events": {
    "retention_days": 365,
    "pii_fields": ["client_ip", "user_agent", "actor_user_id"],
    "anonymization_before_delete": true,
    "compliance_tags": ["GDPR", "SOC2", "等保二级"]
  },
  "supply_usage_records": {
    "retention_days": 90,
    "pii_fields": ["request_id"],
    "anonymization_before_delete": false,
    "compliance_tags": ["SOC2"]
  }
}
```

---

## 九、实施计划

### Phase 1（P0阻塞修复）

| 任务 | 依赖 | 工期 | 交付物 |
|------|------|------|--------|
| P0-03 缓存吊销修复 | P0-01 | 1天 | 主动失效机制 |
| P0-01 Token格式修复 | 无 | 1天 | Token设计文档 |
| P0-011 数据保留修复 | 无 | 1天 | 保留策略+分区 |
| P0-008 分区策略修复 | P0-011 | 2天 | 分区SQL脚本 |

### Phase 2（P1质量修复）

| 任务 | 依赖 | 工期 | 交付物 |
|------|------|------|--------|
| P0-006 Outbox实现 | Phase1完成 | 3天 | Outbox表+处理器 |
| P0-009 外键策略修复 | Phase1完成 | 1天 | 校验服务+任务 |
| P0-02 KMS集成 | P0-01 | 2天 | KMS方案 |

### Phase 3（持续改进）

| 任务 | 依赖 | 工期 | 交付物 |
|------|------|------|--------|
| P0-007 批量补偿 | P0-006 | 2天 | 补偿表+接口 |
| P0-04 QueryKey增强 | 无 | 1天 | 白名单机制 |
| P0-05 限流完善 | 无 | 1天 | 限流中间件 |

---

## 十、验收标准

### 10.1 P0问题验收

| 问题ID | 验收条件 | 测试用例 |
|--------|---------|---------|
| P0-01 | Token格式被各文档引用，代码实现一致 | TC-TOKEN-FORMAT-001~005 |
| P0-03 | 吊销传播延迟 <= 5s | TC-CACHE-REVOKE-001~003 |
| P0-008 | 分区表创建成功，查询性能P95 <= 100ms | TC-PARTITION-001~003 |
| P0-011 | 保留策略被文档化，分区清理正常 | TC-RETENTION-001~002 |

### 10.2 整体验收

- [ ] 所有P0问题已修复
- [ ] 设计文档与代码实现一致
- [ ] TDD测试全部通过
- [ ] 集成测试通过
- [ ] 性能测试基线建立

---

> **审查完成时间**: 2026-04-07
> **下次审查建议**: Phase 1修复后复审
