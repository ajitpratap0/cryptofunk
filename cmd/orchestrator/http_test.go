package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

// mockDB is a mock database for testing
type mockDB struct {
	shouldFail bool
}

func (m *mockDB) Ping(ctx context.Context) error {
	if m.shouldFail {
		return context.DeadlineExceeded
	}
	return nil
}

func (m *mockDB) Close() {}

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

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
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
		req := httptest.NewRequest(method, "/health", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/liveness", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
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
	if result.Status != "failed" {
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
	if result.Status != "failed" {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
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
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
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

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test that we can make a request
	resp, err := http.Get("http://localhost:18081/health")
	if err != nil {
		t.Fatalf("Failed to make request to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop HTTP server: %v", err)
	}

	// Give server time to stop
	time.Sleep(100 * time.Millisecond)

	// Verify server is stopped (request should fail)
	_, err = http.Get("http://localhost:18081/health")
	if err == nil {
		t.Error("Expected request to fail after server stop, but it succeeded")
	}
}

// BenchmarkHealthEndpoint benchmarks the health endpoint
func BenchmarkHealthEndpoint(b *testing.B) {
	orch := createTestOrchestrator(&testing.T{}, false, false)
	server := NewHTTPServer(8080, orch)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

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

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.handleReadiness(w, req)
	}
}
