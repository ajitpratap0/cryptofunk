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
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	IsActive    bool       `json:"is_active"` // TB-006: Active status for rotation support
}

// AuthConfig contains authentication configuration used by AuthMiddleware.
// Note: the HMAC key pepper is configured on the APIKeyStore itself via
// NewAPIKeyStoreWithPepper, not through this struct — AuthMiddleware
// consumes a store argument directly, so threading the pepper through
// both would be dead config.
type AuthConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	HeaderName          string `mapstructure:"header_name"`           // Default: "X-API-Key" or "Authorization"
	RequireHTTPS        bool   `mapstructure:"require_https"`         // Require HTTPS in production
	TrustForwardedProto bool   `mapstructure:"trust_forwarded_proto"` // Trust X-Forwarded-Proto from reverse proxy (default false)
}

// DefaultAuthConfig returns the default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:             false, // Disabled by default for development
		HeaderName:          "X-API-Key",
		RequireHTTPS:        true,
		TrustForwardedProto: false, // Only enable behind a trusted reverse proxy (K8s ingress, ALB)
		// KeyPepper intentionally omitted — see type doc.
	}
}

// Hash-algorithm marker strings stored in api_keys.hash_algorithm. See
// migrations/024_api_keys_hash_algorithm.sql.
const (
	HashAlgoSHA256     = "sha256"      // Legacy raw SHA-256, no pepper.
	HashAlgoHMACSHA256 = "hmac-sha256" // HMAC-SHA256 with server-side pepper.
)

// hashProbe is one (hash, algorithm) pair probed during API key
// validation. Shared by APIKeyStore.ValidateKey (auth_middleware.go) and
// KeyManager.ValidateAPIKey (keys.go) so the dual-probe pattern lives in
// one type — extending one path doesn't leave the other stale.
type hashProbe struct {
	hash      string
	algorithm string
}

// APIKeyStore handles API key storage and validation
type APIKeyStore struct {
	db      *pgxpool.Pool
	enabled bool
	pepper  string // HMAC pepper for new keys; empty = legacy SHA-256 only
}

// rehashLegacyKey is a thin wrapper that delegates to the package-level
// rehashAPIKeyHelper so APIKeyStore and KeyManager share one rehash
// implementation instead of each carrying a copy.
func (s *APIKeyStore) rehashLegacyKey(keyID uuid.UUID, plaintextKey string) {
	rehashAPIKeyAsync(s.db, s.pepper, keyID, plaintextKey)
}

// rehashAPIKeyAsync fires an async UPDATE that replaces a legacy
// sha256-hashed key_hash row with the HMAC-SHA256 equivalent, so the row
// stops being vulnerable to precomputed rainbow tables (SEC-009 / #123).
// Shared by APIKeyStore.rehashLegacyKey (auth middleware hot path) and
// KeyManager.rehashLegacyKey (key-management endpoint path) — keeping the
// SQL, logging, and context lifetime in one place.
//
// The function returns immediately after spawning the goroutine. Errors
// are logged rather than returned because this is best-effort migration:
// the legacy hash still works if we don't manage to upgrade it, and the
// same key will be retried on the next successful validation.
func rehashAPIKeyAsync(db rehashExecutor, pepper string, keyID uuid.UUID, plaintextKey string) {
	if pepper == "" || db == nil {
		return
	}
	newHash := HashAPIKeyHMAC(pepper, plaintextKey)
	// gosec G118 and contextcheck suppressed: the rehash is intentionally
	// detached from the request context so it completes even if the
	// caller cancels. The 5s timeout below bounds the work independently.
	go func(id uuid.UUID, hash string) { //nolint:gosec,contextcheck
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := db.Exec(ctx, `
			UPDATE api_keys
			SET key_hash = $1, hash_algorithm = $2
			WHERE id = $3 AND hash_algorithm = $4
		`, hash, HashAlgoHMACSHA256, id, HashAlgoSHA256)
		if err != nil {
			log.Warn().Err(err).Str("key_id", id.String()).Msg("Failed to rehash legacy API key to HMAC")
			return
		}
		log.Info().Str("key_id", id.String()).Msg("Upgraded legacy API key hash to hmac-sha256")
	}(keyID, newHash)
}

// rehashExecutor is the minimal DB surface rehashAPIKeyAsync needs. Both
// *pgxpool.Pool (used by APIKeyStore) and KeyManagerDB (used by
// KeyManager via an interface) satisfy it without any wrapping.
type rehashExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NewAPIKeyStore creates a new API key store using the legacy raw-SHA-256
// hash scheme. Prefer NewAPIKeyStoreWithPepper in production.
func NewAPIKeyStore(db *pgxpool.Pool, enabled bool) *APIKeyStore {
	return NewAPIKeyStoreWithPepper(db, enabled, "")
}

// NewAPIKeyStoreWithPepper creates a new API key store configured with an
// HMAC pepper for new keys. Legacy SHA-256 rows in api_keys remain valid
// and are opportunistically rehashed on first successful validation.
func NewAPIKeyStoreWithPepper(db *pgxpool.Pool, enabled bool, pepper string) *APIKeyStore {
	return &APIKeyStore{
		db:      db,
		enabled: enabled,
		pepper:  pepper,
	}
}

// HashAPIKey creates a raw SHA-256 hash of an API key. Retained for
// backwards compatibility with legacy keys and tests; new callers should
// prefer HashAPIKeyForStorage which routes through the configured pepper
// when one is available. SEC-009 (#123).
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// HashAPIKeyHMAC computes HMAC-SHA256(pepper, key) for storing as the
// key_hash column. An attacker with a stolen DB dump can no longer run
// rainbow tables against the stored hashes without also obtaining the
// pepper.
//
// Panics on an empty pepper: HMAC-SHA256 with a zero-length key is
// technically valid but has no entropy and is equivalent to a published
// constant — it provides NONE of the rainbow-table protection this
// function exists to deliver. Production code routes through
// HashAPIKeyForStorage which dispatches to HashAPIKey (legacy sha256)
// when no pepper is configured, so this panic only fires when a caller
// reaches HashAPIKeyHMAC directly with an empty pepper — which is a
// programming bug, not a runtime condition.
func HashAPIKeyHMAC(pepper, key string) string {
	if pepper == "" {
		panic("HashAPIKeyHMAC called with empty pepper — route through HashAPIKeyForStorage instead")
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

// HashAPIKeyForStorage returns the hash that should be written to
// api_keys.key_hash for a newly created key along with the algorithm
// marker for the api_keys.hash_algorithm column. When pepper is empty we
// fall back to the legacy sha256 scheme so development environments keep
// working; production deployments must set a pepper (SEC-009 / #123).
func HashAPIKeyForStorage(pepper, key string) (hash, algorithm string) {
	if pepper == "" {
		return HashAPIKey(key), HashAlgoSHA256
	}
	return HashAPIKeyHMAC(pepper, key), HashAlgoHMACSHA256
}

// ValidateKey checks if an API key is valid and returns the associated key record
// TB-006: Now also checks is_active status for rotation support
// SEC-009 (#123): Probes both the HMAC-pepper hash (when a pepper is
// configured) and the legacy raw SHA-256 hash so existing keys keep working
// during the rollout. On a successful legacy-hash match, the key is
// opportunistically rehashed to HMAC so the fleet drains to the new scheme
// as keys are used.
//
// Return convention: (nil, nil) means "no matching key / not authorized"
// — the auth middleware consumes this as a clean rejection without
// caring about the reason (not found, revoked, inactive, expired). This
// differs from KeyManager.ValidateAPIKey in keys.go which returns typed
// errors (ErrKeyNotFound, ErrKeyRevoked, ...) because the
// key-management endpoints need to explain WHY a key was rejected.
// Unifying the two conventions is a separate refactor.
func (s *APIKeyStore) ValidateKey(ctx context.Context, key string) (*APIKey, error) {
	if s.db == nil {
		return nil, nil
	}

	// Build the candidate (hash, algorithm) pairs to probe in priority order.
	// When a pepper is set we try the HMAC hash first (the steady-state hot
	// path) and fall back to the legacy SHA-256. When no pepper is set we
	// only try the legacy hash.
	legacyHash := HashAPIKey(key)
	candidates := make([]hashProbe, 0, 2)
	if s.pepper != "" {
		candidates = append(candidates, hashProbe{
			hash:      HashAPIKeyHMAC(s.pepper, key),
			algorithm: HashAlgoHMACSHA256,
		})
	}
	candidates = append(candidates, hashProbe{
		hash:      legacyHash,
		algorithm: HashAlgoSHA256,
	})

	// hash_algorithm is NOT NULL DEFAULT 'sha256' (migration 024) and the
	// WHERE clause already filters on it, so no COALESCE is needed and we
	// don't need to scan it back out either — matched.algorithm holds the
	// value already.
	const query = `
		SELECT id, key_hash, name, user_id, permissions, last_used_at,
		       created_at, expires_at, revoked, COALESCE(is_active, TRUE) as is_active
		FROM api_keys
		WHERE key_hash = $1 AND hash_algorithm = $2
	`

	var (
		apiKey  APIKey
		matched hashProbe
		found   bool
	)

	for _, c := range candidates {
		var permissions []byte
		err := s.db.QueryRow(ctx, query, c.hash, c.algorithm).Scan(
			&apiKey.ID,
			&apiKey.KeyHash,
			&apiKey.Name,
			&apiKey.UserID,
			&permissions,
			&apiKey.LastUsedAt,
			&apiKey.CreatedAt,
			&apiKey.ExpiresAt,
			&apiKey.Revoked,
			&apiKey.IsActive,
		)
		if err == nil {
			if len(permissions) > 0 {
				if uerr := json.Unmarshal(permissions, &apiKey.Permissions); uerr != nil {
					return nil, fmt.Errorf("invalid permissions JSON: %w", uerr)
				}
			}
			matched = c
			found = true
			break
		}
		// Only ErrNoRows means "this algorithm didn't find a match — try
		// the next candidate". Any other error (connection timeout, pool
		// exhausted, query failure) is fatal and must NOT fall through
		// to the legacy probe: doing so would let a transient DB error
		// on the HMAC path silently bypass the HMAC guarantee by
		// letting the sha256 probe take over. Propagate the error so
		// the caller can reject the request.
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("validate api key (%s): %w", c.algorithm, err)
		}
	}

	if !found {
		return nil, nil
	}

	// Constant-time comparison to prevent timing oracle attacks (#122).
	// Even though the lookup was done via SQL, we verify the hash in Go as
	// well to ensure no optimisation or short-circuit can leak key material
	// through response-time differences.
	if subtle.ConstantTimeCompare([]byte(apiKey.KeyHash), []byte(matched.hash)) != 1 {
		return nil, nil
	}

	// Check if key is revoked
	if apiKey.Revoked {
		return nil, nil
	}

	// TB-006: Check if key is active (may be inactive due to rotation)
	if !apiKey.IsActive {
		return nil, nil
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}

	// SEC-009: opportunistically upgrade a legacy sha256 row to hmac-sha256
	// on first successful validation after a pepper is configured. Fire and
	// forget — validation still succeeds if the upgrade races or errors,
	// and the row will be retried on the next use.
	if s.pepper != "" && matched.algorithm == HashAlgoSHA256 {
		s.rehashLegacyKey(apiKey.ID, key)
	}

	// Update last used timestamp asynchronously with timeout context
	// Using a detached context with timeout to avoid leaking the request context
	apiKeyID := apiKey.ID // Capture value to avoid closure over pointer
	go func() {           //nolint:gosec
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

		// Check HTTPS requirement in production.
		// Only trust X-Forwarded-Proto when TrustForwardedProto is explicitly
		// enabled (e.g. behind a K8s ingress or ALB that terminates TLS).
		// Without that flag the header is user-spoofable over plain HTTP and
		// bypasses the check entirely (SEC-004 / #118).
		if config.RequireHTTPS && c.Request.TLS == nil {
			forwardedHTTPS := config.TrustForwardedProto && c.GetHeader("X-Forwarded-Proto") == "https"
			host := c.Request.Host
			isLocalhost := strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]")
			if !forwardedHTTPS && !isLocalhost {
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
				"error": "API key required",
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
