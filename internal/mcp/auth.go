package mcpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthConfig holds configuration for MCP transport authentication.
type AuthConfig struct {
	// Token is the shared secret required for access.
	// If empty, authentication is disabled.
	Token string
	// SkipPaths are paths that bypass authentication (e.g., /health).
	SkipPaths []string
}

// authMiddleware returns HTTP middleware that enforces Bearer token authentication.
// If token is empty, the middleware is a no-op (allows all requests).
func authMiddleware(cfg AuthConfig, next http.Handler) http.Handler {
	if cfg.Token == "" {
		return next
	}

	skip := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skip[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"invalid authorization format, expected Bearer token"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.Token)) != 1 {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
