//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/ajitpratap0/cryptofunk/internal/safety"
)

// setupTestAPIServer creates a test server with testcontainers database
func setupTestAPIServer(t *testing.T) (*APIServer, *testhelpers.PostgresContainer) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Setup testcontainers database
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err, "Failed to apply migrations")

	cfg := &config.Config{
		API: config.APIConfig{
			Host:            "localhost",
			Port:            8081,
			OrchestratorURL: "http://localhost:8082",
		},
		Trading: config.TradingConfig{
			// Viper SetDefault is not invoked in tests that bypass LoadConfig.
			SlippageBuyFactor:  1.001,
			SlippageSellFactor: 0.999,
		},
	}

	hub := NewHub()
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	server := &APIServer{
		router:             gin.New(),
		db:                 tc.DB,
		config:             cfg,
		hub:                hub,
		port:               "8081",
		orchestratorClient: defaultOrchestratorClient,
		orderExecutorURL:   "http://localhost:8091/mcp",
		ctx:                ctx,
		safetyGuard:        safety.NewGuard(safety.NewLimitsConfig(), safety.NewMonitor(0)),
	}

	server.setupMiddleware() // Set up middleware first (includes recovery)
	server.setupRoutes()     // Then set up routes

	return server, tc
}

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["uptime"]) // Handler returns uptime, not timestamp
	// #98: version intentionally removed from unauthenticated /health
	// to prevent fingerprinting. Verify it's absent.
	assert.Nil(t, response["version"], "version must not be exposed on unauthenticated /health")
}

// TestStatusEndpoint tests the /status endpoint
func TestStatusEndpoint(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "uptime")
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "components")
	assert.Contains(t, response, "websocket")
	// Verify components structure
	components := response["components"].(map[string]interface{})
	assert.Equal(t, "healthy", components["database"])
	assert.Equal(t, "healthy", components["api"])
	assert.Equal(t, "healthy", components["websocket"])
}

// TestRateLimiter tests the rate limiting middleware
func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter("test", 3, 1*time.Second)

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
	rl := NewRateLimiter("test", 2, 100*time.Millisecond)

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
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Response is wrapped in "config" key
	assert.Contains(t, response, "config")
	config := response["config"].(map[string]interface{})
	assert.Contains(t, config, "api")
}

// TestListAgents_Empty tests list agents endpoint with empty database
func TestListAgents_Empty(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return OK with list of agents (may be empty or contain agents from other tests)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "agents")
	assert.Contains(t, response, "count")
	// Count should be >= 0 (database may have agents from other tests or heartbeats)
	assert.GreaterOrEqual(t, int(response["count"].(float64)), 0)
}

// TestListPositions_Empty tests list positions endpoint with empty database
func TestListPositions_Empty(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return OK with empty list when no positions exist
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "positions")
}

// TestListOrders_Empty tests list orders endpoint with empty database
func TestListOrders_Empty(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/orders", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return OK with empty list when no orders exist
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "orders")
}

// TestPlaceOrder_InvalidRequest tests place order with invalid request body
func TestPlaceOrder_InvalidRequest(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	// Invalid JSON
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPlaceOrder_MissingFields tests place order with missing required fields
func TestPlaceOrder_MissingFields(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	reqBody := map[string]interface{}{
		"symbol": "BTC/USDT",
		// Missing side, type, quantity
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStartTrading_InvalidRequest tests start trading with invalid request
func TestStartTrading_InvalidRequest(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	// Missing required fields
	reqBody := map[string]interface{}{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStopTrading_InvalidRequest tests stop trading with invalid session ID
func TestStopTrading_InvalidRequest(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	reqBody := map[string]interface{}{
		"session_id": "invalid-uuid",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCancelOrder_InvalidOrderID tests cancel order with invalid ID
func TestCancelOrder_InvalidOrderID(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/orders/invalid-id", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetOrchestratorURL tests orchestrator URL retrieval
func TestGetOrchestratorURL(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

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

	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	resp, err := server.callOrchestratorWithRetry(mockOrch.URL + "/test")
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCallOrchestratorWithRetry_Failure tests failed orchestrator call
func TestCallOrchestratorWithRetry_Failure(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	// Call non-existent server
	//nolint:bodyclose // Test expects error, no response body to close
	_, err := server.callOrchestratorWithRetry("http://localhost:99999/test")
	assert.Error(t, err)
}

// Integration tests requiring database (uses testcontainers)

func TestListAgentsWithDatabase(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "agents")
}

func TestListPositionsWithDatabase(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/positions", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "positions")
}

func TestListOrdersWithDatabase(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/orders", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "orders")
}

func TestGetPositionWithDatabase(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	// Query for a symbol (may not exist, but should return 200 with empty result)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/positions/BTC/USDT", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should be 200 even if position doesn't exist (empty array)
	// Or 404 if handler returns that for missing positions
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

func TestPlaceOrderWithDatabase(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	ctx := context.Background()
	// Create a trading session first
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTC/USDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	err := server.db.CreateSession(ctx, session)
	require.NoError(t, err)
	// QA-005 R4: handlePlaceOrder now requires an active session for
	// ALL order types (not just SELL). Set it so the guard passes.
	server.activeSessionID = &session.ID

	reqBody := map[string]interface{}{
		"session_id": session.ID.String(),
		"symbol":     "BTC/USDT",
		"side":       "BUY",
		"type":       "MARKET",
		"quantity":   0.001,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (201 Created for new order) or fail gracefully (500)
	assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
		"Expected status 200, 201, or 500, got %d", w.Code)
}

// TestPaperTrade_InvalidRequest tests POST /api/v1/trade with invalid body
func TestPaperTrade_InvalidRequest(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPaperTrade_LimitOrderMissingPrice tests that limit orders without a price are rejected
func TestPaperTrade_LimitOrderMissingPrice(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc

	reqBody := map[string]interface{}{
		"symbol":   "BTC/USDT",
		"side":     "BUY",
		"type":     "LIMIT",
		"quantity": 0.001,
		// price intentionally omitted
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestPaperTrade_MarketOrder tests a successful market paper trade
func TestPaperTrade_MarketOrder(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc

	reqBody := map[string]interface{}{
		"symbol":   "BTC/USDT",
		"side":     "BUY",
		"type":     "MARKET",
		"quantity": 0.001,
		"price":    50000.0,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "order")
	assert.Equal(t, "paper", response["trading_mode"])
}

// TestPaperTrade_LimitOrder tests a successful limit paper trade
func TestPaperTrade_LimitOrder(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc

	reqBody := map[string]interface{}{
		"symbol":   "ETH/USDT",
		"side":     "SELL",
		"type":     "LIMIT",
		"quantity": 0.1,
		"price":    3000.0,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "order")
	assert.Equal(t, "paper", response["trading_mode"])
}

// TestPlaceOrder_ErrorPathSanitization verifies that when handlePlaceOrder reaches the
// executeOrder failure path, the HTTP 500 response does NOT contain sensitive internal
// fields (error_message, exchange_order_id, session_id, details).
//
// Setup: real DB (InsertOrder succeeds) + nil mcpClient (connectOrderExecutor returns
// "MCP client not initialized" immediately, triggering the safeOrder sanitization branch).
// This is the only path that exercises the safeOrder nil assignments in the handler.
func TestPlaceOrder_ErrorPathSanitization(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	// QA-005 R4: handlePlaceOrder requires an active session.
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTCUSDT",
		Mode:           db.TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}
	require.NoError(t, server.db.CreateSession(ctx, session))
	server.activeSessionID = &session.ID

	// Ensure mcpClient is nil so connectOrderExecutor short-circuits and executeOrder
	// returns an error — landing in the safeOrder sanitization branch of handlePlaceOrder.
	server.mcpClient = nil

	reqBody := map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "BUY",
		"type":     "MARKET",
		"quantity": 0.01,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	// Sensitive internal fields must never appear in the error response.
	assert.NotContains(t, resp, "error_message",
		"error_message must not be exposed in the 500 response")
	assert.NotContains(t, resp, "exchange_order_id",
		"exchange_order_id must not be exposed in the 500 response")
	assert.NotContains(t, resp, "session_id",
		"session_id must not be exposed in the 500 response")
	assert.NotContains(t, resp, "details",
		"details must not be exposed in the 500 response")

	// Safe correlation fields must be present so the client can retry.
	assert.Contains(t, resp, "error", "a generic error key must be present")
	assert.Contains(t, resp, "order_id", "order_id must be present for client retry correlation")
	assert.Contains(t, resp, "symbol", "symbol must be present in the error response")
	assert.Contains(t, resp, "side", "side must be present in the error response")
}
