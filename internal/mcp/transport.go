package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

	// Warn when MCP_AUTH_TOKEN is not set for HTTP transport.
	if os.Getenv("MCP_AUTH_TOKEN") == "" && os.Getenv("MCP_ALLOW_NO_AUTH") == "" {
		s.logger.Warn().Msg("MCP_AUTH_TOKEN not set — HTTP transport is UNAUTHENTICATED. Set MCP_AUTH_TOKEN or set MCP_ALLOW_NO_AUTH=1 to suppress this warning.")
	}

	// Warn when CORS origin defaults to wildcard.
	if os.Getenv("MCP_CORS_ORIGIN") == "" {
		s.logger.Warn().Msg("MCP_CORS_ORIGIN not set — defaulting to '*'. Set MCP_CORS_ORIGIN to restrict allowed origins.")
	}

	handler := s.buildHandler()

	addr := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		MaxHeaderBytes:    64 * 1024, // 64KB max headers
		// Note: WriteTimeout is intentionally omitted — it breaks SSE streaming.
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		s.logger.Info().Msg("Shutdown signal received")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error().Err(err).Msg("HTTP server shutdown error")
		}
	}()

	s.logger.Info().Str("addr", addr).Msg("MCP HTTP server listening")

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
	addr := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.buildHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		MaxHeaderBytes:    64 * 1024, // 64KB max headers
		// Note: WriteTimeout is intentionally omitted — it breaks SSE streaming.
	}

	return httpServer, nil
}

// buildHandler constructs the two-layer HTTP handler.
// The outer mux serves /health directly (bypassing all middleware so that K8s
// liveness/readiness probes cannot exhaust the rate limiter).
// All other paths are routed through the full middleware stack.
func (s *Server) buildHandler() http.Handler {
	inner := s.buildMux()

	outer := http.NewServeMux()
	outer.HandleFunc("/health", s.healthHandler) // bypasses all middleware
	outer.Handle("/", s.wrapMiddleware(inner))   // everything else wrapped

	return outer
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
func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
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
	h = rateLimitMiddleware(h, parseRateLimit())
	return h
}

// corsMiddleware adds CORS headers for browser-based clients.
// Uses MCP_CORS_ORIGIN env var if set, otherwise defaults to * with a warning.
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

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware limits request rate using a token bucket.
// Returns HTTP 429 when the limit is exceeded.
func rateLimitMiddleware(next http.Handler, limiter *rate.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parseRateLimit reads MCP_RATE_LIMIT env var (format: "rps,burst" e.g. "100,200")
// and returns a configured rate.Limiter. Defaults to 100 req/s with burst 200.
//
// NOTE: Rate limiting is global per-process, not per-client IP. A single
// aggressive client can starve others. For production deployments with multiple
// clients, consider adding per-IP rate limiting via a reverse proxy (e.g.,
// nginx, envoy).
func parseRateLimit() *rate.Limiter {
	rps := float64(100)
	burst := 200

	if env := os.Getenv("MCP_RATE_LIMIT"); env != "" {
		parts := strings.SplitN(env, ",", 2)
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
func (s *Server) bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		}
		next.ServeHTTP(w, r)
	})
}
