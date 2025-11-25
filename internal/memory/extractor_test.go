package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
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

// TestNewKnowledgeExtractor tests extractor creation with defaults
func TestNewKnowledgeExtractor(t *testing.T) {
	config := ExtractionConfig{
		MinConfidence:  0.0, // Should default to 0.5
		MinOccurrences: 0,   // Should default to 3
		EmbeddingFunc:  mockEmbeddingFunc,
	}

	extractor := NewKnowledgeExtractor(nil, config)

	assert.NotNil(t, extractor)
	assert.Equal(t, 0.5, extractor.minConfidence)
	assert.Equal(t, 3, extractor.minOccurrences)
	assert.NotNil(t, extractor.embeddingFunc)
}

// TestNewKnowledgeExtractor_CustomConfig tests extractor with custom config
func TestNewKnowledgeExtractor_CustomConfig(t *testing.T) {
	config := ExtractionConfig{
		MinConfidence:  0.7,
		MinOccurrences: 5,
		EmbeddingFunc:  mockEmbeddingFunc,
	}

	extractor := NewKnowledgeExtractor(nil, config)

	assert.NotNil(t, extractor)
	assert.Equal(t, 0.7, extractor.minConfidence)
	assert.Equal(t, 5, extractor.minOccurrences)
}

// TestCreateKnowledgeFromExperience tests experience knowledge creation
func TestCreateKnowledgeFromExperience(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	exp := &Experience{
		Description: "Stop losses at 2% work better than 5% for BTC",
		SuccessRate: 0.85,
		AvgPnL:      125.50,
		Occurrences: 20,
		Symbol:      "BTC/USDT",
	}

	knowledge, err := extractor.createKnowledgeFromExperience(ctx, exp, "trend-agent")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.Equal(t, KnowledgeExperience, knowledge.Type)
	assert.Equal(t, exp.Description, knowledge.Content)
	assert.Equal(t, exp.SuccessRate, knowledge.Confidence)
	assert.Equal(t, 0.7, knowledge.Importance)
	assert.Equal(t, "trading_results", knowledge.Source)
	assert.Equal(t, "trend-agent", knowledge.AgentName)
	assert.NotEmpty(t, knowledge.Embedding)
}

// TestCreateKnowledgeFromFact tests fact knowledge creation
func TestCreateKnowledgeFromFact(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	fact := &Fact{
		Statement:  "BTC volatility increases by 20% during US trading hours",
		Confidence: 0.9,
		Source:     "market_data_analysis",
	}

	knowledge, err := extractor.createKnowledgeFromFact(ctx, fact, "BTC/USDT")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.Equal(t, KnowledgeFact, knowledge.Type)
	assert.Equal(t, fact.Statement, knowledge.Content)
	assert.Equal(t, fact.Confidence, knowledge.Confidence)
	assert.Equal(t, 0.6, knowledge.Importance)
	assert.Equal(t, "market_data_analysis", knowledge.Source)
	assert.NotNil(t, knowledge.Symbol)
	assert.Equal(t, "BTC/USDT", *knowledge.Symbol)
	assert.NotEmpty(t, knowledge.Embedding)
}

// TestCreateKnowledgeFromExperience_NoEmbeddingFunc tests without embedding function
func TestCreateKnowledgeFromExperience_NoEmbeddingFunc(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = nil // No embedding function
	extractor := NewKnowledgeExtractor(nil, config)

	exp := &Experience{
		Description: "Test experience",
		SuccessRate: 0.8,
		AvgPnL:      50.0,
		Occurrences: 10,
		Symbol:      "ETH/USDT",
	}

	knowledge, err := extractor.createKnowledgeFromExperience(ctx, exp, "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.Nil(t, knowledge.Embedding) // Should be nil when no embedding func
}

// TestCreateKnowledgeFromFact_NoEmbeddingFunc tests fact creation without embedding
func TestCreateKnowledgeFromFact_NoEmbeddingFunc(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = nil
	extractor := NewKnowledgeExtractor(nil, config)

	fact := &Fact{
		Statement:  "Test fact",
		Confidence: 0.85,
		Source:     "test",
	}

	knowledge, err := extractor.createKnowledgeFromFact(ctx, fact, "BTC/USDT")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.Nil(t, knowledge.Embedding)
}

// TestExtractExperiences tests experience extraction (currently placeholder)
func TestExtractExperiences(t *testing.T) {
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	// Currently returns empty slice
	experiences := extractor.extractExperiences(nil)

	assert.NotNil(t, experiences)
	assert.Empty(t, experiences) // Placeholder implementation
}

// TestAnalyzeVolatilityPatterns tests volatility pattern analysis (placeholder)
func TestAnalyzeVolatilityPatterns(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	facts := extractor.analyzeVolatilityPatterns(ctx, "BTC/USDT", time.Now())

	assert.NotNil(t, facts)
	assert.Empty(t, facts) // Placeholder implementation
}

// TestAnalyzeVolumePatterns tests volume pattern analysis (placeholder)
func TestAnalyzeVolumePatterns(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	facts := extractor.analyzeVolumePatterns(ctx, "ETH/USDT", time.Now())

	assert.NotNil(t, facts)
	assert.Empty(t, facts) // Placeholder implementation
}

// TestFormatIndicatorCondition_EdgeCases tests edge cases
func TestFormatIndicatorCondition_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		indicator string
		value     interface{}
		expected  string
	}{
		{
			name:      "RSI above 70",
			indicator: "rsi",
			value:     float64(75),
			expected:  "RSI exceeds 70 (overbought)",
		},
		{
			name:      "RSI below 30",
			indicator: "rsi",
			value:     float64(25),
			expected:  "RSI below 30 (oversold)",
		},
		{
			name:      "RSI middle range",
			indicator: "rsi",
			value:     float64(50),
			expected:  "rsi is 50.00",
		},
		{
			name:      "MACD positive",
			indicator: "macd",
			value:     float64(0.5),
			expected:  "MACD is positive (bullish)",
		},
		{
			name:      "MACD negative",
			indicator: "macd",
			value:     float64(-0.5),
			expected:  "MACD is negative (bearish)",
		},
		{
			name:      "MACD exactly 0",
			indicator: "macd",
			value:     float64(0),
			expected:  "macd is 0.00",
		},
		{
			name:      "Unknown indicator",
			indicator: "unknown",
			value:     float64(42.5),
			expected:  "unknown is 42.50",
		},
		{
			name:      "Nil value",
			indicator: "test",
			value:     nil,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatIndicatorCondition(tt.indicator, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractConditions_InvalidJSON tests handling of invalid JSON
func TestExtractConditions_InvalidJSON(t *testing.T) {
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	decision := &db.LLMDecision{
		Context: []byte(`{invalid json`),
	}

	conditions := extractor.extractConditions(decision)

	// Should return empty slice (not nil) on parse error
	assert.Empty(t, conditions)
}

// TestCreateKnowledgeFromPattern_MultipleSymbols tests pattern with multiple symbols
func TestCreateKnowledgeFromPattern_MultipleSymbols(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	pattern := &PatternCandidate{
		Condition:    "MACD is positive (bullish)",
		Outcome:      "typically leads to profitable trades",
		Occurrences:  15,
		SuccessCount: 12,
		FailureCount: 3,
		AvgPnL:       67.25,
		Symbols:      []string{"BTC/USDT", "ETH/USDT", "SOL/USDT"},
		AgentNames:   []string{"technical-agent"},
		DecisionIDs:  []uuid.UUID{uuid.New(), uuid.New()},
	}

	knowledge, err := extractor.createKnowledgeFromPattern(ctx, pattern, "technical-agent")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.Nil(t, knowledge.Symbol) // Should be nil for multi-symbol patterns
	assert.Contains(t, knowledge.Content, "MACD is positive")
	assert.Contains(t, knowledge.Content, "15 times")
	assert.Contains(t, knowledge.Content, "80.0%") // 12/15 success rate
}

// TestCreateKnowledgeFromPattern_SingleSymbol tests pattern with single symbol
func TestCreateKnowledgeFromPattern_SingleSymbol(t *testing.T) {
	ctx := context.Background()
	config := DefaultExtractionConfig()
	config.EmbeddingFunc = mockEmbeddingFunc
	extractor := NewKnowledgeExtractor(nil, config)

	pattern := &PatternCandidate{
		Condition:    "RSI below 30 (oversold)",
		Outcome:      "typically leads to profitable trades",
		Occurrences:  8,
		SuccessCount: 7,
		FailureCount: 1,
		AvgPnL:       92.50,
		Symbols:      []string{"BTC/USDT"},
		AgentNames:   []string{"technical-agent"},
		DecisionIDs:  []uuid.UUID{uuid.New()},
	}

	knowledge, err := extractor.createKnowledgeFromPattern(ctx, pattern, "technical-agent")

	require.NoError(t, err)
	assert.NotNil(t, knowledge)
	assert.NotNil(t, knowledge.Symbol)
	assert.Equal(t, "BTC/USDT", *knowledge.Symbol)
}
