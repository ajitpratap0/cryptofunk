-- Migration: Add hash_algorithm column to api_keys (SEC-009 / #123)
--
-- Adds a per-row marker for which hash scheme protects each key_hash value.
-- Values:
--   'sha256'      — legacy (raw SHA-256 of the plaintext key, vulnerable to
--                   precomputed rainbow tables if the DB is stolen)
--   'hmac-sha256' — HMAC-SHA256 keyed with a server-side pepper loaded from
--                   CRYPTOFUNK_API_AUTH_KEY_PEPPER (maps to api.auth.key_pepper
--                   via viper). Protects against DB-only theft because an
--                   attacker needs both the DB dump AND the pepper.
--
-- Existing rows are marked 'sha256' so validation continues to work during
-- the rollout window. New keys created after the application is upgraded
-- with a pepper configured will be stored as 'hmac-sha256'. The application
-- also opportunistically rehashes a legacy 'sha256' key to 'hmac-sha256' on
-- its first successful validation after the upgrade, so the fleet drains
-- to the new scheme as keys are used.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS hash_algorithm VARCHAR(32) NOT NULL DEFAULT 'sha256';

-- Backfill: existing rows are all SHA-256 (the column default handles this
-- for IF NOT EXISTS on a fresh schema, but an explicit UPDATE is a no-op
-- safety net for environments where the column already existed with NULLs).
UPDATE api_keys SET hash_algorithm = 'sha256' WHERE hash_algorithm IS NULL;

-- Lookup index: the application queries by (key_hash, hash_algorithm) when
-- validating, so an index on the pair speeds up the hot path. The existing
-- idx_api_keys_key_hash remains for uniqueness and the single-column path.
CREATE INDEX IF NOT EXISTS idx_api_keys_hash_algo
    ON api_keys(key_hash, hash_algorithm);

COMMENT ON COLUMN api_keys.hash_algorithm IS
    'Which hash scheme protects key_hash: sha256 (legacy) or hmac-sha256 (with server pepper). See SEC-009.';
