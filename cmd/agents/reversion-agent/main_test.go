package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// BeliefBase Tests
// ============================================================================

func TestBeliefBase_UpdateAndRetrieve(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("test_belief", "test_value", 0.8, "test_source")

	belief, exists := bb.GetBelief("test_belief")
	require.True(t, exists, "Belief should exist")

	assert.Equal(t, "test_belief", belief.Key)
	assert.Equal(t, "test_value", belief.Value)
	assert.Equal(t, 0.8, belief.Confidence)
	assert.Equal(t, "test_source", belief.Source)
	assert.False(t, belief.Timestamp.IsZero(), "Timestamp should be set")
}

func TestBeliefBase_GetNonExistent(t *testing.T) {
	bb := NewBeliefBase()

	belief, exists := bb.GetBelief("nonexistent")
	assert.False(t, exists, "Belief should not exist")
	assert.Nil(t, belief, "Belief should be nil")
}

func TestBeliefBase_UpdateExisting(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("test_key", "value1", 0.7, "source1")
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference
	bb.UpdateBelief("test_key", "value2", 0.9, "source2")

	belief, exists := bb.GetBelief("test_key")
	require.True(t, exists)

	assert.Equal(t, "value2", belief.Value, "Value should be updated")
	assert.Equal(t, 0.9, belief.Confidence, "Confidence should be updated")
	assert.Equal(t, "source2", belief.Source, "Source should be updated")
}

func TestBeliefBase_GetAllBeliefs(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("belief1", "value1", 0.8, "source1")
	bb.UpdateBelief("belief2", "value2", 0.6, "source2")
	bb.UpdateBelief("belief3", "value3", 0.4, "source3")

	beliefs := bb.GetAllBeliefs()

	assert.Len(t, beliefs, 3, "Should have 3 beliefs")
	assert.Contains(t, beliefs, "belief1")
	assert.Contains(t, beliefs, "belief2")
	assert.Contains(t, beliefs, "belief3")

	// Verify returned map is a copy (not a reference)
	beliefs["new_belief"] = &Belief{Key: "new_belief", Value: "new_value", Confidence: 0.5, Source: "test"}
	updatedBeliefs := bb.GetAllBeliefs()
	assert.Len(t, updatedBeliefs, 3, "Original belief base should not be modified")
}

func TestBeliefBase_GetConfidence(t *testing.T) {
	bb := NewBeliefBase()

	// Empty belief base should return 0
	conf := bb.GetConfidence()
	assert.Equal(t, 0.0, conf, "Empty belief base should return 0 confidence")

	// Add beliefs
	bb.UpdateBelief("belief1", "value1", 0.8, "source1")
	bb.UpdateBelief("belief2", "value2", 0.6, "source2")
	bb.UpdateBelief("belief3", "value3", 0.4, "source3")

	// Should return average: (0.8 + 0.6 + 0.4) / 3 = 0.6
	conf = bb.GetConfidence()
	assert.InDelta(t, 0.6, conf, 0.01, "Should return average confidence")
}

// ============================================================================
// Bollinger Band Detection Tests
// ============================================================================

func TestDetectBandTouch_BelowLower(t *testing.T) {
	agent := &ReversionAgent{
		bollingerPeriod: 20,
		bollingerStdDev: 2.0,
	}

	tests := []struct {
		name           string
		currentPrice   float64
		lowerBand      float64
		upperBand      float64
		position       string
		wantSignal     string
		minConfidence  float64
		containsReason string
	}{
		{
			name:           "price at lower band",
			currentPrice:   48000.0,
			lowerBand:      48000.0,
			upperBand:      52000.0,
			position:       "at_lower",
			wantSignal:     "BUY",
			minConfidence:  0.7,
			containsReason: "lower",
		},
		{
			name:           "price below lower band",
			currentPrice:   47500.0,
			lowerBand:      48000.0,
			upperBand:      52000.0,
			position:       "below_lower",
			wantSignal:     "BUY",
			minConfidence:  0.8,
			containsReason: "lower",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bollinger := &BollingerIndicators{
				UpperBand:  tt.upperBand,
				MiddleBand: (tt.upperBand + tt.lowerBand) / 2,
				LowerBand:  tt.lowerBand,
				Bandwidth:  0.04, // Low bandwidth for higher confidence
				Position:   tt.position,
			}

			signal, confidence, reasoning := agent.detectBandTouch(bollinger, tt.currentPrice)

			assert.Equal(t, tt.wantSignal, signal)
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
			assert.Contains(t, reasoning, tt.containsReason)
		})
	}
}

func TestDetectBandTouch_AboveUpper(t *testing.T) {
	agent := &ReversionAgent{
		bollingerPeriod: 20,
		bollingerStdDev: 2.0,
	}

	tests := []struct {
		name          string
		currentPrice  float64
		lowerBand     float64
		upperBand     float64
		position      string
		wantSignal    string
		minConfidence float64
	}{
		{
			name:          "price at upper band",
			currentPrice:  52000.0,
			lowerBand:     48000.0,
			upperBand:     52000.0,
			position:      "at_upper",
			wantSignal:    "SELL",
			minConfidence: 0.7,
		},
		{
			name:          "price above upper band",
			currentPrice:  52500.0,
			lowerBand:     48000.0,
			upperBand:     52000.0,
			position:      "above_upper",
			wantSignal:    "SELL",
			minConfidence: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bollinger := &BollingerIndicators{
				UpperBand:  tt.upperBand,
				MiddleBand: (tt.upperBand + tt.lowerBand) / 2,
				LowerBand:  tt.lowerBand,
				Bandwidth:  0.04,
				Position:   tt.position,
			}

			signal, confidence, _ := agent.detectBandTouch(bollinger, tt.currentPrice)

			assert.Equal(t, tt.wantSignal, signal)
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
		})
	}
}

func TestDetectBandTouch_BetweenBands(t *testing.T) {
	agent := &ReversionAgent{
		bollingerPeriod: 20,
		bollingerStdDev: 2.0,
	}

	bollinger := &BollingerIndicators{
		UpperBand:  52000.0,
		MiddleBand: 50000.0,
		LowerBand:  48000.0,
		Bandwidth:  0.08,
		Position:   "between",
	}

	signal, confidence, reasoning := agent.detectBandTouch(bollinger, 50000.0)

	assert.Equal(t, "HOLD", signal)
	assert.Equal(t, 0.5, confidence)
	assert.Contains(t, reasoning, "between")
}

// ============================================================================
// RSI Extreme Detection Tests
// ============================================================================

func TestDetectRSIExtreme_Oversold(t *testing.T) {
	agent := &ReversionAgent{
		rsiOversold:   30.0,
		rsiOverbought: 70.0,
	}

	tests := []struct {
		name           string
		rsi            float64
		wantSignal     string
		minConfidence  float64
		containsReason string
	}{
		{
			name:           "very oversold",
			rsi:            15.0,
			wantSignal:     "BUY",
			minConfidence:  0.9,
			containsReason: "oversold",
		},
		{
			name:           "oversold",
			rsi:            25.0,
			wantSignal:     "BUY",
			minConfidence:  0.7,
			containsReason: "oversold",
		},
		// Note: RSI exactly at 30.0 is treated as neutral zone, not oversold
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence, reasoning := agent.detectRSIExtreme(tt.rsi)

			assert.Equal(t, tt.wantSignal, signal)
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
			assert.Contains(t, reasoning, tt.containsReason)
		})
	}
}

func TestDetectRSIExtreme_Overbought(t *testing.T) {
	agent := &ReversionAgent{
		rsiOversold:   30.0,
		rsiOverbought: 70.0,
	}

	tests := []struct {
		name           string
		rsi            float64
		wantSignal     string
		minConfidence  float64
		containsReason string
	}{
		{
			name:           "very overbought",
			rsi:            85.0,
			wantSignal:     "SELL",
			minConfidence:  0.9,
			containsReason: "overbought",
		},
		{
			name:           "overbought",
			rsi:            75.0,
			wantSignal:     "SELL",
			minConfidence:  0.7,
			containsReason: "overbought",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, confidence, reasoning := agent.detectRSIExtreme(tt.rsi)

			assert.Equal(t, tt.wantSignal, signal)
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
			assert.Contains(t, reasoning, tt.containsReason)
		})
	}
}

func TestDetectRSIExtreme_Neutral(t *testing.T) {
	agent := &ReversionAgent{
		rsiOversold:   30.0,
		rsiOverbought: 70.0,
	}

	signal, confidence, reasoning := agent.detectRSIExtreme(50.0)

	assert.Equal(t, "HOLD", signal)
	assert.Equal(t, 0.5, confidence) // Implementation uses 0.5 for neutral zone
	assert.Contains(t, reasoning, "neutral")
}

// ============================================================================
// Signal Combination Tests
// ============================================================================

func TestCombineSignals_BothBuy(t *testing.T) {
	agent := &ReversionAgent{}

	signal, confidence, reasoning := agent.combineSignalsRuleBased(
		"BUY", 0.8, "Bollinger says BUY",
		"BUY", 0.7, "RSI says BUY",
	)

	assert.Equal(t, "BUY", signal)
	assert.GreaterOrEqual(t, confidence, 0.85) // Both agree, high confidence
	assert.Contains(t, reasoning, "Both")
	assert.Contains(t, reasoning, "agree")
}

func TestCombineSignals_BothSell(t *testing.T) {
	agent := &ReversionAgent{}

	signal, confidence, reasoning := agent.combineSignalsRuleBased(
		"SELL", 0.8, "Bollinger says SELL",
		"SELL", 0.7, "RSI says SELL",
	)

	assert.Equal(t, "SELL", signal)
	assert.GreaterOrEqual(t, confidence, 0.85)
	assert.Contains(t, reasoning, "Both")
	assert.Contains(t, reasoning, "agree")
}

func TestCombineSignals_Conflict(t *testing.T) {
	agent := &ReversionAgent{}

	signal, confidence, reasoning := agent.combineSignalsRuleBased(
		"BUY", 0.8, "Bollinger says BUY",
		"SELL", 0.7, "RSI says SELL",
	)

	assert.Equal(t, "HOLD", signal)
	assert.LessOrEqual(t, confidence, 0.5) // Conflicting signals, low confidence
	assert.Contains(t, reasoning, "CONFLICTING")
}

func TestCombineSignals_OneHold(t *testing.T) {
	agent := &ReversionAgent{}

	signal, confidence, _ := agent.combineSignalsRuleBased(
		"BUY", 0.8, "Bollinger says BUY",
		"HOLD", 0.5, "RSI neutral",
	)

	// When one is HOLD, use the non-HOLD signal but with reduced confidence
	assert.Equal(t, "BUY", signal)
	assert.Less(t, confidence, 0.8) // Confidence should be reduced from original
}

// ============================================================================
// Market Regime Detection Tests
// ============================================================================

func TestDetectMarketRegime(t *testing.T) {
	agent := &ReversionAgent{}

	tests := []struct {
		name        string
		adx         float64
		wantRegime  string
		wantMinConf float64
	}{
		{
			name:        "very ranging (ADX < 20)",
			adx:         15.0,
			wantRegime:  "ranging",
			wantMinConf: 0.9,
		},
		{
			name:        "ranging (ADX = 20)",
			adx:         20.0,
			wantRegime:  "ranging",
			wantMinConf: 0.7, // Implementation uses 0.7 for ADX=20
		},
		{
			name:        "moderate trending (ADX = 30)",
			adx:         30.0,
			wantRegime:  "trending",
			wantMinConf: 0.7,
		},
		{
			name:        "strong trending (ADX = 40)",
			adx:         40.0,
			wantRegime:  "trending",
			wantMinConf: 0.8,
		},
		{
			name:        "volatile (ADX > 50)",
			adx:         55.0,
			wantRegime:  "volatile",
			wantMinConf: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := agent.detectMarketRegime(tt.adx)

			assert.Equal(t, tt.wantRegime, regime.Type)
			assert.GreaterOrEqual(t, regime.Confidence, tt.wantMinConf)
		})
	}
}

// ============================================================================
// Exit Level Calculation Tests
// ============================================================================

func TestCalculateExitLevels_Buy(t *testing.T) {
	agent := &ReversionAgent{
		stopLossPct:   0.02, // 2%
		takeProfitPct: 0.03, // 3%
	}

	entryPrice := 50000.0
	stopLoss, takeProfit, riskReward := agent.calculateExitLevels("BUY", entryPrice)

	assert.Equal(t, 49000.0, stopLoss)       // 50000 * (1 - 0.02)
	assert.Equal(t, 51500.0, takeProfit)     // 50000 * (1 + 0.03)
	assert.InDelta(t, 1.5, riskReward, 0.01) // 3% / 2% = 1.5
}

func TestCalculateExitLevels_Sell(t *testing.T) {
	agent := &ReversionAgent{
		stopLossPct:   0.02,
		takeProfitPct: 0.03,
	}

	entryPrice := 50000.0
	stopLoss, takeProfit, riskReward := agent.calculateExitLevels("SELL", entryPrice)

	assert.Equal(t, 51000.0, stopLoss)       // 50000 * (1 + 0.02)
	assert.Equal(t, 48500.0, takeProfit)     // 50000 * (1 - 0.03)
	assert.InDelta(t, 1.5, riskReward, 0.01) // 3% / 2% = 1.5
}

func TestCalculateExitLevels_Hold(t *testing.T) {
	agent := &ReversionAgent{
		stopLossPct:   0.02,
		takeProfitPct: 0.03,
	}

	entryPrice := 50000.0
	stopLoss, takeProfit, riskReward := agent.calculateExitLevels("HOLD", entryPrice)

	assert.Equal(t, 0.0, stopLoss)
	assert.Equal(t, 0.0, takeProfit)
	assert.Equal(t, 0.0, riskReward)
}

func TestCalculateExitLevels_DifferentPercentages(t *testing.T) {
	agent := &ReversionAgent{
		stopLossPct:   0.015, // 1.5%
		takeProfitPct: 0.045, // 4.5%
	}

	entryPrice := 10000.0
	stopLoss, takeProfit, riskReward := agent.calculateExitLevels("BUY", entryPrice)

	assert.Equal(t, 9850.0, stopLoss)        // 10000 * (1 - 0.015)
	assert.Equal(t, 10450.0, takeProfit)     // 10000 * (1 + 0.045)
	assert.InDelta(t, 3.0, riskReward, 0.01) // 4.5% / 1.5% = 3.0
}

// ============================================================================
// Band Position Detection Tests
// ============================================================================

func TestDetectBandPosition_BelowLower(t *testing.T) {
	agent := &ReversionAgent{}

	position := agent.detectBandPosition(47500.0, 52000.0, 50000.0, 48000.0)
	assert.Equal(t, "below_lower", position)
}

func TestDetectBandPosition_AtLower(t *testing.T) {
	agent := &ReversionAgent{}

	// Within 0.5% of lower band (48000 * 1.005 = 48240)
	position := agent.detectBandPosition(48100.0, 52000.0, 50000.0, 48000.0)
	assert.Equal(t, "at_lower", position)
}

func TestDetectBandPosition_AtUpper(t *testing.T) {
	agent := &ReversionAgent{}

	// Within 0.5% of upper band (52000 * 0.995 = 51740)
	position := agent.detectBandPosition(51900.0, 52000.0, 50000.0, 48000.0)
	assert.Equal(t, "at_upper", position)
}

func TestDetectBandPosition_AboveUpper(t *testing.T) {
	agent := &ReversionAgent{}

	position := agent.detectBandPosition(52500.0, 52000.0, 50000.0, 48000.0)
	assert.Equal(t, "above_upper", position)
}

func TestDetectBandPosition_Between(t *testing.T) {
	agent := &ReversionAgent{}

	position := agent.detectBandPosition(50000.0, 52000.0, 50000.0, 48000.0)
	assert.Equal(t, "between", position)
}

// ============================================================================
// Regime Filtering Tests
// ============================================================================

func TestFilterSignalByRegime_RangingMarket_BuySignal(t *testing.T) {
	agent := &ReversionAgent{}

	regime := &MarketRegime{
		Type:       "ranging",
		Confidence: 0.8,
	}

	signal, confidence, reasoning := agent.filterSignalByRegime("BUY", 0.8, "Original reasoning", regime)

	// Ranging markets are ideal for mean reversion
	assert.Equal(t, "BUY", signal)
	assert.GreaterOrEqual(t, confidence, 0.8) // Confidence should be maintained or increased
	assert.Contains(t, reasoning, "RANGING")  // Implementation uses uppercase
}

func TestFilterSignalByRegime_TrendingMarket_SuppressSignal(t *testing.T) {
	agent := &ReversionAgent{}

	regime := &MarketRegime{
		Type:       "trending",
		Confidence: 0.9,
	}

	signal, confidence, reasoning := agent.filterSignalByRegime("BUY", 0.8, "Original reasoning", regime)

	// Trending markets are not ideal for mean reversion - signal should be suppressed
	assert.Equal(t, "HOLD", signal)
	assert.Less(t, confidence, 0.8) // Confidence should be reduced
	assert.Contains(t, reasoning, "trending")
}

func TestFilterSignalByRegime_VolatileMarket_SuppressSignal(t *testing.T) {
	agent := &ReversionAgent{}

	regime := &MarketRegime{
		Type:       "volatile",
		Confidence: 0.85,
	}

	signal, confidence, reasoning := agent.filterSignalByRegime("SELL", 0.7, "Original reasoning", regime)

	// Volatile markets suppress mean reversion signals
	assert.Equal(t, "HOLD", signal)
	assert.Less(t, confidence, 0.7)
	assert.Contains(t, reasoning, "volatile")
}

func TestFilterSignalByRegime_HoldSignal(t *testing.T) {
	agent := &ReversionAgent{}

	regime := &MarketRegime{
		Type:       "ranging",
		Confidence: 0.8,
	}

	signal, confidence, _ := agent.filterSignalByRegime("HOLD", 0.5, "No signal", regime)

	// HOLD signals should remain HOLD regardless of regime
	assert.Equal(t, "HOLD", signal)
	assert.Equal(t, 0.5, confidence)
}

// ============================================================================
// Additional Signal Combination Tests (for 100% coverage)
// ============================================================================

func TestCombineSignals_BothHold(t *testing.T) {
	agent := &ReversionAgent{}

	signal, confidence, reasoning := agent.combineSignalsRuleBased(
		"HOLD", 0.5, "Bollinger neutral",
		"HOLD", 0.5, "RSI neutral",
	)

	assert.Equal(t, "HOLD", signal)
	assert.Equal(t, 0.5, confidence)
	assert.Contains(t, reasoning, "neutral") // Implementation says "Both ... are neutral"
}

// ============================================================================
// Belief Update Function Tests
// ============================================================================

func TestUpdateBollingerBeliefs_TightBands(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	indicators := &BollingerIndicators{
		UpperBand:  51000.0,
		MiddleBand: 50000.0,
		LowerBand:  49000.0,
		Bandwidth:  0.04, // Tight bands
		Position:   "at_lower",
	}

	agent.updateBollingerBeliefs(indicators, 49500.0)

	// Verify all beliefs are set
	upper, exists := agent.beliefs.GetBelief("bollinger_upper")
	require.True(t, exists)
	assert.Equal(t, 51000.0, upper.Value)
	assert.Equal(t, 0.9, upper.Confidence)

	middle, _ := agent.beliefs.GetBelief("bollinger_middle")
	assert.Equal(t, 50000.0, middle.Value)

	lower, _ := agent.beliefs.GetBelief("bollinger_lower")
	assert.Equal(t, 49000.0, lower.Value)

	bandwidth, _ := agent.beliefs.GetBelief("bollinger_bandwidth")
	assert.Equal(t, 0.04, bandwidth.Value)

	position, _ := agent.beliefs.GetBelief("bollinger_position")
	assert.Equal(t, "at_lower", position.Value)

	price, _ := agent.beliefs.GetBelief("current_price")
	assert.Equal(t, 49500.0, price.Value)

	// For tight bands (< 0.05), confidence should be high (0.9)
	signalConf, _ := agent.beliefs.GetBelief("band_signal_confidence")
	assert.Equal(t, 0.9, signalConf.Value)
}

func TestUpdateBollingerBeliefs_WideBands(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	indicators := &BollingerIndicators{
		UpperBand:  55000.0,
		MiddleBand: 50000.0,
		LowerBand:  45000.0,
		Bandwidth:  0.20, // Wide bands (high volatility)
		Position:   "between",
	}

	agent.updateBollingerBeliefs(indicators, 50000.0)

	// For wide bands (> 0.15), confidence should be lower (0.5)
	signalConf, _ := agent.beliefs.GetBelief("band_signal_confidence")
	assert.Equal(t, 0.5, signalConf.Value)
}

func TestUpdateRSIBeliefs_VeryOversold(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	agent.updateRSIBeliefs(15.0, "BUY", 0.95)

	rsiValue, exists := agent.beliefs.GetBelief("rsi_value")
	require.True(t, exists)
	assert.Equal(t, 15.0, rsiValue.Value)

	rsiSignal, _ := agent.beliefs.GetBelief("rsi_signal")
	assert.Equal(t, "BUY", rsiSignal.Value)
	assert.Equal(t, 0.95, rsiSignal.Confidence)

	rsiState, _ := agent.beliefs.GetBelief("rsi_state")
	assert.Equal(t, "very_oversold", rsiState.Value)
}

func TestUpdateRSIBeliefs_Overbought(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	agent.updateRSIBeliefs(75.0, "SELL", 0.8)

	rsiState, _ := agent.beliefs.GetBelief("rsi_state")
	assert.Equal(t, "overbought", rsiState.Value)
}

func TestUpdateRSIBeliefs_Neutral(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	agent.updateRSIBeliefs(50.0, "HOLD", 0.5)

	rsiState, _ := agent.beliefs.GetBelief("rsi_state")
	assert.Equal(t, "neutral", rsiState.Value)
}

func TestUpdateRegimeBeliefs_Ranging(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	regime := &MarketRegime{
		Type:       "ranging",
		ADX:        15.0,
		Confidence: 0.9,
	}

	agent.updateRegimeBeliefs(regime)

	marketRegime, exists := agent.beliefs.GetBelief("market_regime")
	require.True(t, exists)
	assert.Equal(t, "ranging", marketRegime.Value)
	assert.Equal(t, 0.9, marketRegime.Confidence)

	adxValue, _ := agent.beliefs.GetBelief("adx_value")
	assert.Equal(t, 15.0, adxValue.Value)

	// Ranging markets are favorable for mean reversion
	favorable, _ := agent.beliefs.GetBelief("regime_favorable")
	assert.Equal(t, true, favorable.Value)
}

func TestUpdateRegimeBeliefs_Trending(t *testing.T) {
	agent := &ReversionAgent{
		beliefs: NewBeliefBase(),
	}

	regime := &MarketRegime{
		Type:       "trending",
		ADX:        35.0,
		Confidence: 0.8,
	}

	agent.updateRegimeBeliefs(regime)

	marketRegime, _ := agent.beliefs.GetBelief("market_regime")
	assert.Equal(t, "trending", marketRegime.Value)

	// Trending markets are NOT favorable for mean reversion
	favorable, _ := agent.beliefs.GetBelief("regime_favorable")
	assert.Equal(t, false, favorable.Value)
}

func TestUpdateExitBeliefs_FavorableRiskReward(t *testing.T) {
	agent := &ReversionAgent{
		beliefs:         NewBeliefBase(),
		riskRewardRatio: 1.5, // Minimum required ratio
	}

	agent.updateExitBeliefs(49000.0, 51500.0, 1.5)

	stopLoss, exists := agent.beliefs.GetBelief("stop_loss")
	require.True(t, exists)
	assert.Equal(t, 49000.0, stopLoss.Value)
	assert.Equal(t, 1.0, stopLoss.Confidence)

	takeProfit, _ := agent.beliefs.GetBelief("take_profit")
	assert.Equal(t, 51500.0, takeProfit.Value)

	riskReward, _ := agent.beliefs.GetBelief("risk_reward_ratio")
	assert.Equal(t, 1.5, riskReward.Value)

	// Risk/reward of 1.5 meets the minimum 1.5 requirement
	favorable, _ := agent.beliefs.GetBelief("risk_reward_favorable")
	assert.Equal(t, true, favorable.Value)
}

func TestUpdateExitBeliefs_UnfavorableRiskReward(t *testing.T) {
	agent := &ReversionAgent{
		beliefs:         NewBeliefBase(),
		riskRewardRatio: 2.0, // Higher minimum required
	}

	agent.updateExitBeliefs(49000.0, 51500.0, 1.5)

	// Risk/reward of 1.5 does NOT meet the minimum 2.0 requirement
	favorable, _ := agent.beliefs.GetBelief("risk_reward_favorable")
	assert.Equal(t, false, favorable.Value)
}

func TestUpdateBasicBeliefs(t *testing.T) {
	agent := &ReversionAgent{
		beliefs:    NewBeliefBase(),
		lastSignal: "BUY",
	}

	agent.updateBasicBeliefs()

	agentState, exists := agent.beliefs.GetBelief("agent_state")
	require.True(t, exists)
	assert.Equal(t, "initializing", agentState.Value)

	lastSignal, _ := agent.beliefs.GetBelief("last_signal")
	assert.Equal(t, "BUY", lastSignal.Value)

	strategy, _ := agent.beliefs.GetBelief("strategy")
	assert.Equal(t, "mean_reversion", strategy.Value)
}

// ============================================================================
// Config Helper Function Tests
// ============================================================================

func TestGetIntFromConfig_IntValue(t *testing.T) {
	config := map[string]interface{}{
		"period": 20,
	}

	value := getIntFromConfig(config, "period", 14)
	assert.Equal(t, 20, value)
}

func TestGetIntFromConfig_FloatValue(t *testing.T) {
	config := map[string]interface{}{
		"period": 20.0,
	}

	value := getIntFromConfig(config, "period", 14)
	assert.Equal(t, 20, value)
}

func TestGetIntFromConfig_MissingKey(t *testing.T) {
	config := map[string]interface{}{}

	value := getIntFromConfig(config, "period", 14)
	assert.Equal(t, 14, value) // Returns default
}

func TestGetFloatFromConfig_SimpleKey(t *testing.T) {
	config := map[string]interface{}{
		"threshold": 0.75,
	}

	value := getFloatFromConfig(config, "threshold", 0.5)
	assert.Equal(t, 0.75, value)
}

func TestGetFloatFromConfig_NestedKey(t *testing.T) {
	config := map[string]interface{}{
		"risk_management": map[string]interface{}{
			"stop_loss_pct": 0.02,
		},
	}

	value := getFloatFromConfig(config, "risk_management.stop_loss_pct", 0.05)
	assert.Equal(t, 0.02, value)
}

func TestGetFloatFromConfig_MissingKey(t *testing.T) {
	config := map[string]interface{}{}

	value := getFloatFromConfig(config, "threshold", 0.5)
	assert.Equal(t, 0.5, value) // Returns default
}

func TestGetStringSliceFromConfig_Found(t *testing.T) {
	config := map[string]interface{}{
		"symbols": []interface{}{"bitcoin", "ethereum"},
	}

	value := getStringSliceFromConfig(config, "symbols", []string{"default"})
	assert.Equal(t, []string{"bitcoin", "ethereum"}, value)
}

func TestGetStringSliceFromConfig_MissingKey(t *testing.T) {
	config := map[string]interface{}{}

	value := getStringSliceFromConfig(config, "symbols", []string{"default"})
	assert.Equal(t, []string{"default"}, value)
}
