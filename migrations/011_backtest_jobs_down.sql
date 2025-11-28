-- Migration Down: Backtest Jobs
-- Description: Drops backtest jobs table and related objects
-- Version: 011

-- Drop trigger
DROP TRIGGER IF EXISTS trigger_update_backtest_jobs_updated_at ON backtest_jobs;

-- Drop function
DROP FUNCTION IF EXISTS update_backtest_jobs_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_backtest_jobs_status;
DROP INDEX IF EXISTS idx_backtest_jobs_created_at;
DROP INDEX IF EXISTS idx_backtest_jobs_created_by;
DROP INDEX IF EXISTS idx_backtest_jobs_performance;
DROP INDEX IF EXISTS idx_backtest_jobs_strategy_config;

-- Drop table
DROP TABLE IF EXISTS backtest_jobs;
