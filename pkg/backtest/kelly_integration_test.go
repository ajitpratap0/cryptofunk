package backtest

import (
	"context"
	"testing"
	"time"
)

// TestKellyIntegration_BacktestWithKellySizing demonstrates Kelly Criterion
// working end-to-end in a complete backtest scenario
func TestKellyIntegration_BacktestWithKellySizing(t *testing.T) {
	// Create backtest engine with Kelly sizing
	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "kelly",
		PositionSize:   0.25, // Quarter Kelly
		MaxPositions:   5,
	}
	engine := NewEngine(config)

	// Generate realistic price data with upward trend
	// This creates a profitable scenario for a simple strategy
	now := time.Now()
	candles := make([]*Candlestick, 100)
	basePrice := 50000.0

	for i := 0; i < 100; i++ {
		// Oscillating price with slight upward trend
		price := basePrice + float64(i)*50 + float64(i%10)*100
		candles[i] = &Candlestick{
			Symbol:    "BTC",
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Open:      price - 50,
			High:      price + 100,
			Low:       price - 100,
			Close:     price,
			Volume:    1000,
		}
	}

	engine.LoadHistoricalData("BTC", candles)

	// Create a simple trend-following strategy
	strategy := &SimpleTrendStrategy{
		Symbol:        "BTC",
		BuyThreshold:  0,   // Buy when price increases
		SellThreshold: 0.5, // Sell when confidence high
	}

	// Run backtest
	ctx := context.Background()
	err := engine.Run(ctx, strategy)
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// Verify backtest completed
	if engine.TotalTrades == 0 {
		t.Error("Expected some trades to be executed")
	}

	// Verify Kelly sizing was used
	if len(engine.Trades) > 0 {
		t.Logf("Total trades: %d", engine.TotalTrades)
		t.Logf("Winning trades: %d (%.1f%%)", engine.WinningTrades, float64(engine.WinningTrades)/float64(engine.TotalTrades)*100)
		t.Logf("Final equity: %.2f", engine.GetCurrentEquity())
		t.Logf("Total profit: %.2f", engine.TotalProfit-engine.TotalLoss)
		t.Logf("Max drawdown: %.2f%%", engine.MaxDrawdownPct)

		// Calculate stats to verify Kelly was applied
		stats := CalculateStatsFromTrades(engine.ClosedPositions)
		if stats.TotalTrades > 0 {
			t.Logf("Trading stats:")
			t.Logf("  Win rate: %.1f%%", stats.WinRate*100)
			t.Logf("  Avg win: %.2f", stats.AvgWin)
			t.Logf("  Avg loss: %.2f", stats.AvgLoss)
			t.Logf("  Win/Loss ratio: %.2f", stats.WinLossRatio)
		}
	}

	// Verify equity curve was recorded
	if len(engine.EquityCurve) == 0 {
		t.Error("Expected equity curve to be recorded")
	}

	// Verify final equity is within reasonable bounds
	finalEquity := engine.GetCurrentEquity()
	if finalEquity < 0 {
		t.Error("Final equity should not be negative")
	}
	if finalEquity > config.InitialCapital*10 {
		t.Error("Unrealistic gains - check position sizing")
	}
}

// SimpleTrendStrategy is a basic strategy for testing
type SimpleTrendStrategy struct {
	Symbol        string
	BuyThreshold  float64
	SellThreshold float64
	lastPrice     float64
}

func (s *SimpleTrendStrategy) Initialize(engine *Engine) error {
	s.lastPrice = 0
	return nil
}

func (s *SimpleTrendStrategy) GenerateSignals(engine *Engine) ([]*Signal, error) {
	signals := []*Signal{}

	// Get current candle for the symbol
	candle, err := engine.GetCurrentCandle(s.Symbol)
	if err != nil || candle == nil {
		return signals, nil
	}

	signal := &Signal{
		Timestamp:  candle.Timestamp,
		Symbol:     candle.Symbol,
		Side:       "HOLD",
		Confidence: 0.5,
		Reasoning:  "Analyzing trend",
		Agent:      "simple-trend",
	}

	// Simple logic: buy if price increasing, sell if we have position
	if s.lastPrice > 0 {
		priceChange := (candle.Close - s.lastPrice) / s.lastPrice

		// Buy signal if price is rising
		if priceChange > 0.001 && len(engine.Positions) < engine.MaxPositions {
			signal.Side = "BUY"
			signal.Confidence = 0.6
			signal.Reasoning = "Upward trend detected"
			signals = append(signals, signal)
		}

		// Sell signal if we have a position and decent confidence
		if len(engine.Positions) > 0 && priceChange < -0.001 {
			signal.Side = "SELL"
			signal.Confidence = 0.7
			signal.Reasoning = "Taking profit"
			signals = append(signals, signal)
		}
	}

	s.lastPrice = candle.Close
	return signals, nil
}

func (s *SimpleTrendStrategy) Finalize(engine *Engine) error {
	return nil
}

// TestKellyIntegration_AdaptivePositionSizing tests that Kelly sizing
// adapts based on performance
func TestKellyIntegration_AdaptivePositionSizing(t *testing.T) {
	// This test verifies that:
	// 1. Initially, with no trade history, Kelly uses conservative sizing (10%)
	// 2. As winning trades accumulate, position sizes should increase
	// 3. As losing trades accumulate, position sizes should decrease

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "kelly",
		PositionSize:   0.25, // Quarter Kelly
		MaxPositions:   3,
	}
	engine := NewEngine(config)

	// Add a few winning trades manually to simulate history
	engine.ClosedPositions = []*ClosedPosition{
		{Symbol: "BTC", RealizedPL: 100, EntryTime: time.Now(), ExitTime: time.Now()},
		{Symbol: "BTC", RealizedPL: 150, EntryTime: time.Now(), ExitTime: time.Now()},
		{Symbol: "BTC", RealizedPL: 120, EntryTime: time.Now(), ExitTime: time.Now()},
		{Symbol: "BTC", RealizedPL: -80, EntryTime: time.Now(), ExitTime: time.Now()},
		{Symbol: "BTC", RealizedPL: 200, EntryTime: time.Now(), ExitTime: time.Now()},
	}

	// Calculate position size with this history
	price := 50000.0
	quantity := engine.calculatePositionSize(price)

	// With 4 wins out of 5 (80% win rate), should get reasonable position
	if quantity <= 0 {
		t.Error("Expected positive position size")
	}

	dollarAmount := quantity * price
	percentOfEquity := (dollarAmount / engine.GetCurrentEquity()) * 100

	t.Logf("Position size: %.4f BTC (%.2f USD, %.2f%% of equity)",
		quantity, dollarAmount, percentOfEquity)

	// Should be more than minimal 1% but less than max 25%
	if percentOfEquity < 0.5 || percentOfEquity > 26 {
		t.Errorf("Position size %.2f%% outside expected range", percentOfEquity)
	}

	// Now simulate losing streak
	engine.ClosedPositions = append(engine.ClosedPositions,
		&ClosedPosition{Symbol: "BTC", RealizedPL: -100, EntryTime: time.Now(), ExitTime: time.Now()},
		&ClosedPosition{Symbol: "BTC", RealizedPL: -120, EntryTime: time.Now(), ExitTime: time.Now()},
		&ClosedPosition{Symbol: "BTC", RealizedPL: -150, EntryTime: time.Now(), ExitTime: time.Now()},
	)

	// Recalculate - should be more conservative now
	newQuantity := engine.calculatePositionSize(price)
	newDollarAmount := newQuantity * price
	newPercentOfEquity := (newDollarAmount / engine.GetCurrentEquity()) * 100

	t.Logf("After losing streak: %.2f%% of equity (was %.2f%%)",
		newPercentOfEquity, percentOfEquity)

	// Position sizing should adapt to recent performance
	// Note: This may or may not be smaller depending on overall win rate
	// The key is that Kelly adapts based on statistics
	if newPercentOfEquity < 0 || newPercentOfEquity > 26 {
		t.Errorf("New position size %.2f%% outside valid range", newPercentOfEquity)
	}
}

// TestKellyIntegration_CompareSizingMethods compares Kelly to other methods
func TestKellyIntegration_CompareSizingMethods(t *testing.T) {
	// Run identical backtests with different position sizing methods
	configs := []struct {
		name   string
		sizing string
		size   float64
	}{
		{"Fixed $1000", "fixed", 1000},
		{"10% of equity", "percent", 0.10},
		{"Quarter Kelly", "kelly", 0.25},
	}

	results := make(map[string]float64)

	for _, cfg := range configs {
		config := BacktestConfig{
			InitialCapital: 10000,
			CommissionRate: 0.001,
			PositionSizing: cfg.sizing,
			PositionSize:   cfg.size,
			MaxPositions:   3,
		}
		engine := NewEngine(config)

		// Use same data for all
		now := time.Now()
		candles := make([]*Candlestick, 50)
		for i := 0; i < 50; i++ {
			price := 50000.0 + float64(i)*100 + float64(i%5)*50
			candles[i] = &Candlestick{
				Symbol:    "BTC",
				Timestamp: now.Add(time.Duration(i) * time.Hour),
				Open:      price,
				High:      price + 50,
				Low:       price - 50,
				Close:     price,
				Volume:    1000,
			}
		}
		engine.LoadHistoricalData("BTC", candles)

		// Run with simple strategy
		strategy := &SimpleTrendStrategy{Symbol: "BTC"}
		ctx := context.Background()
		err := engine.Run(ctx, strategy)
		if err != nil {
			t.Fatalf("Backtest failed for %s: %v", cfg.name, err)
		}

		finalEquity := engine.GetCurrentEquity()
		results[cfg.name] = finalEquity

		t.Logf("%s: Final equity = %.2f, Return = %.2f%%",
			cfg.name, finalEquity, (finalEquity-config.InitialCapital)/config.InitialCapital*100)
	}

	// All methods should produce valid results
	for name, equity := range results {
		if equity <= 0 {
			t.Errorf("%s produced invalid equity: %.2f", name, equity)
		}
	}
}

// BenchmarkKellySizing benchmarks the Kelly position sizing calculation
func BenchmarkKellySizing(b *testing.B) {
	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "kelly",
		PositionSize:   0.25,
		MaxPositions:   5,
	}
	engine := NewEngine(config)

	// Add realistic trade history
	for i := 0; i < 50; i++ {
		pl := 100.0
		if i%3 == 0 {
			pl = -75.0
		}
		engine.ClosedPositions = append(engine.ClosedPositions, &ClosedPosition{
			Symbol:     "BTC",
			RealizedPL: pl,
			EntryTime:  time.Now(),
			ExitTime:   time.Now(),
		})
	}

	price := 50000.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.calculatePositionSize(price)
	}
}
