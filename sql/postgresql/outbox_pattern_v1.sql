-- P0-006 Outbox模式实现
-- 基于: docs/P0_issues_enhanced_design_v1_2026-04-07.md

-- ==================== Outbox事件表 ====================
CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGSERIAL PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL,      -- 聚合类型: supply_account, package, settlement
    aggregate_id VARCHAR(128) NOT NULL,       -- 聚合ID
    event_type VARCHAR(128) NOT NULL,        -- 事件类型: created, updated, revoked
    event_id VARCHAR(64) NOT NULL UNIQUE,     -- 事件全局唯一ID (UUID)
    payload JSONB NOT NULL,                   -- 事件载荷
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_letter')),
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    dead_letter_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1
);

-- 索引
CREATE INDEX idx_outbox_events_status_next_retry
    ON outbox_events (status, next_retry_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id);
CREATE INDEX idx_outbox_events_created_at
    ON outbox_events (created_at);
CREATE INDEX idx_outbox_events_event_id
    ON outbox_events (event_id);

COMMENT ON TABLE outbox_events IS 'Outbox事件表，用于可靠消息投递';
COMMENT ON COLUMN outbox_events.aggregate_type IS '聚合类型，如 supply_account, package, settlement';
COMMENT ON COLUMN outbox_events.event_type IS '事件类型，如 created, updated, revoked';
COMMENT ON COLUMN outbox_events.status IS '状态: pending, processing, completed, failed, dead_letter';
COMMENT ON COLUMN outbox_events.max_retries IS '最大重试次数，默认5次';

-- ==================== 死信队列表 ====================
CREATE TABLE IF NOT EXISTS outbox_dead_letter (
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

-- 索引
CREATE INDEX idx_outbox_dead_letter_unhandled
    ON outbox_dead_letter (handled, dead_letter_at)
    WHERE handled = FALSE;
CREATE INDEX idx_outbox_dead_letter_original_event
    ON outbox_dead_letter (original_event_id);

COMMENT ON TABLE outbox_dead_letter IS 'Outbox死信队列，存储超过最大重试次数的事件';
COMMENT ON COLUMN outbox_dead_letter.handled IS '是否已处理';
COMMENT ON COLUMN outbox_dead_letter.handler_notes IS '处理备注';

-- ==================== 补偿记录表 (P0-007) ====================
CREATE TABLE IF NOT EXISTS supply_batch_compensation (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(64) NOT NULL,
    operation_type VARCHAR(32) NOT NULL,
    item_index INT NOT NULL,
    item_payload JSONB NOT NULL,
    failure_reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
        CHECK (status IN ('pending', 'retrying', 'resolved', 'manual_required', 'abandoned')),
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    resolved_at TIMESTAMPTZ,
    resolved_by BIGINT,
    resolution_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT,
    version BIGINT NOT NULL DEFAULT 1
);

-- 索引
CREATE INDEX idx_compensation_batch
    ON supply_batch_compensation (batch_id, status);
CREATE INDEX idx_compensation_status
    ON supply_batch_compensation (status, created_at);

COMMENT ON TABLE supply_batch_compensation IS '批量操作补偿记录';
COMMENT ON COLUMN supply_batch_compensation.batch_id IS '批量任务ID';
COMMENT ON COLUMN supply_batch_compensation.status IS '状态: pending, retrying, resolved, manual_required, abandoned';

-- ==================== Outbox处理器 ====================

-- 获取待处理事件（带悲观锁）
CREATE OR REPLACE FUNCTION fetch_and_lock_outbox_events(
    p_limit INT DEFAULT 100
) RETURNS SETOF outbox_events AS $$
DECLARE
    r outbox_events%ROWTYPE;
BEGIN
    FOR r IN
        SELECT *
        FROM outbox_events
        WHERE status IN ('pending', 'failed')
          AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
        ORDER BY created_at ASC
        LIMIT p_limit
        FOR UPDATE SKIP LOCKED
    LOOP
        -- 更新状态为processing
        UPDATE outbox_events
        SET status = 'processing',
            version = version + 1
        WHERE id = r.id
          AND version = r.version;

        IF FOUND THEN
            RETURN NEXT r;
        END IF;
    END LOOP;
    RETURN;
END;
$$ LANGUAGE plpgsql;

-- 标记事件完成
CREATE OR REPLACE FUNCTION mark_outbox_completed(
    p_event_id VARCHAR(64)
) RETURNS VOID AS $$
BEGIN
    UPDATE outbox_events
    SET status = 'completed',
        processed_at = CURRENT_TIMESTAMP
    WHERE event_id = p_event_id;
END;
$$ LANGUAGE plpgsql;

-- 标记事件失败并计算下次重试时间
CREATE OR REPLACE FUNCTION mark_outbox_failed(
    p_event_id VARCHAR(64),
    p_error_message TEXT
) RETURNS VOID AS $$
DECLARE
    v_event outbox_events%ROWTYPE;
    v_backoff_seconds INT;
BEGIN
    -- 获取事件信息
    SELECT * INTO v_event FROM outbox_events WHERE event_id = p_event_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    -- 计算重试次数
    UPDATE outbox_events
    SET retry_count = retry_count + 1,
        error_message = p_error_message,
        version = version + 1
    WHERE event_id = p_event_id;

    -- 重新获取更新后的事件
    SELECT * INTO v_event FROM outbox_events WHERE event_id = p_event_id;

    -- 检查是否超过最大重试次数
    IF v_event.retry_count >= v_event.max_retries THEN
        -- 移入死信队列
        INSERT INTO outbox_dead_letter (
            original_event_id,
            original_aggregate_type,
            original_aggregate_id,
            event_type,
            payload,
            error_message,
            retry_count,
            first_failed_at
        ) VALUES (
            v_event.event_id,
            v_event.aggregate_type,
            v_event.aggregate_id,
            v_event.event_type,
            v_event.payload,
            p_error_message,
            v_event.retry_count,
            v_event.created_at
        );

        -- 更新状态为dead_letter
        UPDATE outbox_events
        SET status = 'dead_letter',
            dead_letter_reason = p_error_message
        WHERE event_id = p_event_id;
    ELSE
        -- 计算指数退避时间
        v_backoff_seconds := LEAST(60, POWER(2, v_event.retry_count))::INT;

        UPDATE outbox_events
        SET status = 'failed',
            next_retry_at = CURRENT_TIMESTAMP + (v_backoff_seconds || ' seconds')::INTERVAL
        WHERE event_id = p_event_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ==================== 补偿记录操作 ====================

-- 创建补偿记录
CREATE OR REPLACE FUNCTION create_compensation(
    p_batch_id VARCHAR(64),
    p_operation_type VARCHAR(32),
    p_item_index INT,
    p_item_payload JSONB,
    p_failure_reason TEXT
) RETURNS BIGINT AS $$
DECLARE
    v_id BIGINT;
BEGIN
    INSERT INTO supply_batch_compensation (
        batch_id,
        operation_type,
        item_index,
        item_payload,
        failure_reason,
        status
    ) VALUES (
        p_batch_id,
        p_operation_type,
        p_item_index,
        p_item_payload,
        p_failure_reason,
        'pending'
    )
    RETURNING id INTO v_id;

    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- 重试补偿
CREATE OR REPLACE FUNCTION retry_compensation(
    p_id BIGINT
) RETURNS VOID AS $$
BEGIN
    UPDATE supply_batch_compensation
    SET status = 'retrying',
        retry_count = retry_count + 1,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_id;
END;
$$ LANGUAGE plpgsql;

-- 解决补偿
CREATE OR REPLACE FUNCTION resolve_compensation(
    p_id BIGINT,
    p_resolved_by BIGINT,
    p_notes TEXT
) RETURNS VOID AS $$
BEGIN
    UPDATE supply_batch_compensation
    SET status = 'resolved',
        resolved_at = CURRENT_TIMESTAMP,
        resolved_by = p_resolved_by,
        resolution_notes = p_notes,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_id;
END;
$$ LANGUAGE plpgsql;

-- 标记需要人工介入
CREATE OR REPLACE FUNCTION mark_compensation_manual_required(
    p_id BIGINT,
    p_reason TEXT
) RETURNS VOID AS $$
BEGIN
    UPDATE supply_batch_compensation
    SET status = 'manual_required',
        failure_reason = COALESCE(failure_reason || '; ', '') || p_reason,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_id;
END;
$$ LANGUAGE plpgsql;

-- ==================== 统计查询 ====================

-- 获取Outbox处理统计
CREATE OR REPLACE FUNCTION get_outbox_stats() RETURNS TABLE(
    status VARCHAR(20),
    count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        status,
        COUNT(*)
    FROM outbox_events
    GROUP BY status;
END;
$$ LANGUAGE plpgsql;

-- 获取死信队列统计
CREATE OR REPLACE FUNCTION get_dead_letter_stats() RETURNS TABLE(
    handled BOOLEAN,
    count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        handled,
        COUNT(*)
    FROM outbox_dead_letter
    GROUP BY handled;
END;
$$ LANGUAGE plpgsql;

-- 获取需要人工介入的补偿记录
CREATE OR REPLACE FUNCTION get_pending_manual_compensations()
RETURNS SETOF supply_batch_compensation AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM supply_batch_compensation
    WHERE status = 'manual_required'
    ORDER BY created_at ASC;
END;
$$ LANGUAGE plpgsql;
