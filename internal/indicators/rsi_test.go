package indicators

import (
	"testing"
)

func TestCalculateRSI(t *testing.T) {
	service := NewService()

	// Sample price data (increasing trend)
	prices := []float64{
		44.0, 44.5, 45.0, 45.5, 46.0,
		46.5, 47.0, 47.5, 48.0, 48.5,
		49.0, 49.5, 50.0, 50.5, 51.0,
		51.5, 52.0, 52.5, 53.0, 53.5,
	}

	tests := []struct {
		name          string
		args          map[string]interface{}
		wantError     bool
		checkSignal   bool
		expectedRange [2]float64 // min, max range for RSI
	}{
		{
			name: "Valid RSI calculation with default period",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
			},
			wantError:     false,
			checkSignal:   true,
			expectedRange: [2]float64{0, 100},
		},
		{
			name: "Valid RSI with custom period",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": 10,
			},
			wantError:     false,
			checkSignal:   true,
			expectedRange: [2]float64{0, 100},
		},
		{
			name: "Missing prices",
			args: map[string]interface{}{
				"period": 14,
			},
			wantError: true,
		},
		{
			name: "Invalid period (too large)",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": len(prices) + 1,
			},
			wantError: true,
		},
		{
			name: "Invalid period (zero)",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": 0,
			},
			wantError: true,
		},
		{
			name: "Empty prices array",
			args: map[string]interface{}{
				"prices": []interface{}{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateRSI(tt.args)

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

			rsiResult, ok := result.(*RSIResult)
			if !ok {
				t.Fatal("Expected *RSIResult type")
			}

			// Check RSI value is in valid range
			if rsiResult.Value < tt.expectedRange[0] || rsiResult.Value > tt.expectedRange[1] {
				t.Errorf("RSI value %.2f out of expected range [%.2f, %.2f]",
					rsiResult.Value, tt.expectedRange[0], tt.expectedRange[1])
			}

			if tt.checkSignal {
				validSignals := map[string]bool{"oversold": true, "overbought": true, "neutral": true}
				if !validSignals[rsiResult.Signal] {
					t.Errorf("Invalid signal: %s", rsiResult.Signal)
				}

				// Verify signal logic
				if rsiResult.Value < 30 && rsiResult.Signal != "oversold" {
					t.Errorf("Expected 'oversold' signal for RSI %.2f, got %s", rsiResult.Value, rsiResult.Signal)
				}
				if rsiResult.Value > 70 && rsiResult.Signal != "overbought" {
					t.Errorf("Expected 'overbought' signal for RSI %.2f, got %s", rsiResult.Value, rsiResult.Signal)
				}
				if rsiResult.Value >= 30 && rsiResult.Value <= 70 && rsiResult.Signal != "neutral" {
					t.Errorf("Expected 'neutral' signal for RSI %.2f, got %s", rsiResult.Value, rsiResult.Signal)
				}
			}
		})
	}
}

func TestRSISignals(t *testing.T) {
	service := NewService()

	tests := []struct {
		name           string
		prices         []float64
		expectedSignal string
	}{
		{
			name: "Strongly bullish trend (expect high RSI - overbought)",
			prices: []float64{
				10.0, 12.0, 14.0, 16.0, 18.0, 20.0, 22.0, 24.0,
				26.0, 28.0, 30.0, 32.0, 34.0, 36.0, 38.0, 40.0,
			},
			expectedSignal: "overbought",
		},
		{
			name: "Strongly bearish trend (expect low RSI - oversold)",
			prices: []float64{
				40.0, 38.0, 36.0, 34.0, 32.0, 30.0, 28.0, 26.0,
				24.0, 22.0, 20.0, 18.0, 16.0, 14.0, 12.0, 10.0,
			},
			expectedSignal: "oversold",
		},
		{
			name: "Sideways market (expect neutral RSI)",
			prices: []float64{
				20.0, 21.0, 20.5, 20.0, 21.0, 20.5, 20.0, 21.0,
				20.5, 20.0, 21.0, 20.5, 20.0, 21.0, 20.5, 20.0,
			},
			expectedSignal: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"prices": toInterfaceSlice(tt.prices),
				"period": 14,
			}

			result, err := service.CalculateRSI(args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			rsiResult := result.(*RSIResult)
			if rsiResult.Signal != tt.expectedSignal {
				t.Errorf("Expected signal %s, got %s (RSI: %.2f)",
					tt.expectedSignal, rsiResult.Signal, rsiResult.Value)
			}
		})
	}
}

// Helper function to convert float64 slice to []interface{}
func toInterfaceSlice(floats []float64) []interface{} {
	result := make([]interface{}, len(floats))
	for i, f := range floats {
		result[i] = f
	}
	return result
}
