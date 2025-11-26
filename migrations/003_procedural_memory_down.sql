-- Migration 003: Procedural Memory System - DOWN Migration
-- Reverses 003_procedural_memory.sql

-- =============================================================================
-- DROP INDEXES FOR SKILLS
-- =============================================================================
DROP INDEX IF EXISTS idx_procedural_skills_implementation;
DROP INDEX IF EXISTS idx_procedural_skills_created_at;
DROP INDEX IF EXISTS idx_procedural_skills_proficiency;
DROP INDEX IF EXISTS idx_procedural_skills_active;
DROP INDEX IF EXISTS idx_procedural_skills_agent;
DROP INDEX IF EXISTS idx_procedural_skills_type;

-- =============================================================================
-- DROP INDEXES FOR POLICIES
-- =============================================================================
DROP INDEX IF EXISTS idx_procedural_policies_actions;
DROP INDEX IF EXISTS idx_procedural_policies_conditions;
DROP INDEX IF EXISTS idx_procedural_policies_created_at;
DROP INDEX IF EXISTS idx_procedural_policies_performance;
DROP INDEX IF EXISTS idx_procedural_policies_active;
DROP INDEX IF EXISTS idx_procedural_policies_symbol;
DROP INDEX IF EXISTS idx_procedural_policies_agent;
DROP INDEX IF EXISTS idx_procedural_policies_type;

-- =============================================================================
-- DROP TABLES
-- =============================================================================
DROP TABLE IF EXISTS procedural_memory_skills;
DROP TABLE IF EXISTS procedural_memory_policies;

SELECT 'Procedural memory tables dropped successfully!' AS status;
