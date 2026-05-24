-- =============================================================
-- XynPOS PostgreSQL 18 Initialization Script
-- Runs once when the container is first started
-- =============================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";
CREATE EXTENSION IF NOT EXISTS "unaccent";

-- Create global schema for cross-tenant tables
-- (users, tenants, subscriptions, invoices, refresh_tokens, otps)
CREATE SCHEMA IF NOT EXISTS public_xyn;

-- Set default search path
ALTER DATABASE xynpos SET search_path TO public, public_xyn;

-- Create a function to generate new tenant schemas
-- Called by the application when a new tenant registers
CREATE OR REPLACE FUNCTION create_tenant_schema(tenant_uuid TEXT)
RETURNS VOID AS $$
DECLARE
    schema_name TEXT;
BEGIN
    -- Validate UUID format and build schema name
    schema_name := 'tenant_' || REPLACE(tenant_uuid::TEXT, '-', '');
    
    -- Create the schema if it doesn't exist
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);
    
    -- Grant access to the application user
    EXECUTE format('GRANT ALL ON SCHEMA %I TO xynpos', schema_name);
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL ON TABLES TO xynpos', schema_name);
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL ON SEQUENCES TO xynpos', schema_name);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Grant execution to app user
GRANT EXECUTE ON FUNCTION create_tenant_schema(TEXT) TO xynpos;

-- Useful view: list all tenant schemas
CREATE OR REPLACE VIEW public_xyn.v_tenant_schemas AS
SELECT 
    schema_name,
    REPLACE(schema_name, 'tenant_', '') AS tenant_uuid_no_dashes
FROM information_schema.schemata
WHERE schema_name LIKE 'tenant_%'
ORDER BY schema_name;

GRANT SELECT ON public_xyn.v_tenant_schemas TO xynpos;

-- Log
DO $$
BEGIN
    RAISE NOTICE 'XynPOS database initialized successfully (PostgreSQL %)' , version();
END $$;
