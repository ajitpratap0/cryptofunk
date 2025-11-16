-- Migration: 005_audit_logs.sql
-- Description: Create audit logging table for security and compliance
-- Created: 2025-11-16

-- Create audit_logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    user_id VARCHAR(255),
    ip_address VARCHAR(45) NOT NULL,  -- Support both IPv4 and IPv6
    user_agent TEXT,
    resource VARCHAR(255),  -- ID of affected resource (order ID, session ID, etc.)
    action TEXT NOT NULL,  -- Human-readable action description
    success BOOLEAN NOT NULL DEFAULT TRUE,
    error_message TEXT,
    metadata JSONB,  -- Additional context
    request_id VARCHAR(255),  -- For request correlation
    duration_ms BIGINT,  -- Action duration in milliseconds

    -- Indexes for common query patterns
    CONSTRAINT audit_logs_event_type_check CHECK (event_type IN (
        'LOGIN', 'LOGOUT', 'LOGIN_FAILED', 'PASSWORD_CHANGE',
        'TRADING_START', 'TRADING_STOP', 'TRADING_PAUSE', 'TRADING_RESUME',
        'ORDER_PLACED', 'ORDER_CANCELED', 'ORDER_FILLED',
        'CONFIG_UPDATED', 'CONFIG_VIEWED',
        'AGENT_STARTED', 'AGENT_STOPPED', 'AGENT_FAILED',
        'RATE_LIMIT_EXCEEDED', 'UNAUTHORIZED_ACCESS', 'INVALID_INPUT',
        'DATA_EXPORT', 'DATA_DELETE'
    )),
    CONSTRAINT audit_logs_severity_check CHECK (severity IN (
        'INFO', 'WARNING', 'ERROR', 'CRITICAL'
    ))
);

-- Create indexes for efficient querying
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_logs_ip_address ON audit_logs(ip_address);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource) WHERE resource IS NOT NULL;
CREATE INDEX idx_audit_logs_severity ON audit_logs(severity);
CREATE INDEX idx_audit_logs_success ON audit_logs(success);

-- Composite index for common query patterns
CREATE INDEX idx_audit_logs_user_timestamp ON audit_logs(user_id, timestamp DESC) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_logs_ip_timestamp ON audit_logs(ip_address, timestamp DESC);

-- Convert to TimescaleDB hypertable for efficient time-series storage
-- Partition by time with 7-day chunks
SELECT create_hypertable('audit_logs', 'timestamp',
    chunk_time_interval => INTERVAL '7 days',
    if_not_exists => TRUE
);

-- Enable compression for older audit logs (compress data older than 30 days)
ALTER TABLE audit_logs SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'timestamp DESC',
    timescaledb.compress_segmentby = 'event_type, severity'
);

SELECT add_compression_policy('audit_logs', INTERVAL '30 days', if_not_exists => TRUE);

-- Create retention policy: keep audit logs for 1 year
SELECT add_retention_policy('audit_logs', INTERVAL '365 days', if_not_exists => TRUE);

-- Add comment to table
COMMENT ON TABLE audit_logs IS 'Audit log table for security and compliance tracking. Uses TimescaleDB for efficient time-series storage with automatic compression and retention.';

-- Add comments to columns
COMMENT ON COLUMN audit_logs.id IS 'Unique identifier for the audit event';
COMMENT ON COLUMN audit_logs.timestamp IS 'When the event occurred';
COMMENT ON COLUMN audit_logs.event_type IS 'Type of event (LOGIN, ORDER_PLACED, etc.)';
COMMENT ON COLUMN audit_logs.severity IS 'Severity level (INFO, WARNING, ERROR, CRITICAL)';
COMMENT ON COLUMN audit_logs.user_id IS 'User or API key that performed the action';
COMMENT ON COLUMN audit_logs.ip_address IS 'Client IP address';
COMMENT ON COLUMN audit_logs.user_agent IS 'Browser or client user agent string';
COMMENT ON COLUMN audit_logs.resource IS 'Affected resource identifier (order ID, session ID, etc.)';
COMMENT ON COLUMN audit_logs.action IS 'Human-readable description of what happened';
COMMENT ON COLUMN audit_logs.success IS 'Whether the action succeeded';
COMMENT ON COLUMN audit_logs.error_message IS 'Error message if action failed';
COMMENT ON COLUMN audit_logs.metadata IS 'Additional context as JSON';
COMMENT ON COLUMN audit_logs.request_id IS 'Request correlation ID for tracing';
COMMENT ON COLUMN audit_logs.duration_ms IS 'Action duration in milliseconds';
