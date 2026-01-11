// Package api provides HTTP handlers for API key management.
//
// This file implements TB-006: API Key Expiration and Rotation HTTP endpoints
//
// Endpoints:
//   - POST /api/v1/keys - Create a new API key
//   - GET /api/v1/keys - List API keys for authenticated user
//   - GET /api/v1/keys/:id - Get details of a specific key
//   - POST /api/v1/keys/:id/rotate - Rotate an existing key
//   - DELETE /api/v1/keys/:id - Revoke a key
package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Query parameter value constants
const (
	queryParamTrue = "true"
)

// KeysHandler handles API key management HTTP requests
type KeysHandler struct {
	keyManager *KeyManager
}

// NewKeysHandler creates a new KeysHandler
func NewKeysHandler(db *pgxpool.Pool) *KeysHandler {
	return &KeysHandler{
		keyManager: NewKeyManager(db),
	}
}

// GetKeyManager returns the underlying key manager for use by other components
func (h *KeysHandler) GetKeyManager() *KeyManager {
	return h.keyManager
}

// CreateKeyRequest represents a request to create a new API key
type CreateKeyRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=255"`
	Permissions []string `json:"permissions"`          // Optional, defaults to ["read:decisions"]
	ExpiresIn   string   `json:"expires_in,omitempty"` // Optional duration string (e.g., "720h" for 30 days)
	ExpiresAt   string   `json:"expires_at,omitempty"` // Optional RFC3339 timestamp
}

// CreateKeyResponse represents the response when creating a new API key
type CreateKeyResponse struct {
	ID          string     `json:"id"`
	Key         string     `json:"key"` // Plaintext key - shown only once
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Warning     string     `json:"warning,omitempty"`
}

// RotateKeyRequest represents a request to rotate an API key
type RotateKeyRequest struct {
	ExpiresIn string `json:"expires_in,omitempty"` // Optional new duration
	ExpiresAt string `json:"expires_at,omitempty"` // Optional new timestamp
}

// RotateKeyResponse represents the response when rotating an API key
type RotateKeyResponse struct {
	ID          string     `json:"id"`
	Key         string     `json:"key"` // Plaintext key - shown only once
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RotatedFrom string     `json:"rotated_from"`
	Warning     string     `json:"warning,omitempty"`
}

// KeyResponse represents the response for a single API key (without plaintext)
type KeyResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	IsActive    bool       `json:"is_active"`
	Revoked     bool       `json:"revoked"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RotatedFrom *string    `json:"rotated_from,omitempty"`
}

// ListKeysResponse represents the response for listing API keys
type ListKeysResponse struct {
	Keys  []*KeyResponse `json:"keys"`
	Count int            `json:"count"`
}

// RegisterRoutes registers the API key management routes
func (h *KeysHandler) RegisterRoutes(router *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	keys := router.Group("/keys")
	keys.Use(authMiddleware) // All key management requires authentication
	{
		keys.POST("", h.HandleCreateKey)
		keys.GET("", h.HandleListKeys)
		keys.GET("/:id", h.HandleGetKey)
		keys.POST("/:id/rotate", h.HandleRotateKey)
		keys.DELETE("/:id", h.HandleRevokeKey)
		keys.GET("/:id/history", h.HandleGetKeyHistory)
	}
}

// RegisterRoutesWithRateLimiter registers routes with rate limiting
func (h *KeysHandler) RegisterRoutesWithRateLimiter(
	router *gin.RouterGroup,
	authMiddleware gin.HandlerFunc,
	readLimiter gin.HandlerFunc,
	writeLimiter gin.HandlerFunc,
) {
	keys := router.Group("/keys")
	keys.Use(authMiddleware)
	{
		keys.POST("", writeLimiter, h.HandleCreateKey)
		keys.GET("", readLimiter, h.HandleListKeys)
		keys.GET("/:id", readLimiter, h.HandleGetKey)
		keys.POST("/:id/rotate", writeLimiter, h.HandleRotateKey)
		keys.DELETE("/:id", writeLimiter, h.HandleRevokeKey)
		keys.GET("/:id/history", readLimiter, h.HandleGetKeyHistory)
	}
}

// HandleCreateKey handles POST /api/v1/keys
func (h *KeysHandler) HandleCreateKey(c *gin.Context) {
	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	// Parse expiration
	var expiresIn time.Duration
	if req.ExpiresIn != "" {
		var err error
		expiresIn, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid expires_in format",
				"details": "Use duration format like '720h' for 30 days",
			})
			return
		}
		if expiresIn < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "expires_in must be positive",
			})
			return
		}
	} else if req.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid expires_at format",
				"details": "Use RFC3339 format like '2024-12-31T23:59:59Z'",
			})
			return
		}
		if expiresAt.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "expires_at must be in the future",
			})
			return
		}
		expiresIn = time.Until(expiresAt)
	}

	// Create the key
	ctx := c.Request.Context()
	key, err := h.keyManager.CreateAPIKey(ctx, userID.(string), req.Name, req.Permissions, expiresIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create API key",
			"details": err.Error(),
		})
		return
	}

	response := CreateKeyResponse{
		ID:          key.ID.String(),
		Key:         key.Key,
		Name:        key.Name,
		UserID:      key.UserID,
		Permissions: key.Permissions,
		CreatedAt:   key.CreatedAt,
		ExpiresAt:   key.ExpiresAt,
		Warning:     "Store this key securely - it will only be shown once!",
	}

	c.JSON(http.StatusCreated, response)
}

// HandleListKeys handles GET /api/v1/keys
func (h *KeysHandler) HandleListKeys(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	// Optional query param to include inactive keys
	includeInactive := c.Query("include_inactive") == queryParamTrue

	ctx := c.Request.Context()
	keys, err := h.keyManager.ListAPIKeys(ctx, userID.(string), includeInactive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to list API keys",
			"details": err.Error(),
		})
		return
	}

	// Convert to response format
	responseKeys := make([]*KeyResponse, len(keys))
	for i, k := range keys {
		responseKeys[i] = keyDetailsToResponse(k)
	}

	c.JSON(http.StatusOK, ListKeysResponse{
		Keys:  responseKeys,
		Count: len(responseKeys),
	})
}

// HandleGetKey handles GET /api/v1/keys/:id
func (h *KeysHandler) HandleGetKey(c *gin.Context) {
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key ID format",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	ctx := c.Request.Context()
	key, err := h.keyManager.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API key not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get API key",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if key.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "access denied - key belongs to another user",
		})
		return
	}

	c.JSON(http.StatusOK, keyDetailsToResponse(key))
}

// HandleRotateKey handles POST /api/v1/keys/:id/rotate
func (h *KeysHandler) HandleRotateKey(c *gin.Context) {
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key ID format",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	// Parse optional request body
	var req RotateKeyRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid request body",
				"details": err.Error(),
			})
			return
		}
	}

	ctx := c.Request.Context()

	// Verify ownership first
	oldKey, err := h.keyManager.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API key not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get API key",
			"details": err.Error(),
		})
		return
	}

	if oldKey.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "access denied - key belongs to another user",
		})
		return
	}

	// Parse new expiration
	var newExpiresIn *time.Duration
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid expires_in format",
				"details": "Use duration format like '720h' for 30 days",
			})
			return
		}
		if d < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "expires_in must be positive",
			})
			return
		}
		newExpiresIn = &d
	} else if req.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid expires_at format",
				"details": "Use RFC3339 format like '2024-12-31T23:59:59Z'",
			})
			return
		}
		if expiresAt.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "expires_at must be in the future",
			})
			return
		}
		d := time.Until(expiresAt)
		newExpiresIn = &d
	}

	// Rotate the key
	newKey, err := h.keyManager.RotateAPIKey(ctx, keyID, newExpiresIn)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, ErrKeyNotFound) {
			statusCode = http.StatusNotFound
		} else if errors.Is(err, ErrKeyRevoked) || errors.Is(err, ErrKeyInactive) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{
			"error":   "failed to rotate API key",
			"details": err.Error(),
		})
		return
	}

	response := RotateKeyResponse{
		ID:          newKey.ID.String(),
		Key:         newKey.Key,
		Name:        newKey.Name,
		UserID:      newKey.UserID,
		Permissions: newKey.Permissions,
		CreatedAt:   newKey.CreatedAt,
		ExpiresAt:   newKey.ExpiresAt,
		RotatedFrom: keyID.String(),
		Warning:     "Store this key securely - it will only be shown once! The old key has been deactivated.",
	}

	c.JSON(http.StatusOK, response)
}

// HandleRevokeKey handles DELETE /api/v1/keys/:id
func (h *KeysHandler) HandleRevokeKey(c *gin.Context) {
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key ID format",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	ctx := c.Request.Context()

	// Verify ownership first
	key, err := h.keyManager.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API key not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get API key",
			"details": err.Error(),
		})
		return
	}

	if key.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "access denied - key belongs to another user",
		})
		return
	}

	// Revoke the key
	err = h.keyManager.RevokeAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API key not found or already revoked",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to revoke API key",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key revoked successfully",
		"key_id":  keyID.String(),
	})
}

// HandleGetKeyHistory handles GET /api/v1/keys/:id/history
func (h *KeysHandler) HandleGetKeyHistory(c *gin.Context) {
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key ID format",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	ctx := c.Request.Context()

	// Get rotation history
	history, err := h.keyManager.GetKeyRotationHistory(ctx, keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get key history",
			"details": err.Error(),
		})
		return
	}

	if len(history) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found",
		})
		return
	}

	// Verify ownership (check the first key in the chain)
	if history[0].UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "access denied - key belongs to another user",
		})
		return
	}

	// Convert to response format
	responseKeys := make([]*KeyResponse, len(history))
	for i, k := range history {
		responseKeys[i] = keyDetailsToResponse(k)
	}

	c.JSON(http.StatusOK, gin.H{
		"history": responseKeys,
		"count":   len(responseKeys),
	})
}

// keyDetailsToResponse converts APIKeyDetails to KeyResponse
func keyDetailsToResponse(k *APIKeyDetails) *KeyResponse {
	response := &KeyResponse{
		ID:          k.ID.String(),
		Name:        k.Name,
		UserID:      k.UserID,
		Permissions: k.Permissions,
		IsActive:    k.IsActive,
		Revoked:     k.Revoked,
		CreatedAt:   k.CreatedAt,
		ExpiresAt:   k.ExpiresAt,
		LastUsedAt:  k.LastUsedAt,
	}
	if k.RotatedFrom != nil {
		rotatedFrom := k.RotatedFrom.String()
		response.RotatedFrom = &rotatedFrom
	}
	return response
}
