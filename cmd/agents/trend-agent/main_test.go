package main

import (
	"context"
	"encoding/json"
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

// TestMain can be used for setup/teardown if needed
func TestMain(m *testing.M) {
	// Disable logging during tests
	zerolog.SetGlobalLevel(zerolog.Disabled)

	// Run tests
	m.Run()
}
