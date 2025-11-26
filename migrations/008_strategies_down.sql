-- Migration: 008_strategies_down.sql
-- Description: Drop strategies tables
-- Created: 2025-11-26

DROP TABLE IF EXISTS strategy_history;
DROP TABLE IF EXISTS strategies;
