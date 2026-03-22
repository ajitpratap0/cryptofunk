-- This migration supersedes 018_positions_symbol_index.sql which added a non-unique
-- partial index. The two are kept as separate migrations (rather than collapsing into
-- one) to safely handle environments where 018 was already applied — the DROP here
-- is idempotent (IF EXISTS) and the CREATE is idempotent (IF NOT EXISTS).
-- Replace the non-unique partial index from migration 018 with a UNIQUE partial index.
-- FOR UPDATE only locks existing rows; without a unique constraint two concurrent
-- transactions can each observe existingPos == nil and INSERT duplicate positions for
-- the same (session_id, symbol). The unique constraint ensures the second concurrent
-- INSERT fails with 23505, which causes the enclosing transaction to roll back instead
-- of silently creating a duplicate open position.
-- CONCURRENTLY is omitted: migration runners execute inside a transaction block.
DROP INDEX IF EXISTS idx_positions_session_symbol_open;
CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_open_session_symbol_uniq
    ON positions (session_id, symbol)
    WHERE exit_time IS NULL;
