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
// rehash and the last_used_at refresh into one goroutine. The
// graceful-shutdown path in cmd/api/main.go calls DrainAsyncKeyOps()
// with a bounded timeout so in-flight UPDATEs finish before the DB
// pool closes. Tests drain via the waitForRehashesForTest helper in
// export_test.go so the async UPDATE doesn't race mock.Close().
//
// Exposed as a package-level var (not a method) because both the
// store and the key manager spawn goroutines through the shared
// runAsyncKeyOps helper, and both need the same drain target.
var asyncKeyOpsWG sync.WaitGroup

// DrainAsyncKeyOps blocks until every in-flight async key-op
// goroutine has returned, or the context is cancelled. Called from
// the graceful shutdown path so rehash and last_used_at UPDATEs
// started just before SIGTERM can finish before the DB pool closes
// and the process exits. Returns ctx.Err() if the context elapses
// before the drain completes (the caller logs it and proceeds — an
// in-flight UPDATE against a closed pool will fail to commit but
// never corrupts on-disk state).
func DrainAsyncKeyOps(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		asyncKeyOpsWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
