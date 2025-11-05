package backtest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// REPORT GENERATION TESTS
// ============================================================================

func TestNewReportGenerator(t *testing.T) {
	engine := createReportTestEngine()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)
	assert.NotNil(t, generator)
	assert.NotNil(t, generator.engine)
	assert.NotNil(t, generator.metrics)
	assert.Nil(t, generator.summary) // No optimization summary
}

func TestNewOptimizationReportGenerator(t *testing.T) {
	engine := createReportTestEngine()
	summary := &OptimizationSummary{
		Method:     "grid_search",
		TotalRuns:  10,
		Duration:   time.Second,
		BestResult: &OptimizationResult{Score: 1.5},
		TopResults: []*OptimizationResult{},
	}

	generator, err := NewOptimizationReportGenerator(engine, summary)
	require.NoError(t, err)
	assert.NotNil(t, generator)
	assert.NotNil(t, generator.summary)
	assert.Equal(t, "grid_search", generator.summary.Method)
}

func TestGenerateHTML(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	html, err := generator.GenerateHTML()
	require.NoError(t, err)
	assert.NotEmpty(t, html)

	// Check HTML structure
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "<html")
	assert.Contains(t, html, "</html>")
	assert.Contains(t, html, "Backtest Report")
	assert.Contains(t, html, "Performance Summary")
	assert.Contains(t, html, "Equity Curve")
	assert.Contains(t, html, "Drawdown")
	assert.Contains(t, html, "chart.js") // Chart.js CDN script tag
}

func TestSaveToFile(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	// Create temporary file
	tmpfile := "/tmp/backtest_report_test.html"
	defer func() { _ = os.Remove(tmpfile) }() // Test cleanup

	// Save report
	err = generator.SaveToFile(tmpfile)
	require.NoError(t, err)

	// Verify file exists and has content
	data, err := os.ReadFile(tmpfile)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "Backtest Report")
}

func TestPrepareEquityCurveData(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	chartData := generator.prepareEquityCurveData()
	assert.NotEmpty(t, chartData)
	assert.Contains(t, chartData, "labels")
	assert.Contains(t, chartData, "datasets")
	assert.Contains(t, chartData, "Equity")
}

func TestPrepareEquityCurveData_EmptyData(t *testing.T) {
	// Create engine with truly empty equity curve
	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   5,
	}
	engine := NewEngine(config)

	generator := &ReportGenerator{engine: engine}

	chartData := generator.prepareEquityCurveData()
	assert.Contains(t, chartData, "labels: []")
	assert.Contains(t, chartData, "datasets: []")
}

func TestPrepareDrawdownData(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	chartData := generator.prepareDrawdownData()
	assert.NotEmpty(t, chartData)
	assert.Contains(t, chartData, "labels")
	assert.Contains(t, chartData, "datasets")
	assert.Contains(t, chartData, "Drawdown")
}

func TestPrepareMonthlyReturnsData(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	chartData := generator.prepareMonthlyReturnsData()
	assert.NotEmpty(t, chartData)
	assert.Contains(t, chartData, "labels")
	assert.Contains(t, chartData, "datasets")
	assert.Contains(t, chartData, "Monthly P&L")
}

func TestPrepareMonthlyReturnsData_EmptyData(t *testing.T) {
	engine := createReportTestEngine()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	chartData := generator.prepareMonthlyReturnsData()
	assert.Contains(t, chartData, "labels: []")
	assert.Contains(t, chartData, "datasets: []")
}

func TestPrepareTradeDistributionData(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	chartData := generator.prepareTradeDistributionData()
	assert.NotEmpty(t, chartData)
	assert.Contains(t, chartData, "labels")
	assert.Contains(t, chartData, "datasets")
	assert.Contains(t, chartData, "Number of Trades")
}

func TestPrepareWinLossData(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	chartData := generator.prepareWinLossData()
	assert.NotEmpty(t, chartData)
	assert.Contains(t, chartData, "Winning Trades")
	assert.Contains(t, chartData, "Losing Trades")
}

func TestGetTopOptimizationRuns(t *testing.T) {
	engine := createReportTestEngine()

	t.Run("with optimization results", func(t *testing.T) {
		summary := &OptimizationSummary{
			TopResults: []*OptimizationResult{
				{Score: 1.5, Parameters: ParameterSet{"param1": 10}},
				{Score: 1.4, Parameters: ParameterSet{"param1": 20}},
				{Score: 1.3, Parameters: ParameterSet{"param1": 30}},
			},
		}

		generator, err := NewOptimizationReportGenerator(engine, summary)
		require.NoError(t, err)

		top := generator.getTopOptimizationRuns(2)
		assert.Len(t, top, 2)
		assert.Equal(t, 1.5, top[0].Score)
		assert.Equal(t, 1.4, top[1].Score)
	})

	t.Run("without optimization results", func(t *testing.T) {
		generator, err := NewReportGenerator(engine)
		require.NoError(t, err)

		top := generator.getTopOptimizationRuns(10)
		assert.Nil(t, top)
	})

	t.Run("request more than available", func(t *testing.T) {
		summary := &OptimizationSummary{
			TopResults: []*OptimizationResult{
				{Score: 1.5, Parameters: ParameterSet{"param1": 10}},
			},
		}

		generator, err := NewOptimizationReportGenerator(engine, summary)
		require.NoError(t, err)

		top := generator.getTopOptimizationRuns(10)
		assert.Len(t, top, 1)
	})
}

func TestPrepareTemplateData(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	data := generator.prepareTemplateData()

	// Check all required fields
	assert.Equal(t, "Backtest Report", data["Title"])
	assert.NotNil(t, data["GeneratedAt"])
	assert.NotNil(t, data["Config"])
	assert.NotNil(t, data["Metrics"])
	assert.NotEmpty(t, data["EquityCurveData"])
	assert.NotEmpty(t, data["DrawdownData"])
	assert.NotEmpty(t, data["MonthlyReturnsData"])
	assert.NotEmpty(t, data["TradeDistribution"])
	assert.NotEmpty(t, data["WinLossData"])
	assert.NotNil(t, data["ClosedPositions"])
	assert.NotNil(t, data["Trades"])
}

func TestFormatFloat(t *testing.T) {
	assert.Equal(t, "123.46", formatFloat(123.456))
	assert.Equal(t, "0.12", formatFloat(0.123))
	assert.Equal(t, "-45.68", formatFloat(-45.678))
}

func TestFormatPercent(t *testing.T) {
	assert.Equal(t, "12.35%", formatPercent(12.345))
	assert.Equal(t, "0.12%", formatPercent(0.123))
	assert.Equal(t, "-5.68%", formatPercent(-5.678))
}

func TestFormatTime(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	formatted := formatTime(testTime)
	assert.Equal(t, "2024-01-15 10:30:45", formatted)
}

func TestHTMLReportContent(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	html, err := generator.GenerateHTML()
	require.NoError(t, err)

	// Check for key sections
	assert.Contains(t, html, "Performance Summary")
	assert.Contains(t, html, "Equity Curve")
	assert.Contains(t, html, "Drawdown")
	assert.Contains(t, html, "Returns Analysis")
	assert.Contains(t, html, "Trade Breakdown")
	assert.Contains(t, html, "Backtest Configuration")
	assert.Contains(t, html, "Recent Trades")

	// Check for metrics
	assert.Contains(t, html, "Total Return")
	assert.Contains(t, html, "Sharpe Ratio")
	assert.Contains(t, html, "Max Drawdown")
	assert.Contains(t, html, "Win Rate")
	assert.Contains(t, html, "Profit Factor")
	assert.Contains(t, html, "CAGR")

	// Check for charts
	assert.Contains(t, html, "equityChart")
	assert.Contains(t, html, "drawdownChart")
	assert.Contains(t, html, "monthlyReturnsChart")
	assert.Contains(t, html, "tradeDistributionChart")
	assert.Contains(t, html, "winLossChart")
}

func TestOptimizationReportContent(t *testing.T) {
	engine := createReportTestEngineWithData()

	summary := &OptimizationSummary{
		Method:    "grid_search",
		TotalRuns: 16,
		Duration:  2500 * time.Millisecond,
		BestResult: &OptimizationResult{
			Score:      1.8,
			Parameters: ParameterSet{"short_period": 10, "long_period": 30},
			Metrics: &Metrics{
				SharpeRatio:    1.8,
				TotalReturn:    25.5,
				MaxDrawdownPct: -8.2,
			},
		},
		TopResults: []*OptimizationResult{
			{
				Score:      1.8,
				Parameters: ParameterSet{"short_period": 10, "long_period": 30},
				Metrics: &Metrics{
					SharpeRatio:    1.8,
					TotalReturn:    25.5,
					MaxDrawdownPct: -8.2,
				},
			},
		},
	}

	generator, err := NewOptimizationReportGenerator(engine, summary)
	require.NoError(t, err)

	html, err := generator.GenerateHTML()
	require.NoError(t, err)

	// Check for optimization section
	assert.Contains(t, html, "Optimization Results")
	assert.Contains(t, html, "grid_search")
	assert.Contains(t, html, "Total Runs:")
	assert.Contains(t, html, "Duration:")
	assert.Contains(t, html, "Best Score:")
	assert.Contains(t, html, "Top 10 Parameter Sets")
}

func TestReportWithMixedTrades(t *testing.T) {
	// Create engine with both winning and losing trades
	engine := createReportTestEngine()

	// Add equity curve
	baseTime := time.Now()
	engine.EquityCurve = []*EquityPoint{
		{Timestamp: baseTime, Equity: 10000, Cash: 10000},
		{Timestamp: baseTime.Add(time.Hour), Equity: 10500, Cash: 10500},
		{Timestamp: baseTime.Add(2 * time.Hour), Equity: 9800, Cash: 9800},
		{Timestamp: baseTime.Add(3 * time.Hour), Equity: 10200, Cash: 10200},
	}

	// Add winning trade
	engine.ClosedPositions = append(engine.ClosedPositions, &ClosedPosition{
		Symbol:      "BTC/USD",
		Side:        "LONG",
		EntryTime:   baseTime,
		ExitTime:    baseTime.Add(time.Hour),
		EntryPrice:  50000,
		ExitPrice:   51000,
		Quantity:    0.1,
		RealizedPL:  100,
		ReturnPct:   2.0,
		HoldingTime: time.Hour,
	})

	// Add losing trade
	engine.ClosedPositions = append(engine.ClosedPositions, &ClosedPosition{
		Symbol:      "ETH/USD",
		Side:        "LONG",
		EntryTime:   baseTime.Add(time.Hour),
		ExitTime:    baseTime.Add(2 * time.Hour),
		EntryPrice:  3000,
		ExitPrice:   2900,
		Quantity:    1.0,
		RealizedPL:  -100,
		ReturnPct:   -3.33,
		HoldingTime: time.Hour,
	})

	engine.WinningTrades = 1
	engine.LosingTrades = 1
	engine.TotalProfit = 100
	engine.TotalLoss = -100

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	html, err := generator.GenerateHTML()
	require.NoError(t, err)

	// Verify trade data is in HTML
	assert.Contains(t, html, "BTC/USD")
	assert.Contains(t, html, "ETH/USD")

	// Verify P&L coloring classes
	assert.Contains(t, html, "positive")
	assert.Contains(t, html, "negative")
}

func TestReportChartDataFormatting(t *testing.T) {
	engine := createReportTestEngineWithData()

	generator, err := NewReportGenerator(engine)
	require.NoError(t, err)

	t.Run("equity curve has valid JSON", func(t *testing.T) {
		chartData := generator.prepareEquityCurveData()
		// Check for JSON array structure
		assert.True(t, strings.Contains(chartData, "["))
		assert.True(t, strings.Contains(chartData, "]"))
	})

	t.Run("drawdown has valid JSON", func(t *testing.T) {
		chartData := generator.prepareDrawdownData()
		assert.True(t, strings.Contains(chartData, "["))
		assert.True(t, strings.Contains(chartData, "]"))
	})

	t.Run("monthly returns has valid JSON", func(t *testing.T) {
		chartData := generator.prepareMonthlyReturnsData()
		assert.True(t, strings.Contains(chartData, "["))
		assert.True(t, strings.Contains(chartData, "]"))
	})

	t.Run("trade distribution has valid JSON", func(t *testing.T) {
		chartData := generator.prepareTradeDistributionData()
		assert.True(t, strings.Contains(chartData, "["))
		assert.True(t, strings.Contains(chartData, "]"))
	})

	t.Run("win/loss has valid JSON", func(t *testing.T) {
		chartData := generator.prepareWinLossData()
		assert.True(t, strings.Contains(chartData, "["))
		assert.True(t, strings.Contains(chartData, "]"))
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func createReportTestEngine() *Engine {
	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   5,
	}

	engine := NewEngine(config)

	// Add minimal equity curve data to satisfy CalculateMetrics
	engine.EquityCurve = []*EquityPoint{
		{
			Timestamp: time.Now(),
			Equity:    10000,
			Cash:      10000,
			Holdings:  0,
		},
	}

	return engine
}

func createReportTestEngineWithData() *Engine {
	engine := createReportTestEngine()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Add equity curve
	for i := 0; i < 10; i++ {
		equityChange := float64(i * 100)
		engine.EquityCurve = append(engine.EquityCurve, &EquityPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Equity:    10000 + equityChange,
			Cash:      10000 + equityChange,
			Holdings:  0,
		})
	}

	// Add some closed positions
	for i := 0; i < 5; i++ {
		pl := float64((i%2)*200 - 100) // Alternating +100 and -100
		returnPct := pl / 1000 * 100

		engine.ClosedPositions = append(engine.ClosedPositions, &ClosedPosition{
			Symbol:      "BTC/USD",
			Side:        "LONG",
			EntryTime:   baseTime.Add(time.Duration(i) * time.Hour),
			ExitTime:    baseTime.Add(time.Duration(i+1) * time.Hour),
			EntryPrice:  50000 + float64(i*100),
			ExitPrice:   50000 + float64(i*100) + pl*10,
			Quantity:    0.02,
			RealizedPL:  pl,
			ReturnPct:   returnPct,
			HoldingTime: time.Hour,
			Commission:  2.0,
		})

		if pl > 0 {
			engine.WinningTrades++
			engine.TotalProfit += pl
		} else {
			engine.LosingTrades++
			engine.TotalLoss += pl
		}
	}

	engine.TotalTrades = len(engine.ClosedPositions)
	engine.PeakEquity = 10900
	engine.MaxDrawdown = 100
	engine.MaxDrawdownPct = -1.0

	return engine
}
