-- Migration 005: Audit Logs - DOWN Migration
-- Reverses 005_audit_logs.sql

-- =============================================================================
-- REMOVE TIMESCALEDB POLICIES
-- =============================================================================
SELECT remove_retention_policy('audit_logs', if_exists => TRUE);
SELECT remove_compression_policy('audit_logs', if_exists => TRUE);

-- =============================================================================
-- DROP INDEXES
-- =============================================================================
DROP INDEX IF EXISTS idx_audit_logs_ip_timestamp;
DROP INDEX IF EXISTS idx_audit_logs_user_timestamp;
DROP INDEX IF EXISTS idx_audit_logs_success;
DROP INDEX IF EXISTS idx_audit_logs_severity;
DROP INDEX IF EXISTS idx_audit_logs_resource;
DROP INDEX IF EXISTS idx_audit_logs_ip_address;
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_event_type;
DROP INDEX IF EXISTS idx_audit_logs_timestamp;

-- =============================================================================
-- DROP TABLE
-- =============================================================================
DROP TABLE IF EXISTS audit_logs;

SELECT 'Audit logs table dropped successfully!' AS status;
