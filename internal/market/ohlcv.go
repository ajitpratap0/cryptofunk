package market

import (
	"math"
	"strconv"
)

// OHLCVData holds high, low, close price arrays extracted from candlestick data.
// When only close prices are available (e.g., CoinGecko), high and low are
// estimated from adjacent close prices to allow ADX calculation.
type OHLCVData struct {
	High  []float64 `json:"high"`
	Low   []float64 `json:"low"`
	Close []float64 `json:"close"`
}

// EstimateOHLCV estimates high and low prices from close-only data.
// It uses the absolute change between adjacent closes as a proxy for
// intra-period volatility, applying it symmetrically around the close.
func EstimateOHLCV(closes []float64) *OHLCVData {
	n := len(closes)
	ohlcv := &OHLCVData{
		High:  make([]float64, n),
		Low:   make([]float64, n),
		Close: make([]float64, n),
	}
	copy(ohlcv.Close, closes)

	for i := 0; i < n; i++ {
		var halfRange float64
		if i > 0 {
			halfRange = math.Abs(closes[i] - closes[i-1])
		}
		if i < n-1 {
			change := math.Abs(closes[i+1] - closes[i]) //#nosec G602 -- i+1 is safe: loop guard i < n-1 ensures i+1 < n
			if change > halfRange {
				halfRange = change
			}
		}
		// Use at least 0.1% of price to avoid degenerate zero-range candles
		minRange := closes[i] * 0.001
		if halfRange < minRange {
			halfRange = minRange
		}
		halfRange /= 2.0

		ohlcv.High[i] = closes[i] + halfRange
		ohlcv.Low[i] = closes[i] - halfRange
	}

	return ohlcv
}

// ParseStringFloat parses a float64 from a value that may be a string or float64.
// Uses strconv.ParseFloat for proper error handling (returns 0 on failure).
func ParseStringFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
