package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/stretchr/testify/assert"
)

// TestRateLimiter_Allow tests the rate limiter allow method
func TestRateLimiter_Allow(t *testing.T) {
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

// TestRateLimiter_Expiration tests that rate limiter resets after time window
func TestRateLimiter_Expiration(t *testing.T) {
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

// TestRateLimiter_MultipleIPs tests rate limiting with multiple IPs
func TestRateLimiter_MultipleIPs(t *testing.T) {
	rl := newControlEndpointRateLimiter(2, 1*time.Second)

	// IP 1 uses quota
	assert.True(t, rl.allow("192.168.1.1"))
	assert.True(t, rl.allow("192.168.1.1"))
	assert.False(t, rl.allow("192.168.1.1"))

	// IP 2 still has quota
	assert.True(t, rl.allow("192.168.1.2"))
	assert.True(t, rl.allow("192.168.1.2"))
	assert.False(t, rl.allow("192.168.1.2"))

	// IP 3 still has quota
	assert.True(t, rl.allow("192.168.1.3"))
}

// TestGetOrchestratorURL tests orchestrator URL retrieval
func TestGetOrchestratorURL_Default(t *testing.T) {
	server := &APIServer{
		config: &config.Config{
			API: config.APIConfig{
				OrchestratorURL: "http://localhost:8082",
			},
		},
	}

	url := server.getOrchestratorURL()
	assert.Equal(t, "http://localhost:8082", url)
}

// TestGetOrchestratorURL_CustomPort tests orchestrator URL with custom port
func TestGetOrchestratorURL_CustomPort(t *testing.T) {
	server := &APIServer{
		config: &config.Config{
			API: config.APIConfig{
				OrchestratorURL: "http://orchestrator.example.com:9000",
			},
		},
	}

	url := server.getOrchestratorURL()
	assert.Equal(t, "http://orchestrator.example.com:9000", url)
}

// TestOrchestratorCall_Success tests successful orchestrator call
func TestOrchestratorCall_Success(t *testing.T) {
	// Create mock orchestrator
	callCount := 0
	mockOrch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockOrch.Close()

	server := &APIServer{
		orchestratorClient: defaultOrchestratorClient,
	}

	resp, err := server.callOrchestratorWithRetry(mockOrch.URL + "/test")
	assert.NoError(t, err)
	if resp != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
	assert.Equal(t, 1, callCount, "Should call orchestrator once on success")
}

// TestCallOrchestratorWithRetry_HTTPError tests that HTTP errors don't trigger retries
func TestCallOrchestratorWithRetry_HTTPError(t *testing.T) {
	// Note: The current implementation doesn't retry on HTTP status errors (e.g., 500)
	// It only retries on network errors. This test verifies that behavior.
	callCount := 0
	mockOrch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockOrch.Close()

	server := &APIServer{
		orchestratorClient: defaultOrchestratorClient,
	}

	resp, err := server.callOrchestratorWithRetry(mockOrch.URL + "/test")
	assert.NoError(t, err) // No error, just non-200 status
	if resp != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	}
	assert.Equal(t, 1, callCount, "Should not retry on HTTP status errors")
}

// TestCallOrchestratorWithRetry_NetworkRetry tests orchestrator call with network retries
func TestCallOrchestratorWithRetry_NetworkRetry(t *testing.T) {
	// Test retries on actual network errors by using a server that closes connection
	callCount := 0
	server := &APIServer{
		orchestratorClient: &http.Client{
			Timeout: 50 * time.Millisecond,
		},
	}

	// Create a channel to count attempts
	done := make(chan int)
	go func() {
		// This will retry on timeout/network errors
		//nolint:bodyclose // Test expects error, no response body to close
		_, _ = server.callOrchestratorWithRetry("http://10.255.255.1/test") // Non-routable IP
		done <- callCount
	}()

	select {
	case <-done:
		// Test completed
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out")
	}
}

// TestCallOrchestratorWithRetry_NetworkFailure tests orchestrator call with network failure
func TestCallOrchestratorWithRetry_NetworkFailure(t *testing.T) {
	server := &APIServer{
		orchestratorClient: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
	}

	// Call non-existent server
	//nolint:bodyclose // Test expects error, no response body to close
	_, err := server.callOrchestratorWithRetry("http://localhost:99999/test")
	assert.Error(t, err)
}

// TestRateLimiterMiddleware tests the rate limiter middleware
func TestRateLimiterMiddleware(t *testing.T) {
	rl := newControlEndpointRateLimiter(2, 1*time.Second)

	middleware := rl.Middleware()

	// Test that middleware function is created
	assert.NotNil(t, middleware)
}

// TestDefaultOrchestratorClient tests the default orchestrator client configuration
func TestDefaultOrchestratorClient(t *testing.T) {
	assert.NotNil(t, defaultOrchestratorClient)
	assert.Equal(t, 5*time.Second, defaultOrchestratorClient.Timeout)

	transport, ok := defaultOrchestratorClient.Transport.(*http.Transport)
	assert.True(t, ok, "Transport should be *http.Transport")
	if ok {
		assert.Equal(t, 10, transport.MaxIdleConns)
		assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
		assert.Equal(t, 30*time.Second, transport.IdleConnTimeout)
		assert.False(t, transport.DisableKeepAlives)
	}
}

// TestNewControlEndpointRateLimiter tests rate limiter creation
func TestNewControlEndpointRateLimiter(t *testing.T) {
	rl := newControlEndpointRateLimiter(5, 10*time.Second)

	assert.NotNil(t, rl)
	assert.Equal(t, 5, rl.maxRequests)
	assert.Equal(t, 10*time.Second, rl.window)
}

// TestRateLimiter_ConcurrentAccess tests rate limiter with concurrent requests
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := newControlEndpointRateLimiter(10, 1*time.Second)

	// Make concurrent requests from same IP
	done := make(chan bool, 20)
	allowed := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func() {
			result := rl.allow("192.168.1.1")
			allowed <- result
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Count how many were allowed
	allowedCount := 0
	deniedCount := 0
	for i := 0; i < 20; i++ {
		if <-allowed {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	// Should allow exactly 10 (the limit)
	assert.Equal(t, 10, allowedCount, "Should allow exactly maxRequests")
	assert.Equal(t, 10, deniedCount, "Should deny the rest")
}
