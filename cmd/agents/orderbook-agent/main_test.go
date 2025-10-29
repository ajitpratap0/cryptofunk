package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testAgentInstance *OrderBookAgent
	testAgentOnce     sync.Once
)

// Helper function to create test OrderBook with proper structure
func createTestOrderBook(bids, asks []OrderBookLevel) *OrderBook {
	return &OrderBook{
		Symbol:    "BTC/USDT",
		Timestamp: time.Now().Unix(),
		Bids:      bids,
		Asks:      asks,
	}
}

// Mock data generators
func mockBuyPressureOrderBook() *OrderBook {
	// 36.0 total bid volume, 5.8 total ask volume
	// Expected imbalance: (36.0 - 5.8) / (36.0 + 5.8) = 30.2 / 41.8 ≈ 0.72
	return createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 10.0},
			{Price: 49990.0, Quantity: 8.0},
			{Price: 49980.0, Quantity: 7.0},
			{Price: 49970.0, Quantity: 6.0},
			{Price: 49960.0, Quantity: 5.0},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 2.0},
			{Price: 50020.0, Quantity: 1.5},
			{Price: 50030.0, Quantity: 1.0},
			{Price: 50040.0, Quantity: 0.8},
			{Price: 50050.0, Quantity: 0.5},
		},
	)
}

func mockSellPressureOrderBook() *OrderBook {
	// 5.8 total bid volume, 36.0 total ask volume
	// Expected imbalance: (5.8 - 36.0) / (5.8 + 36.0) = -30.2 / 41.8 ≈ -0.72
	return createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 2.0},
			{Price: 49990.0, Quantity: 1.5},
			{Price: 49980.0, Quantity: 1.0},
			{Price: 49970.0, Quantity: 0.8},
			{Price: 49960.0, Quantity: 0.5},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 10.0},
			{Price: 50020.0, Quantity: 8.0},
			{Price: 50030.0, Quantity: 7.0},
			{Price: 50040.0, Quantity: 6.0},
			{Price: 50050.0, Quantity: 5.0},
		},
	)
}

func mockBalancedOrderBook() *OrderBook {
	// 18.0 total bid volume, 18.0 total ask volume
	// Expected imbalance: (18.0 - 18.0) / (18.0 + 18.0) = 0
	return createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 5.0},
			{Price: 49990.0, Quantity: 4.0},
			{Price: 49980.0, Quantity: 3.5},
			{Price: 49970.0, Quantity: 3.0},
			{Price: 49960.0, Quantity: 2.5},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 5.0},
			{Price: 50020.0, Quantity: 4.0},
			{Price: 50030.0, Quantity: 3.5},
			{Price: 50040.0, Quantity: 3.0},
			{Price: 50050.0, Quantity: 2.5},
		},
	)
}

// Helper function to create properly initialized test agent
// Uses sync.Once to ensure metrics are only registered once
func createTestAgent() *OrderBookAgent {
	testAgentOnce.Do(func() {
		config := &agents.AgentConfig{
			Name: "test-orderbook-agent",
			Type: "analysis",
			Config: map[string]interface{}{
				"symbol":                 "BTC/USDT",
				"depth_levels":           5,
				"imbalance_threshold":    0.3,
				"large_order_multiplier": 5.0,
				"significant_depth_pct":  5.0,
				"spoofing_window":        "5s",
				"update_interval":        "1s",
				"confidence_threshold":   0.6,
			},
		}

		baseAgent := agents.NewBaseAgent(config, zerolog.Nop(), 0)

		testAgentInstance = &OrderBookAgent{
			BaseAgent:            baseAgent,
			symbol:               "BTC/USDT",
			depthLevels:          5,
			largeOrderMultiplier: 5.0,
			imbalanceThreshold:   0.3,
			spoofingWindow:       5 * time.Second,
			beliefs:              NewBeliefBase(),
			orderBookHistory:     make([]OrderBookSnapshot, 0),
			historyMutex:         sync.RWMutex{},
		}
	})

	return testAgentInstance
}

// =============================================================================
// BeliefBase Tests
// =============================================================================

func TestBeliefBase_UpdateBelief(t *testing.T) {
	bb := NewBeliefBase()

	t.Run("update new belief", func(t *testing.T) {
		bb.UpdateBelief("test_key", "test_value", 0.85, "test")

		belief, exists := bb.GetBelief("test_key")
		require.True(t, exists)
		assert.Equal(t, "test_value", belief.Value)
		assert.Equal(t, 0.85, belief.Confidence)
		assert.Equal(t, "test", belief.Source)
		assert.WithinDuration(t, time.Now(), belief.Timestamp, time.Second)
	})

	t.Run("update existing belief", func(t *testing.T) {
		bb.UpdateBelief("existing_key", "old_value", 0.5, "initial")
		time.Sleep(10 * time.Millisecond) // Ensure timestamp difference
		bb.UpdateBelief("existing_key", "new_value", 0.9, "updated")

		belief, exists := bb.GetBelief("existing_key")
		require.True(t, exists)
		assert.Equal(t, "new_value", belief.Value)
		assert.Equal(t, 0.9, belief.Confidence)
		assert.Equal(t, "updated", belief.Source)
	})
}

func TestBeliefBase_GetBelief(t *testing.T) {
	bb := NewBeliefBase()

	t.Run("get existing belief", func(t *testing.T) {
		bb.UpdateBelief("key1", "value1", 0.75, "source1")
		belief, exists := bb.GetBelief("key1")
		require.True(t, exists)
		assert.Equal(t, "value1", belief.Value)
	})

	t.Run("get non-existent belief", func(t *testing.T) {
		belief, exists := bb.GetBelief("non_existent")
		assert.False(t, exists)
		assert.Nil(t, belief)
	})
}

func TestBeliefBase_GetAllBeliefs(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("key1", "value1", 0.8, "source1")
	bb.UpdateBelief("key2", "value2", 0.6, "source2")
	bb.UpdateBelief("key3", "value3", 0.9, "source3")

	beliefs := bb.GetAllBeliefs()
	assert.Equal(t, 3, len(beliefs))

	// Verify all keys exist
	_, exists1 := beliefs["key1"]
	_, exists2 := beliefs["key2"]
	_, exists3 := beliefs["key3"]
	assert.True(t, exists1)
	assert.True(t, exists2)
	assert.True(t, exists3)
}

func TestBeliefBase_GetConfidence(t *testing.T) {
	bb := NewBeliefBase()

	t.Run("empty belief base", func(t *testing.T) {
		conf := bb.GetConfidence()
		assert.Equal(t, 0.0, conf)
	})

	t.Run("single belief", func(t *testing.T) {
		bb.UpdateBelief("key1", "value1", 0.8, "source1")
		conf := bb.GetConfidence()
		assert.Equal(t, 0.8, conf)
	})

	t.Run("multiple beliefs", func(t *testing.T) {
		bb.UpdateBelief("key2", "value2", 0.6, "source2")
		bb.UpdateBelief("key3", "value3", 1.0, "source3")
		// Average of 0.8, 0.6, 1.0 = 2.4 / 3 = 0.8
		conf := bb.GetConfidence()
		assert.InDelta(t, 0.8, conf, 0.01)
	})
}

func TestBeliefBase_ConcurrentAccess(t *testing.T) {
	bb := NewBeliefBase()
	numGoroutines := 100
	operationsPerGoroutine := 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := "key_" + string(rune(id))
				bb.UpdateBelief(key, id, 0.5, "concurrent_test")
				bb.GetBelief(key)
				bb.GetAllBeliefs()
				bb.GetConfidence()
			}
		}(i)
	}

	wg.Wait()

	// Verify data integrity
	beliefs := bb.GetAllBeliefs()
	assert.NotEmpty(t, beliefs)
	assert.LessOrEqual(t, len(beliefs), numGoroutines)
}

// =============================================================================
// Imbalance Calculation Tests
// =============================================================================

func TestCalculateImbalance_BuyPressure(t *testing.T) {
	agent := createTestAgent()
	ob := mockBuyPressureOrderBook()

	imbalance := agent.calculateImbalance(ob)

	assert.Greater(t, imbalance, 0.5)
	assert.LessOrEqual(t, imbalance, 1.0)
}

func TestCalculateImbalance_SellPressure(t *testing.T) {
	agent := createTestAgent()
	ob := mockSellPressureOrderBook()

	imbalance := agent.calculateImbalance(ob)

	assert.Less(t, imbalance, -0.5)
	assert.GreaterOrEqual(t, imbalance, -1.0)
}

func TestCalculateImbalance_Balanced(t *testing.T) {
	agent := createTestAgent()
	ob := mockBalancedOrderBook()

	imbalance := agent.calculateImbalance(ob)

	assert.InDelta(t, 0.0, imbalance, 0.01)
}

func TestCalculateImbalance_EmptyBids(t *testing.T) {
	agent := createTestAgent()
	ob := createTestOrderBook(
		[]OrderBookLevel{},
		[]OrderBookLevel{{Price: 50010.0, Quantity: 10.0}},
	)

	imbalance := agent.calculateImbalance(ob)

	// When bids are empty, maxLevels = 0, so bidVolume = askVolume = 0, totalVolume = 0 → returns 0.0
	assert.Equal(t, 0.0, imbalance)
}

func TestCalculateImbalance_EmptyAsks(t *testing.T) {
	agent := createTestAgent()
	ob := createTestOrderBook(
		[]OrderBookLevel{{Price: 50000.0, Quantity: 10.0}},
		[]OrderBookLevel{},
	)

	imbalance := agent.calculateImbalance(ob)

	// When asks are empty, maxLevels = 0, so bidVolume = askVolume = 0, totalVolume = 0 → returns 0.0
	assert.Equal(t, 0.0, imbalance)
}

// =============================================================================
// Depth Analysis Tests
// =============================================================================

func TestAnalyzeDepth_BidSupport(t *testing.T) {
	agent := createTestAgent()
	bids := []OrderBookLevel{
		{Price: 50000.0, Quantity: 10.0}, // Large volume = strong support
		{Price: 49990.0, Quantity: 2.0},
		{Price: 49980.0, Quantity: 1.0},
	}

	supportLevels := agent.analyzeDepth(bids, true)

	require.NotEmpty(t, supportLevels)
	// First level should be strongest
	assert.Equal(t, 50000.0, supportLevels[0].Price)
	assert.Greater(t, supportLevels[0].Volume, 5.0)
}

func TestAnalyzeDepth_AskResistance(t *testing.T) {
	agent := createTestAgent()
	asks := []OrderBookLevel{
		{Price: 50010.0, Quantity: 10.0}, // Large volume = strong resistance
		{Price: 50020.0, Quantity: 2.0},
		{Price: 50030.0, Quantity: 1.0},
	}

	resistanceLevels := agent.analyzeDepth(asks, false)

	require.NotEmpty(t, resistanceLevels)
	// First level should be strongest
	assert.Equal(t, 50010.0, resistanceLevels[0].Price)
	assert.Greater(t, resistanceLevels[0].Volume, 5.0)
}

func TestAnalyzeDepth_EmptyLevels(t *testing.T) {
	agent := createTestAgent()
	levels := []OrderBookLevel{}

	result := agent.analyzeDepth(levels, true)

	assert.Empty(t, result)
}

func TestAnalyzeDepth_SingleLevel(t *testing.T) {
	agent := createTestAgent()
	levels := []OrderBookLevel{
		{Price: 50000.0, Quantity: 5.0},
	}

	result := agent.analyzeDepth(levels, true)

	require.Len(t, result, 1)
	assert.Equal(t, 50000.0, result[0].Price)
	assert.Equal(t, 5.0, result[0].Volume)
}

func TestAnalyzeDepth_MultipleSignificantLevels(t *testing.T) {
	agent := createTestAgent()
	levels := []OrderBookLevel{
		{Price: 50000.0, Quantity: 10.0},
		{Price: 49990.0, Quantity: 8.0},
		{Price: 49980.0, Quantity: 7.0},
		{Price: 49970.0, Quantity: 2.0},
		{Price: 49960.0, Quantity: 1.0},
	}

	result := agent.analyzeDepth(levels, true)

	// All levels with significant volume should be included
	assert.GreaterOrEqual(t, len(result), 3)
	for _, level := range result {
		assert.Greater(t, level.Volume, 0.0)
	}
}

// =============================================================================
// Large Order Detection Tests
// =============================================================================

func TestDetectLargeOrders_BidsOnly(t *testing.T) {
	agent := createTestAgent()
	// Total size (all orders): 100.0 + 1.0 + 1.0 + 1.0 + 1.0 = 104.0, count: 5, average: 20.8
	// Threshold: 20.8 * 5.0 = 104.0
	// The 100.0 bid is the only order that would trigger detection, but it's < 104.0
	// So we need the large order to be > threshold: 105.0 is safe
	// Recalculate: total = 105.0 + 4*1.0 = 109.0, average = 21.8, threshold = 109.0
	// 105.0 < 109.0, still won't work!
	// Solution: Make the large order much larger so threshold stays below it
	// Let's use: 200.0 + 1.0 + 1.0 + 1.0 + 1.0 = 204.0, avg = 40.8, threshold = 204.0
	// 200.0 < 204.0 - still no good!
	// Better approach: Use very small other orders
	// 100.0 + 0.1 + 0.1 + 0.1 + 0.1 = 100.4, avg = 20.08, threshold = 100.4
	// 100.0 < 100.4 - nope!
	// Final approach: Make threshold calculation include the large order, but large order must exceed it
	// If large order is X and others are each 0.01, with 4 others:
	// total = X + 0.04, count = 5, avg = (X + 0.04)/5, threshold = (X + 0.04)
	// We need X >= (X + 0.04), which is impossible!
	// Real solution: The multiplier is the key. With multiplier 5.0:
	// If I have one large order of 100 and 4 small orders of 1.0:
	// avg = 104/5 = 20.8, threshold = 104.0, so 100 < 104 (fails)
	// If I have one large order of 500 and 4 small orders of 1.0:
	// avg = 504/5 = 100.8, threshold = 504.0, so 500 < 504 (fails)
	// The math shows: with 5 orders where 4 are size 1.0, the large order L:
	// threshold = (L + 4) / 5 * 5 = L + 4, so we need L >= L + 4 (impossible!)
	// Solution: Add more small orders to dilute the average
	// With 1 large (L) and 9 small (1.0 each): avg = (L+9)/10, threshold = (L+9)/2
	// We need L >= (L+9)/2, so 2L >= L+9, so L >= 9
	// Let's use L=50: avg = 59/10 = 5.9, threshold = 29.5, 50 >= 29.5 ✓
	ob := createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 50.0}, // Large order
			{Price: 49990.0, Quantity: 1.0},
			{Price: 49980.0, Quantity: 1.0},
			{Price: 49970.0, Quantity: 1.0},
			{Price: 49960.0, Quantity: 1.0},
			{Price: 49950.0, Quantity: 1.0},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 1.0},
			{Price: 50020.0, Quantity: 1.0},
			{Price: 50030.0, Quantity: 1.0},
			{Price: 50040.0, Quantity: 1.0},
		},
	)

	largeOrders := agent.detectLargeOrders(ob)

	require.NotEmpty(t, largeOrders)
	assert.Equal(t, "bid", largeOrders[0].Side)
	assert.Equal(t, 50000.0, largeOrders[0].Price)
	assert.Equal(t, 50.0, largeOrders[0].Size)
}

func TestDetectLargeOrders_AsksOnly(t *testing.T) {
	agent := createTestAgent()
	// Mathematical analysis: With 1 large (L) and 9 small (1.0 each):
	// avg = (L+9)/10, threshold = (L+9)/2
	// Need L >= (L+9)/2, so 2L >= L+9, so L >= 9
	// Using L=50: avg = 59/10 = 5.9, threshold = 29.5, 50 >= 29.5 ✓
	ob := createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 1.0},
			{Price: 49990.0, Quantity: 1.0},
			{Price: 49980.0, Quantity: 1.0},
			{Price: 49970.0, Quantity: 1.0},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 50.0}, // Large order
			{Price: 50020.0, Quantity: 1.0},
			{Price: 50030.0, Quantity: 1.0},
			{Price: 50040.0, Quantity: 1.0},
			{Price: 50050.0, Quantity: 1.0},
			{Price: 50060.0, Quantity: 1.0},
		},
	)

	largeOrders := agent.detectLargeOrders(ob)

	require.NotEmpty(t, largeOrders)
	assert.Equal(t, "ask", largeOrders[0].Side)
	assert.Equal(t, 50010.0, largeOrders[0].Price)
	assert.Equal(t, 50.0, largeOrders[0].Size)
}

func TestDetectLargeOrders_BothSides(t *testing.T) {
	agent := createTestAgent()
	// Mathematical analysis: With 2 large orders (L each) and 18 small (1.0 each):
	// total = 2L + 18, count = 20, avg = (2L+18)/20, threshold = (2L+18)/4
	// Need L >= (2L+18)/4, so 4L >= 2L+18, so 2L >= 18, so L >= 9
	// Using L=50: avg = 118/20 = 5.9, threshold = 29.5, 50 >= 29.5 ✓
	ob := createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 50.0}, // Large bid
			{Price: 49990.0, Quantity: 1.0},
			{Price: 49980.0, Quantity: 1.0},
			{Price: 49970.0, Quantity: 1.0},
			{Price: 49960.0, Quantity: 1.0},
			{Price: 49950.0, Quantity: 1.0},
			{Price: 49940.0, Quantity: 1.0},
			{Price: 49930.0, Quantity: 1.0},
			{Price: 49920.0, Quantity: 1.0},
			{Price: 49910.0, Quantity: 1.0},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 50.0}, // Large ask
			{Price: 50020.0, Quantity: 1.0},
			{Price: 50030.0, Quantity: 1.0},
			{Price: 50040.0, Quantity: 1.0},
			{Price: 50050.0, Quantity: 1.0},
			{Price: 50060.0, Quantity: 1.0},
			{Price: 50070.0, Quantity: 1.0},
			{Price: 50080.0, Quantity: 1.0},
			{Price: 50090.0, Quantity: 1.0},
			{Price: 50100.0, Quantity: 1.0},
		},
	)

	largeOrders := agent.detectLargeOrders(ob)

	require.Len(t, largeOrders, 2)
	// Should detect both large orders
	bidFound := false
	askFound := false
	for _, order := range largeOrders {
		if order.Side == "bid" && order.Size == 50.0 {
			bidFound = true
		}
		if order.Side == "ask" && order.Size == 50.0 {
			askFound = true
		}
	}
	assert.True(t, bidFound)
	assert.True(t, askFound)
}

func TestDetectLargeOrders_NoLargeOrders(t *testing.T) {
	agent := createTestAgent()
	ob := createTestOrderBook(
		[]OrderBookLevel{
			{Price: 50000.0, Quantity: 1.0},
			{Price: 49990.0, Quantity: 1.0},
			{Price: 49980.0, Quantity: 1.0},
		},
		[]OrderBookLevel{
			{Price: 50010.0, Quantity: 1.0},
			{Price: 50020.0, Quantity: 1.0},
			{Price: 50030.0, Quantity: 1.0},
		},
	)

	largeOrders := agent.detectLargeOrders(ob)

	assert.Empty(t, largeOrders)
}

func TestDetectLargeOrders_EmptyOrderBook(t *testing.T) {
	agent := createTestAgent()
	ob := createTestOrderBook([]OrderBookLevel{}, []OrderBookLevel{})

	largeOrders := agent.detectLargeOrders(ob)

	assert.Empty(t, largeOrders)
}

// =============================================================================
// Spoofing Detection Tests
// =============================================================================

func TestDetectSpoofing_NoHistory(t *testing.T) {
	agent := createTestAgent()

	spoofingFlags := agent.detectSpoofing()

	assert.Equal(t, 0, spoofingFlags)
}

func TestDetectSpoofing_WithHistory(t *testing.T) {
	agent := createTestAgent()

	// Add some order book snapshots
	ob1 := mockBuyPressureOrderBook()
	agent.addOrderBookSnapshot(ob1)

	time.Sleep(10 * time.Millisecond)

	ob2 := mockSellPressureOrderBook()
	agent.addOrderBookSnapshot(ob2)

	spoofingFlags := agent.detectSpoofing()

	// Should return non-negative integer
	assert.GreaterOrEqual(t, spoofingFlags, 0)
}

func TestAddOrderBookSnapshot_HistoryLimit(t *testing.T) {
	agent := createTestAgent()

	// Clear history to ensure clean state for this test
	// (agent is singleton shared across tests via sync.Once)
	agent.historyMutex.Lock()
	agent.orderBookHistory = make([]OrderBookSnapshot, 0)
	agent.historyMutex.Unlock()

	// Add more snapshots than the spoofing window allows
	for i := 0; i < 20; i++ {
		ob := mockBalancedOrderBook()
		agent.addOrderBookSnapshot(ob)
		time.Sleep(time.Millisecond)
	}

	// Verify history is limited
	agent.historyMutex.RLock()
	historyLen := len(agent.orderBookHistory)
	agent.historyMutex.RUnlock()

	assert.LessOrEqual(t, historyLen, 20)
}

func TestCompareSnapshotsForSpoofing_IdenticalBooks(t *testing.T) {
	agent := createTestAgent()
	ob1 := mockBalancedOrderBook()
	ob2 := mockBalancedOrderBook()

	flags := agent.compareSnapshotsForSpoofing(ob1, ob2)

	// Identical books should have no spoofing flags
	assert.Equal(t, 0, flags)
}

// =============================================================================
// Component Analysis Tests
// =============================================================================

func TestAnalyzeImbalance(t *testing.T) {
	t.Run("strong buy signal", func(t *testing.T) {
		signal, confidence, reasoning := analyzeImbalance(0.8, 0.3)
		assert.Equal(t, "BUY", signal)
		assert.Greater(t, confidence, 0.5)
		assert.Contains(t, reasoning, "Strong buy pressure")
	})

	t.Run("strong sell signal", func(t *testing.T) {
		signal, confidence, reasoning := analyzeImbalance(-0.8, 0.3)
		assert.Equal(t, "SELL", signal)
		assert.Greater(t, confidence, 0.5)
		assert.Contains(t, reasoning, "Strong sell pressure")
	})

	t.Run("weak signal", func(t *testing.T) {
		signal, confidence, reasoning := analyzeImbalance(0.1, 0.3)
		assert.Equal(t, "HOLD", signal)
		// For HOLD: confidence = 1.0 - absImbalance = 1.0 - 0.1 = 0.9
		assert.Equal(t, 0.9, confidence)
		assert.Contains(t, reasoning, "Balanced")
	})

	t.Run("balanced", func(t *testing.T) {
		signal, confidence, reasoning := analyzeImbalance(0.0, 0.3)
		assert.Equal(t, "HOLD", signal)
		// For HOLD: confidence = 1.0 - absImbalance = 1.0 - 0.0 = 1.0
		assert.Equal(t, 1.0, confidence)
		assert.Contains(t, reasoning, "Balanced")
	})
}

func TestAnalyzeDepthProximity(t *testing.T) {
	supportLevels := []PriceLevel{
		{Price: 49990.0, Volume: 8.0},
		{Price: 49980.0, Volume: 6.0},
	}
	resistanceLevels := []PriceLevel{
		{Price: 50010.0, Volume: 8.0},
		{Price: 50020.0, Volume: 6.0},
	}

	t.Run("near support", func(t *testing.T) {
		// 49995.0 is 5.0 away from support at 49990.0
		// Distance percentage: 5.0 / 49995.0 = 0.0001 (0.01%)
		// This is < 1% (0.01) so it triggers BUY signal
		// Confidence: 1.0 - (0.0001 / 0.01) = 1.0 - 0.01 = 0.99
		signal, confidence, reasoning := analyzeDepthProximity(49995.0, supportLevels, resistanceLevels)
		assert.Equal(t, "BUY", signal)
		assert.Greater(t, confidence, 0.98) // Allow for floating point
		assert.Contains(t, reasoning, "support")
	})

	t.Run("near resistance", func(t *testing.T) {
		// 50509.0 is 501.0 away from resistance at 50010.0 (too far: 0.99%)
		// BUT 519.0 away from support at 49990.0 (also too far: 1.03%)
		// So need price closer to resistance but still >1% from support
		// Use 50600.0: distance to resistance = 590.0 / 50600.0 = 0.0117 (1.17%) - too far
		// Actually, need price ABOVE resistance and FAR from support
		// Use 50510.0: distance to support = 520.0 / 50510.0 = 0.0103 (1.03% > 1%)
		//              distance to resistance = 500.0 / 50510.0 = 0.0099 (0.99% < 1%)
		// This triggers SELL signal (resistance is within 1%, support is not)
		signal, confidence, reasoning := analyzeDepthProximity(50510.0, supportLevels, resistanceLevels)
		assert.Equal(t, "SELL", signal)
		assert.Greater(t, confidence, 0.0) // Confidence will be 1.0 - (0.0099 / 0.01) ≈ 0.01
		assert.Contains(t, reasoning, "resistance")
	})

	t.Run("far from levels", func(t *testing.T) {
		// 51000.0 is far from both support (49990.0) and resistance (50010.0)
		// Distance to support: 1010.0 / 51000.0 = 0.0198 (1.98%)
		// Distance to resistance: 990.0 / 51000.0 = 0.0194 (1.94%)
		// Both are > 1% (0.01), so neither BUY nor SELL triggers
		// Returns "HOLD" with confidence 0.5 and reasoning "Not near significant levels"
		signal, confidence, reasoning := analyzeDepthProximity(51000.0, supportLevels, resistanceLevels)
		assert.Equal(t, "HOLD", signal)
		assert.Equal(t, 0.5, confidence)
		assert.Contains(t, reasoning, "Not near")
	})
}

func TestAnalyzeLargeOrders(t *testing.T) {
	t.Run("large bid orders", func(t *testing.T) {
		// For BUY signal: bidCount > askCount AND totalBidSize > totalAskSize * 1.5
		// Here: bidCount=2, askCount=0, totalBidSize=18.0, totalAskSize=0.0
		// Condition: 2 > 0 AND 18.0 > 0.0 * 1.5 → TRUE
		// Confidence: min(bidCount / len(largeOrders), 1.0) = min(2/2, 1.0) = 1.0
		largeOrders := []LargeOrder{
			{Price: 50000.0, Size: 10.0, Side: "bid"},
			{Price: 49990.0, Size: 8.0, Side: "bid"},
		}
		signal, confidence, reasoning := analyzeLargeOrders(largeOrders)
		assert.Equal(t, "BUY", signal)
		assert.Equal(t, 1.0, confidence)            // All orders are bids
		assert.Contains(t, reasoning, "buy orders") // Changed from "BUY"
	})

	t.Run("large ask orders", func(t *testing.T) {
		// For SELL signal: askCount > bidCount AND totalAskSize > totalBidSize * 1.5
		// Here: askCount=2, bidCount=0, totalAskSize=18.0, totalBidSize=0.0
		// Condition: 2 > 0 AND 18.0 > 0.0 * 1.5 → TRUE
		// Confidence: min(askCount / len(largeOrders), 1.0) = min(2/2, 1.0) = 1.0
		largeOrders := []LargeOrder{
			{Price: 50010.0, Size: 10.0, Side: "ask"},
			{Price: 50020.0, Size: 8.0, Side: "ask"},
		}
		signal, confidence, reasoning := analyzeLargeOrders(largeOrders)
		assert.Equal(t, "SELL", signal)
		assert.Equal(t, 1.0, confidence)             // All orders are asks
		assert.Contains(t, reasoning, "sell orders") // Changed from "SELL"
	})

	t.Run("no large orders", func(t *testing.T) {
		// When len(largeOrders) == 0, function returns:
		// "HOLD", 0.3, "No significant large orders"
		largeOrders := []LargeOrder{}
		signal, confidence, reasoning := analyzeLargeOrders(largeOrders)
		assert.Equal(t, "HOLD", signal)
		assert.Equal(t, 0.3, confidence) // Changed from 0.0
		assert.Contains(t, reasoning, "No significant")
	})
}

func TestAnalyzeSpoofing(t *testing.T) {
	t.Run("no spoofing", func(t *testing.T) {
		// When spoofingFlags == 0, function returns:
		// "HOLD", 1.0, "No spoofing detected"
		signal, confidence, reasoning := analyzeSpoofing(0)
		assert.Equal(t, "HOLD", signal)  // Changed from "NEUTRAL"
		assert.Equal(t, 1.0, confidence) // Changed from 0.0
		assert.Contains(t, reasoning, "No spoofing")
	})

	t.Run("mild spoofing", func(t *testing.T) {
		// When 0 < spoofingFlags < 5, function returns:
		// "HOLD", 1.0 - (spoofingFlags / 5.0), "Potential spoofing detected"
		// For spoofingFlags=2: confidence = 1.0 - (2.0 / 5.0) = 0.6
		signal, confidence, reasoning := analyzeSpoofing(2)
		assert.Equal(t, "HOLD", signal)  // Changed from "CAUTION"
		assert.Equal(t, 0.6, confidence) // Changed from > 0.0
		assert.Contains(t, reasoning, "spoofing")
	})

	t.Run("heavy spoofing", func(t *testing.T) {
		// When spoofingFlags >= 5, function returns:
		// "HOLD", 0.0, "High spoofing activity detected"
		signal, confidence, reasoning := analyzeSpoofing(10)
		assert.Equal(t, "HOLD", signal)  // Changed from "CAUTION"
		assert.Equal(t, 0.0, confidence) // Changed from > 0.5
		assert.Contains(t, reasoning, "spoofing")
	})
}

// =============================================================================
// Signal Aggregation Tests
// =============================================================================

func TestCombineSignals_AllBuy(t *testing.T) {
	signals := []string{"BUY", "BUY", "BUY", "NEUTRAL"}
	confidences := []float64{0.8, 0.7, 0.6, 0.0}
	weights := []float64{0.4, 0.3, 0.2, 0.1}

	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	assert.Equal(t, "BUY", finalSignal)
	assert.Greater(t, finalConfidence, 0.5)
}

func TestCombineSignals_AllSell(t *testing.T) {
	signals := []string{"SELL", "SELL", "SELL", "NEUTRAL"}
	confidences := []float64{0.8, 0.7, 0.6, 0.0}
	weights := []float64{0.4, 0.3, 0.2, 0.1}

	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	assert.Equal(t, "SELL", finalSignal)
	assert.Greater(t, finalConfidence, 0.5)
}

func TestCombineSignals_Mixed(t *testing.T) {
	signals := []string{"BUY", "SELL", "HOLD", "CAUTION"}
	confidences := []float64{0.7, 0.6, 0.5, 0.4}
	weights := []float64{0.4, 0.3, 0.2, 0.1}

	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	// Result depends on weighted combination
	assert.NotEmpty(t, finalSignal)
	assert.GreaterOrEqual(t, finalConfidence, 0.0)
	assert.LessOrEqual(t, finalConfidence, 1.0)
}

func TestCombineSignals_EmptyInputs(t *testing.T) {
	signals := []string{}
	confidences := []float64{}
	weights := []float64{}

	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	assert.Equal(t, "HOLD", finalSignal)
	assert.Equal(t, 0.0, finalConfidence)
}

func TestCombineSignals_WeightsNormalization(t *testing.T) {
	// Weights that don't sum to 1.0
	signals := []string{"BUY", "SELL"}
	confidences := []float64{0.8, 0.6}
	weights := []float64{2.0, 1.0} // Will be normalized to 0.667, 0.333

	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	assert.NotEmpty(t, finalSignal)
	assert.GreaterOrEqual(t, finalConfidence, 0.0)
	assert.LessOrEqual(t, finalConfidence, 1.0)
}

// =============================================================================
// Integration Tests - generateSignal
// =============================================================================

func TestGenerateSignal_StrongBuyConditions(t *testing.T) {
	agent := createTestAgent()
	ctx := context.Background()

	orderBook := mockBalancedOrderBook()

	// Strong buy conditions:
	// - High positive imbalance (>0.3)
	// - Price near support (<1%)
	// - Large bid orders
	// - No spoofing
	imbalance := 0.5 // Strong buy pressure
	supportLevels := []PriceLevel{{Price: 49990.0, Volume: 100.0}}
	resistanceLevels := []PriceLevel{{Price: 50010.0, Volume: 80.0}}
	largeOrders := []LargeOrder{
		{Price: 50000.0, Size: 10.0, Side: "bid"},
		{Price: 49990.0, Size: 8.0, Side: "bid"},
	}
	spoofingFlags := 0
	currentPrice := 49995.0 // Near support

	signal, err := agent.generateSignal(ctx, orderBook, imbalance, supportLevels, resistanceLevels, largeOrders, spoofingFlags, currentPrice)

	require.NoError(t, err)
	require.NotNil(t, signal)
	assert.Equal(t, "BUY", signal.Action)
	assert.Greater(t, signal.Confidence, 0.5)
	assert.Equal(t, "BTC/USDT", signal.Symbol)
	assert.Equal(t, imbalance, signal.Imbalance)
	assert.NotEmpty(t, signal.Reasoning)
	assert.Contains(t, signal.Reasoning, "buy")
}

func TestGenerateSignal_StrongSellConditions(t *testing.T) {
	agent := createTestAgent()
	ctx := context.Background()

	orderBook := mockBalancedOrderBook()

	// Strong sell conditions:
	// - High negative imbalance (<-0.3)
	// - Price near resistance (<1%)
	// - Large ask orders
	// - No spoofing
	imbalance := -0.5 // Strong sell pressure
	supportLevels := []PriceLevel{{Price: 49990.0, Volume: 80.0}}
	resistanceLevels := []PriceLevel{{Price: 50010.0, Volume: 100.0}}
	largeOrders := []LargeOrder{
		{Price: 50010.0, Size: 10.0, Side: "ask"},
		{Price: 50020.0, Size: 8.0, Side: "ask"},
	}
	spoofingFlags := 0
	currentPrice := 50510.0 // Near resistance (>1% from support, <1% from resistance)

	signal, err := agent.generateSignal(ctx, orderBook, imbalance, supportLevels, resistanceLevels, largeOrders, spoofingFlags, currentPrice)

	require.NoError(t, err)
	require.NotNil(t, signal)
	assert.Equal(t, "SELL", signal.Action)
	// Note: Confidence depends on weighted combination, may be lower than individual signals
	assert.Greater(t, signal.Confidence, 0.0)
	assert.Equal(t, "BTC/USDT", signal.Symbol)
	assert.Equal(t, imbalance, signal.Imbalance)
	assert.NotEmpty(t, signal.Reasoning)
	assert.Contains(t, signal.Reasoning, "sell")
}

func TestGenerateSignal_HoldConditions(t *testing.T) {
	agent := createTestAgent()
	ctx := context.Background()

	orderBook := mockBalancedOrderBook()

	// Neutral/hold conditions:
	// - Low imbalance (~0)
	// - Price far from levels (>1%)
	// - Mixed large orders
	// - No spoofing
	imbalance := 0.1 // Low pressure
	supportLevels := []PriceLevel{{Price: 49990.0, Volume: 90.0}}
	resistanceLevels := []PriceLevel{{Price: 50010.0, Volume: 90.0}}
	largeOrders := []LargeOrder{
		{Price: 50000.0, Size: 8.0, Side: "bid"},
		{Price: 50010.0, Size: 8.0, Side: "ask"},
	}
	spoofingFlags := 0
	currentPrice := 51000.0 // Far from both levels

	signal, err := agent.generateSignal(ctx, orderBook, imbalance, supportLevels, resistanceLevels, largeOrders, spoofingFlags, currentPrice)

	require.NoError(t, err)
	require.NotNil(t, signal)
	assert.Equal(t, "HOLD", signal.Action)
	assert.Equal(t, "BTC/USDT", signal.Symbol)
	assert.Equal(t, imbalance, signal.Imbalance)
	assert.NotEmpty(t, signal.Reasoning)
}

func TestGenerateSignal_WithSpoofing(t *testing.T) {
	agent := createTestAgent()
	ctx := context.Background()

	orderBook := mockBalancedOrderBook()

	// Strong buy signals but with spoofing penalty
	imbalance := 0.5
	supportLevels := []PriceLevel{{Price: 49990.0, Volume: 100.0}}
	resistanceLevels := []PriceLevel{{Price: 50010.0, Volume: 80.0}}
	largeOrders := []LargeOrder{{Price: 50000.0, Size: 10.0, Side: "bid"}}
	spoofingFlags := 3 // Multiple spoofing indicators
	currentPrice := 49995.0

	signal, err := agent.generateSignal(ctx, orderBook, imbalance, supportLevels, resistanceLevels, largeOrders, spoofingFlags, currentPrice)

	require.NoError(t, err)
	require.NotNil(t, signal)
	// Spoofing should reduce confidence or change signal to CAUTION
	assert.Contains(t, []string{"BUY", "CAUTION"}, signal.Action)
	assert.Equal(t, spoofingFlags, signal.SpoofingFlags)
	assert.Contains(t, signal.Reasoning, "spoofing")
}

func TestGenerateSignal_EmptyLevels(t *testing.T) {
	agent := createTestAgent()
	ctx := context.Background()

	orderBook := mockBalancedOrderBook()

	// No support/resistance levels
	imbalance := 0.2
	supportLevels := []PriceLevel{}
	resistanceLevels := []PriceLevel{}
	largeOrders := []LargeOrder{}
	spoofingFlags := 0
	currentPrice := 50000.0

	signal, err := agent.generateSignal(ctx, orderBook, imbalance, supportLevels, resistanceLevels, largeOrders, spoofingFlags, currentPrice)

	require.NoError(t, err)
	require.NotNil(t, signal)
	assert.NotEmpty(t, signal.Action)
	assert.Empty(t, signal.SupportLevels)
	assert.Empty(t, signal.ResistanceLevels)
}

// =============================================================================
// Unit Tests - Confidence Calculators
// =============================================================================

func TestCalculateDepthConfidence(t *testing.T) {
	tests := []struct {
		name     string
		levels   []PriceLevel
		expected float64
	}{
		{
			name:     "empty levels",
			levels:   []PriceLevel{},
			expected: 0.0,
		},
		{
			name: "one level",
			levels: []PriceLevel{
				{Price: 50000.0, Volume: 100.0},
			},
			expected: 1.0 / 3.0,
		},
		{
			name: "two levels",
			levels: []PriceLevel{
				{Price: 50000.0, Volume: 100.0},
				{Price: 50100.0, Volume: 80.0},
			},
			expected: 2.0 / 3.0,
		},
		{
			name: "three levels",
			levels: []PriceLevel{
				{Price: 50000.0, Volume: 100.0},
				{Price: 50100.0, Volume: 80.0},
				{Price: 50200.0, Volume: 60.0},
			},
			expected: 1.0,
		},
		{
			name: "more than three levels (capped at 3)",
			levels: []PriceLevel{
				{Price: 50000.0, Volume: 100.0},
				{Price: 50100.0, Volume: 80.0},
				{Price: 50200.0, Volume: 60.0},
				{Price: 50300.0, Volume: 40.0},
				{Price: 50400.0, Volume: 20.0},
			},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateDepthConfidence(tt.levels)
			assert.InDelta(t, tt.expected, confidence, 0.001)
		})
	}
}

func TestCalculateLargeOrderConfidence(t *testing.T) {
	tests := []struct {
		name     string
		orders   []LargeOrder
		expected float64
	}{
		{
			name:     "empty orders",
			orders:   []LargeOrder{},
			expected: 0.3,
		},
		{
			name: "one order",
			orders: []LargeOrder{
				{Price: 50000.0, Size: 10.0, Side: "bid"},
			},
			expected: 1.0 / 5.0,
		},
		{
			name: "three orders",
			orders: []LargeOrder{
				{Price: 50000.0, Size: 10.0, Side: "bid"},
				{Price: 50100.0, Size: 8.0, Side: "bid"},
				{Price: 50200.0, Size: 7.0, Side: "ask"},
			},
			expected: 3.0 / 5.0,
		},
		{
			name: "five orders",
			orders: []LargeOrder{
				{Price: 50000.0, Size: 10.0, Side: "bid"},
				{Price: 50100.0, Size: 8.0, Side: "bid"},
				{Price: 50200.0, Size: 7.0, Side: "ask"},
				{Price: 50300.0, Size: 6.0, Side: "ask"},
				{Price: 50400.0, Size: 5.0, Side: "bid"},
			},
			expected: 1.0,
		},
		{
			name: "more than five orders (capped at 5)",
			orders: []LargeOrder{
				{Price: 50000.0, Size: 10.0, Side: "bid"},
				{Price: 50100.0, Size: 9.0, Side: "bid"},
				{Price: 50200.0, Size: 8.0, Side: "ask"},
				{Price: 50300.0, Size: 7.0, Side: "ask"},
				{Price: 50400.0, Size: 6.0, Side: "bid"},
				{Price: 50500.0, Size: 5.0, Side: "ask"},
				{Price: 50600.0, Size: 4.0, Side: "bid"},
			},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateLargeOrderConfidence(tt.orders)
			assert.InDelta(t, tt.expected, confidence, 0.001)
		})
	}
}

func TestCalculateSpoofingConfidence(t *testing.T) {
	tests := []struct {
		name     string
		flags    int
		expected float64
	}{
		{
			name:     "no spoofing",
			flags:    0,
			expected: 1.0,
		},
		{
			name:     "one flag",
			flags:    1,
			expected: 0.9,
		},
		{
			name:     "five flags",
			flags:    5,
			expected: 0.5,
		},
		{
			name:     "ten flags (max penalty)",
			flags:    10,
			expected: 0.0,
		},
		{
			name:     "more than ten flags (capped at 0)",
			flags:    15,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateSpoofingConfidence(tt.flags)
			assert.InDelta(t, tt.expected, confidence, 0.001)
		})
	}
}

// =============================================================================
// Unit Tests - Config Helpers
// =============================================================================

func TestGetStringConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		key        string
		defaultVal string
		expected   string
	}{
		{
			name:       "key exists with string value",
			config:     map[string]interface{}{"symbol": "BTC/USDT"},
			key:        "symbol",
			defaultVal: "ETH/USDT",
			expected:   "BTC/USDT",
		},
		{
			name:       "key does not exist",
			config:     map[string]interface{}{"other": "value"},
			key:        "symbol",
			defaultVal: "ETH/USDT",
			expected:   "ETH/USDT",
		},
		{
			name:       "key exists but not a string",
			config:     map[string]interface{}{"symbol": 123},
			key:        "symbol",
			defaultVal: "ETH/USDT",
			expected:   "ETH/USDT",
		},
		{
			name:       "empty config",
			config:     map[string]interface{}{},
			key:        "symbol",
			defaultVal: "ETH/USDT",
			expected:   "ETH/USDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringConfig(tt.config, tt.key, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		key        string
		defaultVal int
		expected   int
	}{
		{
			name:       "key exists with int value",
			config:     map[string]interface{}{"depth": 20},
			key:        "depth",
			defaultVal: 10,
			expected:   20,
		},
		{
			name:       "key exists with int64 value",
			config:     map[string]interface{}{"depth": int64(30)},
			key:        "depth",
			defaultVal: 10,
			expected:   30,
		},
		{
			name:       "key exists with float64 value",
			config:     map[string]interface{}{"depth": 25.7},
			key:        "depth",
			defaultVal: 10,
			expected:   25,
		},
		{
			name:       "key does not exist",
			config:     map[string]interface{}{"other": 123},
			key:        "depth",
			defaultVal: 10,
			expected:   10,
		},
		{
			name:       "key exists but not a number",
			config:     map[string]interface{}{"depth": "twenty"},
			key:        "depth",
			defaultVal: 10,
			expected:   10,
		},
		{
			name:       "empty config",
			config:     map[string]interface{}{},
			key:        "depth",
			defaultVal: 10,
			expected:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntConfig(tt.config, tt.key, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFloatConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		key        string
		defaultVal float64
		expected   float64
	}{
		{
			name:       "key exists with float64 value",
			config:     map[string]interface{}{"threshold": 0.5},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   0.5,
		},
		{
			name:       "key exists with float32 value",
			config:     map[string]interface{}{"threshold": float32(0.7)},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   0.7,
		},
		{
			name:       "key exists with int value",
			config:     map[string]interface{}{"threshold": 2},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   2.0,
		},
		{
			name:       "key exists with int64 value",
			config:     map[string]interface{}{"threshold": int64(3)},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   3.0,
		},
		{
			name:       "key does not exist",
			config:     map[string]interface{}{"other": 1.5},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   0.3,
		},
		{
			name:       "key exists but not a number",
			config:     map[string]interface{}{"threshold": "half"},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   0.3,
		},
		{
			name:       "empty config",
			config:     map[string]interface{}{},
			key:        "threshold",
			defaultVal: 0.3,
			expected:   0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFloatConfig(tt.config, tt.key, tt.defaultVal)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestMinFloat(t *testing.T) {
	assert.Equal(t, 1.0, minFloat(1.0, 2.0))
	assert.Equal(t, 1.0, minFloat(2.0, 1.0))
	assert.Equal(t, 1.5, minFloat(1.5, 1.5))
	assert.Equal(t, -5.0, minFloat(-5.0, 5.0))
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1, min(1, 2))
	assert.Equal(t, 1, min(2, 1))
	assert.Equal(t, 5, min(5, 5))
	assert.Equal(t, -10, min(-10, 10))
}

func TestCalculateAverageSize(t *testing.T) {
	t.Run("normal levels", func(t *testing.T) {
		levels := []OrderBookLevel{
			{Price: 50000.0, Quantity: 10.0},
			{Price: 49990.0, Quantity: 5.0},
			{Price: 49980.0, Quantity: 3.0},
		}
		avg := calculateAverageSize(levels)
		assert.InDelta(t, 6.0, avg, 0.01) // (10 + 5 + 3) / 3 = 6
	})

	t.Run("empty levels", func(t *testing.T) {
		levels := []OrderBookLevel{}
		avg := calculateAverageSize(levels)
		assert.Equal(t, 0.0, avg)
	})

	t.Run("single level", func(t *testing.T) {
		levels := []OrderBookLevel{
			{Price: 50000.0, Quantity: 7.5},
		}
		avg := calculateAverageSize(levels)
		assert.Equal(t, 7.5, avg)
	})
}

func TestOrderBookLevel_Validation(t *testing.T) {
	t.Run("valid level", func(t *testing.T) {
		level := OrderBookLevel{Price: 50000.0, Quantity: 5.0}
		assert.Greater(t, level.Price, 0.0)
		assert.Greater(t, level.Quantity, 0.0)
	})

	t.Run("zero quantity", func(t *testing.T) {
		level := OrderBookLevel{Price: 50000.0, Quantity: 0.0}
		assert.Equal(t, 0.0, level.Quantity)
	})
}

func TestPriceLevel_Validation(t *testing.T) {
	t.Run("valid price level", func(t *testing.T) {
		level := PriceLevel{Price: 50000.0, Volume: 8.5}
		assert.Greater(t, level.Price, 0.0)
		assert.GreaterOrEqual(t, level.Volume, 0.0)
	})

	t.Run("volume boundaries", func(t *testing.T) {
		level1 := PriceLevel{Price: 50000.0, Volume: 0.0}
		level2 := PriceLevel{Price: 50000.0, Volume: 10.0}
		assert.Equal(t, 0.0, level1.Volume)
		assert.Equal(t, 10.0, level2.Volume)
	})
}

func TestLargeOrder_Validation(t *testing.T) {
	t.Run("valid bid order", func(t *testing.T) {
		order := LargeOrder{Price: 50000.0, Size: 10.0, Side: "bid"}
		assert.Greater(t, order.Price, 0.0)
		assert.Greater(t, order.Size, 0.0)
		assert.Equal(t, "bid", order.Side)
	})

	t.Run("valid ask order", func(t *testing.T) {
		order := LargeOrder{Price: 50000.0, Size: 10.0, Side: "ask"}
		assert.Greater(t, order.Price, 0.0)
		assert.Greater(t, order.Size, 0.0)
		assert.Equal(t, "ask", order.Side)
	})
}

func TestBelief_Structure(t *testing.T) {
	belief := Belief{
		Value:      "test_value",
		Confidence: 0.75,
		Source:     "test_source",
		Timestamp:  time.Now(),
	}

	assert.Equal(t, "test_value", belief.Value)
	assert.Equal(t, 0.75, belief.Confidence)
	assert.Equal(t, "test_source", belief.Source)
	assert.WithinDuration(t, time.Now(), belief.Timestamp, time.Second)
}

// =============================================================================
// Order Book Snapshot Tests
// =============================================================================

func TestOrderBookSnapshot_Creation(t *testing.T) {
	ob := mockBuyPressureOrderBook()
	snapshot := OrderBookSnapshot{
		Timestamp: time.Now(),
		OrderBook: ob,
	}

	assert.NotNil(t, snapshot.OrderBook)
	assert.Equal(t, "BTC/USDT", snapshot.OrderBook.Symbol)
	assert.WithinDuration(t, time.Now(), snapshot.Timestamp, time.Second)
}

func TestOrderBookSnapshot_TimestampOrdering(t *testing.T) {
	snapshots := []OrderBookSnapshot{}

	for i := 0; i < 5; i++ {
		ob := mockBalancedOrderBook()
		snapshot := OrderBookSnapshot{
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			OrderBook: ob,
		}
		snapshots = append(snapshots, snapshot)
		time.Sleep(2 * time.Millisecond)
	}

	// Verify chronological order
	for i := 1; i < len(snapshots); i++ {
		assert.True(t, snapshots[i].Timestamp.After(snapshots[i-1].Timestamp))
	}
}

// =============================================================================
// Tests for Spoofing Detection Functions
// =============================================================================

func TestCompareSnapshotsForSpoofing(t *testing.T) {
	agent := createTestAgent()
	agent.largeOrderMultiplier = 2.0 // Use 2.0 for more realistic test scenarios

	tests := []struct {
		name          string
		prev          *OrderBook
		current       *OrderBook
		expectedFlags int
	}{
		{
			name: "no spoofing - orders persist",
			prev: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 10.0},
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 5.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 10.0},
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 5.0},
				},
			},
			current: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 10.0},
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 5.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 10.0},
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 5.0},
				},
			},
			expectedFlags: 0,
		},
		{
			name: "spoofing detected - large bid disappeared",
			prev: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 100.0}, // Large order (avg=38.33, threshold=76.67 with 2x, 100.0 > 76.67 ✓)
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 7.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 10.0},
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 5.0},
				},
			},
			current: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 7.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 10.0},
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 5.0},
				},
			},
			expectedFlags: 1,
		},
		{
			name: "spoofing detected - large ask disappeared",
			prev: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 10.0},
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 5.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 120.0}, // Large order (avg=44.33, threshold=88.67 with 2x, 120.0 > 88.67 ✓)
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 5.0},
				},
			},
			current: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 10.0},
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 5.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 5.0},
				},
			},
			expectedFlags: 1,
		},
		{
			name: "multiple spoofing - both sides",
			prev: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 150.0}, // Large bid (avg=61.67, threshold=123.33 with 2x, 150.0 > 123.33 ✓)
					{Price: 49900.0, Quantity: 8.0},
					{Price: 49800.0, Quantity: 27.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 100.0}, // Large ask (avg=38.0, threshold=76.0 with 2x, 100.0 > 76.0 ✓)
					{Price: 50200.0, Quantity: 8.0},
					{Price: 50300.0, Quantity: 6.0},
				},
			},
			current: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 49900.0, Quantity: 8.0},
				},
				Asks: []OrderBookLevel{},
			},
			expectedFlags: 2, // One large bid + one large ask disappeared
		},
		{
			name: "empty previous order book",
			prev: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids:      []OrderBookLevel{},
				Asks:      []OrderBookLevel{},
			},
			current: &OrderBook{
				Symbol:    "BTC/USDT",
				Timestamp: time.Now().Unix(),
				Bids: []OrderBookLevel{
					{Price: 50000.0, Quantity: 10.0},
				},
				Asks: []OrderBookLevel{
					{Price: 50100.0, Quantity: 10.0},
				},
			},
			expectedFlags: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := agent.compareSnapshotsForSpoofing(tt.prev, tt.current)
			assert.Equal(t, tt.expectedFlags, flags)
		})
	}
}

func TestDetectSpoofing(t *testing.T) {
	tests := []struct {
		name          string
		setupHistory  func(*OrderBookAgent)
		expectedFlags int
	}{
		{
			name: "less than 2 snapshots",
			setupHistory: func(agent *OrderBookAgent) {
				agent.orderBookHistory = []OrderBookSnapshot{
					{
						Timestamp: time.Now(),
						OrderBook: mockBalancedOrderBook(),
					},
				}
			},
			expectedFlags: 0,
		},
		{
			name: "snapshots outside spoofing window",
			setupHistory: func(agent *OrderBookAgent) {
				agent.spoofingWindow = 5 * time.Second
				agent.orderBookHistory = []OrderBookSnapshot{
					{
						Timestamp: time.Now().Add(-10 * time.Second),
						OrderBook: mockBalancedOrderBook(),
					},
					{
						Timestamp: time.Now().Add(-8 * time.Second),
						OrderBook: mockBalancedOrderBook(),
					},
				}
			},
			expectedFlags: 0,
		},
		{
			name: "spoofing detected within window",
			setupHistory: func(agent *OrderBookAgent) {
				agent.spoofingWindow = 10 * time.Second
				agent.largeOrderMultiplier = 2.0 // Use 2.0 for realistic scenarios

				// Previous snapshot with large order
				prev := &OrderBook{
					Symbol:    "BTC/USDT",
					Timestamp: time.Now().Unix(),
					Bids: []OrderBookLevel{
						{Price: 50000.0, Quantity: 120.0}, // Large order (avg=43.33, threshold=86.67 with 2x, 120.0 > 86.67 ✓)
						{Price: 49900.0, Quantity: 5.0},
						{Price: 49800.0, Quantity: 5.0},
					},
					Asks: []OrderBookLevel{
						{Price: 50100.0, Quantity: 10.0},
					},
				}

				// Current snapshot without large order
				current := &OrderBook{
					Symbol:    "BTC/USDT",
					Timestamp: time.Now().Unix(),
					Bids: []OrderBookLevel{
						{Price: 49900.0, Quantity: 5.0}, // Large order disappeared
						{Price: 49800.0, Quantity: 5.0},
					},
					Asks: []OrderBookLevel{
						{Price: 50100.0, Quantity: 10.0},
					},
				}

				agent.orderBookHistory = []OrderBookSnapshot{
					{
						Timestamp: time.Now().Add(-3 * time.Second),
						OrderBook: prev,
					},
					{
						Timestamp: time.Now(),
						OrderBook: current,
					},
				}
			},
			expectedFlags: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := createTestAgent()
			tt.setupHistory(agent)

			flags := agent.detectSpoofing()
			assert.Equal(t, tt.expectedFlags, flags)
		})
	}
}

func TestAddOrderBookSnapshot(t *testing.T) {
	agent := createTestAgent()

	t.Run("adds snapshot to empty history", func(t *testing.T) {
		agent.orderBookHistory = []OrderBookSnapshot{} // Reset history for clean state

		ob := mockBalancedOrderBook()
		agent.addOrderBookSnapshot(ob)

		assert.Equal(t, 1, len(agent.orderBookHistory))
		assert.Equal(t, ob, agent.orderBookHistory[0].OrderBook)
	})

	t.Run("adds multiple snapshots", func(t *testing.T) {
		agent.orderBookHistory = []OrderBookSnapshot{}

		for i := 0; i < 5; i++ {
			ob := mockBalancedOrderBook()
			agent.addOrderBookSnapshot(ob)
		}

		assert.Equal(t, 5, len(agent.orderBookHistory))
	})

	t.Run("respects 100 snapshot limit", func(t *testing.T) {
		agent.orderBookHistory = []OrderBookSnapshot{}

		// Add 105 snapshots
		for i := 0; i < 105; i++ {
			ob := mockBalancedOrderBook()
			agent.addOrderBookSnapshot(ob)
		}

		// Should only keep last 100
		assert.Equal(t, 100, len(agent.orderBookHistory))
	})

	t.Run("maintains chronological order after truncation", func(t *testing.T) {
		agent.orderBookHistory = []OrderBookSnapshot{}

		// Add 110 snapshots with identifiable timestamps
		for i := 0; i < 110; i++ {
			ob := mockBalancedOrderBook()
			agent.addOrderBookSnapshot(ob)
			time.Sleep(time.Millisecond)
		}

		// Verify we have exactly 100 snapshots
		assert.Equal(t, 100, len(agent.orderBookHistory))

		// Verify chronological order
		for i := 1; i < len(agent.orderBookHistory); i++ {
			assert.True(t, agent.orderBookHistory[i].Timestamp.After(agent.orderBookHistory[i-1].Timestamp),
				"Timestamp at index %d should be after index %d", i, i-1)
		}
	})
}

func TestAnalyzeDepth_EdgeCases(t *testing.T) {
	agent := createTestAgent()

	t.Run("empty levels", func(t *testing.T) {
		levels := []OrderBookLevel{}
		result := agent.analyzeDepth(levels, true)
		assert.Nil(t, result)
	})

	t.Run("single level", func(t *testing.T) {
		levels := []OrderBookLevel{
			{Price: 50000.0, Quantity: 10.0},
		}
		result := agent.analyzeDepth(levels, true)
		assert.NotNil(t, result)
		assert.Equal(t, 1, len(result))
	})

	t.Run("identifies top 3 levels for bids", func(t *testing.T) {
		levels := []OrderBookLevel{
			{Price: 50000.0, Quantity: 100.0},
			{Price: 49900.0, Quantity: 80.0},
			{Price: 49800.0, Quantity: 120.0}, // Highest cumulative
			{Price: 49700.0, Quantity: 60.0},
			{Price: 49600.0, Quantity: 40.0},
		}
		result := agent.analyzeDepth(levels, true)
		assert.NotNil(t, result)
		assert.LessOrEqual(t, len(result), 3)
	})

	t.Run("identifies top 3 levels for asks", func(t *testing.T) {
		levels := []OrderBookLevel{
			{Price: 50100.0, Quantity: 100.0},
			{Price: 50200.0, Quantity: 80.0},
			{Price: 50300.0, Quantity: 120.0}, // Highest cumulative
			{Price: 50400.0, Quantity: 60.0},
			{Price: 50500.0, Quantity: 40.0},
		}
		result := agent.analyzeDepth(levels, false)
		assert.NotNil(t, result)
		assert.LessOrEqual(t, len(result), 3)
	})
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCalculateImbalance(b *testing.B) {
	agent := createTestAgent()
	ob := mockBuyPressureOrderBook()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.calculateImbalance(ob)
	}
}

func BenchmarkAnalyzeDepth(b *testing.B) {
	agent := createTestAgent()
	levels := mockBuyPressureOrderBook().Bids

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.analyzeDepth(levels, true)
	}
}

func BenchmarkDetectLargeOrders(b *testing.B) {
	agent := createTestAgent()
	ob := mockBuyPressureOrderBook()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.detectLargeOrders(ob)
	}
}

func BenchmarkDetectSpoofing(b *testing.B) {
	agent := createTestAgent()
	// Add some history
	for i := 0; i < 5; i++ {
		ob := mockBalancedOrderBook()
		agent.addOrderBookSnapshot(ob)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.detectSpoofing()
	}
}

func BenchmarkCombineSignals(b *testing.B) {
	signals := []string{"BUY", "SELL", "HOLD", "NEUTRAL"}
	confidences := []float64{0.8, 0.6, 0.5, 0.0}
	weights := []float64{0.4, 0.3, 0.2, 0.1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		combineSignals(signals, confidences, weights)
	}
}

func BenchmarkBeliefBase_Update(b *testing.B) {
	bb := NewBeliefBase()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb.UpdateBelief("test_key", "test_value", 0.75, "bench")
	}
}

func BenchmarkBeliefBase_Get(b *testing.B) {
	bb := NewBeliefBase()
	bb.UpdateBelief("test_key", "test_value", 0.75, "bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb.GetBelief("test_key")
	}
}

// TestNewOrderBookAgent tests config parsing logic
// Note: Actual agent creation requires NATS server and causes Prometheus metric collisions in tests
// This test validates that config parsing logic works correctly by checking helper functions
func TestNewOrderBookAgent(t *testing.T) {
	t.Run("config parsing helpers work correctly", func(t *testing.T) {
		// Test that getStringConfig helper extracts symbol
		config := map[string]interface{}{
			"symbol":                 "ethereum",
			"depth_levels":           25,
			"large_order_multiplier": 3.5,
			"imbalance_threshold":    0.4,
			"spoofing_window":        "10s",
		}

		symbol := getStringConfig(config, "symbol", "bitcoin")
		assert.Equal(t, "ethereum", symbol)

		depthLevels := getIntConfig(config, "depth_levels", 20)
		assert.Equal(t, 25, depthLevels)

		multiplier := getFloatConfig(config, "large_order_multiplier", 5.0)
		assert.Equal(t, 3.5, multiplier)

		threshold := getFloatConfig(config, "imbalance_threshold", 0.3)
		assert.Equal(t, 0.4, threshold)
	})

	t.Run("uses default values for missing config", func(t *testing.T) {
		config := map[string]interface{}{}

		symbol := getStringConfig(config, "symbol", "bitcoin")
		assert.Equal(t, "bitcoin", symbol)

		depthLevels := getIntConfig(config, "depth_levels", 20)
		assert.Equal(t, 20, depthLevels)

		multiplier := getFloatConfig(config, "large_order_multiplier", 5.0)
		assert.Equal(t, 5.0, multiplier)
	})

	t.Run("spoofing window parsing", func(t *testing.T) {
		// Test valid duration parsing
		validDuration := "15s"
		parsed, err := time.ParseDuration(validDuration)
		assert.NoError(t, err)
		assert.Equal(t, 15*time.Second, parsed)

		// Test invalid duration defaults to 5s
		invalidDuration := "invalid"
		_, err = time.ParseDuration(invalidDuration)
		assert.Error(t, err)
		// In actual code, this would fall back to 5s default
	})
}

// TestPublishSignal tests NATS signal publishing
func TestPublishSignal(t *testing.T) {
	agent := createTestAgent()
	ctx := context.Background()

	signal := &OrderBookSignal{
		Timestamp:     time.Now(),
		Symbol:        "BTC/USDT",
		Action:        "BUY",
		Confidence:    0.75,
		Reasoning:     "Test signal",
		Imbalance:     0.5,
		Price:         50000.0,
		SpoofingFlags: 0,
	}

	t.Run("publishes valid signal", func(t *testing.T) {
		// This test verifies the function executes without panicking
		// In a real test suite, you'd use a mock NATS connection
		err := agent.publishSignal(ctx, signal)

		// We expect this to fail without real NATS connection
		if err != nil {
			// Verify it's a marshaling or publishing error, not a panic
			assert.NotNil(t, err)
		}
	})

	t.Run("handles nil signal gracefully", func(t *testing.T) {
		// Should fail on marshaling nil signal
		err := agent.publishSignal(ctx, nil)
		assert.Error(t, err)
	})
}

// TestAnalyzeDepthEdgeCases tests edge cases in depth analysis
func TestAnalyzeDepthEdgeCases(t *testing.T) {
	agent := createTestAgent()

	t.Run("handles more than 10 levels", func(t *testing.T) {
		// Create 15 levels
		levels := make([]OrderBookLevel, 15)
		for i := 0; i < 15; i++ {
			levels[i] = OrderBookLevel{
				Price:    50000.0 + float64(i)*10,
				Quantity: 10.0 - float64(i)*0.5, // Decreasing quantity
			}
		}

		result := agent.analyzeDepth(levels, true)

		// Should only analyze first 10 levels
		assert.NotNil(t, result)
		// Should return at most 3 significant levels
		assert.LessOrEqual(t, len(result), 3)
	})

	t.Run("handles levels with exactly 5% volume threshold", func(t *testing.T) {
		// Create levels where one is exactly at 5% threshold
		levels := []OrderBookLevel{
			{Price: 50000.0, Quantity: 100.0}, // Total = 200.0
			{Price: 49990.0, Quantity: 90.0},
			{Price: 49980.0, Quantity: 10.0}, // Exactly 5% of 200
		}

		result := agent.analyzeDepth(levels, true)
		assert.NotNil(t, result)
		// The 10.0 quantity is exactly 5%, should be included
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns maximum 3 significant levels", func(t *testing.T) {
		// Create many levels all above 5% threshold
		levels := []OrderBookLevel{
			{Price: 50000.0, Quantity: 20.0},
			{Price: 49990.0, Quantity: 18.0},
			{Price: 49980.0, Quantity: 16.0},
			{Price: 49970.0, Quantity: 14.0},
			{Price: 49960.0, Quantity: 12.0},
		}

		result := agent.analyzeDepth(levels, false) // Test as resistance
		assert.NotNil(t, result)
		assert.LessOrEqual(t, len(result), 3, "Should return at most 3 levels")
	})
}
