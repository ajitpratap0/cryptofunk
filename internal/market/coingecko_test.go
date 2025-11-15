package market

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestNewCoinGeckoClient(t *testing.T) {
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
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if client.baseURL != coinGeckoAPIBase {
				t.Errorf("Expected base URL %s, got %s", coinGeckoAPIBase, client.baseURL)
			}

			if client.timeout != defaultTimeout {
				t.Errorf("Expected timeout %v, got %v", defaultTimeout, client.timeout)
			}

			if client.apiKey != tt.apiKey {
				t.Errorf("Expected API key %s, got %s", tt.apiKey, client.apiKey)
			}
		})
	}
}

func TestGetPrice(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	// Run with: COINGECKO_API_TEST=1 go test ./internal/market/...
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - use TestGetPrice_WithMock or set COINGECKO_API_TEST=1")
	}

	client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name       string
		symbol     string
		vsCurrency string
		wantError  bool
	}{
		{
			name:       "Bitcoin price",
			symbol:     "bitcoin",
			vsCurrency: "usd",
			wantError:  false,
		},
		{
			name:       "Ethereum price",
			symbol:     "ethereum",
			vsCurrency: "usd",
			wantError:  false,
		},
		{
			name:       "BTC abbreviation",
			symbol:     "btc",
			vsCurrency: "usd",
			wantError:  false,
		},
		{
			name:       "Unknown symbol",
			symbol:     "unknown_coin",
			vsCurrency: "usd",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.GetPrice(context.Background(), tt.symbol, tt.vsCurrency)

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

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.Symbol != tt.symbol {
				t.Errorf("Expected symbol %s, got %s", tt.symbol, result.Symbol)
			}

			if result.Currency != tt.vsCurrency {
				t.Errorf("Expected currency %s, got %s", tt.vsCurrency, result.Currency)
			}

			if result.Price <= 0 {
				t.Errorf("Expected positive price, got %.2f", result.Price)
			}
		})
	}
}

func TestGetMarketChart(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - use TestGetMarketChart_WithMock or set COINGECKO_API_TEST=1")
	}

	client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

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
			result, err := client.GetMarketChart(context.Background(), tt.symbol, tt.days)

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

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			expectedDataPoints := tt.days * 24
			if len(result.Prices) != expectedDataPoints {
				t.Errorf("Expected %d price points, got %d", expectedDataPoints, len(result.Prices))
			}

			// Verify timestamps are ordered
			for i := 1; i < len(result.Prices); i++ {
				if result.Prices[i].Timestamp.Before(result.Prices[i-1].Timestamp) {
					t.Error("Timestamps should be in ascending order")
					break
				}
			}

			// Verify all prices are positive
			for i, p := range result.Prices {
				if p.Value <= 0 {
					t.Errorf("Price at index %d should be positive, got %.2f", i, p.Value)
				}
			}
		})
	}
}

func TestGetCoinInfo(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - use TestGetCoinInfo_WithMock or set COINGECKO_API_TEST=1")
	}

	client, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

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
			result, err := client.GetCoinInfo(context.Background(), tt.coinID)

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

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.ID != tt.coinID {
				t.Errorf("Expected ID %s, got %s", tt.coinID, result.ID)
			}

			if result.Description == "" {
				t.Error("Expected non-empty description")
			}

			if len(result.Links) == 0 {
				t.Error("Expected at least one link")
			}
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

func TestCandlestickMarshalJSON(t *testing.T) {
	now := time.Now()
	candle := &Candlestick{
		Timestamp: now,
		Open:      100.0,
		High:      110.0,
		Low:       95.0,
		Close:     105.0,
		Volume:    1000.0,
	}

	data, err := json.Marshal(candle)
	if err != nil {
		t.Fatalf("Failed to marshal candlestick: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify all fields are present
	expectedFields := []string{"timestamp", "open", "high", "low", "close", "volume"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected field %s in JSON output", field)
		}
	}

	// Verify timestamp is Unix timestamp
	if timestamp, ok := result["timestamp"].(float64); ok {
		if timestamp != float64(now.Unix()) {
			t.Errorf("Expected timestamp %d, got %.0f", now.Unix(), timestamp)
		}
	} else {
		t.Error("Expected timestamp to be a number")
	}
}

func TestHealth(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - use TestHealth_WithMock or set COINGECKO_API_TEST=1")
	}

	tests := []struct {
		name      string
		url       string
		wantError bool
	}{
		{
			name:      "Valid client",
			url:       "https://api.coingecko.com/mcp",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			err = client.Health(context.Background())

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

func TestClose(t *testing.T) {
	client, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Unexpected error closing client: %v", err)
	}
}
