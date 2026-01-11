-- Migration: API Key Rotation Support (TB-006)
-- Adds support for API key lifecycle management including rotation and active status

-- Add rotated_from column to track key rotation chain
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS rotated_from UUID REFERENCES api_keys(id);

-- Add is_active column to distinguish active keys from rotated/deactivated ones
-- Note: This is different from 'revoked' - a key can be inactive due to rotation
-- but not explicitly revoked for security reasons
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE NOT NULL;

-- Add index for efficient expiration cleanup queries
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at)
WHERE expires_at IS NOT NULL AND is_active = TRUE;

-- Add index for key rotation chain lookups
CREATE INDEX IF NOT EXISTS idx_api_keys_rotated_from ON api_keys(rotated_from)
WHERE rotated_from IS NOT NULL;

-- Add index for active keys lookup
CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active)
WHERE is_active = TRUE;

-- Comments
COMMENT ON COLUMN api_keys.rotated_from IS 'Reference to the previous key that this key was rotated from';
COMMENT ON COLUMN api_keys.is_active IS 'Whether the key is currently active (false = rotated out or deactivated)';

-- Function to rotate an API key (creates new key, deactivates old one)
-- Returns the new key details including the plaintext key (shown only once)
CREATE OR REPLACE FUNCTION rotate_api_key(
    p_old_key_id UUID,
    p_new_expires_at TIMESTAMPTZ DEFAULT NULL
) RETURNS TABLE (
    id UUID,
    api_key TEXT,  -- This is the ONLY time the plaintext key is available
    name VARCHAR(255),
    user_id VARCHAR(255),
    permissions JSONB,
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    rotated_from UUID
) AS $$
DECLARE
    v_old_key api_keys%ROWTYPE;
    v_new_key TEXT;
    v_key_hash VARCHAR(64);
    v_new_id UUID;
    v_created_at TIMESTAMPTZ;
    v_new_name VARCHAR(255);
BEGIN
    -- Get the old key details
    SELECT * INTO v_old_key FROM api_keys WHERE api_keys.id = p_old_key_id;

    IF v_old_key.id IS NULL THEN
        RAISE EXCEPTION 'API key not found: %', p_old_key_id;
    END IF;

    IF v_old_key.revoked = TRUE THEN
        RAISE EXCEPTION 'Cannot rotate a revoked key: %', p_old_key_id;
    END IF;

    IF v_old_key.is_active = FALSE THEN
        RAISE EXCEPTION 'Cannot rotate an inactive key: %', p_old_key_id;
    END IF;

    -- Validate new expiration date is in the future if provided
    IF p_new_expires_at IS NOT NULL AND p_new_expires_at <= NOW() THEN
        RAISE EXCEPTION 'New expiration date must be in the future';
    END IF;

    -- Generate a secure random API key (32 bytes = 64 hex chars)
    v_new_key := encode(gen_random_bytes(32), 'hex');

    -- Hash the key for storage
    v_key_hash := encode(sha256(v_new_key::bytea), 'hex');

    -- Create new key name with rotation indicator
    v_new_name := v_old_key.name || ' (rotated)';

    -- Deactivate the old key
    UPDATE api_keys
    SET is_active = FALSE
    WHERE api_keys.id = p_old_key_id;

    -- Create new key with reference to old key
    INSERT INTO api_keys (
        key_hash,
        name,
        user_id,
        permissions,
        expires_at,
        rotated_from,
        is_active
    )
    VALUES (
        v_key_hash,
        v_new_name,
        v_old_key.user_id,
        v_old_key.permissions,
        COALESCE(p_new_expires_at, v_old_key.expires_at),
        p_old_key_id,
        TRUE
    )
    RETURNING api_keys.id, api_keys.created_at INTO v_new_id, v_created_at;

    -- Return the new key details
    RETURN QUERY SELECT
        v_new_id,
        v_new_key,
        v_new_name,
        v_old_key.user_id,
        v_old_key.permissions,
        v_created_at,
        COALESCE(p_new_expires_at, v_old_key.expires_at),
        p_old_key_id;
END;
$$ LANGUAGE plpgsql;

-- Function to deactivate expired keys (for background cleanup job)
-- Returns the number of keys deactivated
CREATE OR REPLACE FUNCTION deactivate_expired_keys() RETURNS INTEGER AS $$
DECLARE
    v_count INTEGER;
BEGIN
    UPDATE api_keys
    SET is_active = FALSE
    WHERE expires_at IS NOT NULL
      AND expires_at < NOW()
      AND is_active = TRUE
      AND revoked = FALSE;

    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

-- Function to get key rotation history
CREATE OR REPLACE FUNCTION get_key_rotation_history(p_key_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR(255),
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN,
    revoked BOOLEAN,
    rotated_from UUID
) AS $$
WITH RECURSIVE key_chain AS (
    -- Start with the given key
    SELECT
        k.id, k.name, k.created_at, k.expires_at,
        k.is_active, k.revoked, k.rotated_from,
        1 as depth
    FROM api_keys k
    WHERE k.id = p_key_id

    UNION ALL

    -- Find parent keys (what this was rotated from)
    SELECT
        k.id, k.name, k.created_at, k.expires_at,
        k.is_active, k.revoked, k.rotated_from,
        kc.depth + 1
    FROM api_keys k
    JOIN key_chain kc ON k.id = kc.rotated_from
    WHERE kc.depth < 10  -- Limit recursion depth: 10 rotations covers ~10 months of monthly rotations
)
SELECT
    key_chain.id, key_chain.name, key_chain.created_at, key_chain.expires_at,
    key_chain.is_active, key_chain.revoked, key_chain.rotated_from
FROM key_chain
ORDER BY key_chain.depth ASC;
$$ LANGUAGE sql;

-- Update audit_logs constraint to include key management events
DO $$
BEGIN
    -- Try to drop the existing constraint
    ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_event_type_check;
EXCEPTION
    WHEN undefined_object THEN
        NULL;
END $$;

-- Add updated constraint with key management event types
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
        'DECISION_LIST_ACCESSED', 'DECISION_VIEWED', 'DECISION_SEARCHED', 'DECISION_STATS_ACCESSED', 'DECISION_SIMILAR_ACCESSED',
        -- New API key management event types (TB-006)
        'API_KEY_CREATED', 'API_KEY_ROTATED', 'API_KEY_REVOKED', 'API_KEY_EXPIRED', 'API_KEY_CLEANUP'
    )
);

-- Add index for key management audit queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_key_events
ON audit_logs(event_type, timestamp DESC)
WHERE event_type IN ('API_KEY_CREATED', 'API_KEY_ROTATED', 'API_KEY_REVOKED', 'API_KEY_EXPIRED', 'API_KEY_CLEANUP');
