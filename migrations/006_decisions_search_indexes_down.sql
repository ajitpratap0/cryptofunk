-- =============================================================================
-- Migration 006 Down: Remove Search and Performance Indexes for LLM Decisions
-- =============================================================================
-- NOTE: Only drops indexes created in migration 006 UP.
-- idx_llm_decisions_symbol and idx_llm_decisions_type are from migration 001.

DROP INDEX IF EXISTS idx_llm_decisions_fulltext;
DROP INDEX IF EXISTS idx_llm_decisions_model_created;
DROP INDEX IF EXISTS idx_llm_decisions_outcome_pnl;
DROP INDEX IF EXISTS idx_llm_decisions_session_created;
DROP INDEX IF EXISTS idx_llm_decisions_high_confidence;
DROP INDEX IF EXISTS idx_llm_decisions_latency;

SELECT 'Migration 006 down: LLM Decisions search indexes removed!' AS status;
