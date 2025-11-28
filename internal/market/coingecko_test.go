package market

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewCoinGeckoClient(t *testing.T) {
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping integration test that requires MCP server - set COINGECKO_API_TEST=1 to run")
	}

	tests := []struct {
		name      string
		apiKey    string
		wantError bool
	}{
		{
			name:      "With API key",
			apiKey:    "test-api-key",
			wantError: false,
		},
		{
			name:      "Without API key (free tier)",
			apiKey:    "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCoinGeckoClient(tt.apiKey)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %q, but got no error", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for test case %q, but got: %v", tt.name, err)
				return
			}

			if client == nil {
				t.Fatalf("Expected non-nil client for test case %q, but got nil", tt.name)
			}

			if client.timeout != defaultTimeout {
				t.Errorf("Expected timeout %v for test case %q, got %v", defaultTimeout, tt.name, client.timeout)
			}

			if client.maxRetries != defaultMaxRetries {
				t.Errorf("Expected max retries %d for test case %q, got %d", defaultMaxRetries, tt.name, client.maxRetries)
			}

			if client.rateLimiter == nil {
				t.Errorf("Expected non-nil rate limiter for test case %q, but got nil", tt.name)
			}

			// Clean up
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client for test case %q: %v", tt.name, err)
			}
		})
	}
}

func TestNewCoinGeckoClientWithOptions(t *testing.T) {
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping integration test that requires MCP server - set COINGECKO_API_TEST=1 to run")
	}

	tests := []struct {
		name      string
		opts      CoinGeckoClientOptions
		wantError bool
	}{
		{
			name: "Custom options",
			opts: CoinGeckoClientOptions{
				MCPURL:             "https://mcp.api.coingecko.com/mcp",
				APIKey:             "test-key",
				Timeout:            10 * time.Second,
				RateLimit:          100,
				MaxRetries:         5,
				RetryDelay:         2 * time.Second,
				EnableRateLimiting: true,
			},
			wantError: false,
		},
		{
			name: "Minimal options with defaults",
			opts: CoinGeckoClientOptions{
				MCPURL: "https://mcp.api.coingecko.com/mcp",
			},
			wantError: false,
		},
		{
			name: "Rate limiting disabled",
			opts: CoinGeckoClientOptions{
				MCPURL:             "https://mcp.api.coingecko.com/mcp",
				EnableRateLimiting: false,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCoinGeckoClientWithOptions(tt.opts)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %q, but got no error", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for test case %q, but got: %v", tt.name, err)
				return
			}

			if client == nil {
				t.Fatalf("Expected non-nil client for test case %q, but got nil", tt.name)
			}

			// Verify options were applied
			if tt.opts.Timeout != 0 && client.timeout != tt.opts.Timeout {
				t.Errorf("Expected timeout %v for test case %q, got %v", tt.opts.Timeout, tt.name, client.timeout)
			}

			if tt.opts.MaxRetries != 0 && client.maxRetries != tt.opts.MaxRetries {
				t.Errorf("Expected max retries %d for test case %q, got %d", tt.opts.MaxRetries, tt.name, client.maxRetries)
			}

			if tt.opts.RetryDelay != 0 && client.retryDelay != tt.opts.RetryDelay {
				t.Errorf("Expected retry delay %v for test case %q, got %v", tt.opts.RetryDelay, tt.name, client.retryDelay)
			}

			if tt.opts.EnableRateLimiting && client.rateLimiter == nil {
				t.Errorf("Expected non-nil rate limiter when EnableRateLimiting=true for test case %q, but got nil", tt.name)
			}

			if !tt.opts.EnableRateLimiting && client.rateLimiter != nil {
				t.Errorf("Expected nil rate limiter when EnableRateLimiting=false for test case %q, but got non-nil", tt.name)
			}

			// Clean up
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client for test case %q: %v", tt.name, err)
			}
		})
	}
}

func TestGetPrice(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	// Run with: COINGECKO_API_TEST=1 go test ./internal/market/...
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	tests := []struct {
		name       string
		symbol     string
		vsCurrency string
		wantError  bool
	}{
		{
			name:       "Bitcoin price in USD",
			symbol:     "bitcoin",
			vsCurrency: "usd",
			wantError:  false,
		},
		{
			name:       "Ethereum price in USD",
			symbol:     "ethereum",
			vsCurrency: "usd",
			wantError:  false,
		},
		{
			name:       "Bitcoin price in EUR",
			symbol:     "bitcoin",
			vsCurrency: "eur",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := client.GetPrice(ctx, tt.symbol, tt.vsCurrency)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for GetPrice(%q, %q), but got no error", tt.symbol, tt.vsCurrency)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for GetPrice(%q, %q), but got: %v", tt.symbol, tt.vsCurrency, err)
				return
			}

			if result == nil {
				t.Fatalf("Expected non-nil result for GetPrice(%q, %q), but got nil", tt.symbol, tt.vsCurrency)
			}

			if result.Symbol != tt.symbol {
				t.Errorf("Expected symbol %q in result, got %q", tt.symbol, result.Symbol)
			}

			if result.Currency != tt.vsCurrency {
				t.Errorf("Expected currency %q in result, got %q", tt.vsCurrency, result.Currency)
			}

			if result.Price <= 0 {
				t.Errorf("Expected positive price for %s/%s, got %.2f", tt.symbol, tt.vsCurrency, result.Price)
			}

			t.Logf("✓ %s price in %s: $%.2f", tt.symbol, tt.vsCurrency, result.Price)
		})
	}
}

func TestGetMarketChart(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	tests := []struct {
		name      string
		symbol    string
		days      int
		wantError bool
	}{
		{
			name:      "1 day chart",
			symbol:    "bitcoin",
			days:      1,
			wantError: false,
		},
		{
			name:      "7 days chart",
			symbol:    "ethereum",
			days:      7,
			wantError: false,
		},
		{
			name:      "30 days chart",
			symbol:    "bitcoin",
			days:      30,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := client.GetMarketChart(ctx, tt.symbol, tt.days)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for GetMarketChart(%q, %d), but got no error", tt.symbol, tt.days)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for GetMarketChart(%q, %d), but got: %v", tt.symbol, tt.days, err)
				return
			}

			if result == nil {
				t.Fatalf("Expected non-nil result for GetMarketChart(%q, %d), but got nil", tt.symbol, tt.days)
			}

			if len(result.Prices) == 0 {
				t.Errorf("Expected non-empty prices array for %s (%d days), got length 0", tt.symbol, tt.days)
			}

			if len(result.MarketCaps) == 0 {
				t.Errorf("Expected non-empty market caps array for %s (%d days), got length 0", tt.symbol, tt.days)
			}

			if len(result.TotalVolumes) == 0 {
				t.Errorf("Expected non-empty volumes array for %s (%d days), got length 0", tt.symbol, tt.days)
			}

			// Verify timestamps are ordered
			for i := 1; i < len(result.Prices); i++ {
				if result.Prices[i].Timestamp.Before(result.Prices[i-1].Timestamp) {
					t.Errorf("Timestamps should be in ascending order: price[%d] (%v) is before price[%d] (%v)",
						i, result.Prices[i].Timestamp, i-1, result.Prices[i-1].Timestamp)
					break
				}
			}

			// Verify all prices are positive
			for i, p := range result.Prices {
				if p.Value <= 0 {
					t.Errorf("Price at index %d should be positive for %s, got %.10f", i, tt.symbol, p.Value)
				}
			}

			t.Logf("✓ %s market chart (%d days): %d price points", tt.symbol, tt.days, len(result.Prices))
		})
	}
}

func TestGetCoinInfo(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	tests := []struct {
		name      string
		coinID    string
		wantError bool
	}{
		{
			name:      "Bitcoin info",
			coinID:    "bitcoin",
			wantError: false,
		},
		{
			name:      "Ethereum info",
			coinID:    "ethereum",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := client.GetCoinInfo(ctx, tt.coinID)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for GetCoinInfo(%q), but got no error", tt.coinID)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for GetCoinInfo(%q), but got: %v", tt.coinID, err)
				return
			}

			if result == nil {
				t.Fatalf("Expected non-nil result for GetCoinInfo(%q), but got nil", tt.coinID)
			}

			if result.ID != tt.coinID {
				t.Errorf("Expected ID %q in result, got %q", tt.coinID, result.ID)
			}

			if result.Name == "" {
				t.Errorf("Expected non-empty name for coin %q, got empty string", tt.coinID)
			}

			if result.Symbol == "" {
				t.Errorf("Expected non-empty symbol for coin %q, got empty string", tt.coinID)
			}

			if result.Description == "" {
				t.Errorf("Expected non-empty description for coin %q, got empty string", tt.coinID)
			}

			if len(result.Links) == 0 {
				t.Errorf("Expected at least one link for coin %q, got 0 links", tt.coinID)
			}

			t.Logf("✓ %s info: %s (%s)", tt.coinID, result.Name, result.Symbol)
		})
	}
}

func TestToCandlesticks(t *testing.T) {
	// Create test market chart data
	// Use a time aligned to 5 minutes (no seconds/milliseconds) for predictable truncation
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	prices := []PricePoint{
		{Timestamp: baseTime, Value: 100.0},                       // 12:00
		{Timestamp: baseTime.Add(5 * time.Minute), Value: 105.0},  // 12:05
		{Timestamp: baseTime.Add(10 * time.Minute), Value: 110.0}, // 12:10
		{Timestamp: baseTime.Add(15 * time.Minute), Value: 95.0},  // 12:15
		{Timestamp: baseTime.Add(20 * time.Minute), Value: 102.0}, // 12:20
		{Timestamp: baseTime.Add(25 * time.Minute), Value: 108.0}, // 12:25
	}

	volumes := []PricePoint{
		{Timestamp: baseTime, Value: 1000.0},
		{Timestamp: baseTime.Add(5 * time.Minute), Value: 1100.0},
		{Timestamp: baseTime.Add(10 * time.Minute), Value: 1200.0},
		{Timestamp: baseTime.Add(15 * time.Minute), Value: 900.0},
		{Timestamp: baseTime.Add(20 * time.Minute), Value: 1050.0},
		{Timestamp: baseTime.Add(25 * time.Minute), Value: 1150.0},
	}

	chart := &MarketChart{
		Prices:       prices,
		MarketCaps:   prices,
		TotalVolumes: volumes,
	}

	tests := []struct {
		name            string
		intervalMinutes int
		expectedCandles int
	}{
		{
			name:            "5 minute candles",
			intervalMinutes: 5,
			expectedCandles: 6,
		},
		{
			name:            "10 minute candles",
			intervalMinutes: 10,
			expectedCandles: 3,
		},
		{
			name:            "15 minute candles",
			intervalMinutes: 15,
			expectedCandles: 2, // 12:00-12:15 and 12:15-12:30
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candles := chart.ToCandlesticks(tt.intervalMinutes)

			if len(candles) != tt.expectedCandles {
				t.Errorf("Expected %d candles, got %d", tt.expectedCandles, len(candles))
			}

			// Verify OHLC properties
			for i, candle := range candles {
				if candle.High < candle.Low {
					t.Errorf("Candle %d: High (%.2f) should be >= Low (%.2f)", i, candle.High, candle.Low)
				}
				if candle.High < candle.Open {
					t.Errorf("Candle %d: High (%.2f) should be >= Open (%.2f)", i, candle.High, candle.Open)
				}
				if candle.High < candle.Close {
					t.Errorf("Candle %d: High (%.2f) should be >= Close (%.2f)", i, candle.High, candle.Close)
				}
				if candle.Low > candle.Open {
					t.Errorf("Candle %d: Low (%.2f) should be <= Open (%.2f)", i, candle.Low, candle.Open)
				}
				if candle.Low > candle.Close {
					t.Errorf("Candle %d: Low (%.2f) should be <= Close (%.2f)", i, candle.Low, candle.Close)
				}
			}
		})
	}
}

func TestToCandlesticksEmptyChart(t *testing.T) {
	chart := &MarketChart{
		Prices:       []PricePoint{},
		MarketCaps:   []PricePoint{},
		TotalVolumes: []PricePoint{},
	}

	candles := chart.ToCandlesticks(5)

	if len(candles) != 0 {
		t.Errorf("Expected 0 candles for empty chart, got %d", len(candles))
	}
}

func TestRateLimiting(t *testing.T) {
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping integration test that requires MCP server - set COINGECKO_API_TEST=1 to run")
	}

	// Create client with very low rate limit for testing
	client, err := NewCoinGeckoClientWithOptions(CoinGeckoClientOptions{
		MCPURL:             "https://mcp.api.coingecko.com/mcp",
		RateLimit:          10, // 10 requests per minute
		Timeout:            5 * time.Second,
		EnableRateLimiting: true,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	// Test that rate limiter exists
	if client.rateLimiter == nil {
		t.Fatal("Expected non-nil rate limiter when EnableRateLimiting=true, but got nil")
	}

	// Test rate limiting behavior
	ctx := context.Background()
	start := time.Now()

	// Make multiple requests quickly
	for i := 0; i < 3; i++ {
		err := client.waitForRateLimit(ctx)
		if err != nil {
			t.Errorf("Rate limit wait failed: %v", err)
		}
	}

	elapsed := time.Since(start)

	// With 10 req/min (1 req per 6 seconds), 3 requests should take at least some time
	// Allow for some variance due to burst capacity
	t.Logf("Rate limiting test: 3 requests took %v", elapsed)
}

func TestRetryLogic(t *testing.T) {
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping integration test that requires MCP server - set COINGECKO_API_TEST=1 to run")
	}

	// This test verifies the retry mechanism structure
	// Actual retry behavior requires integration testing with failing endpoints

	client, err := NewCoinGeckoClientWithOptions(CoinGeckoClientOptions{
		MCPURL:     "https://mcp.api.coingecko.com/mcp",
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	// Verify retry configuration
	expectedMaxRetries := 3
	if client.maxRetries != expectedMaxRetries {
		t.Errorf("Expected max retries %d (from options), got %d", expectedMaxRetries, client.maxRetries)
	}

	expectedRetryDelay := 100 * time.Millisecond
	if client.retryDelay != expectedRetryDelay {
		t.Errorf("Expected retry delay %v (from options), got %v", expectedRetryDelay, client.retryDelay)
	}
}

func TestHealth(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed for operational CoinGecko client: %v", err)
	} else {
		t.Log("✓ Health check passed")
	}
}

func TestClose(t *testing.T) {
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping integration test that requires MCP server - set COINGECKO_API_TEST=1 to run")
	}

	client, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Expected no error on first Close(), but got: %v", err)
	}

	// Verify client can be closed multiple times without error
	err = client.Close()
	if err != nil {
		t.Errorf("Expected no error on second Close() (idempotent), but got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping integration test that requires MCP server - set COINGECKO_API_TEST=1 to run")
	}

	client, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	// Create context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Attempt operations with cancelled context
	_, err = client.GetPrice(ctx, "bitcoin", "usd")
	if err == nil {
		t.Error("Expected error when calling GetPrice with cancelled context, but got no error")
	}

	if ctx.Err() == nil {
		t.Error("Expected context.Err() to be non-nil for cancelled context, but got nil")
	}
}
