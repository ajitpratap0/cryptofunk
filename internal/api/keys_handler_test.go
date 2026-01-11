package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// =============================================================================
// Mock KeyManager
// =============================================================================

// MockKeyManager implements KeyManagerInterface for testing handlers
type MockKeyManager struct {
	CreateAPIKeyFunc          func(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error)
	GetAPIKeyFunc             func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error)
	ListAPIKeysFunc           func(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error)
	RotateAPIKeyFunc          func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error)
	RevokeAPIKeyFunc          func(ctx context.Context, keyID uuid.UUID) error
	GetKeyRotationHistoryFunc func(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error)
}

// Ensure MockKeyManager implements KeyManagerInterface
var _ KeyManagerInterface = (*MockKeyManager)(nil)

func (m *MockKeyManager) CreateAPIKey(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error) {
	if m.CreateAPIKeyFunc != nil {
		return m.CreateAPIKeyFunc(ctx, userID, name, permissions, expiresIn)
	}
	return nil, errors.New("CreateAPIKeyFunc not implemented")
}

func (m *MockKeyManager) GetAPIKey(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
	if m.GetAPIKeyFunc != nil {
		return m.GetAPIKeyFunc(ctx, keyID)
	}
	return nil, errors.New("GetAPIKeyFunc not implemented")
}

func (m *MockKeyManager) ListAPIKeys(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error) {
	if m.ListAPIKeysFunc != nil {
		return m.ListAPIKeysFunc(ctx, userID, includeInactive)
	}
	return nil, errors.New("ListAPIKeysFunc not implemented")
}

func (m *MockKeyManager) RotateAPIKey(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
	if m.RotateAPIKeyFunc != nil {
		return m.RotateAPIKeyFunc(ctx, keyID, newExpiresIn)
	}
	return nil, errors.New("RotateAPIKeyFunc not implemented")
}

func (m *MockKeyManager) RevokeAPIKey(ctx context.Context, keyID uuid.UUID) error {
	if m.RevokeAPIKeyFunc != nil {
		return m.RevokeAPIKeyFunc(ctx, keyID)
	}
	return errors.New("RevokeAPIKeyFunc not implemented")
}

func (m *MockKeyManager) GetKeyRotationHistory(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error) {
	if m.GetKeyRotationHistoryFunc != nil {
		return m.GetKeyRotationHistoryFunc(ctx, keyID)
	}
	return nil, errors.New("GetKeyRotationHistoryFunc not implemented")
}

// StartCleanupWorker is a no-op for testing
func (m *MockKeyManager) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	// No-op for testing
}

// StopCleanupWorker is a no-op for testing
func (m *MockKeyManager) StopCleanupWorker() {
	// No-op for testing
}

// =============================================================================
// Test Setup Helpers
// =============================================================================

// setupKeysTestRouter creates a router with the keys handler using a mock
func setupKeysTestRouter(mock *MockKeyManager) (*gin.Engine, *KeysHandler) {
	router := gin.New()
	handler := NewKeysHandlerWithKeyManager(mock)

	// Setup auth middleware that always authenticates
	authMiddleware := func(c *gin.Context) {
		// Set user_id in context (simulating authenticated user)
		userID := c.GetHeader("X-Test-User-ID")
		if userID == "" {
			userID = "test-user"
		}
		c.Set("user_id", userID)
		c.Next()
	}

	keys := router.Group("/api/v1/keys")
	keys.Use(authMiddleware)
	{
		keys.POST("", handler.HandleCreateKey)
		keys.GET("", handler.HandleListKeys)
		keys.GET("/:id", handler.HandleGetKey)
		keys.POST("/:id/rotate", handler.HandleRotateKey)
		keys.DELETE("/:id", handler.HandleRevokeKey)
		keys.GET("/:id/history", handler.HandleGetKeyHistory)
	}

	return router, handler
}

// setupKeysTestRouterNoAuth creates a router without auth middleware
func setupKeysTestRouterNoAuth(mock *MockKeyManager) (*gin.Engine, *KeysHandler) {
	router := gin.New()
	handler := NewKeysHandlerWithKeyManager(mock)

	keys := router.Group("/api/v1/keys")
	{
		keys.POST("", handler.HandleCreateKey)
		keys.GET("", handler.HandleListKeys)
		keys.GET("/:id", handler.HandleGetKey)
		keys.POST("/:id/rotate", handler.HandleRotateKey)
		keys.DELETE("/:id", handler.HandleRevokeKey)
		keys.GET("/:id/history", handler.HandleGetKeyHistory)
	}

	return router, handler
}

// =============================================================================
// CreateKey Handler Tests
// =============================================================================

func TestHandleCreateKey(t *testing.T) {
	testUserID := "test-user"
	testKeyID := uuid.New()
	testCreatedAt := time.Now()

	t.Run("create key with name only", func(t *testing.T) {
		mock := &MockKeyManager{
			CreateAPIKeyFunc: func(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          testKeyID,
					Key:         "test-plaintext-key-12345",
					Name:        name,
					UserID:      userID,
					Permissions: []string{"read:decisions"},
					CreatedAt:   testCreatedAt,
					ExpiresAt:   nil,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name": "My API Key",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response CreateKeyResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "My API Key", response.Name)
		assert.Equal(t, testUserID, response.UserID)
		assert.NotEmpty(t, response.Key)
		assert.NotEmpty(t, response.Warning)
	})

	t.Run("create key with custom permissions", func(t *testing.T) {
		mock := &MockKeyManager{
			CreateAPIKeyFunc: func(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          testKeyID,
					Key:         "test-plaintext-key-12345",
					Name:        name,
					UserID:      userID,
					Permissions: permissions,
					CreatedAt:   testCreatedAt,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name":        "Admin Key",
			"permissions": []string{"read:*", "write:*"},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response CreateKeyResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, []string{"read:*", "write:*"}, response.Permissions)
	})

	t.Run("create key with expires_in", func(t *testing.T) {
		expiresAt := testCreatedAt.Add(720 * time.Hour)
		mock := &MockKeyManager{
			CreateAPIKeyFunc: func(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          testKeyID,
					Key:         "test-plaintext-key-12345",
					Name:        name,
					UserID:      userID,
					Permissions: permissions,
					CreatedAt:   testCreatedAt,
					ExpiresAt:   &expiresAt,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name":       "Expiring Key",
			"expires_in": "720h",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response CreateKeyResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.ExpiresAt)
	})

	t.Run("create key with expires_at", func(t *testing.T) {
		futureTime := time.Now().Add(30 * 24 * time.Hour)
		mock := &MockKeyManager{
			CreateAPIKeyFunc: func(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          testKeyID,
					Key:         "test-plaintext-key-12345",
					Name:        name,
					UserID:      userID,
					Permissions: permissions,
					CreatedAt:   testCreatedAt,
					ExpiresAt:   &futureTime,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name":       "Expiring Key",
			"expires_at": futureTime.Format(time.RFC3339),
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("missing name returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid expires_in format returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name":       "Test Key",
			"expires_in": "invalid",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "invalid expires_in format")
	})

	t.Run("negative expires_in returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name":       "Test Key",
			"expires_in": "-1h",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "expires_in must be positive")
	})

	t.Run("invalid expires_at format returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name":       "Test Key",
			"expires_at": "invalid-date",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "invalid expires_at format")
	})

	t.Run("past expires_at returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		pastTime := time.Now().Add(-24 * time.Hour)
		body := map[string]interface{}{
			"name":       "Test Key",
			"expires_at": pastTime.Format(time.RFC3339),
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "expires_at must be in the future")
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouterNoAuth(mock)

		body := map[string]interface{}{
			"name": "Test Key",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("database error returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			CreateAPIKeyFunc: func(ctx context.Context, userID, name string, permissions []string, expiresIn time.Duration) (*CreatedAPIKey, error) {
				return nil, errors.New("database connection failed")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"name": "Test Key",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// ListKeys Handler Tests
// =============================================================================

func TestHandleListKeys(t *testing.T) {
	testUserID := "test-user"
	testKeyID1 := uuid.New()
	testKeyID2 := uuid.New()
	testCreatedAt := time.Now()

	testKeys := []*APIKeyDetails{
		{
			ID:          testKeyID1,
			Name:        "Key 1",
			UserID:      testUserID,
			Permissions: []string{"read:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   testCreatedAt,
		},
		{
			ID:          testKeyID2,
			Name:        "Key 2",
			UserID:      testUserID,
			Permissions: []string{"read:*", "write:*"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   testCreatedAt.Add(-24 * time.Hour),
		},
	}

	t.Run("list active keys only", func(t *testing.T) {
		mock := &MockKeyManager{
			ListAPIKeysFunc: func(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error) {
				if !includeInactive {
					return testKeys, nil
				}
				return testKeys, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response ListKeysResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 2, response.Count)
		assert.Len(t, response.Keys, 2)
	})

	t.Run("list including inactive keys", func(t *testing.T) {
		inactiveKey := &APIKeyDetails{
			ID:          uuid.New(),
			Name:        "Inactive Key",
			UserID:      testUserID,
			Permissions: []string{"read:decisions"},
			IsActive:    false,
			Revoked:     true,
			CreatedAt:   testCreatedAt,
		}
		allKeys := append(testKeys, inactiveKey)

		mock := &MockKeyManager{
			ListAPIKeysFunc: func(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error) {
				if includeInactive {
					return allKeys, nil
				}
				return testKeys, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys?include_inactive=true", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response ListKeysResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 3, response.Count)
	})

	t.Run("empty list returns empty array", func(t *testing.T) {
		mock := &MockKeyManager{
			ListAPIKeysFunc: func(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error) {
				return []*APIKeyDetails{}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response ListKeysResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 0, response.Count)
		assert.Empty(t, response.Keys)
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouterNoAuth(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("database error returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			ListAPIKeysFunc: func(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error) {
				return nil, errors.New("database error")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// GetKey Handler Tests
// =============================================================================

func TestHandleGetKey(t *testing.T) {
	testUserID := "test-user"
	testKeyID := uuid.New()
	testCreatedAt := time.Now()

	testKey := &APIKeyDetails{
		ID:          testKeyID,
		Name:        "Test Key",
		UserID:      testUserID,
		Permissions: []string{"read:decisions"},
		IsActive:    true,
		Revoked:     false,
		CreatedAt:   testCreatedAt,
	}

	t.Run("get existing key", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				if keyID == testKeyID {
					return testKey, nil
				}
				return nil, ErrKeyNotFound
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response KeyResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, testKeyID.String(), response.ID)
		assert.Equal(t, "Test Key", response.Name)
	})

	t.Run("invalid key ID format", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/invalid-uuid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "invalid key ID format")
	})

	t.Run("key not found", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return nil, ErrKeyNotFound
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+uuid.New().String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("access denied for other user's key", func(t *testing.T) {
		otherUserKey := &APIKeyDetails{
			ID:          testKeyID,
			Name:        "Other User Key",
			UserID:      "other-user",
			Permissions: []string{"read:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   testCreatedAt,
		}

		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return otherUserKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "access denied")
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouterNoAuth(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("database error returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return nil, errors.New("database error")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// RotateKey Handler Tests
// =============================================================================

func TestHandleRotateKey(t *testing.T) {
	testUserID := "test-user"
	testKeyID := uuid.New()
	newKeyID := uuid.New()
	testCreatedAt := time.Now()

	testKey := &APIKeyDetails{
		ID:          testKeyID,
		Name:        "Test Key",
		UserID:      testUserID,
		Permissions: []string{"read:decisions"},
		IsActive:    true,
		Revoked:     false,
		CreatedAt:   testCreatedAt,
	}

	t.Run("rotate key without body", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				if keyID == testKeyID {
					return testKey, nil
				}
				return nil, ErrKeyNotFound
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          newKeyID,
					Key:         "new-plaintext-key-12345",
					Name:        "Test Key (rotated)",
					UserID:      testUserID,
					Permissions: []string{"read:decisions"},
					CreatedAt:   time.Now(),
					RotatedFrom: &testKeyID,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response RotateKeyResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, newKeyID.String(), response.ID)
		assert.NotEmpty(t, response.Key)
		assert.Equal(t, testKeyID.String(), response.RotatedFrom)
		assert.NotEmpty(t, response.Warning)
	})

	t.Run("rotate key with new expires_in", func(t *testing.T) {
		expiresAt := time.Now().Add(720 * time.Hour)
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          newKeyID,
					Key:         "new-plaintext-key-12345",
					Name:        "Test Key (rotated)",
					UserID:      testUserID,
					Permissions: []string{"read:decisions"},
					CreatedAt:   time.Now(),
					ExpiresAt:   &expiresAt,
					RotatedFrom: &testKeyID,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"expires_in": "720h",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response RotateKeyResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.ExpiresAt)
	})

	t.Run("rotate key with expires_at", func(t *testing.T) {
		futureTime := time.Now().Add(30 * 24 * time.Hour)
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return &CreatedAPIKey{
					ID:          newKeyID,
					Key:         "new-plaintext-key-12345",
					Name:        "Test Key (rotated)",
					UserID:      testUserID,
					Permissions: []string{"read:decisions"},
					CreatedAt:   time.Now(),
					ExpiresAt:   &futureTime,
					RotatedFrom: &testKeyID,
				}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"expires_at": futureTime.Format(time.RFC3339),
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid key ID format", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/invalid-uuid/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("key not found", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return nil, ErrKeyNotFound
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+uuid.New().String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("access denied for other user's key", func(t *testing.T) {
		otherUserKey := &APIKeyDetails{
			ID:          testKeyID,
			Name:        "Other User Key",
			UserID:      "other-user",
			Permissions: []string{"read:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   testCreatedAt,
		}

		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return otherUserKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("rotate revoked key returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return nil, ErrKeyRevoked
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rotate inactive key returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return nil, ErrKeyInactive
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid expires_in returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"expires_in": "invalid",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("negative expires_in returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"expires_in": "-1h",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid expires_at returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		body := map[string]interface{}{
			"expires_at": "invalid-date",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("past expires_at returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		pastTime := time.Now().Add(-24 * time.Hour)
		body := map[string]interface{}{
			"expires_at": pastTime.Format(time.RFC3339),
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouterNoAuth(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("database error on get returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return nil, errors.New("database connection failed")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("rotate key not found on rotate returns not found", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return nil, ErrKeyNotFound
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("rotate database error returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RotateAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID, newExpiresIn *time.Duration) (*CreatedAPIKey, error) {
				return nil, errors.New("database error during rotation")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("invalid json body returns bad request", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		// Send invalid JSON
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/keys/"+testKeyID.String()+"/rotate", bytes.NewBufferString("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "invalid request body")
	})
}

// =============================================================================
// RevokeKey Handler Tests
// =============================================================================

func TestHandleRevokeKey(t *testing.T) {
	testUserID := "test-user"
	testKeyID := uuid.New()
	testCreatedAt := time.Now()

	testKey := &APIKeyDetails{
		ID:          testKeyID,
		Name:        "Test Key",
		UserID:      testUserID,
		Permissions: []string{"read:decisions"},
		IsActive:    true,
		Revoked:     false,
		CreatedAt:   testCreatedAt,
	}

	t.Run("revoke key successfully", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				if keyID == testKeyID {
					return testKey, nil
				}
				return nil, ErrKeyNotFound
			},
			RevokeAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) error {
				return nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["message"], "revoked successfully")
		assert.Equal(t, testKeyID.String(), response["key_id"])
	})

	t.Run("invalid key ID format", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/invalid-uuid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("key not found on get", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return nil, ErrKeyNotFound
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+uuid.New().String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("key not found on revoke", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RevokeAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) error {
				return ErrKeyNotFound
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("access denied for other user's key", func(t *testing.T) {
		otherUserKey := &APIKeyDetails{
			ID:          testKeyID,
			Name:        "Other User Key",
			UserID:      "other-user",
			Permissions: []string{"read:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   testCreatedAt,
		}

		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return otherUserKey, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouterNoAuth(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("database error on get returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return nil, errors.New("database error")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("database error on revoke returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			GetAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
				return testKey, nil
			},
			RevokeAPIKeyFunc: func(ctx context.Context, keyID uuid.UUID) error {
				return errors.New("database error")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/keys/"+testKeyID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// GetKeyHistory Handler Tests
// =============================================================================

func TestHandleGetKeyHistory(t *testing.T) {
	testUserID := "test-user"
	testKeyID := uuid.New()
	oldKeyID := uuid.New()
	testCreatedAt := time.Now()

	testHistory := []*APIKeyDetails{
		{
			ID:          testKeyID,
			Name:        "Current Key (rotated)",
			UserID:      testUserID,
			Permissions: []string{"read:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   testCreatedAt,
			RotatedFrom: &oldKeyID,
		},
		{
			ID:          oldKeyID,
			Name:        "Original Key",
			UserID:      testUserID,
			Permissions: []string{"read:decisions"},
			IsActive:    false,
			Revoked:     false,
			CreatedAt:   testCreatedAt.Add(-24 * time.Hour),
		},
	}

	t.Run("get key history successfully", func(t *testing.T) {
		mock := &MockKeyManager{
			GetKeyRotationHistoryFunc: func(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error) {
				if keyID == testKeyID {
					return testHistory, nil
				}
				return nil, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String()+"/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(2), response["count"])
		history := response["history"].([]interface{})
		assert.Len(t, history, 2)
	})

	t.Run("invalid key ID format", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/invalid-uuid/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("key not found (empty history)", func(t *testing.T) {
		mock := &MockKeyManager{
			GetKeyRotationHistoryFunc: func(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error) {
				return []*APIKeyDetails{}, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+uuid.New().String()+"/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("access denied for other user's key", func(t *testing.T) {
		otherUserHistory := []*APIKeyDetails{
			{
				ID:          testKeyID,
				Name:        "Other User Key",
				UserID:      "other-user",
				Permissions: []string{"read:decisions"},
				IsActive:    true,
				Revoked:     false,
				CreatedAt:   testCreatedAt,
			},
		}

		mock := &MockKeyManager{
			GetKeyRotationHistoryFunc: func(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error) {
				return otherUserHistory, nil
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String()+"/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		mock := &MockKeyManager{}
		router, _ := setupKeysTestRouterNoAuth(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String()+"/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("database error returns internal server error", func(t *testing.T) {
		mock := &MockKeyManager{
			GetKeyRotationHistoryFunc: func(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error) {
				return nil, errors.New("database error")
			},
		}
		router, _ := setupKeysTestRouter(mock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/keys/"+testKeyID.String()+"/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// Request/Response Struct Tests
// =============================================================================

func TestCreateKeyRequest(t *testing.T) {
	req := CreateKeyRequest{
		Name:        "Test Key",
		Permissions: []string{"read:decisions", "write:decisions"},
		ExpiresIn:   "720h",
		ExpiresAt:   "",
	}

	assert.Equal(t, "Test Key", req.Name)
	assert.Len(t, req.Permissions, 2)
	assert.Equal(t, "720h", req.ExpiresIn)
}

func TestRotateKeyRequest(t *testing.T) {
	req := RotateKeyRequest{
		ExpiresIn: "720h",
	}

	assert.Equal(t, "720h", req.ExpiresIn)
	assert.Empty(t, req.ExpiresAt)
}

func TestCreateKeyResponse(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(720 * time.Hour)

	resp := CreateKeyResponse{
		ID:          "test-id",
		Key:         "test-key",
		Name:        "Test Key",
		UserID:      "test-user",
		Permissions: []string{"read:decisions"},
		CreatedAt:   now,
		ExpiresAt:   &expiresAt,
		Warning:     "Store this key securely!",
	}

	assert.Equal(t, "test-id", resp.ID)
	assert.Equal(t, "test-key", resp.Key)
	assert.NotEmpty(t, resp.Warning)
}

func TestRotateKeyResponse(t *testing.T) {
	now := time.Now()

	resp := RotateKeyResponse{
		ID:          "new-id",
		Key:         "new-key",
		Name:        "Key (rotated)",
		UserID:      "test-user",
		Permissions: []string{"read:decisions"},
		CreatedAt:   now,
		RotatedFrom: "old-id",
		Warning:     "Store this key securely!",
	}

	assert.Equal(t, "new-id", resp.ID)
	assert.Equal(t, "old-id", resp.RotatedFrom)
	assert.NotEmpty(t, resp.Warning)
}

func TestKeyResponse(t *testing.T) {
	now := time.Now()
	rotatedFrom := "old-key-id"

	resp := KeyResponse{
		ID:          "key-id",
		Name:        "Test Key",
		UserID:      "test-user",
		Permissions: []string{"read:decisions"},
		IsActive:    true,
		Revoked:     false,
		CreatedAt:   now,
		LastUsedAt:  &now,
		RotatedFrom: &rotatedFrom,
	}

	assert.Equal(t, "key-id", resp.ID)
	assert.True(t, resp.IsActive)
	assert.False(t, resp.Revoked)
	assert.Equal(t, &rotatedFrom, resp.RotatedFrom)
}

func TestListKeysResponse(t *testing.T) {
	keys := []*KeyResponse{
		{ID: "key-1", Name: "Key 1"},
		{ID: "key-2", Name: "Key 2"},
	}

	resp := ListKeysResponse{
		Keys:  keys,
		Count: 2,
	}

	assert.Equal(t, 2, resp.Count)
	assert.Len(t, resp.Keys, 2)
}

// =============================================================================
// keyDetailsToResponse Helper Tests
// =============================================================================

func TestKeyDetailsToResponse(t *testing.T) {
	t.Run("full key details", func(t *testing.T) {
		keyID := uuid.New()
		rotatedFrom := uuid.New()
		now := time.Now()
		expiresAt := now.Add(720 * time.Hour)

		details := &APIKeyDetails{
			ID:          keyID,
			Name:        "Test Key",
			UserID:      "test-user",
			Permissions: []string{"read:decisions", "write:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   now,
			ExpiresAt:   &expiresAt,
			LastUsedAt:  &now,
			RotatedFrom: &rotatedFrom,
		}

		resp := keyDetailsToResponse(details)

		assert.Equal(t, keyID.String(), resp.ID)
		assert.Equal(t, "Test Key", resp.Name)
		assert.Equal(t, "test-user", resp.UserID)
		assert.Len(t, resp.Permissions, 2)
		assert.True(t, resp.IsActive)
		assert.False(t, resp.Revoked)
		assert.NotNil(t, resp.ExpiresAt)
		assert.NotNil(t, resp.LastUsedAt)
		assert.NotNil(t, resp.RotatedFrom)
		assert.Equal(t, rotatedFrom.String(), *resp.RotatedFrom)
	})

	t.Run("minimal key details", func(t *testing.T) {
		keyID := uuid.New()
		now := time.Now()

		details := &APIKeyDetails{
			ID:          keyID,
			Name:        "Minimal Key",
			UserID:      "test-user",
			Permissions: []string{"read:decisions"},
			IsActive:    true,
			Revoked:     false,
			CreatedAt:   now,
			ExpiresAt:   nil,
			LastUsedAt:  nil,
			RotatedFrom: nil,
		}

		resp := keyDetailsToResponse(details)

		assert.Equal(t, keyID.String(), resp.ID)
		assert.Nil(t, resp.ExpiresAt)
		assert.Nil(t, resp.LastUsedAt)
		assert.Nil(t, resp.RotatedFrom)
	})
}

// =============================================================================
// NewKeysHandler Tests
// =============================================================================

func TestNewKeysHandler(t *testing.T) {
	// Test with nil db pool (should not panic)
	handler := NewKeysHandler(nil)
	require.NotNil(t, handler)
	require.NotNil(t, handler.keyManager)
}

func TestGetKeyManager(t *testing.T) {
	handler := NewKeysHandler(nil)
	km := handler.GetKeyManager()
	require.NotNil(t, km)
	assert.Equal(t, handler.keyManager, km)
}

// =============================================================================
// Route Registration Tests
// =============================================================================

func TestRegisterRoutes(t *testing.T) {
	handler := NewKeysHandler(nil)
	router := gin.New()

	// Create a simple auth middleware for testing
	authMiddleware := func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, authMiddleware)

	// Verify routes are registered by checking if the router has registered paths
	// We use a handler interceptor to verify the route is reachable
	routeInfo := router.Routes()

	// Check that all expected routes are registered
	expectedRoutes := map[string]string{
		"POST":   "/api/v1/keys",
		"GET":    "/api/v1/keys",
		"DELETE": "/api/v1/keys/:id",
	}

	registeredPaths := make(map[string]bool)
	for _, route := range routeInfo {
		key := route.Method + ":" + route.Path
		registeredPaths[key] = true
	}

	for method, path := range expectedRoutes {
		key := method + ":" + path
		assert.True(t, registeredPaths[key], "Expected route %s %s to be registered", method, path)
	}
}

func TestRegisterRoutesWithRateLimiter(t *testing.T) {
	handler := NewKeysHandler(nil)
	router := gin.New()

	authMiddleware := func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	}

	readLimiter := func(c *gin.Context) {
		c.Next()
	}

	writeLimiter := func(c *gin.Context) {
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutesWithRateLimiter(v1, authMiddleware, readLimiter, writeLimiter)

	// Verify routes are registered
	routeInfo := router.Routes()

	// Check that all expected routes are registered
	expectedRoutes := map[string]string{
		"POST":   "/api/v1/keys",
		"GET":    "/api/v1/keys",
		"DELETE": "/api/v1/keys/:id",
	}

	registeredPaths := make(map[string]bool)
	for _, route := range routeInfo {
		key := route.Method + ":" + route.Path
		registeredPaths[key] = true
	}

	for method, path := range expectedRoutes {
		key := method + ":" + path
		assert.True(t, registeredPaths[key], "Expected route %s %s to be registered", method, path)
	}
}

// =============================================================================
// NewKeysHandlerWithKeyManager Tests
// =============================================================================

func TestNewKeysHandlerWithKeyManager(t *testing.T) {
	mock := &MockKeyManager{}
	handler := NewKeysHandlerWithKeyManager(mock)
	require.NotNil(t, handler)
	require.NotNil(t, handler.keyManager)
	assert.Equal(t, mock, handler.keyManager)
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestQueryParamConstant(t *testing.T) {
	assert.Equal(t, "true", queryParamTrue)
}
