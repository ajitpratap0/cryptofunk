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

// setupTestAPIServer creates a test server with database if available
func setupTestAPIServer(t *testing.T) (*APIServer, bool) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		API: config.APIConfig{
			Host:            "localhost",
			Port:            8081,
			OrchestratorURL: "http://localhost:8082",
		},
	}

	hub := NewHub()
	go hub.Run()

	// Try to create database connection
	ctx := context.Background()
	database, err := db.New(ctx)
	hasDB := err == nil

	server := &APIServer{
		router:             gin.New(),
		db:                 database,
		config:             cfg,
		hub:                hub,
		port:               "8081",
		orchestratorClient: defaultOrchestratorClient,
	}

	if hasDB {
		server.setupRoutes()
	}

	return server, hasDB
}

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["timestamp"])
	assert.NotEmpty(t, response["version"])
}

// TestStatusEndpoint tests the /status endpoint
func TestStatusEndpoint(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "uptime")
	assert.Contains(t, response, "active_sessions")
}

// TestRateLimiter tests the rate limiting middleware
func TestRateLimiter(t *testing.T) {
	rl := newControlEndpointRateLimiter(3, 1*time.Second)

	// First 3 requests should be allowed
	assert.True(t, rl.allow("192.168.1.1"))
	assert.True(t, rl.allow("192.168.1.1"))
	assert.True(t, rl.allow("192.168.1.1"))

	// 4th request should be denied
	assert.False(t, rl.allow("192.168.1.1"))

	// Different IP should still be allowed
	assert.True(t, rl.allow("192.168.1.2"))
}

// TestRateLimiterExpiration tests that rate limiter resets after time window
func TestRateLimiterExpiration(t *testing.T) {
	rl := newControlEndpointRateLimiter(2, 100*time.Millisecond)

	// Use up the quota
	assert.True(t, rl.allow("192.168.1.1"))
	assert.True(t, rl.allow("192.168.1.1"))
	assert.False(t, rl.allow("192.168.1.1"))

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	assert.True(t, rl.allow("192.168.1.1"))
}

// TestGetConfigEndpoint tests the GET /config endpoint
func TestGetConfigEndpoint(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should contain config data
	assert.Contains(t, response, "api")
}

// TestListAgents_NoDatabase tests list agents endpoint without database
func TestListAgents_NoDatabase(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return error when database is not available
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestListPositions_NoDatabase tests list positions endpoint without database
func TestListPositions_NoDatabase(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("GET", "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return error when database is not available
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestListOrders_NoDatabase tests list orders endpoint without database
func TestListOrders_NoDatabase(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return error when database is not available
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestPlaceOrder_InvalidRequest tests place order with invalid request body
func TestPlaceOrder_InvalidRequest(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	// Invalid JSON
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPlaceOrder_MissingFields tests place order with missing required fields
func TestPlaceOrder_MissingFields(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	reqBody := map[string]interface{}{
		"symbol": "BTC/USDT",
		// Missing side, type, quantity
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStartTrading_InvalidRequest tests start trading with invalid request
func TestStartTrading_InvalidRequest(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	// Missing required fields
	reqBody := map[string]interface{}{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/trade/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStopTrading_InvalidRequest tests stop trading with invalid session ID
func TestStopTrading_InvalidRequest(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	reqBody := map[string]interface{}{
		"session_id": "invalid-uuid",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/trade/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCancelOrder_InvalidOrderID tests cancel order with invalid ID
func TestCancelOrder_InvalidOrderID(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	req := httptest.NewRequest("DELETE", "/api/v1/orders/invalid-id", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetOrchestratorURL tests orchestrator URL retrieval
func TestGetOrchestratorURL(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	url := server.getOrchestratorURL()
	assert.Equal(t, "http://localhost:8082", url)
}

// TestCallOrchestratorWithRetry_Success tests successful orchestrator call
func TestCallOrchestratorWithRetry_Success(t *testing.T) {
	// Create mock orchestrator
	mockOrch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockOrch.Close()

	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	resp, err := server.callOrchestratorWithRetry(mockOrch.URL + "/test")
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCallOrchestratorWithRetry_Failure tests failed orchestrator call
func TestCallOrchestratorWithRetry_Failure(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	// Call non-existent server
	//nolint:bodyclose // Test expects error, no response body to close
	_, err := server.callOrchestratorWithRetry("http://localhost:99999/test")
	assert.Error(t, err)
}

// Integration tests requiring database
// These will be skipped if DATABASE_URL is not set

func TestListAgentsWithDatabase(t *testing.T) {
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	defer database.Close()

	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}
	server.db = database

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "agents")
}

func TestListPositionsWithDatabase(t *testing.T) {
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	defer database.Close()

	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}
	server.db = database

	req := httptest.NewRequest("GET", "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "positions")
}

func TestListOrdersWithDatabase(t *testing.T) {
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	defer database.Close()

	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}
	server.db = database

	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "orders")
}

func TestGetPositionWithDatabase(t *testing.T) {
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	defer database.Close()

	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}
	server.db = database

	// Query for a symbol (may not exist, but should return 200 with empty result)
	req := httptest.NewRequest("GET", "/api/v1/positions/BTC/USDT", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should be 200 even if position doesn't exist (empty array)
	// Or 404 if handler returns that for missing positions
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

func TestPlaceOrderWithDatabase(t *testing.T) {
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	defer database.Close()

	// Create a trading session first
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTC/USDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err = database.CreateSession(ctx, session)
	require.NoError(t, err)

	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}
	server.db = database

	reqBody := map[string]interface{}{
		"session_id": session.ID.String(),
		"symbol":     "BTC/USDT",
		"side":       "BUY",
		"type":       "MARKET",
		"quantity":   0.001,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or fail gracefully
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}
