CREATE TABLE IF NOT EXISTS cs_platform_callbacks (
    platform VARCHAR(32) NOT NULL,
    target_name VARCHAR(64) NOT NULL,
    callback_url TEXT NOT NULL,
    callback_secret TEXT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (platform, target_name)
);

CREATE TABLE IF NOT EXISTS cs_platform_event_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    session_id UUID NULL REFERENCES cs_sessions(id) ON DELETE SET NULL,
    ticket_id UUID NULL REFERENCES cs_tickets(id) ON DELETE SET NULL,
    source_message_id VARCHAR(128) NULL,
    callback_target VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempt_count INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cs_platform_event_outbox_status CHECK (status IN ('pending','retrying','delivered','dead_letter'))
);
CREATE INDEX IF NOT EXISTS idx_cs_platform_event_outbox_due ON cs_platform_event_outbox(status, next_attempt_at, created_at);
CREATE INDEX IF NOT EXISTS idx_cs_platform_event_outbox_platform ON cs_platform_event_outbox(platform, callback_target, created_at DESC);

CREATE TABLE IF NOT EXISTS cs_platform_event_delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES cs_platform_event_outbox(id) ON DELETE CASCADE,
    attempt_no INT NOT NULL,
    response_status INT NULL,
    response_body TEXT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cs_platform_event_delivery_attempts_event ON cs_platform_event_delivery_attempts(event_id, created_at DESC);

CREATE TABLE IF NOT EXISTS cs_platform_event_dead_letters (
    event_id UUID PRIMARY KEY REFERENCES cs_platform_event_outbox(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    callback_target VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempt_count INT NOT NULL DEFAULT 0,
    final_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
