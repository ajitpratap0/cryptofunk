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
		assert.Equal(t, HashAPIKeyHMAC(pepper, created.Key), storedHash,
			"stored hash must equal HMAC(pepper, plaintext)")
		assert.NotEqual(t, HashAPIKey(created.Key), storedHash,
			"stored hash must NOT equal raw SHA-256 of plaintext")
	})

	t.Run("legacy sha256 row validates with pepper-configured validator", func(t *testing.T) {
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
			return algo == HashAlgoHMACSHA256 && hash == HashAPIKeyHMAC(pepper, legacy.Key)
		}, 5*time.Second, 50*time.Millisecond,
			"legacy key should be opportunistically rehashed to hmac-sha256 on first use")
	})

	t.Run("key created under one pepper does not validate under another", func(t *testing.T) {
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
}
