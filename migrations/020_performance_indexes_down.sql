-- Rollback: 020_performance_indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_positions_unrealized_pnl;
DROP INDEX CONCURRENTLY IF EXISTS idx_trading_sessions_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_orders_session_id_created;

-- Restore original index
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_orders_session
    ON orders(session_id, placed_at DESC);
