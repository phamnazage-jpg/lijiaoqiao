-- P0-08 分区策略实现
-- 为 audit_events 和 billing_ledger_entries 创建分区表
-- 基于: docs/P0_issues_enhanced_design_v1_2026-04-07.md

-- ==================== 审计日志分区 (audit_events) ====================

-- 删除旧表（如果存在且可以重建）
DROP TABLE IF EXISTS audit_events CASCADE;

-- 创建分区表
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
    severity VARCHAR(16) NOT NULL DEFAULT 'info'
        CHECK (severity IN ('info', 'warn', 'error', 'critical')),
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

-- 创建索引（在分区父表上定义，子表会自动继承）
CREATE INDEX idx_audit_events_tenant_domain_time
    ON audit_events (tenant_id, domain_code, created_at DESC);
CREATE INDEX idx_audit_events_request_id
    ON audit_events (request_id);
CREATE INDEX idx_audit_events_trace_id
    ON audit_events (trace_id);
CREATE INDEX idx_audit_events_result_code
    ON audit_events (result_code);

-- 2026年月度分区
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
CREATE TABLE audit_events_2026_07 PARTITION OF audit_events
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_events_2026_08 PARTITION OF audit_events
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE audit_events_2026_09 PARTITION OF audit_events
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE audit_events_2026_10 PARTITION OF audit_events
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE audit_events_2026_11 PARTITION OF audit_events
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE audit_events_2026_12 PARTITION OF audit_events
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- 2027年季度分区（简化管理）
CREATE TABLE audit_events_2027_q1 PARTITION OF audit_events
    FOR VALUES FROM ('2027-01-01') TO ('2027-04-01');
CREATE TABLE audit_events_2027_q2 PARTITION OF audit_events
    FOR VALUES FROM ('2027-04-01') TO ('2027-07-01');
CREATE TABLE audit_events_2027_q3 PARTITION OF audit_events
    FOR VALUES FROM ('2027-07-01') TO ('2027-10-01');
CREATE TABLE audit_events_2027_q4 PARTITION OF audit_events
    FOR VALUES FROM ('2027-10-01') TO ('2028-01-01');

-- 默认分区（捕获未预期的数据）
CREATE TABLE audit_events_default PARTITION OF audit_events DEFAULT;

-- ==================== 账务分录分区 (billing_ledger_entries) ====================

-- 删除旧表（如果存在且可以重建）
DROP TABLE IF EXISTS billing_ledger_entries CASCADE;

-- 创建分区表
CREATE TABLE billing_ledger_entries (
    id BIGINT NOT NULL,
    billing_account_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    project_id BIGINT,
    user_id BIGINT,
    request_id VARCHAR(64) NOT NULL,
    trace_id VARCHAR(64),
    entry_type VARCHAR(32) NOT NULL,
    direction VARCHAR(2) NOT NULL
        CHECK (direction IN ('dr', 'cr')),
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

-- 创建索引
CREATE INDEX idx_billing_ledger_entries_account_time
    ON billing_ledger_entries (billing_account_id, occurred_at DESC);
CREATE INDEX idx_billing_ledger_entries_tenant_time
    ON billing_ledger_entries (tenant_id, occurred_at DESC);
CREATE INDEX idx_billing_ledger_entries_trace_id
    ON billing_ledger_entries (trace_id);
CREATE UNIQUE INDEX idx_billing_ledger_entries_idem_key
    ON billing_ledger_entries (tenant_id, request_id, entry_type)
    WHERE idempotency_key IS NOT NULL;

-- 2026年月度分区
CREATE TABLE billing_ledger_2026_04 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE billing_ledger_2026_05 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE billing_ledger_2026_06 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE billing_ledger_2026_07 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE billing_ledger_2026_08 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE billing_ledger_2026_09 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE billing_ledger_2026_10 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE billing_ledger_2026_11 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE billing_ledger_2026_12 PARTITION OF billing_ledger_entries
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- 默认分区
CREATE TABLE billing_ledger_default PARTITION OF billing_ledger_entries DEFAULT;

-- ==================== 分区维护存储过程 ====================

-- 自动创建新分区的存储过程（每日执行）
CREATE OR REPLACE FUNCTION create_monthly_partition(
    target_table TEXT,
    partition_date DATE
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := date_trunc('month', partition_date);
    end_date := start_date + INTERVAL '1 month';
    partition_name := target_table || '_' || to_char(start_date, 'YYYY_MM');

    -- 检查分区是否已存在
    IF NOT EXISTS (
        SELECT 1 FROM pg_tables
        WHERE tablename = partition_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
            partition_name, target_table, start_date, end_date
        );
        RAISE NOTICE 'Created partition: %', partition_name;
    ELSE
        RAISE NOTICE 'Partition already exists: %', partition_name;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- 分区清理存储过程（保留24个月）
CREATE OR REPLACE FUNCTION drop_old_partitions(
    target_table TEXT,
    retention_months INT DEFAULT 24
) RETURNS VOID AS $$
DECLARE
    partition_record RECORD;
    cutoff_date DATE;
    partition_name TEXT;
    partition_date DATE;
BEGIN
    cutoff_date := date_trunc('month', CURRENT_DATE) - (retention_months || ' months')::INTERVAL;

    FOR partition_record IN
        SELECT inhrelid::regclass::text AS partition_name
        FROM pg_inherits
        WHERE inhparent = target_table::regclass
    LOOP
        partition_name := partition_record.partition_name;

        -- 检查是否是月度分区（格式: table_YYYY_MM）
        IF partition_name ~ (target_table || '_[0-9]{4}_[0-9]{2}$') THEN
            partition_date := to_date(
                substring(partition_name from target_table || '_(.*)'), 'YYYY_MM'
            );

            IF partition_date < cutoff_date THEN
                RAISE NOTICE 'Dropping partition: % (older than %)', partition_name, cutoff_date;
                EXECUTE format('DROP TABLE IF EXISTS %I', partition_name);
            END IF;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- ==================== 保留策略清理 ====================

-- 清理超过保留期限的审计日志（保留1年）
CREATE OR REPLACE FUNCTION cleanup_audit_events_partitions(
    retention_months INT DEFAULT 12
) RETURNS VOID AS $$
BEGIN
    PERFORM drop_old_partitions('audit_events', retention_months);
END;
$$ LANGUAGE plpgsql;

-- 账务分录永久保留，不执行清理
-- CREATE OR REPLACE FUNCTION cleanup_billing_ledger_partitions() IS NOT NEEDED
-- billing_ledger_entries 应该永久保留

-- ==================== 验证查询 ====================

-- 查看所有分区
SELECT
    parent.relname AS parent_table,
    child.relname AS partition_name,
    pg_get_expr(child.relpartbound, child.oid, true) AS partition_range
FROM pg_inherits
JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
JOIN pg_class child ON pg_inherits.inhrelid = child.oid
WHERE parent.relname IN ('audit_events', 'billing_ledger_entries')
ORDER BY parent.relname, child.relname;

-- ==================== 权限设置 ====================

-- 授予应用角色必要权限
-- GRANT SELECT, INSERT, UPDATE, DELETE ON audit_events TO app_role;
-- GRANT SELECT, INSERT, UPDATE, DELETE ON billing_ledger_entries TO app_role;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_role;

COMMENT ON TABLE audit_events IS '审计事件表，按月分区，保留1年';
COMMENT ON TABLE billing_ledger_entries IS '账务分录表，按月分区，永久保留';
