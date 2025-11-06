package indicators

import (
	"testing"
)

func TestCalculateBollingerBands(t *testing.T) {
	service := NewService()

	prices := generatePriceData(30, 100.0, 2.0)

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantError   bool
		checkValues bool
	}{
		{
			name: "Valid Bollinger Bands with default period",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Valid Bollinger Bands with custom period",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": 10,
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Valid Bollinger Bands with custom std_dev",
			args: map[string]interface{}{
				"prices":  toInterfaceSlice(prices),
				"period":  15,
				"std_dev": 2.5,
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Missing prices",
			args: map[string]interface{}{
				"period": 20,
			},
			wantError: true,
		},
		{
			name: "Invalid period (too small)",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": 1,
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
			name: "Invalid std_dev (negative)",
			args: map[string]interface{}{
				"prices":  toInterfaceSlice(prices),
				"std_dev": -1.0,
			},
			wantError: true,
		},
		{
			name: "Invalid std_dev (zero)",
			args: map[string]interface{}{
				"prices":  toInterfaceSlice(prices),
				"std_dev": 0.0,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateBollingerBands(tt.args)

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

			bbResult, ok := result.(*BollingerBandsResult)
			if !ok {
				t.Fatal("Expected *BollingerBandsResult type")
			}

			if tt.checkValues {
				// Upper should be greater than middle
				if bbResult.Upper <= bbResult.Middle {
					t.Errorf("Upper band (%.2f) should be > middle band (%.2f)",
						bbResult.Upper, bbResult.Middle)
				}

				// Middle should be greater than lower
				if bbResult.Middle <= bbResult.Lower {
					t.Errorf("Middle band (%.2f) should be > lower band (%.2f)",
						bbResult.Middle, bbResult.Lower)
				}

				// Width should be positive
				if bbResult.Width <= 0 {
					t.Errorf("Band width should be positive, got %.2f", bbResult.Width)
				}

				// Signal should be valid
				validSignals := map[string]bool{"buy": true, "sell": true, "neutral": true}
				if !validSignals[bbResult.Signal] {
					t.Errorf("Invalid signal: %s", bbResult.Signal)
				}
			}
		})
	}
}

func TestBollingerBandsSignals(t *testing.T) {
	service := NewService()

	// Create price data that ends at lower band (buy signal)
	buyPrices := make([]float64, 30)
	for i := range buyPrices {
		if i < 20 {
			buyPrices[i] = 100.0 + float64(i%5)
		} else {
			// Drop significantly in last 10 periods
			buyPrices[i] = 90.0 - float64(i-20)*2.0
		}
	}

	// Create price data that ends at upper band (sell signal)
	sellPrices := make([]float64, 30)
	for i := range sellPrices {
		if i < 20 {
			sellPrices[i] = 100.0 + float64(i%5)
		} else {
			// Rise significantly in last 10 periods
			sellPrices[i] = 110.0 + float64(i-20)*2.0
		}
	}

	// Create price data that stays in middle (neutral signal)
	neutralPrices := make([]float64, 30)
	for i := range neutralPrices {
		neutralPrices[i] = 100.0 + float64(i%3)
	}

	tests := []struct {
		name            string
		prices          []float64
		possibleSignals []string // Multiple possible outcomes
	}{
		{
			name:            "Price at lower band",
			prices:          buyPrices,
			possibleSignals: []string{"buy", "neutral"},
		},
		{
			name:            "Price at upper band",
			prices:          sellPrices,
			possibleSignals: []string{"sell", "neutral"},
		},
		{
			name:            "Price in middle range",
			prices:          neutralPrices,
			possibleSignals: []string{"neutral"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"prices": toInterfaceSlice(tt.prices),
				"period": 20,
			}

			result, err := service.CalculateBollingerBands(args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			bbResult := result.(*BollingerBandsResult)

			// Check if signal is one of the possible values
			isValid := false
			for _, expected := range tt.possibleSignals {
				if bbResult.Signal == expected {
					isValid = true
					break
				}
			}

			if !isValid {
				t.Errorf("Expected signal to be one of %v, got %s",
					tt.possibleSignals, bbResult.Signal)
			}

			t.Logf("Upper: %.2f, Middle: %.2f, Lower: %.2f, Width: %.2f%%, Signal: %s, Price: %.2f",
				bbResult.Upper, bbResult.Middle, bbResult.Lower, bbResult.Width,
				bbResult.Signal, tt.prices[len(tt.prices)-1])
		})
	}
}

func TestBollingerBandsDifferentPeriods(t *testing.T) {
	service := NewService()

	prices := generatePriceData(50, 100.0, 2.0)
	periods := []int{10, 20, 30}

	for _, period := range periods {
		t.Run("Period_"+string(rune(period+'0')), func(t *testing.T) {
			args := map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": period,
			}

			result, err := service.CalculateBollingerBands(args)
			if err != nil {
				t.Fatalf("Unexpected error for period %d: %v", period, err)
			}

			bbResult := result.(*BollingerBandsResult)

			// Verify band structure
			if bbResult.Upper <= bbResult.Middle {
				t.Errorf("Invalid band structure: upper (%.2f) <= middle (%.2f)",
					bbResult.Upper, bbResult.Middle)
			}
			if bbResult.Middle <= bbResult.Lower {
				t.Errorf("Invalid band structure: middle (%.2f) <= lower (%.2f)",
					bbResult.Middle, bbResult.Lower)
			}

			t.Logf("Period %d: Width = %.2f%%", period, bbResult.Width)
		})
	}
}
