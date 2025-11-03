package memory

import (
	"context"
	"testing"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPatternCandidate_SuccessRate tests pattern success rate calculation
func TestPatternCandidate_SuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expectedRate float64
	}{
		{
			name:         "No data",
			successCount: 0,
			failureCount: 0,
			expectedRate: 0.0,
		},
		{
			name:         "All successful",
			successCount: 15,
			failureCount: 0,
			expectedRate: 1.0,
		},
		{
			name:         "All failed",
			successCount: 0,
			failureCount: 15,
			expectedRate: 0.0,
		},
		{
			name:         "Mixed - 75% success",
			successCount: 15,
			failureCount: 5,
			expectedRate: 0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := &PatternCandidate{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			assert.Equal(t, tt.expectedRate, pattern.SuccessRate())
		})
	}
}

// TestPatternCandidate_Confidence tests confidence scoring
func TestPatternCandidate_Confidence(t *testing.T) {
	tests := []struct {
		name          string
		pattern       *PatternCandidate
		minConfidence float64
		maxConfidence float64
	}{
		{
			name: "High occurrences, high success rate",
			pattern: &PatternCandidate{
				Occurrences:  20,
				SuccessCount: 18,
				FailureCount: 2,
			},
			minConfidence: 0.8,
			maxConfidence: 1.0,
		},
		{
			name: "Low occurrences, high success rate",
			pattern: &PatternCandidate{
				Occurrences:  3,
				SuccessCount: 3,
				FailureCount: 0,
			},
			minConfidence: 0.5,
			maxConfidence: 0.8,
		},
		{
			name: "High occurrences, low success rate",
			pattern: &PatternCandidate{
				Occurrences:  20,
				SuccessCount: 8,
				FailureCount: 12,
			},
			minConfidence: 0.5,
			maxConfidence: 0.7,
		},
		{
			name: "Low occurrences, low success rate",
			pattern: &PatternCandidate{
				Occurrences:  3,
				SuccessCount: 1,
				FailureCount: 2,
			},
			minConfidence: 0.1,
			maxConfidence: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := tt.pattern.Confidence()
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
			assert.LessOrEqual(t, confidence, tt.maxConfidence)
		})
	}
}

// TestExtractionConfig_Defaults tests default configuration
func TestExtractionConfig_Defaults(t *testing.T) {
	config := DefaultExtractionConfig()

	assert.Equal(t, 0.5, config.MinConfidence)
	assert.Equal(t, 3, config.MinOccurrences)
	assert.Nil(t, config.EmbeddingFunc)
}

// TestFormatIndicatorCondition tests indicator condition formatting
func TestFormatIndicatorCondition(t *testing.T) {
	tests := []struct {
		name      string
		indicator string
		value     interface{}
		expected  string
	}{
		{
			name:      "RSI overbought",
			indicator: "rsi",
			value:     float64(75),
			expected:  "RSI exceeds 70 (overbought)",
		},
		{
			name:      "RSI oversold",
			indicator: "rsi",
			value:     float64(25),
			expected:  "RSI below 30 (oversold)",
		},
		{
			name:      "MACD bullish",
			indicator: "macd",
			value:     float64(0.5),
			expected:  "MACD is positive (bullish)",
		},
		{
			name:      "MACD bearish",
			indicator: "macd",
			value:     float64(-0.5),
			expected:  "MACD is negative (bearish)",
		},
		{
			name:      "Boolean indicator",
			indicator: "trend_up",
			value:     true,
			expected:  "trend_up is true",
		},
		{
			name:      "String indicator",
			indicator: "market_condition",
			value:     "bullish",
			expected:  "market_condition is bullish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatIndicatorCondition(tt.indicator, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateImportance tests importance calculation
func TestCalculateImportance(t *testing.T) {
	tests := []struct {
		name          string
		pattern       *PatternCandidate
		minImportance float64
		maxImportance float64
	}{
		{
			name: "High occurrences, high P&L",
			pattern: &PatternCandidate{
				Occurrences: 30,
				AvgPnL:      150.0,
			},
			minImportance: 0.8,
			maxImportance: 1.0,
		},
		{
			name: "Low occurrences, low P&L",
			pattern: &PatternCandidate{
				Occurrences: 3,
				AvgPnL:      10.0,
			},
			minImportance: 0.1,
			maxImportance: 0.3,
		},
		{
			name: "Medium occurrences, medium P&L",
			pattern: &PatternCandidate{
				Occurrences: 10,
				AvgPnL:      50.0,
			},
			minImportance: 0.4,
			maxImportance: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importance := calculateImportance(tt.pattern)
			assert.GreaterOrEqual(t, importance, tt.minImportance)
			assert.LessOrEqual(t, importance, tt.maxImportance)
		})
	}
}

// TestAppendUnique tests unique string append
func TestAppendUnique(t *testing.T) {
	slice := []string{"a", "b", "c"}

	// Adding new item
	result := appendUnique(slice, "d")
	assert.Len(t, result, 4)
	assert.Contains(t, result, "d")

	// Adding duplicate
	result = appendUnique(slice, "b")
	assert.Len(t, result, 3)
	assert.Equal(t, slice, result)
}

// TestExtractConditions tests condition extraction from decisions
func TestExtractConditions(t *testing.T) {
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	tests := []struct {
		name     string
		decision *db.LLMDecision
		minCount int
	}{
		{
			name: "Decision with RSI indicator",
			decision: &db.LLMDecision{
				Context: []byte(`{"indicators": {"rsi": 75, "macd": 0.5}}`),
			},
			minCount: 2, // Should extract RSI and MACD conditions
		},
		{
			name: "Decision with market condition",
			decision: &db.LLMDecision{
				Context: []byte(`{"market_condition": "bullish"}`),
			},
			minCount: 1,
		},
		{
			name: "Decision with empty context",
			decision: &db.LLMDecision{
				Context: []byte{},
			},
			minCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := extractor.extractConditions(tt.decision)
			assert.GreaterOrEqual(t, len(conditions), tt.minCount)
		})
	}
}

// TestIdentifyPatterns tests pattern identification
func TestIdentifyPatterns(t *testing.T) {
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	pnl1 := 50.0
	pnl2 := 75.0
	pnl3 := -25.0

	successfulDecisions := []*db.LLMDecision{
		{
			ID:        uuid.New(),
			Symbol:    "BTC/USDT",
			AgentName: "technical-agent",
			Context:   []byte(`{"indicators": {"rsi": 75}}`),
			PnL:       &pnl1,
		},
		{
			ID:        uuid.New(),
			Symbol:    "BTC/USDT",
			AgentName: "technical-agent",
			Context:   []byte(`{"indicators": {"rsi": 72}}`),
			PnL:       &pnl2,
		},
	}

	failedDecisions := []*db.LLMDecision{
		{
			ID:        uuid.New(),
			Symbol:    "ETH/USDT",
			AgentName: "technical-agent",
			Context:   []byte(`{"indicators": {"rsi": 78}}`),
			PnL:       &pnl3,
		},
	}

	patterns := extractor.identifyPatterns(successfulDecisions, failedDecisions)

	assert.NotEmpty(t, patterns)

	// Check that patterns have proper structure
	for _, p := range patterns {
		assert.NotEmpty(t, p.Condition)
		assert.NotEmpty(t, p.Outcome)
		assert.Greater(t, p.Occurrences, 0)
	}
}

// TestKnowledgeExtractor_CreateKnowledgeFromPattern tests knowledge creation
func TestKnowledgeExtractor_CreateKnowledgeFromPattern(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	pattern := &PatternCandidate{
		Condition:    "RSI exceeds 70 (overbought)",
		Outcome:      "typically leads to profitable trades",
		Occurrences:  10,
		SuccessCount: 8,
		FailureCount: 2,
		AvgPnL:       45.50,
		Symbols:      []string{"BTC/USDT"},
		AgentNames:   []string{"technical-agent"},
		DecisionIDs:  []uuid.UUID{uuid.New()},
	}

	knowledge, err := extractor.createKnowledgeFromPattern(ctx, pattern, "technical-agent")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.Equal(t, KnowledgePattern, knowledge.Type)
	assert.Contains(t, knowledge.Content, "RSI exceeds 70")
	assert.Contains(t, knowledge.Content, "profitable trades")
	assert.Contains(t, knowledge.Content, "80.0%") // Success rate
	assert.NotEmpty(t, knowledge.Embedding)
	assert.Equal(t, "technical-agent", knowledge.AgentName)
	assert.Equal(t, "pattern_extraction", knowledge.Source)
	assert.Greater(t, knowledge.Confidence, 0.0)
}

// TestKnowledgeExtractor_Integration tests full extraction flow (requires database)
func TestKnowledgeExtractor_ExtractFromLLMDecisions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup")

	// Example of how this test would work:
	/*
		ctx := context.Background()
		database, _ := db.New(ctx)
		defer database.Close()

		config := DefaultExtractionConfig()
		config.EmbeddingFunc = mockEmbeddingFunc
		extractor := NewKnowledgeExtractorFromDB(database, config)

		// Extract knowledge from recent decisions
		since := time.Now().Add(-30 * 24 * time.Hour)
		count, err := extractor.ExtractFromLLMDecisions(ctx, "technical-agent", since)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	*/
}

// Mock embedding function for testing
func mockEmbeddingFunc(ctx context.Context, text string) ([]float32, error) {
	// Generate deterministic fake embedding based on text length
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = float32(len(text)%100) / 100.0
	}
	return embedding, nil
}

// MockKnowledgeExtractor for unit testing without database
type MockKnowledgeExtractor struct {
	ExtractFromLLMDecisionsFunc   func(ctx context.Context, agentName string, since interface{}) (int, error)
	ExtractFromTradingResultsFunc func(ctx context.Context, agentName string, since interface{}) (int, error)
}

func (m *MockKnowledgeExtractor) ExtractFromLLMDecisions(ctx context.Context, agentName string, since interface{}) (int, error) {
	if m.ExtractFromLLMDecisionsFunc != nil {
		return m.ExtractFromLLMDecisionsFunc(ctx, agentName, since)
	}
	return 5, nil // Return 5 extracted knowledge items
}

func (m *MockKnowledgeExtractor) ExtractFromTradingResults(ctx context.Context, agentName string, since interface{}) (int, error) {
	if m.ExtractFromTradingResultsFunc != nil {
		return m.ExtractFromTradingResultsFunc(ctx, agentName, since)
	}
	return 3, nil // Return 3 extracted experiences
}

// Example of using MockKnowledgeExtractor
func TestMockKnowledgeExtractor(t *testing.T) {
	mock := &MockKnowledgeExtractor{
		ExtractFromLLMDecisionsFunc: func(ctx context.Context, agentName string, since interface{}) (int, error) {
			assert.Equal(t, "technical-agent", agentName)
			return 10, nil
		},
	}

	ctx := context.Background()
	count, err := mock.ExtractFromLLMDecisions(ctx, "technical-agent", nil)

	require.NoError(t, err)
	assert.Equal(t, 10, count)
}
