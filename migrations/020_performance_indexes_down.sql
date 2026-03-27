-- Rollback: 020_performance_indexes
-- NOTE: CONCURRENTLY is omitted because the migration runner executes inside a
-- transaction (BEGIN/COMMIT), which is incompatible with DROP/CREATE INDEX CONCURRENTLY.
DROP INDEX IF EXISTS idx_positions_unrealized_pnl;
DROP INDEX IF EXISTS idx_trading_sessions_status;
DROP INDEX IF EXISTS idx_orders_session_id_created;

-- Restore original index
CREATE INDEX IF NOT EXISTS idx_orders_session
    ON orders(session_id, placed_at DESC);
