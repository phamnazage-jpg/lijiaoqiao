-- Migrate supply_idempotency_records from partitioned table to plain unique table.
-- This migration is intended for environments that previously applied partition_strategy_v1.sql
-- with supply_idempotency_records partitioned by expires_at.

BEGIN;

LOCK TABLE supply_idempotency_records IN ACCESS EXCLUSIVE MODE;

CREATE TABLE IF NOT EXISTS supply_idempotency_records_v2 (
    id                  BIGSERIAL PRIMARY KEY,
    tenant_id           BIGINT NOT NULL,
    operator_id         BIGINT NOT NULL,
    api_path            VARCHAR(200) NOT NULL,
    idempotency_key     VARCHAR(128) NOT NULL,
    request_id          VARCHAR(64) NOT NULL,
    payload_hash        CHAR(64) NOT NULL,
    response_code       INT,
    response_body       JSONB,
    status              VARCHAR(20) NOT NULL DEFAULT 'processing',
    expires_at          TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_supply_idempotency_records_v2_key
        UNIQUE (tenant_id, operator_id, api_path, idempotency_key)
);

INSERT INTO supply_idempotency_records_v2 (
    tenant_id,
    operator_id,
    api_path,
    idempotency_key,
    request_id,
    payload_hash,
    response_code,
    response_body,
    status,
    expires_at,
    created_at,
    updated_at
)
SELECT DISTINCT ON (tenant_id, operator_id, api_path, idempotency_key)
    tenant_id,
    operator_id,
    api_path,
    idempotency_key,
    request_id,
    payload_hash,
    response_code,
    response_body,
    status,
    expires_at,
    created_at,
    updated_at
FROM supply_idempotency_records
ORDER BY
    tenant_id,
    operator_id,
    api_path,
    idempotency_key,
    CASE WHEN expires_at > CURRENT_TIMESTAMP THEN 0 ELSE 1 END,
    expires_at DESC,
    updated_at DESC,
    id DESC;

ALTER TABLE supply_idempotency_records RENAME TO supply_idempotency_records_partitioned_legacy;
ALTER TABLE supply_idempotency_records_v2 RENAME TO supply_idempotency_records;

CREATE INDEX IF NOT EXISTS idx_idempotency_request_id
    ON supply_idempotency_records (request_id);
CREATE INDEX IF NOT EXISTS idx_idempotency_expires_at
    ON supply_idempotency_records (expires_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_status_expires
    ON supply_idempotency_records (status, expires_at);

COMMENT ON TABLE supply_idempotency_records IS '幂等记录表 - 非分区唯一表，保留7天以上';

COMMIT;
