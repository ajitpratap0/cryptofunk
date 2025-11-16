-- =============================================================================
-- Migration 004: Enhance LLM Decisions Table
-- =============================================================================
-- This migration adds missing columns to llm_decisions table that are used
-- by the application code but weren't in the initial schema.

-- Add agent_name column to track which agent made the decision
ALTER TABLE llm_decisions
ADD COLUMN IF NOT EXISTS agent_name VARCHAR(100) NOT NULL DEFAULT 'unknown';

-- Add confidence column to track decision confidence
ALTER TABLE llm_decisions
ADD COLUMN IF NOT EXISTS confidence DECIMAL(5, 4) DEFAULT 0;

-- Add context column (JSONB) for market conditions
ALTER TABLE llm_decisions
ADD COLUMN IF NOT EXISTS context JSONB;

-- Add alias for outcome_pnl as pnl for backward compatibility
-- Note: We'll keep outcome_pnl as the primary column but add indexes
-- that applications can use with either name pattern

-- Create index on agent_name for faster queries
CREATE INDEX IF NOT EXISTS idx_llm_decisions_agent_name ON llm_decisions (agent_name, created_at DESC);

-- Create index on confidence for filtering high-confidence decisions
CREATE INDEX IF NOT EXISTS idx_llm_decisions_confidence ON llm_decisions (confidence) WHERE confidence > 0.8;

-- Update existing rows to have default agent_name if they don't have one
UPDATE llm_decisions
SET agent_name = 'migration-default'
WHERE agent_name = 'unknown';

COMMENT ON COLUMN llm_decisions.agent_name IS 'Name of the agent that made this decision';
COMMENT ON COLUMN llm_decisions.confidence IS 'Confidence level of the decision (0.0 to 1.0)';
COMMENT ON COLUMN llm_decisions.context IS 'Market context and indicators at decision time';
