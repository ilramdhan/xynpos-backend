-- =============================================================
-- Auth Service — Global Schema Migrations
-- Runs against public_xyn schema (cross-tenant tables)
-- =============================================================

-- Users table (global — shared across all tenants)
CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               VARCHAR(320) NOT NULL,
    phone               VARCHAR(20),
    password_hash       VARCHAR(255) NOT NULL,
    name                VARCHAR(200) NOT NULL,
    avatar_url          TEXT,
    google_id           VARCHAR(255),
    is_active           BOOLEAN NOT NULL DEFAULT true,
    email_verified      BOOLEAN NOT NULL DEFAULT false,
    email_verified_at   TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,

    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_phone_key UNIQUE (phone),
    CONSTRAINT users_google_id_key UNIQUE (google_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users (LOWER(email)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_phone ON users (phone) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- Refresh tokens table (global — all devices)
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL,
    token_hash      VARCHAR(64) NOT NULL,   -- SHA-256 hex
    token_family    UUID NOT NULL,          -- For reuse detection
    device_id       VARCHAR(255),
    device_name     VARCHAR(255),
    ip_address      INET,
    is_revoked      BOOLEAN NOT NULL DEFAULT false,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT refresh_tokens_hash_key UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id) WHERE is_revoked = false;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens (token_family);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);

-- OTP table (global)
CREATE TABLE IF NOT EXISTS otps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(50) NOT NULL,   -- email_verification | password_reset
    code        VARCHAR(10) NOT NULL,
    is_used     BOOLEAN NOT NULL DEFAULT false,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT otps_type_check CHECK (type IN ('email_verification', 'password_reset'))
);

CREATE INDEX IF NOT EXISTS idx_otps_user_id_type ON otps (user_id, type) WHERE is_used = false;
CREATE INDEX IF NOT EXISTS idx_otps_expires_at ON otps (expires_at);

-- Auto-update updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
