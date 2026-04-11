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
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// APIKeyStore handles API key storage and validation.
//
// The db field is typed as the KeyManagerDB interface rather than
// *pgxpool.Pool (concrete) so tests can inject pgxmock and exercise
// the dual-probe + rehash paths without needing testcontainers.
// *pgxpool.Pool satisfies the interface so production wiring is
// unchanged.
type APIKeyStore struct {
	db      KeyManagerDB
	enabled bool
	pepper  string // HMAC pepper for new keys; empty = legacy SHA-256 only
}

// inFlightRehashes dedupes concurrent rehash attempts across all
// APIKeyStore / KeyManager instances in a process. When a legacy key is
// validated many times concurrently before the first rehash completes,
// the second-and-later callers all observe the same "hash_algorithm =
// 'sha256'" row and would each spawn a goroutine issuing an identical
// UPDATE. The subsequent UPDATEs are harmless (they'll find 0 matching
// rows because the first one flipped the algorithm) but waste DB
// connections and log noise. This guard keeps at most one in-flight
// rehash per key ID.
//
// Package-level so the store and the key manager share the same map —
// a key validated through both the middleware and the key-management
// endpoint still only produces one UPDATE. Known limitation: two
// parallel subtests that use the same fixture UUID can silently
// suppress each other's rehash. The current tests use uuid.New() per
// subtest so this doesn't bite, but a future struct-field refactor
// would give full test isolation.
var inFlightRehashes sync.Map // map[uuid.UUID]struct{}

// asyncKeyOpsWG tracks every background goroutine spawned by key
// validation paths — runAsyncKeyOps serialises the opportunistic
// rehash and the last_used_at refresh into one goroutine. Tests
// drain this WaitGroup via waitForRehashesForTest before tearing
// down pgxmock so the async UPDATE doesn't race the mock teardown.
//
// Production shutdown note: main.go does NOT call asyncKeyOpsWG.Wait()
// on SIGTERM today. The goroutines carry their own 5s context timeout
// so a deployment drain window of >=5s lets them complete cleanly. If
// sub-5s terminations become a concern (tight rolling deploys, pod
// evictions), wire asyncKeyOpsWG.Wait() with a bounded timeout into
// the graceful shutdown path (see TODO in cmd/api/main.go shutdown).
var asyncKeyOpsWG sync.WaitGroup

// waitForRehashesForTest blocks until every async key-op goroutine
// spawned by runAsyncKeyOps has returned. Test-only: used by
// pgxmock-based unit tests to prevent the async UPDATE from racing
// mock.Close().
func waitForRehashesForTest() {
	asyncKeyOpsWG.Wait()
}

// runAsyncKeyOps is the single shared async post-validation worker
// for both APIKeyStore.ValidateKey and KeyManager.ValidateAPIKey. It
// spawns ONE goroutine that:
//  1. Runs the opportunistic rehash when needRehash is true (gated
//     by inFlightRehashes so only one goroutine at a time touches a
//     given key ID).
//  2. Runs the last_used_at refresh unconditionally for successful
//     validations.
//
// Serialising both UPDATEs in one goroutine gives deterministic
// execution order (pgxmock in-order expectations are stable), and
// having one helper shared between the two validation paths
// eliminates the drift risk where adding a third async op to one
// path leaves the other stale.
//
// gosec G118 and contextcheck suppressed: the goroutine is
// intentionally detached from the request context so these
// fire-and-forget writes complete even if the request is cancelled.
// The 5s context.WithTimeout below bounds the work independently.
func runAsyncKeyOps(db KeyManagerDB, keyID uuid.UUID, needRehash bool, rehashHash string) {
	if db == nil {
		return
	}
	asyncKeyOpsWG.Add(1)
	go func() { //nolint:gosec,contextcheck
		defer asyncKeyOpsWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if needRehash {
			// Claim the in-flight slot; skip the rehash step if
			// another goroutine is already rehashing this key.
			if _, alreadyInFlight := inFlightRehashes.LoadOrStore(keyID, struct{}{}); !alreadyInFlight {
				// defer the delete so a panic in db.Exec (nil pool
				// under teardown, driver bug, etc.) doesn't leak
				// the slot and permanently silence future rehashes
				// of this key ID. inFlightRehashes is a
				// package-level sync.Map so a leaked entry would
				// persist for the process lifetime.
				func() {
					defer inFlightRehashes.Delete(keyID)
					_, rerr := db.Exec(ctx, `
						UPDATE api_keys
						SET key_hash = $1, hash_algorithm = $2
						WHERE id = $3 AND hash_algorithm = $4
					`, rehashHash, HashAlgoHMACSHA256, keyID, HashAlgoSHA256)
					if rerr != nil {
						log.Warn().Err(rerr).Str("key_id", keyID.String()).Msg("Failed to rehash legacy API key to HMAC")
					} else {
						log.Info().Str("key_id", keyID.String()).Msg("Upgraded legacy API key hash to hmac-sha256")
					}
				}()
			}
		}

		_, luErr := db.Exec(ctx, "UPDATE api_keys SET last_used_at = NOW() WHERE id = $1 AND is_active = TRUE", keyID)
		if luErr != nil {
			log.Debug().Err(luErr).Str("key_id", keyID.String()).Msg("Failed to update last_used_at")
		}
	}()
}

// NewAPIKeyStore creates a new API key store using the legacy raw-SHA-256
// hash scheme.
//
// Deprecated: prefer NewAPIKeyStoreWithPepper in production. Raw SHA-256
// without a pepper is vulnerable to precomputed rainbow tables if the DB
// is stolen — see SEC-009 (#123).
func NewAPIKeyStore(db KeyManagerDB, enabled bool) *APIKeyStore {
	return NewAPIKeyStoreWithPepper(db, enabled, "")
}

// NewAPIKeyStoreWithPepper creates a new API key store configured with an
// HMAC pepper for new keys. Legacy SHA-256 rows in api_keys remain valid
// and are opportunistically rehashed on first successful validation.
// Accepts the KeyManagerDB interface so tests can pass pgxmock while
// production passes a *pgxpool.Pool (which satisfies the interface).
func NewAPIKeyStoreWithPepper(db KeyManagerDB, enabled bool, pepper string) *APIKeyStore {
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

// hmacAPIKeyHash computes HMAC-SHA256(pepper, key) for storing as
// the key_hash column. An attacker with a stolen DB dump can no longer
// run rainbow tables against the stored hashes without also obtaining
// the pepper.
//
// Panics on an empty pepper: HMAC-SHA256 with a zero-length key is
// technically valid but has no entropy and is equivalent to a
// published constant — it provides NONE of the rainbow-table
// protection this function exists to deliver. Production code routes
// through HashAPIKeyForStorage which dispatches to HashAPIKey (legacy
// sha256) when no pepper is configured, so this panic only fires when
// a caller reaches hmacAPIKeyHash directly with an empty pepper —
// which is a programming bug, not a runtime condition. Callers inside
// this package that build probe lists already gate on pepper != ""
// before reaching this function. (The conventional Must-prefix is
// reserved for exported helpers; this function is unexported and
// panics for the same reason a Must-helper would, but keeping the
// prefix off unexported names matches standard-library convention.)
func hmacAPIKeyHash(pepper, key string) string {
	if pepper == "" {
		panic("hmacAPIKeyHash called with empty pepper — route through HashAPIKeyForStorage instead")
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	// hash.Hash.Write never returns an error per the io.Writer contract
	// for in-memory hashes; discard the return explicitly for linters.
	_, _ = mac.Write([]byte(key))
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
	return hmacAPIKeyHash(pepper, key), HashAlgoHMACSHA256
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
//
// Timing channel note: during the migration window a legacy sha256 key
// takes two DB round-trips (HMAC miss, then sha256 hit) while an
// HMAC-stored key takes one — an observer measuring response times
// could in principle distinguish the two code paths. The difference
// is 1-10 ms (two PostgreSQL round trips vs one), not sub-millisecond.
// In practice network RTT variance and query planning noise dominate
// this difference, and the exposure window is temporary — it closes
// as keys drain forward via opportunistic rehash (observable via the
// "Upgraded legacy API key hash to hmac-sha256" log lines). The
// steady state after migration is one probe, one round-trip.
func (s *APIKeyStore) ValidateKey(ctx context.Context, key string) (*APIKey, error) {
	if s.db == nil {
		return nil, nil
	}

	// Build the candidate (hash, algorithm) pairs to probe in priority order.
	// When a pepper is set we try the HMAC hash first (the steady-state hot
	// path) and fall back to the legacy SHA-256. When no pepper is set we
	// only try the legacy hash.
	//
	// hmacHash is cached here so a later rehash of a legacy-hit key doesn't
	// recompute it — HMAC-SHA256 is non-trivial work on every migration-
	// window validation and would run twice per call without this cache.
	legacyHash := HashAPIKey(key)
	var hmacHash string
	candidates := make([]hashProbe, 0, 2)
	if s.pepper != "" {
		hmacHash = hmacAPIKeyHash(s.pepper, key)
		candidates = append(candidates, hashProbe{
			hash:      hmacHash,
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

	// Reuse hmacHash computed above for the HMAC probe — no need to
	// recompute, which would double the HMAC-SHA256 cost on the
	// legacy-fallback hot path during the migration window.
	needRehash := s.pepper != "" && matched.algorithm == HashAlgoSHA256
	var rehashHash string
	if needRehash {
		rehashHash = hmacHash
	}
	// Single async goroutine chains the opportunistic rehash (when
	// applicable) and the last_used_at update — same pattern as
	// KeyManager.ValidateAPIKey in keys.go. Serialising both UPDATEs
	// in one goroutine gives deterministic execution order (useful
	// for in-order pgxmock tests) and makes the async behaviour
	// identical across both validation paths.
	runAsyncKeyOps(s.db, apiKey.ID, needRehash, rehashHash)

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
