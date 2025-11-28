-- Migration: Orchestrator State Table
-- Description: Stores the orchestrator's state including pause/resume status for persistence
-- Author: T293 - Wire API Pause Trading to Orchestrator
-- Date: 2025-01-27

-- Create orchestrator_state table
CREATE TABLE IF NOT EXISTS orchestrator_state (
    id SERIAL PRIMARY KEY,
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    paused_at TIMESTAMPTZ,
    resumed_at TIMESTAMPTZ,
    paused_by VARCHAR(255), -- Who initiated the pause (e.g., "api", "circuit_breaker", "manual")
    pause_reason TEXT, -- Why trading was paused
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert initial state (not paused)
INSERT INTO orchestrator_state (paused, updated_at, created_at)
VALUES (FALSE, NOW(), NOW());

-- Create index on updated_at for efficient queries
CREATE INDEX idx_orchestrator_state_updated_at ON orchestrator_state(updated_at DESC);

-- Add comment to table
COMMENT ON TABLE orchestrator_state IS 'Stores orchestrator pause/resume state for persistence across restarts';
COMMENT ON COLUMN orchestrator_state.paused IS 'Whether trading is currently paused';
COMMENT ON COLUMN orchestrator_state.paused_at IS 'Timestamp when trading was paused';
COMMENT ON COLUMN orchestrator_state.resumed_at IS 'Timestamp when trading was resumed';
COMMENT ON COLUMN orchestrator_state.paused_by IS 'Source of the pause command (api, circuit_breaker, manual)';
COMMENT ON COLUMN orchestrator_state.pause_reason IS 'Reason for pausing trading';
