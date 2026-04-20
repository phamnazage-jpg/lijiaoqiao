-- Partition Strategy Schema v1.0
-- 按月分区的大表分区策略

-- ==================== 1. audit_events 分区 (按月分区，保留12个月) ====================

-- 创建父表
CREATE TABLE IF NOT EXISTS audit_events (
    id                  BIGSERIAL,
    event_id            VARCHAR(100) NOT NULL,
    event_name          VARCHAR(100) NOT NULL,
    event_category      VARCHAR(50),
    event_sub_category  VARCHAR(50),
    timestamp           TIMESTAMPTZ NOT NULL,
    timestamp_ms        BIGINT NOT NULL,
    request_id          VARCHAR(100),
    idempotency_key     VARCHAR(128),
    tenant_id           BIGINT,
    object_type         VARCHAR(100),
    object_id           VARCHAR(100),
    action              VARCHAR(100) NOT NULL,
    result_code         VARCHAR(50),
    source_ip           VARCHAR(50),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- 创建月度分区函数
CREATE OR REPLACE FUNCTION create_audit_events_partition(partition_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := date_trunc('month', partition_date)::DATE;
    end_date := (start_date + INTERVAL '1 month')::DATE;
    partition_name := 'audit_events_' || to_char(start_date, 'YYYY_MM');

    -- 检查分区是否已存在
    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = partition_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF audit_events FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
        RAISE NOTICE 'Created partition: %', partition_name;
    ELSE
        RAISE NOTICE 'Partition already exists: %', partition_name;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ==================== 2. supply_usage_records 分区 (按月分区，保留3个月) ====================

CREATE TABLE IF NOT EXISTS supply_usage_records (
    id                  BIGSERIAL,
    order_id            BIGINT NOT NULL,
    buyer_user_id       BIGINT NOT NULL,
    supply_account_id   BIGINT NOT NULL,
    supplier_user_id    BIGINT NOT NULL,
    request_id          VARCHAR(64) NOT NULL,
    upstream_request_id VARCHAR(128),
    api_key_id          BIGINT,
    platform            VARCHAR(50) NOT NULL,
    model               VARCHAR(100) NOT NULL,
    endpoint            VARCHAR(100) NOT NULL,
    request_tokens      BIGINT,
    response_tokens     BIGINT,
    total_tokens        BIGINT,
    input_cost          NUMERIC(20,6),
    output_cost          NUMERIC(20,6),
    total_cost          NUMERIC(20,6) NOT NULL,
    unit_price          NUMERIC(20,6) NOT NULL,
    response_status     INT,
    latency_ms          INT,
    error_message       TEXT,
    success             BOOLEAN DEFAULT TRUE,
    started_at          TIMESTAMPTZ NOT NULL,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, started_at)
) PARTITION BY RANGE (started_at);

CREATE OR REPLACE FUNCTION create_usage_records_partition(partition_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := date_trunc('month', partition_date)::DATE;
    end_date := (start_date + INTERVAL '1 month')::DATE;
    partition_name := 'supply_usage_records_' || to_char(start_date, 'YYYY_MM');

    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = partition_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF supply_usage_records FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
        RAISE NOTICE 'Created partition: %', partition_name;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ==================== 3. supply_idempotency_records 幂等表 (非分区，保留7天) ====================

CREATE TABLE IF NOT EXISTS supply_idempotency_records (
    id                  BIGSERIAL,
    tenant_id           BIGINT NOT NULL,
    operator_id         BIGINT NOT NULL,
    api_path            VARCHAR(200) NOT NULL,
    idempotency_key     VARCHAR(128) NOT NULL,
    request_id          VARCHAR(64) NOT NULL,
    payload_hash        CHAR(64) NOT NULL,
    response_code       INT,
    response_body       JSONB,
    status              VARCHAR(20) NOT NULL DEFAULT 'processing',
    expires_at          TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    CONSTRAINT uq_supply_idempotency_records_key
        UNIQUE (tenant_id, operator_id, api_path, idempotency_key)
);

-- 向后兼容保留函数名；幂等表不再分区。
CREATE OR REPLACE FUNCTION create_idempotency_partition(partition_date DATE)
RETURNS VOID AS $$
BEGIN
    RAISE NOTICE 'supply_idempotency_records is no longer partitioned; skip partition setup for %', partition_date;
END;
$$ LANGUAGE plpgsql;

-- ==================== 4. 自动创建未来分区 ====================

CREATE OR REPLACE FUNCTION ensure_future_partitions()
RETURNS VOID AS $$
DECLARE
    i INT;
    future_date DATE;
BEGIN
    -- 为未来3个月创建分区
    FOR i IN 0..3 LOOP
        future_date := (CURRENT_DATE + (i || ' months')::INTERVAL)::DATE;
        PERFORM create_audit_events_partition(future_date);
        PERFORM create_usage_records_partition(future_date);
        PERFORM create_idempotency_partition(future_date);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- ==================== 5. 清理过期分区 ====================

CREATE OR REPLACE FUNCTION drop_old_audit_partitions(retention_months INT DEFAULT 12)
RETURNS INTEGER AS $$
DECLARE
    partition_name TEXT;
    cutoff_date DATE;
    dropped_count INTEGER := 0;
BEGIN
    cutoff_date := (CURRENT_DATE - (retention_months || ' months')::INTERVAL)::DATE;

    FOR partition_name IN
        SELECT relname
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE relname ~ 'audit_events_20[0-9]{2}_[0-9]{2}'
        AND n.nspname = 'public'
    LOOP
        -- 提取分区日期
        IF partition_name < 'audit_events_' || to_char(cutoff_date, 'YYYY_MM') THEN
            EXECUTE format('DROP TABLE IF EXISTS %I', partition_name);
            dropped_count := dropped_count + 1;
            RAISE NOTICE 'Dropped partition: %', partition_name;
        END IF;
    END LOOP;

    RETURN dropped_count;
END;
$$ LANGUAGE plpgsql;

-- ==================== 6. 初始化分区 (首次运行) ====================

-- 创建初始分区（过去12个月 + 未来3个月）
DO $$
DECLARE
    i INT;
    target_date DATE;
BEGIN
    -- 过去12个月
    FOR i IN -12..0 LOOP
        target_date := (CURRENT_DATE + (i || ' months')::INTERVAL)::DATE;
        PERFORM create_audit_events_partition(target_date);
        PERFORM create_usage_records_partition(target_date);
        PERFORM create_idempotency_partition(target_date);
    END LOOP;

    -- 未来3个月
    FOR i IN 1..3 LOOP
        target_date := (CURRENT_DATE + (i || ' months')::INTERVAL)::DATE;
        PERFORM create_audit_events_partition(target_date);
        PERFORM create_usage_records_partition(target_date);
        PERFORM create_idempotency_partition(target_date);
    END LOOP;
END $$;

-- ==================== 7. 索引 ====================

-- 在父表上创建索引（会自动继承到分区）
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_id ON audit_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_request_id ON audit_events(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_object ON audit_events(object_type, object_id);

CREATE INDEX IF NOT EXISTS idx_usage_records_order_id ON supply_usage_records(order_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_started_at ON supply_usage_records(started_at);

CREATE INDEX IF NOT EXISTS idx_idempotency_request_id ON supply_idempotency_records(request_id);
CREATE INDEX IF NOT EXISTS idx_idempotency_expires_at ON supply_idempotency_records(expires_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_status_expires ON supply_idempotency_records(status, expires_at);

-- ==================== 8. 注释 ====================

COMMENT ON TABLE audit_events IS '审计事件表 - 按月分区，保留12个月';
COMMENT ON TABLE supply_usage_records IS '使用记录表 - 按月分区，保留3个月';
COMMENT ON TABLE supply_idempotency_records IS '幂等记录表 - 非分区唯一表，保留7天以上';

-- 创建pg_cron作业定期维护分区（需要扩展 pg_cron）
-- SELECT cron.schedule('partition-maintenance', '0 0 * * *', 'SELECT ensure_future_partitions()');
-- SELECT cron.schedule('partition-cleanup', '0 1 * * *', 'SELECT drop_old_audit_partitions(12)');
