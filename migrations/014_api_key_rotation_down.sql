-- Down migration: Remove API Key Rotation Support (TB-006)

-- Drop functions
DROP FUNCTION IF EXISTS get_key_rotation_history(UUID);
DROP FUNCTION IF EXISTS deactivate_expired_keys();
DROP FUNCTION IF EXISTS rotate_api_key(UUID, TIMESTAMPTZ);

-- Drop indexes
DROP INDEX IF EXISTS idx_api_keys_is_active;
DROP INDEX IF EXISTS idx_api_keys_rotated_from;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_audit_logs_key_events;

-- Remove columns
ALTER TABLE api_keys DROP COLUMN IF EXISTS is_active;
ALTER TABLE api_keys DROP COLUMN IF EXISTS rotated_from;

-- Restore previous audit_logs constraint (from migration 009)
DO $$
BEGIN
    ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_event_type_check;
EXCEPTION
    WHEN undefined_object THEN
        NULL;
END $$;

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
        'DECISION_LIST_ACCESSED', 'DECISION_VIEWED', 'DECISION_SEARCHED', 'DECISION_STATS_ACCESSED', 'DECISION_SIMILAR_ACCESSED'
    )
);
