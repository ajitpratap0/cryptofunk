package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/time/rate"
)

// runHTTP starts the server with Streamable HTTP transport.
func (s *Server) runHTTP(port int) error {
	s.logger.Info().Int("port", port).Msg("MCP server starting with Streamable HTTP transport")

	httpServer := s.buildHTTPServer(port)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh) // M1: prevent goroutine leak if runHTTP returns early

	go func() {
		<-sigCh
		s.logger.Info().Msg("Shutdown signal received")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error().Err(err).Msg("HTTP server shutdown error")
		}
		if s.metricsSrv != nil {
			// Give metrics server its own independent timeout so it is not starved
			// if the main server uses the full 10-second grace period.
			metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer metricsCancel()
			if err := s.metricsSrv.Shutdown(metricsCtx); err != nil {
				s.logger.Error().Err(err).Msg("Metrics server shutdown error")
			}
		}
	}()

	s.logger.Info().Str("addr", httpServer.Addr).Msg("MCP HTTP server listening")

	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		s.logger.Info().Msg("HTTP server shut down gracefully")
		return nil
	}
	return err
}

// NewHTTPServer creates an HTTP server with the MCP handler configured but does
// not start it. The caller is responsible for starting and shutting it down.
func (s *Server) NewHTTPServer(port int) (*http.Server, error) {
	return s.buildHTTPServer(port), nil
}

// buildHTTPServer constructs an *http.Server for the given port. Used by both
// runHTTP and NewHTTPServer to avoid duplicating server configuration (M3).
func (s *Server) buildHTTPServer(port int) *http.Server {
	addr := fmt.Sprintf(":%d", port)
	return &http.Server{
		Addr:              addr,
		Handler:           s.buildHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		MaxHeaderBytes:    64 * 1024, // 64KB max headers
		// Note: WriteTimeout is intentionally omitted — it breaks SSE streaming.
	}
}

// buildHandler constructs the two-layer HTTP handler.
// The outer mux serves /health directly (bypassing all middleware so that K8s
// liveness/readiness probes cannot exhaust the rate limiter).
// All other paths are routed through the full middleware stack.
// Auth/CORS startup warnings are emitted here so both runHTTP and NewHTTPServer
// paths surface them (M3).
func (s *Server) buildHandler() http.Handler {
	// Require MCP_AUTH_TOKEN for HTTP transport.
	// MCP_ALLOW_NO_AUTH must be explicitly "true" or "1" — "false" or empty will NOT suppress.
	allowNoAuth, _ := strconv.ParseBool(os.Getenv("MCP_ALLOW_NO_AUTH"))
	if os.Getenv("MCP_AUTH_TOKEN") == "" && !allowNoAuth {
		s.logger.Fatal().Msg("MCP_AUTH_TOKEN must be set (or set MCP_ALLOW_NO_AUTH=true to explicitly disable auth)")
	}

	// Warn when CORS origin defaults to wildcard.
	if os.Getenv("MCP_CORS_ORIGIN") == "" {
		s.logger.Warn().Msg("MCP_CORS_ORIGIN not set — defaulting to '*'. Set MCP_CORS_ORIGIN to restrict allowed origins.")
	}

	// panicRecoveryMiddleware is scoped to the inner MCP mux only.
	// /health is registered on the outer mux OUTSIDE the panic recovery so that
	// a panic in an MCP handler never causes /health to return a JSON-RPC error
	// shape — K8s liveness/readiness probes would misinterpret that response.
	inner := s.panicRecoveryMiddleware(s.buildMux())

	outer := http.NewServeMux()
	outer.HandleFunc("/health", s.healthHandler) // bypasses panic recovery and all middleware
	outer.Handle("/", s.wrapMiddleware(inner))   // everything else wrapped

	return outer
}

// panicRecoveryMiddleware wraps an http.Handler and recovers from panics,
// returning a JSON-RPC error response instead of crashing the process.
func (s *Server) panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				s.logger.Error().
					Interface("panic", rec).
					Str("path", r.URL.Path).					Str("stack", string(stack)).
					Msg("panic recovered in MCP HTTP handler")
				// NOTE: For SSE streaming responses (GET /mcp), if a panic occurs after
				// response headers have been flushed, the WriteHeader(500) and body write
				// below will be silently ignored by Go's HTTP library — the status code
				// and Content-Type header have already been sent. The log entry above is
				// still emitted. This is an inherent limitation of HTTP middleware-level
				// recovery for streaming protocols.
				//
				// NOTE: JSON-RPC 2.0 compliance — "id":null limitation.
				// The JSON-RPC 2.0 spec (§5) requires error responses to echo the request's
				// "id" field. Using "id":null is only correct when the id cannot be
				// determined. However, by the time a panic fires at this middleware layer,
				// the request body has already been consumed by the handler chain; we
				// cannot re-parse the body here to extract the id without significant
				// complexity (e.g., body-peeking before routing).
				//
				// A compliant JSON-RPC 2.0 client receiving "id":null will treat this as
				// an unmatched error notification per spec §5: "If there was an error in
				// detecting the id in the Request object (e.g. Parse error/Invalid
				// Request), it MUST be Null." The client may log the notification and
				// continue, but could hang waiting for the real response on its matching
				// id. This is a known limitation of middleware-level panic recovery.
				//
				// Future improvement: intercept and buffer/peek the request body before
				// passing to the handler so the id can be echoed accurately on panic.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal server error"}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// buildMux constructs the inner HTTP mux with /mcp routes (no /health).
func (s *Server) buildMux() *http.ServeMux {
	// Create the StreamableHTTPHandler from the SDK
	streamHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpSrv
	}, nil)

	mux := http.NewServeMux()

	// Mount the MCP handler at /mcp (handles both POST and GET /mcp for SSE).
	// Accept header: Clients should send "Accept: application/json, text/event-stream"
	// for POST requests per the MCP Streamable HTTP spec. GET requests for SSE
	// should use "Accept: text/event-stream".
	// Note: bodyLimitMiddleware is applied in wrapMiddleware AFTER auth so that
	// unauthenticated requests are rejected before their body is read.
	mux.Handle("/mcp", streamHandler)

	return mux
}

// healthHandler responds to health probe requests. When MCP_HEALTH_VERBOSE is
// set, the response includes the server name and version.
// Only GET and HEAD are accepted; other methods return 405 (m1).
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if os.Getenv("MCP_HEALTH_VERBOSE") != "" {
		resp := struct {
			Status  string `json:"status"`
			Server  string `json:"server"`
			Version string `json:"version"`
		}{
			Status:  "ok",
			Server:  s.config.Name,
			Version: s.config.Version,
		}
		_ = json.NewEncoder(w).Encode(resp)
	} else {
		// Use json.NewEncoder for consistent trailing newline behavior with the verbose path.
		_ = json.NewEncoder(w).Encode(struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}
}

// wrapMiddleware applies the full middleware stack around a handler.
// /health is served on the outer mux before reaching this stack, so it
// never hits rate limiting or auth.
func (s *Server) wrapMiddleware(handler http.Handler) http.Handler {
	authCfg := AuthConfig{
		Token: os.Getenv("MCP_AUTH_TOKEN"),
	}

	// Apply from innermost to outermost (request execution order is reversed):
	// Request flow: rate limit -> CORS -> auth -> body limit -> content-type -> handler
	// bodyLimitMiddleware is placed AFTER auth so unauthenticated requests are
	// rejected before their body is consumed.
	h := contentTypeMiddleware(handler)
	h = s.bodyLimitMiddleware(h)
	h = authMiddleware(authCfg, h)
	h = corsMiddleware(h)
	h = rateLimitMiddleware(h, parseRateLimit(os.Getenv("MCP_RATE_LIMIT")))
	return h
}

// corsMiddleware adds CORS headers for browser-based clients.
// Uses MCP_CORS_ORIGIN env var if set, otherwise defaults to * with a warning.
//
// NOTE (m7): The origin value is captured once at server startup. A process
// restart is required to pick up changes to MCP_CORS_ORIGIN at runtime.
func corsMiddleware(next http.Handler) http.Handler {
	origin := os.Getenv("MCP_CORS_ORIGIN")
	if origin == "" {
		origin = "*"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, Mcp-Session-Id, Last-Event-ID")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		// M4: Add Vary: Origin when the allowed origin is a specific domain
		// (not "*") so caches serve the correct response per requesting origin.
		if origin != "*" {
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware limits request rate using a token bucket.
// Returns HTTP 429 when the limit is exceeded.
//
// NOTE (M2): The limiter is global per-process, not per-client IP. A single
// aggressive client can starve others. For production deployments consider
// per-IP limiting via a reverse proxy (e.g. nginx, envoy).
// Retry-After: 1 is a conservative approximation based on the token-bucket
// refill interval; it is not precise for high-burst configurations.
func rateLimitMiddleware(next http.Handler, limiter *rate.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1") // M2: inform clients when to retry
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parseRateLimit parses the rate limit string (format: "rps,burst" e.g. "100,200")
// and returns a configured rate.Limiter. Defaults to 100 req/s with burst 200.
//
// The envValue parameter accepts the raw value of MCP_RATE_LIMIT. Accepting
// it as a parameter (rather than reading the env directly) makes this function
// testable without global environment mutation (m4).
// Production callers should pass os.Getenv("MCP_RATE_LIMIT").
func parseRateLimit(envValue string) *rate.Limiter {
	rps := float64(100)
	burst := 200

	if envValue != "" {
		parts := strings.SplitN(envValue, ",", 2)
		if v, err := strconv.ParseFloat(parts[0], 64); err == nil && v > 0 {
			rps = v
		}
		if len(parts) == 2 {
			if v, err := strconv.Atoi(parts[1]); err == nil && v > 0 {
				burst = v
			}
		}
	}

	return rate.NewLimiter(rate.Limit(rps), burst)
}

// contentTypeMiddleware validates that POST requests to /mcp have
// Content-Type: application/json. Returns 415 Unsupported Media Type if not.
func contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/mcp" {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_, _ = w.Write([]byte(`{"error":"Content-Type must be application/json"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// bodyLimitMiddleware wraps a handler to limit POST request body size to 1MB.
// C1: Two-stage enforcement:
//  1. Content-Length pre-check: reject immediately with 413 JSON error when
//     the client advertises a body larger than the limit (avoids reading any
//     bytes for clearly oversized requests).
//  2. MaxBytesReader fallback: handles chunked / no Content-Length bodies by
//     limiting the bytes actually read during handler execution.
func (s *Server) bodyLimitMiddleware(next http.Handler) http.Handler {
	const maxBodyBytes = int64(1 << 20) // 1MB
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Fast-path: reject immediately when Content-Length exceeds limit.
			if r.ContentLength > maxBodyBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_, _ = w.Write([]byte(`{"error":"request body too large"}`))
				return
			}
			// Fallback for chunked / unknown-length bodies.
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
