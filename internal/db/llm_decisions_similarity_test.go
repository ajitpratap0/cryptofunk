package db

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateIndicatorSimilarity(t *testing.T) {
	tests := []struct {
		name               string
		currentIndicators  map[string]interface{}
		decisionContext    map[string]interface{}
		expectedMinScore   float64
		expectedMaxScore   float64
	}{
		{
			name: "Exact match - all indicators",
			currentIndicators: map[string]interface{}{
				"RSI":  65.0,
				"MACD": 125.0,
				"ADX":  28.0,
			},
			decisionContext: map[string]interface{}{
				"indicators": map[string]interface{}{
					"RSI":  65.0,
					"MACD": 125.0,
					"ADX":  28.0,
				},
			},
			expectedMinScore: 3.0,
			expectedMaxScore: 3.0,
		},
		{
			name: "Close match - within 15% tolerance",
			currentIndicators: map[string]interface{}{
				"RSI":  65.0,
				"MACD": 125.0,
			},
			decisionContext: map[string]interface{}{
				"indicators": map[string]interface{}{
					"RSI":  67.0,  // ~3% diff
					"MACD": 130.0, // ~4% diff
				},
			},
			expectedMinScore: 2.0,
			expectedMaxScore: 2.0,
		},
		{
			name: "Partial match - one within tolerance, one outside",
			currentIndicators: map[string]interface{}{
				"RSI":  65.0,
				"MACD": 125.0,
			},
			decisionContext: map[string]interface{}{
				"indicators": map[string]interface{}{
					"RSI":  67.0,  // ~3% diff (within tolerance)
					"MACD": 200.0, // ~46% diff (outside tolerance)
				},
			},
			expectedMinScore: 1.0,
			expectedMaxScore: 1.0,
		},
		{
			name: "No matching indicators",
			currentIndicators: map[string]interface{}{
				"RSI":  65.0,
				"MACD": 125.0,
			},
			decisionContext: map[string]interface{}{
				"indicators": map[string]interface{}{
					"EMA":    50000.0,
					"Volume": 1000000.0,
				},
			},
			expectedMinScore: 0.0,
			expectedMaxScore: 0.0,
		},
		{
			name: "Missing indicators in decision context",
			currentIndicators: map[string]interface{}{
				"RSI": 65.0,
			},
			decisionContext: map[string]interface{}{
				"current_price": 50000.0,
			},
			expectedMinScore: 0.0,
			expectedMaxScore: 0.0,
		},
		{
			name: "Empty decision context",
			currentIndicators: map[string]interface{}{
				"RSI": 65.0,
			},
			decisionContext:  map[string]interface{}{},
			expectedMinScore: 0.0,
			expectedMaxScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextJSON, err := json.Marshal(tt.decisionContext)
			assert.NoError(t, err)

			score := calculateIndicatorSimilarity(tt.currentIndicators, contextJSON)
			assert.GreaterOrEqual(t, score, tt.expectedMinScore)
			assert.LessOrEqual(t, score, tt.expectedMaxScore)
		})
	}
}

func TestCalculateIndicatorSimilarity_InvalidJSON(t *testing.T) {
	currentIndicators := map[string]interface{}{
		"RSI": 65.0,
	}

	score := calculateIndicatorSimilarity(currentIndicators, []byte("invalid json"))
	assert.Equal(t, 0.0, score)
}

func TestCalculateIndicatorSimilarity_EmptyJSON(t *testing.T) {
	currentIndicators := map[string]interface{}{
		"RSI": 65.0,
	}

	score := calculateIndicatorSimilarity(currentIndicators, []byte{})
	assert.Equal(t, 0.0, score)
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"float64", float64(65.5), 65.5},
		{"float32", float32(65.5), 65.5},
		{"int", int(65), 65.0},
		{"int64", int64(65), 65.0},
		{"int32", int32(65), 65.0},
		{"string (unsupported)", "65.5", 0.0},
		{"nil", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloat64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateIndicatorSimilarity_Tolerance(t *testing.T) {
	// Test that 15% tolerance works correctly
	currentIndicators := map[string]interface{}{
		"RSI": 100.0,
	}

	tests := []struct {
		name          string
		pastValue     float64
		shouldMatch   bool
		expectedScore float64
	}{
		{"Exact match", 100.0, true, 1.0},
		{"Within tolerance (12%)", 112.0, true, 1.0},
		{"Within tolerance (14%)", 114.0, true, 1.0},
		{"Just outside tolerance (18%)", 118.0, false, 0.0},
		{"Within tolerance (negative 12%)", 88.0, true, 1.0},
		{"Outside tolerance (negative 18%)", 82.0, false, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decisionContext := map[string]interface{}{
				"indicators": map[string]interface{}{
					"RSI": tt.pastValue,
				},
			}
			contextJSON, _ := json.Marshal(decisionContext)

			score := calculateIndicatorSimilarity(currentIndicators, contextJSON)
			assert.Equal(t, tt.expectedScore, score)
		})
	}
}

func TestCalculateIndicatorSimilarity_MultipleIndicators(t *testing.T) {
	currentIndicators := map[string]interface{}{
		"RSI":  65.0,
		"MACD": 125.0,
		"ADX":  28.0,
		"EMA":  50000.0,
	}

	// Test with varying degrees of similarity
	decisionContext := map[string]interface{}{
		"indicators": map[string]interface{}{
			"RSI":  67.0,   // Match (3% diff)
			"MACD": 130.0,  // Match (4% diff)
			"ADX":  35.0,   // No match (22% diff)
			"EMA":  51000.0, // Match (2% diff)
			"SMA":  49500.0, // Not in current indicators
		},
	}

	contextJSON, _ := json.Marshal(decisionContext)
	score := calculateIndicatorSimilarity(currentIndicators, contextJSON)

	// Should match 3 out of 4 indicators (RSI, MACD, EMA)
	assert.Equal(t, 3.0, score)
}
