-- =============================================================================
-- Migration 006 Down: Remove Search and Performance Indexes for LLM Decisions
-- =============================================================================

DROP INDEX IF EXISTS idx_llm_decisions_fulltext;
DROP INDEX IF EXISTS idx_llm_decisions_symbol_created;
DROP INDEX IF EXISTS idx_llm_decisions_model_created;
DROP INDEX IF EXISTS idx_llm_decisions_type_created;
DROP INDEX IF EXISTS idx_llm_decisions_outcome_pnl;
DROP INDEX IF EXISTS idx_llm_decisions_session_created;
DROP INDEX IF EXISTS idx_llm_decisions_high_confidence;
DROP INDEX IF EXISTS idx_llm_decisions_latency;

SELECT 'Migration 006 down: LLM Decisions search indexes removed!' AS status;
