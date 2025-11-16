package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInputValidationSQLInjection tests SQL injection prevention
func TestInputValidationSQLInjection(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	tests := []struct {
		name     string
		endpoint string
		method   string
		body     string
		expected int
	}{
		{
			name:     "SQL injection in order ID",
			endpoint: "/api/v1/orders/1' OR '1'='1",
			method:   "GET",
			expected: http.StatusBadRequest,
		},
		{
			name:     "SQL injection in symbol",
			endpoint: "/api/v1/positions/BTC'; DROP TABLE positions--",
			method:   "GET",
			expected: http.StatusBadRequest,
		},
		{
			name:     "SQL injection in agent name",
			endpoint: "/api/v1/agents/agent' UNION SELECT * FROM users--",
			method:   "GET",
			expected: http.StatusNotFound, // Should safely return 404, not execute query
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.endpoint, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.endpoint, nil)
			}
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not return 500 (internal server error) for SQL injection attempts
			assert.NotEqual(t, http.StatusInternalServerError, w.Code, "SQL injection should not cause internal server error")
		})
	}
}

// TestInputValidationXSS tests XSS prevention
func TestInputValidationXSS(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"javascript:alert('XSS')",
		"<iframe src='javascript:alert(1)'>",
	}

	for _, payload := range xssPayloads {
		t.Run("XSS_"+payload[:10], func(t *testing.T) {
			body := map[string]interface{}{
				"symbol":   payload,
				"quantity": 1.0,
				"side":     "BUY",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should reject XSS attempts with 400 Bad Request
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnauthorized,
				"XSS payload should be rejected")

			// Response should not contain unescaped script tags
			responseBody := w.Body.String()
			assert.NotContains(t, responseBody, "<script>", "Response should not contain unescaped script tags")
		})
	}
}

// TestInputValidationOversizedPayload tests protection against large payloads
func TestInputValidationOversizedPayload(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	// Create a 10MB payload (should be rejected)
	largePayload := strings.Repeat("A", 10*1024*1024)
	body := map[string]interface{}{
		"data": largePayload,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should reject oversized payload
	assert.True(t, w.Code == http.StatusRequestEntityTooLarge || w.Code == http.StatusBadRequest,
		"Oversized payload should be rejected")
}

// TestInputValidationInvalidJSON tests invalid JSON handling
func TestInputValidationInvalidJSON(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	invalidJSONPayloads := []string{
		"{invalid json}",
		"{'single': 'quotes'}",
		"{\"unclosed\": ",
		"not json at all",
		"",
	}

	for i, payload := range invalidJSONPayloads {
		t.Run("InvalidJSON_"+string(rune(i+'0')), func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should return 400 Bad Request for invalid JSON
			assert.Equal(t, http.StatusBadRequest, w.Code, "Invalid JSON should return 400")

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err == nil {
				// If response is valid JSON, it should contain an error message
				assert.Contains(t, response, "error", "Error response should contain error field")
			}
		})
	}
}

// TestInputValidationCommandInjection tests command injection prevention
func TestInputValidationCommandInjection(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	commandInjectionPayloads := []string{
		"; ls -la",
		"| cat /etc/passwd",
		"`whoami`",
		"$(cat /etc/passwd)",
		"& ping -c 10 google.com &",
	}

	for _, payload := range commandInjectionPayloads {
		t.Run("CommandInjection_"+payload[:5], func(t *testing.T) {
			// Try command injection in agent name parameter
			req := httptest.NewRequest("GET", "/api/v1/agents/"+payload, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should safely handle command injection attempts
			assert.NotEqual(t, http.StatusInternalServerError, w.Code,
				"Command injection should not cause internal server error")
		})
	}
}

// TestInputValidationPathTraversal tests path traversal prevention
func TestInputValidationPathTraversal(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	pathTraversalPayloads := []string{
		"../../etc/passwd",
		"..\\..\\windows\\system32\\config\\sam",
		"/etc/passwd",
		"....//....//etc/passwd",
		"%2e%2e%2f%2e%2e%2fetc%2fpasswd",
	}

	for _, payload := range pathTraversalPayloads {
		t.Run("PathTraversal_"+payload[:10], func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/agents/"+payload, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not allow path traversal
			assert.NotEqual(t, http.StatusOK, w.Code,
				"Path traversal should not succeed")
			assert.NotEqual(t, http.StatusInternalServerError, w.Code,
				"Path traversal should not cause internal server error")
		})
	}
}

// TestHTTPMethodRestrictions tests that endpoints only accept correct HTTP methods
func TestHTTPMethodRestrictions(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	tests := []struct {
		endpoint      string
		allowedMethod string
		bannedMethods []string
	}{
		{
			endpoint:      "/api/v1/health",
			allowedMethod: "GET",
			bannedMethods: []string{"POST", "PUT", "DELETE", "PATCH"},
		},
		{
			endpoint:      "/api/v1/status",
			allowedMethod: "GET",
			bannedMethods: []string{"POST", "PUT", "DELETE", "PATCH"},
		},
		{
			endpoint:      "/api/v1/agents",
			allowedMethod: "GET",
			bannedMethods: []string{"POST", "PUT", "DELETE", "PATCH"},
		},
	}

	for _, tt := range tests {
		for _, method := range tt.bannedMethods {
			t.Run(tt.endpoint+"_"+method, func(t *testing.T) {
				req := httptest.NewRequest(method, tt.endpoint, nil)
				w := httptest.NewRecorder()

				server.router.ServeHTTP(w, req)

				// Should return 405 Method Not Allowed or 404 Not Found
				assert.True(t, w.Code == http.StatusMethodNotAllowed || w.Code == http.StatusNotFound,
					"Banned method should be rejected")
			})
		}
	}
}

// TestCORSHeaders tests CORS header configuration
func TestCORSHeaders(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	server.setupMiddleware()
	server.setupRoutes()

	// Test OPTIONS preflight request
	req := httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 or 204 for OPTIONS
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNoContent,
		"OPTIONS request should succeed")

	// Should have CORS headers
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"),
		"Should have Access-Control-Allow-Origin header")
}

// TestContentTypeValidation tests Content-Type header validation
func TestContentTypeValidation(t *testing.T) {
	server, hasDB := setupTestAPIServer(t)
	if !hasDB {
		t.Skip("Skipping test: DATABASE_URL not set or database not available")
	}
	if server.db != nil {
		defer server.db.Close()
	}

	// Valid JSON request with wrong Content-Type
	body := map[string]interface{}{
		"symbol":   "BTCUSDT",
		"quantity": 1.0,
		"side":     "BUY",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "text/plain") // Wrong content type
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should reject or handle gracefully
	assert.NotEqual(t, http.StatusInternalServerError, w.Code,
		"Wrong Content-Type should not cause internal server error")
}
