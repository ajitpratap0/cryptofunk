package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

// HTTPServer provides health checks and metrics endpoints for Kubernetes
type HTTPServer struct {
	server       *http.Server
	orchestrator *orchestrator.Orchestrator
	port         int
	startTime    time.Time
}

// HealthCheckResult represents the result of a single health check
type HealthCheckResult struct {
	Component string `json:"component"`
	Status    string `json:"status"` // "ok", "failed", "degraded"
	Message   string `json:"message,omitempty"`
	Latency   int64  `json:"latency_ms"`
}

// HealthCheckMetrics holds Prometheus metrics for health checks
type HealthCheckMetrics struct {
	Status  *prometheus.GaugeVec
	Latency *prometheus.HistogramVec
	Total   *prometheus.CounterVec
}

var (
	healthCheckMetrics     *HealthCheckMetrics
	healthCheckMetricsOnce sync.Once
)

// getOrCreateHealthCheckMetrics returns the singleton health check metrics instance
func getOrCreateHealthCheckMetrics() *HealthCheckMetrics {
	healthCheckMetricsOnce.Do(func() {
		healthCheckMetrics = &HealthCheckMetrics{
			Status: promauto.NewGaugeVec(prometheus.GaugeOpts{
				Name: "health_check_status",
				Help: "Health check status by component (1=ok, 0=failed, 0.5=degraded)",
			}, []string{"component"}),
			Latency: promauto.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "health_check_latency_ms",
				Help:    "Health check latency in milliseconds by component",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000},
			}, []string{"component"}),
			Total: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "health_check_total",
				Help: "Total number of health checks by component and status",
			}, []string{"component", "status"}),
		}
	})
	return healthCheckMetrics
}

// NewHTTPServer creates a new HTTP server for health checks and metrics
func NewHTTPServer(port int, orch *orchestrator.Orchestrator) *HTTPServer {
	// Initialize health check metrics
	getOrCreateHealthCheckMetrics()

	return &HTTPServer{
		orchestrator: orch,
		port:         port,
		startTime:    time.Now(),
	}
}

// orchestratorAuthMiddleware gates a handler behind ORCHESTRATOR_SECRET.
//
// Accepted credential carriers (first non-empty wins, both compared in
// constant time):
//  1. `X-Orchestrator-Secret: <secret>` — original control-plane header,
//     used by ops tooling that calls /pause / /resume / /status.
//  2. `Authorization: Bearer <secret>` — Prometheus and most off-the-shelf
//     scrape clients only support the standard Authorization header,
//     so /metrics needs this carrier to be reachable from a real
//     scrape config without a custom relabel hack.
//
// When `secret` is empty (dev mode), the middleware is a no-op pass
// through and BOTH carriers are ignored — the startup Warn log already
// makes this degradation loud.
func orchestratorAuthMiddleware(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			next(w, r)
			return
		}

		// Try the legacy custom header first.
		if header := r.Header.Get("X-Orchestrator-Secret"); header != "" {
			if subtle.ConstantTimeCompare([]byte(header), []byte(secret)) == 1 {
				next(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Fall back to Authorization: Bearer <secret>.
		// strings.HasPrefix is intentionally NOT used here because we
		// want a strict match on the "Bearer " token to avoid spurious
		// matches on schemes like "Bearer123". Trim once and compare.
		const bearerPrefix = "Bearer "
		if authz := r.Header.Get("Authorization"); len(authz) > len(bearerPrefix) && authz[:len(bearerPrefix)] == bearerPrefix {
			token := authz[len(bearerPrefix):]
			if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1 {
				next(w, r)
				return
			}
		}

		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

// Start starts the HTTP server in a goroutine
func (h *HTTPServer) Start() error {
	mux := http.NewServeMux()
	secret := os.Getenv("ORCHESTRATOR_SECRET")

	if secret == "" {
		log.Warn().Msg("ORCHESTRATOR_SECRET is not set: /pause, /resume, /status, and /metrics endpoints are unprotected")
	}

	// Health check endpoints
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/readiness", h.handleReadiness)
	mux.HandleFunc("/liveness", h.handleLiveness)

	// Status endpoint
	mux.HandleFunc("/api/v1/status", h.handleStatus)

	// Control endpoints (pause/resume trading)
	mux.HandleFunc("/pause", orchestratorAuthMiddleware(secret, h.orchestrator.HandlePauseRequest))
	mux.HandleFunc("/resume", orchestratorAuthMiddleware(secret, h.orchestrator.HandleResumeRequest))
	mux.HandleFunc("/status", orchestratorAuthMiddleware(secret, h.orchestrator.HandleControlStatusRequest))

	// Prometheus metrics endpoint (SEC-006 / #120). Wrapped with the same
	// orchestratorAuthMiddleware as the control endpoints because the
	// metric labels expose internal state — active agent counts, queue
	// depths, decision counters, error rates — that an attacker can use
	// to fingerprint the deployment, time trades, or pick off agents.
	// promhttp.Handler() returns an http.Handler; we adapt it to
	// HandlerFunc by calling .ServeHTTP so the existing middleware
	// signature stays unchanged.
	//
	// When ORCHESTRATOR_SECRET is empty (development), the middleware is
	// a no-op pass-through — operators get the warning logged above and
	// /metrics keeps working unchanged. Production deployments MUST set
	// the secret; the K8s orchestrator-secret manifest already wires it.
	mux.HandleFunc("/metrics", orchestratorAuthMiddleware(secret, promhttp.Handler().ServeHTTP))

	h.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", h.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Bind the listener SYNCHRONOUSLY before returning so callers (and
	// tests) know that an immediate connection attempt will reach the
	// listener queue. Previous behaviour spawned ListenAndServe in a
	// goroutine and returned immediately, leaving a race window where
	// the kernel had not yet bound the port — tests papered over this
	// with a 100ms time.Sleep that flaked under -race on busy CI runs.
	//
	// net.ListenConfig.Listen (rather than the package-level net.Listen)
	// satisfies the noctx linter and lets us bound the bind itself by
	// the supplied context if we ever need to. The context is
	// background here because Start() has no caller-supplied lifetime
	// — Stop() owns shutdown via h.server.Shutdown.
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", h.server.Addr)
	if err != nil {
		return fmt.Errorf("orchestrator http: bind %s: %w", h.server.Addr, err)
	}

	go func() {
		log.Info().
			Int("port", h.port).
			Msg("HTTP server started (health checks, metrics)")

		if err := h.server.Serve(listener); err != nil && err != http.ErrServerClosed {
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

	// Gather NATS and agent info for health response
	natsStatus := "unknown"
	if conn := h.orchestrator.GetNATSConnection(); conn != nil {
		if conn.IsConnected() {
			natsStatus = "connected"
		} else {
			natsStatus = "disconnected"
		}
	}

	response := map[string]interface{}{
		"status":        "healthy",
		"timestamp":     time.Now().Unix(),
		"service":       "orchestrator",
		"version":       config.Version,
		"uptime_sec":    int(time.Since(h.startTime).Seconds()),
		"nats":          natsStatus,
		"active_agents": h.orchestrator.GetActiveAgentCount(),
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
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

// recordHealthCheckMetrics records health check metrics to Prometheus
func (h *HTTPServer) recordHealthCheckMetrics(result HealthCheckResult) {
	metrics := getOrCreateHealthCheckMetrics()

	// Record status as gauge (1=ok, 0.5=degraded, 0=failed)
	statusValue := 0.0
	switch result.Status {
	case "ok":
		statusValue = 1.0
	case "degraded":
		statusValue = 0.5
	case "failed":
		statusValue = 0.0
	}
	metrics.Status.WithLabelValues(result.Component).Set(statusValue)

	// Record latency
	metrics.Latency.WithLabelValues(result.Component).Observe(float64(result.Latency))

	// Record total counter
	metrics.Total.WithLabelValues(result.Component, result.Status).Inc()
}

// checkDatabase checks database connectivity
func (h *HTTPServer) checkDatabase(ctx context.Context) HealthCheckResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var result HealthCheckResult

	db := h.orchestrator.GetDB()
	if db == nil {
		result = HealthCheckResult{
			Component: "database",
			Status:    "failed",
			Message:   "database connection is nil",
			Latency:   time.Since(start).Milliseconds(),
		}
		h.recordHealthCheckMetrics(result)
		return result
	}

	err := db.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		log.Error().Err(err).Msg("database health check failed")
		result = HealthCheckResult{
			Component: "database",
			Status:    "failed",
			Message:   "database unavailable",
			Latency:   latency,
		}
		h.recordHealthCheckMetrics(result)
		return result
	}

	result = HealthCheckResult{
		Component: "database",
		Status:    "ok",
		Latency:   latency,
	}
	h.recordHealthCheckMetrics(result)
	return result
}

// checkNATS checks NATS connectivity
func (h *HTTPServer) checkNATS(ctx context.Context) HealthCheckResult {
	start := time.Now()
	var result HealthCheckResult

	natsConn := h.orchestrator.GetNATSConnection()
	if natsConn == nil {
		result = HealthCheckResult{
			Component: "nats",
			Status:    "failed",
			Message:   "NATS connection is nil",
			Latency:   time.Since(start).Milliseconds(),
		}
		h.recordHealthCheckMetrics(result)
		return result
	}

	if !natsConn.IsConnected() {
		result = HealthCheckResult{
			Component: "nats",
			Status:    "failed",
			Message:   "NATS not connected",
			Latency:   time.Since(start).Milliseconds(),
		}
		h.recordHealthCheckMetrics(result)
		return result
	}

	result = HealthCheckResult{
		Component: "nats",
		Status:    "ok",
		Latency:   time.Since(start).Milliseconds(),
	}
	h.recordHealthCheckMetrics(result)
	return result
}

// checkAgents checks if at least one agent is active
func (h *HTTPServer) checkAgents(ctx context.Context) HealthCheckResult {
	start := time.Now()
	var result HealthCheckResult

	activeCount := h.orchestrator.GetActiveAgentCount()

	if activeCount == 0 {
		result = HealthCheckResult{
			Component: "agents",
			Status:    "degraded",
			Message:   "no active agents",
			Latency:   time.Since(start).Milliseconds(),
		}
		h.recordHealthCheckMetrics(result)
		return result
	}

	result = HealthCheckResult{
		Component: "agents",
		Status:    "ok",
		Message:   fmt.Sprintf("%d active agents", activeCount),
		Latency:   time.Since(start).Milliseconds(),
	}
	h.recordHealthCheckMetrics(result)
	return result
}
