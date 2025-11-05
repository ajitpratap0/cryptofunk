// Backtest Engine Unit Tests
package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ENGINE TESTS
// ============================================================================

func TestNewEngine(t *testing.T) {
	config := BacktestConfig{
		InitialCapital: 10000.0,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000.0,
		MaxPositions:   5,
	}

	engine := NewEngine(config)

	assert.Equal(t, 10000.0, engine.InitialCapital)
	assert.Equal(t, 10000.0, engine.Cash)
	assert.Equal(t, 0.001, engine.CommissionRate)
	assert.Equal(t, "fixed", engine.PositionSizing)
	assert.Equal(t, 1000.0, engine.PositionSize)
	assert.Equal(t, 5, engine.MaxPositions)
	assert.NotNil(t, engine.Positions)
	assert.NotNil(t, engine.Trades)
	assert.NotNil(t, engine.Data)
}

func TestLoadHistoricalData(t *testing.T) {
	engine := NewEngine(BacktestConfig{InitialCapital: 10000.0})

	// Create test candlesticks
	candlesticks := []*Candlestick{
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Close: 50000},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 51000},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 49000},
	}

	err := engine.LoadHistoricalData("BTC", candlesticks)
	require.NoError(t, err)

	assert.Len(t, engine.Data["BTC"], 3)
	assert.Equal(t, 0, engine.CurrentIndex["BTC"])
}

func TestLoadHistoricalDataSorting(t *testing.T) {
	engine := NewEngine(BacktestConfig{InitialCapital: 10000.0})

	// Candlesticks in wrong order
	candlesticks := []*Candlestick{
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 49000},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Close: 50000},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 51000},
	}

	err := engine.LoadHistoricalData("BTC", candlesticks)
	require.NoError(t, err)

	// Should be sorted by timestamp
	data := engine.Data["BTC"]
	assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), data[0].Timestamp)
	assert.Equal(t, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), data[1].Timestamp)
	assert.Equal(t, time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), data[2].Timestamp)
}

func TestGetCurrentCandle(t *testing.T) {
	engine := createTestEngine()

	candle, err := engine.GetCurrentCandle("BTC")
	require.NoError(t, err)
	assert.Equal(t, 50000.0, candle.Close)
}

func TestGetHistoricalCandles(t *testing.T) {
	engine := createTestEngine()

	// Advance a few steps
	engine.CurrentIndex["BTC"] = 3

	candles, err := engine.GetHistoricalCandles("BTC", 2)
	require.NoError(t, err)
	assert.Len(t, candles, 2)
	// GetHistoricalCandles returns candles BEFORE current index
	// Current index is 3, so we get indices 1 and 2
	assert.Equal(t, 51000.0, candles[0].Close) // Index 1
	assert.Equal(t, 49000.0, candles[1].Close) // Index 2
}

func TestStep(t *testing.T) {
	engine := createTestEngine()
	ctx := context.Background()

	// First step
	hasMore, err := engine.Step(ctx)
	require.NoError(t, err)
	assert.True(t, hasMore)
	assert.Equal(t, 1, engine.CurrentIndex["BTC"])

	// Verify equity point recorded
	assert.Len(t, engine.EquityCurve, 1)
}

func TestExecuteBuy(t *testing.T) {
	engine := createTestEngine()

	signal := &Signal{
		Symbol:     "BTC",
		Side:       "BUY",
		Confidence: 0.8,
		Reasoning:  "Test buy",
		Agent:      "test",
	}

	err := engine.ExecuteSignal(signal)
	require.NoError(t, err)

	// Should have opened a position
	position, exists := engine.Positions["BTC"]
	require.True(t, exists)
	assert.Equal(t, "LONG", position.Side)
	assert.Equal(t, 50000.0, position.EntryPrice)

	// Should have less cash
	assert.Less(t, engine.Cash, 10000.0)

	// Should have recorded a trade
	assert.Len(t, engine.Trades, 1)
	assert.Equal(t, "BUY", engine.Trades[0].Side)
}

func TestExecuteSell(t *testing.T) {
	engine := createTestEngine()

	// First buy
	buySignal := &Signal{
		Symbol:     "BTC",
		Side:       "BUY",
		Confidence: 0.8,
		Reasoning:  "Test buy",
		Agent:      "test",
	}
	_ = engine.ExecuteSignal(buySignal) // Test setup - error handled by test

	// Advance to next candle (price increased)
	_, _ = engine.Step(context.Background()) // Test setup - error handled by test

	// Now sell
	sellSignal := &Signal{
		Symbol:     "BTC",
		Side:       "SELL",
		Confidence: 0.8,
		Reasoning:  "Test sell",
		Agent:      "test",
	}
	err := engine.ExecuteSignal(sellSignal)
	require.NoError(t, err)

	// Should have closed the position
	_, exists := engine.Positions["BTC"]
	assert.False(t, exists)

	// Should have more cash than after buy
	assert.Greater(t, engine.Cash, 9000.0)

	// Should have recorded sell trade
	assert.Len(t, engine.Trades, 2)
	assert.Equal(t, "SELL", engine.Trades[1].Side)

	// Should have a closed position
	assert.Len(t, engine.ClosedPositions, 1)
	assert.Greater(t, engine.ClosedPositions[0].RealizedPL, 0.0) // Profitable (price went up)
}

func TestMaxPositionsLimit(t *testing.T) {
	config := BacktestConfig{
		InitialCapital: 100000.0,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   10000.0,
		MaxPositions:   2, // Only 2 concurrent positions
	}
	engine := NewEngine(config)

	// Load data for 3 symbols
	for _, symbol := range []string{"BTC", "ETH", "SOL"} {
		candlesticks := []*Candlestick{
			{Symbol: symbol, Timestamp: time.Now(), Close: 1000},
		}
		_ = engine.LoadHistoricalData(symbol, candlesticks) // Test setup - error handled by test
	}

	// Try to buy all 3
	for _, symbol := range []string{"BTC", "ETH", "SOL"} {
		signal := &Signal{
			Symbol:     symbol,
			Side:       "BUY",
			Confidence: 0.8,
			Agent:      "test",
		}
		_ = engine.ExecuteSignal(signal) // Test setup - error handled by test
	}

	// Should only have 2 positions (max limit)
	assert.Len(t, engine.Positions, 2)
}

func TestInsufficientCash(t *testing.T) {
	config := BacktestConfig{
		InitialCapital: 100.0, // Very small capital
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   10000.0, // Trying to buy more than we have
		MaxPositions:   5,
	}
	engine := NewEngine(config)

	candlesticks := []*Candlestick{
		{Symbol: "BTC", Timestamp: time.Now(), Close: 50000},
	}
	_ = engine.LoadHistoricalData("BTC", candlesticks) // Test setup - error handled by test

	signal := &Signal{
		Symbol:     "BTC",
		Side:       "BUY",
		Confidence: 0.8,
		Agent:      "test",
	}
	err := engine.ExecuteSignal(signal)

	// Should not error, just skip the trade
	assert.NoError(t, err)

	// Should have no positions
	assert.Len(t, engine.Positions, 0)

	// Cash unchanged
	assert.Equal(t, 100.0, engine.Cash)
}

func TestPositionSizing(t *testing.T) {
	t.Run("fixed sizing", func(t *testing.T) {
		engine := NewEngine(BacktestConfig{
			PositionSizing: "fixed",
			PositionSize:   1000.0,
		})

		quantity := engine.calculatePositionSize(50.0) // Price = $50
		assert.InDelta(t, 20.0, quantity, 0.01)        // $1000 / $50 = 20 units
	})

	t.Run("percent sizing", func(t *testing.T) {
		engine := NewEngine(BacktestConfig{
			InitialCapital: 10000.0,
			PositionSizing: "percent",
			PositionSize:   0.1, // 10% of equity
		})

		quantity := engine.calculatePositionSize(100.0) // Price = $100
		assert.InDelta(t, 10.0, quantity, 0.01)         // 10% of $10000 = $1000 / $100 = 10 units
	})

	t.Run("kelly sizing", func(t *testing.T) {
		engine := NewEngine(BacktestConfig{
			InitialCapital: 10000.0,
			PositionSizing: "kelly",
		})

		quantity := engine.calculatePositionSize(100.0)
		assert.Greater(t, quantity, 0.0)
		assert.Less(t, quantity, 100.0) // Should be reasonable
	})
}

func TestGetCurrentEquity(t *testing.T) {
	engine := createTestEngine()

	// Initially, equity equals cash
	assert.Equal(t, 10000.0, engine.GetCurrentEquity())

	// Buy a position
	signal := &Signal{Symbol: "BTC", Side: "BUY", Agent: "test"}
	_ = engine.ExecuteSignal(signal) // Test setup - error handled by test

	// Equity should include position value
	equity := engine.GetCurrentEquity()
	assert.Greater(t, equity, 0.0)
	assert.NotEqual(t, engine.Cash, equity)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func createTestEngine() *Engine {
	config := BacktestConfig{
		InitialCapital: 10000.0,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000.0,
		MaxPositions:   5,
	}

	engine := NewEngine(config)

	// Load test data
	candlesticks := []*Candlestick{
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Close: 50000, Open: 49500, High: 50500, Low: 49000, Volume: 100},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 51000, Open: 50000, High: 51500, Low: 49500, Volume: 120},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 49000, Open: 51000, High: 51000, Low: 48500, Volume: 150},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC), Close: 52000, Open: 49000, High: 52500, Low: 48800, Volume: 130},
		{Symbol: "BTC", Timestamp: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), Close: 53000, Open: 52000, High: 53500, Low: 51500, Volume: 140},
	}

	_ = engine.LoadHistoricalData("BTC", candlesticks) // Test setup - error handled by test

	return engine
}

// ============================================================================
// STRATEGY TESTS
// ============================================================================

type TestStrategy struct {
	initCalled     bool
	finalizeCalled bool
	signals        []*Signal
}

func (s *TestStrategy) Initialize(engine *Engine) error {
	s.initCalled = true
	return nil
}

func (s *TestStrategy) GenerateSignals(engine *Engine) ([]*Signal, error) {
	return s.signals, nil
}

func (s *TestStrategy) Finalize(engine *Engine) error {
	s.finalizeCalled = true
	return nil
}

func TestStrategyIntegration(t *testing.T) {
	engine := createTestEngine()
	strategy := &TestStrategy{
		signals: []*Signal{
			{Symbol: "BTC", Side: "BUY", Confidence: 0.8, Agent: "test"},
		},
	}

	ctx := context.Background()
	err := engine.Run(ctx, strategy)
	require.NoError(t, err)

	assert.True(t, strategy.initCalled)
	assert.True(t, strategy.finalizeCalled)

	// Should have executed the buy signal
	assert.Greater(t, len(engine.Trades), 0)
}

func TestBacktestWithProfitableTrade(t *testing.T) {
	engine := createTestEngine()
	ctx := context.Background()

	// Manually control the backtest
	_, _ = engine.Step(ctx) // Test setup - error acceptable
	signal := &Signal{Symbol: "BTC", Side: "BUY", Confidence: 0.8, Agent: "test"}
	_ = engine.ExecuteSignal(signal) // Test setup - error handled by test

	// Advance 2 more steps (price increases to 52000)
	_, _ = engine.Step(ctx) // Test setup - error acceptable
	_, _ = engine.Step(ctx) // Test setup - error acceptable

	// Sell
	signal = &Signal{Symbol: "BTC", Side: "SELL", Confidence: 0.8, Agent: "test"}
	_ = engine.ExecuteSignal(signal) // Test setup - error handled by test

	// Should have made a profit
	assert.Len(t, engine.ClosedPositions, 1)
	assert.Greater(t, engine.ClosedPositions[0].RealizedPL, 0.0)

	// Winning trades counter should be 1
	assert.Equal(t, 1, engine.WinningTrades)
}
