-- Forward-only migration: align supply-api audit_events with the repository contract.
-- This migration is intended for databases that were initialized from the older
-- compact audit_events schema in supply-api/sql/postgresql/partition_strategy_v1.sql.

ALTER TABLE IF EXISTS audit_events
    ADD COLUMN IF NOT EXISTS trace_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS span_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS operator_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS operator_type VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS operator_role VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tenant_type VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS action_detail TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS credential_type VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS credential_id VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS credential_fingerprint VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_region VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_type VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_endpoint TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_direct BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS result_message TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS success BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS before_state JSONB,
    ADD COLUMN IF NOT EXISTS after_state JSONB,
    ADD COLUMN IF NOT EXISTS security_flags JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS risk_score INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS compliance_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    ADD COLUMN IF NOT EXISTS invariant_rule VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS extensions JSONB,
    ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

UPDATE audit_events
SET
    event_category = COALESCE(event_category, ''),
    event_sub_category = COALESCE(event_sub_category, ''),
    timestamp_ms = COALESCE(timestamp_ms, EXTRACT(EPOCH FROM timestamp) * 1000),
    request_id = COALESCE(request_id, ''),
    idempotency_key = COALESCE(idempotency_key, ''),
    tenant_id = COALESCE(tenant_id, 0),
    object_type = COALESCE(object_type, ''),
    action = COALESCE(action, ''),
    result_code = COALESCE(result_code, ''),
    source_ip = COALESCE(source_ip, '');

ALTER TABLE IF EXISTS audit_events
    ALTER COLUMN event_category SET DEFAULT '',
    ALTER COLUMN event_category SET NOT NULL,
    ALTER COLUMN event_sub_category SET DEFAULT '',
    ALTER COLUMN event_sub_category SET NOT NULL,
    ALTER COLUMN timestamp_ms SET DEFAULT 0,
    ALTER COLUMN timestamp_ms SET NOT NULL,
    ALTER COLUMN request_id SET DEFAULT '',
    ALTER COLUMN request_id SET NOT NULL,
    ALTER COLUMN idempotency_key SET DEFAULT '',
    ALTER COLUMN idempotency_key SET NOT NULL,
    ALTER COLUMN tenant_id SET DEFAULT 0,
    ALTER COLUMN tenant_id SET NOT NULL,
    ALTER COLUMN object_type SET DEFAULT '',
    ALTER COLUMN object_type SET NOT NULL,
    ALTER COLUMN result_code SET DEFAULT '',
    ALTER COLUMN result_code SET NOT NULL,
    ALTER COLUMN source_ip SET DEFAULT '',
    ALTER COLUMN source_ip SET NOT NULL;

DO $$
DECLARE
    current_type TEXT;
BEGIN
    SELECT data_type
    INTO current_type
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'audit_events'
      AND column_name = 'object_id';

    IF current_type IS NULL THEN
        ALTER TABLE audit_events
            ADD COLUMN object_id BIGINT NOT NULL DEFAULT 0;
    ELSIF current_type <> 'bigint' THEN
        UPDATE audit_events
        SET object_id = '0'
        WHERE object_id IS NULL
           OR object_id = ''
           OR object_id !~ '^[0-9]+$';

        ALTER TABLE audit_events
            ALTER COLUMN object_id TYPE BIGINT USING object_id::BIGINT;
    END IF;

    ALTER TABLE audit_events
        ALTER COLUMN object_id SET DEFAULT 0,
        ALTER COLUMN object_id SET NOT NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_audit_events_event_id ON audit_events(event_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_event_name ON audit_events(event_name);
CREATE INDEX IF NOT EXISTS idx_audit_events_trace_id ON audit_events(trace_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_idempotency_key ON audit_events(idempotency_key);
