package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

// HTTPServer provides health checks and metrics endpoints for Kubernetes
type HTTPServer struct {
	server       *http.Server
	orchestrator *orchestrator.Orchestrator
	port         int
}

// NewHTTPServer creates a new HTTP server for health checks and metrics
func NewHTTPServer(port int, orch *orchestrator.Orchestrator) *HTTPServer {
	return &HTTPServer{
		orchestrator: orch,
		port:         port,
	}
}

// Start starts the HTTP server in a goroutine
func (h *HTTPServer) Start() error {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/readiness", h.handleReadiness)
	mux.HandleFunc("/liveness", h.handleLiveness)

	// Status endpoint
	mux.HandleFunc("/api/v1/status", h.handleStatus)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	h.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", h.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Int("port", h.port).
			Msg("HTTP server started (health checks, metrics)")

		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (h *HTTPServer) Stop(ctx context.Context) error {
	if h.server == nil {
		return nil
	}

	log.Info().Msg("Shutting down HTTP server...")
	return h.server.Shutdown(ctx)
}

// handleHealth handles GET /health - basic liveness check
// Returns 200 if the orchestrator process is running
func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "orchestrator",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// handleLiveness handles GET /liveness - Kubernetes liveness probe
// Returns 200 if the process is alive (same as /health for now)
func (h *HTTPServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// handleReadiness handles GET /readiness - Kubernetes readiness probe
// Returns 200 if orchestrator is ready to handle requests (dependencies connected)
func (h *HTTPServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check orchestrator readiness
	// TODO: Add actual readiness checks (NATS connected, DB connected, etc.)
	// For now, return ready if orchestrator exists
	if h.orchestrator == nil {
		http.Error(w, `{"status":"not ready","reason":"orchestrator not initialized"}`,
			http.StatusServiceUnavailable)
		return
	}

	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().Unix(),
		"checks": map[string]string{
			"orchestrator": "ok",
			// TODO: Add more checks
			// "database": checkDatabase(),
			// "nats": checkNATS(),
			// "redis": checkRedis(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// handleStatus handles GET /api/v1/status - detailed orchestrator status
// Returns information about active agents, sessions, etc.
func (h *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.orchestrator == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`,
			http.StatusServiceUnavailable)
		return
	}

	// Get orchestrator status
	// TODO: Implement GetStatus() method in orchestrator
	response := map[string]interface{}{
		"status":    "running",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0", // TODO: Get from config
		// "active_agents": h.orchestrator.GetActiveAgentCount(),
		// "active_sessions": h.orchestrator.GetActiveSessionCount(),
		// "uptime_seconds": h.orchestrator.GetUptimeSeconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
