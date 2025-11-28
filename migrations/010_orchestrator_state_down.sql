-- Migration Rollback: Orchestrator State Table
-- Description: Drops the orchestrator_state table and its index
-- Author: T293 - Wire API Pause Trading to Orchestrator
-- Date: 2025-01-27

-- Drop index first (best practice, though CASCADE would handle it)
DROP INDEX IF EXISTS idx_orchestrator_state_updated_at;

-- Drop orchestrator_state table
DROP TABLE IF EXISTS orchestrator_state;

SELECT 'Migration 010 down: Orchestrator state table removed!' AS status;
