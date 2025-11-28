-- Migration: Backtest Jobs
-- Description: Creates tables for storing backtest job configurations and results
-- Version: 011
-- Created: 2025-11-28

-- Create backtest_jobs table for storing backtest configurations and status
CREATE TABLE IF NOT EXISTS backtest_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, running, completed, failed

    -- Configuration
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    symbols TEXT[] NOT NULL, -- Array of trading symbols
    initial_capital DECIMAL(20, 8) NOT NULL,
    strategy_config JSONB NOT NULL, -- Strategy configuration from T310 format
    parameter_grid JSONB, -- Optional: Parameter grid for optimization

    -- Results (populated after completion)
    results JSONB, -- Complete backtest results including metrics, equity curve, trades

    -- Performance metrics (denormalized for quick querying)
    total_return_pct DECIMAL(10, 4),
    sharpe_ratio DECIMAL(10, 4),
    max_drawdown_pct DECIMAL(10, 4),
    win_rate DECIMAL(10, 4),
    total_trades INTEGER,

    -- Error information
    error_message TEXT,
    error_details TEXT,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- User tracking (for multi-user support)
    created_by VARCHAR(255),

    -- Constraints
    CONSTRAINT valid_date_range CHECK (end_date > start_date),
    CONSTRAINT valid_capital CHECK (initial_capital > 0),
    CONSTRAINT valid_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'))
);

-- Create index for querying by status
CREATE INDEX idx_backtest_jobs_status ON backtest_jobs(status);

-- Create index for querying by created_at (for pagination)
CREATE INDEX idx_backtest_jobs_created_at ON backtest_jobs(created_at DESC);

-- Create index for querying by user
CREATE INDEX idx_backtest_jobs_created_by ON backtest_jobs(created_by);

-- Create composite index for filtering completed jobs by performance
CREATE INDEX idx_backtest_jobs_performance ON backtest_jobs(status, sharpe_ratio DESC, total_return_pct DESC)
    WHERE status = 'completed';

-- Create GIN index for searching strategy_config JSON
CREATE INDEX idx_backtest_jobs_strategy_config ON backtest_jobs USING GIN (strategy_config);

-- Add comments for documentation
COMMENT ON TABLE backtest_jobs IS 'Stores backtest job configurations and results for strategy testing';
COMMENT ON COLUMN backtest_jobs.id IS 'Unique identifier for the backtest job';
COMMENT ON COLUMN backtest_jobs.name IS 'User-friendly name for the backtest';
COMMENT ON COLUMN backtest_jobs.status IS 'Current status: pending, running, completed, failed, cancelled';
COMMENT ON COLUMN backtest_jobs.strategy_config IS 'Strategy configuration in T310 format (JSONB)';
COMMENT ON COLUMN backtest_jobs.parameter_grid IS 'Optional parameter grid for optimization runs';
COMMENT ON COLUMN backtest_jobs.results IS 'Complete backtest results including equity curve and trade log';
COMMENT ON COLUMN backtest_jobs.total_return_pct IS 'Total return percentage (denormalized for quick querying)';
COMMENT ON COLUMN backtest_jobs.sharpe_ratio IS 'Sharpe ratio (denormalized for quick querying)';
COMMENT ON COLUMN backtest_jobs.max_drawdown_pct IS 'Maximum drawdown percentage (denormalized for quick querying)';
COMMENT ON COLUMN backtest_jobs.win_rate IS 'Win rate percentage (denormalized for quick querying)';
COMMENT ON COLUMN backtest_jobs.total_trades IS 'Total number of trades executed (denormalized for quick querying)';

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_backtest_jobs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update updated_at
CREATE TRIGGER trigger_update_backtest_jobs_updated_at
    BEFORE UPDATE ON backtest_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_backtest_jobs_updated_at();
