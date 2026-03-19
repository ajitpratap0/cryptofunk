-- Index to support efficient per-symbol open position lookup within a session.
-- Used by GetOpenPositionBySymbolTx on the critical paper trade path (FOR UPDATE).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_positions_session_symbol_open
    ON positions (session_id, symbol, exit_time)
    WHERE exit_time IS NULL;
