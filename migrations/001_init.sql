-- 001_init.sql
-- Run this migration to set up the database schema.

-- Workspaces table
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    plan VARCHAR(50) NOT NULL DEFAULT 'free',
    stripe_sub_id VARCHAR(255),
    trace_used BIGINT NOT NULL DEFAULT 0,
    trace_limit BIGINT NOT NULL DEFAULT 10000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- API keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

-- Traces table
CREATE TABLE IF NOT EXISTS traces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    trace_id VARCHAR(255) NOT NULL, -- client-supplied idempotency key
    model VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    prompt_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    cost_usd NUMERIC(12, 8) NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'success',
    status_code INT,
    error_msg TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_traces_workspace_created
    ON traces(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_traces_model
    ON traces(workspace_id, model);
CREATE INDEX IF NOT EXISTS idx_traces_status
    ON traces(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_api_keys_workspace
    ON api_keys(workspace_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash
    ON api_keys(key_hash) WHERE revoked_at IS NULL;

-- Insert a default free workspace for local development
INSERT INTO workspaces (id, name, plan, trace_limit)
VALUES ('00000000-0000-0000-0000-000000000001', 'Development', 'free', 10000)
ON CONFLICT (id) DO NOTHING;
