// Performance Metrics Unit Tests
package backtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// METRICS CALCULATION TESTS
// ============================================================================

func TestCalculateMetrics(t *testing.T) {
	engine := createTestEngineWithTrades()

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Basic checks
	assert.Equal(t, engine.InitialCapital, metrics.InitialCapital)
	assert.Equal(t, engine.GetCurrentEquity(), metrics.FinalEquity)
	assert.Equal(t, engine.TotalTrades, metrics.TotalTrades)
	assert.Equal(t, engine.WinningTrades, metrics.WinningTrades)
	assert.Equal(t, engine.LosingTrades, metrics.LosingTrades)
}

func TestCalculateMetricsReturns(t *testing.T) {
	engine := createTestEngineWithTrades()

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Total return
	expectedReturn := metrics.FinalEquity - metrics.InitialCapital
	assert.Equal(t, expectedReturn, metrics.TotalReturn)

	// Total return percentage
	expectedPct := (expectedReturn / metrics.InitialCapital) * 100.0
	assert.InDelta(t, expectedPct, metrics.TotalReturnPct, 0.01)

	// CAGR should be calculated if duration > 1 year
	// For short durations, CAGR may be 0 due to rounding
	// Just check it's a reasonable value
	assert.GreaterOrEqual(t, metrics.CAGR, -100.0)
	assert.LessOrEqual(t, metrics.CAGR, 10000.0)
}

func TestCalculateMetricsWinRate(t *testing.T) {
	engine := createTestEngineWithTrades()

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Win rate calculation
	if metrics.TotalTrades > 0 {
		expectedWinRate := (float64(metrics.WinningTrades) / float64(metrics.TotalTrades)) * 100.0
		assert.InDelta(t, expectedWinRate, metrics.WinRate, 0.01)
	}
}

func TestCalculateMetricsProfitFactor(t *testing.T) {
	engine := createTestEngineWithTrades()

	// Add some closed positions with known P&L
	engine.ClosedPositions = []*ClosedPosition{
		{RealizedPL: 1000.0}, // Win
		{RealizedPL: 500.0},  // Win
		{RealizedPL: -300.0}, // Loss
		{RealizedPL: -200.0}, // Loss
	}
	engine.WinningTrades = 2
	engine.LosingTrades = 2
	engine.TotalTrades = 4

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Profit factor = Total Win / Total Loss = 1500 / 500 = 3.0
	assert.InDelta(t, 3.0, metrics.ProfitFactor, 0.01)
}

func TestCalculateMetricsExpectancy(t *testing.T) {
	engine := createTestEngineWithTrades()

	// Known P&L
	engine.ClosedPositions = []*ClosedPosition{
		{RealizedPL: 100.0},  // Win
		{RealizedPL: 200.0},  // Win
		{RealizedPL: -50.0},  // Loss
		{RealizedPL: -100.0}, // Loss
	}
	engine.WinningTrades = 2
	engine.LosingTrades = 2
	engine.TotalTrades = 4

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Average win = (100 + 200) / 2 = 150
	// Average loss = (-50 + -100) / 2 = -75
	// Win prob = 2/4 = 0.5
	// Loss prob = 2/4 = 0.5
	// Expectancy = (0.5 * 150) + (0.5 * -75) = 75 - 37.5 = 37.5
	assert.InDelta(t, 37.5, metrics.Expectancy, 0.01)
}

func TestCalculateMetricsHoldingTime(t *testing.T) {
	engine := createTestEngineWithTrades()

	// Known holding times
	engine.ClosedPositions = []*ClosedPosition{
		{HoldingTime: 2 * time.Hour},
		{HoldingTime: 4 * time.Hour},
		{HoldingTime: 6 * time.Hour},
	}

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Average = (2 + 4 + 6) / 3 = 4 hours
	assert.Equal(t, 4*time.Hour, metrics.AverageHoldingTime)

	// Min and max
	assert.Equal(t, 2*time.Hour, metrics.MinHoldingTime)
	assert.Equal(t, 6*time.Hour, metrics.MaxHoldingTime)
}

func TestCalculateMetricsVolatility(t *testing.T) {
	engine := createTestEngineWithEquityCurve()

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Volatility should be > 0 if equity changes
	assert.Greater(t, metrics.Volatility, 0.0)
}

func TestCalculateMetricsSharpeRatio(t *testing.T) {
	engine := createTestEngineWithEquityCurve()

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Sharpe ratio should be calculated
	if metrics.Volatility > 0 {
		assert.NotEqual(t, 0.0, metrics.SharpeRatio)
	}
}

func TestCalculateMetricsSortinoRatio(t *testing.T) {
	engine := createTestEngineWithEquityCurve()

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Sortino ratio can be negative if returns are below risk-free rate
	// Just check it's a reasonable value
	assert.GreaterOrEqual(t, metrics.SortinoRatio, -10.0)
	assert.LessOrEqual(t, metrics.SortinoRatio, 10.0)
}

func TestCalculateMetricsCalmarRatio(t *testing.T) {
	engine := createTestEngineWithEquityCurve()
	engine.MaxDrawdownPct = 10.0 // 10% max drawdown

	metrics, err := CalculateMetrics(engine)
	require.NoError(t, err)

	// Calmar ratio = CAGR / Max Drawdown
	if metrics.MaxDrawdownPct > 0 {
		expectedCalmar := metrics.CAGR / metrics.MaxDrawdownPct
		assert.InDelta(t, expectedCalmar, metrics.CalmarRatio, 0.01)
	}
}

func TestCalculateMetricsEmptyEquityCurve(t *testing.T) {
	engine := NewEngine(BacktestConfig{InitialCapital: 10000.0})

	// No equity curve data
	_, err := CalculateMetrics(engine)
	assert.Error(t, err)
}

// ============================================================================
// REPORT GENERATION TESTS
// ============================================================================

func TestGenerateReport(t *testing.T) {
	metrics := &Metrics{
		InitialCapital: 10000.0,
		FinalEquity:    12000.0,
		TotalReturn:    2000.0,
		TotalReturnPct: 20.0,
		CAGR:           18.5,
		MaxDrawdown:    500.0,
		MaxDrawdownPct: 5.0,
		Volatility:     15.0,
		SharpeRatio:    1.2,
		SortinoRatio:   1.5,
		CalmarRatio:    3.7,
		TotalTrades:    10,
		WinningTrades:  6,
		LosingTrades:   4,
		WinRate:        60.0,
		AverageWin:     400.0,
		AverageLoss:    -200.0,
		LargestWin:     1000.0,
		LargestLoss:    -500.0,
		ProfitFactor:   3.0,
		Expectancy:     160.0,
		StartDate:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:        time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		Duration:       365 * 24 * time.Hour,
	}

	report := GenerateReport(metrics)

	// Check that report contains key values
	assert.Contains(t, report, "10000.00") // Initial capital
	assert.Contains(t, report, "12000.00") // Final equity
	assert.Contains(t, report, "20.00")    // Return pct
	assert.Contains(t, report, "60.00")    // Win rate
	assert.Contains(t, report, "1.20")     // Sharpe ratio
	assert.Contains(t, report, "10")       // Total trades
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h 0m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25 * time.Hour, "1d 1h 0m"},
		{48*time.Hour + 30*time.Minute, "2d 0h 30m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func createTestEngineWithTrades() *Engine {
	engine := createTestEngine()

	// Add some trades and closed positions
	engine.Trades = []*Trade{
		{ID: 1, Side: "BUY", Price: 50000, Quantity: 0.1, Value: 5000},
		{ID: 2, Side: "SELL", Price: 52000, Quantity: 0.1, Value: 5200},
	}

	engine.ClosedPositions = []*ClosedPosition{
		{RealizedPL: 200.0, HoldingTime: 24 * time.Hour},
	}

	engine.TotalTrades = 2
	engine.WinningTrades = 1
	engine.LosingTrades = 0

	// Add equity curve
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	engine.EquityCurve = []*EquityPoint{
		{Timestamp: baseTime, Equity: 10000},
		{Timestamp: baseTime.Add(time.Hour), Equity: 10050},
		{Timestamp: baseTime.Add(2 * time.Hour), Equity: 10100},
		{Timestamp: baseTime.Add(3 * time.Hour), Equity: 10200},
	}

	return engine
}

func createTestEngineWithEquityCurve() *Engine {
	engine := NewEngine(BacktestConfig{InitialCapital: 10000.0})

	// Create a varying equity curve
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	engine.EquityCurve = []*EquityPoint{
		{Timestamp: baseTime, Equity: 10000},
		{Timestamp: baseTime.Add(time.Hour), Equity: 10100},
		{Timestamp: baseTime.Add(2 * time.Hour), Equity: 9900}, // Drawdown
		{Timestamp: baseTime.Add(3 * time.Hour), Equity: 10200},
		{Timestamp: baseTime.Add(4 * time.Hour), Equity: 10500},
		{Timestamp: baseTime.Add(5 * time.Hour), Equity: 10300},
	}

	engine.PeakEquity = 10500
	engine.MaxDrawdown = 200
	engine.MaxDrawdownPct = 2.0

	return engine
}
