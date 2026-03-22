-- Index to support efficient per-symbol open position lookup within a session.
-- Used by GetOpenPositionBySymbolTx on the critical paper trade path (FOR UPDATE).
-- CONCURRENTLY is omitted: migration runners execute inside a transaction block,
-- which is incompatible with CREATE INDEX CONCURRENTLY. The index is small enough
-- that a standard build does not cause meaningful locking during migration.
CREATE INDEX IF NOT EXISTS idx_positions_session_symbol_open
    ON positions (session_id, symbol, exit_time)
    WHERE exit_time IS NULL;
