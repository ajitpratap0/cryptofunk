package risk

import (
	"context"
	"testing"
)

func TestIsValidSymbol(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		// Valid symbols
		{
			name:     "simple uppercase symbol",
			symbol:   "BTC",
			expected: true,
		},
		{
			name:     "symbol with pair",
			symbol:   "BTC/USDT",
			expected: true,
		},
		{
			name:     "symbol with different pair",
			symbol:   "ETH/USD",
			expected: true,
		},
		{
			name:     "numeric symbol",
			symbol:   "BTC123",
			expected: true,
		},
		{
			name:     "numeric pair",
			symbol:   "BTC/USDT20",
			expected: true,
		},
		{
			name:     "two letter symbol",
			symbol:   "BT",
			expected: true,
		},
		{
			name:     "ten letter symbol",
			symbol:   "ABCDEFGHIJ",
			expected: true,
		},
		{
			name:     "symbol with ten letter pair",
			symbol:   "ABCDEFGHIJ/KLMNOPQRST",
			expected: true,
		},

		// Invalid symbols - format violations
		{
			name:     "empty string",
			symbol:   "",
			expected: false,
		},
		{
			name:     "lowercase symbol",
			symbol:   "btc",
			expected: false,
		},
		{
			name:     "mixed case symbol",
			symbol:   "Btc",
			expected: false,
		},
		{
			name:     "single character",
			symbol:   "B",
			expected: false,
		},
		{
			name:     "too long base symbol",
			symbol:   "ABCDEFGHIJK",
			expected: false,
		},
		{
			name:     "too long pair symbol",
			symbol:   "BTC/ABCDEFGHIJK",
			expected: false,
		},
		{
			name:     "multiple slashes",
			symbol:   "BTC/USD/EUR",
			expected: false,
		},
		{
			name:     "trailing slash",
			symbol:   "BTC/",
			expected: false,
		},
		{
			name:     "leading slash",
			symbol:   "/USDT",
			expected: false,
		},

		// Invalid symbols - SQL injection attempts
		{
			name:     "SQL injection with semicolon",
			symbol:   "BTC'; DROP TABLE positions; --",
			expected: false,
		},
		{
			name:     "SQL injection with single quote",
			symbol:   "BTC' OR '1'='1",
			expected: false,
		},
		{
			name:     "SQL injection with double quote",
			symbol:   "BTC\" OR \"1\"=\"1",
			expected: false,
		},
		{
			name:     "SQL injection with SELECT keyword",
			symbol:   "SELECT",
			expected: false,
		},
		{
			name:     "SQL injection with DROP keyword",
			symbol:   "DROP",
			expected: false,
		},
		{
			name:     "SQL injection with UNION keyword",
			symbol:   "UNION",
			expected: false,
		},
		{
			name:     "SQL injection with INSERT keyword",
			symbol:   "INSERT",
			expected: false,
		},
		{
			name:     "SQL injection with DELETE keyword",
			symbol:   "DELETE",
			expected: false,
		},
		{
			name:     "SQL injection with UPDATE keyword",
			symbol:   "UPDATE",
			expected: false,
		},
		{
			name:     "SQL injection with backslash",
			symbol:   "BTC\\USDT",
			expected: false,
		},
		{
			name:     "SQL injection with parentheses",
			symbol:   "BTC()",
			expected: false,
		},
		{
			name:     "SQL injection with brackets",
			symbol:   "BTC[]",
			expected: false,
		},
		{
			name:     "SQL injection with braces",
			symbol:   "BTC{}",
			expected: false,
		},
		{
			name:     "SQL injection with asterisk",
			symbol:   "BTC*",
			expected: false,
		},
		{
			name:     "SQL injection with percent",
			symbol:   "BTC%",
			expected: false,
		},
		{
			name:     "SQL injection with dash",
			symbol:   "BTC-USDT",
			expected: false,
		},
		{
			name:     "SQL injection with WHERE keyword in symbol",
			symbol:   "WHEREHOUSE",
			expected: false,
		},
		{
			name:     "SQL injection with OR keyword",
			symbol:   "ORACLE",
			expected: false,
		},
		{
			name:     "SQL injection with AND keyword",
			symbol:   "ANDROID",
			expected: false,
		},
		{
			name:     "SQL injection with NULL keyword",
			symbol:   "NULL",
			expected: false,
		},
		{
			name:     "SQL injection with TRUE keyword",
			symbol:   "TRUE",
			expected: false,
		},
		{
			name:     "SQL injection with FALSE keyword",
			symbol:   "FALSE",
			expected: false,
		},
		{
			name:     "SQL injection with EXEC keyword",
			symbol:   "EXEC",
			expected: false,
		},
		{
			name:     "SQL injection with EXECUTE keyword",
			symbol:   "EXECUTE",
			expected: false,
		},

		// Invalid symbols - special characters
		{
			name:     "space in symbol",
			symbol:   "BTC USDT",
			expected: false,
		},
		{
			name:     "newline in symbol",
			symbol:   "BTC\nUSDT",
			expected: false,
		},
		{
			name:     "tab in symbol",
			symbol:   "BTC\tUSDT",
			expected: false,
		},
		{
			name:     "symbol with underscore",
			symbol:   "BTC_USDT",
			expected: false,
		},
		{
			name:     "symbol with period",
			symbol:   "BTC.USDT",
			expected: false,
		},
		{
			name:     "symbol with comma",
			symbol:   "BTC,USDT",
			expected: false,
		},
		{
			name:     "symbol with plus",
			symbol:   "BTC+USDT",
			expected: false,
		},
		{
			name:     "symbol with equals",
			symbol:   "BTC=USDT",
			expected: false,
		},
		{
			name:     "symbol with less than",
			symbol:   "BTC<USDT",
			expected: false,
		},
		{
			name:     "symbol with greater than",
			symbol:   "BTC>USDT",
			expected: false,
		},
		{
			name:     "symbol with ampersand",
			symbol:   "BTC&USDT",
			expected: false,
		},
		{
			name:     "symbol with pipe",
			symbol:   "BTC|USDT",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidSymbol(tt.symbol)
			if result != tt.expected {
				t.Errorf("isValidSymbol(%q) = %v, expected %v", tt.symbol, result, tt.expected)
			}
		})
	}
}

func TestSymbolValidationInFunctions(t *testing.T) {
	// This test ensures that validation is called in the relevant functions
	// We test with nil pool to avoid database dependency
	calc := NewCalculator(nil)

	invalidSymbols := []string{
		"'; DROP TABLE positions; --",
		"btc",
		"BTC' OR '1'='1",
		"SELECT",
		"BTC/USD/EUR",
		"",
	}

	validSymbol := "BTC/USDT"

	t.Run("LoadHistoricalPrices validation", func(t *testing.T) {
		ctx := context.Background()
		// Test invalid symbols
		for _, symbol := range invalidSymbols {
			_, err := calc.LoadHistoricalPrices(ctx, symbol, "1h", 30)
			if err == nil {
				t.Errorf("LoadHistoricalPrices should reject invalid symbol: %s", symbol)
			}
			if err != nil && err.Error() != "invalid symbol format: "+symbol {
				// If error is not about invalid format, it's coming from nil pool
				// which means validation was not called first
				if err.Error() == "no database pool available" {
					t.Errorf("LoadHistoricalPrices should validate symbol before checking pool")
				}
			}
		}

		// Test valid symbol (should fail on nil pool check, not validation)
		_, err := calc.LoadHistoricalPrices(ctx, validSymbol, "1h", 30)
		if err == nil {
			t.Error("Expected error due to nil pool")
		}
		if err.Error() == "invalid symbol format: "+validSymbol {
			t.Error("Valid symbol should not trigger validation error")
		}
	})

	t.Run("GetCurrentPrice validation", func(t *testing.T) {
		ctx := context.Background()
		// Test invalid symbols
		for _, symbol := range invalidSymbols {
			_, err := calc.GetCurrentPrice(ctx, symbol, "1h")
			if err == nil {
				t.Errorf("GetCurrentPrice should reject invalid symbol: %s", symbol)
			}
			if err != nil && err.Error() != "invalid symbol format: "+symbol {
				if err.Error() == "no database pool available" {
					t.Errorf("GetCurrentPrice should validate symbol before checking pool")
				}
			}
		}

		// Test valid symbol
		_, err := calc.GetCurrentPrice(ctx, validSymbol, "1h")
		if err == nil {
			t.Error("Expected error due to nil pool")
		}
		if err.Error() == "invalid symbol format: "+validSymbol {
			t.Error("Valid symbol should not trigger validation error")
		}
	})

	t.Run("CalculateWinRate validation", func(t *testing.T) {
		ctx := context.Background()
		// Test invalid symbols
		for _, symbol := range invalidSymbols {
			if symbol == "" {
				// Empty symbol is allowed for CalculateWinRate (gets all positions)
				continue
			}
			_, err := calc.CalculateWinRate(ctx, symbol)
			if err == nil {
				t.Errorf("CalculateWinRate should reject invalid symbol: %s", symbol)
			}
			// For this function, nil pool returns default values, not error
		}

		// Test empty symbol (should be allowed)
		result, err := calc.CalculateWinRate(ctx, "")
		if err != nil {
			t.Error("Empty symbol should be allowed for CalculateWinRate")
		}
		if result == nil {
			t.Error("Should return default values for empty symbol with nil pool")
		}
	})

	t.Run("DetectMarketRegime validation", func(t *testing.T) {
		ctx := context.Background()
		// Test invalid symbols
		for _, symbol := range invalidSymbols {
			_, err := calc.DetectMarketRegime(ctx, symbol, 30)
			if err == nil {
				t.Errorf("DetectMarketRegime should reject invalid symbol: %s", symbol)
			}
			if err != nil && err.Error() != "invalid symbol format: "+symbol {
				// Should get validation error before LoadHistoricalPrices is called
				if err.Error() != "invalid symbol format: "+symbol {
					t.Errorf("Expected validation error for symbol %s, got: %v", symbol, err)
				}
			}
		}
	})

	t.Run("CalculateVaRFromPrices validation", func(t *testing.T) {
		ctx := context.Background()
		// Test invalid symbols
		for _, symbol := range invalidSymbols {
			_, _, err := calc.CalculateVaRFromPrices(ctx, symbol, "1h", 30, 0.95)
			if err == nil {
				t.Errorf("CalculateVaRFromPrices should reject invalid symbol: %s", symbol)
			}
			if err != nil && err.Error() != "invalid symbol format: "+symbol {
				// Should get validation error before LoadHistoricalPrices is called
				if err.Error() != "invalid symbol format: "+symbol {
					t.Errorf("Expected validation error for symbol %s, got: %v", symbol, err)
				}
			}
		}
	})
}
