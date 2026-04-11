-- Token Status Registry v1.0
-- 存储Token吊销状态，支持主动失效机制（P0-03修复）
-- 设计目标：吊销传播延迟 <= 5s

-- Token状态枚举
DO $$ BEGIN
    CREATE TYPE token_status AS ENUM ('active', 'revoked', 'expired');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Token状态注册表
CREATE TABLE IF NOT EXISTS token_status_registry (
    id              BIGSERIAL PRIMARY KEY,
    token_id        VARCHAR(128) NOT NULL UNIQUE,
    subject_id      BIGINT NOT NULL,
    tenant_id       BIGINT NOT NULL,
    role            VARCHAR(50) NOT NULL DEFAULT 'user',
    status          token_status NOT NULL DEFAULT 'active',
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    revoked_reason  VARCHAR(256),
    revoked_by      BIGINT,
    last_verified_at TIMESTAMPTZ,
    verification_count BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 约束
    CONSTRAINT valid_token_status CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT valid_expiry CHECK (expires_at > issued_at)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_token_status_token_id ON token_status_registry(token_id);
CREATE INDEX IF NOT EXISTS idx_token_status_subject_id ON token_status_registry(subject_id);
CREATE INDEX IF NOT EXISTS idx_token_status_tenant_id ON token_status_registry(tenant_id);
CREATE INDEX IF NOT EXISTS idx_token_status_status ON token_status_registry(status);
CREATE INDEX IF NOT EXISTS idx_token_status_expires_at ON token_status_registry(expires_at) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_token_status_revoked_at ON token_status_registry(revoked_at) WHERE status = 'revoked';

-- 注释
COMMENT ON TABLE token_status_registry IS 'Token状态注册表，用于管理Token吊销状态';
COMMENT ON COLUMN token_status_registry.token_id IS 'Token唯一标识符（JWT jti claim）';
COMMENT ON COLUMN token_status_registry.subject_id IS 'Token所属用户ID';
COMMENT ON COLUMN token_status_registry.tenant_id IS '租户ID';
COMMENT ON COLUMN token_status_registry.status IS 'Token状态：active/revoked/expired';
COMMENT ON COLUMN token_status_registry.revoked_at IS '吊销时间';
COMMENT ON COLUMN token_status_registry.revoked_reason IS '吊销原因';
COMMENT ON COLUMN token_status_registry.last_verified_at IS '最后验证时间（用于活跃度追踪）';
COMMENT ON COLUMN token_status_registry.verification_count IS '验证次数统计';

-- 自动更新updated_at触发器
CREATE OR REPLACE FUNCTION update_token_status_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_token_status_updated_at ON token_status_registry;
CREATE TRIGGER trigger_token_status_updated_at
    BEFORE UPDATE ON token_status_registry
    FOR EACH ROW
    EXECUTE FUNCTION update_token_status_updated_at();
