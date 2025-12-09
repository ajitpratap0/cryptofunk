package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
