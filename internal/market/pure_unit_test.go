package market

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoinGeckoIDToBinanceSymbol(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bitcoin", "BTCUSDT"},
		{"ethereum", "ETHUSDT"},
		{"solana", "SOLUSDT"},
		{"cardano", "ADAUSDT"},
		{"ripple", "XRPUSDT"},
		{"dogecoin", "DOGEUSDT"},
		{"polkadot", "DOTUSDT"},
		{"avalanche", "AVAXUSDT"},
		{"chainlink", "LINKUSDT"},
		{"polygon", "MATICUSDT"},
		// unknown → uppercase + USDT
		{"shiba", "SHIBAUSDT"},
		{"unknowncoin", "UNKNOWNCOINUSDT"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, CoinGeckoIDToBinanceSymbol(tc.input))
		})
	}
}

func TestEstimateOHLCV(t *testing.T) {
	t.Run("nil / empty input", func(t *testing.T) {
		result := EstimateOHLCV(nil)
		require.NotNil(t, result)
		assert.Empty(t, result.Close)
	})

	t.Run("single price uses min range", func(t *testing.T) {
		result := EstimateOHLCV([]float64{100.0})
		require.Len(t, result.High, 1)
		assert.Greater(t, result.High[0], 100.0, "high should be above close")
		assert.Less(t, result.Low[0], 100.0, "low should be below close")
		assert.Equal(t, 100.0, result.Close[0])
	})

	t.Run("two prices", func(t *testing.T) {
		closes := []float64{100.0, 110.0}
		result := EstimateOHLCV(closes)
		require.Len(t, result.High, 2)
		for i := range closes {
			assert.GreaterOrEqual(t, result.High[i], closes[i])
			assert.LessOrEqual(t, result.Low[i], closes[i])
			assert.Equal(t, closes[i], result.Close[i])
		}
	})

	t.Run("flat prices use min range of 0.1 pct", func(t *testing.T) {
		closes := []float64{1000.0, 1000.0, 1000.0}
		result := EstimateOHLCV(closes)
		// halfRange = max(0, 0) → falls back to minRange = 1.0 → halfRange/2 = 0.5
		expectedHalfRange := 1000.0 * 0.001 / 2.0
		assert.InDelta(t, 1000.0+expectedHalfRange, result.High[1], 1e-9)
		assert.InDelta(t, 1000.0-expectedHalfRange, result.Low[1], 1e-9)
	})

	t.Run("high is always >= close and low <= close", func(t *testing.T) {
		closes := []float64{50000, 51000, 49000, 52000, 48000}
		result := EstimateOHLCV(closes)
		for i, c := range closes {
			assert.GreaterOrEqual(t, result.High[i], c)
			assert.LessOrEqual(t, result.Low[i], c)
			assert.False(t, math.IsNaN(result.High[i]))
			assert.False(t, math.IsNaN(result.Low[i]))
		}
	})
}

func TestParseStringFloat(t *testing.T) {
	assert.Equal(t, 3.14, ParseStringFloat(3.14))
	assert.Equal(t, 42.0, ParseStringFloat("42"))
	assert.Equal(t, 0.0, ParseStringFloat("notanumber"))
	assert.Equal(t, 0.0, ParseStringFloat(nil))
	assert.Equal(t, 0.0, ParseStringFloat(42)) // int is not handled
}
