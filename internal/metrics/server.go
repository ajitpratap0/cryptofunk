// Package metrics provides HTTP server for exposing Prometheus metrics
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// Server provides HTTP server for Prometheus metrics
type Server struct {
	port   int
	server *http.Server
	mux    *http.ServeMux
	log    zerolog.Logger
}

// NewServer creates a new metrics server
func NewServer(port int, log zerolog.Logger) *Server {
	return &Server{
		port: port,
		log:  log.With().Str("component", "metrics_server").Logger(),
	}
}

// Start starts the metrics HTTP server
func (s *Server) Start() error {
	s.mux = http.NewServeMux()

	// Prometheus metrics endpoint
	s.mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint with detailed JSON response
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   config.Version,
		}

		json.NewEncoder(w).Encode(health)
	})

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.log.Info().Int("port", s.port).Msg("Starting metrics server")

	// Start in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("Metrics server error")
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the metrics server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.log.Info().Msg("Shutting down metrics server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown metrics server: %w", err)
	}

	s.log.Info().Msg("Metrics server shutdown complete")
	return nil
}

// RegisterHandler registers a custom HTTP handler
func (s *Server) RegisterHandler(pattern string, handler http.HandlerFunc) {
	if s.mux != nil {
		s.mux.HandleFunc(pattern, handler)
	}
}
