package indicators

import (
	"testing"
)

func TestCalculateMACD(t *testing.T) {
	service := NewService()

	// Generate enough price data for MACD (need slow + signal minimum)
	prices := generatePriceData(50, 100.0, 2.0)

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantError   bool
		checkValues bool
	}{
		{
			name: "Valid MACD with default periods",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Valid MACD with custom periods",
			args: map[string]interface{}{
				"prices":        toInterfaceSlice(prices),
				"fast_period":   8,
				"slow_period":   17,
				"signal_period": 9,
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Missing prices",
			args: map[string]interface{}{
				"fast_period": 12,
			},
			wantError: true,
		},
		{
			name: "Fast period >= slow period",
			args: map[string]interface{}{
				"prices":      toInterfaceSlice(prices),
				"fast_period": 26,
				"slow_period": 12,
			},
			wantError: true,
		},
		{
			name: "Invalid fast period (zero)",
			args: map[string]interface{}{
				"prices":      toInterfaceSlice(prices),
				"fast_period": 0,
			},
			wantError: true,
		},
		{
			name: "Insufficient data",
			args: map[string]interface{}{
				"prices":        toInterfaceSlice(prices[:20]),
				"fast_period":   12,
				"slow_period":   26,
				"signal_period": 9,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateMACD(tt.args)

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

			macdResult, ok := result.(*MACDResult)
			if !ok {
				t.Fatal("Expected *MACDResult type")
			}

			if tt.checkValues {
				// Histogram should be MACD - Signal
				expectedHistogram := macdResult.MACD - macdResult.Signal
				if abs(macdResult.Histogram-expectedHistogram) > 0.001 {
					t.Errorf("Histogram mismatch: expected %.6f, got %.6f",
						expectedHistogram, macdResult.Histogram)
				}

				// Crossover should be valid
				validCrossovers := map[string]bool{"bullish": true, "bearish": true, "none": true}
				if !validCrossovers[macdResult.Crossover] {
					t.Errorf("Invalid crossover: %s", macdResult.Crossover)
				}
			}
		})
	}
}

func TestMACDCrossovers(t *testing.T) {
	service := NewService()

	// Create prices that will generate a bullish crossover
	// (start low, trend up strongly)
	bullishPrices := make([]float64, 50)
	for i := range bullishPrices {
		bullishPrices[i] = 90.0 + float64(i)*0.5
	}

	// Create prices that will generate a bearish crossover
	// (start high, trend down)
	bearishPrices := make([]float64, 50)
	for i := range bearishPrices {
		bearishPrices[i] = 120.0 - float64(i)*0.5
	}

	tests := []struct {
		name               string
		prices             []float64
		possibleCrossovers []string // Multiple possible outcomes
	}{
		{
			name:               "Bullish trend",
			prices:             bullishPrices,
			possibleCrossovers: []string{"bullish", "none"},
		},
		{
			name:               "Bearish trend",
			prices:             bearishPrices,
			possibleCrossovers: []string{"bearish", "none"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"prices": toInterfaceSlice(tt.prices),
			}

			result, err := service.CalculateMACD(args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			macdResult := result.(*MACDResult)

			// Check if crossover is one of the possible values
			isValid := false
			for _, expected := range tt.possibleCrossovers {
				if macdResult.Crossover == expected {
					isValid = true
					break
				}
			}

			if !isValid {
				t.Errorf("Expected crossover to be one of %v, got %s",
					tt.possibleCrossovers, macdResult.Crossover)
			}

			t.Logf("MACD: %.2f, Signal: %.2f, Histogram: %.2f, Crossover: %s",
				macdResult.MACD, macdResult.Signal, macdResult.Histogram, macdResult.Crossover)
		})
	}
}

// Helper function to generate price data with trend
func generatePriceData(count int, start float64, volatility float64) []float64 {
	prices := make([]float64, count)
	prices[0] = start
	for i := 1; i < count; i++ {
		// Simple random walk with slight upward bias
		change := (float64(i%3) - 1.0) * volatility
		prices[i] = prices[i-1] + change
	}
	return prices
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
