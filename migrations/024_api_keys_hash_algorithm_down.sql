-- Rollback for 024_api_keys_hash_algorithm.sql
--
-- WARNING: DATA LOSS — read before running.
--
-- This rollback drops the hash_algorithm column unconditionally. Any row
-- whose key_hash was opportunistically rehashed to hmac-sha256 after
-- migration 024 was applied no longer has a recoverable SHA-256 hash —
-- the HMAC hash is stored in key_hash (a keyed one-way function of the
-- plaintext + pepper), and the original SHA-256 is lost. Rolling back
-- will leave those rows with key_hash values that NO plaintext key can
-- ever match again, permanently invalidating every API key that has
-- been validated at least once since the pepper was enabled.
--
-- Before running this rollback:
--   1. Check how many rows were rehashed:
--        SELECT hash_algorithm, COUNT(*) FROM api_keys GROUP BY hash_algorithm;
--   2. If any rows are hash_algorithm='hmac-sha256', every affected
--      user must re-create their API key after rollback — there is no
--      in-place recovery.
--   3. Communicate the invalidation to affected users BEFORE rolling
--      back. A silent rollback will look like a platform-wide auth
--      outage to anyone holding a previously-working key.

ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_hash_algorithm_check;
DROP INDEX IF EXISTS idx_api_keys_hash_algo;
ALTER TABLE api_keys DROP COLUMN IF EXISTS hash_algorithm;
