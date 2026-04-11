-- Outbox Pattern Schema v1.0
-- 用于跨系统事件发布的可靠消息传递

-- Outbox状态枚举
DO $$ BEGIN
    CREATE TYPE outbox_status AS ENUM ('pending', 'processing', 'completed', 'failed', 'dead_letter');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Outbox事件表
CREATE TABLE IF NOT EXISTS supply_outbox (
    id                  BIGSERIAL PRIMARY KEY,
    aggregate_type      VARCHAR(100) NOT NULL,
    aggregate_id        VARCHAR(100) NOT NULL,
    event_type          VARCHAR(100) NOT NULL,
    event_id            VARCHAR(100) NOT NULL UNIQUE,
    payload             JSONB NOT NULL,
    status              outbox_status NOT NULL DEFAULT 'pending',
    retry_count         INT NOT NULL DEFAULT 0,
    max_retries        INT NOT NULL DEFAULT 5,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at        TIMESTAMPTZ,
    next_retry_at      TIMESTAMPTZ,
    dead_letter_reason  TEXT,
    version             BIGINT NOT NULL DEFAULT 0,

    -- 约束
    CONSTRAINT valid_outbox_status CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_letter')),
    CONSTRAINT valid_retry_count CHECK (retry_count >= 0 AND retry_count <= max_retries)
);

-- 死信队列表
CREATE TABLE IF NOT EXISTS supply_outbox_dead_letter (
    id                   BIGSERIAL PRIMARY KEY,
    original_event_id    VARCHAR(100) NOT NULL,
    original_aggregate_type VARCHAR(100) NOT NULL,
    original_aggregate_id VARCHAR(100) NOT NULL,
    event_type           VARCHAR(100) NOT NULL,
    payload              JSONB NOT NULL,
    error_message        TEXT,
    retry_count          INT NOT NULL DEFAULT 0,
    first_failed_at      TIMESTAMPTZ NOT NULL,
    dead_letter_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    handled              BOOLEAN NOT NULL DEFAULT FALSE,
    handled_at           TIMESTAMPTZ,
    handler_notes        TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_outbox_status ON supply_outbox(status);
CREATE INDEX IF NOT EXISTS idx_outbox_status_next_retry ON supply_outbox(status, next_retry_at) WHERE status = 'failed';
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON supply_outbox(aggregate_type, aggregate_id);
CREATE INDEX IF NOT EXISTS idx_outbox_created_at ON supply_outbox(created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_event_id ON supply_outbox(event_id);

CREATE INDEX IF NOT EXISTS idx_dead_letter_original_event_id ON supply_outbox_dead_letter(original_event_id);
CREATE INDEX IF NOT EXISTS idx_dead_letter_handled ON supply_outbox_dead_letter(handled) WHERE handled = FALSE;
CREATE INDEX IF NOT EXISTS idx_dead_letter_created_at ON supply_outbox_dead_letter(created_at);

-- 注释
COMMENT ON TABLE supply_outbox IS 'Outbox事件表，用于可靠的事件发布';
COMMENT ON COLUMN supply_outbox.aggregate_type IS '聚合类型，如 account, package, settlement';
COMMENT ON COLUMN supply_outbox.aggregate_id IS '聚合ID';
COMMENT ON COLUMN supply_outbox.event_type IS '事件类型，如 account.created, package.updated';
COMMENT ON COLUMN supply_outbox.event_id IS '事件唯一ID（UUID）';
COMMENT ON COLUMN supply_outbox.payload IS '事件负载（JSON格式）';
COMMENT ON COLUMN supply_outbox.status IS '处理状态：pending/processing/completed/failed/dead_letter';
COMMENT ON COLUMN supply_outbox.retry_count IS '已重试次数';
COMMENT ON COLUMN supply_outbox.max_retries IS '最大重试次数';
COMMENT ON COLUMN supply_outbox.version IS '乐观锁版本号';

COMMENT ON TABLE supply_outbox_dead_letter IS 'Outbox死信队列，用于存储无法处理的事件';
COMMENT ON COLUMN supply_outbox_dead_letter.original_event_id IS '原始事件ID';
COMMENT ON COLUMN supply_outbox_dead_letter.first_failed_at IS '首次失败时间';

-- 自动更新version触发器
CREATE OR REPLACE FUNCTION update_outbox_version()
RETURNS TRIGGER AS $$
BEGIN
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_outbox_version ON supply_outbox;
CREATE TRIGGER trigger_outbox_version
    BEFORE UPDATE ON supply_outbox
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION update_outbox_version();
