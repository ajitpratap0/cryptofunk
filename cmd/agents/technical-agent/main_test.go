package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// BeliefBase Tests
// ============================================================================

func TestBeliefBase_UpdateBelief(t *testing.T) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	tests := []struct {
		name       string
		key        string
		value      interface{}
		confidence float64
		source     string
	}{
		{
			name:       "update float belief",
			key:        "BTC_price",
			value:      50000.0,
			confidence: 0.95,
			source:     "market-data",
		},
		{
			name:       "update string belief",
			key:        "BTC_trend",
			value:      "bullish",
			confidence: 0.80,
			source:     "technical-analysis",
		},
		{
			name:       "update struct belief",
			key:        "BTC_rsi",
			value:      &RSIResult{Value: 65.5, Signal: "neutral"},
			confidence: 0.90,
			source:     "rsi-indicator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb.UpdateBelief(tt.key, tt.value, tt.confidence, tt.source)

			belief, exists := bb.GetBelief(tt.key)
			require.True(t, exists, "Belief should exist after update")
			assert.Equal(t, tt.value, belief.Value)
			assert.Equal(t, tt.confidence, belief.Confidence)
			assert.Equal(t, tt.source, belief.Source)
			assert.WithinDuration(t, time.Now(), belief.Timestamp, 1*time.Second)
		})
	}
}

func TestBeliefBase_GetBelief(t *testing.T) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	// Setup initial belief
	bb.UpdateBelief("test_key", 42.0, 0.85, "test-source")

	tests := []struct {
		name        string
		key         string
		shouldExist bool
	}{
		{
			name:        "get existing belief",
			key:         "test_key",
			shouldExist: true,
		},
		{
			name:        "get non-existent belief",
			key:         "non_existent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			belief, exists := bb.GetBelief(tt.key)
			assert.Equal(t, tt.shouldExist, exists)
			if tt.shouldExist {
				assert.NotNil(t, belief)
				assert.Equal(t, 42.0, belief.Value)
			} else {
				assert.Nil(t, belief)
			}
		})
	}
}

func TestBeliefBase_GetAllBeliefs(t *testing.T) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	// Test empty belief base
	beliefs := bb.GetAllBeliefs()
	assert.Empty(t, beliefs)

	// Add multiple beliefs
	bb.UpdateBelief("key1", "value1", 0.9, "source1")
	bb.UpdateBelief("key2", "value2", 0.8, "source2")
	bb.UpdateBelief("key3", "value3", 0.7, "source3")

	beliefs = bb.GetAllBeliefs()
	assert.Len(t, beliefs, 3)
	assert.Contains(t, beliefs, "key1")
	assert.Contains(t, beliefs, "key2")
	assert.Contains(t, beliefs, "key3")
}

func TestBeliefBase_GetConfidence(t *testing.T) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	// Empty belief base should return 0.0
	confidence := bb.GetConfidence()
	assert.Equal(t, 0.0, confidence)

	// Add beliefs with different confidences
	bb.UpdateBelief("key1", 100.0, 0.95, "test-source")
	bb.UpdateBelief("key2", 200.0, 0.80, "test-source")
	bb.UpdateBelief("key3", 300.0, 0.90, "test-source")

	// GetConfidence returns average confidence across all beliefs
	confidence = bb.GetConfidence()
	assert.Greater(t, confidence, 0.0)
	assert.LessOrEqual(t, confidence, 1.0)
	// Average should be (0.95 + 0.80 + 0.90) / 3 = 0.883...
	assert.InDelta(t, 0.883, confidence, 0.01)
}

func TestBeliefBase_ConcurrentAccess(t *testing.T) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	// Test concurrent writes and reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := "concurrent_key"
			bb.UpdateBelief(key, float64(idx), 0.5, "concurrent-test")
			_, _ = bb.GetBelief(key)
			_ = bb.GetAllBeliefs()
			_ = bb.GetConfidence()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have final belief
	belief, exists := bb.GetBelief("concurrent_key")
	assert.True(t, exists)
	assert.NotNil(t, belief)
}

// ============================================================================
// Indicator Analysis Tests (Pure Functions)
// ============================================================================

func TestAnalyzeRSI(t *testing.T) {
	config := map[string]interface{}{
		"oversold":   30.0,
		"overbought": 70.0,
	}

	tests := []struct {
		name              string
		rsi               *RSIResult
		expectedSignal    string
		reasoningContains string
	}{
		{
			name:              "oversold RSI",
			rsi:               &RSIResult{Value: 25.0, Signal: "oversold"},
			expectedSignal:    "BUY",
			reasoningContains: "oversold",
		},
		{
			name:              "overbought RSI",
			rsi:               &RSIResult{Value: 75.0, Signal: "overbought"},
			expectedSignal:    "SELL",
			reasoningContains: "overbought",
		},
		{
			name:              "neutral RSI - middle range",
			rsi:               &RSIResult{Value: 50.0, Signal: "neutral"},
			expectedSignal:    "HOLD",
			reasoningContains: "neutral",
		},
		{
			name:              "neutral RSI - closer to oversold",
			rsi:               &RSIResult{Value: 35.0, Signal: "neutral"},
			expectedSignal:    "HOLD",
			reasoningContains: "neutral",
		},
		{
			name:              "neutral RSI - closer to overbought",
			rsi:               &RSIResult{Value: 65.0, Signal: "neutral"},
			expectedSignal:    "HOLD",
			reasoningContains: "neutral",
		},
		{
			name:              "extreme oversold",
			rsi:               &RSIResult{Value: 5.0, Signal: "oversold"},
			expectedSignal:    "BUY",
			reasoningContains: "oversold",
		},
		{
			name:              "extreme overbought",
			rsi:               &RSIResult{Value: 95.0, Signal: "overbought"},
			expectedSignal:    "SELL",
			reasoningContains: "overbought",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence, reasoning := analyzeRSI(tt.rsi, config)

			assert.Equal(t, tt.expectedSignal, signal)
			assert.GreaterOrEqual(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)
			assert.Contains(t, reasoning, tt.reasoningContains)
		})
	}
}

func TestAnalyzeMACD(t *testing.T) {
	tests := []struct {
		name              string
		macd              *MACDResult
		expectedSignal    string
		reasoningContains string
	}{
		{
			name: "bullish crossover",
			macd: &MACDResult{
				MACD:      0.5,
				Signal:    0.3,
				Histogram: 0.2,
				Crossover: "bullish",
			},
			expectedSignal:    "BUY",
			reasoningContains: "crossover",
		},
		{
			name: "bearish crossover",
			macd: &MACDResult{
				MACD:      -0.5,
				Signal:    -0.3,
				Histogram: -0.2,
				Crossover: "bearish",
			},
			expectedSignal:    "SELL",
			reasoningContains: "crossover",
		},
		{
			name: "bullish histogram no crossover",
			macd: &MACDResult{
				MACD:      0.8,
				Signal:    0.5,
				Histogram: 0.3,
				Crossover: "none",
			},
			expectedSignal:    "BUY",
			reasoningContains: "MACD",
		},
		{
			name: "bearish histogram no crossover",
			macd: &MACDResult{
				MACD:      -0.8,
				Signal:    -0.5,
				Histogram: -0.3,
				Crossover: "none",
			},
			expectedSignal:    "SELL",
			reasoningContains: "MACD",
		},
		{
			name: "neutral MACD",
			macd: &MACDResult{
				MACD:      0.1,
				Signal:    0.1,
				Histogram: 0.0,
				Crossover: "none",
			},
			expectedSignal:    "HOLD",
			reasoningContains: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence, reasoning := analyzeMACD(tt.macd)

			assert.Equal(t, tt.expectedSignal, signal)
			assert.GreaterOrEqual(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)
			assert.Contains(t, reasoning, tt.reasoningContains)
		})
	}
}

func TestAnalyzeBollingerBands(t *testing.T) {
	tests := []struct {
		name              string
		bb                *BollingerBandsResult
		expectedSignal    string
		reasoningContains string
	}{
		{
			name: "price at lower band - buy signal",
			bb: &BollingerBandsResult{
				Upper:  52000.0,
				Middle: 50000.0,
				Lower:  48000.0,
				Width:  0.08,
				Signal: "buy",
			},
			expectedSignal:    "BUY",
			reasoningContains: "Band",
		},
		{
			name: "price at upper band - sell signal",
			bb: &BollingerBandsResult{
				Upper:  52000.0,
				Middle: 50000.0,
				Lower:  48000.0,
				Width:  0.08,
				Signal: "sell",
			},
			expectedSignal:    "SELL",
			reasoningContains: "Band",
		},
		{
			name: "price near middle band - neutral",
			bb: &BollingerBandsResult{
				Upper:  52000.0,
				Middle: 50000.0,
				Lower:  48000.0,
				Width:  0.08,
				Signal: "neutral",
			},
			expectedSignal:    "HOLD",
			reasoningContains: "middle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence, reasoning := analyzeBollingerBands(tt.bb)

			assert.Equal(t, tt.expectedSignal, signal)
			assert.GreaterOrEqual(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)
			assert.Contains(t, reasoning, tt.reasoningContains)
		})
	}
}

func TestAnalyzeEMATrend(t *testing.T) {
	tests := []struct {
		name           string
		emas           map[int]float64
		expectedSignal string
	}{
		{
			name: "strong bullish trend - all EMAs aligned",
			emas: map[int]float64{
				9:   51000.0,
				21:  50500.0,
				50:  50000.0,
				200: 49000.0,
			},
			expectedSignal: "BUY",
		},
		{
			name: "strong bearish trend - all EMAs aligned",
			emas: map[int]float64{
				9:   49000.0,
				21:  49500.0,
				50:  50000.0,
				200: 51000.0,
			},
			expectedSignal: "SELL",
		},
		{
			name: "insufficient data - neutral",
			emas: map[int]float64{
				21: 50000.0,
			},
			expectedSignal: "HOLD",
		},
		{
			name: "partial bullish - only short-term alignment",
			emas: map[int]float64{
				9:  50500.0,
				21: 50000.0,
			},
			expectedSignal: "BUY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence, reasoning := analyzeEMATrend(tt.emas)

			assert.Equal(t, tt.expectedSignal, signal)
			assert.GreaterOrEqual(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)
			assert.NotEmpty(t, reasoning)
		})
	}
}

// ============================================================================
// Signal Combination Tests
// ============================================================================

func TestCombineSignals(t *testing.T) {
	tests := []struct {
		name           string
		signals        []string
		confidences    []float64
		weights        []float64
		expectedSignal string
	}{
		{
			name:           "all BUY signals",
			signals:        []string{"BUY", "BUY", "BUY"},
			confidences:    []float64{0.9, 0.8, 0.85},
			weights:        []float64{1.0, 1.0, 1.0},
			expectedSignal: "BUY",
		},
		{
			name:           "all SELL signals",
			signals:        []string{"SELL", "SELL", "SELL"},
			confidences:    []float64{0.9, 0.8, 0.85},
			weights:        []float64{1.0, 1.0, 1.0},
			expectedSignal: "SELL",
		},
		{
			name:           "majority BUY with weighted confidence",
			signals:        []string{"BUY", "BUY", "SELL"},
			confidences:    []float64{0.9, 0.8, 0.6},
			weights:        []float64{1.5, 1.0, 1.0},
			expectedSignal: "BUY",
		},
		{
			name:           "majority SELL with weighted confidence",
			signals:        []string{"SELL", "SELL", "BUY"},
			confidences:    []float64{0.9, 0.8, 0.5},
			weights:        []float64{1.5, 1.0, 1.0},
			expectedSignal: "SELL",
		},
		{
			name:           "all HOLD signals",
			signals:        []string{"HOLD", "HOLD", "HOLD"},
			confidences:    []float64{0.6, 0.5, 0.6},
			weights:        []float64{1.0, 1.0, 1.0},
			expectedSignal: "HOLD",
		},
		{
			name:           "empty signals",
			signals:        []string{},
			confidences:    []float64{},
			weights:        []float64{},
			expectedSignal: "HOLD",
		},
		{
			name:           "single signal",
			signals:        []string{"BUY"},
			confidences:    []float64{0.9},
			weights:        []float64{1.0},
			expectedSignal: "BUY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence := combineSignals(tt.signals, tt.confidences, tt.weights)

			assert.Equal(t, tt.expectedSignal, signal)
			assert.GreaterOrEqual(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)
		})
	}
}

// ============================================================================
// IndicatorValues Tests
// ============================================================================

func TestIndicatorValues_ToJSON(t *testing.T) {
	iv := &IndicatorValues{
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		RSI: &RSIResult{
			Value:  65.5,
			Signal: "neutral",
		},
		MACD: &MACDResult{
			MACD:      0.5,
			Signal:    0.3,
			Histogram: 0.2,
			Crossover: "bullish",
		},
		BollingerBands: &BollingerBandsResult{
			Upper:  52000.0,
			Middle: 50000.0,
			Lower:  48000.0,
			Width:  0.08,
			Signal: "neutral",
		},
		EMA: map[int]float64{
			9:   50500.0,
			21:  50000.0,
			50:  49500.0,
			200: 49000.0,
		},
		ADX: &ADXResult{
			Value:  25.5,
			Signal: "trending",
		},
	}

	jsonData, err := json.Marshal(iv)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Verify it can be unmarshaled back
	var decoded IndicatorValues
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)
	assert.Equal(t, iv.RSI.Value, decoded.RSI.Value)
	assert.Equal(t, iv.MACD.MACD, decoded.MACD.MACD)
	assert.Equal(t, iv.BollingerBands.Upper, decoded.BollingerBands.Upper)
}

// ============================================================================
// TechnicalSignal Tests
// ============================================================================

func TestTechnicalSignal_ToJSON(t *testing.T) {
	signal := &TechnicalSignal{
		Timestamp:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
		Price:      50000.0,
		Reasoning:  "Strong bullish signals from RSI and MACD",
		Indicators: &IndicatorValues{
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			RSI: &RSIResult{
				Value:  30.0,
				Signal: "oversold",
			},
		},
	}

	jsonData, err := json.Marshal(signal)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Verify it can be unmarshaled back
	var decoded TechnicalSignal
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)
	assert.Equal(t, signal.Symbol, decoded.Symbol)
	assert.Equal(t, signal.Signal, decoded.Signal)
	assert.Equal(t, signal.Confidence, decoded.Confidence)
	assert.Equal(t, signal.Price, decoded.Price)
}

// ============================================================================
// Candlestick Tests
// ============================================================================

func TestCandlestick_Validation(t *testing.T) {
	tests := []struct {
		name    string
		candle  Candlestick
		isValid bool
	}{
		{
			name: "valid candlestick",
			candle: Candlestick{
				Timestamp: time.Now().Unix(),
				Open:      50000.0,
				High:      51000.0,
				Low:       49000.0,
				Close:     50500.0,
				Volume:    100.0,
			},
			isValid: true,
		},
		{
			name: "high less than open",
			candle: Candlestick{
				Timestamp: time.Now().Unix(),
				Open:      50000.0,
				High:      49000.0,
				Low:       48000.0,
				Close:     49500.0,
				Volume:    100.0,
			},
			isValid: false,
		},
		{
			name: "low greater than close",
			candle: Candlestick{
				Timestamp: time.Now().Unix(),
				Open:      50000.0,
				High:      51000.0,
				Low:       50500.0,
				Close:     50200.0,
				Volume:    100.0,
			},
			isValid: false,
		},
		{
			name: "negative volume",
			candle: Candlestick{
				Timestamp: time.Now().Unix(),
				Open:      50000.0,
				High:      51000.0,
				Low:       49000.0,
				Close:     50500.0,
				Volume:    -100.0,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate high >= max(open, close)
			highValid := tt.candle.High >= tt.candle.Open && tt.candle.High >= tt.candle.Close
			// Validate low <= min(open, close)
			lowValid := tt.candle.Low <= tt.candle.Open && tt.candle.Low <= tt.candle.Close
			// Validate volume >= 0
			volumeValid := tt.candle.Volume >= 0

			isValid := highValid && lowValid && volumeValid
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkAnalyzeRSI(b *testing.B) {
	config := map[string]interface{}{
		"oversold":   30.0,
		"overbought": 70.0,
	}
	rsi := &RSIResult{Value: 65.5, Signal: "neutral"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = analyzeRSI(rsi, config)
	}
}

func BenchmarkAnalyzeMACD(b *testing.B) {
	macd := &MACDResult{
		MACD:      0.5,
		Signal:    0.3,
		Histogram: 0.2,
		Crossover: "bullish",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = analyzeMACD(macd)
	}
}

func BenchmarkCombineSignals(b *testing.B) {
	signals := []string{"BUY", "BUY", "SELL", "HOLD", "BUY"}
	confidences := []float64{0.9, 0.8, 0.7, 0.6, 0.85}
	weights := []float64{1.5, 1.0, 1.0, 0.8, 1.2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = combineSignals(signals, confidences, weights)
	}
}

func BenchmarkBeliefBase_UpdateBelief(b *testing.B) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb.UpdateBelief("test_key", 42.0, 0.85, "test-source")
	}
}

func BenchmarkBeliefBase_GetBelief(b *testing.B) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}
	bb.UpdateBelief("test_key", 42.0, 0.85, "test-source")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bb.GetBelief("test_key")
	}
}

func BenchmarkBeliefBase_ConcurrentAccess(b *testing.B) {
	bb := &BeliefBase{
		beliefs: make(map[string]*Belief),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bb.UpdateBelief("concurrent_key", 42.0, 0.85, "test")
			_, _ = bb.GetBelief("concurrent_key")
		}
	})
}

// Helper function tests to increase coverage

func TestGetFloat64FromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		key           string
		defaultValue  float64
		expectedValue float64
	}{
		{
			name:          "float64 value",
			config:        map[string]interface{}{"threshold": 70.0},
			key:           "threshold",
			defaultValue:  50.0,
			expectedValue: 70.0,
		},
		{
			name:          "int value converted to float64",
			config:        map[string]interface{}{"threshold": 70},
			key:           "threshold",
			defaultValue:  50.0,
			expectedValue: 70.0,
		},
		{
			name:          "string value converted to float64",
			config:        map[string]interface{}{"threshold": "70.5"},
			key:           "threshold",
			defaultValue:  50.0,
			expectedValue: 70.5,
		},
		{
			name:          "missing key returns default",
			config:        map[string]interface{}{},
			key:           "threshold",
			defaultValue:  50.0,
			expectedValue: 50.0,
		},
		{
			name:          "invalid type returns default",
			config:        map[string]interface{}{"threshold": true},
			key:           "threshold",
			defaultValue:  50.0,
			expectedValue: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFloat64FromConfig(tt.config, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestExtractFloat64(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		key         string
		expected    float64
		expectError bool
	}{
		{
			name:        "float64 value",
			config:      map[string]interface{}{"price": 42.5},
			key:         "price",
			expected:    42.5,
			expectError: false,
		},
		{
			name:        "int value",
			config:      map[string]interface{}{"price": 42},
			key:         "price",
			expected:    42.0,
			expectError: false,
		},
		{
			name:        "string value",
			config:      map[string]interface{}{"price": "42.5"},
			key:         "price",
			expected:    42.5,
			expectError: false,
		},
		{
			name:        "missing key",
			config:      map[string]interface{}{},
			key:         "price",
			expected:    0.0,
			expectError: true,
		},
		{
			name:        "invalid string",
			config:      map[string]interface{}{"price": "not-a-number"},
			key:         "price",
			expected:    0.0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractFloat64(tt.config, tt.key)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetIntFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		key           string
		defaultValue  int
		expectedValue int
	}{
		{
			name:          "int value",
			config:        map[string]interface{}{"period": 14},
			key:           "period",
			defaultValue:  10,
			expectedValue: 14,
		},
		{
			name:          "float64 value converted to int",
			config:        map[string]interface{}{"period": 14.0},
			key:           "period",
			defaultValue:  10,
			expectedValue: 14,
		},
		{
			name:          "string value returns default",
			config:        map[string]interface{}{"period": "14"},
			key:           "period",
			defaultValue:  10,
			expectedValue: 10, // String not converted, returns default
		},
		{
			name:          "missing key returns default",
			config:        map[string]interface{}{},
			key:           "period",
			defaultValue:  10,
			expectedValue: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntFromConfig(tt.config, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestGetFloatFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		key           string
		defaultValue  float64
		expectedValue float64
	}{
		{
			name:          "float64 value",
			config:        map[string]interface{}{"multiplier": 2.5},
			key:           "multiplier",
			defaultValue:  1.0,
			expectedValue: 2.5,
		},
		{
			name:          "missing key returns default",
			config:        map[string]interface{}{},
			key:           "multiplier",
			defaultValue:  1.0,
			expectedValue: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFloatFromConfig(tt.config, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "simple path",
			path:     "indicators.rsi.value",
			expected: []string{"indicators", "rsi", "value"},
		},
		{
			name:     "single element",
			path:     "value",
			expected: []string{"value"},
		},
		{
			name:     "empty path",
			path:     "",
			expected: []string{}, // Empty path returns empty slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{
			name:     "a greater than b",
			a:        10,
			b:        5,
			expected: 10,
		},
		{
			name:     "b greater than a",
			a:        5,
			b:        10,
			expected: 10,
		},
		{
			name:     "equal values",
			a:        10,
			b:        10,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := max(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
