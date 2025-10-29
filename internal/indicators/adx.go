package indicators

import (
	"fmt"
	"math"

	"github.com/rs/zerolog/log"
)

// ADXResult represents the ADX calculation result
type ADXResult struct {
	Value    float64 `json:"value"`
	Strength string  `json:"strength"` // "weak", "strong", "very_strong"
}

// CalculateADX calculates the Average Directional Index manually
// ADX is not available in cinar/indicator v2, so we implement it ourselves
func (s *Service) CalculateADX(args map[string]interface{}) (interface{}, error) {
	// Extract high, low, close arrays
	high, err := extractPrices(args, "high")
	if err != nil {
		return nil, fmt.Errorf("high prices: %w", err)
	}

	low, err := extractPrices(args, "low")
	if err != nil {
		return nil, fmt.Errorf("low prices: %w", err)
	}

	closePrices, err := extractPrices(args, "close")
	if err != nil {
		return nil, fmt.Errorf("close prices: %w", err)
	}

	// Validate arrays are same length
	if len(high) != len(low) || len(high) != len(closePrices) {
		return nil, fmt.Errorf("high, low, and close arrays must have the same length")
	}

	// Extract period (default: 14)
	period := extractPeriod(args, "period", 14)

	// Validate period and data length
	if period < 1 {
		return nil, fmt.Errorf("invalid period: %d (must be >= 1)", period)
	}

	minRequired := period * 2 // Need enough data for smoothing
	if len(closePrices) < minRequired {
		return nil, fmt.Errorf("insufficient data: need at least %d prices, got %d", minRequired, len(closePrices))
	}

	log.Debug().
		Int("prices_count", len(closePrices)).
		Int("period", period).
		Msg("Calculating ADX")

	// Calculate ADX manually
	adx := calculateADXManual(high, low, closePrices, period)

	if adx == 0 {
		return nil, fmt.Errorf("ADX calculation failed")
	}

	// Determine trend strength
	// ADX < 25: Weak or absent trend
	// ADX 25-50: Strong trend
	// ADX > 50: Very strong trend
	strength := "weak"
	if adx >= 25 && adx < 50 {
		strength = "strong"
	} else if adx >= 50 {
		strength = "very_strong"
	}

	result := &ADXResult{
		Value:    adx,
		Strength: strength,
	}

	log.Info().
		Float64("adx", adx).
		Str("strength", strength).
		Msg("ADX calculated")

	return result, nil
}

// calculateADXManual implements ADX calculation
func calculateADXManual(high, low, close []float64, period int) float64 {
	n := len(close)
	if n < period*2 {
		return 0
	}

	// Calculate True Range, +DM, -DM
	tr := make([]float64, n)
	plusDM := make([]float64, n)
	minusDM := make([]float64, n)

	for i := 1; i < n; i++ {
		// True Range
		tr[i] = math.Max(high[i]-low[i],
			math.Max(math.Abs(high[i]-close[i-1]),
				math.Abs(low[i]-close[i-1])))

		// Directional Movement
		upMove := high[i] - high[i-1]
		downMove := low[i-1] - low[i]

		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
	}

	// Smooth TR, +DM, -DM using Wilder's smoothing
	smoothTR := smoothWilder(tr, period)
	smoothPlusDM := smoothWilder(plusDM, period)
	smoothMinusDM := smoothWilder(minusDM, period)

	// Calculate +DI and -DI
	plusDI := make([]float64, n)
	minusDI := make([]float64, n)
	dx := make([]float64, n)

	for i := period; i < n; i++ {
		if smoothTR[i] != 0 {
			plusDI[i] = 100 * smoothPlusDM[i] / smoothTR[i]
			minusDI[i] = 100 * smoothMinusDM[i] / smoothTR[i]

			diSum := plusDI[i] + minusDI[i]
			if diSum != 0 {
				dx[i] = 100 * math.Abs(plusDI[i]-minusDI[i]) / diSum
			}
		}
	}

	// Calculate ADX as smoothed DX
	adxValues := smoothWilder(dx, period)

	// Return the most recent ADX value
	return adxValues[n-1]
}

// smoothWilder applies Wilder's smoothing method
func smoothWilder(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)

	if n < period {
		return result
	}

	// Calculate first smoothed value as simple average
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	result[period-1] = sum / float64(period)

	// Apply Wilder's smoothing for remaining values
	for i := period; i < n; i++ {
		result[i] = (result[i-1]*float64(period-1) + data[i]) / float64(period)
	}

	return result
}
