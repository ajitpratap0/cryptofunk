package market

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewCoinGeckoClient(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantError bool
	}{
		{
			name:      "Valid URL",
			url:       "https://api.coingecko.com/mcp",
			wantError: false,
		},
		{
			name:      "Empty URL",
			url:       "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCoinGeckoClient(tt.url)

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

			if client.url != tt.url {
				t.Errorf("Expected URL %s, got %s", tt.url, client.url)
			}

			if client.timeout != 30*time.Second {
				t.Errorf("Expected timeout 30s, got %v", client.timeout)
			}
		})
	}
}

func TestGetPrice(t *testing.T) {
	client, err := NewCoinGeckoClient("https://api.coingecko.com/mcp")
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
	client, err := NewCoinGeckoClient("https://api.coingecko.com/mcp")
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
	client, err := NewCoinGeckoClient("https://api.coingecko.com/mcp")
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
	now := time.Now()
	prices := []PricePoint{
		{Timestamp: now, Value: 100.0},
		{Timestamp: now.Add(5 * time.Minute), Value: 105.0},
		{Timestamp: now.Add(10 * time.Minute), Value: 110.0},
		{Timestamp: now.Add(15 * time.Minute), Value: 95.0},
		{Timestamp: now.Add(20 * time.Minute), Value: 102.0},
		{Timestamp: now.Add(25 * time.Minute), Value: 108.0},
	}

	volumes := []PricePoint{
		{Timestamp: now, Value: 1000.0},
		{Timestamp: now.Add(5 * time.Minute), Value: 1100.0},
		{Timestamp: now.Add(10 * time.Minute), Value: 1200.0},
		{Timestamp: now.Add(15 * time.Minute), Value: 900.0},
		{Timestamp: now.Add(20 * time.Minute), Value: 1050.0},
		{Timestamp: now.Add(25 * time.Minute), Value: 1150.0},
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
			expectedCandles: 3,
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
			client, err := NewCoinGeckoClient(tt.url)
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
	client, err := NewCoinGeckoClient("https://api.coingecko.com/mcp")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Unexpected error closing client: %v", err)
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		check func(*testing.T, time.Time)
	}{
		{
			name:  "Float64 milliseconds",
			input: float64(1609459200000), // 2021-01-01 00:00:00 UTC
			check: func(t *testing.T, result time.Time) {
				expected := time.Unix(1609459200, 0)
				if result.Unix() != expected.Unix() {
					t.Errorf("Expected timestamp %v, got %v", expected, result)
				}
			},
		},
		{
			name:  "Int64 milliseconds",
			input: int64(1609459200000),
			check: func(t *testing.T, result time.Time) {
				if result.Unix() != 1609459200 {
					t.Errorf("Expected Unix timestamp 1609459200, got %d", result.Unix())
				}
			},
		},
		{
			name:  "Int milliseconds",
			input: int(1609459200000),
			check: func(t *testing.T, result time.Time) {
				if result.Unix() != 1609459200 {
					t.Errorf("Expected Unix timestamp 1609459200, got %d", result.Unix())
				}
			},
		},
		{
			name:  "Invalid type (string)",
			input: "invalid",
			check: func(t *testing.T, result time.Time) {
				// Should return current time for invalid input
				if result.IsZero() {
					t.Error("Expected non-zero time for invalid input")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimestamp(tt.input)
			tt.check(t, result)
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{
			name:     "Float64",
			input:    float64(123.45),
			expected: 123.45,
		},
		{
			name:     "Int",
			input:    int(123),
			expected: 123.0,
		},
		{
			name:     "Int64",
			input:    int64(456),
			expected: 456.0,
		},
		{
			name:     "String number",
			input:    "789.12",
			expected: 789.12,
		},
		{
			name:     "Invalid string",
			input:    "invalid",
			expected: 0.0,
		},
		{
			name:     "Invalid type (bool)",
			input:    true,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFloat(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"string_key": "test_value",
		"int_key":    123,
		"bool_key":   true,
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "Valid string key",
			key:      "string_key",
			expected: "test_value",
		},
		{
			name:     "Non-string value",
			key:      "int_key",
			expected: "",
		},
		{
			name:     "Missing key",
			key:      "missing_key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(m, tt.key)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
