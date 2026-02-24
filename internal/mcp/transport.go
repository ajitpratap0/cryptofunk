package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runHTTP starts the server with Streamable HTTP transport.
func (s *Server) runHTTP(port int) error {
	s.logger.Info().Int("port", port).Msg("MCP server starting with Streamable HTTP transport")

	// Create the StreamableHTTPHandler from the SDK
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpSrv
	}, nil)

	mux := http.NewServeMux()

	// Mount the MCP handler at /mcp (handles both POST and GET /mcp for SSE)
	mux.Handle("/mcp", handler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","server":"%s","version":"%s"}`, s.config.Name, s.config.Version)
	})

	// Wrap with auth middleware, then CORS
	authCfg := AuthConfig{
		Token:     os.Getenv("MCP_AUTH_TOKEN"),
		SkipPaths: []string{"/health"},
	}
	authedHandler := authMiddleware(authCfg, mux)
	corsHandler := corsMiddleware(authedHandler)

	addr := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           corsHandler,
		ReadHeaderTimeout: 10 * time.Second,
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

// StartHTTPServer starts the HTTP server and returns it for testing.
// The caller is responsible for shutting it down.
func (s *Server) StartHTTPServer(port int) (*http.Server, error) {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpSrv
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","server":"%s","version":"%s"}`, s.config.Name, s.config.Version)
	})

	authCfg := AuthConfig{
		Token:     os.Getenv("MCP_AUTH_TOKEN"),
		SkipPaths: []string{"/health"},
	}

	addr := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           corsMiddleware(authMiddleware(authCfg, mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	return httpServer, nil
}

// corsMiddleware adds CORS headers for browser-based clients.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
