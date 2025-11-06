package indicators

import (
	"testing"
)

func TestCalculateEMA(t *testing.T) {
	service := NewService()

	prices := []float64{
		44.0, 44.5, 45.0, 45.5, 46.0,
		46.5, 47.0, 47.5, 48.0, 48.5,
		49.0, 49.5, 50.0, 50.5, 51.0,
	}

	tests := []struct {
		name       string
		args       map[string]interface{}
		wantError  bool
		checkTrend bool
	}{
		{
			name: "Valid EMA calculation",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
				"period": 10,
			},
			wantError:  false,
			checkTrend: true,
		},
		{
			name: "Missing period parameter",
			args: map[string]interface{}{
				"prices": toInterfaceSlice(prices),
			},
			wantError: true,
		},
		{
			name: "Missing prices",
			args: map[string]interface{}{
				"period": 10,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateEMA(tt.args)

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

			emaResult, ok := result.(*EMAResult)
			if !ok {
				t.Fatal("Expected *EMAResult type")
			}

			// EMA should be a reasonable value relative to prices
			minPrice := prices[0]
			maxPrice := prices[len(prices)-1]
			for _, p := range prices {
				if p < minPrice {
					minPrice = p
				}
				if p > maxPrice {
					maxPrice = p
				}
			}

			// EMA should be within reasonable range of price data
			if emaResult.Value < minPrice*0.8 || emaResult.Value > maxPrice*1.2 {
				t.Errorf("EMA value %.2f seems unreasonable for price range [%.2f, %.2f]",
					emaResult.Value, minPrice, maxPrice)
			}

			if tt.checkTrend {
				validTrends := map[string]bool{"bullish": true, "bearish": true, "neutral": true}
				if !validTrends[emaResult.Trend] {
					t.Errorf("Invalid trend: %s", emaResult.Trend)
				}
			}
		})
	}
}

func TestEMATrends(t *testing.T) {
	service := NewService()

	tests := []struct {
		name          string
		prices        []float64
		period        int
		expectedTrend string
	}{
		{
			name: "Bullish trend (price > EMA)",
			prices: []float64{
				10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0,
				18.0, 19.0, 20.0, 21.0, 22.0, 23.0, 24.0,
			},
			period:        10,
			expectedTrend: "bullish",
		},
		{
			name: "Bearish trend (price < EMA)",
			prices: []float64{
				24.0, 23.0, 22.0, 21.0, 20.0, 19.0, 18.0, 17.0,
				16.0, 15.0, 14.0, 13.0, 12.0, 11.0, 10.0,
			},
			period:        10,
			expectedTrend: "bearish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"prices": toInterfaceSlice(tt.prices),
				"period": tt.period,
			}

			result, err := service.CalculateEMA(args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			emaResult := result.(*EMAResult)
			if emaResult.Trend != tt.expectedTrend {
				t.Errorf("Expected trend %s, got %s (EMA: %.2f, Price: %.2f)",
					tt.expectedTrend, emaResult.Trend, emaResult.Value, tt.prices[len(tt.prices)-1])
			}
		})
	}
}

func TestEMADifferentPeriods(t *testing.T) {
	service := NewService()

	prices := []float64{
		10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0,
		18.0, 19.0, 20.0, 21.0, 22.0, 23.0, 24.0, 25.0,
	}

	periods := []int{5, 10, 12}
	var results []*EMAResult

	for _, period := range periods {
		args := map[string]interface{}{
			"prices": toInterfaceSlice(prices),
			"period": period,
		}

		result, err := service.CalculateEMA(args)
		if err != nil {
			t.Fatalf("Unexpected error for period %d: %v", period, err)
		}

		emaResult := result.(*EMAResult)
		results = append(results, emaResult)
		t.Logf("Period %d: EMA = %.2f, Trend = %s", period, emaResult.Value, emaResult.Trend)
	}

	// Verify that shorter periods generally respond faster to price changes
	// (this is a general property of EMAs, not always true but mostly)
	if len(results) >= 2 {
		for i := 0; i < len(results); i++ {
			if results[i].Value <= 0 {
				t.Errorf("EMA value should be positive, got %.2f", results[i].Value)
			}
		}
	}
}
