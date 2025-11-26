// Package api provides HTTP API handlers and middleware for the CryptoFunk trading system.
//
// # Authentication Middleware
//
// This package includes a complete API key authentication system (auth_middleware.go)
// that provides:
//   - API key validation via SHA-256 hashing
//   - Permission-based authorization
//   - Configurable authentication (enabled/disabled via config)
//   - Support for X-API-Key header and Authorization: Bearer tokens
//
// # Enabling Authentication
//
// To enable authentication for decision endpoints:
//
//  1. Run migration 009_api_keys.sql to create the api_keys table
//  2. Set api.auth.enabled = true in config.yaml
//  3. Create API keys using the create_api_key() PostgreSQL function
//  4. Wire up AuthMiddleware in cmd/api/main.go setupRoutes()
//
// Example configuration (config.yaml):
//
//	api:
//	  auth:
//	    enabled: true
//	    header_name: "X-API-Key"
//	    require_https: true
//
// The auth middleware is currently NOT enabled by default to allow for easier
// development and testing. Enable it before production deployment.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// APIKey represents an API key stored in the database
type APIKey struct {
	ID          uuid.UUID  `json:"id"`
	KeyHash     string     `json:"-"` // Never expose the hash
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Revoked     bool       `json:"revoked"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	HeaderName   string `mapstructure:"header_name"`   // Default: "X-API-Key" or "Authorization"
	RequireHTTPS bool   `mapstructure:"require_https"` // Require HTTPS in production
}

// DefaultAuthConfig returns the default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:      false, // Disabled by default for development
		HeaderName:   "X-API-Key",
		RequireHTTPS: true,
	}
}

// APIKeyStore handles API key storage and validation
type APIKeyStore struct {
	db      *pgxpool.Pool
	enabled bool
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore(db *pgxpool.Pool, enabled bool) *APIKeyStore {
	return &APIKeyStore{
		db:      db,
		enabled: enabled,
	}
}

// HashAPIKey creates a SHA-256 hash of an API key
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ValidateKey checks if an API key is valid and returns the associated key record
func (s *APIKeyStore) ValidateKey(ctx context.Context, key string) (*APIKey, error) {
	if s.db == nil {
		return nil, nil
	}

	keyHash := HashAPIKey(key)

	query := `
		SELECT id, key_hash, name, user_id, permissions, last_used_at,
		       created_at, expires_at, revoked
		FROM api_keys
		WHERE key_hash = $1
	`

	var apiKey APIKey
	var permissions []byte

	err := s.db.QueryRow(ctx, query, keyHash).Scan(
		&apiKey.ID,
		&apiKey.KeyHash,
		&apiKey.Name,
		&apiKey.UserID,
		&permissions,
		&apiKey.LastUsedAt,
		&apiKey.CreatedAt,
		&apiKey.ExpiresAt,
		&apiKey.Revoked,
	)

	if err != nil {
		return nil, err // Key not found or DB error
	}

	// Unmarshal permissions JSON into slice
	if len(permissions) > 0 {
		if err := json.Unmarshal(permissions, &apiKey.Permissions); err != nil {
			return nil, fmt.Errorf("invalid permissions JSON: %w", err)
		}
	}

	// Check if key is revoked
	if apiKey.Revoked {
		return nil, nil
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}

	// Update last used timestamp asynchronously with timeout context
	// Using a detached context with timeout to avoid leaking the request context
	apiKeyID := apiKey.ID // Capture value to avoid closure over pointer
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		updateQuery := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
		_, _ = s.db.Exec(updateCtx, updateQuery, apiKeyID)
	}()

	return &apiKey, nil
}

// AuthMiddleware creates a Gin middleware that validates API keys
// When auth is disabled, it allows all requests through
// When enabled, it requires a valid API key in the configured header
func AuthMiddleware(store *APIKeyStore, config *AuthConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultAuthConfig()
	}

	return func(c *gin.Context) {
		// If auth is disabled, allow all requests
		if !config.Enabled || !store.enabled {
			c.Next()
			return
		}

		// Check HTTPS requirement in production
		if config.RequireHTTPS && c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			// Allow localhost for development
			host := c.Request.Host
			if !strings.HasPrefix(host, "localhost") && !strings.HasPrefix(host, "127.0.0.1") {
				log.Warn().
					Str("host", host).
					Str("ip", c.ClientIP()).
					Msg("Auth: HTTPS required but request is HTTP")
				c.JSON(http.StatusForbidden, gin.H{
					"error": "HTTPS required for API access",
				})
				c.Abort()
				return
			}
		}

		// Extract API key from header
		var apiKey string

		// Try configured header first
		apiKey = c.GetHeader(config.HeaderName)

		// If not found, try Authorization: Bearer header
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// No API key provided
		if apiKey == "" {
			log.Debug().
				Str("ip", c.ClientIP()).
				Str("path", c.Request.URL.Path).
				Msg("Auth: No API key provided")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "API key required",
				"message": "Provide API key via X-API-Key header or Authorization: Bearer <key>",
			})
			c.Abort()
			return
		}

		// Validate the API key
		keyRecord, err := store.ValidateKey(c.Request.Context(), apiKey)
		if err != nil {
			log.Error().Err(err).
				Str("ip", c.ClientIP()).
				Msg("Auth: Error validating API key")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication error",
			})
			c.Abort()
			return
		}

		if keyRecord == nil {
			log.Warn().
				Str("ip", c.ClientIP()).
				Str("path", c.Request.URL.Path).
				Msg("Auth: Invalid or expired API key")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired API key",
			})
			c.Abort()
			return
		}

		// Set user context for downstream handlers and audit logging
		c.Set("user_id", keyRecord.UserID)
		c.Set("api_key_id", keyRecord.ID.String())
		c.Set("api_key_name", keyRecord.Name)
		c.Set("permissions", keyRecord.Permissions)

		log.Debug().
			Str("user_id", keyRecord.UserID).
			Str("key_name", keyRecord.Name).
			Str("path", c.Request.URL.Path).
			Msg("Auth: Request authenticated")

		c.Next()
	}
}

// RequirePermission creates middleware that checks if the authenticated user has a specific permission
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions, exists := c.Get("permissions")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
			c.Abort()
			return
		}

		perms, ok := permissions.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Permission check failed",
			})
			c.Abort()
			return
		}

		// Check for the required permission or wildcard
		hasPermission := false
		for _, p := range perms {
			if p == permission || p == "*" || p == "admin" {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			log.Warn().
				Str("required", permission).
				Strs("has", perms).
				Str("path", c.Request.URL.Path).
				Msg("Auth: Permission denied")
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient permissions",
				"required": permission,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuth creates middleware that validates API keys if provided but doesn't require them
// Useful for endpoints that provide enhanced functionality for authenticated users
func OptionalAuth(store *APIKeyStore, config *AuthConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultAuthConfig()
	}

	return func(c *gin.Context) {
		if !store.enabled {
			c.Next()
			return
		}

		// Try to extract API key
		var apiKey string
		apiKey = c.GetHeader(config.HeaderName)
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// No key provided - continue without authentication
		if apiKey == "" {
			c.Next()
			return
		}

		// Validate the key if provided
		keyRecord, err := store.ValidateKey(c.Request.Context(), apiKey)
		if err != nil || keyRecord == nil {
			// Invalid key provided - reject (they tried to auth but failed)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", keyRecord.UserID)
		c.Set("api_key_id", keyRecord.ID.String())
		c.Set("api_key_name", keyRecord.Name)
		c.Set("permissions", keyRecord.Permissions)

		c.Next()
	}
}
