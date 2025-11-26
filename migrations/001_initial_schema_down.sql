-- CryptoFunk Initial Database Schema - DOWN Migration
-- Reverses 001_initial_schema.sql

-- =============================================================================
-- DROP VIEWS
-- =============================================================================
DROP VIEW IF EXISTS daily_performance;
DROP VIEW IF EXISTS open_orders;
DROP VIEW IF EXISTS active_positions;

-- =============================================================================
-- DROP TRIGGERS
-- =============================================================================
DROP TRIGGER IF EXISTS update_agent_status_updated_at ON agent_status;
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
DROP TRIGGER IF EXISTS update_positions_updated_at ON positions;
DROP TRIGGER IF EXISTS update_trading_sessions_updated_at ON trading_sessions;

-- =============================================================================
-- DROP FUNCTIONS
-- =============================================================================
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS cleanup_market_data_cache();

-- =============================================================================
-- DROP TABLES (in dependency order)
-- =============================================================================

-- Remove compression and retention policies first (TimescaleDB)
SELECT remove_compression_policy('performance_metrics', if_exists => TRUE);
SELECT remove_compression_policy('candlesticks', if_exists => TRUE);

DROP TABLE IF EXISTS schema_version;
DROP TABLE IF EXISTS market_data_cache;
DROP TABLE IF EXISTS performance_metrics;
DROP TABLE IF EXISTS llm_decisions;
DROP TABLE IF EXISTS agent_status;
DROP TABLE IF EXISTS agent_signals;
DROP TABLE IF EXISTS trades;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS trading_sessions;
DROP TABLE IF EXISTS candlesticks;

-- =============================================================================
-- DROP ENUMS
-- =============================================================================
DROP TYPE IF EXISTS agent_state_type;
DROP TYPE IF EXISTS trading_mode;
DROP TYPE IF EXISTS signal_type;
DROP TYPE IF EXISTS position_side;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_type;
DROP TYPE IF EXISTS order_side;

-- =============================================================================
-- NOTE: Extensions are NOT dropped to avoid affecting other databases/schemas
-- If you need to drop extensions, uncomment the following:
-- =============================================================================
-- DROP EXTENSION IF EXISTS "uuid-ossp";
-- DROP EXTENSION IF EXISTS vector;
-- DROP EXTENSION IF EXISTS timescaledb;

SELECT 'Schema dropped successfully!' AS status;
