//go:build integration

package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// TestKeyManager_HMACPepper_SEC009 exercises the end-to-end flow for the
// HMAC pepper change (SEC-009 / #123). We verify:
//  1. A KeyManager with a configured pepper stores new keys with
//     hash_algorithm='hmac-sha256' and the HMAC-derived hash.
//  2. Legacy sha256 rows — simulated by inserting via a KeyManager with
//     no pepper — still validate successfully when the validator has a
//     pepper configured.
//  3. Legacy sha256 rows are opportunistically upgraded to hmac-sha256
//     on first successful validation through APIKeyStore.ValidateKey.
//  4. Keys created with one pepper DO NOT validate under a different
//     pepper (the HMAC keying is effective).
func TestKeyManager_HMACPepper_SEC009(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if os.Getenv("SKIP_TESTCONTAINER_TESTS") == "true" {
		t.Skip("Skipping testcontainer test (SKIP_TESTCONTAINER_TESTS=true)")
	}

	tc := testhelpers.SetupTestDatabase(t)
	require.NoError(t, tc.ApplyMigrations("../../migrations"))
	pool := tc.DB.Pool()
	ctx := context.Background()

	const pepper = "test-pepper-sec009-integration"

	t.Run("new keys stored with hmac-sha256 algorithm", func(t *testing.T) {
		t.Cleanup(waitForRehashesForTest)
		km := NewKeyManagerWithPepper(pool, pepper)
		created, err := km.CreateAPIKey(ctx, "user-hmac-new", "HMAC New", nil, 0)
		require.NoError(t, err)
		require.NotNil(t, created)

		var (
			storedHash string
			storedAlgo string
		)
		err = pool.QueryRow(ctx, `
			SELECT key_hash, hash_algorithm FROM api_keys WHERE id = $1
		`, created.ID).Scan(&storedHash, &storedAlgo)
		require.NoError(t, err)

		assert.Equal(t, HashAlgoHMACSHA256, storedAlgo, "new keys should use hmac-sha256")
		assert.Equal(t, hmacAPIKeyHash(pepper, created.Key), storedHash,
			"stored hash must equal HMAC(pepper, plaintext)")
		assert.NotEqual(t, HashAPIKey(created.Key), storedHash,
			"stored hash must NOT equal raw SHA-256 of plaintext")
	})

	t.Run("legacy sha256 row validates with pepper-configured validator", func(t *testing.T) {
		t.Cleanup(waitForRehashesForTest)
		// Create a key with NO pepper → gets stored as sha256.
		legacyKM := NewKeyManagerWithPepper(pool, "")
		legacy, err := legacyKM.CreateAPIKey(ctx, "user-legacy", "Legacy Key", nil, 0)
		require.NoError(t, err)

		var storedAlgo string
		err = pool.QueryRow(ctx, `SELECT hash_algorithm FROM api_keys WHERE id = $1`,
			legacy.ID).Scan(&storedAlgo)
		require.NoError(t, err)
		require.Equal(t, HashAlgoSHA256, storedAlgo,
			"precondition: legacy key should be stored as sha256")

		// Validator with pepper should still accept the legacy key.
		store := NewAPIKeyStoreWithPepper(pool, true, pepper)
		apiKey, err := store.ValidateKey(ctx, legacy.Key)
		require.NoError(t, err)
		require.NotNil(t, apiKey, "legacy sha256 key should validate via fallback probe")
		assert.Equal(t, "Legacy Key", apiKey.Name)

		// Give the async rehash goroutine time to run, then confirm the
		// row was upgraded in place (same id, new hash + algorithm).
		require.Eventually(t, func() bool {
			var hash, algo string
			if err := pool.QueryRow(ctx, `
				SELECT key_hash, hash_algorithm FROM api_keys WHERE id = $1
			`, legacy.ID).Scan(&hash, &algo); err != nil {
				return false
			}
			return algo == HashAlgoHMACSHA256 && hash == hmacAPIKeyHash(pepper, legacy.Key)
		}, 5*time.Second, 50*time.Millisecond,
			"legacy key should be opportunistically rehashed to hmac-sha256 on first use")
	})

	t.Run("key created under one pepper does not validate under another", func(t *testing.T) {
		t.Cleanup(waitForRehashesForTest)
		kmA := NewKeyManagerWithPepper(pool, "pepper-A")
		created, err := kmA.CreateAPIKey(ctx, "user-pepper-a", "Pepper A Key", nil, 0)
		require.NoError(t, err)

		// Validator with a DIFFERENT pepper should not find this key.
		storeB := NewAPIKeyStoreWithPepper(pool, true, "pepper-B")
		apiKey, err := storeB.ValidateKey(ctx, created.Key)
		require.NoError(t, err, "validation with wrong pepper should return (nil, nil), not error")
		assert.Nil(t, apiKey,
			"HMAC keying must make keys unusable under a different pepper")
	})

	t.Run("ValidateAPIKey in KeyManager probes both algorithms", func(t *testing.T) {
		t.Cleanup(waitForRehashesForTest)
		// Create a legacy key, then verify a pepper-configured KeyManager
		// can still ValidateAPIKey it (sanity for the keys.go path).
		legacyKM := NewKeyManagerWithPepper(pool, "")
		legacy, err := legacyKM.CreateAPIKey(ctx, "user-legacy-km", "Legacy KM", nil, 0)
		require.NoError(t, err)

		pepperKM := NewKeyManagerWithPepper(pool, pepper)
		details, err := pepperKM.ValidateAPIKey(ctx, legacy.Key)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, "Legacy KM", details.Name)
	})

	// Each subtest registers t.Cleanup(waitForRehashesForTest) at the
	// top so the drain call is robust against future refactors — a
	// statement-level waitForRehashesForTest() between subtests can
	// be accidentally deleted, but a Cleanup registered with the
	// subtest's t is tied to the subtest's lifecycle.

	t.Run("pepper rotation invalidates HMAC keys after opportunistic rehash", func(t *testing.T) {
		t.Cleanup(waitForRehashesForTest)
		// The documented operator footgun: a key created and then rehashed
		// to hmac-sha256 under pepper-A stops working when the service is
		// restarted with pepper-B. Verifies the invalidation is silent at
		// the API layer (no fallback), not accidentally recoverable.
		const originalPepper = "rotation-pepper-original"
		const rotatedPepper = "rotation-pepper-rotated"

		// 1. Create a key with the original pepper — stored as hmac-sha256.
		originalStore := NewAPIKeyStoreWithPepper(pool, true, originalPepper)
		originalKM := NewKeyManagerWithPepper(pool, originalPepper)
		created, err := originalKM.CreateAPIKey(ctx, "user-rotation", "Rotation Key", nil, 0)
		require.NoError(t, err)

		// Confirm it validates under the original pepper.
		apiKey, err := originalStore.ValidateKey(ctx, created.Key)
		require.NoError(t, err)
		require.NotNil(t, apiKey, "key must validate under the pepper it was created with")

		// Confirm the DB row is hmac-sha256 (not sha256 — this subtest
		// exercises the rotation scenario, NOT the legacy fallback).
		var algo string
		err = pool.QueryRow(ctx, `SELECT hash_algorithm FROM api_keys WHERE id = $1`,
			created.ID).Scan(&algo)
		require.NoError(t, err)
		require.Equal(t, HashAlgoHMACSHA256, algo,
			"precondition: key should be stored as hmac-sha256 under original pepper")

		// 2. Restart with rotated pepper. The key was hmac-stored so the
		// legacy fallback probe also can't find it (the stored hash is
		// the HMAC of the old pepper, not the plaintext SHA-256).
		rotatedStore := NewAPIKeyStoreWithPepper(pool, true, rotatedPepper)
		apiKey, err = rotatedStore.ValidateKey(ctx, created.Key)
		require.NoError(t, err, "rotation must return (nil, nil), not an error")
		assert.Nil(t, apiKey,
			"HMAC-rotated key must NOT validate under the new pepper — no accidental fallback")

		// 3. Same check via KeyManager.ValidateAPIKey (the /keys endpoint path).
		rotatedKM := NewKeyManagerWithPepper(pool, rotatedPepper)
		details, err := rotatedKM.ValidateAPIKey(ctx, created.Key)
		require.Error(t, err, "rotated pepper must produce ErrKeyNotFound")
		assert.ErrorIs(t, err, ErrKeyNotFound)
		assert.Nil(t, details)

		// 4. A legacy sha256 row DOES survive pepper rotation because its
		// hash is independent of the pepper. This half of the contract
		// is the mirror image: legacy keys keep working across rotations,
		// HMAC keys don't.
		legacyCreated, err := NewKeyManagerWithPepper(pool, "").CreateAPIKey(
			ctx, "user-rotation-legacy", "Rotation Legacy", nil, 0,
		)
		require.NoError(t, err)
		apiKey, err = rotatedStore.ValidateKey(ctx, legacyCreated.Key)
		require.NoError(t, err)
		require.NotNil(t, apiKey,
			"legacy sha256 keys survive pepper rotation — they aren't dependent on the pepper value")
	})
}
