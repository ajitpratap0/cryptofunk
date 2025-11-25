-- Migration 002: Semantic Memory System - DOWN Migration
-- Reverses 002_semantic_memory.sql

-- =============================================================================
-- DROP INDEXES
-- =============================================================================
DROP INDEX IF EXISTS idx_semantic_memory_context;
DROP INDEX IF EXISTS idx_semantic_memory_type_agent;
DROP INDEX IF EXISTS idx_semantic_memory_expires_at;
DROP INDEX IF EXISTS idx_semantic_memory_created_at;
DROP INDEX IF EXISTS idx_semantic_memory_importance;
DROP INDEX IF EXISTS idx_semantic_memory_source;
DROP INDEX IF EXISTS idx_semantic_memory_symbol;
DROP INDEX IF EXISTS idx_semantic_memory_agent;
DROP INDEX IF EXISTS idx_semantic_memory_type;
DROP INDEX IF EXISTS idx_semantic_memory_embedding;

-- =============================================================================
-- DROP TABLE
-- =============================================================================
DROP TABLE IF EXISTS semantic_memory;

SELECT 'Semantic memory tables dropped successfully!' AS status;
