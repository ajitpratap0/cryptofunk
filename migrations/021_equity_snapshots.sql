-- migrations/021_equity_snapshots.sql
-- Equity snapshots: one row per paper trade, recording account equity at that point.
-- Used to render equity curve charts and compute max_drawdown / Sharpe ratio.

CREATE TABLE equity_snapshots (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id     UUID NOT NULL REFERENCES trading_sessions(id) ON DELETE CASCADE,
    equity         DOUBLE PRECISION NOT NULL,
    realized_pnl   DOUBLE PRECISION NOT NULL DEFAULT 0,
    unrealized_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    recorded_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Composite index for efficient per-session time-series queries and the
-- GetSessionIDWithMostSnapshots aggregation (session_id prefix is used by both).
CREATE INDEX idx_equity_snapshots_session_time
    ON equity_snapshots (session_id, recorded_at DESC);
