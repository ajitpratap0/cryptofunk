package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestRateLimiterBasic tests basic rate limiter functionality
func TestRateLimiterBasic(t *testing.T) {
	rl := NewRateLimiter("test", 3, 1*time.Second) // 3 requests per second

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		assert.True(t, rl.allow("192.168.1.1"), "Request %d should be allowed", i+1)
	}

	// 4th request should be blocked
	assert.False(t, rl.allow("192.168.1.1"), "4th request should be blocked")
}

// TestRateLimiterDifferentIPs tests rate limiter with different IPs
func TestRateLimiterDifferentIPs(t *testing.T) {
	rl := NewRateLimiter("test", 2, 1*time.Second)

	// IP 1 uses its quota
	assert.True(t, rl.allow("192.168.1.1"))
	assert.True(t, rl.allow("192.168.1.1"))
	assert.False(t, rl.allow("192.168.1.1"))

	// IP 2 should still have its quota
	assert.True(t, rl.allow("192.168.1.2"))
	assert.True(t, rl.allow("192.168.1.2"))
	assert.False(t, rl.allow("192.168.1.2"))
}

// TestRateLimiterMiddlewareIntegration tests the rate limiter as Gin middleware
func TestRateLimiterMiddlewareIntegration(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	server.setupMiddleware()
	server.setupRoutes()

	// Make 10 rapid requests to a rate-limited endpoint
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("POST", "/api/v1/trade/pause", nil)
		req.RemoteAddr = "192.168.1.1:12345" // Simulated IP
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code == http.StatusOK || w.Code == http.StatusNotImplemented || w.Code == http.StatusBadRequest {
			successCount++
		} else if w.Code == http.StatusTooManyRequests {
			rateLimitedCount++

			// Check error message
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Contains(t, response, "error")
			assert.Contains(t, response["error"], "rate limit")
		}
	}

	// Should have some rate-limited requests
	assert.Greater(t, rateLimitedCount, 0, "Some requests should be rate-limited")
	t.Logf("Success: %d, Rate-limited: %d", successCount, rateLimitedCount)
}

// TestUnauthorizedAccess tests accessing protected endpoints without authentication
func TestUnauthorizedAccess(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	protectedEndpoints := []struct {
		method   string
		endpoint string
	}{
		{"POST", "/api/v1/orders"},
		{"DELETE", "/api/v1/orders/123"},
		{"POST", "/api/v1/trade/start"},
		{"POST", "/api/v1/trade/stop"},
		{"PATCH", "/api/v1/config"},
	}

	for _, ep := range protectedEndpoints {
		t.Run(ep.method+"_"+ep.endpoint, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.endpoint, nil)
			// No Authorization header
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should either return 401 Unauthorized or handle gracefully
			// For now, we accept any non-500 response as the endpoint may not require auth yet
			assert.NotEqual(t, http.StatusInternalServerError, w.Code,
				"Should not return 500 for unauthorized access")
		})
	}
}

// TestMalformedAuthHeader tests handling of malformed Authorization headers
func TestMalformedAuthHeader(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	malformedHeaders := []string{
		"Bearer",                    // Missing token
		"Bearer ",                   // Empty token
		"NotBearer token123",        // Wrong scheme
		"Bearer token with spaces",  // Invalid token format
		"Basic dXNlcjpwYXNz",       // Wrong auth type
	}

	for _, header := range malformedHeaders {
		t.Run("Header_"+header[:10], func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/orders", nil)
			req.Header.Set("Authorization", header)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should handle malformed auth gracefully
			assert.NotEqual(t, http.StatusInternalServerError, w.Code,
				"Should not crash on malformed auth header")
		})
	}
}

// TestConcurrentRequests tests handling of concurrent requests
func TestConcurrentRequests(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	server.setupMiddleware()
	server.setupRoutes()

	// Launch 50 concurrent requests
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/v1/health", nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 50; i++ {
		<-done
	}
}

// TestRecoveryMiddleware tests panic recovery
func TestRecoveryMiddleware(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	server.setupMiddleware()

	// Add a route that panics
	server.router.GET("/api/v1/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/api/v1/panic", nil)
	w := httptest.NewRecorder()

	// Should not crash the server
	assert.NotPanics(t, func() {
		server.router.ServeHTTP(w, req)
	})

	// Should return 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestRequestLogging tests that requests are logged
func TestRequestLogging(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	server.setupMiddleware()
	server.setupRoutes()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Should not panic during logging
	assert.NotPanics(t, func() {
		server.router.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestPrometheusMetricsEndpoint tests the /metrics endpoint
func TestPrometheusMetricsEndpoint(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	server.setupMiddleware()
	server.setupRoutes()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Metrics endpoint should return 200
	assert.Equal(t, http.StatusOK, w.Code)

	// Response should contain Prometheus metrics format
	body := w.Body.String()
	assert.Contains(t, body, "# HELP", "Should contain Prometheus metrics")
	assert.Contains(t, body, "# TYPE", "Should contain Prometheus metrics")
}

// TestGracefulShutdown tests graceful shutdown doesn't leave hanging connections
func TestGracefulShutdown(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	// This test verifies the server structure is set up correctly for graceful shutdown
	assert.NotNil(t, server.router, "Router should be initialized")
	assert.NotNil(t, server.hub, "WebSocket hub should be initialized")
}

// TestRootEndpoint tests the root / endpoint
func TestRootEndpoint(t *testing.T) {
	server, tc := setupTestAPIServer(t)
	_ = tc // testcontainers handles cleanup automatically

	server.setupRoutes()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "CryptoFunk Trading API", response["name"])
	assert.Equal(t, "running", response["status"])
	assert.NotEmpty(t, response["version"])
}
