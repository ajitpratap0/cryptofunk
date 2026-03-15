package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// setupMinimalServer creates an APIServer with minimal config for unit testing
// (no database, no testcontainers needed)
func setupMinimalServer(t *testing.T) *APIServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{
			Name:        "CryptoFunk",
			Version:     "1.0.0-test",
			Environment: "test",
			LogLevel:    "debug",
		},
		API: config.APIConfig{
			Host:            "localhost",
			Port:            8081,
			OrchestratorURL: "http://localhost:8082",
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "cryptofunk_test",
			SSLMode:  "disable",
			PoolSize: 5,
		},
		Trading: config.TradingConfig{
			Mode:            "paper",
			Symbols:         []string{"BTCUSDT"},
			Exchange:        "binance",
			InitialCapital:  10000,
			MaxPositions:    5,
			DefaultQuantity: 0.01,
		},
		Risk: config.RiskConfig{
			MaxPositionSize:     0.1,
			MaxDailyLoss:        0.05,
			MaxDrawdown:         0.2,
			DefaultStopLoss:     0.02,
			DefaultTakeProfit:   0.05,
			LLMApprovalRequired: false,
			MinConfidence:       0.6,
		},
	}

	hub := NewHub()
	go hub.Run()

	server := &APIServer{
		router:             gin.New(),
		config:             cfg,
		hub:                hub,
		port:               "8081",
		orchestratorClient: defaultOrchestratorClient,
		orderExecutorURL:   "http://localhost:8091/mcp",
		orderExecClient:    &http.Client{Timeout: 5 * time.Second},
		// db is nil — tests for endpoints that don't need DB
	}

	return server
}

// TestSanitizeConfig tests that config sanitization works correctly
func TestSanitizeConfig(t *testing.T) {
	server := setupMinimalServer(t)

	sanitized := server.sanitizeConfig(server.config)

	// Should have all expected sections
	assert.Contains(t, sanitized, "app")
	assert.Contains(t, sanitized, "database")
	assert.Contains(t, sanitized, "api")
	assert.Contains(t, sanitized, "trading")
	assert.Contains(t, sanitized, "risk")
	assert.Contains(t, sanitized, "llm")
	assert.Contains(t, sanitized, "monitoring")
	assert.Contains(t, sanitized, "mcp")

	// Should NOT contain sensitive fields
	dbSection := sanitized["database"].(map[string]interface{})
	assert.NotContains(t, dbSection, "password")
	assert.NotContains(t, dbSection, "user")

	// Should contain safe fields
	assert.Contains(t, dbSection, "host")
	assert.Contains(t, dbSection, "port")

	// API section should exist
	apiSection := sanitized["api"].(map[string]interface{})
	assert.Contains(t, apiSection, "host")
	assert.Contains(t, apiSection, "port")
}

// TestTruncateString tests the truncateString helper
func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"},
		{"", 5, ""},
		{"abcdefgh", 8, "abcdefgh"},
		{"abcdefghi", 8, "abcde..."},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+string(rune(tt.maxLen+'0')), func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetPort tests port retrieval
func TestGetPort(t *testing.T) {
	port := getPort()
	assert.NotEmpty(t, port)
}

// TestParseUUID tests UUID parsing
func TestParseUUID(t *testing.T) {
	// Valid UUID
	_, err := parseUUID("550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)

	// Invalid UUID
	_, err = parseUUID("invalid")
	assert.Error(t, err)

	// Empty string
	_, err = parseUUID("")
	assert.Error(t, err)
}

// TestWebSocketUpgraderDev tests WebSocket upgrader in dev mode
func TestWebSocketUpgraderDev(t *testing.T) {
	server := setupMinimalServer(t)
	upgrader := server.createWebSocketUpgrader()
	assert.NotNil(t, upgrader)
	assert.Equal(t, 4096, upgrader.ReadBufferSize)
	assert.Equal(t, 4096, upgrader.WriteBufferSize)
}

// TestWebSocketUpgraderProd tests WebSocket upgrader in production mode
func TestWebSocketUpgraderProd(t *testing.T) {
	server := setupMinimalServer(t)
	server.config.App.Environment = "production"
	server.config.API.AllowedOrigins = []string{"https://app.cryptofunk.io"}

	upgrader := server.createWebSocketUpgrader()
	assert.NotNil(t, upgrader)

	// Test origin check
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/ws", nil)
	req.Header.Set("Origin", "https://app.cryptofunk.io")
	assert.True(t, upgrader.CheckOrigin(req))

	req2, _ := http.NewRequestWithContext(context.Background(), "GET", "/ws", nil)
	req2.Header.Set("Origin", "https://evil.com")
	assert.False(t, upgrader.CheckOrigin(req2))

	// No origin in production should be rejected
	req3, _ := http.NewRequestWithContext(context.Background(), "GET", "/ws", nil)
	assert.False(t, upgrader.CheckOrigin(req3))
}

// TestConfigEndpoint_Unit tests config endpoint returns "api" section
func TestConfigEndpoint_Unit(t *testing.T) {
	server := setupMinimalServer(t)

	// Manually set up minimal routes
	server.router.Use(gin.Recovery())
	server.router.GET("/api/v1/config", server.handleGetConfig)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "config")
	cfg := response["config"].(map[string]interface{})
	assert.Contains(t, cfg, "api")
	assert.Contains(t, cfg, "trading")
	assert.Contains(t, cfg, "risk")
	assert.Contains(t, cfg, "database")
}

// TestStatusEndpoint_Unit tests status endpoint returns expected structure
func TestStatusEndpoint_Unit(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.GET("/api/v1/status", server.handleStatus)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "status")
	assert.Contains(t, response, "uptime")
	assert.Contains(t, response, "components")
	assert.Contains(t, response, "websocket")

	components := response["components"].(map[string]interface{})
	assert.Equal(t, "healthy", components["database"])
	assert.Equal(t, "healthy", components["api"])
	assert.Equal(t, "healthy", components["websocket"])
}

// TestUpdateConfig_Unit tests config update endpoint
func TestUpdateConfig_Unit(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.PATCH("/api/v1/config", server.handleUpdateConfig)

	// Test valid update
	body := `{"trading_mode": "paper", "initial_capital": 5000}`
	req := httptest.NewRequestWithContext(context.Background(), "PATCH", "/api/v1/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "updates")

	// Verify config was updated
	assert.Equal(t, "paper", server.config.Trading.Mode)
	assert.Equal(t, 5000.0, server.config.Trading.InitialCapital)
}

// TestUpdateConfig_InvalidMode tests config update with invalid mode
func TestUpdateConfig_InvalidMode(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.PATCH("/api/v1/config", server.handleUpdateConfig)

	body := `{"trading_mode": "invalid_mode"}`
	req := httptest.NewRequestWithContext(context.Background(), "PATCH", "/api/v1/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateConfig_EmptyBody tests config update with empty body
func TestUpdateConfig_EmptyBody(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.PATCH("/api/v1/config", server.handleUpdateConfig)

	body := `{}`
	req := httptest.NewRequestWithContext(context.Background(), "PATCH", "/api/v1/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateConfig_RiskParams tests updating risk parameters
func TestUpdateConfig_RiskParams(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.PATCH("/api/v1/config", server.handleUpdateConfig)

	body := `{"max_position_size": 0.05, "max_daily_loss": 0.03, "max_drawdown": 0.15}`
	req := httptest.NewRequestWithContext(context.Background(), "PATCH", "/api/v1/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0.05, server.config.Risk.MaxPositionSize)
	assert.Equal(t, 0.03, server.config.Risk.MaxDailyLoss)
	assert.Equal(t, 0.15, server.config.Risk.MaxDrawdown)
}

// TestPlaceOrder_InvalidJSON tests place order with invalid JSON
func TestPlaceOrder_InvalidJSON(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/orders", server.handlePlaceOrder)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPlaceOrder_MissingSymbol tests place order with missing symbol
func TestPlaceOrder_MissingSymbol(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/orders", server.handlePlaceOrder)

	body := `{"side": "BUY", "type": "MARKET", "quantity": 0.01}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPlaceOrder_LimitNoPrice tests limit order without price
func TestPlaceOrder_LimitNoPrice(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/orders", server.handlePlaceOrder)

	body := `{"symbol": "BTCUSDT", "side": "BUY", "type": "LIMIT", "quantity": 0.01}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStartTrading_InvalidJSON tests start trading with bad JSON
func TestStartTrading_InvalidJSON(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/start", server.handleStartTrading)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/start", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStartTrading_MissingFields tests start trading with missing required fields
func TestStartTrading_MissingFields(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/start", server.handleStartTrading)

	body := `{"mode": "paper"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestStopTrading_InvalidSessionID tests stop trading with invalid session UUID
func TestStopTrading_InvalidUUID(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/stop", server.handleStopTrading)

	body := `{"session_id": "not-a-uuid", "final_capital": 10000}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/stop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCancelOrder_InvalidUUID tests cancel order with invalid UUID
func TestCancelOrder_InvalidUUID(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.DELETE("/api/v1/orders/:id", server.handleCancelOrder)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/orders/invalid-id", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetOrder_InvalidUUID tests get order with invalid UUID
func TestGetOrder_InvalidUUID(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.GET("/api/v1/orders/:id", server.handleGetOrder)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/orders/not-a-uuid", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPauseTrading_InvalidJSON tests pause trading with invalid body
func TestPauseTrading_InvalidJSON(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/pause", server.handlePauseTrading)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/pause", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPauseTrading_InvalidUUID tests pause trading with invalid session UUID
func TestPauseTrading_InvalidUUID(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/pause", server.handlePauseTrading)

	body := `{"session_id": "not-a-uuid"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/pause", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeTrading_InvalidJSON tests resume trading with invalid body
func TestResumeTrading_InvalidJSON(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/resume", server.handleResumeTrading)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/resume", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResumeTrading_InvalidUUID tests resume trading with invalid session UUID
func TestResumeTrading_InvalidUUID(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.POST("/api/v1/trade/resume", server.handleResumeTrading)

	body := `{"session_id": "not-a-uuid"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/trade/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListPositions_InvalidSessionID tests positions with bad session_id query
func TestListPositions_InvalidSessionID(t *testing.T) {
	server := setupMinimalServer(t)

	server.router.Use(gin.Recovery())
	server.router.GET("/api/v1/positions", server.handleListPositions)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/positions?session_id=bad-uuid", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRateLimiterMiddleware_Headers tests rate limit response headers
func TestRateLimiterMiddleware_Headers(t *testing.T) {
	rl := NewRateLimiter("test", 2, 60*60*1000000000) // 1hr window

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

// TestRateLimiterMiddleware_Blocked tests rate limit 429 response
func TestRateLimiterMiddleware_Blocked(t *testing.T) {
	rl := NewRateLimiter("test", 1, 60*60*1000000000)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request — allowed
	req1 := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request — blocked
	req2 := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.NotEmpty(t, w2.Header().Get("Retry-After"))

	var body map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body["error"], "rate limit")
}

// TestRateLimiterMiddleware_Config tests RateLimiterMiddleware configuration
func TestRateLimiterMiddleware_Config(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	assert.Equal(t, 100, cfg.GlobalMaxRequests)
	assert.Equal(t, 10, cfg.ControlMaxRequests)
	assert.Equal(t, 30, cfg.OrderMaxRequests)
	assert.Equal(t, 60, cfg.ReadMaxRequests)
	assert.Equal(t, 20, cfg.SearchMaxRequests)
	assert.True(t, cfg.Enabled)

	rlm := NewRateLimiterMiddleware(cfg)
	assert.NotNil(t, rlm)

	// Test disabled middleware
	disabledCfg := DefaultRateLimiterConfig()
	disabledCfg.Enabled = false
	rlm2 := NewRateLimiterMiddleware(disabledCfg)

	// Disabled middleware should pass through
	router := gin.New()
	router.Use(rlm2.GlobalMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	for i := 0; i < 200; i++ {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

// TestRateLimiterCleanup tests cleanup of stale entries
func TestRateLimiterCleanup(t *testing.T) {
	rlm := NewRateLimiterMiddleware(DefaultRateLimiterConfig())

	// Make some requests
	rlm.global.allow("1.2.3.4")
	rlm.control.allow("1.2.3.4")

	// Cleanup should not panic
	assert.NotPanics(t, func() {
		rlm.CleanupOldEntries()
	})
}

// TestRateLimiterStop tests stop without starting
func TestRateLimiterStop(t *testing.T) {
	rlm := NewRateLimiterMiddleware(DefaultRateLimiterConfig())

	// Stop without start should not panic
	assert.NotPanics(t, func() {
		rlm.Stop()
	})
}

// TestSecurityHeaders tests security headers middleware
func TestSecurityHeaders(t *testing.T) {
	router := gin.New()
	router.Use(securityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
}

// TestDetermineEventType tests audit event type determination
func TestDetermineEventType(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"POST", "/api/v1/trade/start", "TRADING_START"},
		{"POST", "/api/v1/trade/stop", "TRADING_STOP"},
		{"POST", "/api/v1/trade/pause", "TRADING_PAUSE"},
		{"POST", "/api/v1/trade/resume", "TRADING_RESUME"},
		{"POST", "/api/v1/orders", "ORDER_PLACED"},
		{"DELETE", "/api/v1/orders/123", "ORDER_CANCELED"},
		{"PATCH", "/api/v1/config", "CONFIG_UPDATED"},
		{"GET", "/api/v1/config", "CONFIG_VIEWED"},
		{"GET", "/api/v1/health", ""}, // Non-critical, empty
		{"GET", "/api/v1/agents", ""}, // Non-critical, empty
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			result := determineEventType(tt.method, tt.path)
			assert.Equal(t, string(tt.expected), string(result))
		})
	}
}

// TestRateLimiterCheck tests the check method with rate limit info
func TestRateLimiterCheck(t *testing.T) {
	rl := NewRateLimiter("test", 3, 1000000000) // 1 sec window

	info1 := rl.check("10.0.0.1")
	assert.True(t, info1.Allowed)
	assert.Equal(t, 3, info1.Limit)
	assert.Equal(t, 2, info1.Remaining)

	info2 := rl.check("10.0.0.1")
	assert.True(t, info2.Allowed)
	assert.Equal(t, 1, info2.Remaining)

	info3 := rl.check("10.0.0.1")
	assert.True(t, info3.Allowed)
	assert.Equal(t, 0, info3.Remaining)

	info4 := rl.check("10.0.0.1")
	assert.False(t, info4.Allowed)
	assert.Equal(t, 0, info4.Remaining)
}
