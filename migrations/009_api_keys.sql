-- Migration: API Keys for Authentication
-- Provides API key-based authentication for the dashboard and API endpoints

-- Create api_keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 hash of the API key
    name VARCHAR(255) NOT NULL,            -- Human-readable name for the key
    user_id VARCHAR(255) NOT NULL,         -- Owner identifier (can be 'admin', 'system', or specific user)
    permissions JSONB DEFAULT '["read:decisions"]'::jsonb,  -- Array of permission strings
    last_used_at TIMESTAMPTZ,              -- Track when key was last used
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    expires_at TIMESTAMPTZ,                -- Optional expiration date
    revoked BOOLEAN DEFAULT FALSE NOT NULL,
    revoked_at TIMESTAMPTZ,                -- When the key was revoked
    metadata JSONB DEFAULT '{}'::jsonb     -- Additional metadata (rate limits, etc.)
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked ON api_keys(revoked) WHERE revoked = FALSE;
CREATE INDEX IF NOT EXISTS idx_api_keys_expires ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- Add comments
COMMENT ON TABLE api_keys IS 'API keys for authentication to dashboard and API endpoints';
COMMENT ON COLUMN api_keys.key_hash IS 'SHA-256 hash of the plaintext API key';
COMMENT ON COLUMN api_keys.permissions IS 'JSON array of permission strings (e.g., ["read:decisions", "write:feedback"])';
COMMENT ON COLUMN api_keys.metadata IS 'Additional key-specific settings like custom rate limits';

-- Function to create a new API key (returns the plaintext key only once)
-- Usage: SELECT * FROM create_api_key('My Key', 'admin', '["read:decisions", "write:feedback"]'::jsonb);
CREATE OR REPLACE FUNCTION create_api_key(
    p_name VARCHAR(255),
    p_user_id VARCHAR(255),
    p_permissions JSONB DEFAULT '["read:decisions"]'::jsonb,
    p_expires_at TIMESTAMPTZ DEFAULT NULL
) RETURNS TABLE (
    id UUID,
    api_key TEXT,  -- This is the ONLY time the plaintext key is available
    name VARCHAR(255),
    user_id VARCHAR(255),
    permissions JSONB,
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
) AS $$
DECLARE
    v_key TEXT;
    v_key_hash VARCHAR(64);
    v_id UUID;
    v_created_at TIMESTAMPTZ;
BEGIN
    -- Generate a secure random API key (32 bytes = 64 hex chars)
    v_key := encode(gen_random_bytes(32), 'hex');

    -- Hash the key for storage
    v_key_hash := encode(sha256(v_key::bytea), 'hex');

    -- Insert the key
    INSERT INTO api_keys (key_hash, name, user_id, permissions, expires_at)
    VALUES (v_key_hash, p_name, p_user_id, p_permissions, p_expires_at)
    RETURNING api_keys.id, api_keys.created_at INTO v_id, v_created_at;

    -- Return the key details including the plaintext key
    RETURN QUERY SELECT v_id, v_key, p_name, p_user_id, p_permissions, v_created_at, p_expires_at;
END;
$$ LANGUAGE plpgsql;

-- Function to revoke an API key
CREATE OR REPLACE FUNCTION revoke_api_key(p_id UUID) RETURNS BOOLEAN AS $$
DECLARE
    v_updated BOOLEAN;
BEGIN
    UPDATE api_keys
    SET revoked = TRUE, revoked_at = NOW()
    WHERE id = p_id AND revoked = FALSE;

    GET DIAGNOSTICS v_updated = ROW_COUNT;
    RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql;

-- Update audit_logs constraint to include new decision-related event types
-- First drop the existing constraint if it exists
DO $$
BEGIN
    -- Try to drop the constraint - it may not exist
    ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_event_type_check;
EXCEPTION
    WHEN undefined_object THEN
        NULL; -- Constraint doesn't exist, that's fine
END $$;

-- Add updated constraint with decision event types
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_event_type_check CHECK (
    event_type IN (
        'LOGIN', 'LOGOUT', 'LOGIN_FAILED', 'PASSWORD_CHANGE',
        'TRADING_START', 'TRADING_STOP', 'TRADING_PAUSE', 'TRADING_RESUME',
        'ORDER_PLACED', 'ORDER_CANCELED', 'ORDER_FILLED',
        'CONFIG_UPDATED', 'CONFIG_VIEWED',
        'STRATEGY_UPDATED', 'STRATEGY_IMPORTED', 'STRATEGY_EXPORTED', 'STRATEGY_CLONED', 'STRATEGY_MERGED',
        'AGENT_STARTED', 'AGENT_STOPPED', 'AGENT_FAILED',
        'RATE_LIMIT_EXCEEDED', 'UNAUTHORIZED_ACCESS', 'INVALID_INPUT',
        'DATA_EXPORT', 'DATA_DELETE',
        -- New decision-related event types
        'DECISION_LIST_ACCESSED', 'DECISION_VIEWED', 'DECISION_SEARCHED', 'DECISION_STATS_ACCESSED', 'DECISION_SIMILAR_ACCESSED'
    )
);

-- Add index for decision-related audit queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_decision_events
ON audit_logs(event_type, timestamp DESC)
WHERE event_type IN ('DECISION_LIST_ACCESSED', 'DECISION_VIEWED', 'DECISION_SEARCHED', 'DECISION_STATS_ACCESSED', 'DECISION_SIMILAR_ACCESSED');
