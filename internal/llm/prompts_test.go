package llm

import (
	"strings"
	"testing"
	"time"
)

func TestPromptBuilder_GetSystemPrompt(t *testing.T) {
	tests := []struct {
		name          string
		agentType     AgentType
		wantSubstring string
	}{
		{
			name:          "Technical Analysis Agent",
			agentType:     AgentTypeTechnical,
			wantSubstring: "technical analysis",
		},
		{
			name:          "Trend Following Agent",
			agentType:     AgentTypeTrend,
			wantSubstring: "trend-following",
		},
		{
			name:          "Mean Reversion Agent",
			agentType:     AgentTypeReversion,
			wantSubstring: "mean reversion",
		},
		{
			name:          "Risk Management Agent",
			agentType:     AgentTypeRisk,
			wantSubstring: "risk management",
		},
		{
			name:          "Orderbook Analysis Agent",
			agentType:     AgentTypeOrderbook,
			wantSubstring: "order book",
		},
		{
			name:          "Sentiment Analysis Agent",
			agentType:     AgentTypeSentiment,
			wantSubstring: "sentiment",
		},
		{
			name:          "Default Agent",
			agentType:     "unknown",
			wantSubstring: "trading agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewPromptBuilder(tt.agentType)
			prompt := pb.GetSystemPrompt()

			if prompt == "" {
				t.Error("Expected non-empty system prompt")
			}

			if !strings.Contains(strings.ToLower(prompt), tt.wantSubstring) {
				t.Errorf("Expected system prompt to contain %q, got: %s", tt.wantSubstring, prompt)
			}

			// All prompts should instruct JSON-only responses
			if !strings.Contains(prompt, "JSON") {
				t.Error("Expected system prompt to mention JSON format requirement")
			}
		})
	}
}

func TestPromptBuilder_BuildTechnicalAnalysisPrompt(t *testing.T) {
	pb := NewPromptBuilder(AgentTypeTechnical)

	ctx := MarketContext{
		Symbol:         "BTC/USDT",
		CurrentPrice:   45000.50,
		PriceChange24h: 2.5,
		Volume24h:      1234567890.50,
		Indicators: map[string]float64{
			"RSI":      65.5,
			"MACD":     125.3,
			"BB_Upper": 46500.0,
			"BB_Lower": 43500.0,
			"SMA_20":   44800.0,
		},
	}

	prompt := pb.BuildTechnicalAnalysisPrompt(ctx)

	// Check that all context is included
	if !strings.Contains(prompt, "BTC/USDT") {
		t.Error("Expected prompt to contain symbol")
	}
	if !strings.Contains(prompt, "45000.50") {
		t.Error("Expected prompt to contain current price")
	}
	if !strings.Contains(prompt, "2.5") {
		t.Error("Expected prompt to contain price change")
	}

	// Check that indicators are formatted
	if !strings.Contains(prompt, "RSI") {
		t.Error("Expected prompt to contain RSI indicator")
	}

	// Check JSON format instruction
	if !strings.Contains(prompt, `"action"`) {
		t.Error("Expected prompt to specify action field in JSON format")
	}
	if !strings.Contains(prompt, `"confidence"`) {
		t.Error("Expected prompt to specify confidence field in JSON format")
	}

	// Check that indicators appear in sorted order (deterministic)
	rsiIdx := strings.Index(prompt, "RSI:")
	smaIdx := strings.Index(prompt, "SMA_20:")
	if rsiIdx > smaIdx {
		t.Error("Expected indicators to be sorted alphabetically (RSI before SMA_20)")
	}
}

func TestPromptBuilder_BuildTrendFollowingPrompt(t *testing.T) {
	pb := NewPromptBuilder(AgentTypeTrend)

	ctx := MarketContext{
		Symbol:         "ETH/USDT",
		CurrentPrice:   3200.75,
		PriceChange24h: 5.2,
		Indicators: map[string]float64{
			"EMA_12": 3180.0,
			"EMA_26": 3150.0,
			"ADX":    35.5,
			"Trend":  1.0,
		},
	}

	historicalDecisions := []HistoricalDecision{
		{
			Action:     "BUY",
			Confidence: 0.85,
			Reasoning:  "Strong uptrend confirmed",
			Outcome:    "SUCCESS",
			PnL:        250.50,
			Timestamp:  time.Now().Add(-24 * time.Hour),
		},
	}

	prompt := pb.BuildTrendFollowingPrompt(ctx, historicalDecisions)

	// Check context
	if !strings.Contains(prompt, "ETH/USDT") {
		t.Error("Expected prompt to contain symbol")
	}

	// Check historical decisions are included
	if !strings.Contains(prompt, "Recent Similar Decisions") {
		t.Error("Expected prompt to include historical decisions section")
	}
	if !strings.Contains(prompt, "Strong uptrend confirmed") {
		t.Error("Expected prompt to include historical reasoning")
	}

	// Check trend-specific fields
	if !strings.Contains(prompt, `"trend_strength"`) {
		t.Error("Expected prompt to specify trend_strength field")
	}
	if !strings.Contains(prompt, `"trend_direction"`) {
		t.Error("Expected prompt to specify trend_direction field")
	}
}

func TestPromptBuilder_BuildMeanReversionPrompt(t *testing.T) {
	pb := NewPromptBuilder(AgentTypeReversion)

	ctx := MarketContext{
		Symbol:         "SOL/USDT",
		CurrentPrice:   125.30,
		PriceChange24h: -8.5,
		Indicators: map[string]float64{
			"RSI":         28.5, // Oversold
			"BB_Position": -2.1, // Below lower band
			"SMA_50":      135.0,
		},
	}

	positions := []PositionContext{
		{
			Symbol:        "SOL/USDT",
			Side:          "LONG",
			EntryPrice:    130.0,
			CurrentPrice:  125.30,
			Quantity:      10.0,
			UnrealizedPnL: -47.0,
			OpenDuration:  "2h 15m",
		},
	}

	prompt := pb.BuildMeanReversionPrompt(ctx, positions)

	// Check context
	if !strings.Contains(prompt, "SOL/USDT") {
		t.Error("Expected prompt to contain symbol")
	}

	// Check positions are included
	if !strings.Contains(prompt, "Current Positions") {
		t.Error("Expected prompt to include positions section")
	}
	if !strings.Contains(prompt, "Entry: $130.00") {
		t.Error("Expected prompt to include position entry price")
	}

	// Check mean reversion specific fields
	if !strings.Contains(prompt, `"deviation_from_mean"`) {
		t.Error("Expected prompt to specify deviation_from_mean field")
	}
	if !strings.Contains(prompt, `"reversion_likelihood"`) {
		t.Error("Expected prompt to specify reversion_likelihood field")
	}
}

func TestPromptBuilder_BuildRiskAssessmentPrompt(t *testing.T) {
	pb := NewPromptBuilder(AgentTypeRisk)

	signal := Signal{
		Symbol:     "BTC/USDT",
		Side:       "BUY",
		Confidence: 0.75,
		Reasoning:  "Bullish reversal pattern forming",
	}

	ctx := MarketContext{
		Symbol:         "BTC/USDT",
		CurrentPrice:   45000.0,
		PriceChange24h: 3.2,
	}

	positions := []PositionContext{
		{
			Symbol:        "ETH/USDT",
			Side:          "LONG",
			EntryPrice:    3200.0,
			CurrentPrice:  3250.0,
			Quantity:      5.0,
			UnrealizedPnL: 250.0,
			OpenDuration:  "1d 3h",
		},
	}

	portfolioValue := 100000.0
	maxPositionSize := 0.1 // 10%

	prompt := pb.BuildRiskAssessmentPrompt(signal, ctx, positions, portfolioValue, maxPositionSize)

	// Check signal details
	if !strings.Contains(prompt, "BTC/USDT") {
		t.Error("Expected prompt to contain signal symbol")
	}
	if !strings.Contains(prompt, "BUY") {
		t.Error("Expected prompt to contain signal side")
	}
	if !strings.Contains(prompt, "Bullish reversal pattern") {
		t.Error("Expected prompt to contain signal reasoning")
	}

	// Check portfolio details
	if !strings.Contains(prompt, "$100000.00") {
		t.Error("Expected prompt to contain portfolio value")
	}
	if !strings.Contains(prompt, "10.00%") {
		t.Error("Expected prompt to contain max position size percentage")
	}

	// Check risk-specific fields
	if !strings.Contains(prompt, `"approved"`) {
		t.Error("Expected prompt to specify approved field")
	}
	if !strings.Contains(prompt, `"position_size"`) {
		t.Error("Expected prompt to specify position_size field")
	}
	if !strings.Contains(prompt, `"risk_score"`) {
		t.Error("Expected prompt to specify risk_score field")
	}
}

func TestFormatIndicators(t *testing.T) {
	tests := []struct {
		name       string
		indicators map[string]float64
		wantCount  int
		checkOrder bool
	}{
		{
			name:       "Empty indicators",
			indicators: map[string]float64{},
			wantCount:  0,
			checkOrder: false,
		},
		{
			name: "Single indicator",
			indicators: map[string]float64{
				"RSI": 65.5,
			},
			wantCount:  1,
			checkOrder: false,
		},
		{
			name: "Multiple indicators - sorted",
			indicators: map[string]float64{
				"RSI":    65.5,
				"MACD":   125.3,
				"ADX":    35.2,
				"SMA_20": 44800.0,
			},
			wantCount:  4,
			checkOrder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatIndicators(tt.indicators)

			if tt.wantCount == 0 {
				if result != "No indicators available" {
					t.Errorf("Expected 'No indicators available', got: %s", result)
				}
				return
			}

			lines := strings.Split(result, "\n")
			if len(lines) != tt.wantCount {
				t.Errorf("Expected %d lines, got %d", tt.wantCount, len(lines))
			}

			if tt.checkOrder {
				// Verify alphabetical order
				if !strings.Contains(lines[0], "ADX:") {
					t.Error("Expected ADX to be first (alphabetically)")
				}
				if !strings.Contains(lines[1], "MACD:") {
					t.Error("Expected MACD to be second")
				}
				if !strings.Contains(lines[2], "RSI:") {
					t.Error("Expected RSI to be third")
				}
				if !strings.Contains(lines[3], "SMA_20:") {
					t.Error("Expected SMA_20 to be fourth")
				}
			}
		})
	}
}

func TestFormatPositions(t *testing.T) {
	tests := []struct {
		name          string
		positions     []PositionContext
		wantSubstring string
	}{
		{
			name:          "Empty positions",
			positions:     []PositionContext{},
			wantSubstring: "No open positions",
		},
		{
			name: "Long position with profit",
			positions: []PositionContext{
				{
					Symbol:        "BTC/USDT",
					Side:          "LONG",
					EntryPrice:    44000.0,
					CurrentPrice:  45000.0,
					Quantity:      1.0,
					UnrealizedPnL: 1000.0,
					OpenDuration:  "2h 30m",
				},
			},
			wantSubstring: "BTC/USDT LONG",
		},
		{
			name: "Short position with loss",
			positions: []PositionContext{
				{
					Symbol:        "ETH/USDT",
					Side:          "SHORT",
					EntryPrice:    3200.0,
					CurrentPrice:  3300.0,
					Quantity:      5.0,
					UnrealizedPnL: -500.0,
					OpenDuration:  "1d 5h",
				},
			},
			wantSubstring: "ETH/USDT SHORT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPositions(tt.positions)

			if !strings.Contains(result, tt.wantSubstring) {
				t.Errorf("Expected result to contain %q, got: %s", tt.wantSubstring, result)
			}

			// For non-empty positions, check that key info is included
			if len(tt.positions) > 0 {
				if !strings.Contains(result, "Entry:") {
					t.Error("Expected result to contain entry price")
				}
				if !strings.Contains(result, "Current:") {
					t.Error("Expected result to contain current price")
				}
				if !strings.Contains(result, "P&L") {
					t.Error("Expected result to contain P&L")
				}
			}
		})
	}
}

func TestFormatHistoricalDecisions(t *testing.T) {
	tests := []struct {
		name      string
		decisions []HistoricalDecision
		wantLines int
	}{
		{
			name:      "Empty decisions",
			decisions: []HistoricalDecision{},
			wantLines: 0,
		},
		{
			name: "Single decision",
			decisions: []HistoricalDecision{
				{
					Action:     "BUY",
					Confidence: 0.85,
					Reasoning:  "Strong bullish momentum",
					Outcome:    "SUCCESS",
					PnL:        500.0,
					Timestamp:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				},
			},
			wantLines: 1,
		},
		{
			name: "Multiple decisions (limited to 5)",
			decisions: []HistoricalDecision{
				{Action: "BUY", Confidence: 0.8, Reasoning: "Test 1", Outcome: "SUCCESS", PnL: 100, Timestamp: time.Now()},
				{Action: "SELL", Confidence: 0.7, Reasoning: "Test 2", Outcome: "SUCCESS", PnL: 200, Timestamp: time.Now()},
				{Action: "BUY", Confidence: 0.9, Reasoning: "Test 3", Outcome: "FAILURE", PnL: -50, Timestamp: time.Now()},
				{Action: "HOLD", Confidence: 0.6, Reasoning: "Test 4", Outcome: "SUCCESS", PnL: 0, Timestamp: time.Now()},
				{Action: "BUY", Confidence: 0.85, Reasoning: "Test 5", Outcome: "SUCCESS", PnL: 300, Timestamp: time.Now()},
				{Action: "SELL", Confidence: 0.75, Reasoning: "Test 6", Outcome: "SUCCESS", PnL: 150, Timestamp: time.Now()},
			},
			wantLines: 5, // Should be limited to 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatHistoricalDecisions(tt.decisions)

			if tt.wantLines == 0 {
				if result != "" {
					t.Errorf("Expected empty string for no decisions, got: %s", result)
				}
				return
			}

			// Count decision blocks
			decisionCount := strings.Count(result, "Decision ")
			if decisionCount != tt.wantLines {
				t.Errorf("Expected %d decisions in output, got %d", tt.wantLines, decisionCount)
			}

			// Check that key fields are present
			if !strings.Contains(result, "Action:") {
				t.Error("Expected result to contain Action field")
			}
			if !strings.Contains(result, "Confidence:") {
				t.Error("Expected result to contain Confidence field")
			}
			if !strings.Contains(result, "Outcome:") {
				t.Error("Expected result to contain Outcome field")
			}
		})
	}
}

func TestFormatContextAsJSON(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		wantValid bool
	}{
		{
			name: "Simple struct",
			data: struct {
				Symbol string
				Price  float64
			}{
				Symbol: "BTC/USDT",
				Price:  45000.0,
			},
			wantValid: true,
		},
		{
			name: "Map",
			data: map[string]interface{}{
				"rsi":  65.5,
				"macd": 125.3,
			},
			wantValid: true,
		},
		{
			name:      "Nil",
			data:      nil,
			wantValid: true, // Should return "null"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatContextAsJSON(tt.data)

			if result == "" {
				t.Error("Expected non-empty JSON string")
			}

			// Should start with { or [ or null
			if !strings.HasPrefix(result, "{") && !strings.HasPrefix(result, "[") && !strings.HasPrefix(result, "null") {
				t.Errorf("Expected valid JSON start, got: %s", result[:10])
			}
		})
	}
}
