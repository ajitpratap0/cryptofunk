// Polymarket Agent Unit Tests
//
//nolint:goconst // Test files use repeated strings for clarity
package main

import (
	"fmt"
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
		_, exists := bb.GetBelief("nope")
		assert.False(t, exists)
	})

	t.Run("overwrite belief", func(t *testing.T) {
		bb := NewBeliefBase()
		bb.UpdateBelief("k", "v1", 0.5, "s1")
		bb.UpdateBelief("k", "v2", 0.9, "s2")

		belief, _ := bb.GetBelief("k")
		assert.Equal(t, "v2", belief.Value)
		assert.Equal(t, 0.9, belief.Confidence)
	})

	t.Run("get all beliefs returns copy", func(t *testing.T) {
		bb := NewBeliefBase()
		bb.UpdateBelief("a", 1, 0.5, "s")
		bb.UpdateBelief("b", 2, 0.7, "s")

		all := bb.GetAllBeliefs()
		assert.Len(t, all, 2)
		// Mutating copy should not affect original
		delete(all, "a")
		_, exists := bb.GetBelief("a")
		assert.True(t, exists)
	})

	t.Run("confidence average", func(t *testing.T) {
		bb := NewBeliefBase()
		bb.UpdateBelief("a", 1, 0.6, "s")
		bb.UpdateBelief("b", 2, 0.8, "s")
		bb.UpdateBelief("c", 3, 1.0, "s")

		assert.InDelta(t, 0.8, bb.GetConfidence(), 0.01)
	})

	t.Run("empty confidence", func(t *testing.T) {
		bb := NewBeliefBase()
		assert.Equal(t, 0.0, bb.GetConfidence())
	})
}

// ============================================================================
// MARKET DOMAIN TESTS
// ============================================================================

func newTestMarket(yesPrice, noPrice float64, daysOut float64) *PolyMarket {
	return &PolyMarket{
		ID:             "test-market-1",
		Question:       "Will BTC hit $100k?",
		Category:       "crypto",
		OutcomeType:    "binary",
		YesPrice:       yesPrice,
		NoPrice:        noPrice,
		Spread:         math.Abs((yesPrice + noPrice) - 1.0),
		Volume24h:      50000,
		Liquidity:      100000,
		ResolutionTime: time.Now().Add(time.Duration(daysOut*24) * time.Hour),
		Active:         true,
	}
}

func TestPolyMarket_BidAskSpread(t *testing.T) {
	m := newTestMarket(0.55, 0.50, 3)
	spread := m.BidAskSpread()
	assert.InDelta(t, 0.05, spread, 0.001)
}

func TestPolyMarket_DaysUntilResolution(t *testing.T) {
	m := newTestMarket(0.5, 0.5, 5.0)
	days := m.DaysUntilResolution()
	assert.InDelta(t, 5.0, days, 0.1)
}

func TestPosition_UnrealizedPnL(t *testing.T) {
	pos := &Position{
		MarketID:   "test",
		Side:       "YES",
		EntryPrice: 0.40,
		Size:       10.0,
	}

	// Price went up to 0.50
	pnl := pos.UnrealizedPnL(0.50)
	// shares = 10/0.40 = 25, value = 25*0.50 = 12.5, pnl = 12.5 - 10 = 2.5
	assert.InDelta(t, 2.5, pnl, 0.01)

	pnlPct := pos.UnrealizedPnLPct(0.50)
	assert.InDelta(t, 25.0, pnlPct, 0.1)
}

func TestPosition_UnrealizedPnL_Loss(t *testing.T) {
	pos := &Position{
		MarketID:   "test",
		Side:       "YES",
		EntryPrice: 0.60,
		Size:       10.0,
	}

	pnl := pos.UnrealizedPnL(0.50)
	// shares = 10/0.60 ≈ 16.67, value = 16.67*0.50 ≈ 8.33, pnl ≈ -1.67
	assert.True(t, pnl < 0)
}

func TestPosition_ZeroSize(t *testing.T) {
	pos := &Position{Size: 0}
	assert.Equal(t, 0.0, pos.UnrealizedPnLPct(0.5))
}

func TestPosition_CurrentPrice(t *testing.T) {
	market := newTestMarket(0.65, 0.40, 3)

	posYes := &Position{Side: "YES"}
	assert.Equal(t, 0.65, posYes.CurrentPrice(market))

	posNo := &Position{Side: "NO"}
	assert.Equal(t, 0.40, posNo.CurrentPrice(market))
}

// ============================================================================
// MARKET FILTERING TESTS
// ============================================================================

func newTestAgent() *PolymarketAgent {
	return &PolymarketAgent{
		positions:           make(map[string]*Position),
		pollInterval:        5 * time.Minute,
		maxPositionSize:     10.0,
		maxTotalExposure:    50.0,
		confidenceThreshold: 0.70,
		profitTarget:        0.15,
		stopLoss:            0.10,
		minLiquidity:        1000.0,
		maxSpread:           0.10,
		minDaysToResolution: 1.0,
		maxDaysToResolution: 7.0,
		preferredCategories: []string{"politics", "crypto", "tech", "news"},
		beliefs:             NewBeliefBase(),
	}
}

func TestFilterMarkets_Basic(t *testing.T) {
	agent := newTestAgent()

	markets := []*PolyMarket{
		// Good market: binary, liquid, proper spread, right timeframe, right category
		{
			ID: "good", Question: "Will X happen?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
		// Bad: not binary
		{
			ID: "multi", Question: "Who wins?", Category: "politics",
			OutcomeType: "multi", YesPrice: 0.30, NoPrice: 0.30,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
		// Bad: too illiquid
		{
			ID: "illiquid", Question: "Obscure?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: 100, Liquidity: 500, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
		// Bad: spread too wide
		{
			ID: "wide-spread", Question: "Wide?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.60, NoPrice: 0.55,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
		// Bad: resolves too soon
		{
			ID: "too-soon", Question: "Soon?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(6 * time.Hour),
		},
		// Bad: resolves too late
		{
			ID: "too-late", Question: "Late?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(30 * 24 * time.Hour),
		},
		// Bad: wrong category
		{
			ID: "sports", Question: "Game?", Category: "sports",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
		// Bad: inactive
		{
			ID: "inactive", Question: "Dead?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: 50000, Liquidity: 10000, Active: false,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
	}

	filtered := agent.filterMarkets(markets)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "good", filtered[0].ID)
}

func TestFilterMarkets_SpreadTooTight(t *testing.T) {
	agent := newTestAgent()

	// Market is efficient (yes+no = 1.0 exactly), no edge
	markets := []*PolyMarket{
		{
			ID: "efficient", Question: "Efficient?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.50, NoPrice: 0.50,
			Volume24h: 50000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		},
	}

	filtered := agent.filterMarkets(markets)
	assert.Len(t, filtered, 0)
}

func TestFilterMarkets_LimitsTo10(t *testing.T) {
	agent := newTestAgent()

	var markets []*PolyMarket
	for i := 0; i < 20; i++ {
		markets = append(markets, &PolyMarket{
			ID: fmt.Sprintf("m%d", i), Question: "Q?", Category: "crypto",
			OutcomeType: "binary", YesPrice: 0.55, NoPrice: 0.50,
			Volume24h: float64(20-i) * 1000, Liquidity: 10000, Active: true,
			ResolutionTime: time.Now().Add(3 * 24 * time.Hour),
		})
	}

	filtered := agent.filterMarkets(markets)
	assert.LessOrEqual(t, len(filtered), 10)
}

// ============================================================================
// POSITION SIZE CALCULATION TESTS
// ============================================================================

func TestCalculatePositionSize(t *testing.T) {
	agent := newTestAgent()

	t.Run("positive edge gives position", func(t *testing.T) {
		size := agent.calculatePositionSize(0.15, 0.80, 0.40)
		assert.True(t, size > 0, "Should have positive size for good edge")
		assert.True(t, size <= agent.maxPositionSize, "Should not exceed max")
	})

	t.Run("no edge gives zero", func(t *testing.T) {
		size := agent.calculatePositionSize(0.0, 0.50, 0.50)
		assert.Equal(t, 0.0, size)
	})

	t.Run("low confidence gives smaller position", func(t *testing.T) {
		highConf := agent.calculatePositionSize(0.15, 0.85, 0.40)
		lowConf := agent.calculatePositionSize(0.15, 0.55, 0.40)
		assert.True(t, highConf > lowConf || lowConf == 0)
	})

	t.Run("price at boundary gives zero", func(t *testing.T) {
		assert.Equal(t, 0.0, agent.calculatePositionSize(0.15, 0.80, 0.0))
		assert.Equal(t, 0.0, agent.calculatePositionSize(0.15, 0.80, 1.0))
	})
}

// ============================================================================
// SIGNAL GENERATION TESTS
// ============================================================================

func TestGenerateSignal_YES(t *testing.T) {
	agent := newTestAgent()

	market := newTestMarket(0.40, 0.55, 3)
	analysis := &LLMAnalysis{
		MarketID:   "test-market-1",
		Prediction: "YES",
		Confidence: 0.85,
		FairPrice:  0.60, // LLM thinks YES is worth 0.60 but market says 0.40
		Reasoning:  "Strong evidence",
	}

	signal := agent.generateSignal(market, analysis)
	require.NotNil(t, signal)
	assert.Equal(t, "BUY_YES", signal.Signal)
	assert.Equal(t, "YES", signal.Side)
	assert.InDelta(t, 0.40, signal.Price, 0.001)
	assert.InDelta(t, 0.20, signal.Edge, 0.001) // 0.60 - 0.40
	assert.True(t, signal.Size > 0)
}

func TestGenerateSignal_NO(t *testing.T) {
	agent := newTestAgent()

	market := newTestMarket(0.70, 0.35, 3)
	analysis := &LLMAnalysis{
		MarketID:   "test-market-1",
		Prediction: "NO",
		Confidence: 0.80,
		FairPrice:  0.50, // LLM thinks YES is worth 0.50, so NO is worth 0.50
		Reasoning:  "Overpriced YES",
	}

	signal := agent.generateSignal(market, analysis)
	require.NotNil(t, signal)
	assert.Equal(t, "BUY_NO", signal.Signal)
	assert.Equal(t, "NO", signal.Side)
	assert.InDelta(t, 0.35, signal.Price, 0.001)
	// edge = (1.0 - 0.50) - 0.35 = 0.15
	assert.InDelta(t, 0.15, signal.Edge, 0.001)
}

func TestGenerateSignal_NoEdge(t *testing.T) {
	agent := newTestAgent()

	market := newTestMarket(0.60, 0.45, 3)
	analysis := &LLMAnalysis{
		MarketID:   "test-market-1",
		Prediction: "YES",
		Confidence: 0.80,
		FairPrice:  0.62, // Edge = 0.62 - 0.60 = 0.02, below 0.05 threshold
		Reasoning:  "Slightly underpriced",
	}

	signal := agent.generateSignal(market, analysis)
	assert.Nil(t, signal, "Should not generate signal when edge < 5%")
}

// ============================================================================
// POSITION MANAGEMENT TESTS
// ============================================================================

func TestPositionTracking(t *testing.T) {
	agent := newTestAgent()

	assert.False(t, agent.hasPosition("m1"))
	assert.Equal(t, 0.0, agent.totalExposure())

	agent.openPosition(&TradeSignal{
		MarketID:   "m1",
		Question:   "Test?",
		Side:       "YES",
		Price:      0.40,
		Size:       10.0,
		Confidence: 0.80,
	})

	assert.True(t, agent.hasPosition("m1"))
	assert.Equal(t, 10.0, agent.totalExposure())

	agent.openPosition(&TradeSignal{
		MarketID:   "m2",
		Question:   "Test2?",
		Side:       "NO",
		Price:      0.50,
		Size:       15.0,
		Confidence: 0.75,
	})

	assert.Equal(t, 25.0, agent.totalExposure())
	assert.Len(t, agent.getPositions(), 2)

	agent.closePosition("m1")
	assert.False(t, agent.hasPosition("m1"))
	assert.Equal(t, 15.0, agent.totalExposure())
}

func TestExposureLimits(t *testing.T) {
	agent := newTestAgent()
	agent.maxTotalExposure = 20.0

	agent.openPosition(&TradeSignal{MarketID: "m1", Size: 10.0, Side: "YES", Price: 0.5, Confidence: 0.8})
	agent.openPosition(&TradeSignal{MarketID: "m2", Size: 10.0, Side: "YES", Price: 0.5, Confidence: 0.8})

	assert.Equal(t, 20.0, agent.totalExposure())
	assert.True(t, agent.totalExposure() >= agent.maxTotalExposure)
}

// ============================================================================
// FULL DECISION LOOP MOCK TEST
// ============================================================================

func TestDecisionLoop_Integration(t *testing.T) {
	agent := newTestAgent()

	// Simulate: we have a market, LLM says YES with high confidence
	market := newTestMarket(0.35, 0.60, 4)
	market.ID = "btc-100k"

	analysis := &LLMAnalysis{
		MarketID:   "btc-100k",
		Prediction: "YES",
		Confidence: 0.85,
		FairPrice:  0.55,
		Reasoning:  "Strong momentum and institutional buying",
	}

	// Check confidence threshold
	assert.True(t, analysis.Confidence >= agent.confidenceThreshold)

	// Generate signal
	signal := agent.generateSignal(market, analysis)
	require.NotNil(t, signal)
	assert.Equal(t, "BUY_YES", signal.Signal)
	assert.True(t, signal.Edge >= 0.05, "Edge should be >= 5%%")

	// Open position
	agent.openPosition(signal)
	assert.True(t, agent.hasPosition("btc-100k"))
	assert.True(t, agent.totalExposure() > 0)

	// Verify position details
	positions := agent.getPositions()
	pos := positions["btc-100k"]
	require.NotNil(t, pos)
	assert.Equal(t, "YES", pos.Side)
	assert.Equal(t, signal.Price, pos.EntryPrice)
}

// ensure fmt is used
var _ = fmt.Sprintf
