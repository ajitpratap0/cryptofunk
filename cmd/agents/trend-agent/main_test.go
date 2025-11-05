//nolint:goconst // Test files use repeated strings for clarity
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// TrendIndicators Tests
// ============================================================================

func TestTrendIndicators_TrendDetection(t *testing.T) {
	tests := []struct {
		name         string
		fastEMA      float64
		slowEMA      float64
		adx          float64
		threshold    float64
		wantTrend    string
		wantStrength string
	}{
		{
			name:         "strong uptrend",
			fastEMA:      45000.0,
			slowEMA:      44000.0,
			adx:          30.0,
			threshold:    25.0,
			wantTrend:    "uptrend",
			wantStrength: "strong",
		},
		{
			name:         "weak uptrend",
			fastEMA:      45000.0,
			slowEMA:      44000.0,
			adx:          20.0,
			threshold:    25.0,
			wantTrend:    "uptrend",
			wantStrength: "weak",
		},
		{
			name:         "strong downtrend",
			fastEMA:      44000.0,
			slowEMA:      45000.0,
			adx:          28.0,
			threshold:    25.0,
			wantTrend:    "downtrend",
			wantStrength: "strong",
		},
		{
			name:         "weak downtrend",
			fastEMA:      44000.0,
			slowEMA:      45000.0,
			adx:          22.0,
			threshold:    25.0,
			wantTrend:    "downtrend",
			wantStrength: "weak",
		},
		{
			name:         "ranging market",
			fastEMA:      44500.0,
			slowEMA:      44500.0,
			adx:          15.0,
			threshold:    25.0,
			wantTrend:    "ranging",
			wantStrength: "weak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indicators := &TrendIndicators{
				FastEMA: tt.fastEMA,
				SlowEMA: tt.slowEMA,
				ADX:     tt.adx,
			}

			// Determine trend direction
			var trend string
			if indicators.FastEMA > indicators.SlowEMA {
				trend = "uptrend"
			} else if indicators.FastEMA < indicators.SlowEMA {
				trend = "downtrend"
			} else {
				trend = "ranging"
			}

			// Determine strength
			var strength string
			if indicators.ADX >= tt.threshold {
				strength = "strong"
			} else {
				strength = "weak"
			}

			assert.Equal(t, tt.wantTrend, trend)
			assert.Equal(t, tt.wantStrength, strength)
		})
	}
}

// ============================================================================
// Signal Generation Tests
// ============================================================================

func TestGenerateTrendSignal_BuySignal(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:     &agents.BaseAgent{},
		beliefs:       NewBeliefBase(),
		fastEMAPeriod: 9,
		slowEMAPeriod: 21,
		adxThreshold:  25.0,
		lastCrossover: "none",
	}

	indicators := &TrendIndicators{
		FastEMA:   45000.0, // Fast > Slow = Golden Cross
		SlowEMA:   44000.0,
		ADX:       30.0, // Strong trend
		Trend:     "uptrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	signal, err := agent.generateTrendSignal(context.Background(), "bitcoin", indicators, 45200.0)
	require.NoError(t, err)
	require.NotNil(t, signal)

	assert.Equal(t, "BUY", signal.Signal)
	assert.Greater(t, signal.Confidence, 0.5) // Should have decent confidence
	assert.Equal(t, "bitcoin", signal.Symbol)
	assert.Equal(t, 45200.0, signal.Price)
	assert.Contains(t, signal.Reasoning, "uptrend")
	assert.Contains(t, signal.Reasoning, "ADX")
}

func TestGenerateTrendSignal_SellSignal(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:     &agents.BaseAgent{},
		beliefs:       NewBeliefBase(),
		fastEMAPeriod: 9,
		slowEMAPeriod: 21,
		adxThreshold:  25.0,
		lastCrossover: "none",
	}

	indicators := &TrendIndicators{
		FastEMA:   44000.0, // Fast < Slow = Death Cross
		SlowEMA:   45000.0,
		ADX:       28.0, // Strong trend
		Trend:     "downtrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	signal, err := agent.generateTrendSignal(context.Background(), "bitcoin", indicators, 43800.0)
	require.NoError(t, err)
	require.NotNil(t, signal)

	assert.Equal(t, "SELL", signal.Signal)
	assert.Greater(t, signal.Confidence, 0.5)
	assert.Contains(t, signal.Reasoning, "downtrend")
}

func TestGenerateTrendSignal_HoldSignal(t *testing.T) {
	tests := []struct {
		name       string
		indicators *TrendIndicators
		reason     string
	}{
		{
			name: "weak trend - low ADX",
			indicators: &TrendIndicators{
				FastEMA:   45000.0,
				SlowEMA:   44000.0,
				ADX:       15.0, // Below threshold
				Trend:     "uptrend",
				Strength:  "weak",
				Timestamp: time.Now(),
			},
			reason: "Weak trend",
		},
		{
			name: "ranging market",
			indicators: &TrendIndicators{
				FastEMA:   44500.0,
				SlowEMA:   44500.0, // EMAs converged
				ADX:       30.0,
				Trend:     "ranging",
				Strength:  "strong",
				Timestamp: time.Now(),
			},
			reason: "converged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &TrendAgent{
				BaseAgent:     &agents.BaseAgent{},
				beliefs:       NewBeliefBase(),
				fastEMAPeriod: 9,
				slowEMAPeriod: 21,
				adxThreshold:  25.0,
				lastCrossover: "none",
			}

			signal, err := agent.generateTrendSignal(context.Background(), "bitcoin", tt.indicators, 44500.0)
			require.NoError(t, err)
			require.NotNil(t, signal)

			assert.Equal(t, "HOLD", signal.Signal)
			assert.Contains(t, signal.Reasoning, tt.reason)
		})
	}
}

// ============================================================================
// Confidence Scoring Tests
// ============================================================================

func TestConfidenceScoring_Algorithm(t *testing.T) {
	tests := []struct {
		name          string
		fastEMA       float64
		slowEMA       float64
		adx           float64
		expectedRange [2]float64 // Min and max expected confidence
	}{
		{
			name:          "high confidence - strong ADX, large EMA separation",
			fastEMA:       45000.0,
			slowEMA:       44100.0,               // 2% separation
			adx:           40.0,                  // 0.4 normalized
			expectedRange: [2]float64{0.60, 1.0}, // Should be high
		},
		{
			name:          "medium confidence - moderate indicators",
			fastEMA:       45000.0,
			slowEMA:       44550.0, // 1% separation
			adx:           25.0,    // 0.25 normalized
			expectedRange: [2]float64{0.30, 0.60},
		},
		{
			name:          "low confidence - weak indicators",
			fastEMA:       45000.0,
			slowEMA:       44900.0, // 0.2% separation
			adx:           15.0,    // 0.15 normalized
			expectedRange: [2]float64{0.13, 0.35},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate confidence using the algorithm from generateTrendSignal
			emaDiff := tt.fastEMA - tt.slowEMA
			emaPercent := (emaDiff / tt.slowEMA) * 100

			// Normalize ADX to 0-1 range (ADX typically 0-100)
			adxConfidence := min(tt.adx/100, 1.0)

			// Normalize EMA difference (assume 2% separation = full confidence)
			emaConfidence := min(abs(emaPercent)/2.0, 1.0)

			// Weighted average: 60% ADX, 40% EMA separation
			confidence := (adxConfidence * 0.6) + (emaConfidence * 0.4)

			assert.GreaterOrEqual(t, confidence, tt.expectedRange[0], "Confidence below expected minimum")
			assert.LessOrEqual(t, confidence, tt.expectedRange[1], "Confidence above expected maximum")
		})
	}
}

// Helper function for min
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Helper function for abs
func abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

// ============================================================================
// Crossover Detection Tests
// ============================================================================

func TestCrossoverDetection(t *testing.T) {
	tests := []struct {
		name              string
		fastEMA           float64
		slowEMA           float64
		lastCrossover     string
		expectedCrossover string
		shouldLog         bool
	}{
		{
			name:              "golden cross from no crossover",
			fastEMA:           45000.0,
			slowEMA:           44000.0,
			lastCrossover:     "none",
			expectedCrossover: "bullish",
			shouldLog:         true,
		},
		{
			name:              "death cross from no crossover",
			fastEMA:           44000.0,
			slowEMA:           45000.0,
			lastCrossover:     "none",
			expectedCrossover: "bearish",
			shouldLog:         true,
		},
		{
			name:              "golden cross from bearish",
			fastEMA:           45000.0,
			slowEMA:           44000.0,
			lastCrossover:     "bearish",
			expectedCrossover: "bullish",
			shouldLog:         true,
		},
		{
			name:              "death cross from bullish",
			fastEMA:           44000.0,
			slowEMA:           45000.0,
			lastCrossover:     "bullish",
			expectedCrossover: "bearish",
			shouldLog:         true,
		},
		{
			name:              "continued bullish crossover",
			fastEMA:           45000.0,
			slowEMA:           44000.0,
			lastCrossover:     "bullish",
			expectedCrossover: "bullish",
			shouldLog:         false, // No change, shouldn't log
		},
		{
			name:              "continued bearish crossover",
			fastEMA:           44000.0,
			slowEMA:           45000.0,
			lastCrossover:     "bearish",
			expectedCrossover: "bearish",
			shouldLog:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate crossover detection logic from calculateTrendIndicators
			var crossover string
			if tt.fastEMA > tt.slowEMA {
				crossover = "bullish"
			} else if tt.fastEMA < tt.slowEMA {
				crossover = "bearish"
			} else {
				crossover = "none"
			}

			shouldLog := tt.lastCrossover != crossover

			assert.Equal(t, tt.expectedCrossover, crossover)
			assert.Equal(t, tt.shouldLog, shouldLog, "Logging decision incorrect")
		})
	}
}

// ============================================================================
// Signal JSON Marshaling Tests
// ============================================================================

func TestTrendSignal_JSONMarshaling(t *testing.T) {
	signal := &TrendSignal{
		Timestamp:  time.Date(2025, 10, 28, 12, 0, 0, 0, time.UTC),
		Symbol:     "bitcoin",
		Signal:     "BUY",
		Confidence: 0.82,
		Indicators: &TrendIndicators{
			FastEMA:   45000.0,
			SlowEMA:   44000.0,
			ADX:       30.0,
			Trend:     "uptrend",
			Strength:  "strong",
			Timestamp: time.Date(2025, 10, 28, 12, 0, 0, 0, time.UTC),
		},
		Reasoning: "Strong uptrend: Fast EMA (45000.00) > Slow EMA (44000.00), ADX=30.0 (>25)",
		Price:     45200.0,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(signal)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Unmarshal back
	var decoded TrendSignal
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, signal.Symbol, decoded.Symbol)
	assert.Equal(t, signal.Signal, decoded.Signal)
	assert.Equal(t, signal.Confidence, decoded.Confidence)
	assert.Equal(t, signal.Price, decoded.Price)
	assert.Equal(t, signal.Reasoning, decoded.Reasoning)
	require.NotNil(t, decoded.Indicators)
	assert.Equal(t, signal.Indicators.FastEMA, decoded.Indicators.FastEMA)
	assert.Equal(t, signal.Indicators.SlowEMA, decoded.Indicators.SlowEMA)
	assert.Equal(t, signal.Indicators.ADX, decoded.Indicators.ADX)
}

// ============================================================================
// Configuration Helper Tests
// ============================================================================

func TestGetIntFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "get existing int",
			config:       map[string]interface{}{"period": 14},
			key:          "period",
			defaultValue: 10,
			expected:     14,
		},
		{
			name:         "get existing float64 as int",
			config:       map[string]interface{}{"period": 14.0},
			key:          "period",
			defaultValue: 10,
			expected:     14,
		},
		{
			name:         "get missing key returns default",
			config:       map[string]interface{}{},
			key:          "period",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "get invalid type returns default",
			config:       map[string]interface{}{"period": "invalid"},
			key:          "period",
			defaultValue: 10,
			expected:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntFromConfig(tt.config, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFloatFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		key          string
		defaultValue float64
		expected     float64
	}{
		{
			name:         "get existing float64",
			config:       map[string]interface{}{"threshold": 25.5},
			key:          "threshold",
			defaultValue: 20.0,
			expected:     25.5,
		},
		{
			name:         "get existing int as float64",
			config:       map[string]interface{}{"threshold": 25},
			key:          "threshold",
			defaultValue: 20.0,
			expected:     25.0,
		},
		{
			name:         "get missing key returns default",
			config:       map[string]interface{}{},
			key:          "threshold",
			defaultValue: 20.0,
			expected:     20.0,
		},
		{
			name:         "get invalid type returns default",
			config:       map[string]interface{}{"threshold": "invalid"},
			key:          "threshold",
			defaultValue: 20.0,
			expected:     20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFloatFromConfig(tt.config, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Integration-Style Tests
// ============================================================================

func TestTrendAgent_FullDecisionCycle(t *testing.T) {
	// This test verifies the full logic flow without external dependencies
	// It tests the decision-making process with various market conditions

	testCases := []struct {
		name           string
		indicators     *TrendIndicators
		expectedSignal string
		minConfidence  float64
	}{
		{
			name: "strong bullish trend",
			indicators: &TrendIndicators{
				FastEMA:   50000.0,
				SlowEMA:   48000.0,
				ADX:       35.0,
				Trend:     "uptrend",
				Strength:  "strong",
				Timestamp: time.Now(),
			},
			expectedSignal: "BUY",
			minConfidence:  0.60,
		},
		{
			name: "strong bearish trend",
			indicators: &TrendIndicators{
				FastEMA:   48000.0,
				SlowEMA:   50000.0,
				ADX:       32.0,
				Trend:     "downtrend",
				Strength:  "strong",
				Timestamp: time.Now(),
			},
			expectedSignal: "SELL",
			minConfidence:  0.55,
		},
		{
			name: "weak uptrend should hold",
			indicators: &TrendIndicators{
				FastEMA:   50000.0,
				SlowEMA:   49000.0,
				ADX:       18.0,
				Trend:     "uptrend",
				Strength:  "weak",
				Timestamp: time.Now(),
			},
			expectedSignal: "HOLD",
			minConfidence:  0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent := &TrendAgent{
				BaseAgent:     &agents.BaseAgent{},
				beliefs:       NewBeliefBase(),
				adxThreshold:  25.0,
				fastEMAPeriod: 9,
				slowEMAPeriod: 21,
				lastCrossover: "none",
			}

			signal, err := agent.generateTrendSignal(context.Background(), "bitcoin", tc.indicators, 50000.0)
			require.NoError(t, err)
			require.NotNil(t, signal)

			assert.Equal(t, tc.expectedSignal, signal.Signal)
			if tc.minConfidence > 0 {
				assert.GreaterOrEqual(t, signal.Confidence, tc.minConfidence)
			}
			assert.NotEmpty(t, signal.Reasoning)
			assert.Equal(t, "bitcoin", signal.Symbol)
		})
	}
}

// ============================================================================
// Utility Function Tests
// ============================================================================

func TestExtractFloat64(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		key      string
		expected float64
		wantErr  bool
	}{
		{
			name:     "extract existing float64",
			data:     map[string]interface{}{"value": 45.5},
			key:      "value",
			expected: 45.5,
			wantErr:  false,
		},
		{
			name:     "extract existing int as float64",
			data:     map[string]interface{}{"value": 45},
			key:      "value",
			expected: 45.0,
			wantErr:  false,
		},
		{
			name:     "extract missing key",
			data:     map[string]interface{}{},
			key:      "value",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "extract invalid type",
			data:     map[string]interface{}{"value": "not a number"},
			key:      "value",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractFloat64(tt.data, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
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
			name:     "first greater",
			a:        10,
			b:        5,
			expected: 10,
		},
		{
			name:     "second greater",
			a:        5,
			b:        10,
			expected: 10,
		},
		{
			name:     "equal values",
			a:        7,
			b:        7,
			expected: 7,
		},
		{
			name:     "negative values",
			a:        -5,
			b:        -10,
			expected: -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := max(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkGenerateTrendSignal(b *testing.B) {
	agent := &TrendAgent{
		BaseAgent:     &agents.BaseAgent{},
		beliefs:       NewBeliefBase(),
		adxThreshold:  25.0,
		fastEMAPeriod: 9,
		slowEMAPeriod: 21,
		lastCrossover: "none",
	}

	indicators := &TrendIndicators{
		FastEMA:   45000.0,
		SlowEMA:   44000.0,
		ADX:       30.0,
		Trend:     "uptrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = agent.generateTrendSignal(ctx, "bitcoin", indicators, 45000.0)
	}
}

func BenchmarkConfidenceCalculation(b *testing.B) {
	fastEMA := 45000.0
	slowEMA := 44000.0
	adx := 30.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		emaDiff := fastEMA - slowEMA
		emaPercent := (emaDiff / slowEMA) * 100
		adxConfidence := min(adx/100, 1.0)
		emaConfidence := min(abs(emaPercent)/2.0, 1.0)
		_ = (adxConfidence * 0.6) + (emaConfidence * 0.4)
	}
}

// ============================================================================
// Risk Management Tests (T079+T080)
// ============================================================================

func TestCalculateStopLoss(t *testing.T) {
	agent := &TrendAgent{
		stopLossPct: 0.02, // 2%
	}

	tests := []struct {
		name       string
		entryPrice float64
		signal     string
		expected   float64
	}{
		{
			name:       "buy signal - stop loss below entry",
			entryPrice: 50000.0,
			signal:     "BUY",
			expected:   49000.0, // 50000 * (1 - 0.02)
		},
		{
			name:       "sell signal - stop loss above entry",
			entryPrice: 50000.0,
			signal:     "SELL",
			expected:   51000.0, // 50000 * (1 + 0.02)
		},
		{
			name:       "hold signal - no stop loss",
			entryPrice: 50000.0,
			signal:     "HOLD",
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.calculateStopLoss(tt.entryPrice, tt.signal)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestCalculateTakeProfit(t *testing.T) {
	agent := &TrendAgent{
		takeProfitPct: 0.03, // 3%
	}

	tests := []struct {
		name       string
		entryPrice float64
		signal     string
		expected   float64
	}{
		{
			name:       "buy signal - take profit above entry",
			entryPrice: 50000.0,
			signal:     "BUY",
			expected:   51500.0, // 50000 * (1 + 0.03)
		},
		{
			name:       "sell signal - take profit below entry",
			entryPrice: 50000.0,
			signal:     "SELL",
			expected:   48500.0, // 50000 * (1 - 0.03)
		},
		{
			name:       "hold signal - no take profit",
			entryPrice: 50000.0,
			signal:     "HOLD",
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.calculateTakeProfit(tt.entryPrice, tt.signal)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestCalculateRiskReward(t *testing.T) {
	agent := &TrendAgent{}

	tests := []struct {
		name       string
		entryPrice float64
		stopLoss   float64
		takeProfit float64
		expected   float64
	}{
		{
			name:       "2:1 risk/reward for long",
			entryPrice: 50000.0,
			stopLoss:   49000.0, // Risk: 1000
			takeProfit: 52000.0, // Reward: 2000
			expected:   2.0,
		},
		{
			name:       "3:1 risk/reward for short",
			entryPrice: 50000.0,
			stopLoss:   51000.0, // Risk: 1000
			takeProfit: 47000.0, // Reward: 3000
			expected:   3.0,
		},
		{
			name:       "1:1 risk/reward",
			entryPrice: 50000.0,
			stopLoss:   49000.0, // Risk: 1000
			takeProfit: 51000.0, // Reward: 1000
			expected:   1.0,
		},
		{
			name:       "zero risk returns zero",
			entryPrice: 50000.0,
			stopLoss:   50000.0, // Risk: 0
			takeProfit: 51000.0,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.calculateRiskReward(tt.entryPrice, tt.stopLoss, tt.takeProfit)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestUpdateTrailingStop_LongPosition(t *testing.T) {
	agent := &TrendAgent{
		useTrailingStop: true,
		trailingStopPct: 0.015, // 1.5%
		lastSignal:      "HOLD",
	}

	// First BUY signal - should initialize tracking
	trailingStop := agent.updateTrailingStop(50000.0, "BUY")
	assert.Equal(t, 50000.0, agent.entryPrice)
	assert.Equal(t, 50000.0, agent.highestPrice)
	assert.InDelta(t, 49250.0, trailingStop, 1.0) // 50000 * (1 - 0.015)

	// Update lastSignal to simulate production behavior
	agent.lastSignal = "BUY"

	// Price goes up - trailing stop should move up
	trailingStop = agent.updateTrailingStop(51000.0, "BUY")
	assert.Equal(t, 51000.0, agent.highestPrice)
	assert.InDelta(t, 50235.0, trailingStop, 1.0) // 51000 * (1 - 0.015)

	// Price goes down - trailing stop should NOT move down
	trailingStop = agent.updateTrailingStop(50500.0, "BUY")
	assert.Equal(t, 51000.0, agent.highestPrice)  // Still 51000
	assert.InDelta(t, 50235.0, trailingStop, 1.0) // Still based on 51000
}

func TestUpdateTrailingStop_ShortPosition(t *testing.T) {
	agent := &TrendAgent{
		useTrailingStop: true,
		trailingStopPct: 0.015, // 1.5%
		lastSignal:      "HOLD",
	}

	// First SELL signal - should initialize tracking
	trailingStop := agent.updateTrailingStop(50000.0, "SELL")
	assert.Equal(t, 50000.0, agent.entryPrice)
	assert.Equal(t, 50000.0, agent.lowestPrice)
	assert.InDelta(t, 50750.0, trailingStop, 1.0) // 50000 * (1 + 0.015)

	// Update lastSignal to simulate production behavior
	agent.lastSignal = "SELL"

	// Price goes down - trailing stop should move down
	trailingStop = agent.updateTrailingStop(49000.0, "SELL")
	assert.Equal(t, 49000.0, agent.lowestPrice)
	assert.InDelta(t, 49735.0, trailingStop, 1.0) // 49000 * (1 + 0.015)

	// Price goes up - trailing stop should NOT move up
	trailingStop = agent.updateTrailingStop(49500.0, "SELL")
	assert.Equal(t, 49000.0, agent.lowestPrice)   // Still 49000
	assert.InDelta(t, 49735.0, trailingStop, 1.0) // Still based on 49000
}

func TestUpdateTrailingStop_Disabled(t *testing.T) {
	agent := &TrendAgent{
		useTrailingStop: false,
	}

	trailingStop := agent.updateTrailingStop(50000.0, "BUY")
	assert.Equal(t, 0.0, trailingStop)
}

func TestResetPositionTracking(t *testing.T) {
	agent := &TrendAgent{
		entryPrice:   50000.0,
		highestPrice: 51000.0,
		lowestPrice:  49000.0,
	}

	agent.resetPositionTracking()

	assert.Equal(t, 0.0, agent.entryPrice)
	assert.Equal(t, 0.0, agent.highestPrice)
	assert.Equal(t, 0.0, agent.lowestPrice)
}

func TestRiskManagement_SignalFields(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:       &agents.BaseAgent{},
		beliefs:         NewBeliefBase(),
		adxThreshold:    25.0,
		stopLossPct:     0.01, // 1% stop loss
		takeProfitPct:   0.03, // 3% take profit
		trailingStopPct: 0.015,
		useTrailingStop: true,
		riskRewardRatio: 2.0, // Require 2:1, actual will be 3:1
		lastSignal:      "HOLD",
	}

	indicators := &TrendIndicators{
		FastEMA:   50000.0,
		SlowEMA:   48000.0,
		ADX:       30.0,
		Trend:     "uptrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	signal, err := agent.generateTrendSignal(ctx, "bitcoin", indicators, 50000.0)

	require.NoError(t, err)
	assert.Equal(t, "BUY", signal.Signal)
	assert.Greater(t, signal.StopLoss, 0.0)
	assert.Greater(t, signal.TakeProfit, 0.0)
	assert.Greater(t, signal.RiskReward, 0.0)
	assert.InDelta(t, 49500.0, signal.StopLoss, 1.0)   // 50000 * (1 - 0.01)
	assert.InDelta(t, 51500.0, signal.TakeProfit, 1.0) // 50000 * (1 + 0.03)
	assert.InDelta(t, 3.0, signal.RiskReward, 0.1)     // (1500 / 500) = 3.0
}

func TestRiskManagement_LowRiskRewardConvertsToHold(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:       &agents.BaseAgent{},
		beliefs:         NewBeliefBase(),
		adxThreshold:    25.0,
		stopLossPct:     0.03, // 3% stop loss
		takeProfitPct:   0.02, // 2% take profit (lower than stop loss)
		riskRewardRatio: 2.0,  // Require 2:1, but actual will be 0.67:1
		useTrailingStop: false,
		lastSignal:      "HOLD",
	}

	indicators := &TrendIndicators{
		FastEMA:   50000.0,
		SlowEMA:   48000.0,
		ADX:       30.0,
		Trend:     "uptrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	signal, err := agent.generateTrendSignal(ctx, "bitcoin", indicators, 50000.0)

	require.NoError(t, err)
	// Signal should be converted to HOLD because risk/reward is too low
	assert.Equal(t, "HOLD", signal.Signal)
	assert.Equal(t, 0.0, signal.StopLoss)
	assert.Equal(t, 0.0, signal.TakeProfit)
	assert.Equal(t, 0.0, signal.RiskReward)
	assert.Contains(t, signal.Reasoning, "risk/reward")
}

// ============================================================================
// BDI (Belief-Desire-Intention) Belief System Tests
// ============================================================================

func TestBeliefBase_UpdateAndRetrieve(t *testing.T) {
	bb := NewBeliefBase()

	// Update a belief
	bb.UpdateBelief("trend_direction", "uptrend", 0.8, "EMA")

	// Retrieve it
	belief, exists := bb.GetBelief("trend_direction")
	require.True(t, exists)
	assert.Equal(t, "trend_direction", belief.Key)
	assert.Equal(t, "uptrend", belief.Value)
	assert.Equal(t, 0.8, belief.Confidence)
	assert.Equal(t, "EMA", belief.Source)
	assert.False(t, belief.Timestamp.IsZero())
}

func TestBeliefBase_GetNonExistent(t *testing.T) {
	bb := NewBeliefBase()

	belief, exists := bb.GetBelief("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, belief)
}

func TestBeliefBase_UpdateExisting(t *testing.T) {
	bb := NewBeliefBase()

	// Initial belief
	bb.UpdateBelief("trend_direction", "uptrend", 0.6, "EMA")
	firstTimestamp, _ := bb.GetBelief("trend_direction")

	// Update with new value
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference
	bb.UpdateBelief("trend_direction", "downtrend", 0.9, "ADX")

	// Verify update
	belief, exists := bb.GetBelief("trend_direction")
	require.True(t, exists)
	assert.Equal(t, "downtrend", belief.Value)
	assert.Equal(t, 0.9, belief.Confidence)
	assert.Equal(t, "ADX", belief.Source)
	assert.True(t, belief.Timestamp.After(firstTimestamp.Timestamp))
}

func TestBeliefBase_GetAllBeliefs(t *testing.T) {
	bb := NewBeliefBase()

	// Add multiple beliefs
	bb.UpdateBelief("trend_direction", "uptrend", 0.8, "EMA")
	bb.UpdateBelief("trend_strength", "strong", 0.7, "ADX")
	bb.UpdateBelief("position_state", "long", 1.0, "agent_state")

	// Get all
	beliefs := bb.GetAllBeliefs()
	assert.Equal(t, 3, len(beliefs))

	// Verify all beliefs present
	assert.Contains(t, beliefs, "trend_direction")
	assert.Contains(t, beliefs, "trend_strength")
	assert.Contains(t, beliefs, "position_state")

	// Verify it's a copy (modify shouldn't affect original)
	beliefs["new_belief"] = &Belief{Key: "test"}
	assert.Equal(t, 3, len(bb.GetAllBeliefs())) // Still 3
}

func TestBeliefBase_GetConfidence(t *testing.T) {
	bb := NewBeliefBase()

	// Empty belief base
	assert.Equal(t, 0.0, bb.GetConfidence())

	// Add beliefs with known confidences
	bb.UpdateBelief("belief1", "value1", 0.6, "source1")
	bb.UpdateBelief("belief2", "value2", 0.8, "source2")
	bb.UpdateBelief("belief3", "value3", 1.0, "source3")

	// Average: (0.6 + 0.8 + 1.0) / 3 = 0.8
	assert.InDelta(t, 0.8, bb.GetConfidence(), 0.01)
}

func TestUpdateBeliefs_StrongUptrend(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:    &agents.BaseAgent{},
		beliefs:      NewBeliefBase(),
		adxThreshold: 25.0,
		lastSignal:   "HOLD",
	}

	indicators := &TrendIndicators{
		FastEMA:   50000.0,
		SlowEMA:   48000.0,
		ADX:       35.0,
		Trend:     "uptrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	agent.updateBeliefs("bitcoin", indicators, 50000.0)

	// Verify trend direction belief
	trendDir, exists := agent.beliefs.GetBelief("trend_direction")
	require.True(t, exists)
	assert.Equal(t, "uptrend", trendDir.Value)
	assert.Equal(t, 0.8, trendDir.Confidence) // Strong trend = 0.8
	assert.Equal(t, "EMA_crossover", trendDir.Source)

	// Verify trend strength belief
	trendStrength, exists := agent.beliefs.GetBelief("trend_strength")
	require.True(t, exists)
	assert.Equal(t, "strong", trendStrength.Value)
	assert.InDelta(t, 0.35, trendStrength.Confidence, 0.01) // ADX/100 = 35/100
	assert.Equal(t, "ADX", trendStrength.Source)

	// Verify EMA beliefs
	fastEMA, exists := agent.beliefs.GetBelief("fast_ema")
	require.True(t, exists)
	assert.Equal(t, 50000.0, fastEMA.Value)
	assert.Equal(t, 0.9, fastEMA.Confidence)

	slowEMA, exists := agent.beliefs.GetBelief("slow_ema")
	require.True(t, exists)
	assert.Equal(t, 48000.0, slowEMA.Value)
	assert.Equal(t, 0.9, slowEMA.Confidence)

	// Verify position state
	posState, exists := agent.beliefs.GetBelief("position_state")
	require.True(t, exists)
	assert.Equal(t, "none", posState.Value) // lastSignal was HOLD
	assert.Equal(t, 1.0, posState.Confidence)

	// Verify price and symbol
	price, exists := agent.beliefs.GetBelief("current_price")
	require.True(t, exists)
	assert.Equal(t, 50000.0, price.Value)

	symbol, exists := agent.beliefs.GetBelief("symbol")
	require.True(t, exists)
	assert.Equal(t, "bitcoin", symbol.Value)

	// Overall confidence should be reasonable
	assert.Greater(t, agent.beliefs.GetConfidence(), 0.5)
}

func TestUpdateBeliefs_WeakDowntrend(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:    &agents.BaseAgent{},
		beliefs:      NewBeliefBase(),
		adxThreshold: 25.0,
		lastSignal:   "SELL",
	}

	indicators := &TrendIndicators{
		FastEMA:   48000.0,
		SlowEMA:   50000.0,
		ADX:       18.0, // Below threshold
		Trend:     "downtrend",
		Strength:  "weak",
		Timestamp: time.Now(),
	}

	agent.updateBeliefs("ethereum", indicators, 48000.0)

	// Verify trend direction belief
	trendDir, exists := agent.beliefs.GetBelief("trend_direction")
	require.True(t, exists)
	assert.Equal(t, "downtrend", trendDir.Value)
	assert.Equal(t, 0.4, trendDir.Confidence) // Weak trend = 0.4

	// Verify trend strength belief
	trendStrength, exists := agent.beliefs.GetBelief("trend_strength")
	require.True(t, exists)
	assert.Equal(t, "weak", trendStrength.Value)
	assert.InDelta(t, 0.18, trendStrength.Confidence, 0.01) // ADX/100 = 18/100

	// Verify position state (should be "short" because lastSignal was SELL)
	posState, exists := agent.beliefs.GetBelief("position_state")
	require.True(t, exists)
	assert.Equal(t, "short", posState.Value)
}

func TestUpdateBeliefs_RangingMarket(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:    &agents.BaseAgent{},
		beliefs:      NewBeliefBase(),
		adxThreshold: 25.0,
		lastSignal:   "BUY",
	}

	indicators := &TrendIndicators{
		FastEMA:   49500.0,
		SlowEMA:   49500.0, // EMAs converged
		ADX:       20.0,
		Trend:     "ranging",
		Strength:  "weak",
		Timestamp: time.Now(),
	}

	agent.updateBeliefs("bitcoin", indicators, 49500.0)

	// Verify trend direction belief
	trendDir, exists := agent.beliefs.GetBelief("trend_direction")
	require.True(t, exists)
	assert.Equal(t, "ranging", trendDir.Value)
	assert.Equal(t, 0.4, trendDir.Confidence) // Weak = 0.4

	// Position state should be "long" because lastSignal was BUY
	posState, exists := agent.beliefs.GetBelief("position_state")
	require.True(t, exists)
	assert.Equal(t, "long", posState.Value)
}

func TestSignalContainsBeliefs(t *testing.T) {
	agent := &TrendAgent{
		BaseAgent:       &agents.BaseAgent{},
		beliefs:         NewBeliefBase(),
		adxThreshold:    25.0,
		stopLossPct:     0.02,
		takeProfitPct:   0.03,
		trailingStopPct: 0.015,
		useTrailingStop: true,
		riskRewardRatio: 1.0, // Lower threshold so signal passes risk/reward check
		lastSignal:      "HOLD",
	}

	// Update beliefs first
	indicators := &TrendIndicators{
		FastEMA:   50000.0,
		SlowEMA:   48000.0,
		ADX:       30.0,
		Trend:     "uptrend",
		Strength:  "strong",
		Timestamp: time.Now(),
	}

	agent.updateBeliefs("bitcoin", indicators, 50000.0)

	// Generate signal
	ctx := context.Background()
	signal, err := agent.generateTrendSignal(ctx, "bitcoin", indicators, 50000.0)

	require.NoError(t, err)
	assert.Equal(t, "BUY", signal.Signal)

	// Verify beliefs are included in signal
	require.NotNil(t, signal.Beliefs)
	assert.Greater(t, len(signal.Beliefs), 0)

	// Spot check some beliefs
	assert.Contains(t, signal.Beliefs, "trend_direction")
	assert.Contains(t, signal.Beliefs, "trend_strength")
	assert.Contains(t, signal.Beliefs, "fast_ema")
	assert.Contains(t, signal.Beliefs, "slow_ema")
	assert.Contains(t, signal.Beliefs, "current_price")
}

func TestBeliefBase_ThreadSafety(t *testing.T) {
	bb := NewBeliefBase()
	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				bb.UpdateBelief(
					fmt.Sprintf("belief_%d", id),
					fmt.Sprintf("value_%d", j),
					float64(j)/100.0,
					"test",
				)
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				bb.GetAllBeliefs()
				bb.GetConfidence()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Verify data integrity
	beliefs := bb.GetAllBeliefs()
	assert.Equal(t, 10, len(beliefs)) // Should have 10 beliefs (one per writer goroutine)
}

// TestMain can be used for setup/teardown if needed
func TestMain(m *testing.M) {
	// Disable logging during tests
	zerolog.SetGlobalLevel(zerolog.Disabled)

	// Run tests
	m.Run()
}
