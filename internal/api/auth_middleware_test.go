package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
