// Package api provides HTTP API handlers and middleware for the CryptoFunk trading system.
//
// # Authentication Middleware
//
// This package includes a complete API key authentication system (auth_middleware.go)
// that provides:
//   - API key validation via SHA-256 hashing (HMAC pepper rollout in flight, see SEC-009 / #123)
//   - Permission-based authorization
//   - Configurable authentication (enabled/disabled via config)
//   - Two middleware variants sharing one validation core:
//   - AuthMiddleware: standard HTTP, accepts the configured header
//     (default X-API-Key) or Authorization: Bearer
//   - WebSocketAuthMiddleware: same as AuthMiddleware plus an
//     ?api_key=<value> query-parameter fallback because browser
//     WebSocket clients cannot set custom headers on the upgrade
//     handshake (SEC-010 / #124)
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
	"crypto/subtle"
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
	IsActive    bool       `json:"is_active"` // TB-006: Active status for rotation support
}

// AuthConfig contains authentication configuration
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
// TB-006: Now also checks is_active status for rotation support
func (s *APIKeyStore) ValidateKey(ctx context.Context, key string) (*APIKey, error) {
	if s.db == nil {
		return nil, nil
	}

	keyHash := HashAPIKey(key)

	query := `
		SELECT id, key_hash, name, user_id, permissions, last_used_at,
		       created_at, expires_at, revoked, COALESCE(is_active, TRUE) as is_active
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
		&apiKey.IsActive,
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

	// Constant-time comparison to prevent timing oracle attacks (#122)
	// Even though the lookup was done via SQL, we verify the hash in Go as well
	// to ensure no optimisation or short-circuit can leak key material through
	// response-time differences.
	if subtle.ConstantTimeCompare([]byte(apiKey.KeyHash), []byte(keyHash)) != 1 {
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

// authVariant captures the per-middleware customization that
// distinguishes AuthMiddleware from WebSocketAuthMiddleware. The
// shared body in authMiddlewareCore reads from this struct so the
// HTTPS gate, validation, context-setting, and logging code lives in
// one place — eliminating ~100 lines of duplication and the drift
// risk where one middleware grew an audit log entry that the other
// didn't.
type authVariant struct {
	// logPrefix is prepended to log messages (e.g. "Auth" or "WS auth").
	logPrefix string
	// keyRequiredMessage is the JSON `error` body returned when no
	// credential carrier produced a key.
	keyRequiredMessage string
	// httpsRequiredMessage is the JSON `error` body returned when the
	// HTTPS gate rejects a request.
	httpsRequiredMessage string
	// extractKey reads the API key from the request using the variant's
	// strategy. AuthMiddleware tries header → Bearer; the WS variant
	// adds a `?api_key=` query-parameter fallback as the third tier.
	extractKey func(c *gin.Context, headerName string) string
}

// authMiddlewareCore implements the shared HTTPS-gate, key-extract,
// validate, identity-stash, and log flow for both AuthMiddleware and
// WebSocketAuthMiddleware. The two public middlewares are now thin
// wrappers around this body — they only differ by the authVariant
// they pass in.
//
// Keep this function package-private. The seam between the two
// middlewares is the variant struct, not a third public surface.
func authMiddlewareCore(store *APIKeyStore, config *AuthConfig, variant authVariant) gin.HandlerFunc {
	if config == nil {
		config = DefaultAuthConfig()
	}

	return func(c *gin.Context) {
		// Auth-disabled bypass. Both variants honor this so dev mode
		// (auth.enabled=false) works without any per-route gating.
		if !config.Enabled || !store.enabled {
			c.Next()
			return
		}

		// HTTPS gate. Only trust X-Forwarded-Proto when
		// TrustForwardedProto is explicitly enabled (e.g. behind a K8s
		// ingress or ALB that terminates TLS). Without that flag the
		// header is user-spoofable over plain HTTP and bypasses the
		// check entirely (SEC-004 / #118).
		//
		// We return 403 Forbidden (not 401 Unauthorized) because the
		// problem is the transport, not the credentials. 401 implies
		// "try again with a valid key", but no key will pass until the
		// transport is upgraded to TLS. 403 signals "your access is
		// forbidden at this transport level" — semantically the
		// correct HTTP status for "wrong scheme, can't continue".
		//
		// `middleware` carries the variant's logPrefix as a structured
		// zerolog field instead of being concatenated into the message
		// string. Two reasons: (1) zero allocations per-request on the
		// log path, (2) Loki/Datadog can filter on
		// `middleware="WS auth"` without regex matching the body.
		if config.RequireHTTPS && c.Request.TLS == nil {
			forwardedHTTPS := config.TrustForwardedProto && c.GetHeader("X-Forwarded-Proto") == "https"
			host := c.Request.Host
			isLocalhost := strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]")
			if !forwardedHTTPS && !isLocalhost {
				log.Warn().
					Str("middleware", variant.logPrefix).
					Str("host", host).
					Str("ip", c.ClientIP()).
					Msg("HTTPS required but request is plain")
				c.JSON(http.StatusForbidden, gin.H{
					"error": variant.httpsRequiredMessage,
				})
				c.Abort()
				return
			}
		}

		apiKey := variant.extractKey(c, config.HeaderName)
		if apiKey == "" {
			log.Debug().
				Str("middleware", variant.logPrefix).
				Str("ip", c.ClientIP()).
				Str("path", c.Request.URL.Path).
				Msg("No API key provided")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": variant.keyRequiredMessage,
			})
			c.Abort()
			return
		}

		keyRecord, err := store.ValidateKey(c.Request.Context(), apiKey)
		if err != nil {
			log.Error().Err(err).
				Str("middleware", variant.logPrefix).
				Str("ip", c.ClientIP()).
				Msg("Error validating API key")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication error",
			})
			c.Abort()
			return
		}
		if keyRecord == nil {
			log.Warn().
				Str("middleware", variant.logPrefix).
				Str("ip", c.ClientIP()).
				Str("path", c.Request.URL.Path).
				Msg("Invalid or expired API key")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired API key",
			})
			c.Abort()
			return
		}

		// Stash identity for downstream handlers and audit logging.
		// The WS handler reads these via c.GetString("api_key_id") so
		// every accepted connection can be tied back to the issuing
		// key in audit logs.
		c.Set("user_id", keyRecord.UserID)
		c.Set("api_key_id", keyRecord.ID.String())
		c.Set("api_key_name", keyRecord.Name)
		c.Set("permissions", keyRecord.Permissions)

		// Scrub the api_key query parameter from the request URL so
		// downstream middleware (Gin's built-in Logger, any access-log
		// sidecar reading c.Request.URL) never sees the raw key. This
		// is a defence-in-depth measure on top of the operator guidance
		// in the WebSocketAuthMiddleware godoc. Only fires when the
		// param is actually present — the standard AuthMiddleware
		// variant never populates it, so the branch is a no-op there.
		if c.Request.URL.RawQuery != "" {
			q := c.Request.URL.Query()
			if q.Has("api_key") {
				q.Del("api_key")
				c.Request.URL.RawQuery = q.Encode()
			}
		}

		log.Debug().
			Str("middleware", variant.logPrefix).
			Str("user_id", keyRecord.UserID).
			Str("key_name", keyRecord.Name).
			Str("path", c.Request.URL.Path).
			Msg("Request authenticated")

		c.Next()
	}
}

// extractKeyHeaderOrBearer is the standard HTTP key-lookup strategy:
// configured header → Authorization: Bearer. Used by AuthMiddleware.
func extractKeyHeaderOrBearer(c *gin.Context, headerName string) string {
	if k := c.GetHeader(headerName); k != "" {
		return k
	}
	if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

// extractKeyHeaderBearerOrQuery is the WebSocket-specific key-lookup
// strategy: header → Bearer → `?api_key=<value>` query parameter. The
// query-parameter tier is the browser WebSocket fallback (the standard
// new WebSocket(url) constructor offers no per-request header API).
// SEC-010 / #124.
func extractKeyHeaderBearerOrQuery(c *gin.Context, headerName string) string {
	if k := extractKeyHeaderOrBearer(c, headerName); k != "" {
		return k
	}
	return c.Query("api_key")
}

// AuthMiddleware creates a Gin middleware that validates API keys.
// When auth is disabled, it allows all requests through. When enabled,
// it requires a valid API key in the configured header (default
// X-API-Key) OR an Authorization: Bearer token.
func AuthMiddleware(store *APIKeyStore, config *AuthConfig) gin.HandlerFunc {
	return authMiddlewareCore(store, config, authVariant{
		logPrefix:            "Auth",
		keyRequiredMessage:   "API key required",
		httpsRequiredMessage: "HTTPS required for API access",
		extractKey:           extractKeyHeaderOrBearer,
	})
}

// WebSocketAuthMiddleware creates a Gin middleware that authenticates
// WebSocket upgrade requests. It is API-compatible with AuthMiddleware
// but adds an `?api_key=<value>` query-parameter fallback because
// browser WebSocket clients cannot set custom headers on the upgrade
// handshake — the standard `new WebSocket(url)` constructor offers no
// per-request header API. SEC-010 / #124.
//
// Lookup precedence (first non-empty wins):
//  1. The configured header (default `X-API-Key`)
//  2. `Authorization: Bearer <key>`
//  3. `?api_key=<value>` query parameter
//
// The query-param fallback is intentionally NOT extended to
// AuthMiddleware: putting API keys in the query string of normal HTTP
// endpoints would leak them into proxy logs, browser history, and
// referrer headers. The fallback only exists here because the WS
// upgrade is a one-shot handshake whose URL is never followed by
// further GETs that could leak it onward, AND because browsers leave
// WS clients with no other option.
//
// # Access-log exposure (operator note)
//
// Even though the WS upgrade URL is never re-fetched, the `?api_key=`
// value WILL appear verbatim in any HTTP access log that captures the
// full request URI — nginx ingress, ALB, sidecar proxies, etc. Anyone
// reading those logs will see the raw key. Mitigations:
//
//   - Mask `api_key` in the access-log format. nginx: a `map` block
//     that rewrites `api_key=[^&]+` to `api_key=REDACTED` in
//     `$request_uri` before logging. ALB / GCP HTTPS LB: equivalent
//     strip-query-param annotations on the listener.
//   - Use short-lived / scoped tokens for browser sessions so a
//     leaked log line is bounded in time and capability.
//   - Prefer the header carrier when the client can set headers
//     (server-to-server, CLI tools, the gorilla/websocket Go Dialer).
//
// Validation runs BEFORE upgrader.Upgrade so a rejection produces a
// real HTTP 401 the client can read; once Upgrade has flipped the
// connection into WS frames, the only way to signal failure is a
// close frame, which most browsers surface as a generic onerror with
// no detail.
func WebSocketAuthMiddleware(store *APIKeyStore, config *AuthConfig) gin.HandlerFunc {
	return authMiddlewareCore(store, config, authVariant{
		logPrefix:            "WS auth",
		keyRequiredMessage:   "API key required for WebSocket connections",
		httpsRequiredMessage: "wss:// required for WebSocket access",
		extractKey:           extractKeyHeaderBearerOrQuery,
	})
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
