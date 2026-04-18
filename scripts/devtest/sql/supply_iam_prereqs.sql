-- Devtest-only prerequisites for enabling supply-api IAM routes in the supply database.
-- Purpose:
--   1. Create the minimum platform-side tables required by sql/postgresql/iam_schema_v1.sql.
--   2. Avoid importing platform_core_schema_v1.sql wholesale, because its audit_events baseline
--      conflicts with supply-api/sql/postgresql/partition_strategy_v1.sql in the same schema.

BEGIN;

CREATE TABLE IF NOT EXISTS core_tenants (
    id BIGINT PRIMARY KEY,
    tenant_code VARCHAR(64) NOT NULL UNIQUE,
    tenant_name VARCHAR(128) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'disabled')),
    plan_code VARCHAR(32) NOT NULL DEFAULT 'devtest',
    billing_currency CHAR(3) NOT NULL DEFAULT 'USD',
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT,
    updated_by BIGINT
);

CREATE INDEX IF NOT EXISTS idx_core_tenants_status ON core_tenants (status);

CREATE TABLE IF NOT EXISTS iam_users (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES core_tenants(id),
    email VARCHAR(256) NOT NULL,
    display_name VARCHAR(128),
    role_code VARCHAR(32) NOT NULL DEFAULT 'developer',
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'locked', 'disabled')),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, email)
);

CREATE INDEX IF NOT EXISTS idx_iam_users_tenant_role ON iam_users (tenant_id, role_code);
CREATE INDEX IF NOT EXISTS idx_iam_users_tenant_status ON iam_users (tenant_id, status);

COMMIT;
