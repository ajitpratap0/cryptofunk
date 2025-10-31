// Arbitrage Agent Unit Tests
package main

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// BELIEF SYSTEM TESTS
// ============================================================================

func TestBeliefBase(t *testing.T) {
	t.Run("update and retrieve belief", func(t *testing.T) {
		bb := NewBeliefBase()

		bb.UpdateBelief("test_key", "test_value", 0.85, "test_source")

		belief, exists := bb.GetBelief("test_key")
		require.True(t, exists)
		assert.Equal(t, "test_key", belief.Key)
		assert.Equal(t, "test_value", belief.Value)
		assert.Equal(t, 0.85, belief.Confidence)
		assert.Equal(t, "test_source", belief.Source)
	})

	t.Run("get non-existent belief", func(t *testing.T) {
		bb := NewBeliefBase()

		_, exists := bb.GetBelief("non_existent")
		assert.False(t, exists)
	})

	t.Run("update overwrites existing belief", func(t *testing.T) {
		bb := NewBeliefBase()

		bb.UpdateBelief("key", "value1", 0.5, "source1")
		bb.UpdateBelief("key", "value2", 0.9, "source2")

		belief, exists := bb.GetBelief("key")
		require.True(t, exists)
		assert.Equal(t, "value2", belief.Value)
		assert.Equal(t, 0.9, belief.Confidence)
		assert.Equal(t, "source2", belief.Source)
	})

	t.Run("get all beliefs", func(t *testing.T) {
		bb := NewBeliefBase()

		bb.UpdateBelief("key1", "value1", 0.5, "source1")
		bb.UpdateBelief("key2", "value2", 0.7, "source2")
		bb.UpdateBelief("key3", "value3", 0.9, "source3")

		beliefs := bb.GetAllBeliefs()
		assert.Len(t, beliefs, 3)
		assert.Contains(t, beliefs, "key1")
		assert.Contains(t, beliefs, "key2")
		assert.Contains(t, beliefs, "key3")
	})

	t.Run("calculate overall confidence", func(t *testing.T) {
		bb := NewBeliefBase()

		bb.UpdateBelief("key1", "value1", 0.6, "source1")
		bb.UpdateBelief("key2", "value2", 0.8, "source2")
		bb.UpdateBelief("key3", "value3", 1.0, "source3")

		// Average: (0.6 + 0.8 + 1.0) / 3 = 0.8
		confidence := bb.GetConfidence()
		assert.InDelta(t, 0.8, confidence, 0.01)
	})

	t.Run("confidence for empty belief base", func(t *testing.T) {
		bb := NewBeliefBase()

		confidence := bb.GetConfidence()
		assert.Equal(t, 0.0, confidence)
	})
}

// ============================================================================
// SPREAD CALCULATION TESTS
// ============================================================================

func TestCalculateOpportunity(t *testing.T) {
	agent := &ArbitrageAgent{
		minSpread:    0.005, // 0.5%
		maxLatencyMs: 1000,
		exchangeFees: map[string]*ExchangeFees{
			"binance": {
				MakerFee:    0.001,  // 0.1%
				TakerFee:    0.001,  // 0.1%
				WithdrawFee: 0.0005, // 0.05%
			},
			"coinbase": {
				MakerFee:    0.005, // 0.5%
				TakerFee:    0.005, // 0.5%
				WithdrawFee: 0.001, // 0.1%
			},
		},
	}

	t.Run("profitable opportunity", func(t *testing.T) {
		buyPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     50000.0,
			Volume24h: 1000000.0,
			Timestamp: time.Now(),
			Latency:   100,
		}

		sellPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     51000.0, // 2% higher - enough to be profitable after fees
			Volume24h: 1200000.0,
			Timestamp: time.Now(),
			Latency:   150,
		}

		opp := agent.calculateOpportunity("BTC/USDT", "binance", "coinbase", buyPrice, sellPrice)

		require.NotNil(t, opp)
		assert.Equal(t, "BTC/USDT", opp.Symbol)
		assert.Equal(t, "binance", opp.BuyExchange)
		assert.Equal(t, "coinbase", opp.SellExchange)
		assert.Equal(t, 50000.0, opp.BuyPrice)
		assert.Equal(t, 51000.0, opp.SellPrice)
		assert.Greater(t, opp.ProfitPct, 0.5) // At least 0.5% after fees
		assert.False(t, opp.LatencyWarning)
	})

	t.Run("unprofitable opportunity filtered", func(t *testing.T) {
		buyPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     50000.0,
			Volume24h: 1000000.0,
			Timestamp: time.Now(),
			Latency:   100,
		}

		sellPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     50100.0, // Only 0.2% higher - not enough after fees
			Volume24h: 1200000.0,
			Timestamp: time.Now(),
			Latency:   150,
		}

		opp := agent.calculateOpportunity("BTC/USDT", "binance", "coinbase", buyPrice, sellPrice)

		// Should be nil because profit is below minSpread threshold
		assert.Nil(t, opp)
	})

	t.Run("high latency warning", func(t *testing.T) {
		buyPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     50000.0,
			Volume24h: 1000000.0,
			Timestamp: time.Now(),
			Latency:   2000, // High latency
		}

		sellPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     51000.0, // 2% higher
			Volume24h: 1200000.0,
			Timestamp: time.Now(),
			Latency:   150,
		}

		opp := agent.calculateOpportunity("BTC/USDT", "binance", "coinbase", buyPrice, sellPrice)

		require.NotNil(t, opp)
		assert.True(t, opp.LatencyWarning)
	})

	t.Run("execution risk classification", func(t *testing.T) {
		// High profit -> low risk
		buyPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     50000.0,
			Volume24h: 1000000.0,
			Timestamp: time.Now(),
			Latency:   100,
		}

		sellPrice := &ExchangePrice{
			Symbol:    "BTC/USDT",
			Price:     51000.0, // 2% spread
			Volume24h: 1200000.0,
			Timestamp: time.Now(),
			Latency:   100,
		}

		opp := agent.calculateOpportunity("BTC/USDT", "binance", "coinbase", buyPrice, sellPrice)

		require.NotNil(t, opp)
		assert.Equal(t, "low", opp.ExecutionRisk)
	})
}

// ============================================================================
// OPPORTUNITY SCORING TESTS
// ============================================================================

func TestCalculateOpportunityScore(t *testing.T) {
	agent := &ArbitrageAgent{}

	t.Run("high profit high volume", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			ProfitPct:      2.0,      // 2% profit
			Volume24h:      10000000, // $10M volume
			ExecutionRisk:  "low",
			LatencyWarning: false,
			ExpiresAt:      time.Now().Add(30 * time.Second),
		}

		score := agent.calculateOpportunityScore(opp)

		// Scoring algorithm is conservative: profit (50%) + liquidity (25%) weighted
		// At 2% profit and $10M volume, expect score in range 0.55-0.7
		assert.Greater(t, score, 0.5)
		assert.LessOrEqual(t, score, 1.0)
	})

	t.Run("low profit low volume", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			ProfitPct:      0.3,   // 0.3% profit
			Volume24h:      10000, // $10k volume
			ExecutionRisk:  "high",
			LatencyWarning: true,
			ExpiresAt:      time.Now().Add(30 * time.Second),
		}

		score := agent.calculateOpportunityScore(opp)

		assert.Less(t, score, 0.5)
		assert.GreaterOrEqual(t, score, 0.0)
	})

	t.Run("expired opportunity", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			ProfitPct:      2.0,
			Volume24h:      10000000,
			ExecutionRisk:  "low",
			LatencyWarning: false,
			ExpiresAt:      time.Now().Add(-10 * time.Second), // Already expired
		}

		score := agent.calculateOpportunityScore(opp)

		// Score should be very low or zero for expired opportunity
		assert.Less(t, score, 0.1)
	})

	t.Run("expiring soon penalty", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			ProfitPct:      2.0,
			Volume24h:      10000000,
			ExecutionRisk:  "low",
			LatencyWarning: false,
			ExpiresAt:      time.Now().Add(10 * time.Second), // Expiring soon
		}

		score := agent.calculateOpportunityScore(opp)

		// Should be lower than if it had 30+ seconds
		assert.Less(t, score, 0.9)
	})

	t.Run("multiple risk factors penalty", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			ProfitPct:      2.0,
			Volume24h:      10000000,
			ExecutionRisk:  "high",                           // Risk factor 1
			LatencyWarning: true,                             // Risk factor 2
			ExpiresAt:      time.Now().Add(10 * time.Second), // Risk factor 3
		}

		score := agent.calculateOpportunityScore(opp)

		// Multiple risk factors should significantly reduce score
		assert.Less(t, score, 0.5)
	})
}

func TestCalculateOpportunityConfidence(t *testing.T) {
	agent := &ArbitrageAgent{}

	t.Run("high quality opportunity", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			Score:          0.9,
			ProfitPct:      2.0,
			Volume24h:      10000000,
			ExecutionRisk:  "low",
			LatencyWarning: false,
		}

		confidence := agent.calculateOpportunityConfidence(opp)

		assert.Greater(t, confidence, 0.8)
		assert.LessOrEqual(t, confidence, 1.0)
	})

	t.Run("low quality opportunity", func(t *testing.T) {
		opp := &ArbitrageOpportunity{
			Score:          0.3,
			ProfitPct:      0.1,   // Very small spread
			Volume24h:      50000, // Low volume
			ExecutionRisk:  "high",
			LatencyWarning: true,
		}

		confidence := agent.calculateOpportunityConfidence(opp)

		assert.Less(t, confidence, 0.5)
		assert.GreaterOrEqual(t, confidence, 0.0)
	})
}

func TestScoreOpportunities(t *testing.T) {
	agent := &ArbitrageAgent{
		beliefs: NewBeliefBase(),
	}

	t.Run("sorts by score descending", func(t *testing.T) {
		opportunities := []*ArbitrageOpportunity{
			{
				Symbol:         "BTC/USDT",
				ProfitPct:      0.5,
				Volume24h:      100000,
				ExecutionRisk:  "medium",
				LatencyWarning: false,
				ExpiresAt:      time.Now().Add(30 * time.Second),
			},
			{
				Symbol:         "ETH/USDT",
				ProfitPct:      2.0,
				Volume24h:      5000000,
				ExecutionRisk:  "low",
				LatencyWarning: false,
				ExpiresAt:      time.Now().Add(30 * time.Second),
			},
			{
				Symbol:         "SOL/USDT",
				ProfitPct:      1.0,
				Volume24h:      1000000,
				ExecutionRisk:  "low",
				LatencyWarning: false,
				ExpiresAt:      time.Now().Add(30 * time.Second),
			},
		}

		scored := agent.scoreOpportunities(opportunities)

		require.Len(t, scored, 3)

		// Verify sorted by score descending
		for i := 0; i < len(scored)-1; i++ {
			assert.GreaterOrEqual(t, scored[i].Score, scored[i+1].Score)
		}

		// ETH should be first (highest profit + volume)
		assert.Equal(t, "ETH/USDT", scored[0].Symbol)
	})

	t.Run("empty opportunities", func(t *testing.T) {
		opportunities := []*ArbitrageOpportunity{}

		scored := agent.scoreOpportunities(opportunities)

		assert.Len(t, scored, 0)
	})
}

// ============================================================================
// DECISION GENERATION TESTS
// ============================================================================

func TestGenerateDecision(t *testing.T) {
	agent := &ArbitrageAgent{
		confidenceThresh: 0.5,
		minSpread:        0.005,
		beliefs:          NewBeliefBase(),
	}

	t.Run("generates ARBITRAGE signal for good opportunity", func(t *testing.T) {
		opportunities := []*ArbitrageOpportunity{
			{
				Symbol:         "BTC/USDT",
				BuyExchange:    "binance",
				SellExchange:   "coinbase",
				BuyPrice:       50000.0,
				SellPrice:      51000.0,
				ProfitPct:      2.0,
				Score:          0.85,
				Confidence:     0.8,
				Volume24h:      10000000,
				ExecutionRisk:  "low",
				LatencyWarning: false,
				ExpiresAt:      time.Now().Add(30 * time.Second),
			},
		}

		signal := agent.generateDecision(opportunities)

		assert.Equal(t, "ARBITRAGE", signal.Signal)
		assert.Equal(t, "BTC/USDT", signal.Symbol)
		assert.Equal(t, 0.8, signal.Confidence)
		assert.NotNil(t, signal.Opportunity)
		assert.Contains(t, signal.Reasoning, "ARBITRAGE OPPORTUNITY DETECTED")
	})

	t.Run("generates HOLD for no opportunities", func(t *testing.T) {
		opportunities := []*ArbitrageOpportunity{}

		signal := agent.generateDecision(opportunities)

		assert.Equal(t, "HOLD", signal.Signal)
		assert.Equal(t, 0.0, signal.Confidence)
		assert.Contains(t, signal.Reasoning, "No opportunities detected")
	})

	t.Run("generates HOLD for below confidence threshold", func(t *testing.T) {
		opportunities := []*ArbitrageOpportunity{
			{
				Symbol:         "BTC/USDT",
				BuyExchange:    "binance",
				SellExchange:   "coinbase",
				BuyPrice:       50000.0,
				SellPrice:      50300.0,
				ProfitPct:      0.6,
				Score:          0.3,
				Confidence:     0.3, // Below 0.5 threshold
				Volume24h:      100000,
				ExecutionRisk:  "high",
				LatencyWarning: true,
				ExpiresAt:      time.Now().Add(30 * time.Second),
			},
		}

		signal := agent.generateDecision(opportunities)

		assert.Equal(t, "HOLD", signal.Signal)
		assert.Contains(t, signal.Reasoning, "below threshold")
	})

	t.Run("generates HOLD for expired opportunity", func(t *testing.T) {
		opportunities := []*ArbitrageOpportunity{
			{
				Symbol:         "BTC/USDT",
				BuyExchange:    "binance",
				SellExchange:   "coinbase",
				BuyPrice:       50000.0,
				SellPrice:      51000.0,
				ProfitPct:      2.0,
				Score:          0.85,
				Confidence:     0.8,
				Volume24h:      10000000,
				ExecutionRisk:  "low",
				LatencyWarning: false,
				ExpiresAt:      time.Now().Add(-10 * time.Second), // Expired
			},
		}

		signal := agent.generateDecision(opportunities)

		assert.Equal(t, "HOLD", signal.Signal)
		assert.Contains(t, signal.Reasoning, "expired")
	})
}

func TestBuildReasoning(t *testing.T) {
	agent := &ArbitrageAgent{
		minSpread:        0.005,
		confidenceThresh: 0.5,
		beliefs:          NewBeliefBase(),
	}

	topOpp := &ArbitrageOpportunity{
		Symbol:         "BTC/USDT",
		BuyExchange:    "binance",
		SellExchange:   "coinbase",
		BuyPrice:       50000.0,
		SellPrice:      51000.0,
		ProfitPct:      2.0,
		NetSpread:      1000.0,
		Score:          0.85,
		Confidence:     0.8,
		Volume24h:      10000000,
		ExecutionRisk:  "low",
		LatencyWarning: false,
		ExpiresAt:      time.Now().Add(30 * time.Second),
	}

	allOpps := []*ArbitrageOpportunity{topOpp}

	t.Run("contains key sections", func(t *testing.T) {
		reasoning := agent.buildReasoning(topOpp, allOpps)

		assert.Contains(t, reasoning, "ARBITRAGE OPPORTUNITY DETECTED")
		assert.Contains(t, reasoning, "RISK ASSESSMENT")
		assert.Contains(t, reasoning, "STRATEGY CONTEXT")
		assert.Contains(t, reasoning, "RECOMMENDATION")
	})

	t.Run("contains opportunity details", func(t *testing.T) {
		reasoning := agent.buildReasoning(topOpp, allOpps)

		assert.Contains(t, reasoning, "BTC/USDT")
		assert.Contains(t, reasoning, "binance")
		assert.Contains(t, reasoning, "coinbase")
		assert.Contains(t, reasoning, "low")
	})

	t.Run("includes alternatives when multiple opportunities", func(t *testing.T) {
		allOpps := []*ArbitrageOpportunity{
			topOpp,
			{
				Symbol:        "ETH/USDT",
				BuyExchange:   "kraken",
				SellExchange:  "binance",
				ProfitPct:     1.5,
				Score:         0.7,
				ExecutionRisk: "medium",
			},
		}

		reasoning := agent.buildReasoning(topOpp, allOpps)

		assert.Contains(t, reasoning, "ALTERNATIVE OPPORTUNITIES")
		assert.Contains(t, reasoning, "ETH/USDT")
	})

	t.Run("recommendation based on score", func(t *testing.T) {
		// High score + low risk
		topOpp.Score = 0.75
		topOpp.ExecutionRisk = "low"
		reasoning := agent.buildReasoning(topOpp, allOpps)
		assert.Contains(t, reasoning, "STRONG BUY")

		// Medium score
		topOpp.Score = 0.55
		reasoning = agent.buildReasoning(topOpp, allOpps)
		assert.Contains(t, reasoning, "MODERATE BUY")

		// Low score
		topOpp.Score = 0.45
		reasoning = agent.buildReasoning(topOpp, allOpps)
		assert.Contains(t, reasoning, "CAUTIOUS BUY")
	})
}

// ============================================================================
// HELPER FUNCTION TESTS
// ============================================================================

func TestProfitScoreCalculation(t *testing.T) {
	t.Run("profit score increases with profit percentage", func(t *testing.T) {
		// Test that profit score follows expected curve
		testCases := []struct {
			profitPct float64
			minScore  float64
			maxScore  float64
		}{
			{0.5, 0.3, 0.5},   // Low profit
			{1.0, 0.6, 0.7},   // Medium profit
			{2.0, 0.85, 0.95}, // High profit
			{5.0, 0.98, 1.0},  // Very high profit
		}

		for _, tc := range testCases {
			profitScore := 1.0 - math.Exp(-tc.profitPct)
			if profitScore > 1.0 {
				profitScore = 1.0
			}

			assert.GreaterOrEqual(t, profitScore, tc.minScore,
				"Profit %.2f%% should have score >= %.2f", tc.profitPct, tc.minScore)
			assert.LessOrEqual(t, profitScore, tc.maxScore,
				"Profit %.2f%% should have score <= %.2f", tc.profitPct, tc.maxScore)
		}
	})
}

func TestLiquidityScoreCalculation(t *testing.T) {
	t.Run("liquidity score increases with volume", func(t *testing.T) {
		testCases := []struct {
			volume   float64
			minScore float64
			maxScore float64
		}{
			{10000, 0.0, 0.3},     // Very low volume
			{100000, 0.25, 0.55},  // Low volume
			{1000000, 0.45, 0.75}, // Medium volume
			{10000000, 0.7, 1.0},  // High volume
		}

		for _, tc := range testCases {
			logVolume := math.Log10(tc.volume + 1)
			liquidityScore := (logVolume - 4.0) / 4.0
			if liquidityScore < 0 {
				liquidityScore = 0
			}
			if liquidityScore > 1.0 {
				liquidityScore = 1.0
			}

			assert.GreaterOrEqual(t, liquidityScore, tc.minScore,
				"Volume $%.0f should have score >= %.2f", tc.volume, tc.minScore)
			assert.LessOrEqual(t, liquidityScore, tc.maxScore,
				"Volume $%.0f should have score <= %.2f", tc.volume, tc.maxScore)
		}
	})
}
