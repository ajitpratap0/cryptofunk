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

-- Pre-flight sanity check: if this migration has run before and rows
-- were re-tagged to a value other than 'sha256' or 'hmac-sha256', fail
-- loudly rather than overwriting them. A fresh schema has zero rows
-- and hits the NO-OP path.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'api_keys' AND column_name = 'hash_algorithm'
    ) THEN
        IF EXISTS (
            SELECT 1 FROM api_keys
            WHERE hash_algorithm IS NOT NULL
              AND hash_algorithm NOT IN ('sha256', 'hmac-sha256')
            LIMIT 1
        ) THEN
            RAISE EXCEPTION 'migration 024: api_keys.hash_algorithm contains unexpected values — manual inspection required before re-running';
        END IF;
    END IF;
END $$;

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS hash_algorithm VARCHAR(32) NOT NULL DEFAULT 'sha256';

-- Enforce the algorithm marker at the database layer so a direct SQL
-- INSERT (bypassing the application) or a future refactor that adds a
-- third scheme can't silently land an unexpected value in the column.
-- Postgres allows ADD CONSTRAINT to be idempotent via a DO block.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'api_keys' AND constraint_name = 'api_keys_hash_algorithm_check'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT api_keys_hash_algorithm_check
            CHECK (hash_algorithm IN ('sha256', 'hmac-sha256'));
    END IF;
END $$;

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
