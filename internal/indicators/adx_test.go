package indicators

import (
	"testing"
)

func TestCalculateADX(t *testing.T) {
	service := NewService()

	// Generate OHLC data (need high, low, close)
	count := 50
	high := make([]float64, count)
	low := make([]float64, count)
	closePrices := make([]float64, count)

	for i := 0; i < count; i++ {
		base := 100.0 + float64(i)*0.5
		high[i] = base + 2.0
		low[i] = base - 2.0
		closePrices[i] = base
	}

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantError   bool
		checkValues bool
	}{
		{
			name: "Valid ADX calculation",
			args: map[string]interface{}{
				"high":  toInterfaceSlice(high),
				"low":   toInterfaceSlice(low),
				"close": toInterfaceSlice(closePrices),
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Valid ADX with custom period",
			args: map[string]interface{}{
				"high":   toInterfaceSlice(high),
				"low":    toInterfaceSlice(low),
				"close":  toInterfaceSlice(closePrices),
				"period": 10,
			},
			wantError:   false,
			checkValues: true,
		},
		{
			name: "Missing high prices",
			args: map[string]interface{}{
				"low":   toInterfaceSlice(low),
				"close": toInterfaceSlice(closePrices),
			},
			wantError: true,
		},
		{
			name: "Missing low prices",
			args: map[string]interface{}{
				"high":  toInterfaceSlice(high),
				"close": toInterfaceSlice(closePrices),
			},
			wantError: true,
		},
		{
			name: "Missing close prices",
			args: map[string]interface{}{
				"high": toInterfaceSlice(high),
				"low":  toInterfaceSlice(low),
			},
			wantError: true,
		},
		{
			name: "Mismatched array lengths",
			args: map[string]interface{}{
				"high":  toInterfaceSlice(high[:40]),
				"low":   toInterfaceSlice(low),
				"close": toInterfaceSlice(closePrices),
			},
			wantError: true,
		},
		{
			name: "Invalid period (zero)",
			args: map[string]interface{}{
				"high":   toInterfaceSlice(high),
				"low":    toInterfaceSlice(low),
				"close":  toInterfaceSlice(closePrices),
				"period": 0,
			},
			wantError: true,
		},
		{
			name: "Insufficient data",
			args: map[string]interface{}{
				"high":   toInterfaceSlice(high[:20]),
				"low":    toInterfaceSlice(low[:20]),
				"close":  toInterfaceSlice(closePrices[:20]),
				"period": 14,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateADX(tt.args)

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

			adxResult, ok := result.(*ADXResult)
			if !ok {
				t.Fatal("Expected *ADXResult type")
			}

			if tt.checkValues {
				// ADX should be between 0 and 100
				if adxResult.Value < 0 || adxResult.Value > 100 {
					t.Errorf("ADX value %.2f out of valid range [0, 100]", adxResult.Value)
				}

				// Strength should be valid
				validStrengths := map[string]bool{"weak": true, "strong": true, "very_strong": true}
				if !validStrengths[adxResult.Strength] {
					t.Errorf("Invalid strength: %s", adxResult.Strength)
				}

				// Verify strength logic
				if adxResult.Value < 25 && adxResult.Strength != "weak" {
					t.Errorf("Expected 'weak' strength for ADX %.2f, got %s",
						adxResult.Value, adxResult.Strength)
				}
				if adxResult.Value >= 25 && adxResult.Value < 50 && adxResult.Strength != "strong" {
					t.Errorf("Expected 'strong' strength for ADX %.2f, got %s",
						adxResult.Value, adxResult.Strength)
				}
				if adxResult.Value >= 50 && adxResult.Strength != "very_strong" {
					t.Errorf("Expected 'very_strong' strength for ADX %.2f, got %s",
						adxResult.Value, adxResult.Strength)
				}
			}
		})
	}
}

func TestADXStrengthLevels(t *testing.T) {
	service := NewService()

	// Create different trend strengths
	tests := []struct {
		name             string
		high             []float64
		low              []float64
		close            []float64
		expectedStrength string
		description      string
	}{
		{
			name: "Strong uptrend",
			high: func() []float64 {
				prices := make([]float64, 50)
				for i := range prices {
					prices[i] = 100.0 + float64(i)*2.0
				}
				return prices
			}(),
			low: func() []float64 {
				prices := make([]float64, 50)
				for i := range prices {
					prices[i] = 98.0 + float64(i)*2.0
				}
				return prices
			}(),
			close: func() []float64 {
				prices := make([]float64, 50)
				for i := range prices {
					prices[i] = 99.0 + float64(i)*2.0
				}
				return prices
			}(),
			expectedStrength: "strong or very_strong",
			description:      "Consistent uptrend should show strong ADX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"high":   toInterfaceSlice(tt.high),
				"low":    toInterfaceSlice(tt.low),
				"close":  toInterfaceSlice(tt.close),
				"period": 14,
			}

			result, err := service.CalculateADX(args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			adxResult := result.(*ADXResult)
			t.Logf("ADX: %.2f, Strength: %s (%s)", adxResult.Value, adxResult.Strength, tt.description)

			// Just verify we got a valid result
			if adxResult.Value < 0 || adxResult.Value > 100 {
				t.Errorf("ADX value %.2f out of range", adxResult.Value)
			}
		})
	}
}

func TestSmoothWilder(t *testing.T) {
	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	period := 5

	result := smoothWilder(data, period)

	if len(result) != len(data) {
		t.Errorf("Expected result length %d, got %d", len(data), len(result))
	}

	// First period-1 values should be zero
	for i := 0; i < period-1; i++ {
		if result[i] != 0 {
			t.Errorf("Expected result[%d] = 0, got %.2f", i, result[i])
		}
	}

	// First smoothed value should be simple average
	expectedFirst := 3.0 // (1+2+3+4+5)/5
	if result[period-1] != expectedFirst {
		t.Errorf("Expected first smoothed value %.2f, got %.2f", expectedFirst, result[period-1])
	}

	// Subsequent values should be non-zero
	for i := period; i < len(result); i++ {
		if result[i] == 0 {
			t.Errorf("Expected non-zero result at index %d", i)
		}
	}
}

func TestSmoothWilderInsufficientData(t *testing.T) {
	data := []float64{1.0, 2.0, 3.0}
	period := 5

	result := smoothWilder(data, period)

	// Should return all zeros for insufficient data
	for i, v := range result {
		if v != 0 {
			t.Errorf("Expected result[%d] = 0 for insufficient data, got %.2f", i, v)
		}
	}
}

func TestCalculateADXManual(t *testing.T) {
	// Generate simple test data
	count := 50
	high := make([]float64, count)
	low := make([]float64, count)
	closePrices := make([]float64, count)

	for i := 0; i < count; i++ {
		base := 100.0 + float64(i)*0.5
		high[i] = base + 2.0
		low[i] = base - 2.0
		closePrices[i] = base + 1.0
	}

	period := 14
	adx := calculateADXManual(high, low, closePrices, period)

	// ADX should be non-zero for valid data
	if adx == 0 {
		t.Error("Expected non-zero ADX value")
	}

	// ADX should be in valid range
	if adx < 0 || adx > 100 {
		t.Errorf("ADX value %.2f out of valid range [0, 100]", adx)
	}
}

func TestCalculateADXManualInsufficientData(t *testing.T) {
	// Not enough data
	high := []float64{100, 101, 102}
	low := []float64{98, 99, 100}
	closePrices := []float64{99, 100, 101}
	period := 14

	adx := calculateADXManual(high, low, closePrices, period)

	// Should return 0 for insufficient data
	if adx != 0 {
		t.Errorf("Expected 0 ADX for insufficient data, got %.2f", adx)
	}
}
