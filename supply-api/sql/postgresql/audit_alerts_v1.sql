-- 审计告警持久化表
-- 用途：为 /api/v1/audit/alerts 提供 PostgreSQL-backed 存储，避免 HTTP 层固定回退到内存实现。

CREATE TABLE IF NOT EXISTS audit_alerts (
    alert_id TEXT PRIMARY KEY,
    alert_name TEXT NOT NULL DEFAULT '',
    alert_type TEXT NOT NULL,
    alert_level TEXT NOT NULL,
    tenant_id BIGINT NOT NULL,
    supplier_id BIGINT,
    title TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    event_id TEXT,
    event_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    trigger_condition TEXT NOT NULL DEFAULT '',
    threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
    current_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    resolved_at TIMESTAMPTZ,
    resolved_by TEXT,
    resolve_note TEXT,
    notify_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notify_channels JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_audit_alerts_tenant_status_created_at
    ON audit_alerts (tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_alerts_tenant_type_level
    ON audit_alerts (tenant_id, alert_type, alert_level);

CREATE INDEX IF NOT EXISTS idx_audit_alerts_supplier_created_at
    ON audit_alerts (supplier_id, created_at DESC)
    WHERE supplier_id IS NOT NULL;
