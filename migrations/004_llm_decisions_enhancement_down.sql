-- Migration 004: Enhance LLM Decisions Table - DOWN Migration
-- Reverses 004_llm_decisions_enhancement.sql

-- =============================================================================
-- DROP INDEXES
-- =============================================================================
DROP INDEX IF EXISTS idx_llm_decisions_confidence;
DROP INDEX IF EXISTS idx_llm_decisions_agent_name;

-- =============================================================================
-- DROP COLUMNS
-- Note: Using ALTER TABLE to remove columns added by migration 004
-- =============================================================================
ALTER TABLE llm_decisions DROP COLUMN IF EXISTS context;
ALTER TABLE llm_decisions DROP COLUMN IF EXISTS confidence;
ALTER TABLE llm_decisions DROP COLUMN IF EXISTS agent_name;

SELECT 'LLM decisions enhancement reverted successfully!' AS status;
