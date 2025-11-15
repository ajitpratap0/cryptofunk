package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGetPrice_WithMock tests price fetching with mocked HTTP responses
func TestGetPrice_WithMock(t *testing.T) {
	tests := []struct {
		name           string
		symbol         string
		vsCurrency     string
		mockResponse   map[string]interface{}
		statusCode     int
		wantError      bool
		expectedPrice  float64
		expectedSymbol string
	}{
		{
			name:       "Bitcoin price success",
			symbol:     "bitcoin",
			vsCurrency: "usd",
			mockResponse: map[string]interface{}{
				"bitcoin": map[string]float64{
					"usd": 45000.50,
				},
			},
			statusCode:     http.StatusOK,
			wantError:      false,
			expectedPrice:  45000.50,
			expectedSymbol: "bitcoin",
		},
		{
			name:       "Ethereum price success",
			symbol:     "ethereum",
			vsCurrency: "usd",
			mockResponse: map[string]interface{}{
				"ethereum": map[string]float64{
					"usd": 2500.25,
				},
			},
			statusCode:     http.StatusOK,
			wantError:      false,
			expectedPrice:  2500.25,
			expectedSymbol: "ethereum",
		},
		{
			name:       "API rate limit error",
			symbol:     "bitcoin",
			vsCurrency: "usd",
			mockResponse: map[string]interface{}{
				"status": map[string]interface{}{
					"error_code":    429,
					"error_message": "Rate limit exceeded",
				},
			},
			statusCode: http.StatusTooManyRequests,
			wantError:  true,
		},
		{
			name:       "API server error",
			symbol:     "bitcoin",
			vsCurrency: "usd",
			mockResponse: map[string]interface{}{
				"error": "Internal server error",
			},
			statusCode: http.StatusInternalServerError,
			wantError:  true,
		},
		{
			name:       "Invalid coin ID",
			symbol:     "nonexistent",
			vsCurrency: "usd",
			mockResponse: map[string]interface{}{
				"error": "Could not find coin with the given id",
			},
			statusCode: http.StatusNotFound,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				expectedPath := "/api/v3/simple/price"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				// Verify query parameters
				ids := r.URL.Query().Get("ids")
				if ids != tt.symbol {
					t.Errorf("Expected ids=%s, got %s", tt.symbol, ids)
				}

				vsCurrencies := r.URL.Query().Get("vs_currencies")
				if vsCurrencies != tt.vsCurrency {
					t.Errorf("Expected vs_currencies=%s, got %s", tt.vsCurrency, vsCurrencies)
				}

				// Write mock response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create client with mocked server URL (include /api/v3 to match real client)
			client := &CoinGeckoClient{
				baseURL:    server.URL + "/api/v3",
				timeout:    5 * time.Second,
				httpClient: server.Client(),
			}

			// Execute test
			result, err := client.GetPrice(context.Background(), tt.symbol, tt.vsCurrency)

			// Verify error handling
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.Symbol != tt.expectedSymbol {
				t.Errorf("Expected symbol %s, got %s", tt.expectedSymbol, result.Symbol)
			}

			if result.Price != tt.expectedPrice {
				t.Errorf("Expected price %.2f, got %.2f", tt.expectedPrice, result.Price)
			}

			if result.Currency != tt.vsCurrency {
				t.Errorf("Expected currency %s, got %s", tt.vsCurrency, result.Currency)
			}
		})
	}
}

// TestGetMarketChart_WithMock tests market chart fetching with mocked responses
func TestGetMarketChart_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		days        int
		mockPrices  [][]float64
		mockVolumes [][]float64
		statusCode  int
		wantError   bool
		expectedPts int
	}{
		{
			name:   "1 day chart success",
			symbol: "bitcoin",
			days:   1,
			mockPrices: [][]float64{
				{1640000000000, 45000.0},
				{1640003600000, 45500.0},
				{1640007200000, 46000.0},
			},
			mockVolumes: [][]float64{
				{1640000000000, 1000000000},
				{1640003600000, 1100000000},
				{1640007200000, 1200000000},
			},
			statusCode:  http.StatusOK,
			wantError:   false,
			expectedPts: 3,
		},
		{
			name:        "API error",
			symbol:      "bitcoin",
			days:        1,
			mockPrices:  [][]float64{},
			mockVolumes: [][]float64{},
			statusCode:  http.StatusServiceUnavailable,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				expectedPath := fmt.Sprintf("/api/v3/coins/%s/market_chart", tt.symbol)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				// Write mock response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)

				if tt.statusCode == http.StatusOK {
					response := map[string]interface{}{
						"prices":        tt.mockPrices,
						"market_caps":   tt.mockPrices,
						"total_volumes": tt.mockVolumes,
					}
					_ = json.NewEncoder(w).Encode(response)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Service unavailable"})
				}
			}))
			defer server.Close()

			// Create client with mocked server URL (include /api/v3 to match real client)
			client := &CoinGeckoClient{
				baseURL:    server.URL + "/api/v3",
				timeout:    5 * time.Second,
				httpClient: server.Client(),
			}

			// Execute test
			result, err := client.GetMarketChart(context.Background(), tt.symbol, tt.days)

			// Verify error handling
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if len(result.Prices) != tt.expectedPts {
				t.Errorf("Expected %d price points, got %d", tt.expectedPts, len(result.Prices))
			}

			if len(result.TotalVolumes) != tt.expectedPts {
				t.Errorf("Expected %d volume points, got %d", tt.expectedPts, len(result.TotalVolumes))
			}

			// Verify price values
			for i, pricePoint := range result.Prices {
				expectedPrice := tt.mockPrices[i][1]
				if pricePoint.Value != expectedPrice {
					t.Errorf("Price point %d: expected %.2f, got %.2f", i, expectedPrice, pricePoint.Value)
				}
			}
		})
	}
}

// TestGetCoinInfo_WithMock tests coin info fetching with mocked responses
func TestGetCoinInfo_WithMock(t *testing.T) {
	tests := []struct {
		name         string
		coinID       string
		mockResponse map[string]interface{}
		statusCode   int
		wantError    bool
		expectedID   string
		expectedDesc string
	}{
		{
			name:   "Bitcoin info success",
			coinID: "bitcoin",
			mockResponse: map[string]interface{}{
				"id":   "bitcoin",
				"name": "Bitcoin",
				"description": map[string]string{
					"en": "Bitcoin is a decentralized digital currency",
				},
				"links": map[string]interface{}{
					"homepage":                      []string{"https://bitcoin.org"},
					"blockchain_site":               []string{"https://blockchain.com"},
					"official_forum_url":            []string{"https://bitcointalk.org"},
					"chat_url":                      []string{""},
					"announcement_url":              []string{""},
					"twitter_screen_name":           "bitcoin",
					"facebook_username":             "",
					"bitcointalk_thread_identifier": nil,
					"telegram_channel_identifier":   "",
					"subreddit_url":                 "https://www.reddit.com/r/Bitcoin/",
					"repos_url": map[string][]string{
						"github": {"https://github.com/bitcoin/bitcoin"},
					},
				},
				"market_data": map[string]interface{}{
					"current_price": map[string]float64{
						"usd": 45000.0,
					},
					"market_cap": map[string]float64{
						"usd": 850000000000.0,
					},
				},
			},
			statusCode:   http.StatusOK,
			wantError:    false,
			expectedID:   "bitcoin",
			expectedDesc: "Bitcoin is a decentralized digital currency",
		},
		{
			name:   "Coin not found",
			coinID: "nonexistent",
			mockResponse: map[string]interface{}{
				"error": "Could not find coin with the given id",
			},
			statusCode: http.StatusNotFound,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				expectedPath := fmt.Sprintf("/api/v3/coins/%s", tt.coinID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				// Write mock response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create client with mocked server URL (include /api/v3 to match real client)
			client := &CoinGeckoClient{
				baseURL:    server.URL + "/api/v3",
				timeout:    5 * time.Second,
				httpClient: server.Client(),
			}

			// Execute test
			result, err := client.GetCoinInfo(context.Background(), tt.coinID)

			// Verify error handling
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.ID != tt.expectedID {
				t.Errorf("Expected ID %s, got %s", tt.expectedID, result.ID)
			}

			if result.Description != tt.expectedDesc {
				t.Errorf("Expected description %s, got %s", tt.expectedDesc, result.Description)
			}

			// Verify links
			if len(result.Links) == 0 {
				t.Error("Expected at least one link")
			}

			// The actual implementation only extracts "homepage" link
			if _, ok := result.Links["homepage"]; !ok {
				t.Error("Expected 'homepage' link key not found")
			}
		})
	}
}

// TestHealth_WithMock tests health check with mocked responses
// Health() method calls GetPrice("bitcoin", "usd") as a health check
func TestHealth_WithMock(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		mockResp   map[string]interface{}
		wantError  bool
	}{
		{
			name:       "Health check success",
			statusCode: http.StatusOK,
			mockResp: map[string]interface{}{
				"bitcoin": map[string]float64{
					"usd": 45000.0,
				},
			},
			wantError: false,
		},
		{
			name:       "Health check failure",
			statusCode: http.StatusServiceUnavailable,
			mockResp: map[string]interface{}{
				"error": "Service unavailable",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Health() calls GetPrice("bitcoin", "usd"), so expect /simple/price
				expectedPath := "/api/v3/simple/price"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				// Write mock response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResp)
			}))
			defer server.Close()

			// Create client with mocked server URL (include /api/v3 to match real client)
			client := &CoinGeckoClient{
				baseURL:    server.URL + "/api/v3",
				timeout:    5 * time.Second,
				httpClient: server.Client(),
			}

			// Execute test
			err := client.Health(context.Background())

			// Verify error handling
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestConcurrentRequests_WithMock tests concurrent API requests with mocks
func TestConcurrentRequests_WithMock(t *testing.T) {
	requestCount := 0

	// Create mock HTTP server that counts requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"bitcoin": map[string]float64{
				"usd": 45000.0,
			},
		})
	}))
	defer server.Close()

	// Create client with mocked server URL (include /api/v3 to match real client)
	client := &CoinGeckoClient{
		baseURL:    server.URL + "/api/v3",
		timeout:    5 * time.Second,
		httpClient: server.Client(),
	}

	// Make 10 concurrent requests
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			_, err := client.GetPrice(context.Background(), "bitcoin", "usd")
			if err != nil {
				t.Errorf("Concurrent request failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify all requests were made
	if requestCount != numRequests {
		t.Errorf("Expected %d requests, got %d", numRequests, requestCount)
	}
}

// TestContextCancellation_WithMock tests context cancellation with mocks
func TestContextCancellation_WithMock(t *testing.T) {
	// Create mock HTTP server with artificial delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"bitcoin": map[string]float64{
				"usd": 45000.0,
			},
		})
	}))
	defer server.Close()

	// Create client with mocked server URL (include /api/v3 to match real client)
	client := &CoinGeckoClient{
		baseURL:    server.URL + "/api/v3",
		timeout:    5 * time.Second,
		httpClient: server.Client(),
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Execute test
	_, err := client.GetPrice(ctx, "bitcoin", "usd")

	// Verify context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
}
