-- Migration Rollback: Orchestrator State Table
-- Description: Drops the orchestrator_state table
-- Author: T293 - Wire API Pause Trading to Orchestrator
-- Date: 2025-01-27

-- Drop orchestrator_state table
DROP TABLE IF EXISTS orchestrator_state;
