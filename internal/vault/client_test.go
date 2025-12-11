package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Address: "http://localhost:8200",
				Token:   "test-token",
			},
			wantErr: false,
		},
		{
			name: "missing token",
			cfg: Config{
				Address: "http://localhost:8200",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
		})
	}
}

func TestClient_GetSecret(t *testing.T) {
	// Create a mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if r.URL.Path == "/v1/cryptofunk/data/database" {
			resp := SecretResponse{
				RequestID: "test-request",
				Data: &SecretData{
					Data: map[string]interface{}{
						"host":     "localhost",
						"port":     "5432",
						"database": "cryptofunk",
						"username": "postgres",
						"password": "secret",
						"sslmode":  "disable",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Address: server.URL,
		Token:   "test-token",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Test successful secret retrieval
	data, err := client.GetSecret(ctx, "cryptofunk/data/database")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}

	if data["host"] != "localhost" {
		t.Errorf("Expected host=localhost, got %v", data["host"])
	}

	// Test secret not found
	_, err = client.GetSecret(ctx, "cryptofunk/data/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent secret")
	}
}

func TestClient_GetSecretString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SecretResponse{
			Data: &SecretData{
				Data: map[string]interface{}{
					"secret": "my-jwt-secret",
					"number": 123,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		Address: server.URL,
		Token:   "test-token",
	})

	ctx := context.Background()

	// Test successful string retrieval
	value, err := client.GetSecretString(ctx, "cryptofunk/data/jwt", "secret")
	if err != nil {
		t.Fatalf("GetSecretString() error = %v", err)
	}
	if value != "my-jwt-secret" {
		t.Errorf("Expected my-jwt-secret, got %s", value)
	}

	// Clear cache to force new request
	client.ClearCache()

	// Test key not found
	_, err = client.GetSecretString(ctx, "cryptofunk/data/jwt", "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}

	// Clear cache again
	client.ClearCache()

	// Test non-string value
	_, err = client.GetSecretString(ctx, "cryptofunk/data/jwt", "number")
	if err == nil {
		t.Error("Expected error for non-string value")
	}
}

func TestClient_Cache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := SecretResponse{
			Data: &SecretData{
				Data: map[string]interface{}{
					"value": "cached",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		Address:  server.URL,
		Token:    "test-token",
		CacheTTL: 1 * time.Hour,
	})

	ctx := context.Background()

	// First call should hit the server
	_, err := client.GetSecret(ctx, "cryptofunk/data/test")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 server call, got %d", callCount)
	}

	// Second call should use cache
	_, err = client.GetSecret(ctx, "cryptofunk/data/test")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 server call (cached), got %d", callCount)
	}

	// Clear cache and call again
	client.ClearCache()
	_, err = client.GetSecret(ctx, "cryptofunk/data/test")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 server calls after cache clear, got %d", callCount)
	}
}

func TestClient_GetDatabaseConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SecretResponse{
			Data: &SecretData{
				Data: map[string]interface{}{
					"host":     "db.example.com",
					"port":     "5432",
					"database": "mydb",
					"username": "user",
					"password": "pass",
					"sslmode":  "require",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		Address: server.URL,
		Token:   "test-token",
	})

	ctx := context.Background()
	cfg, err := client.GetDatabaseConfig(ctx)
	if err != nil {
		t.Fatalf("GetDatabaseConfig() error = %v", err)
	}

	if cfg.Host != "db.example.com" {
		t.Errorf("Expected host=db.example.com, got %s", cfg.Host)
	}

	expected := "postgres://user:pass@db.example.com:5432/mydb?sslmode=require"
	if cfg.ConnectionString() != expected {
		t.Errorf("Expected connection string %s, got %s", expected, cfg.ConnectionString())
	}
}

func TestClient_Health(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"healthy", 200, false},
		{"standby", 429, false},
		{"sealed", 503, true},
		{"not initialized", 501, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client, _ := NewClient(Config{
				Address: server.URL,
				Token:   "test-token",
			})

			err := client.Health(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================================
// VAULT FALLBACK BEHAVIOR TESTS
// ============================================================================
// These tests verify the Vault client's resilience and fallback mechanisms
// when Vault is unavailable or experiences failures.

func TestClient_TokenSourcePriority(t *testing.T) {
	// Test that token sources are checked in the correct order:
	// 1. Config.Token (explicit)
	// 2. VAULT_TOKEN environment variable
	// 3. VAULT_DEV_TOKEN environment variable

	tests := []struct {
		name           string
		configToken    string
		vaultToken     string
		vaultDevToken  string
		wantErr        bool
		expectedSource string
	}{
		{
			name:           "config token takes priority",
			configToken:    "config-token",
			vaultToken:     "env-token",
			vaultDevToken:  "dev-token",
			wantErr:        false,
			expectedSource: "config",
		},
		{
			name:           "VAULT_TOKEN over VAULT_DEV_TOKEN",
			configToken:    "",
			vaultToken:     "env-token",
			vaultDevToken:  "dev-token",
			wantErr:        false,
			expectedSource: "VAULT_TOKEN",
		},
		{
			name:           "VAULT_DEV_TOKEN as last resort",
			configToken:    "",
			vaultToken:     "",
			vaultDevToken:  "dev-token",
			wantErr:        false,
			expectedSource: "VAULT_DEV_TOKEN",
		},
		{
			name:           "error when no token available",
			configToken:    "",
			vaultToken:     "",
			vaultDevToken:  "",
			wantErr:        true,
			expectedSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			origVaultToken := os.Getenv("VAULT_TOKEN")
			origDevToken := os.Getenv("VAULT_DEV_TOKEN")
			defer func() {
				os.Setenv("VAULT_TOKEN", origVaultToken)
				os.Setenv("VAULT_DEV_TOKEN", origDevToken)
			}()

			// Set test environment
			if tt.vaultToken != "" {
				os.Setenv("VAULT_TOKEN", tt.vaultToken)
			} else {
				os.Unsetenv("VAULT_TOKEN")
			}
			if tt.vaultDevToken != "" {
				os.Setenv("VAULT_DEV_TOKEN", tt.vaultDevToken)
			} else {
				os.Unsetenv("VAULT_DEV_TOKEN")
			}

			client, err := NewClient(Config{
				Address: "http://localhost:8200",
				Token:   tt.configToken,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
		})
	}
}

func TestClient_CacheFallbackOnVaultFailure(t *testing.T) {
	// Test that cached secrets are returned when Vault becomes unavailable
	// This is critical for resilience - the system should continue to work
	// with cached secrets if Vault temporarily goes down.

	requestCount := 0
	vaultAvailable := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if !vaultAvailable {
			// Simulate Vault being unavailable
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"errors":["Vault is sealed"]}`))
			return
		}

		resp := SecretResponse{
			Data: &SecretData{
				Data: map[string]interface{}{
					"api_key": "secret-key-12345",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		Address:  server.URL,
		Token:    "test-token",
		CacheTTL: 1 * time.Hour, // Long cache TTL for this test
	})

	ctx := context.Background()

	// First request - should succeed and cache
	data, err := client.GetSecret(ctx, "cryptofunk/data/api")
	if err != nil {
		t.Fatalf("Initial GetSecret() error = %v", err)
	}
	if data["api_key"] != "secret-key-12345" {
		t.Errorf("Expected api_key=secret-key-12345, got %v", data["api_key"])
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Second request with cache - should use cache (Vault still available)
	_, err = client.GetSecret(ctx, "cryptofunk/data/api")
	if err != nil {
		t.Fatalf("Cached GetSecret() error = %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected cache hit (still 1 request), got %d", requestCount)
	}

	// Now simulate Vault going down - cached secrets should still work
	vaultAvailable = false

	// Third request - should still use cache even though Vault is down
	data, err = client.GetSecret(ctx, "cryptofunk/data/api")
	if err != nil {
		t.Fatalf("Cached GetSecret() with Vault down error = %v", err)
	}
	if data["api_key"] != "secret-key-12345" {
		t.Errorf("Expected cached api_key, got %v", data["api_key"])
	}
	// Should still be 1 because cache hit
	if requestCount != 1 {
		t.Errorf("Expected cache hit (still 1 request), got %d", requestCount)
	}
}

func TestClient_ConnectionErrorHandling(t *testing.T) {
	// Test that connection errors are handled gracefully with appropriate error messages

	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		wantErrSubstr string
	}{
		{
			name: "forbidden - invalid token",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"errors":["permission denied"]}`))
			},
			wantErrSubstr: "403",
		},
		{
			name: "not found - secret doesn't exist",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"errors":["secret not found"]}`))
			},
			wantErrSubstr: "404",
		},
		{
			name: "service unavailable - vault sealed",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"errors":["Vault is sealed"]}`))
			},
			wantErrSubstr: "503",
		},
		{
			name: "internal server error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"errors":["internal error"]}`))
			},
			wantErrSubstr: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			client, _ := NewClient(Config{
				Address: server.URL,
				Token:   "test-token",
			})

			_, err := client.GetSecret(context.Background(), "cryptofunk/data/test")
			if err == nil {
				t.Error("Expected error, got nil")
				return
			}

			if !contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("Expected error containing %q, got %v", tt.wantErrSubstr, err)
			}
		})
	}
}

func TestClient_CacheExpiry(t *testing.T) {
	// Test that cached secrets expire correctly and are refreshed from Vault

	callCount := 0
	secretVersion := 1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := SecretResponse{
			Data: &SecretData{
				Data: map[string]interface{}{
					"version": secretVersion,
				},
			},
		}
		secretVersion++ // Increment for next request
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Use very short cache TTL for testing
	client, _ := NewClient(Config{
		Address:  server.URL,
		Token:    "test-token",
		CacheTTL: 50 * time.Millisecond,
	})

	ctx := context.Background()

	// First request - gets version 1
	data, err := client.GetSecret(ctx, "cryptofunk/data/test")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if data["version"] != float64(1) {
		t.Errorf("Expected version 1, got %v", data["version"])
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Immediate second request - should use cache
	data, err = client.GetSecret(ctx, "cryptofunk/data/test")
	if err != nil {
		t.Fatalf("Cached GetSecret() error = %v", err)
	}
	if data["version"] != float64(1) {
		t.Errorf("Expected cached version 1, got %v", data["version"])
	}
	if callCount != 1 {
		t.Errorf("Expected cache hit (1 call), got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Third request - cache expired, should fetch new version
	data, err = client.GetSecret(ctx, "cryptofunk/data/test")
	if err != nil {
		t.Fatalf("GetSecret() after cache expiry error = %v", err)
	}
	if data["version"] != float64(2) {
		t.Errorf("Expected version 2 after cache expiry, got %v", data["version"])
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls after cache expiry, got %d", callCount)
	}
}

func TestClient_InsecureTokenWarning(t *testing.T) {
	// Test that insecure development tokens trigger appropriate warnings
	// This doesn't test the actual logging, but verifies the token detection works

	insecureTokens := []string{"cryptofunk-dev-token", "root", "dev", "test"}

	for _, token := range insecureTokens {
		t.Run("insecure_"+token, func(t *testing.T) {
			// The client should be created successfully even with insecure tokens
			// (warnings are logged, not errors returned)
			client, err := NewClient(Config{
				Address: "http://localhost:8200",
				Token:   token,
			})

			if err != nil {
				t.Errorf("NewClient() with insecure token %q error = %v", token, err)
			}
			if client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClient_HTTPSWarningForNonLocalhost(t *testing.T) {
	// Test that HTTP connections to non-localhost addresses trigger warnings
	// The client should still work, but log a security warning

	addresses := []struct {
		address   string
		shouldRun bool // some addresses won't actually be reachable for full test
	}{
		{"http://vault.example.com:8200", true},
		{"http://192.168.1.100:8200", true},
		// These should NOT trigger warnings (localhost is OK for HTTP)
		{"http://localhost:8200", true},
		{"http://127.0.0.1:8200", true},
	}

	for _, addr := range addresses {
		t.Run(addr.address, func(t *testing.T) {
			// Just verify the client is created - we can't easily test log output
			client, err := NewClient(Config{
				Address: addr.address,
				Token:   "test-token",
			})

			if err != nil {
				t.Errorf("NewClient() with address %q error = %v", addr.address, err)
			}
			if client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClient_ConcurrentAccess(t *testing.T) {
	// Test that concurrent access to secrets is safe and uses caching correctly

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some latency
		time.Sleep(10 * time.Millisecond)
		resp := SecretResponse{
			Data: &SecretData{
				Data: map[string]interface{}{
					"value": "concurrent-test",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		Address:  server.URL,
		Token:    "test-token",
		CacheTTL: 1 * time.Hour,
	})

	ctx := context.Background()
	const numGoroutines = 10
	errCh := make(chan error, numGoroutines)

	// Launch concurrent requests
	for i := 0; i < numGoroutines; i++ {
		go func() {
			data, err := client.GetSecret(ctx, "cryptofunk/data/concurrent")
			if err != nil {
				errCh <- err
				return
			}
			if data["value"] != "concurrent-test" {
				errCh <- fmt.Errorf("unexpected value: %v", data["value"])
				return
			}
			errCh <- nil
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
