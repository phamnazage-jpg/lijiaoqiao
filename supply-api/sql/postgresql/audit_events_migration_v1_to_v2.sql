-- ============================================================================
-- 审计事件表 Schema 迁移 v1 -> v2
-- 问题: 数据库中的 audit_events 表是旧版 schema，与代码中的 model 不匹配
--
-- 旧版 schema (当前生产表):
--   domain_code, action_code, severity, client_ip, before_data, after_data
--
-- 新版 schema (代码期望):
--   event_id, event_name, event_category, timestamp, timestamp_ms,
--   action, source_ip, operator_id, operator_type 等
-- ============================================================================

-- 1. 备份现有数据（如果表中有数据）
CREATE TABLE IF NOT EXISTS audit_events_backup AS
SELECT * FROM audit_events WHERE 1=0;

-- 如果有数据则备份
INSERT INTO audit_events_backup SELECT * FROM audit_events;

-- 2. 删除旧表和约束
DROP TABLE IF EXISTS audit_events CASCADE;

-- 3. 创建新表（使用 partition_strategy_v1.sql 中的定义）
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

-- 4. 创建索引
CREATE INDEX idx_audit_events_tenant_id ON audit_events(tenant_id);
CREATE INDEX idx_audit_events_request_id ON audit_events(request_id);
CREATE INDEX idx_audit_events_created_at ON audit_events(created_at);
CREATE INDEX idx_audit_events_object ON audit_events(object_type, object_id);

-- 5. 创建初始分区（过去12个月 + 未来3个月）
DO $$
DECLARE
    i INT;
    target_date DATE;
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    -- 过去12个月
    FOR i IN -12..0 LOOP
        target_date := (CURRENT_DATE + (i || ' months')::INTERVAL)::DATE;
        start_date := date_trunc('month', target_date)::DATE;
        end_date := (start_date + INTERVAL '1 month')::DATE;
        partition_name := 'audit_events_' || to_char(start_date, 'YYYY_MM');

        IF NOT EXISTS (
            SELECT 1 FROM pg_class WHERE relname = partition_name
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF audit_events FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
            RAISE NOTICE 'Created partition: %', partition_name;
        END IF;
    END LOOP;

    -- 未来3个月
    FOR i IN 1..3 LOOP
        target_date := (CURRENT_DATE + (i || ' months')::INTERVAL)::DATE;
        start_date := date_trunc('month', target_date)::DATE;
        end_date := (start_date + INTERVAL '1 month')::DATE;
        partition_name := 'audit_events_' || to_char(start_date, 'YYYY_MM');

        IF NOT EXISTS (
            SELECT 1 FROM pg_class WHERE relname = partition_name
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF audit_events FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
            RAISE NOTICE 'Created partition: %', partition_name;
        END IF;
    END LOOP;
END $$;

-- 6. 验证
SELECT table_name, count(*) as partition_count
FROM pg_tables
WHERE table_name LIKE 'audit_events%' AND table_name != 'audit_events' AND table_name != 'audit_events_backup'
GROUP BY table_name;

COMMENT ON TABLE audit_events IS '审计事件表 - 按月分区，保留12个月';