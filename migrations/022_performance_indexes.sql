-- Migration: 022_performance_indexes
-- Performance indexes from 2026-03-25 audit (issues #137, #139, #140)
--
-- 1. idx_positions_unrealized_pnl   – supports VaR queries that ORDER BY unrealized_pnl
-- 2. idx_trading_sessions_stopped_at – supports ListActiveSessions WHERE stopped_at IS NULL
-- 3. idx_orders_session_created     – replaces the placed_at-sorted index with one that
--    matches the actual ORDER BY created_at DESC used by GetOrdersBySession / ListOrders
--
-- NOTE: trading_sessions has no "status" column; active sessions are identified by
-- stopped_at IS NULL (see ListActiveSessionsPaginated in internal/db/sessions.go).

-- Index for VaR / risk calculations that scan open positions ordered by unrealized_pnl
-- NOTE: CONCURRENTLY is omitted because the migration runner executes inside a
-- transaction (BEGIN/COMMIT), which is incompatible with CREATE INDEX CONCURRENTLY.
CREATE INDEX IF NOT EXISTS idx_positions_unrealized_pnl
    ON positions(unrealized_pnl);

-- Index for ListActiveSessions (WHERE stopped_at IS NULL ORDER BY started_at DESC).
-- A partial index covering only the open-session rows keeps it small and highly selective.
CREATE INDEX IF NOT EXISTS idx_trading_sessions_stopped_at
    ON trading_sessions(started_at DESC)
    WHERE stopped_at IS NULL;

-- Drop the wrong-sort-key orders index (placed_at) and replace with created_at
-- which matches the ORDER BY used in GetOrdersBySession and related queries.
DROP INDEX IF EXISTS idx_orders_session;
CREATE INDEX IF NOT EXISTS idx_orders_session_id_created
    ON orders(session_id, created_at DESC);
