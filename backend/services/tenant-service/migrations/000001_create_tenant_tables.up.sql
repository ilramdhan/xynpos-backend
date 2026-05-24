-- =============================================================
-- Tenant Service — Tenant Schema Migrations
-- =============================================================

-- Tenants table (global schema: public_xyn)
CREATE TABLE IF NOT EXISTS tenants (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(200) NOT NULL,
    slug          VARCHAR(100) NOT NULL,
    business_type VARCHAR(50)  NOT NULL,
    owner_id      UUID NOT NULL,
    plan          VARCHAR(50)  NOT NULL DEFAULT 'starter',
    logo_url      TEXT,
    website       VARCHAR(500),
    address       TEXT,
    city          VARCHAR(100),
    province      VARCHAR(100),
    country       CHAR(2)  NOT NULL DEFAULT 'ID',
    currency      CHAR(3)  NOT NULL DEFAULT 'IDR',
    timezone      VARCHAR(50) NOT NULL DEFAULT 'Asia/Jakarta',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    trial_ends_at TIMESTAMPTZ,
    schema_name   VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT tenants_slug_key UNIQUE (slug),
    CONSTRAINT tenants_schema_key UNIQUE (schema_name),
    CONSTRAINT tenants_business_type_check CHECK (
        business_type IN ('retail', 'fnb', 'service', 'cafe', 'restaurant', 'general')
    )
);

CREATE INDEX IF NOT EXISTS idx_tenants_owner_id ON tenants (owner_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants (slug) WHERE deleted_at IS NULL;

-- Roles table (global schema)
CREATE TABLE IF NOT EXISTS roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID,                   -- NULL = system role
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(100) NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]',
    is_system   BOOLEAN NOT NULL DEFAULT false,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_roles_tenant_id ON roles (tenant_id);

-- Seed system roles
INSERT INTO roles (id, name, slug, description, permissions, is_system) VALUES
    (gen_random_uuid(), 'Owner',     'owner',     'Full access to all features',       '["*"]',                                                 true),
    (gen_random_uuid(), 'Manager',   'manager',   'Manage products, inventory, reports','["product:*","inventory:*","report:read","transaction:*","customer:*","user:read"]', true),
    (gen_random_uuid(), 'Cashier',   'cashier',   'Process transactions',               '["product:read","inventory:read","transaction:*","customer:read"]', true),
    (gen_random_uuid(), 'Inventory', 'inventory', 'Manage inventory only',             '["product:read","inventory:*"]',                         true)
ON CONFLICT DO NOTHING;

-- Tenant users (maps user ↔ tenant with role)
CREATE TABLE IF NOT EXISTS tenant_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    user_id     UUID NOT NULL,
    role_id     UUID NOT NULL REFERENCES roles(id),
    outlet_id   UUID,                   -- NULL = all outlets
    is_active   BOOLEAN NOT NULL DEFAULT true,
    invited_by  UUID,
    joined_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT tenant_users_unique UNIQUE (tenant_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_users_tenant_id ON tenant_users (tenant_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_tenant_users_user_id ON tenant_users (user_id) WHERE is_active = true;

-- Invitations
CREATE TABLE IF NOT EXISTS invitations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    email       VARCHAR(320) NOT NULL,
    role_id     UUID NOT NULL REFERENCES roles(id),
    outlet_id   UUID,
    invited_by  UUID NOT NULL,
    token       VARCHAR(255) NOT NULL,
    is_accepted BOOLEAN NOT NULL DEFAULT false,
    expires_at  TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT invitations_token_key UNIQUE (token)
);

CREATE INDEX IF NOT EXISTS idx_invitations_email ON invitations (email) WHERE is_accepted = false;
CREATE INDEX IF NOT EXISTS idx_invitations_tenant_id ON invitations (tenant_id);

-- Updated_at trigger for tenants
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tenant_users_updated_at
    BEFORE UPDATE ON tenant_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
