-- =============================================================================
-- Migration 006: Add Search and Performance Indexes for LLM Decisions
-- =============================================================================
-- This migration adds indexes to optimize the decision explainability API:
-- 1. Full-text search index on prompt and response
-- 2. Additional indexes for filtering and pagination
--
-- NOTE: idx_llm_decisions_symbol and idx_llm_decisions_type already exist
-- in migration 001 with the same columns, so we don't recreate them here.

-- Full-text search index for text-based decision search
-- Uses GIN index for efficient to_tsvector lookups
CREATE INDEX IF NOT EXISTS idx_llm_decisions_fulltext
ON llm_decisions
USING gin(to_tsvector('english', COALESCE(prompt, '') || ' ' || COALESCE(response, '')));

-- Index for model analysis queries (new - not in migration 001)
CREATE INDEX IF NOT EXISTS idx_llm_decisions_model_created
ON llm_decisions (model, created_at DESC);

-- Index for outcome analysis (filtering successful/failed decisions)
CREATE INDEX IF NOT EXISTS idx_llm_decisions_outcome_pnl
ON llm_decisions (outcome, outcome_pnl)
WHERE outcome IS NOT NULL;

-- Index for session-based queries
CREATE INDEX IF NOT EXISTS idx_llm_decisions_session_created
ON llm_decisions (session_id, created_at DESC)
WHERE session_id IS NOT NULL;

-- Partial index for high-confidence decisions (commonly queried)
CREATE INDEX IF NOT EXISTS idx_llm_decisions_high_confidence
ON llm_decisions (created_at DESC)
WHERE confidence >= 0.8;

-- Index for latency analysis
CREATE INDEX IF NOT EXISTS idx_llm_decisions_latency
ON llm_decisions (latency_ms)
WHERE latency_ms IS NOT NULL;

-- Update table statistics for better query planning
ANALYZE llm_decisions;

-- Add comments for documentation
COMMENT ON INDEX idx_llm_decisions_fulltext IS 'Full-text search index for prompt and response fields';
COMMENT ON INDEX idx_llm_decisions_model_created IS 'Index for model analysis queries';
COMMENT ON INDEX idx_llm_decisions_outcome_pnl IS 'Index for outcome and P&L analysis';
COMMENT ON INDEX idx_llm_decisions_session_created IS 'Index for session-based queries';
COMMENT ON INDEX idx_llm_decisions_high_confidence IS 'Partial index for high confidence decisions';
COMMENT ON INDEX idx_llm_decisions_latency IS 'Index for latency analysis';

SELECT 'Migration 006: LLM Decisions search indexes created successfully!' AS status;
