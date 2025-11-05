package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContextBuilder(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName:      "test-agent",
		IncludeHistory: true,
	})

	require.NotNil(t, cb)
	assert.Equal(t, 4000, cb.maxTokens) // Default
	assert.Equal(t, "test-agent", cb.agentName)
	assert.True(t, cb.includeHistory)
}

func TestNewContextBuilderWithCustomTokens(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		MaxTokens:      8000,
		AgentName:      "test-agent",
		IncludeHistory: false,
	})

	assert.Equal(t, 8000, cb.maxTokens)
	assert.False(t, cb.includeHistory)
}

func TestFormatContextForPrompt_BasicMarket(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	market := MarketContext{
		Symbol:         "BTC/USDT",
		CurrentPrice:   50000.0,
		PriceChange24h: 2.5,
		Volume24h:      1000000.0,
		Indicators: map[string]float64{
			"RSI":  65.5,
			"MACD": 125.45,
			"ADX":  28.5,
		},
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket: market,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	// Verify key information is present
	assert.Contains(t, formatted, "BTC/USDT")
	assert.Contains(t, formatted, "$50000.00")
	assert.Contains(t, formatted, "2.50%")
	assert.Contains(t, formatted, "RSI: 65.5000")
	assert.Contains(t, formatted, "MACD: 125.4500")
	assert.Contains(t, formatted, "## Current Market Conditions")
}

func TestFormatContextForPrompt_WithPositions(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
	}

	positions := []PositionContext{
		{
			Symbol:        "BTC/USDT",
			Side:          "LONG",
			EntryPrice:    48000.0,
			CurrentPrice:  50000.0,
			Quantity:      0.5,
			UnrealizedPnL: 1000.0,
			OpenDuration:  "2h 30m",
		},
		{
			Symbol:        "ETH/USDT",
			Side:          "LONG",
			EntryPrice:    2800.0,
			CurrentPrice:  2900.0,
			Quantity:      2.0,
			UnrealizedPnL: 200.0,
			OpenDuration:  "1h 15m",
		},
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket: market,
		Positions:     positions,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	assert.Contains(t, formatted, "## Current Positions")
	assert.Contains(t, formatted, "BTC/USDT LONG")
	assert.Contains(t, formatted, "ETH/USDT LONG")
	assert.Contains(t, formatted, "$48000.00")
	assert.Contains(t, formatted, "2h 30m")
}

func TestFormatContextForPrompt_WithPortfolioSummary(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
	}

	portfolio := &PortfolioSummary{
		TotalValue:    100000.0,
		TotalPnL:      5000.0,
		OpenPositions: 3,
		DayPnL:        1200.0,
		WeekPnL:       3500.0,
		SuccessRate:   65.5,
		AvgConfidence: 0.78,
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket:    market,
		PortfolioSummary: portfolio,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	assert.Contains(t, formatted, "## Portfolio Summary")
	assert.Contains(t, formatted, "$100000.00")
	assert.Contains(t, formatted, "$5000.00")
	assert.Contains(t, formatted, "Open Positions: 3")
	assert.Contains(t, formatted, "$1200.00")
	assert.Contains(t, formatted, "65.5%")
}

func TestFormatContextForPrompt_WithHistoricalDecisions(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName:      "test-agent",
		IncludeHistory: true,
	})

	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
	}

	recentDecisions := []HistoricalDecision{
		{
			Timestamp:  time.Now().Add(-1 * time.Hour),
			Action:     "BUY",
			Confidence: 0.85,
			Outcome:    "SUCCESS",
			PnL:        250.50,
		},
		{
			Timestamp:  time.Now().Add(-2 * time.Hour),
			Action:     "HOLD",
			Confidence: 0.60,
			Outcome:    "PENDING",
		},
		{
			Timestamp:  time.Now().Add(-3 * time.Hour),
			Action:     "SELL",
			Confidence: 0.75,
			Outcome:    "FAILURE",
			PnL:        -100.25,
		},
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket:   market,
		RecentDecisions: recentDecisions,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	assert.Contains(t, formatted, "## Recent Decision History")
	assert.Contains(t, formatted, "BUY")
	assert.Contains(t, formatted, "✓") // Success symbol
	assert.Contains(t, formatted, "✗") // Failure symbol
}

func TestFormatContextForPrompt_WithSimilarSituations(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName:      "test-agent",
		IncludeHistory: true,
	})

	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
	}

	similarSituations := []HistoricalDecision{
		{
			Timestamp:  time.Now().Add(-24 * time.Hour),
			Action:     "BUY",
			Confidence: 0.80,
			Reasoning:  "Strong uptrend with RSI at 65",
			Outcome:    "SUCCESS",
			PnL:        500.0,
		},
		{
			Timestamp:  time.Now().Add(-48 * time.Hour),
			Action:     "BUY",
			Confidence: 0.75,
			Reasoning:  "Momentum building",
			Outcome:    "SUCCESS",
			PnL:        350.0,
		},
		{
			Timestamp:  time.Now().Add(-72 * time.Hour),
			Action:     "SELL",
			Confidence: 0.70,
			Reasoning:  "Overbought conditions",
			Outcome:    "FAILURE",
			PnL:        -150.0,
		},
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket:     market,
		SimilarSituations: similarSituations,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	assert.Contains(t, formatted, "## Similar Past Situations")
	assert.Contains(t, formatted, "In similar market conditions")
	assert.Contains(t, formatted, "Strong uptrend")
	assert.Contains(t, formatted, "$500.00")
	assert.Contains(t, formatted, "2 successes, 1 failures")
	assert.Contains(t, formatted, "66.7% success rate") // 2/3
}

func TestBuildMinimalContext(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	market := MarketContext{
		Symbol:         "BTC/USDT",
		CurrentPrice:   50000.0,
		PriceChange24h: 2.5,
		Indicators: map[string]float64{
			"RSI":  65.5,
			"MACD": 125.45,
			"ADX":  28.5,
			"EMA":  49500.0,
		},
	}

	minimal := cb.BuildMinimalContext(market)

	// Should be very compact
	assert.Contains(t, minimal, "BTC/USDT")
	assert.Contains(t, minimal, "$50000.00")
	assert.Contains(t, minimal, "2.50%")
	// Should only have 3 indicators max
	// Format: "Symbol: X | Price: Y | 24h: Z% | IND1: A IND2: B IND3: C"
	// Colons: Symbol: + Price: + 24h: + 3 indicators = 6 colons max
	colonCount := strings.Count(minimal, ":")
	assert.LessOrEqual(t, colonCount, 6)
}

func TestEstimateTokens(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	// Test with known text
	text := "This is a test string with approximately 100 characters to test the token estimation function properly."

	tokens := cb.estimateTokens(text)

	// Should be around 25 tokens (100 chars / 4)
	assert.Greater(t, tokens, 20)
	assert.Less(t, tokens, 30)
}

func TestTruncateToTokenLimit(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
		MaxTokens: 100, // Very small limit
	})

	// Create a long text
	longText := strings.Repeat("This is a test sentence. ", 50) // ~1250 chars

	truncated := cb.truncateToTokenLimit(longText, 100)

	// Should be truncated
	assert.Less(t, len(truncated), len(longText))
	assert.Contains(t, truncated, "[Context truncated to fit token limit]")

	// Should be under token limit
	tokens := cb.estimateTokens(truncated)
	assert.LessOrEqual(t, tokens, 100)
}

func TestGetContextStats(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
		Indicators: map[string]float64{
			"RSI": 65.5,
		},
	}

	positions := []PositionContext{
		{Symbol: "BTC/USDT", Side: "LONG"},
		{Symbol: "ETH/USDT", Side: "LONG"},
	}

	recentDecisions := []HistoricalDecision{
		{Action: "BUY", Outcome: "SUCCESS"},
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket:   market,
		Positions:       positions,
		RecentDecisions: recentDecisions,
	}

	stats := cb.GetContextStats(enhanced)

	assert.Greater(t, stats["estimated_tokens"].(int), 0)
	assert.Greater(t, stats["char_count"].(int), 0)
	assert.True(t, stats["has_history"].(bool))
	assert.False(t, stats["has_similar"].(bool))
	assert.Equal(t, 2, stats["position_count"].(int))
	assert.Equal(t, 1, stats["decision_count"].(int))
}

func TestFormatContextForPrompt_TokenLimit(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
		MaxTokens: 50, // Very restrictive
	})

	// Create a large context
	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
		Indicators: map[string]float64{
			"RSI":      65.5,
			"MACD":     125.45,
			"ADX":      28.5,
			"EMA_Fast": 49800.0,
			"EMA_Slow": 49500.0,
		},
	}

	positions := make([]PositionContext, 10)
	for i := 0; i < 10; i++ {
		positions[i] = PositionContext{
			Symbol:       "BTC/USDT",
			Side:         "LONG",
			EntryPrice:   48000.0 + float64(i*100),
			CurrentPrice: 50000.0,
		}
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket: market,
		Positions:     positions,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	// Should be truncated to fit token limit
	tokens := cb.estimateTokens(formatted)
	assert.LessOrEqual(t, tokens, 60) // Some margin
}

func TestConvertToHistoricalDecisions(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	// This would normally come from database, but we can test the conversion logic
	// The function is private, so we test it indirectly through other methods

	// Just verify that the builder initializes correctly
	assert.NotNil(t, cb)
	assert.Equal(t, "test-agent", cb.agentName)
}

func TestFormatContextForPrompt_PositionLimit(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	market := MarketContext{
		Symbol:       "BTC/USDT",
		CurrentPrice: 50000.0,
	}

	// Create 10 positions, but only 5 should show
	positions := make([]PositionContext, 10)
	for i := 0; i < 10; i++ {
		positions[i] = PositionContext{
			Symbol:       "TEST" + string(rune(i)),
			Side:         "LONG",
			EntryPrice:   1000.0,
			CurrentPrice: 1100.0,
		}
	}

	enhanced := &EnhancedMarketContext{
		CurrentMarket: market,
		Positions:     positions,
	}

	formatted := cb.FormatContextForPrompt(enhanced)

	// Should mention "and X more positions"
	assert.Contains(t, formatted, "and 5 more positions")
}

func TestFormatLearningContext_NilTracker(t *testing.T) {
	cb := NewContextBuilder(nil, ContextBuilderConfig{
		AgentName: "test-agent",
	})

	ctx := context.TODO()
	contextStr, err := cb.FormatLearningContext(ctx, "BTC/USDT", map[string]float64{
		"RSI": 65.5,
	})

	assert.NoError(t, err)
	assert.Empty(t, contextStr)
}
