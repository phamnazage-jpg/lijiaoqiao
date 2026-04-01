-- Supply Idempotency Record Schema
-- Based on: XR-001 (supply_technical_design_enhanced_v1_2026-03-25.md)
-- Updated: 2026-03-27

BEGIN;

CREATE TABLE IF NOT EXISTS supply_idempotency_records (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    operator_id BIGINT NOT NULL,
    api_path VARCHAR(200) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    payload_hash CHAR(64) NOT NULL, -- SHA256 of request body
    response_code INT,
    response_body JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'processing'
        CHECK (status IN ('processing', 'succeeded', 'failed')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, operator_id, api_path, idempotency_key)
);

-- 高频查询索引
CREATE INDEX IF NOT EXISTS idx_idempotency_tenant_operator_path_key
    ON supply_idempotency_records (tenant_id, operator_id, api_path, idempotency_key)
    WHERE expires_at > CURRENT_TIMESTAMP;

-- RequestID 反查索引
CREATE INDEX IF NOT EXISTS idx_idempotency_request_id
    ON supply_idempotency_records (request_id);

-- 过期清理索引
CREATE INDEX IF NOT EXISTS idx_idempotency_expires_at
    ON supply_idempotency_records (expires_at)
    WHERE status != 'processing';

-- 状态查询索引
CREATE INDEX IF NOT EXISTS idx_idempotency_status_expires
    ON supply_idempotency_records (status, expires_at);

COMMENT ON TABLE supply_idempotency_records IS '幂等记录表 - XR-001';
COMMENT ON COLUMN supply_idempotency_records.payload_hash IS '请求体SHA256摘要，用于检测异参重放';
COMMENT ON COLUMN supply_idempotency_records.expires_at IS '过期时间，默认24小时，提现类72小时';

COMMIT;
