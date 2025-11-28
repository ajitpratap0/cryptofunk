-- Migration 009 Down: Remove API Keys and Restore Audit Logs Constraint
-- Reverses 009_api_keys.sql

-- =============================================================================
-- DROP INDEXES
-- =============================================================================
DROP INDEX IF EXISTS idx_audit_logs_decision_events;
DROP INDEX IF EXISTS idx_api_keys_expires;
DROP INDEX IF EXISTS idx_api_keys_revoked;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_key_hash;

-- =============================================================================
-- DROP FUNCTIONS
-- =============================================================================
DROP FUNCTION IF EXISTS revoke_api_key(UUID);
DROP FUNCTION IF EXISTS create_api_key(VARCHAR(255), VARCHAR(255), JSONB, TIMESTAMPTZ);

-- =============================================================================
-- DROP TABLE
-- =============================================================================
DROP TABLE IF EXISTS api_keys;

-- =============================================================================
-- RESTORE ORIGINAL AUDIT_LOGS CONSTRAINT
-- =============================================================================
-- Migration 009 expanded the audit_logs_event_type_check constraint to include
-- decision-related event types. Rolling back to the original constraint from 005.

-- Drop the expanded constraint added in migration 009
DO $$
BEGIN
    ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_event_type_check;
EXCEPTION
    WHEN undefined_object THEN
        NULL; -- Constraint doesn't exist, that's fine
END $$;

-- Restore the original constraint from migration 005
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_event_type_check CHECK (
    event_type IN (
        'LOGIN', 'LOGOUT', 'LOGIN_FAILED', 'PASSWORD_CHANGE',
        'TRADING_START', 'TRADING_STOP', 'TRADING_PAUSE', 'TRADING_RESUME',
        'ORDER_PLACED', 'ORDER_CANCELED', 'ORDER_FILLED',
        'CONFIG_UPDATED', 'CONFIG_VIEWED',
        'AGENT_STARTED', 'AGENT_STOPPED', 'AGENT_FAILED',
        'RATE_LIMIT_EXCEEDED', 'UNAUTHORIZED_ACCESS', 'INVALID_INPUT',
        'DATA_EXPORT', 'DATA_DELETE'
    )
);

SELECT 'Migration 009 down: API keys removed and audit_logs constraint restored!' AS status;
