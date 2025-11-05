package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a test API server with mocked dependencies
func setupTestServer(t *testing.T, mockOrchestrator *httptest.Server) *APIServer {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create test config
	cfg := &config.Config{
		API: config.APIConfig{
			Host:            "localhost",
			Port:            8081,
			OrchestratorURL: mockOrchestrator.URL,
		},
	}

	// Create test database connection (requires test database)
	// Skip if DATABASE_URL is not set
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}

	// Create test hub
	hub := NewHub()
	go hub.Run()

	// Create server
	server := &APIServer{
		router:             gin.New(),
		db:                 database,
		config:             cfg,
		hub:                hub,
		port:               "8081",
		orchestratorClient: defaultOrchestratorClient,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// TestPauseTrading_Success tests successful pause operation
func TestPauseTrading_Success(t *testing.T) {
	// Create mock orchestrator
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/pause", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "paused",
		}); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer mockOrchestrator.Close()

	// Setup test server
	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Create a test trading session
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTCUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create request
	reqBody := map[string]string{
		"session_id": session.ID.String(),
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute request
	server.router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Trading paused successfully", response["message"])
	assert.Equal(t, session.ID.String(), response["session_id"])
}

// TestPauseTrading_InvalidSessionID tests pause with invalid session
func TestPauseTrading_InvalidSessionID(t *testing.T) {
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockOrchestrator.Close()

	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Request with invalid UUID
	reqBody := map[string]string{
		"session_id": "invalid-uuid",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	assert.Equal(t, "invalid session_id format", response["error"])
}

// TestPauseTrading_NonExistentSession tests pause with non-existent session
func TestPauseTrading_NonExistentSession(t *testing.T) {
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockOrchestrator.Close()

	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Request with valid UUID but non-existent session
	reqBody := map[string]string{
		"session_id": uuid.New().String(),
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	assert.Equal(t, "session not found", response["error"])
}

// TestResumeTrading_Success tests successful resume operation
func TestResumeTrading_Success(t *testing.T) {
	// Create mock orchestrator
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/resume", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "resumed",
		}); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer mockOrchestrator.Close()

	// Setup test server
	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Create a test trading session
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "ETHUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 5000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create request
	reqBody := map[string]string{
		"session_id": session.ID.String(),
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/trade/resume", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute request
	server.router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Trading resumed successfully", response["message"])
	assert.Equal(t, session.ID.String(), response["session_id"])
}

// TestOrchestratorFailure tests handling of orchestrator failure
func TestOrchestratorFailure(t *testing.T) {
	// Create mock orchestrator that fails
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockOrchestrator.Close()

	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Create a test trading session
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTCUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Test pause with orchestrator failure
	reqBody := map[string]string{
		"session_id": session.ID.String(),
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	assert.Equal(t, "orchestrator failed to pause trading", response["error"])
}

// TestRateLimiting tests that rate limiting is applied correctly
func TestRateLimiting(t *testing.T) {
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	}))
	defer mockOrchestrator.Close()

	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Create a test trading session
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTCUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session)
	require.NoError(t, err)

	reqBody := map[string]string{
		"session_id": session.ID.String(),
	}
	body, _ := json.Marshal(reqBody)

	// Make 11 requests rapidly (rate limit is 10 per minute)
	var successCount, rateLimitedCount int
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Set a consistent client IP for rate limiting
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		} else if w.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	// At least one request should be rate limited
	assert.GreaterOrEqual(t, rateLimitedCount, 1, "Expected at least one request to be rate limited")
	assert.LessOrEqual(t, successCount, 10, "Expected at most 10 successful requests")
}

// TestConcurrentPauseResume tests concurrent pause/resume operations
func TestConcurrentPauseResume(t *testing.T) {
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slight delay
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	}))
	defer mockOrchestrator.Close()

	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Create test sessions
	ctx := context.Background()
	session1 := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTCUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session1)
	require.NoError(t, err)

	session2 := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "ETHUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 5000.0,
		StartedAt:      time.Now(),
	}
	err = server.db.CreateSession(ctx, session2)
	require.NoError(t, err)

	// Run concurrent requests
	done := make(chan bool)
	errors := make(chan error, 4)

	// Concurrent pause for session1
	go func() {
		reqBody := map[string]string{"session_id": session1.ID.String()}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:10001"
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			errors <- assert.AnError
		}
		done <- true
	}()

	// Concurrent resume for session1
	go func() {
		time.Sleep(5 * time.Millisecond) // Slight offset
		reqBody := map[string]string{"session_id": session1.ID.String()}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/trade/resume", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:10002"
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			errors <- assert.AnError
		}
		done <- true
	}()

	// Concurrent pause for session2
	go func() {
		reqBody := map[string]string{"session_id": session2.ID.String()}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:10003"
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			errors <- assert.AnError
		}
		done <- true
	}()

	// Concurrent resume for session2
	go func() {
		time.Sleep(5 * time.Millisecond)
		reqBody := map[string]string{"session_id": session2.ID.String()}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/trade/resume", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:10004"
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			errors <- assert.AnError
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	close(errors)
	assert.Equal(t, 0, len(errors), "All concurrent requests should succeed")
}

// TestOrchestratorRetry tests retry logic when orchestrator is temporarily unavailable
func TestOrchestratorRetry(t *testing.T) {
	// Create mock orchestrator that fails first 2 times, then succeeds
	attemptCount := 0
	mockOrchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	}))
	defer mockOrchestrator.Close()

	server := setupTestServer(t, mockOrchestrator)
	defer server.db.Close()

	// Create a test trading session
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTCUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Make request
	reqBody := map[string]string{
		"session_id": session.ID.String(),
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/trade/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed after retries
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 3, attemptCount, "Should have retried 3 times")
}
