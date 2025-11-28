//go:build integration

package main

import (
	"encoding/json"
	"testing"

	"github.com/ajitpratap0/cryptofunk/internal/indicators"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_CalculateRSI tests RSI calculation with real service
func TestIntegration_CalculateRSI(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	// Use standard RSI test data (20 periods)
	prices := []interface{}{
		44.34, 44.09, 43.61, 43.03, 43.52, 43.13, 42.66,
		42.82, 42.67, 43.13, 43.37, 43.23, 43.08, 42.07,
		41.99, 42.18, 42.49, 42.28, 42.51, 43.13,
	}

	t.Run("DefaultPeriod14", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_rsi",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 14,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Error, "Expected no error, got: %v", resp.Error)
		require.NotNil(t, resp.Result)

		// Parse result
		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var rsiResult indicators.RSIResult
		err = json.Unmarshal(resultBytes, &rsiResult)
		require.NoError(t, err)

		// Verify RSI value is in valid range
		assert.GreaterOrEqual(t, rsiResult.Value, 0.0)
		assert.LessOrEqual(t, rsiResult.Value, 100.0)

		// Verify signal is valid
		validSignals := []string{"oversold", "overbought", "neutral"}
		assert.Contains(t, validSignals, rsiResult.Signal)

		t.Logf("RSI: %.2f, Signal: %s", rsiResult.Value, rsiResult.Signal)
	})

	t.Run("CustomPeriod10", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_rsi",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 10,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var rsiResult indicators.RSIResult
		err = json.Unmarshal(resultBytes, &rsiResult)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, rsiResult.Value, 0.0)
		assert.LessOrEqual(t, rsiResult.Value, 100.0)

		t.Logf("RSI (period=10): %.2f, Signal: %s", rsiResult.Value, rsiResult.Signal)
	})

	t.Run("OverboughtScenario", func(t *testing.T) {
		// Strong uptrend should produce high RSI
		overboughtPrices := []interface{}{
			10.0, 12.0, 14.0, 16.0, 18.0, 20.0, 22.0, 24.0,
			26.0, 28.0, 30.0, 32.0, 34.0, 36.0, 38.0, 40.0,
		}

		params := map[string]interface{}{
			"name": "calculate_rsi",
			"arguments": map[string]interface{}{
				"prices": overboughtPrices,
				"period": 14,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      3,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var rsiResult indicators.RSIResult
		err = json.Unmarshal(resultBytes, &rsiResult)
		require.NoError(t, err)

		// Strong uptrend should produce overbought signal
		assert.Equal(t, "overbought", rsiResult.Signal)
		assert.Greater(t, rsiResult.Value, 70.0)

		t.Logf("Overbought RSI: %.2f", rsiResult.Value)
	})

	t.Run("OversoldScenario", func(t *testing.T) {
		// Strong downtrend should produce low RSI
		oversoldPrices := []interface{}{
			40.0, 38.0, 36.0, 34.0, 32.0, 30.0, 28.0, 26.0,
			24.0, 22.0, 20.0, 18.0, 16.0, 14.0, 12.0, 10.0,
		}

		params := map[string]interface{}{
			"name": "calculate_rsi",
			"arguments": map[string]interface{}{
				"prices": oversoldPrices,
				"period": 14,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      4,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var rsiResult indicators.RSIResult
		err = json.Unmarshal(resultBytes, &rsiResult)
		require.NoError(t, err)

		// Strong downtrend should produce oversold signal
		assert.Equal(t, "oversold", rsiResult.Signal)
		assert.Less(t, rsiResult.Value, 30.0)

		t.Logf("Oversold RSI: %.2f", rsiResult.Value)
	})
}

// TestIntegration_CalculateMACD tests MACD calculation with real service
func TestIntegration_CalculateMACD(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	// Generate 50 price points for MACD (needs slow_period + signal_period at minimum)
	prices := make([]interface{}, 50)
	for i := 0; i < 50; i++ {
		prices[i] = 100.0 + float64(i)*0.5
	}

	t.Run("DefaultPeriods", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_macd",
			"arguments": map[string]interface{}{
				"prices":        prices,
				"fast_period":   12,
				"slow_period":   26,
				"signal_period": 9,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      5,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Nil(t, resp.Error, "Expected no error, got: %v", resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var macdResult indicators.MACDResult
		err = json.Unmarshal(resultBytes, &macdResult)
		require.NoError(t, err)

		// Verify MACD components exist
		assert.NotZero(t, macdResult.MACD)
		assert.NotZero(t, macdResult.Signal)
		// Histogram is MACD - Signal
		expectedHistogram := macdResult.MACD - macdResult.Signal
		assert.InDelta(t, expectedHistogram, macdResult.Histogram, 0.001)

		// Verify crossover signal is valid
		validCrossovers := []string{"bullish", "bearish", "none"}
		assert.Contains(t, validCrossovers, macdResult.Crossover)

		t.Logf("MACD: %.4f, Signal: %.4f, Histogram: %.4f, Crossover: %s",
			macdResult.MACD, macdResult.Signal, macdResult.Histogram, macdResult.Crossover)
	})

	t.Run("CustomPeriods", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_macd",
			"arguments": map[string]interface{}{
				"prices":        prices,
				"fast_period":   8,
				"slow_period":   17,
				"signal_period": 5,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      6,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var macdResult indicators.MACDResult
		err = json.Unmarshal(resultBytes, &macdResult)
		require.NoError(t, err)

		assert.NotZero(t, macdResult.MACD)
		assert.NotZero(t, macdResult.Signal)

		t.Logf("MACD (custom periods): %.4f, Signal: %.4f", macdResult.MACD, macdResult.Signal)
	})
}

// TestIntegration_CalculateBollingerBands tests Bollinger Bands with real service
func TestIntegration_CalculateBollingerBands(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	// Generate 30 price points (need at least period=20)
	prices := make([]interface{}, 30)
	for i := 0; i < 30; i++ {
		prices[i] = 100.0 + float64(i%10)
	}

	t.Run("DefaultParameters", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_bollinger_bands",
			"arguments": map[string]interface{}{
				"prices":  prices,
				"period":  20,
				"std_dev": 2,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      7,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Nil(t, resp.Error, "Expected no error, got: %v", resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var bbResult indicators.BollingerBandsResult
		err = json.Unmarshal(resultBytes, &bbResult)
		require.NoError(t, err)

		// Verify band ordering: lower < middle < upper
		assert.Less(t, bbResult.Lower, bbResult.Middle)
		assert.Less(t, bbResult.Middle, bbResult.Upper)

		// Verify width is positive
		assert.Greater(t, bbResult.Width, 0.0)

		// Verify signal is valid
		validSignals := []string{"buy", "sell", "neutral"}
		assert.Contains(t, validSignals, bbResult.Signal)

		t.Logf("BB - Upper: %.2f, Middle: %.2f, Lower: %.2f, Width: %.2f%%, Signal: %s",
			bbResult.Upper, bbResult.Middle, bbResult.Lower, bbResult.Width, bbResult.Signal)
	})

	t.Run("CustomPeriod", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_bollinger_bands",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 15,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      8,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var bbResult indicators.BollingerBandsResult
		err = json.Unmarshal(resultBytes, &bbResult)
		require.NoError(t, err)

		assert.Less(t, bbResult.Lower, bbResult.Middle)
		assert.Less(t, bbResult.Middle, bbResult.Upper)

		t.Logf("BB (period=15): Middle: %.2f, Width: %.2f%%", bbResult.Middle, bbResult.Width)
	})
}

// TestIntegration_CalculateEMA tests EMA calculation with real service
func TestIntegration_CalculateEMA(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	prices := make([]interface{}, 30)
	for i := 0; i < 30; i++ {
		prices[i] = 100.0 + float64(i)
	}

	t.Run("Period10", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_ema",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 10,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      9,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Nil(t, resp.Error, "Expected no error, got: %v", resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var emaResult indicators.EMAResult
		err = json.Unmarshal(resultBytes, &emaResult)
		require.NoError(t, err)

		// EMA should be positive for positive prices
		assert.Greater(t, emaResult.Value, 0.0)

		// Verify trend signal is valid
		validTrends := []string{"bullish", "bearish", "neutral"}
		assert.Contains(t, validTrends, emaResult.Trend)

		// For uptrending prices, current price should be above EMA (bullish)
		currentPrice := prices[len(prices)-1].(float64)
		if currentPrice > emaResult.Value {
			assert.Equal(t, "bullish", emaResult.Trend)
		}

		t.Logf("EMA(10): %.2f, Current Price: %.2f, Trend: %s",
			emaResult.Value, currentPrice, emaResult.Trend)
	})

	t.Run("Period20", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_ema",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 20,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      10,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var emaResult indicators.EMAResult
		err = json.Unmarshal(resultBytes, &emaResult)
		require.NoError(t, err)

		assert.Greater(t, emaResult.Value, 0.0)

		t.Logf("EMA(20): %.2f, Trend: %s", emaResult.Value, emaResult.Trend)
	})

	t.Run("CompareDifferentPeriods", func(t *testing.T) {
		// EMA with shorter period should be closer to current price
		var ema10, ema20 indicators.EMAResult

		// Calculate EMA(10)
		params10 := map[string]interface{}{
			"name": "calculate_ema",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 10,
			},
		}
		paramsBytes10, _ := json.Marshal(params10)
		req10 := MCPRequest{JSONRPC: "2.0", ID: 11, Method: "tools/call", Params: paramsBytes10}
		resp10 := server.handleRequest(&req10)
		resultBytes10, _ := json.Marshal(resp10.Result)
		json.Unmarshal(resultBytes10, &ema10)

		// Calculate EMA(20)
		params20 := map[string]interface{}{
			"name": "calculate_ema",
			"arguments": map[string]interface{}{
				"prices": prices,
				"period": 20,
			},
		}
		paramsBytes20, _ := json.Marshal(params20)
		req20 := MCPRequest{JSONRPC: "2.0", ID: 12, Method: "tools/call", Params: paramsBytes20}
		resp20 := server.handleRequest(&req20)
		resultBytes20, _ := json.Marshal(resp20.Result)
		json.Unmarshal(resultBytes20, &ema20)

		currentPrice := prices[len(prices)-1].(float64)

		// For uptrending data, shorter period EMA should be higher
		if ema10.Value > ema20.Value {
			t.Logf("✓ EMA(10)=%.2f > EMA(20)=%.2f (uptrend confirmed)", ema10.Value, ema20.Value)
		} else {
			t.Logf("Current Price: %.2f, EMA(10): %.2f, EMA(20): %.2f", currentPrice, ema10.Value, ema20.Value)
		}
	})
}

// TestIntegration_CalculateADX tests ADX calculation with real service
func TestIntegration_CalculateADX(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	// Generate 40 OHLC data points (need at least period * 2 = 28 for period=14)
	high := make([]interface{}, 40)
	low := make([]interface{}, 40)
	close := make([]interface{}, 40)
	for i := 0; i < 40; i++ {
		base := 100.0 + float64(i)*0.5
		high[i] = base + 2.0
		low[i] = base - 2.0
		close[i] = base
	}

	t.Run("DefaultPeriod14", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_adx",
			"arguments": map[string]interface{}{
				"high":   high,
				"low":    low,
				"close":  close,
				"period": 14,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      13,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Nil(t, resp.Error, "Expected no error, got: %v", resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var adxResult indicators.ADXResult
		err = json.Unmarshal(resultBytes, &adxResult)
		require.NoError(t, err)

		// ADX should be between 0 and 100
		assert.GreaterOrEqual(t, adxResult.Value, 0.0)
		assert.LessOrEqual(t, adxResult.Value, 100.0)

		// Verify strength signal is valid
		validStrengths := []string{"weak", "strong", "very_strong"}
		assert.Contains(t, validStrengths, adxResult.Strength)

		// Verify strength classification
		if adxResult.Value < 25 {
			assert.Equal(t, "weak", adxResult.Strength)
		} else if adxResult.Value < 50 {
			assert.Equal(t, "strong", adxResult.Strength)
		} else {
			assert.Equal(t, "very_strong", adxResult.Strength)
		}

		t.Logf("ADX: %.2f, Strength: %s", adxResult.Value, adxResult.Strength)
	})

	t.Run("StrongTrendData", func(t *testing.T) {
		// Create data with strong trend
		strongHigh := make([]interface{}, 40)
		strongLow := make([]interface{}, 40)
		strongClose := make([]interface{}, 40)
		for i := 0; i < 40; i++ {
			base := 100.0 + float64(i)*2.0 // Stronger trend
			strongHigh[i] = base + 3.0
			strongLow[i] = base - 1.0
			strongClose[i] = base + 1.0 // Closes near high
		}

		params := map[string]interface{}{
			"name": "calculate_adx",
			"arguments": map[string]interface{}{
				"high":   strongHigh,
				"low":    strongLow,
				"close":  strongClose,
				"period": 14,
			},
		}

		paramsBytes, err := json.Marshal(params)
		require.NoError(t, err)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      14,
			Method:  "tools/call",
			Params:  paramsBytes,
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)

		resultBytes, err := json.Marshal(resp.Result)
		require.NoError(t, err)

		var adxResult indicators.ADXResult
		err = json.Unmarshal(resultBytes, &adxResult)
		require.NoError(t, err)

		// Strong trend should produce higher ADX
		assert.GreaterOrEqual(t, adxResult.Value, 0.0)

		t.Logf("ADX (strong trend): %.2f, Strength: %s", adxResult.Value, adxResult.Strength)
	})
}

// TestIntegration_AllIndicatorsTogether tests all indicators in sequence
func TestIntegration_AllIndicatorsTogether(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	// Generate comprehensive price data
	numPrices := 50
	prices := make([]interface{}, numPrices)
	high := make([]interface{}, numPrices)
	low := make([]interface{}, numPrices)
	close := make([]interface{}, numPrices)

	for i := 0; i < numPrices; i++ {
		base := 100.0 + float64(i)*0.3
		prices[i] = base
		high[i] = base + 1.5
		low[i] = base - 1.5
		close[i] = base
	}

	t.Run("CalculateAllIndicators", func(t *testing.T) {
		// Test all 5 indicators with the same data
		indicators := []struct {
			name string
			args map[string]interface{}
		}{
			{
				name: "calculate_rsi",
				args: map[string]interface{}{
					"prices": prices,
					"period": 14,
				},
			},
			{
				name: "calculate_macd",
				args: map[string]interface{}{
					"prices":        prices,
					"fast_period":   12,
					"slow_period":   26,
					"signal_period": 9,
				},
			},
			{
				name: "calculate_bollinger_bands",
				args: map[string]interface{}{
					"prices":  prices,
					"period":  20,
					"std_dev": 2,
				},
			},
			{
				name: "calculate_ema",
				args: map[string]interface{}{
					"prices": prices,
					"period": 12,
				},
			},
			{
				name: "calculate_adx",
				args: map[string]interface{}{
					"high":   high,
					"low":    low,
					"close":  close,
					"period": 14,
				},
			},
		}

		for i, indicator := range indicators {
			params := map[string]interface{}{
				"name":      indicator.name,
				"arguments": indicator.args,
			}

			paramsBytes, err := json.Marshal(params)
			require.NoError(t, err, "Failed to marshal params for %s", indicator.name)

			req := MCPRequest{
				JSONRPC: "2.0",
				ID:      100 + i,
				Method:  "tools/call",
				Params:  paramsBytes,
			}

			resp := server.handleRequest(&req)

			assert.Equal(t, "2.0", resp.JSONRPC, "Invalid JSONRPC version for %s", indicator.name)
			assert.Equal(t, 100+i, resp.ID, "Invalid response ID for %s", indicator.name)
			assert.Nil(t, resp.Error, "Error in %s: %v", indicator.name, resp.Error)
			assert.NotNil(t, resp.Result, "No result for %s", indicator.name)

			t.Logf("✓ %s calculated successfully", indicator.name)
		}
	})
}

// TestIntegration_ErrorHandling tests error cases with real service
func TestIntegration_ErrorHandling(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	t.Run("InsufficientDataForRSI", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_rsi",
			"arguments": map[string]interface{}{
				"prices": []interface{}{100.0, 101.0, 99.0}, // Only 3 prices
				"period": 14,                                // Needs 14
			},
		}

		paramsBytes, _ := json.Marshal(params)
		req := MCPRequest{JSONRPC: "2.0", ID: 200, Method: "tools/call", Params: paramsBytes}
		resp := server.handleRequest(&req)

		assert.NotNil(t, resp.Error, "Expected error for insufficient data")
		t.Logf("Expected error: %s", resp.Error.Message)
	})

	t.Run("InsufficientDataForMACD", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_macd",
			"arguments": map[string]interface{}{
				"prices": []interface{}{100.0, 101.0, 102.0}, // Too few
			},
		}

		paramsBytes, _ := json.Marshal(params)
		req := MCPRequest{JSONRPC: "2.0", ID: 201, Method: "tools/call", Params: paramsBytes}
		resp := server.handleRequest(&req)

		assert.NotNil(t, resp.Error)
		t.Logf("Expected error: %s", resp.Error.Message)
	})

	t.Run("MismatchedArrayLengthsForADX", func(t *testing.T) {
		params := map[string]interface{}{
			"name": "calculate_adx",
			"arguments": map[string]interface{}{
				"high":  []interface{}{102.0, 103.0, 104.0},
				"low":   []interface{}{98.0, 99.0}, // Different length
				"close": []interface{}{100.0, 101.0, 102.0},
			},
		}

		paramsBytes, _ := json.Marshal(params)
		req := MCPRequest{JSONRPC: "2.0", ID: 202, Method: "tools/call", Params: paramsBytes}
		resp := server.handleRequest(&req)

		assert.NotNil(t, resp.Error)
		assert.Contains(t, resp.Error.Message, "same length")
		t.Logf("Expected error: %s", resp.Error.Message)
	})

	t.Run("MissingRequiredPeriodForEMA", func(t *testing.T) {
		prices := make([]interface{}, 20)
		for i := 0; i < 20; i++ {
			prices[i] = 100.0 + float64(i)
		}

		params := map[string]interface{}{
			"name": "calculate_ema",
			"arguments": map[string]interface{}{
				"prices": prices,
				// Missing required period
			},
		}

		paramsBytes, _ := json.Marshal(params)
		req := MCPRequest{JSONRPC: "2.0", ID: 203, Method: "tools/call", Params: paramsBytes}
		resp := server.handleRequest(&req)

		assert.NotNil(t, resp.Error)
		assert.Contains(t, resp.Error.Message, "period is required")
		t.Logf("Expected error: %s", resp.Error.Message)
	})
}

// TestIntegration_MCPProtocolCompliance tests MCP protocol adherence
func TestIntegration_MCPProtocolCompliance(t *testing.T) {
	service := indicators.NewService()
	server := &MCPServer{service: service}

	t.Run("InitializeReturnsCorrectStructure", func(t *testing.T) {
		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params:  json.RawMessage(`{}`),
		}

		resp := server.handleRequest(&req)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(map[string]interface{})
		require.True(t, ok)

		assert.Equal(t, "2024-11-05", result["protocolVersion"])
		assert.NotNil(t, result["serverInfo"])
		assert.NotNil(t, result["capabilities"])
	})

	t.Run("ListToolsReturnsAll5Tools", func(t *testing.T) {
		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/list",
		}

		resp := server.handleRequest(&req)

		assert.Nil(t, resp.Error)
		result, ok := resp.Result.(map[string]interface{})
		require.True(t, ok)

		tools, ok := result["tools"].([]map[string]interface{})
		require.True(t, ok)
		assert.Len(t, tools, 5, "Should have exactly 5 tools")

		// Verify all tool names are present
		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = tool["name"].(string)
		}

		expectedTools := []string{
			"calculate_rsi",
			"calculate_macd",
			"calculate_bollinger_bands",
			"calculate_ema",
			"calculate_adx",
		}

		for _, expected := range expectedTools {
			assert.Contains(t, toolNames, expected, "Missing tool: %s", expected)
		}
	})

	t.Run("ToolSchemaCompliance", func(t *testing.T) {
		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      3,
			Method:  "tools/list",
		}

		resp := server.handleRequest(&req)
		result := resp.Result.(map[string]interface{})
		tools := result["tools"].([]map[string]interface{})

		for _, tool := range tools {
			// Each tool must have required fields
			assert.NotEmpty(t, tool["name"], "Tool missing name")
			assert.NotEmpty(t, tool["description"], "Tool %s missing description", tool["name"])
			assert.NotNil(t, tool["inputSchema"], "Tool %s missing inputSchema", tool["name"])

			// Verify inputSchema structure
			schema, ok := tool["inputSchema"].(map[string]interface{})
			require.True(t, ok, "Tool %s has invalid inputSchema", tool["name"])
			assert.Equal(t, "object", schema["type"], "Tool %s schema type is not 'object'", tool["name"])
			assert.NotNil(t, schema["properties"], "Tool %s missing properties", tool["name"])
		}
	})
}
