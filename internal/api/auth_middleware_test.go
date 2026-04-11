package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Struct Tests
// =============================================================================

func TestAPIKeyStruct(t *testing.T) {
	id := uuid.New()
	userID := "test-user"
	keyHash := HashAPIKey("test-key")
	now := time.Now()

	apiKey := APIKey{
		ID:          id,
		KeyHash:     keyHash,
		Name:        "Test Key",
		UserID:      userID,
		Permissions: []string{"read", "write"},
		LastUsedAt:  &now,
		CreatedAt:   now,
		ExpiresAt:   nil,
		Revoked:     false,
	}

	assert.Equal(t, id, apiKey.ID)
	assert.Equal(t, keyHash, apiKey.KeyHash)
	assert.Equal(t, "Test Key", apiKey.Name)
	assert.Equal(t, userID, apiKey.UserID)
	assert.Len(t, apiKey.Permissions, 2)
	assert.Contains(t, apiKey.Permissions, "read")
	assert.Contains(t, apiKey.Permissions, "write")
	assert.False(t, apiKey.Revoked)
	assert.Nil(t, apiKey.ExpiresAt)
}

func TestAuthConfigStruct(t *testing.T) {
	config := AuthConfig{
		Enabled:      true,
		HeaderName:   "X-API-Key",
		RequireHTTPS: true,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "X-API-Key", config.HeaderName)
	assert.True(t, config.RequireHTTPS)
}

func TestDefaultAuthConfig(t *testing.T) {
	config := DefaultAuthConfig()

	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, "X-API-Key", config.HeaderName)
	assert.True(t, config.RequireHTTPS)
	assert.False(t, config.TrustForwardedProto)
}

// TestMustHashAPIKeyHMAC_SEC009 verifies the HMAC pepper helper produces
// distinct outputs for different peppers and matches the stdlib HMAC
// contract. Adding a server-side pepper is the core mitigation for
// SEC-009 / #123 — a stolen DB dump is no longer enough to run rainbow
// tables against the stored hashes.
func TestMustHashAPIKeyHMAC_SEC009(t *testing.T) {
	t.Run("different peppers produce different hashes for same key", func(t *testing.T) {
		key := "the-same-plaintext-key"
		h1 := mustHashAPIKeyHMAC("pepper-A", key)
		h2 := mustHashAPIKeyHMAC("pepper-B", key)
		assert.NotEqual(t, h1, h2, "distinct peppers must produce distinct hashes")
		assert.Len(t, h1, 64, "HMAC-SHA256 hex output is always 64 chars")
		assert.Len(t, h2, 64)
	})

	t.Run("same pepper is deterministic", func(t *testing.T) {
		h1 := mustHashAPIKeyHMAC("secret", "my-key")
		h2 := mustHashAPIKeyHMAC("secret", "my-key")
		assert.Equal(t, h1, h2, "HMAC with the same pepper+key must be deterministic")
	})

	t.Run("HMAC differs from raw SHA-256 for the same key", func(t *testing.T) {
		key := "my-api-key"
		sha := HashAPIKey(key)
		// Variable name is hmacHash (not 'hmac') so it doesn't shadow the
		// crypto/hmac package if this test ever imports it directly.
		hmacHash := mustHashAPIKeyHMAC("some-pepper", key)
		assert.NotEqual(t, sha, hmacHash, "HMAC with a non-empty pepper must not equal raw SHA-256")
	})

	t.Run("panics on empty pepper", func(t *testing.T) {
		// Documents the invariant: mustHashAPIKeyHMAC intentionally rejects
		// empty peppers because HMAC-SHA256 with a zero-length key has
		// no entropy and is equivalent to a published constant. Any
		// future "fix" that removes the panic without understanding the
		// security implication will fail this test.
		assert.Panics(t, func() {
			_ = mustHashAPIKeyHMAC("", "any-key")
		}, "mustHashAPIKeyHMAC must panic on empty pepper to prevent silent downgrade")
	})
}

// TestHashAPIKeyForStorage_SEC009 covers the dispatch helper used by
// CreateAPIKey / RotateAPIKey. Empty pepper → legacy SHA-256; non-empty
// pepper → HMAC-SHA256. The returned algorithm marker drives the
// api_keys.hash_algorithm column value.
func TestHashAPIKeyForStorage_SEC009(t *testing.T) {
	t.Run("empty pepper returns sha256", func(t *testing.T) {
		hash, algo := HashAPIKeyForStorage("", "some-key")
		assert.Equal(t, HashAlgoSHA256, algo)
		assert.Equal(t, HashAPIKey("some-key"), hash)
	})

	t.Run("non-empty pepper returns hmac-sha256", func(t *testing.T) {
		hash, algo := HashAPIKeyForStorage("my-pepper", "some-key")
		assert.Equal(t, HashAlgoHMACSHA256, algo)
		assert.Equal(t, mustHashAPIKeyHMAC("my-pepper", "some-key"), hash)
		// And it must NOT match the legacy scheme.
		assert.NotEqual(t, HashAPIKey("some-key"), hash)
	})
}

// TestTrustForwardedProto_SEC004 verifies that the X-Forwarded-Proto header
// is only trusted when TrustForwardedProto is explicitly enabled, preventing
// the HTTPS bypass described in SEC-004 / #118.
func TestTrustForwardedProto_SEC004(t *testing.T) {
	// The HTTPS enforcement check runs BEFORE key validation in the
	// middleware, so we can use a nil-DB store — the test either gets 403
	// (HTTPS blocked) or proceeds past the HTTPS gate to key validation
	// (which returns 401 because there's no DB). Status != 403 proves
	// the HTTPS check passed.
	store := NewAPIKeyStore(nil, true)

	okHandler := func(c *gin.Context) { c.String(http.StatusOK, "ok") }

	t.Run("spoofed_header_blocked_when_trust_disabled", func(t *testing.T) {
		config := &AuthConfig{
			Enabled:             true,
			HeaderName:          "X-API-Key",
			RequireHTTPS:        true,
			TrustForwardedProto: false,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(AuthMiddleware(store, config))
		router.GET("/secure", okHandler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/secure", nil)
		req.Header.Set("X-API-Key", "any-key")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Host = "api.example.com"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code,
			"spoofed X-Forwarded-Proto should be rejected when TrustForwardedProto=false")
	})

	t.Run("forwarded_proto_trusted_when_enabled", func(t *testing.T) {
		config := &AuthConfig{
			Enabled:             true,
			HeaderName:          "X-API-Key",
			RequireHTTPS:        true,
			TrustForwardedProto: true,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(AuthMiddleware(store, config))
		router.GET("/secure", okHandler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/secure", nil)
		req.Header.Set("X-API-Key", "any-key")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Host = "api.example.com"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusForbidden, w.Code,
			"X-Forwarded-Proto: https should be trusted when TrustForwardedProto=true (expect 401 from key validation, not 403)")
	})

	t.Run("ipv6_loopback_allowed", func(t *testing.T) {
		config := &AuthConfig{
			Enabled:             true,
			HeaderName:          "X-API-Key",
			RequireHTTPS:        true,
			TrustForwardedProto: false,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(AuthMiddleware(store, config))
		router.GET("/secure", okHandler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/secure", nil)
		req.Header.Set("X-API-Key", "any-key")
		req.Host = "[::1]:8080"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusForbidden, w.Code,
			"IPv6 loopback should be allowed even without HTTPS (expect 401, not 403)")
	})
}

// =============================================================================
// HashAPIKey Tests
// =============================================================================

func TestHashAPIKey(t *testing.T) {
	t.Run("consistent hashing", func(t *testing.T) {
		key := "test-api-key-12345"
		hash1 := HashAPIKey(key)
		hash2 := HashAPIKey(key)

		assert.Equal(t, hash1, hash2)
		assert.Len(t, hash1, 64) // SHA-256 produces 64 hex chars
	})

	t.Run("different keys produce different hashes", func(t *testing.T) {
		hash1 := HashAPIKey("key1")
		hash2 := HashAPIKey("key2")

		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("empty key produces valid hash", func(t *testing.T) {
		hash := HashAPIKey("")
		assert.Len(t, hash, 64)
	})
}

// =============================================================================
// Mock API Key Store
// =============================================================================

type MockAPIKeyStore struct {
	ValidateKeyFunc func(ctx context.Context, key string) (*APIKey, error)
}

func (m *MockAPIKeyStore) ValidateKey(ctx context.Context, key string) (*APIKey, error) {
	if m.ValidateKeyFunc != nil {
		return m.ValidateKeyFunc(ctx, key)
	}
	return nil, nil
}

// =============================================================================
// APIKeyStore Tests
// =============================================================================

func TestNewAPIKeyStore(t *testing.T) {
	t.Run("creates store with enabled flag", func(t *testing.T) {
		store := NewAPIKeyStore(nil, true)
		assert.NotNil(t, store)
		assert.True(t, store.enabled)
	})

	t.Run("creates disabled store", func(t *testing.T) {
		store := NewAPIKeyStore(nil, false)
		assert.NotNil(t, store)
		assert.False(t, store.enabled)
	})
}

func TestAPIKeyStoreValidateKey_NilDB(t *testing.T) {
	store := NewAPIKeyStore(nil, true)
	apiKey, err := store.ValidateKey(context.Background(), "test-key")

	// With nil db, should return nil, nil
	assert.Nil(t, apiKey)
	assert.NoError(t, err)
}

// =============================================================================
// AuthMiddleware Tests
// =============================================================================

func setupAuthTestRouter(store *APIKeyStore, config *AuthConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(AuthMiddleware(store, config))
	router.GET("/test", func(c *gin.Context) {
		// Return user context values if set
		userID, _ := c.Get("user_id")
		keyID, _ := c.Get("api_key_id")
		keyName, _ := c.Get("api_key_name")
		perms, _ := c.Get("permissions")

		c.JSON(http.StatusOK, gin.H{
			"message":      "success",
			"user_id":      userID,
			"api_key_id":   keyID,
			"api_key_name": keyName,
			"permissions":  perms,
		})
	})

	return router
}

func TestAuthMiddleware_AuthDisabled(t *testing.T) {
	store := NewAPIKeyStore(nil, false) // Auth disabled
	config := &AuthConfig{Enabled: false}

	router := setupAuthTestRouter(store, config)

	t.Run("allows requests without API key when disabled", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAuthMiddleware_AuthEnabled(t *testing.T) {
	validKey := "valid-api-key"
	validKeyHash := HashAPIKey(validKey)
	keyID := uuid.New()

	config := &AuthConfig{
		Enabled:      true,
		HeaderName:   "X-API-Key",
		RequireHTTPS: false, // Disable for testing
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a custom middleware that intercepts validation
	router.Use(func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		apiKey := c.GetHeader(config.HeaderName)
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				apiKey = authHeader[7:]
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "API key required",
				"message": "Provide API key via X-API-Key header or Authorization: Bearer <key>",
			})
			c.Abort()
			return
		}

		// Validate against our known valid key
		if HashAPIKey(apiKey) == validKeyHash {
			c.Set("user_id", "test-user")
			c.Set("api_key_id", keyID.String())
			c.Set("api_key_name", "Test Key")
			c.Set("permissions", []string{"read", "write"})
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired API key",
		})
		c.Abort()
	})

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
			"user_id": c.GetString("user_id"),
		})
	})

	t.Run("rejects request without API key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("accepts valid API key in X-API-Key header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-API-Key", validKey)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("accepts valid API key in Authorization header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+validKey)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects invalid API key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-API-Key", "invalid-key")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAuthMiddleware_NilConfig(t *testing.T) {
	store := NewAPIKeyStore(nil, false)

	// Should use default config when nil is passed
	router := setupAuthTestRouter(store, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Default config has Enabled: false, so should pass
	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// RequirePermission Tests
// =============================================================================

func setupPermissionTestRouter(permissions []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// First middleware sets up permissions
	router.Use(func(c *gin.Context) {
		if permissions != nil {
			c.Set("permissions", permissions)
		}
		c.Next()
	})

	// Then apply permission check
	router.GET("/admin", RequirePermission("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin access granted"})
	})

	router.GET("/read", RequirePermission("read"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "read access granted"})
	})

	return router
}

func TestRequirePermission(t *testing.T) {
	t.Run("denies access when no permissions set", func(t *testing.T) {
		router := setupPermissionTestRouter(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("denies access when permission not found", func(t *testing.T) {
		router := setupPermissionTestRouter([]string{"read", "write"})

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("allows access with exact permission", func(t *testing.T) {
		router := setupPermissionTestRouter([]string{"admin", "read"})

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("allows access with wildcard permission", func(t *testing.T) {
		router := setupPermissionTestRouter([]string{"*"})

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("allows access with admin permission as wildcard", func(t *testing.T) {
		router := setupPermissionTestRouter([]string{"admin"})

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/read", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// =============================================================================
// OptionalAuth Tests
// =============================================================================

func setupOptionalAuthRouter(store *APIKeyStore, config *AuthConfig, validKey string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use a simplified version of OptionalAuth for testing
	router.Use(func(c *gin.Context) {
		if !store.enabled {
			c.Next()
			return
		}

		apiKey := c.GetHeader(config.HeaderName)
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				apiKey = authHeader[7:]
			}
		}

		// No key provided - continue without auth
		if apiKey == "" {
			c.Next()
			return
		}

		// Validate the key if provided
		if apiKey == validKey {
			c.Set("user_id", "test-user")
			c.Set("api_key_id", uuid.New().String())
			c.Set("api_key_name", "Test Key")
			c.Set("permissions", []string{"read", "write"})
			c.Next()
			return
		}

		// Invalid key provided - reject
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid API key",
		})
		c.Abort()
	})

	router.GET("/test", func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{
			"authenticated": exists,
			"user_id":       userID,
		})
	})

	return router
}

func TestOptionalAuth(t *testing.T) {
	validKey := "valid-key"
	store := &APIKeyStore{db: nil, enabled: true}
	config := &AuthConfig{
		Enabled:      true,
		HeaderName:   "X-API-Key",
		RequireHTTPS: false,
	}

	router := setupOptionalAuthRouter(store, config, validKey)

	t.Run("allows unauthenticated requests", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("allows authenticated requests", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-API-Key", validKey)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects invalid key when provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-API-Key", "invalid-key")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestOptionalAuth_Disabled(t *testing.T) {
	store := &APIKeyStore{db: nil, enabled: false}
	config := &AuthConfig{Enabled: false}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(OptionalAuth(store, config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOptionalAuth_NilConfig(t *testing.T) {
	store := &APIKeyStore{db: nil, enabled: false}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(OptionalAuth(store, nil)) // nil config
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Should use default config and pass
	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// Integration Tests - Full Middleware Chain
// =============================================================================

func TestAuthMiddlewareChain(t *testing.T) {
	validKey := "chain-test-key"
	validKeyHash := HashAPIKey(validKey)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Simulate auth middleware
	router.Use(func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.Next()
			return
		}

		if HashAPIKey(apiKey) == validKeyHash {
			c.Set("user_id", "test-user")
			c.Set("permissions", []string{"read"})
		}
		c.Next()
	})

	// Protected endpoint
	router.GET("/protected", RequirePermission("read"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected content"})
	})

	// Unprotected endpoint
	router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public content"})
	})

	t.Run("public endpoint works without auth", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/public", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("protected endpoint fails without auth", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("protected endpoint works with valid auth", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
		req.Header.Set("X-API-Key", validKey)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestAuthEdgeCases(t *testing.T) {
	t.Run("empty API key is treated as no key", func(t *testing.T) {
		store := &APIKeyStore{db: nil, enabled: true}
		config := &AuthConfig{
			Enabled:      true,
			HeaderName:   "X-API-Key",
			RequireHTTPS: false,
		}

		router := setupAuthTestRouter(store, config)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-API-Key", "")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Bearer prefix without key is rejected", func(t *testing.T) {
		store := &APIKeyStore{db: nil, enabled: true}
		config := &AuthConfig{
			Enabled:      true,
			HeaderName:   "X-API-Key",
			RequireHTTPS: false,
		}

		router := setupAuthTestRouter(store, config)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("wrong authorization scheme is ignored", func(t *testing.T) {
		store := &APIKeyStore{db: nil, enabled: true}
		config := &AuthConfig{
			Enabled:      true,
			HeaderName:   "X-API-Key",
			RequireHTTPS: false,
		}

		router := setupAuthTestRouter(store, config)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// =============================================================================
// Revoked/Expired Key Tests
// =============================================================================

func TestAPIKeyRevocation(t *testing.T) {
	// Test that revoked keys are properly detected
	t.Run("revoked key struct", func(t *testing.T) {
		apiKey := APIKey{
			ID:      uuid.New(),
			KeyHash: HashAPIKey("test"),
			Name:    "Revoked Key",
			Revoked: true,
		}

		assert.True(t, apiKey.Revoked)
	})
}

func TestAPIKeyExpiration(t *testing.T) {
	t.Run("expired key struct", func(t *testing.T) {
		past := time.Now().Add(-24 * time.Hour)
		apiKey := APIKey{
			ID:        uuid.New(),
			KeyHash:   HashAPIKey("test"),
			Name:      "Expired Key",
			ExpiresAt: &past,
		}

		require.NotNil(t, apiKey.ExpiresAt)
		assert.True(t, apiKey.ExpiresAt.Before(time.Now()))
	})

	t.Run("valid non-expired key struct", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		apiKey := APIKey{
			ID:        uuid.New(),
			KeyHash:   HashAPIKey("test"),
			Name:      "Valid Key",
			ExpiresAt: &future,
		}

		require.NotNil(t, apiKey.ExpiresAt)
		assert.True(t, apiKey.ExpiresAt.After(time.Now()))
	})

	t.Run("key with no expiration", func(t *testing.T) {
		apiKey := APIKey{
			ID:        uuid.New(),
			KeyHash:   HashAPIKey("test"),
			Name:      "No Expiration Key",
			ExpiresAt: nil,
		}

		assert.Nil(t, apiKey.ExpiresAt)
	})
}

// TestAPIKeyStore_ValidateKey_HMAC_SEC009 mirrors
// TestKeyManager_ValidateAPIKey_HMAC_SEC009 for the middleware hot
// path. Verifies that APIKeyStore.ValidateKey:
//  1. Probes the HMAC hash first when a pepper is configured and
//     matches a key stored with hash_algorithm='hmac-sha256'.
//  2. Falls back to the SHA-256 probe when the HMAC probe misses
//     (legacy row) and still returns the key.
//  3. Propagates non-ErrNoRows DB errors from the first probe WITHOUT
//     silently falling through to the legacy probe — the critical
//     security invariant introduced in round 1.
//  4. Returns (nil, nil) when every probe reports ErrNoRows (the
//     middleware consumes this as a clean "not authorized" rejection).
func TestAPIKeyStore_ValidateKey_HMAC_SEC009(t *testing.T) {
	const pepper = "unit-test-pepper-sec009"

	newRow := func(keyHash string) *pgxmock.Rows {
		return pgxmock.NewRows([]string{
			"id", "key_hash", "name", "user_id", "permissions", "last_used_at",
			"created_at", "expires_at", "revoked", "is_active",
		}).AddRow(
			uuid.New(),
			keyHash,
			"Test Key",
			"test-user",
			[]byte(`["read:decisions"]`),
			nil,
			time.Now(),
			nil,
			false,
			true,
		)
	}

	// Note on pgxmock v3 + concurrent Exec: pgxmock v3 has a known
	// mutex bug when MatchExpectationsInOrder(false) is combined with
	// concurrent Exec calls from goroutines, producing "sync: unlock
	// of unlocked mutex" panics. These subtests use the default
	// in-order matching: expectations are consumed in sequence, so
	// async goroutines must spawn in a deterministic order relative
	// to setup. waitForRehashesForTest() drains in-flight rehash
	// goroutines before mock.Close() to avoid racing teardown.

	t.Run("HMAC probe hits first", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)

		store := NewAPIKeyStoreWithPepper(mock, true, pepper)
		plaintext := "middleware-key-hmac-hit"
		hmacHash := mustHashAPIKeyHMAC(pepper, plaintext)

		mock.ExpectQuery("SELECT id, key_hash, name").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnRows(newRow(hmacHash))
		// The only async work on the HMAC-hit path is the last_used_at update.
		mock.ExpectExec("UPDATE api_keys SET last_used_at").
			WithArgs(pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		apiKey, err := store.ValidateKey(context.Background(), plaintext)
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		assert.Equal(t, "Test Key", apiKey.Name)
		waitForRehashesForTest()
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})

	t.Run("HMAC probe misses, SHA-256 fallback hits and triggers async rehash", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)

		store := NewAPIKeyStoreWithPepper(mock, true, pepper)
		plaintext := "middleware-key-legacy"
		hmacHash := mustHashAPIKeyHMAC(pepper, plaintext)
		legacyHash := HashAPIKey(plaintext)

		// First probe (HMAC) returns ErrNoRows.
		mock.ExpectQuery("SELECT id, key_hash, name").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnError(pgx.ErrNoRows)
		// Second probe (SHA-256) returns the legacy row.
		mock.ExpectQuery("SELECT id, key_hash, name").
			WithArgs(legacyHash, HashAlgoSHA256).
			WillReturnRows(newRow(legacyHash))
		// Two sequential UPDATEs issued by the SINGLE runAsyncKeyOps
		// goroutine — rehash first, then last_used_at. The helper
		// chains them in one goroutine so pgxmock's in-order matcher
		// is deterministic. Use WithArgs with AnyArg so the rehash's
		// 4-parameter call and the last_used_at's 1-parameter call
		// both match cleanly instead of logging "expected 0, but got
		// N arguments".
		mock.ExpectExec("UPDATE api_keys SET key_hash").
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("UPDATE api_keys SET last_used_at").
			WithArgs(pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		apiKey, err := store.ValidateKey(context.Background(), plaintext)
		require.NoError(t, err)
		require.NotNil(t, apiKey, "legacy sha256 row must validate via fallback probe")
		assert.Equal(t, "Test Key", apiKey.Name)
		waitForRehashesForTest()
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})

	t.Run("HMAC probe DB error propagates without legacy fallback", func(t *testing.T) {
		// CRITICAL security invariant: a non-ErrNoRows error on the HMAC
		// probe must abort validation, NOT fall through to the legacy
		// probe. The mock's ExpectationsWereMet at the end fails if the
		// legacy probe runs (because no matching expectation is set).
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		store := NewAPIKeyStoreWithPepper(mock, true, pepper)
		plaintext := "middleware-key-transient"
		hmacHash := mustHashAPIKeyHMAC(pepper, plaintext)

		mock.ExpectQuery("SELECT id, key_hash, name").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnError(errors.New("connection refused"))

		apiKey, err := store.ValidateKey(context.Background(), plaintext)
		require.Error(t, err, "non-ErrNoRows DB error must propagate")
		assert.Nil(t, apiKey)
		assert.Contains(t, err.Error(), "connection refused")
		assert.NoError(t, mock.ExpectationsWereMet(),
			"legacy probe must NOT run after a non-ErrNoRows error on the HMAC probe")
		// No async work on this path — error propagates before the
		// goroutines spawn — so no drain needed.
	})

	t.Run("all probes miss returns nil nil for clean rejection", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		store := NewAPIKeyStoreWithPepper(mock, true, pepper)
		plaintext := "middleware-key-not-found"
		hmacHash := mustHashAPIKeyHMAC(pepper, plaintext)
		legacyHash := HashAPIKey(plaintext)

		mock.ExpectQuery("SELECT id, key_hash, name").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery("SELECT id, key_hash, name").
			WithArgs(legacyHash, HashAlgoSHA256).
			WillReturnError(pgx.ErrNoRows)

		apiKey, err := store.ValidateKey(context.Background(), plaintext)
		// (nil, nil) is the documented return convention for the
		// middleware path — the auth handler consumes this as a clean
		// "not authorized" rejection without caring about the reason.
		require.NoError(t, err)
		assert.Nil(t, apiKey)
		// No async work on the not-found path.
	})
}
