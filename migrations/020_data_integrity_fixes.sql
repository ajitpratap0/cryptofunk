-- Migration 020: data-integrity fixes from 2026-03-25 audit
-- Fixes issues: #133 (DB-007 trades.session_id), #133 (DB-009 positions.session_id NOT NULL)

-- DB-007: Add session_id column to trades table so P&L can be queried directly per session.
-- The column is nullable because existing rows have no session context and the MCP
-- order-executor does not yet pass a session_id when inserting trades.
-- A follow-up code change in the order-executor will begin populating this column.
ALTER TABLE trades ADD COLUMN IF NOT EXISTS session_id UUID REFERENCES trading_sessions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_trades_session ON trades (session_id, executed_at DESC);

-- DB-009: Enforce NOT NULL on positions.session_id.
-- First backfill any NULLs by joining to the orders table via position_id FK.
-- This recovers session context for positions that were created before Issue #128 was fixed.
UPDATE positions p
SET session_id = (
    SELECT o.session_id
    FROM orders o
    WHERE o.position_id = p.id
      AND o.session_id IS NOT NULL
    LIMIT 1
)
WHERE p.session_id IS NULL;

-- Delete positions that are still NULL after backfill (orphaned rows with no traceable session).
-- This is safe because these rows have no associated session and cannot appear in any dashboard.
DELETE FROM positions WHERE session_id IS NULL;

-- Now apply the NOT NULL constraint.
ALTER TABLE positions ALTER COLUMN session_id SET NOT NULL;
