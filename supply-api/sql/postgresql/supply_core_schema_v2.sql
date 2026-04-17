-- Supply Core Schema v2
-- 统一维护 supply-api 核心业务表的当前事实源。
-- 范围：
--   1. supply_accounts
--   2. supply_packages
--   3. supply_settlements
--
-- 说明：
-- - 审计事件、分区策略、outbox、token 状态、告警表仍由各自独立 DDL 维护。
-- - 当前仓库仍使用应用层外键校验，因此这里先不引入数据库级外键，避免把未收口的关系语义写死。
-- - status CHECK 只收敛当前代码真实使用的状态集合；如果领域状态扩展，必须先修改 domain 常量与仓储测试，再更新本 DDL。

CREATE TABLE IF NOT EXISTS supply_accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    platform VARCHAR(64) NOT NULL,
    account_type VARCHAR(32) NOT NULL,
    account_name VARCHAR(255) NOT NULL DEFAULT '',
    encrypted_credentials TEXT NOT NULL DEFAULT '',
    key_id VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL
        CHECK (status IN ('pending', 'active', 'suspended', 'disabled')),
    risk_level VARCHAR(32) NOT NULL DEFAULT '',
    total_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    available_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    frozen_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    last_check_at TIMESTAMPTZ,
    tos_compliant BOOLEAN NOT NULL DEFAULT FALSE,
    tos_check_result TEXT NOT NULL DEFAULT '',
    total_requests BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    total_cost NUMERIC(20, 6) NOT NULL DEFAULT 0,
    success_rate NUMERIC(10, 4) NOT NULL DEFAULT 0,
    risk_score INTEGER NOT NULL DEFAULT 0,
    risk_reason TEXT NOT NULL DEFAULT '',
    is_frozen BOOLEAN NOT NULL DEFAULT FALSE,
    frozen_reason TEXT NOT NULL DEFAULT '',
    credential_cipher_algo VARCHAR(64) NOT NULL DEFAULT '',
    credential_kms_key_alias VARCHAR(255) NOT NULL DEFAULT '',
    credential_key_version INTEGER NOT NULL DEFAULT 1,
    quota_unit VARCHAR(32) NOT NULL DEFAULT 'token',
    currency_code VARCHAR(16) NOT NULL DEFAULT 'USD',
    version INTEGER NOT NULL DEFAULT 0,
    created_ip INET,
    updated_ip INET,
    audit_trace_id VARCHAR(128) NOT NULL DEFAULT '',
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_supply_accounts_user_status_created_at
    ON supply_accounts (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_supply_accounts_request_id
    ON supply_accounts (request_id)
    WHERE request_id <> '';

CREATE INDEX IF NOT EXISTS idx_supply_accounts_idempotency_key
    ON supply_accounts (user_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS supply_packages (
    id BIGSERIAL PRIMARY KEY,
    supply_account_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    platform VARCHAR(64) NOT NULL DEFAULT '',
    model VARCHAR(128) NOT NULL,
    total_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    available_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    sold_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    reserved_quota NUMERIC(20, 6) NOT NULL DEFAULT 0,
    price_per_1m_input NUMERIC(20, 6) NOT NULL DEFAULT 0,
    price_per_1m_output NUMERIC(20, 6) NOT NULL DEFAULT 0,
    min_purchase NUMERIC(20, 6) NOT NULL DEFAULT 0,
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    valid_days INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL
        CHECK (status IN ('draft', 'active', 'paused', 'sold_out', 'expired')),
    max_concurrent INTEGER NOT NULL DEFAULT 0,
    rate_limit_rpm INTEGER NOT NULL DEFAULT 0,
    total_orders INTEGER NOT NULL DEFAULT 0,
    total_revenue NUMERIC(20, 6) NOT NULL DEFAULT 0,
    rating NUMERIC(10, 4) NOT NULL DEFAULT 0,
    rating_count INTEGER NOT NULL DEFAULT 0,
    quota_unit VARCHAR(32) NOT NULL DEFAULT 'token',
    price_unit VARCHAR(32) NOT NULL DEFAULT 'per_1m_tokens',
    currency_code VARCHAR(16) NOT NULL DEFAULT 'USD',
    version INTEGER NOT NULL DEFAULT 0,
    created_ip INET,
    updated_ip INET,
    audit_trace_id VARCHAR(128) NOT NULL DEFAULT '',
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_supply_packages_user_status_created_at
    ON supply_packages (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_supply_packages_account_id
    ON supply_packages (supply_account_id);

CREATE INDEX IF NOT EXISTS idx_supply_packages_request_id
    ON supply_packages (request_id)
    WHERE request_id <> '';

CREATE TABLE IF NOT EXISTS supply_settlements (
    id BIGSERIAL PRIMARY KEY,
    settlement_no VARCHAR(128) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    total_amount NUMERIC(20, 6) NOT NULL DEFAULT 0,
    fee_amount NUMERIC(20, 6) NOT NULL DEFAULT 0,
    net_amount NUMERIC(20, 6) NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    payment_method VARCHAR(32) NOT NULL DEFAULT '',
    payment_account VARCHAR(255) NOT NULL DEFAULT '',
    period_start TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    period_end TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    total_orders INTEGER NOT NULL DEFAULT 0,
    total_usage_records INTEGER NOT NULL DEFAULT 0,
    payment_transaction_id VARCHAR(128) NOT NULL DEFAULT '',
    paid_at TIMESTAMPTZ,
    currency_code VARCHAR(16) NOT NULL DEFAULT 'USD',
    amount_unit VARCHAR(16) NOT NULL DEFAULT 'minor',
    version INTEGER NOT NULL DEFAULT 0,
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
    audit_trace_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_supply_settlements_user_status_created_at
    ON supply_settlements (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_supply_settlements_idempotency_key
    ON supply_settlements (user_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE INDEX IF NOT EXISTS idx_supply_settlements_request_id
    ON supply_settlements (request_id)
    WHERE request_id <> '';
