package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

// statusFailed is a constant for the "failed" status string
const statusFailed = "failed"

// createTestOrchestrator creates a test orchestrator with optional mock DB
func createTestOrchestrator(t *testing.T, withDB bool, dbShouldFail bool) *orchestrator.Orchestrator {
	t.Helper()

	config := &orchestrator.OrchestratorConfig{
		Name:                "test-orchestrator",
		NATSUrl:             "nats://localhost:4222",
		SignalTopic:         "test.signals",
		DecisionTopic:       "test.decisions",
		HeartbeatTopic:      "test.heartbeats",
		StepInterval:        10 * time.Second,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        5 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
	}

	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	var database *db.DB
	if withDB {
		// For now, we'll use nil DB in tests since we can't easily create a real DB connection
		// TODO: Use testcontainers to create a real PostgreSQL instance for integration tests
		database = nil
	}

	orch, err := orchestrator.NewOrchestrator(config, logger, database, 8080)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	return orch
}

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}

	if response["service"] != "orchestrator" {
		t.Errorf("Expected service 'orchestrator', got %v", response["service"])
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("Expected timestamp in response")
	}
}

// TestHealthEndpointMethodNotAllowed tests invalid HTTP methods
func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequestWithContext(context.Background(), method, "/health", nil)
		w := httptest.NewRecorder()

		server.handleHealth(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405 for %s, got %d", method, w.Code)
		}
	}
}

// TestLivenessEndpoint tests the /liveness endpoint
func TestLivenessEndpoint(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/liveness", nil)
	w := httptest.NewRecorder()

	server.handleLiveness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "alive" {
		t.Errorf("Expected status 'alive', got %v", response["status"])
	}
}

// TestReadinessEndpointOrchestratorNil tests readiness when orchestrator is nil
func TestReadinessEndpointOrchestratorNil(t *testing.T) {
	server := NewHTTPServer(8080, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readiness", nil)
	w := httptest.NewRecorder()

	server.handleReadiness(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

// TestReadinessEndpointHealthy tests readiness when all dependencies are healthy
func TestReadinessEndpointHealthy(t *testing.T) {
	orch := createTestOrchestrator(t, true, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readiness", nil)
	w := httptest.NewRecorder()

	server.handleReadiness(w, req)

	// Since NATS is not connected in test, we expect degraded status
	// TODO: Mock NATS connection for full integration test
	if w.Code != http.StatusServiceUnavailable {
		t.Log("Expected degraded status due to NATS not being connected")
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := response["checks"]; !ok {
		t.Error("Expected checks in response")
	}
}

// TestCheckDatabase tests database health check
func TestCheckDatabase(t *testing.T) {
	orch := createTestOrchestrator(t, true, false)
	server := NewHTTPServer(8080, orch)

	ctx := context.Background()
	result := server.checkDatabase(ctx)

	// Since we don't have a real DB in tests, we expect failure
	if result.Component != "database" {
		t.Errorf("Expected component 'database', got %s", result.Component)
	}

	// We expect "failed" since DB is nil or not connected
	if result.Status != statusFailed {
		t.Log("Database check status:", result.Status)
	}

	if result.Latency < 0 {
		t.Errorf("Expected non-negative latency, got %d", result.Latency)
	}
}

// TestCheckNATS tests NATS health check
func TestCheckNATS(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	ctx := context.Background()
	result := server.checkNATS(ctx)

	if result.Component != "nats" {
		t.Errorf("Expected component 'nats', got %s", result.Component)
	}

	// We expect "failed" since NATS is not connected in tests
	if result.Status != statusFailed {
		t.Logf("Unexpected NATS status: %s", result.Status)
	}
}

// TestCheckAgents tests agent health check
func TestCheckAgents(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	ctx := context.Background()
	result := server.checkAgents(ctx)

	if result.Component != "agents" {
		t.Errorf("Expected component 'agents', got %s", result.Component)
	}

	// We expect "degraded" since no agents are active
	if result.Status != "degraded" {
		t.Logf("Expected degraded status for no agents, got %s", result.Status)
	}

	if result.Message != "no active agents" {
		t.Logf("Unexpected message: %s", result.Message)
	}
}

// TestStatusEndpoint tests the /api/v1/status endpoint
func TestStatusEndpoint(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response orchestrator.OrchestratorStatus
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "running" {
		t.Errorf("Expected status 'running', got %s", response.Status)
	}

	if response.Version == "" {
		t.Error("Expected version to be set")
	}

	if response.Uptime < 0 {
		t.Errorf("Expected non-negative uptime, got %f", response.Uptime)
	}

	if response.ActiveAgents < 0 {
		t.Errorf("Expected non-negative active agents, got %d", response.ActiveAgents)
	}

	if response.TotalSignals < 0 {
		t.Errorf("Expected non-negative total signals, got %d", response.TotalSignals)
	}

	if response.Configuration == nil {
		t.Error("Expected configuration to be set")
	}

	if response.AgentSummary == nil {
		t.Error("Expected agent summary to be set")
	}
}

// TestStatusEndpointOrchestratorNil tests status when orchestrator is nil
func TestStatusEndpointOrchestratorNil(t *testing.T) {
	server := NewHTTPServer(8080, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

// TestConcurrentHealthChecks tests multiple concurrent health check requests
func TestConcurrentHealthChecks(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(8080, orch)

	// Start 10 concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()
			server.handleHealth(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
}

// TestHealthCheckTimeout tests health check timeout scenarios
func TestHealthCheckTimeout(t *testing.T) {
	orch := createTestOrchestrator(t, true, true)
	server := NewHTTPServer(8080, orch)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := server.checkDatabase(ctx)

	// We expect the check to fail due to timeout or DB unavailability
	if result.Component != "database" {
		t.Errorf("Expected component 'database', got %s", result.Component)
	}

	// The status should be "failed" due to timeout
	if result.Status == "ok" {
		t.Error("Expected check to fail with timeout")
	}
}

// TestHTTPServerStartStop tests starting and stopping the HTTP server
func TestHTTPServerStartStop(t *testing.T) {
	orch := createTestOrchestrator(t, false, false)
	server := NewHTTPServer(18081, orch) // Use different port to avoid conflicts

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	// Start now binds the listener synchronously, so an immediate
	// request below is guaranteed to reach the kernel listen queue.

	// Test that we can make a request
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://localhost:18081/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request to server: %v", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Stop the server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop HTTP server: %v", err)
	}

	// Give server time to stop
	time.Sleep(100 * time.Millisecond)

	// Verify server is stopped (request should fail)
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer verifyCancel()

	verifyReq, _ := http.NewRequestWithContext(verifyCtx, http.MethodGet, "http://localhost:18081/health", nil)
	verifyResp, err := http.DefaultClient.Do(verifyReq)
	if err == nil {
		// Close body if request unexpectedly succeeded
		if verifyResp != nil {
			_, _ = io.Copy(io.Discard, verifyResp.Body)
			_ = verifyResp.Body.Close()
		}
		t.Error("Expected request to fail after server stop, but it succeeded")
	}
}

// TestMetricsEndpointRequiresAuth_SEC006 verifies that /metrics on the
// orchestrator HTTP server is gated behind ORCHESTRATOR_SECRET when the
// secret is set, and that requests without the X-Orchestrator-Secret
// header (or with the wrong value) get 401. The metrics surface
// includes active agent counts, decision counters, queue depths, and
// error rates — fingerprinting data an attacker would use to time
// trades or pick off agents.
//
// Spins up the real Start() flow on an ephemeral port so the actual
// mux wiring is exercised end-to-end (not just a hand-built mux). The
// test sets ORCHESTRATOR_SECRET via t.Setenv so the value is restored
// for sibling tests.
func TestMetricsEndpointRequiresAuth_SEC006(t *testing.T) {
	const secret = "test-orch-secret-32-bytes-of-material"
	t.Setenv("ORCHESTRATOR_SECRET", secret)

	orch := createTestOrchestrator(t, false, false)
	// Port 0 → kernel-assigned ephemeral port. Build the request URL
	// from server.Addr() after Start so two suite runs (e.g. -count=2)
	// or any future t.Parallel() reorganisation can never collide on
	// a hardcoded port.
	server := NewHTTPServer(0, orch)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = server.Stop(stopCtx)
	})

	addr := server.Addr()
	if addr == nil {
		t.Fatal("server.Addr() returned nil after Start")
	}
	metricsURL := "http://" + addr.String() + "/metrics"

	cases := []struct {
		name         string
		xHeaderValue string
		bearerValue  string
		wantStatus   int
	}{
		{name: "no creds → 401", wantStatus: http.StatusUnauthorized},
		{name: "wrong X-Orchestrator-Secret → 401", xHeaderValue: "wrong-secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong Bearer → 401", bearerValue: "wrong-secret", wantStatus: http.StatusUnauthorized},
		{name: "correct X-Orchestrator-Secret → 200", xHeaderValue: secret, wantStatus: http.StatusOK},
		{name: "correct Bearer → 200 (Prometheus path)", bearerValue: secret, wantStatus: http.StatusOK},
		// Header-priority edge case: if the legacy X-Orchestrator-Secret
		// header is present but wrong, the middleware MUST NOT fall
		// through to try Authorization: Bearer — a stale custom header
		// should not be silently overridden by a fresh Bearer token. The
		// behaviour is documented as "if the legacy header is present
		// it is tried exclusively" and the test pins it down so a
		// future refactor doesn't relax it into a confusing fall-through.
		{name: "wrong X-Orchestrator-Secret blocks valid Bearer → 401", xHeaderValue: "wrong-secret", bearerValue: secret, wantStatus: http.StatusUnauthorized},
	}

	// Per-test http.Client avoids cross-subtest connection reuse via
	// http.DefaultClient. On -count=2 runs DefaultClient's idle
	// connection pool can mask cleanup ordering issues between subtests;
	// a fresh Transport per subtest makes each test fully isolated.
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: &http.Transport{}}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			if tc.xHeaderValue != "" {
				req.Header.Set("X-Orchestrator-Secret", tc.xHeaderValue)
			}
			if tc.bearerValue != "" {
				req.Header.Set("Authorization", "Bearer "+tc.bearerValue)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer func() {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status: got %d want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// TestMetricsEndpointOpenWhenNoSecret_SEC006 documents the dev-mode
// pass-through path: when ORCHESTRATOR_SECRET is empty the middleware
// is a no-op and /metrics serves without auth so local Prometheus and
// dev tooling keep working. The startup Warn log makes the degradation
// loud at process start.
func TestMetricsEndpointOpenWhenNoSecret_SEC006(t *testing.T) {
	t.Setenv("ORCHESTRATOR_SECRET", "")

	orch := createTestOrchestrator(t, false, false)
	// Port 0 → kernel-assigned ephemeral. See sibling test for rationale.
	server := NewHTTPServer(0, orch)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = server.Stop(stopCtx)
	})

	addr := server.Addr()
	if addr == nil {
		t.Fatal("server.Addr() returned nil after Start")
	}

	client := &http.Client{Transport: &http.Transport{}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr.String()+"/metrics", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("dev-mode /metrics should serve without auth: got %d", resp.StatusCode)
	}
}

// BenchmarkHealthEndpoint benchmarks the health endpoint
func BenchmarkHealthEndpoint(b *testing.B) {
	orch := createTestOrchestrator(&testing.T{}, false, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.handleHealth(w, req)
	}
}

// BenchmarkReadinessEndpoint benchmarks the readiness endpoint
func BenchmarkReadinessEndpoint(b *testing.B) {
	orch := createTestOrchestrator(&testing.T{}, false, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readiness", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.handleReadiness(w, req)
	}
}
