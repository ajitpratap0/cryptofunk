-- Rollback for 024_api_keys_hash_algorithm.sql

DROP INDEX IF EXISTS idx_api_keys_hash_algo;
ALTER TABLE api_keys DROP COLUMN IF EXISTS hash_algorithm;
