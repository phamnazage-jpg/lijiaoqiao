# 审计日志增强设计方案（P1）

- 版本：v2.0
- 日期：2026-04-03
- 状态：已定稿
- 目标：为 M-013~M-016 指标提供完整的审计基础设施支撑

---

## 1. 现状分析

### 1.1 现有实现

#### supply-api/internal/audit/audit.go

```go
// 审计事件
type Event struct {
    EventID     string         `json:"event_id,omitempty"`
    TenantID    int64          `json:"tenant_id"`
    ObjectType  string         `json:"object_type"`
    ObjectID    int64          `json:"object_id"`
    Action      string         `json:"action"`
    BeforeState map[string]any `json:"before_state,omitempty"`
    AfterState  map[string]any `json:"after_state,omitempty"`
    RequestID   string         `json:"request_id,omitempty"`
    ResultCode  string         `json:"result_code"`
    ClientIP    string         `json:"client_ip,omitempty"`
    CreatedAt   time.Time      `json:"created_at"`
}
```

- 仅内存存储（MemoryAuditStore），无持久化
- 无事件分类体系
- 无 M-013~M-016 指标映射能力
- 无脱敏扫描能力

#### gateway/internal/middleware/audit.go

- DatabaseAuditEmitter 实现（PostgreSQL）
- 关注 Token 认证事件
- 字段：event_id, event_name, request_id, token_id, subject_id, route, result_code, client_ip, created_at
- 与 supply-api 审计体系割裂

### 1.2 差距分析

| 维度 | 现有实现 | M-013~M-016 要求 | 差距 |
|------|---------|-----------------|------|
| 凭证暴露事件 | 无专门记录 | M-013: 凭证泄露事件=0，需完整溯源 | 严重不足 |
| 凭证入站类型 | 无区分 | M-014: 平台凭证覆盖率=100% | 无追踪 |
| 直连绕过事件 | 无 | M-015: 直连事件=0 | 无感知 |
| query key 拒绝 | 无 | M-016: 拒绝率=100% | 无记录 |
| 事件分类 | 无 | 安全事件分类体系 | 缺失 |
| 存储 | 内存 | 持久化+可查询 | 需改造 |
| 溯源能力 | 基本 | 全链路追踪 | 不足 |

---

## 2. 设计目标

### 2.1 核心目标

1. **M-013 支撑**：供应方上游凭证泄露事件追踪
   - 凭证相关操作完整记录
   - 脱敏扫描集成
   - 实时告警能力

2. **M-014 支撑**：平台凭证入站覆盖率
   - 入站凭证类型标记
   - 覆盖率自动计算
   - 违规事件捕获

3. **M-015 支撑**：需求方直连绕过追踪
   - 出网行为监控
   - 跨域调用检测
   - 异常模式识别

4. **M-016 支撑**：外部 query key 拒绝率
   - query key 请求全记录
   - 拒绝原因分类
   - 拒绝率实时计算

### 2.2 非功能目标

- 审计写入延迟 < 50ms（半同步模式）
- 查询响应时间 < 500ms（1000条记录）
- 支持 **5,000-8,000 TPS** 写入
- 数据保留 365 天

#### TPS 达成路径（简化架构）

**架构原则**：保持单体服务，聚焦核心，去除过度工程化

| 阶段 | 策略 | 预期TPS | 组件 | 说明 |
|------|------|---------|------|------|
| **Phase 1 (立即)** | 单条INSERT + 连接池 | ~3,000 | PostgreSQL + pgx | 当前基线，验证功能 |
| **Phase 2 (1-2周)** | 批量写入(50条/批) | ~5,000-6,000 | BatchBuffer + pgx | 核心性能优化 |
| **Phase 3 (按需)** | Go Channel异步 + 批量 | ~6,000-8,000 | goroutine + channel | 无Kafka，零额外运维 |

**为什么不需要 Kafka**：
1. **运维成本高**：需要集群管理、监控、调优
2. **Go channel足够**：内存队列 + 后台worker实现异步写入
3. **按需升级**：只有TPS持续超过8K时才考虑消息队列

**关键技术**：
1. **批量写入API**：`POST /api/v1/audit/events/batch` 支持最多50条/批次
2. **异步写入队列**：Go Channel + 后台goroutine（无Kafka）
3. **PgBouncer连接池**：减少连接建立开销
4. **暂不考虑分区**：数据量超过1000万再加分区

#### 简化架构图

```
┌──────────────────────────────────────────────────────────────────┐
│                      单体服务架构                                  │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│   ┌─────────────┐     ┌──────────────┐     ┌──────────────────┐ │
│   │   Client    │────▶│   Gateway    │────▶│  Audit Handler    │ │
│   └─────────────┘     └──────────────┘     └────────┬─────────┘ │
│                                                       │            │
│                                                       ▼            │
│   ┌───────────────────────────────────────────────────────────┐  │
│   │                     AuditService                         │  │
│   │  ┌────────────────┐    ┌────────────────┐              │  │
│   │  │ Sync Path      │    │ Async Channel │              │  │
│   │  │ (<50 events/s) │    │ (>50 events/s)│              │  │
│   │  └───────┬────────┘    └───────┬────────┘              │  │
│   │          └──────────┬───────────┘                          │  │
│   │                     ▼                                      │  │
│   │          ┌─────────────────────┐                          │  │
│   │          │   BatchBuffer      │                          │  │
│   │          │   (50条/批, 5ms)  │                          │  │
│   │          └──────────┬──────────┘                          │  │
│   └─────────────────────┼────────────────────────────────────┘  │
│                         ▼                                        │
│   ┌────────────────────────────────────────────────────────┐  │
│   │           PostgreSQL + pgx连接池                        │  │
│   │           批量INSERT (50条/批)                          │  │
│   │           专用索引 (M-013~M-016)                       │  │
│   └────────────────────────────────────────────────────────┘  │
│                                                                   │
│   **无Kafka | 无Redis | 无分区表 | 无额外中间件**              │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## 3. 审计事件分类体系

### 3.1 事件大类

| 大类编码 | 大类名称 | 说明 |
|---------|---------|------|
| CRED | 凭证事件 | 凭证相关操作 |
| AUTH | 认证授权事件 | 身份验证与权限检查 |
| DATA | 数据访问事件 | 数据读写操作 |
| CONFIG | 配置变更事件 | 系统配置修改 |
| SECURITY | 安全相关事件 | 安全策略触发 |

### 3.2 凭证事件子类（CRED）

| 子类编码 | 子类名称 | M-013 映射 | 记录场景 |
|---------|---------|-----------|---------|
| CRED-EXPOSE | 凭证暴露 | 直接相关 | 响应/导出/日志中出现可复用凭证片段 |
| CRED-INGRESS | 凭证入站 | 直接相关 | 入站请求凭证类型校验 |
| CRED-ROTATE | 凭证轮换 | 间接相关 | 凭证主动轮换操作 |
| CRED-REVOKE | 凭证吊销 | 间接相关 | 凭证吊销/禁用操作 |
| CRED-VALIDATE | 凭证验证 | 间接相关 | 凭证验证结果 |
| CRED-DIRECT | 直连绕过 | M-015 直接相关 | 需求方绕过平台直连供应方 |

### 3.3 认证授权事件子类（AUTH）

| 子类编码 | 子类名称 | M-016 映射 | 记录场景 |
|---------|---------|-----------|---------|
| AUTH-TOKEN-OK | Token认证成功 | 间接相关 | 平台Token认证通过 |
| AUTH-TOKEN-FAIL | Token认证失败 | 间接相关 | Token无效/过期/格式错误 |
| AUTH-QUERY-KEY | query key 请求 | M-016 直接相关 | 外部 query key 请求 |
| AUTH-QUERY-REJECT | query key 拒绝 | M-016 直接相关 | query key 被拒绝 |
| AUTH-SCOPE-DENY | Scope权限不足 | 间接相关 | 权限不足拒绝 |

### 3.4 数据访问事件子类（DATA）

| 子类编码 | 子类名称 | 说明 |
|---------|---------|------|
| DATA-READ | 数据读取 | GET 请求 |
| DATA-WRITE | 数据写入 | POST/PUT/PATCH 请求 |
| DATA-DELETE | 数据删除 | DELETE 请求 |
| DATA-EXPORT | 数据导出 | 导出操作 |

### 3.5 配置变更事件子类（CONFIG）

| 子类编码 | 子类名称 | 说明 |
|---------|---------|------|
| CONFIG-CREATE | 配置创建 | 新增配置 |
| CONFIG-UPDATE | 配置更新 | 修改配置 |
| CONFIG-DELETE | 配置删除 | 删除配置 |

### 3.6 安全相关事件子类（SECURITY）

| 子类编码 | 子类名称 | M-013 映射 | 说明 |
|---------|---------|-----------|------|
| INVARIANT-VIOLATION | 不变量违反 | 直接相关 | 业务不变量检查失败（依据XR-001要求：所有不变量失败必须写入invariant_violation事件，并携带rule_code） |
| SECURITY-BREACH | 安全突破 | 直接相关 | 安全机制被突破 |
| SECURITY-ALERT | 安全告警 | 间接相关 | 安全相关告警事件 |

#### 3.6.1 invariant_violation 事件详细定义

根据XR-001要求，所有不变量失败必须写入审计事件 `invariant_violation`，并携带 `rule_code`。

| 规则ID | 规则名称 | 触发场景 | 结果码 |
|--------|----------|----------|--------|
| INV-PKG-001 | 供应方资质过期 | 资质验证 | `SEC_INV_PKG_001` |
| INV-PKG-002 | 供应方余额为负 | 余额检查 | `SEC_INV_PKG_002` |
| INV-PKG-003 | 售价不得低于保护价 | 发布/调价 | `SEC_INV_PKG_003` |
| INV-SET-001 | `processing/completed` 不可撤销 | 撤销申请 | `SEC_INV_SET_001` |
| INV-SET-002 | 提现金额不得超过可提现余额 | 发起提现 | `SEC_INV_SET_002` |
| INV-SET-003 | 结算单金额与余额流水必须平衡 | 结算入账 | `SEC_INV_SET_003` |

---

## 4. 审计字段标准化

### 4.1 统一审计事件结构

```go
// AuditEvent 统一审计事件
type AuditEvent struct {
    // 基础标识
    EventID       string    `json:"event_id"`                // 事件唯一ID (UUID)
    EventName     string    `json:"event_name"`              // 事件名称 (e.g., "CRED-EXPOSE")
    EventCategory string    `json:"event_category"`          // 事件大类 (e.g., "CRED")
    EventSubCategory string `json:"event_sub_category"`      // 事件子类

    // 时间戳
    Timestamp     time.Time `json:"timestamp"`               // 事件发生时间
    TimestampMs  int64     `json:"timestamp_ms"`            // 毫秒时间戳

    // 请求上下文
    RequestID     string    `json:"request_id"`             // 请求追踪ID
    TraceID       string    `json:"trace_id"`               // 分布式追踪ID
    SpanID        string    `json:"span_id"`                // Span ID

    // 幂等性
    IdempotencyKey string   `json:"idempotency_key,omitempty"` // 幂等键

    // 操作者信息
    OperatorID    int64     `json:"operator_id"`            // 操作者ID
    OperatorType  string    `json:"operator_type"`          // 操作者类型 (user/system/admin)
    OperatorRole  string    `json:"operator_role"`          // 操作者角色

    // 租户信息
    TenantID      int64     `json:"tenant_id"`              // 租户ID
    TenantType    string    `json:"tenant_type"`            // 租户类型 (supplier/consumer/platform)

    // 对象信息
    ObjectType     string   `json:"object_type"`             // 对象类型 (account/package/settlement)
    ObjectID       int64    `json:"object_id"`              // 对象ID

    // 操作信息
    Action         string   `json:"action"`                 // 操作类型 (create/update/delete)
    ActionDetail   string   `json:"action_detail"`          // 操作详情

    // 凭证信息 (M-013/M-014/M-015/M-016 关键)
    CredentialType string   `json:"credential_type"`         // 凭证类型 (platform_token/query_key/upstream_api_key/none)
    CredentialID   string   `json:"credential_id,omitempty"` // 凭证标识 (脱敏)
    CredentialFingerprint string `json:"credential_fingerprint,omitempty"` // 凭证指纹

    // 来源信息
    SourceType     string   `json:"source_type"`            // 来源类型 (api/ui/cron/internal)
    SourceIP       string   `json:"source_ip"`              // 来源IP
    SourceRegion   string   `json:"source_region"`          // 来源区域
    UserAgent      string   `json:"user_agent,omitempty"`  // User Agent

    // 目标信息 (用于直连检测 M-015)
    TargetType     string   `json:"target_type,omitempty"` // 目标类型
    TargetEndpoint string   `json:"target_endpoint,omitempty"` // 目标端点
    TargetDirect   bool     `json:"target_direct"`          // 是否直连

    // 结果信息
    ResultCode     string   `json:"result_code"`            // 结果码
    ResultMessage  string   `json:"result_message,omitempty"` // 结果消息
    Success        bool     `json:"success"`               // 是否成功

    // 状态变更 (用于溯源)
    BeforeState    map[string]any `json:"before_state,omitempty"` // 操作前状态
    AfterState     map[string]any `json:"after_state,omitempty"` // 操作后状态

    // 安全标记 (M-013 关键)
    SecurityFlags  SecurityFlags `json:"security_flags"`      // 安全标记
    RiskScore      int          `json:"risk_score"`          // 风险评分 0-100

    // 合规信息
    ComplianceTags []string `json:"compliance_tags,omitempty"` // 合规标签 (e.g., ["GDPR", "SOC2"])
    InvariantRule string   `json:"invariant_rule,omitempty"` // 触发的不变量规则

    // 扩展字段
    Extensions     map[string]any `json:"extensions,omitempty"` // 扩展数据

    // 元数据
    Version        int       `json:"version"`               // 事件版本
    CreatedAt      time.Time `json:"created_at"`            // 创建时间
}

// SecurityFlags 安全标记
type SecurityFlags struct {
    HasCredential       bool     `json:"has_credential"`        // 是否包含凭证
    CredentialExposed  bool     `json:"credential_exposed"`   // 凭证是否暴露
    Desensitized        bool     `json:"desensitized"`          // 是否已脱敏
    Scanned             bool     `json:"scanned"`              // 是否已扫描
    ScanPassed          bool     `json:"scan_passed"`          // 扫描是否通过
    ViolationTypes      []string `json:"violation_types"`      // 违规类型列表
}
```

### 4.2 M-013~M-016 指标专用字段

```go
// M-013: 凭证暴露事件专用
type CredentialExposureDetail struct {
    ExposureType     string   `json:"exposure_type"`      // exposed_in_response/exposed_in_log/exposed_in_export
    ExposureLocation string   `json:"exposure_location"`  // response_body/response_header/log_file/export_file
    ExposurePattern  string   `json:"exposure_pattern"`  // 匹配到的正则模式
    Exposed片段       string   `json:"exposed_fragment"`  // 暴露的片段（已脱敏）
    ScanRuleID       string   `json:"scan_rule_id"`      // 触发扫描规则ID
}

// M-014: 凭证入站类型专用
type CredentialIngressDetail struct {
    RequestCredentialType string `json:"request_credential_type"` // 请求中的凭证类型
    ExpectedCredentialType string `json:"expected_credential_type"` // 期望的凭证类型
    CoverageCompliant     bool   `json:"coverage_compliant"`      // 是否合规
    PlatformTokenPresent  bool   `json:"platform_token_present"`  // 平台Token是否存在
    UpstreamKeyPresent    bool   `json:"upstream_key_present"`    // 上游Key是否存在
}

// M-015: 直连绕过专用
type DirectCallDetail struct {
    ConsumerID       int64   `json:"consumer_id"`
    SupplierID        int64   `json:"supplier_id"`
    DirectEndpoint    string  `json:"direct_endpoint"`
    ViaPlatform      bool    `json:"via_platform"`
    BypassType       string  `json:"bypass_type"`         // ip_bypass/proxy_bypass/config_bypass
    DetectionMethod   string  `json:"detection_method"`   // how detected
}

// M-016: query key 拒绝专用
type QueryKeyRejectDetail struct {
    QueryKeyID       string  `json:"query_key_id"`
    RequestedEndpoint string  `json:"requested_endpoint"`
    RejectReason     string  `json:"reject_reason"`      // not_allowed/expired/malformed
    RejectCode       string  `json:"reject_code"`
}
```

---

## 5. 存储设计

### 5.1 PostgreSQL 表结构

```sql
-- 统一审计事件表
CREATE TABLE IF NOT EXISTS audit_events (
    -- 基础标识
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_name VARCHAR(64) NOT NULL,
    event_category VARCHAR(32) NOT NULL,
    event_sub_category VARCHAR(32),

    -- 时间戳
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    timestamp_ms BIGINT NOT NULL,

    -- 请求上下文
    request_id VARCHAR(128),
    trace_id VARCHAR(128),
    span_id VARCHAR(64),
    idempotency_key VARCHAR(128),

    -- 操作者信息
    operator_id BIGINT NOT NULL,
    operator_type VARCHAR(32) NOT NULL,
    operator_role VARCHAR(64),

    -- 租户信息
    tenant_id BIGINT NOT NULL,
    tenant_type VARCHAR(32) NOT NULL,

    -- 对象信息
    object_type VARCHAR(64) NOT NULL,
    object_id BIGINT NOT NULL,

    -- 操作信息
    action VARCHAR(64) NOT NULL,
    action_detail TEXT,

    -- 凭证信息
    credential_type VARCHAR(32) NOT NULL,
    credential_id VARCHAR(128),
    credential_fingerprint VARCHAR(64),

    -- 来源信息
    source_type VARCHAR(32),
    source_ip INET,
    source_region VARCHAR(32),
    user_agent TEXT,

    -- 目标信息
    target_type VARCHAR(32),
    target_endpoint TEXT,
    target_direct BOOLEAN DEFAULT FALSE,

    -- 结果信息
    result_code VARCHAR(64) NOT NULL,
    result_message TEXT,
    success BOOLEAN NOT NULL DEFAULT TRUE,

    -- 状态变更 (JSONB)
    before_state JSONB,
    after_state JSONB,

    -- 安全标记 (JSONB)
    security_flags JSONB,

    -- 风险评分
    risk_score INT DEFAULT 0,

    -- 合规信息
    compliance_tags TEXT[],
    invariant_rule VARCHAR(128),

    -- 扩展字段 (JSONB)
    extensions JSONB,

    -- 元数据
    version INT DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引策略
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_events(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_trace_id ON audit_events(trace_id);
CREATE INDEX IF NOT EXISTS idx_audit_tenant_id ON audit_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_event_category ON audit_events(event_category);
CREATE INDEX IF NOT EXISTS idx_audit_event_name ON audit_events(event_name);
CREATE INDEX IF NOT EXISTS idx_audit_credential_type ON audit_events(credential_type);
CREATE INDEX IF NOT EXISTS idx_audit_object ON audit_events(object_type, object_id);
CREATE INDEX IF NOT EXISTS idx_audit_success ON audit_events(success) WHERE NOT success;
CREATE INDEX IF NOT EXISTS idx_audit_risk_score ON audit_events(risk_score) WHERE risk_score > 50;

-- 安全标记索引（使用冗余布尔字段，替代JSONB表达式索引以提升性能）
-- 注意：应用层需要在写入时同步更新 has_credential_exposed 字段
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS has_credential_exposed BOOLEAN DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_audit_cred_exposed ON audit_events(has_credential_exposed) WHERE has_credential_exposed = TRUE;

-- GIN索引（备选方案，如果需要全文搜索JSONB字段）
-- CREATE INDEX IF NOT EXISTS idx_audit_security_flags_gin ON audit_events USING GIN (security_flags);

-- M-013 专用索引
CREATE INDEX IF NOT EXISTS idx_audit_cred_exposure ON audit_events(event_name, timestamp DESC) WHERE event_name LIKE 'CRED-EXPOSE%';

-- M-014 专用索引
CREATE INDEX IF NOT EXISTS idx_audit_cred_ingress ON audit_events(credential_type, timestamp DESC) WHERE event_category = 'CRED' AND event_sub_category = 'INGRESS';

-- M-015 专用索引
CREATE INDEX IF NOT EXISTS idx_audit_direct_call ON audit_events(target_direct, timestamp DESC) WHERE target_direct = TRUE;

-- M-016 专用索引
CREATE INDEX IF NOT EXISTS idx_audit_query_key_reject ON audit_events(event_name, timestamp DESC) WHERE event_name LIKE 'AUTH-QUERY%';

-- 分区表（按月分区）
CREATE TABLE IF NOT EXISTS audit_events_partitioned () INHERITS (audit_events);

-- 创建分区函数
CREATE OR REPLACE FUNCTION create_audit_partition()
RETURNS void AS $$
DECLARE
    partition_date DATE;
    partition_name TEXT;
BEGIN
    partition_date := CURRENT_DATE;
    partition_name := 'audit_events_' || TO_CHAR(partition_date, 'YYYYMM');

    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_events_partitioned FOR VALUES FROM (%L) TO (%L)',
        partition_name,
        partition_date,
        partition_date + INTERVAL '1 month'
    );
END;
$$ LANGUAGE plpgsql;

-- 凭证暴露事件详情表 (M-013 专用)
CREATE TABLE IF NOT EXISTS credential_exposure_events (
    event_id UUID PRIMARY KEY REFERENCES audit_events(event_id),
    exposure_type VARCHAR(64) NOT NULL,
    exposure_location VARCHAR(64) NOT NULL,
    exposure_pattern VARCHAR(256),
    exposed_fragment TEXT,
    scan_rule_id VARCHAR(64),
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMPTZ,
    resolved_by BIGINT,
    resolution_notes TEXT
);

-- 凭证入站事件表 (M-014 专用)
CREATE TABLE IF NOT EXISTS credential_ingress_events (
    event_id UUID PRIMARY KEY REFERENCES audit_events(event_id),
    request_credential_type VARCHAR(32) NOT NULL,
    expected_credential_type VARCHAR(32) NOT NULL,
    coverage_compliant BOOLEAN NOT NULL,
    platform_token_present BOOLEAN NOT NULL,
    upstream_key_present BOOLEAN NOT NULL,
    reviewed BOOLEAN DEFAULT FALSE,
    reviewed_at TIMESTAMPTZ,
    reviewed_by BIGINT
);

-- 直连绕过事件表 (M-015 专用)
CREATE TABLE IF NOT EXISTS direct_call_events (
    event_id UUID PRIMARY KEY REFERENCES audit_events(event_id),
    consumer_id BIGINT NOT NULL,
    supplier_id BIGINT NOT NULL,
    direct_endpoint TEXT NOT NULL,
    via_platform BOOLEAN NOT NULL,
    bypass_type VARCHAR(32),
    detection_method VARCHAR(64),
    blocked BOOLEAN DEFAULT FALSE,
    blocked_at TIMESTAMPTZ,
    block_reason TEXT
);

-- query key 拒绝事件表 (M-016 专用)
CREATE TABLE IF NOT EXISTS query_key_reject_events (
    event_id UUID PRIMARY KEY REFERENCES audit_events(event_id),
    query_key_id VARCHAR(128) NOT NULL,
    requested_endpoint TEXT NOT NULL,
    reject_reason VARCHAR(64) NOT NULL,
    reject_code VARCHAR(64) NOT NULL,
    first_occurrence BOOLEAN DEFAULT TRUE,
    occurrence_count INT DEFAULT 1
);

-- 审计事件归档表 (历史数据)
CREATE TABLE IF NOT EXISTS audit_events_archive (
    LIKE audit_events INCLUDING ALL
);

-- 触发器：自动更新 updated_at
CREATE OR REPLACE FUNCTION update_created_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.created_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_audit_events_created_at
    BEFORE INSERT ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION update_created_at();
```

### 5.2 Redis 缓存（热点数据）

```json
{
  "key_pattern": "audit:metric:{metric_type}:{date}",
  "ttl": 86400,
  "fields": {
    "m013_cred_exposure_count": 0,
    "m014_platform_ingress_count": 0,
    "m014_total_ingress_count": 0,
    "m015_direct_call_count": 0,
    "m016_query_key_reject_count": 0,
    "m016_query_key_total_count": 0
  }
}
```

---

## 6. API 设计

### 6.1 事件写入 API

```
POST /api/v1/audit/events
Content-Type: application/json
X-Request-Id: {request_id}
X-Idempotency-Key: {idempotency_key}

{
  "event": AuditEvent
}
```

#### 幂等性响应语义

| 状态码 | 场景 | 响应体 |
|--------|------|--------|
| 201 | 首次成功 | `{"event_id": "...", "status": "created"}` |
| 202 | 处理中 | `{"status": "processing", "retry_after_ms": 1000}` |
| 409 | 重放异参 | `{"error": {"code": "IDEMPOTENCY_PAYLOAD_MISMATCH", "message": "Idempotency key reused with different payload"}}` |
| 200 | 重放同参 | `{"event_id": "...", "status": "duplicate", "original_created_at": "..."}` |

**幂等性协议说明**：
- **首次成功**：请求的幂等键从未使用过，处理成功后返回201
- **重放同参**：请求的幂等键已使用且payload相同，返回200（不重复创建）
- **重放异参**：请求的幂等键已使用但payload不同，返回409冲突
- **处理中**：请求的幂等键正在处理中（异步场景），返回202

### 6.2 事件查询 API

```
GET /api/v1/audit/events
```

| 参数 | 类型 | 说明 |
|-----|------|------|
| tenant_id | int64 | 租户ID（必填） |
| start_date | string | 开始日期 ISO8601 |
| end_date | string | 结束日期 ISO8601 |
| event_category | string | 事件大类 |
| event_name | string | 事件名称 |
| object_type | string | 对象类型 |
| object_id | int64 | 对象ID |
| credential_type | string | 凭证类型 |
| success | bool | 是否成功 |
| risk_score_min | int | 最小风险评分 |
| limit | int | 返回数量（默认100，最大1000） |
| offset | int | 偏移量 |

```
GET /api/v1/audit/events/{event_id}
```

### 6.3 M-013~M-016 指标 API

```
GET /api/v1/audit/metrics/m013
```

```json
{
  "metric_id": "M-013",
  "metric_name": "supplier_credential_exposure_events",
  "period": {
    "start": "2026-04-01T00:00:00Z",
    "end": "2026-04-02T00:00:00Z"
  },
  "value": 0,
  "unit": "count",
  "status": "PASS",
  "details": {
    "total_exposure_events": 0,
    "unresolved_events": 0,
    "recent_events": []
  }
}
```

```
GET /api/v1/audit/metrics/m014
```

```json
{
  "metric_id": "M-014",
  "metric_name": "platform_credential_ingress_coverage_pct",
  "period": {
    "start": "2026-04-01T00:00:00Z",
    "end": "2026-04-02T00:00:00Z"
  },
  "value": 100.0,
  "unit": "percentage",
  "status": "PASS",
  "details": {
    "platform_token_requests": 10000,
    "total_requests": 10000,
    "non_compliant_requests": 0
  }
}
```

```
GET /api/v1/audit/metrics/m015
```

```json
{
  "metric_id": "M-015",
  "metric_name": "direct_supplier_call_by_consumer_events",
  "period": {
    "start": "2026-04-01T00:00:00Z",
    "end": "2026-04-02T00:00:00Z"
  },
  "value": 0,
  "unit": "count",
  "status": "PASS",
  "details": {
    "total_direct_call_events": 0,
    "blocked_events": 0
  }
}
```

```
GET /api/v1/audit/metrics/m016
```

```json
{
  "metric_id": "M-016",
  "metric_name": "query_key_external_reject_rate_pct",
  "period": {
    "start": "2026-04-01T00:00:00Z",
    "end": "2026-04-02T00:00:00Z"
  },
  "value": 100.0,
  "unit": "percentage",
  "status": "PASS",
  "details": {
    "rejected_requests": 0,
    "total_external_query_key_requests": 0,
    "reject_breakdown": {}
  }
}
```

### 6.4 告警配置 API

```
POST /api/v1/audit/alerts
GET /api/v1/audit/alerts
PUT /api/v1/audit/alerts/{alert_id}
DELETE /api/v1/audit/alerts/{alert_id}
```

---

## 7. 集成方案

### 7.1 supply-api 集成

#### Domain 层改造

```go
// audit/event.go

package audit

// 事件类别常量
const (
    CategoryCRED = "CRED"
    CategoryAUTH = "AUTH"
    CategoryDATA = "DATA"
    CategoryCONFIG = "CONFIG"
    CategorySECURITY = "SECURITY"
)

// 凭证事件子类别
const (
    SubCategoryCredExpose   = "EXPOSE"
    SubCategoryCredIngress  = "INGRESS"
    SubCategoryCredRotate   = "ROTATE"
    SubCategoryCredRevoke   = "REVOKE"
    SubCategoryCredValidate = "VALIDATE"
    SubCategoryCredDirect   = "DIRECT"
)

// 凭证类型
const (
    CredentialTypePlatformToken  = "platform_token"
    CredentialTypeQueryKey       = "query_key"
    CredentialTypeUpstreamAPIKey  = "upstream_api_key"
    CredentialTypeNone            = "none"
)

// 操作者类型
const (
    OperatorTypeUser    = "user"
    OperatorTypeSystem = "system"
    OperatorTypeAdmin  = "admin"
)

// 租户类型
const (
    TenantTypeSupplier  = "supplier"
    TenantTypeConsumer  = "consumer"
    TenantTypePlatform  = "platform"
)
```

#### 审计中间件集成

```go
// httpapi/middleware/audit.go

package httpapi

import (
    "context"
    "supply-api/internal/audit"
)

type AuditMiddleware struct {
    auditStore audit.AuditStore
}

func (m *AuditMiddleware) Handle(ctx context.Context, req *Request, next Handler) (*Response, error) {
    // 创建审计上下文
    auditCtx := audit.WithContext(ctx, &audit.Context{
        RequestID:    req.Header.Get("X-Request-Id"),
        TraceID:      req.Header.Get("X-Trace-Id"),
        SpanID:       req.Header.Get("X-Span-Id"),
        OperatorID:   req.OperatorID,
        OperatorType: req.OperatorType,
        TenantID:     req.TenantID,
        TenantType:   req.TenantType,
        SourceIP:     req.ClientIP,
        UserAgent:    req.Header.Get("User-Agent"),
    })

    // 处理请求
    resp, err := next.Handle(auditCtx, req)

    // 记录审计事件
    m.emitFromResponse(auditCtx, req, resp, err)

    return resp, err
}

func (m *AuditMiddleware) emitFromResponse(ctx context.Context, req *Request, resp *Response, err error) {
    event := &audit.Event{
        EventName:     m.determineEventName(req),
        EventCategory: audit.CategoryAUTH,
        Timestamp:     time.Now(),
        RequestID:     req.Header.Get("X-Request-Id"),
        OperatorID:    req.OperatorID,
        TenantID:      req.TenantID,
        ObjectType:    m.determineObjectType(req),
        ObjectID:      req.ObjectID,
        Action:        req.Method,
        CredentialType: m.determineCredentialType(req),
        SourceIP:      req.ClientIP,
        ResultCode:    m.determineResultCode(resp, err),
        Success:       err == nil,
        RiskScore:     m.calculateRiskScore(req, resp, err),
    }

    m.auditStore.Emit(ctx, event)
}
```

#### 凭证暴露检测集成

```go
// security/credential_scanner.go

package security

type CredentialScanner struct {
    rules []ScanRule
}

type ScanRule struct {
    ID          string
    Pattern     *regexp.Regexp
    Severity    string
    Description string
}

func (s *CredentialScanner) Scan(content string) (*ScanResult, error) {
    result := &ScanResult{
        Violations: []Violation{},
    }

    for _, rule := range s.rules {
        if matches := rule.Pattern.FindAllString(content, -1); len(matches) > 0 {
            result.Violations = append(result.Violations, Violation{
                RuleID:     rule.ID,
                Matched:    matches,
                Severity:   rule.Severity,
                Described:  s.desensitize(matches),
            })
        }
    }

    return result, nil
}

func (s *CredentialScanner) desensitize(matches []string) []string {
    desensitized := make([]string, len(matches))
    for i, match := range matches {
        if len(match) > 8 {
            desensitized[i] = match[:4] + "****" + match[len(match)-4:]
        } else {
            desensitized[i] = "****"
        }
    }
    return desensitized
}
```

### 7.2 gateway 集成

#### Token 认证审计增强

```go
// middleware/auth.go

func (m *AuthMiddleware) authn(ctx context.Context, req *Request) error {
    // ... 认证逻辑 ...

    // 审计事件
    event := &middleware.AuditEvent{
        EventID:    generateEventID(),
        EventName:  determineEventName(credType, success),
        RequestID:  req.Header.Get("X-Request-Id"),
        TokenID:    tokenID,
        SubjectID:  subjectID,
        Route:     req.URL.Path,
        ResultCode: resultCode,
        ClientIP:  req.ClientIP,
        CreatedAt: time.Now(),
        // 扩展字段
        Extensions: map[string]any{
            "credential_type": credType,
            "tenant_id":      tenantID,
            "m014_compliant":  credType == CredentialTypePlatformToken,
            "m016_query_key":  credType == CredentialTypeQueryKey,
        },
    }

    if err := m.Auditor.Emit(ctx, *event); err != nil {
        log.Errorf("failed to emit audit event: %v", err)
    }

    return nil
}
```

### 7.3 脱敏扫描集成

```go
// security/desensitization.go

package security

// 脱敏规则
var DesensitizationRules = []DesensitizationRule{
    {
        Name:        "api_key",
        Pattern:     `sk-[a-zA-Z0-9]{20,}`,
        Replacement: "sk-****",
        Level:       LevelSensitive,
    },
    {
        Name:        "openai_key",
        Pattern:     `(sk-[a-zA-Z0-9]{20,})`,
        Replacement: "${1:0:4}****${1:-4}",
        Level:       LevelSensitive,
    },
    {
        Name:        "upstream_credential",
        Pattern:     `(sk-|api-|key-)[a-zA-Z0-9]{16,}`,
        Replacement: "${1}****",
        Level:       LevelSensitive,
    },
}

func Desensitize(content string) (string, []Violation) {
    result := content
    violations := []Violation{}

    for _, rule := range DesensitizationRules {
        if matches := rule.Pattern.FindAllString(result, -1); len(matches) > 0 {
            result = rule.Pattern.ReplaceAllString(result, rule.Replacement)
            violations = append(violations, Violation{
                Rule:   rule.Name,
                Count:  len(matches),
                Level:  rule.Level,
            })
        }
    }

    return result, violations
}
```

---

## 8. M-013~M-016 指标实现

### 8.1 M-013: 凭证泄露事件数 = 0

#### 检测点

1. **响应检测**：所有 API 响应在返回前扫描凭证片段
2. **日志检测**：日志输出前扫描凭证片段
3. **导出检测**：导出文件生成前扫描凭证片段
4. **实时告警**：检测到立即告警

#### SQL 计算

```sql
SELECT COUNT(*) as exposure_count
FROM audit_events
WHERE event_name LIKE 'CRED-EXPOSE%'
    AND timestamp >= $start_date
    AND timestamp < $end_date;
```

### 8.2 M-014: 平台凭证入站覆盖率 = 100%

#### 检测点

1. **入站校验**：每个入站请求记录凭证类型
2. **覆盖率计算**：平台Token请求数 / 总请求数

#### M-014 与 M-016 边界说明

- **M-014 分母定义**：经平台凭证校验的入站请求（`credential_type = 'platform_token'`），**不含**被拒绝的无效请求
- **M-016 分母定义**：检测到的所有query key请求（`event_name LIKE 'AUTH-QUERY%'`），**含**被拒绝的请求
- **两者互不影响**：query key请求在通过平台认证前不会进入M-014的计数范围，因此query key拒绝事件不会影响M-014的覆盖率计算

**示例**：
- 如果有100个请求，其中80个使用platform_token，20个使用query key（被拒绝）
- M-014 = 80/80 = 100%（分母只计算platform_token请求）
- M-016 = 20/20 = 100%（分母计算所有query key请求）

#### SQL 计算

```sql
WITH credential_stats AS (
    SELECT
        COUNT(*) FILTER (WHERE credential_type = 'platform_token') as platform_count,
        COUNT(*) as total_count
    FROM audit_events
    WHERE event_category = 'CRED'
        AND event_sub_category = 'INGRESS'
        AND timestamp >= $start_date
        AND timestamp < $end_date
)
SELECT
    CASE WHEN total_count = 0 THEN 100.0
         ELSE (platform_count::DECIMAL / total_count::DECIMAL) * 100
    END as coverage_pct
FROM credential_stats;
```

### 8.3 M-015: 直连事件数 = 0

#### 检测点

1. **出网监控**：监控所有出站连接
2. **直连识别**：检测绕过平台的直接连接
3. **模式识别**：异常访问模式识别

#### M-015 直连检测机制详细设计

根据合规能力包（C015-R01~C015-R03），直连检测有以下机制：

##### 8.3.1 检测方法

| 检测方法 | 说明 | 实现位置 |
|---------|------|----------|
| **IP/域名白名单比对** | 请求目标为已知供应商IP/域名时标记为直连 | Gateway层 |
| **上游API模式匹配** | 请求路径匹配 `*/v1/chat/completions` 等上游端点 | Gateway层 |
| **DNS解析监控** | 检测到Consumer直接解析Supplier域名 | Network层 |
| **连接来源检测** | 出站连接直接来自Consumer IP而非平台代理 | Network层 |

##### 8.3.2 检测流程

```
直连检测流程 (M015-FLOW-01)

1. 请求发起
         │
         ▼
2. 检查请求目标
   - 若目标IP在供应商白名单 → 标记 target_direct = TRUE
   - 若目标域名解析指向供应商IP段 → 标记 target_direct = TRUE
         │
         ▼
3. 检查请求路径
   - 若路径匹配上游API模式（如 */v1/chat/completions）
   - 且来源不是平台代理 → 标记 target_direct = TRUE
         │
         ▼
4. 记录审计事件
   - 记录 target_direct = TRUE
   - 记录 bypass_type（ip_bypass/proxy_bypass/config_bypass）
   - 记录 detection_method（检测方法）
         │
         ▼
5. 触发阻断/告警
   - P0事件立即阻断
   - 发送告警到安全通道
```

##### 8.3.3 target_direct 字段填充规则

| 场景 | target_direct | bypass_type | detection_method |
|------|---------------|-------------|------------------|
| Consumer直接调用Supplier API | TRUE | ip_bypass | upstream_api_pattern_match |
| Consumer DNS直解析Supplier | TRUE | dns_bypass | dns_resolution_check |
| 通过平台代理调用 | FALSE | - | - |
| 内部服务调用 | FALSE | - | - |

#### SQL 计算

```sql
SELECT COUNT(*) as direct_call_count
FROM audit_events
WHERE target_direct = TRUE
    AND timestamp >= $start_date
    AND timestamp < $end_date;
```

### 8.4 M-016: query key 拒绝率 = 100%

#### 检测点

1. **请求记录**：所有 query key 请求
2. **拒绝记录**：所有拒绝事件
3. **覆盖率计算**：拒绝数 / 请求数

#### SQL 计算

```sql
WITH query_key_stats AS (
    SELECT
        COUNT(*) FILTER (WHERE event_name = 'AUTH-QUERY-KEY') as total_requests,
        COUNT(*) FILTER (WHERE event_name = 'AUTH-QUERY-REJECT') as rejected_requests
    FROM audit_events
    WHERE event_name LIKE 'AUTH-QUERY%'
        AND timestamp >= $start_date
        AND timestamp < $end_date
)
SELECT
    CASE WHEN total_requests = 0 THEN 100.0
         ELSE (rejected_requests::DECIMAL / total_requests::DECIMAL) * 100
    END as reject_rate_pct
FROM query_key_stats;
```

---

## 9. CI/CD 集成

### 9.1 Gate 脚本

```bash
#!/bin/bash
# scripts/ci/audit_metrics_gate.sh
# 生产级Gate脚本：带重试、超时、错误处理

set -e

# 数据库连接配置（从环境变量读取，支持CI/CD secrets）
PGHOST=${PGHOST:-localhost}
PGPORT=${PGPORT:-5432}
PGUSER=${PGUSER:-audit_service}
PGDATABASE=${PGDATABASE:-audit_db}
export PGPASSWORD=${PGPASSWORD:-}

# 时间范围（默认昨天）
METRICS_START_DATE=${METRICS_START_DATE:-$(date -d '1 day ago' +%Y-%m-%d)}
METRICS_END_DATE=${METRICS_END_DATE:-$(date +%Y-%m-%d)}

# SQL超时设置（30秒）
SQL_TIMEOUT="SET statement_timeout = '30s';"

# 数据库连接重试函数
retry_psql() {
    local max_attempts=3
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if result=$(PGPASSWORD="$PGPASSWORD" psql -t -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -c "$@" 2>/dev/null); then
            echo "$result"
            return 0
        fi
        echo "警告: psql尝试 $attempt/$max_attempts 失败，等待5秒后重试..."
        attempt=$((attempt + 1))
        sleep 5
    done
    echo "错误: psql在 $max_attempts 次尝试后失败"
    return 1
}

# 比较函数（使用awk替代bc，兼容更多环境）
compare_lt() {
    local a=$1
    local b=$2
   awk "BEGIN { exit !($a < $b) }"
}

echo "=== M-013 凭证泄露事件数检查 ==="
M013_COUNT=$(retry_psql "$SQL_TIMEOUT SELECT COUNT(*) FROM audit_events WHERE event_name LIKE 'CRED-EXPOSE%' AND timestamp >= '$METRICS_START_DATE' AND timestamp < '$METRICS_END_DATE';")
M013_COUNT=$(echo "$M013_COUNT" | tr -d '[:space:]')
echo "M-013 凭证暴露事件数: $M013_COUNT"
if [ -n "$M013_COUNT" ] && [ "$M013_COUNT" -gt 0 ]; then
    echo "FAIL: M-013 超标 (要求 = 0)"
    exit 1
fi
echo "PASS: M-013"

echo "=== M-014 平台凭证覆盖率检查 ==="
M014_RATE=$(retry_psql "$SQL_TIMEOUT WITH stats AS (SELECT COUNT(*) FILTER (WHERE credential_type = 'platform_token') as p, COUNT(*) as t FROM audit_events WHERE event_category = 'CRED' AND event_sub_category = 'INGRESS' AND timestamp >= '$METRICS_START_DATE' AND timestamp < '$METRICS_END_DATE') SELECT CASE WHEN t = 0 THEN 100.0 ELSE (p::DECIMAL / t::DECIMAL) * 100 END FROM stats;")
M014_RATE=$(echo "$M014_RATE" | tr -d '[:space:]' | awk '{printf "%.2f", $0}')
echo "M-014 平台凭证覆盖率: $M014_RATE%"
if [ -n "$M014_RATE" ] && compare_lt "$M014_RATE" "100"; then
    echo "FAIL: M-014 不达标 (要求 = 100%)"
    exit 1
fi
echo "PASS: M-014"

echo "=== M-015 直连绕过事件数检查 ==="
M015_COUNT=$(retry_psql "$SQL_TIMEOUT SELECT COUNT(*) FROM audit_events WHERE target_direct = TRUE AND timestamp >= '$METRICS_START_DATE' AND timestamp < '$METRICS_END_DATE';")
M015_COUNT=$(echo "$M015_COUNT" | tr -d '[:space:]')
echo "M-015 直连事件数: $M015_COUNT"
if [ -n "$M015_COUNT" ] && [ "$M015_COUNT" -gt 0 ]; then
    echo "FAIL: M-015 超标 (要求 = 0)"
    exit 1
fi
echo "PASS: M-015"

echo "=== M-016 query key 拒绝率检查 ==="
M016_RATE=$(retry_psql "$SQL_TIMEOUT WITH stats AS (SELECT COUNT(*) FILTER (WHERE event_name = 'AUTH-QUERY-KEY') as t, COUNT(*) FILTER (WHERE event_name = 'AUTH-QUERY-REJECT') as r FROM audit_events WHERE event_name LIKE 'AUTH-QUERY%' AND timestamp >= '$METRICS_START_DATE' AND timestamp < '$METRICS_END_DATE') SELECT CASE WHEN t = 0 THEN 100.0 ELSE (r::DECIMAL / t::DECIMAL) * 100 END FROM stats;")
M016_RATE=$(echo "$M016_RATE" | tr -d '[:space:]' | awk '{printf "%.2f", $0}')
echo "M-016 query key 拒绝率: $M016_RATE%"
if [ -n "$M016_RATE" ] && compare_lt "$M016_RATE" "100"; then
    echo "FAIL: M-016 不达标 (要求 = 100%)"
    exit 1
fi
echo "PASS: M-016"

echo "=== 所有 M-013~M-016 检查通过 ==="
```

**生产级改进说明**：
1. **使用awk替代bc**：避免bc命令依赖问题，awk是POSIX标准
2. **数据库连接重试**：最多3次重试，每次间隔5秒
3. **SQL超时控制**：statement_timeout = '30s' 防止查询无限阻塞
4. **NULL/空值处理**：检查变量是否为空
5. **环境变量配置**：支持CI/CD secrets注入

### 9.2 测试用例

```go
// internal/audit/audit_test.go

package audit

import (
    "testing"
)

func TestM013_CredentialExposureDetection(t *testing.T) {
    scanner := NewCredentialScanner()

    testCases := []struct {
        name        string
        content     string
        expectFound bool
    }{
        {
            name:        "OpenAI API Key",
            content:     "sk-1234567890abcdefghijklmnopqrstuvwxyz",
            expectFound: true,
        },
        {
            name:        "Platform Token",
            content:     "platform_token_xxx",
            expectFound: false,
        },
        {
            name:        "Normal Text",
            content:     "This is normal text without credentials",
            expectFound: false,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := scanner.Scan(tc.content)
            if err != nil {
                t.Fatalf("scan failed: %v", err)
            }
            if tc.expectFound && len(result.Violations) == 0 {
                t.Error("expected to find credential but none found")
            }
            if !tc.expectFound && len(result.Violations) > 0 {
                t.Errorf("expected no credential but found %d", len(result.Violations))
            }
        })
    }
}

func TestM014_PlatformCredentialIngressCoverage(t *testing.T) {
    store := NewTestStore()

    // 模拟入站请求
    testCases := []struct {
        credType    string
        shouldCount bool
    }{
        {CredentialTypePlatformToken, true},
        {CredentialTypeQueryKey, false},
        {CredentialTypeUpstreamAPIKey, false},
    }

    for _, tc := range testCases {
        event := &Event{
            EventCategory:    CategoryCRED,
            EventSubCategory: SubCategoryCredIngress,
            CredentialType:   tc.credType,
            Success:          true,
            Timestamp:        time.Now(),
        }
        store.Emit(context.Background(), *event)
    }

    // 计算覆盖率
    total := 0
    platformCount := 0
    events, _ := store.Query(context.Background(), EventFilter{})
    for _, e := range events {
        total++
        if e.CredentialType == CredentialTypePlatformToken {
            platformCount++
        }
    }

    coverage := float64(platformCount) / float64(total) * 100
    if coverage != 100.0 {
        t.Errorf("expected 100%% coverage, got %.2f%%", coverage)
    }
}
```

---

## 10. 实施计划

### 10.1 Phase 1: 基础设施（1-2周）

| 任务 | 依赖 | 负责人 | 验收标准 |
|------|------|--------|---------|
| 数据库表结构创建 | - | 后端 | 表创建成功，索引正常 |
| 统一 Event 结构体 | - | 后端 | 结构体定义完成 |
| AuditStore 接口定义 | - | 后端 | 接口评审通过 |
| PostgreSQL 实现 | 表结构 | 后端 | 单元测试通过 |

### 10.2 Phase 2: 核心功能（2-3周）

| 任务 | 依赖 | 负责人 | 验收标准 |
|------|------|--------|---------|
| supply-api 审计中间件 | Phase 1 | 后端 | 集成测试通过 |
| 凭证暴露扫描器 | Phase 1 | 安全 | 扫描准确率 > 99% |
| 脱敏规则库 | Phase 1 | 安全 | 规则覆盖主要场景 |
| API 实现 | Phase 1 | 后端 | API 测试通过 |
| M-014 覆盖率计算 | API | 后端 | 指标计算正确 |

### 10.3 Phase 3: M-013~M-016 指标（1-2周）

| 任务 | 依赖 | 负责人 | 验收标准 |
|------|------|--------|---------|
| M-013 事件记录 | Phase 2 | 后端 | 事件正确分类 |
| M-015 直连检测 | Phase 2 | 安全 | 检测逻辑正确 |
| M-016 拒绝记录 | Phase 2 | 后端 | 记录完整 |
| 指标 API | Phase 2 | 后端 | API 正确返回 |
| CI Gate 脚本 | Phase 3 | DevOps | Gate 检查通过 |

### 10.4 Phase 4: 集成与优化（1周）

| 任务 | 依赖 | 负责人 | 验收标准 |
|------|------|--------|---------|
| 端到端测试 | Phase 3 | QA | 测试通过 |
| 性能优化 | Phase 3 | 后端 | 满足性能目标 |
| 文档完善 | Phase 3 | 后端 | 文档完整 |
| 告警配置 | Phase 3 | 运维 | 告警正常工作 |

---

## 11. 风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| 审计写入影响性能 | 高 | 中 | 异步写入，批量处理 |
| 数据量膨胀 | 中 | 中 | 分区表，定期归档 |
| 误报导致 M-014 误判 | 高 | 低 | 双校验机制 |
| 直连检测覆盖不全 | 高 | 中 | 多维度检测 |
| 历史数据迁移 | 中 | 低 | 分阶段迁移 |

---

## 12. 附录

### 12.1 事件名称规范

格式：`{Category}-{SubCategory}[-{Detail}]`

示例：
- `CRED-EXPOSE-RESPONSE`
- `CRED-INGRESS-PLATFORM`
- `AUTH-QUERY-KEY`
- `AUTH-TOKEN-OK`

#### 12.1.1 事件名称与TOK-002对齐映射

为确保与TOK-002 Token中间件设计一致，以下事件名称建立等价映射关系：

| 设计文档事件名 | TOK-002事件名 | 说明 |
|---------------|---------------|------|
| `AUTH-TOKEN-OK` | `token.authn.success` | 平台Token认证成功 |
| `AUTH-TOKEN-FAIL` | `token.authn.fail` | 平台Token认证失败 |
| `AUTH-SCOPE-DENY` | `token.authz.denied` | Scope权限不足 |
| `AUTH-QUERY-REJECT` | `token.query_key.rejected` | query key被拒绝 |
| `AUTH-QUERY-KEY` | - | query key请求（仅审计记录） |

**命名风格说明**：
- 设计文档使用 `CATEGORY-SUBCATEGORY` 格式（如 `AUTH-TOKEN-OK`），适合数据库索引和SQL查询
- TOK-002使用 `token.category.action` 格式（如 `token.authn.success`），适合日志和监控
- 两种格式等价，系统内部统一使用设计文档格式，外部接口可转换

### 12.2 结果码规范

格式：`{Domain}_{Code}`

示例：
- `SEC_CRED_EXPOSED`：凭证暴露
- `SEC_DIRECT_BYPASS`：直连绕过
- `AUTH_TOKEN_INVALID`：Token无效
- `AUTH_SCOPE_DENIED`：权限不足

#### 12.2.1 错误码体系对照表

本设计错误码与现有体系对齐：

| 错误码 | 来源 | 说明 | 对应事件 |
|--------|------|------|----------|
| `AUTH_MISSING_BEARER` | TOK-002 | 请求头缺失Bearer | AUTH-TOKEN-FAIL |
| `AUTH_INVALID_TOKEN` | TOK-002 | Token无效/签名失败 | AUTH-TOKEN-FAIL |
| `AUTH_TOKEN_INACTIVE` | TOK-002 | Token已吊销/过期 | AUTH-TOKEN-FAIL |
| `AUTH_SCOPE_DENIED` | TOK-002 | 权限不足 | AUTH-SCOPE-DENY |
| `QUERY_KEY_NOT_ALLOWED` | TOK-002 | query key外部入站拒绝 | AUTH-QUERY-REJECT |
| `SEC_CRED_EXPOSED` | XR-001 | 凭证泄露 | CRED-EXPOSE |
| `SEC_DIRECT_BYPASS` | XR-001 | 直连绕过 | CRED-DIRECT |
| `SEC_INV_PKG_*` | XR-001 | 供应方不变量违反 | INVARIANT-VIOLATION |
| `SEC_INV_SET_*` | XR-001 | 结算不变量违反 | INVARIANT-VIOLATION |
| `SUP_PKG_*` | 供应侧 | 供应方包相关错误 | CONFIG-* |
| `SUP_SET_*` | 供应侧 | 结算相关错误 | CONFIG-* |

### 12.3 参考文档

- `docs/acceptance_gate_single_source_v1_2026-03-18.md`
- `docs/supply_technical_design_enhanced_v1_2026-03-25.md`
- `docs/security_solution_v1_2026-03-18.md`
- `docs/supply_traceability_matrix_generation_rules_v1_2026-03-27.md`
