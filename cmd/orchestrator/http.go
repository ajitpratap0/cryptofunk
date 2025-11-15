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

// HealthCheckResult represents the result of a single health check
type HealthCheckResult struct {
	Component string `json:"component"`
	Status    string `json:"status"` // "ok", "failed", "degraded"
	Message   string `json:"message,omitempty"`
	Latency   int64  `json:"latency_ms"`
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
	if h.orchestrator == nil {
		http.Error(w, `{"status":"not ready","reason":"orchestrator not initialized"}`,
			http.StatusServiceUnavailable)
		return
	}

	// Perform comprehensive health checks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checks := h.runHealthChecks(ctx)

	// Determine overall status
	allHealthy := true
	for _, check := range checks {
		if check.Status != "ok" {
			allHealthy = false
			break
		}
	}

	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().Unix(),
		"checks":    checks,
	}

	// Return 503 if any check failed
	if !allHealthy {
		response["status"] = "not ready"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(response)
		return
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
	status := h.orchestrator.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// runHealthChecks executes all health checks and returns results
func (h *HTTPServer) runHealthChecks(ctx context.Context) []HealthCheckResult {
	results := make([]HealthCheckResult, 0)

	// Check database connectivity
	results = append(results, h.checkDatabase(ctx))

	// Check NATS connectivity
	results = append(results, h.checkNATS(ctx))

	// Check agent connectivity (at least 1 agent should be active)
	results = append(results, h.checkAgents(ctx))

	return results
}

// checkDatabase checks database connectivity
func (h *HTTPServer) checkDatabase(ctx context.Context) HealthCheckResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	db := h.orchestrator.GetDB()
	if db == nil {
		return HealthCheckResult{
			Component: "database",
			Status:    "failed",
			Message:   "database connection is nil",
			Latency:   time.Since(start).Milliseconds(),
		}
	}

	err := db.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return HealthCheckResult{
			Component: "database",
			Status:    "failed",
			Message:   err.Error(),
			Latency:   latency,
		}
	}

	return HealthCheckResult{
		Component: "database",
		Status:    "ok",
		Latency:   latency,
	}
}

// checkNATS checks NATS connectivity
func (h *HTTPServer) checkNATS(ctx context.Context) HealthCheckResult {
	start := time.Now()

	natsConn := h.orchestrator.GetNATSConnection()
	if natsConn == nil {
		return HealthCheckResult{
			Component: "nats",
			Status:    "failed",
			Message:   "NATS connection is nil",
			Latency:   time.Since(start).Milliseconds(),
		}
	}

	if !natsConn.IsConnected() {
		return HealthCheckResult{
			Component: "nats",
			Status:    "failed",
			Message:   "NATS not connected",
			Latency:   time.Since(start).Milliseconds(),
		}
	}

	return HealthCheckResult{
		Component: "nats",
		Status:    "ok",
		Latency:   time.Since(start).Milliseconds(),
	}
}

// checkAgents checks if at least one agent is active
func (h *HTTPServer) checkAgents(ctx context.Context) HealthCheckResult {
	start := time.Now()

	activeCount := h.orchestrator.GetActiveAgentCount()

	if activeCount == 0 {
		return HealthCheckResult{
			Component: "agents",
			Status:    "degraded",
			Message:   "no active agents",
			Latency:   time.Since(start).Milliseconds(),
		}
	}

	return HealthCheckResult{
		Component: "agents",
		Status:    "ok",
		Message:   fmt.Sprintf("%d active agents", activeCount),
		Latency:   time.Since(start).Milliseconds(),
	}
}
