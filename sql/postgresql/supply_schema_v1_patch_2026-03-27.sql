-- Supply schema patch for crypto/unit/audit/index gaps (PostgreSQL 15)
-- Base schema: supply_schema_v1.sql
-- Updated: 2026-03-27

BEGIN;

-- supply_accounts: crypto metadata, unit fields, audit fields, optimistic lock version
ALTER TABLE supply_accounts
    ADD COLUMN IF NOT EXISTS credential_cipher_algo VARCHAR(32) NOT NULL DEFAULT 'AES-256-GCM',
    ADD COLUMN IF NOT EXISTS credential_kms_key_alias VARCHAR(128) NOT NULL DEFAULT 'kms/supply/default',
    ADD COLUMN IF NOT EXISTS credential_key_version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS credential_fingerprint CHAR(64),
    ADD COLUMN IF NOT EXISTS last_rotation_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS quota_unit VARCHAR(16) NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_ip INET,
    ADD COLUMN IF NOT EXISTS updated_ip INET,
    ADD COLUMN IF NOT EXISTS audit_trace_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_supply_accounts_user_status_updated
    ON supply_accounts (user_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_accounts_platform_status_updated
    ON supply_accounts (platform, status, updated_at DESC);

-- supply_packages: unit and version fields
ALTER TABLE supply_packages
    ADD COLUMN IF NOT EXISTS quota_unit VARCHAR(16) NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS price_unit VARCHAR(32) NOT NULL DEFAULT 'per_1m_tokens',
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_ip INET,
    ADD COLUMN IF NOT EXISTS updated_ip INET,
    ADD COLUMN IF NOT EXISTS audit_trace_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_supply_packages_user_status_updated
    ON supply_packages (user_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_packages_platform_model_status
    ON supply_packages (platform, model, status);
CREATE INDEX IF NOT EXISTS idx_supply_packages_active_lookup
    ON supply_packages (user_id, platform, model)
    WHERE status = 'active';

-- supply_orders: idempotency and unit fields
ALTER TABLE supply_orders
    ADD COLUMN IF NOT EXISTS quota_unit VARCHAR(16) NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS billing_unit VARCHAR(32) NOT NULL DEFAULT 'per_1m_tokens',
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(128),
    ADD COLUMN IF NOT EXISTS audit_trace_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_ip INET,
    ADD COLUMN IF NOT EXISTS updated_ip INET;

CREATE INDEX IF NOT EXISTS idx_supply_orders_buyer_status_created
    ON supply_orders (buyer_user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_orders_supplier_status_created
    ON supply_orders (supplier_user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_orders_request_id
    ON supply_orders (request_id);

-- supply_usage_records: unit and trace fields
ALTER TABLE supply_usage_records
    ADD COLUMN IF NOT EXISTS token_unit VARCHAR(16) NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS cost_currency CHAR(3) NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS billing_unit VARCHAR(32) NOT NULL DEFAULT 'per_1m_tokens',
    ADD COLUMN IF NOT EXISTS trace_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS client_tenant_id BIGINT,
    ADD COLUMN IF NOT EXISTS created_ip INET;

CREATE INDEX IF NOT EXISTS idx_supply_usage_records_order_started
    ON supply_usage_records (order_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_usage_records_supplier_started
    ON supply_usage_records (supplier_user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_usage_records_trace_id
    ON supply_usage_records (trace_id);

-- supply_earnings: accounting unit and audit fields
ALTER TABLE supply_earnings
    ADD COLUMN IF NOT EXISTS amount_unit VARCHAR(16) NOT NULL DEFAULT 'minor',
    ADD COLUMN IF NOT EXISTS source_request_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS audit_trace_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_ip INET,
    ADD COLUMN IF NOT EXISTS updated_ip INET;

CREATE INDEX IF NOT EXISTS idx_supply_earnings_user_status_available
    ON supply_earnings (user_id, status, available_at);
CREATE INDEX IF NOT EXISTS idx_supply_earnings_source_request_id
    ON supply_earnings (source_request_id);

-- supply_settlements: accounting units and idempotency fields
ALTER TABLE supply_settlements
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS amount_unit VARCHAR(16) NOT NULL DEFAULT 'minor',
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(128),
    ADD COLUMN IF NOT EXISTS audit_trace_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_ip INET,
    ADD COLUMN IF NOT EXISTS updated_ip INET;

CREATE INDEX IF NOT EXISTS idx_supply_settlements_user_status_updated
    ON supply_settlements (user_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_supply_settlements_request_id
    ON supply_settlements (request_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_supply_settlements_user_processing
    ON supply_settlements (user_id)
    WHERE status = 'processing';

COMMIT;
