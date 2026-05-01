CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS cs_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel VARCHAR(16) NOT NULL,
    open_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(64) NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'idle',
    turn_count INT NOT NULL DEFAULT 0,
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cs_sessions_channel CHECK (channel IN ('telegram','discord','wechat','widget')),
    CONSTRAINT chk_cs_sessions_status CHECK (status IN ('idle','processing','waiting_feedback','handoff','closed'))
);
CREATE INDEX IF NOT EXISTS idx_sessions_channel_openid ON cs_sessions(channel, open_id);

CREATE TABLE IF NOT EXISTS cs_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES cs_sessions(id) ON DELETE CASCADE,
    direction VARCHAR(8) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(16) NOT NULL DEFAULT 'text',
    intent VARCHAR(32) NULL,
    confidence NUMERIC(3,2) NULL,
    model_provider VARCHAR(32) NULL,
    latency_ms INT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cs_messages_direction CHECK (direction IN ('in','out'))
);
CREATE INDEX IF NOT EXISTS idx_messages_session_id ON cs_messages(session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS cs_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES cs_sessions(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NULL,
    priority VARCHAR(4) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'open',
    handoff_reason VARCHAR(32) NOT NULL,
    assigned_to VARCHAR(64) NULL,
    context_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    resolution TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cs_tickets_priority CHECK (priority IN ('P0','P1','P2','P3')),
    CONSTRAINT chk_cs_tickets_status CHECK (status IN ('open','assigned','processing','resolved','closed'))
);
CREATE INDEX IF NOT EXISTS idx_tickets_status_priority ON cs_tickets(status, priority, created_at);

CREATE TABLE IF NOT EXISTS cs_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(64) NOT NULL,
    object_type VARCHAR(32) NOT NULL,
    object_id VARCHAR(64) NOT NULL,
    action VARCHAR(16) NOT NULL,
    before_state JSONB NULL,
    after_state JSONB NULL,
    actor_id VARCHAR(64) NOT NULL,
    source_ip VARCHAR(45) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_object ON cs_audit_logs(object_type, object_id, created_at DESC);

CREATE TABLE IF NOT EXISTS cs_message_dedup (
    channel VARCHAR(16) NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    session_id UUID NULL REFERENCES cs_sessions(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel, message_id)
);
