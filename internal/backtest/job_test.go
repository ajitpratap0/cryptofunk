package backtest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	btengine "github.com/ajitpratap0/cryptofunk/pkg/backtest"
)

func TestBacktestJob(t *testing.T) {
	job := &BacktestJob{
		ID:             uuid.New(),
		Name:           "Test Backtest",
		Status:         JobStatusPending,
		StartDate:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:        time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Symbols:        []string{"BTC/USDT"},
		InitialCapital: 10000.0,
		StrategyConfig: map[string]interface{}{
			"type": "trend_following",
			"parameters": map[string]interface{}{
				"period": 20,
			},
		},
	}

	assert.NotEqual(t, uuid.Nil, job.ID)
	assert.Equal(t, "Test Backtest", job.Name)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 1, len(job.Symbols))
	assert.Equal(t, 10000.0, job.InitialCapital)
}

func TestJobStatus(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
	}{
		{"pending", JobStatusPending},
		{"running", JobStatusRunning},
		{"completed", JobStatusCompleted},
		{"failed", JobStatusFailed},
		{"cancelled", JobStatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.status)
		})
	}
}

func TestBacktestResults(t *testing.T) {
	results := &BacktestResults{
		TotalReturnPct: 15.2,
		SharpeRatio:    1.8,
		MaxDrawdownPct: -8.5,
		WinRate:        62.0,
		TotalTrades:    45,
		ProfitFactor:   2.5,
		SortinoRatio:   2.1,
		CalmarRatio:    1.9,
		Expectancy:     125.5,
		WinningTrades:  28,
		LosingTrades:   17,
		AverageWin:     350.0,
		AverageLoss:    -180.0,
		LargestWin:     1200.0,
		LargestLoss:    -450.0,
		EquityCurve: []EquityPoint{
			{Date: "2024-01-01", Value: 10000.0},
			{Date: "2024-06-01", Value: 11520.0},
		},
		Trades: []TradeResult{
			{
				Symbol:     "BTC/USDT",
				Side:       "LONG",
				EntryTime:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				ExitTime:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
				EntryPrice: 42000.0,
				ExitPrice:  43500.0,
				Quantity:   0.1,
				PnL:        150.0,
				PnLPct:     3.57,
				Commission: 4.2,
			},
		},
	}

	assert.Equal(t, 15.2, results.TotalReturnPct)
	assert.Equal(t, 1.8, results.SharpeRatio)
	assert.Equal(t, 45, results.TotalTrades)
	assert.Equal(t, 2, len(results.EquityCurve))
	assert.Equal(t, 1, len(results.Trades))
}

func TestConvertEngineResultsToBacktestResults(t *testing.T) {
	// Create a mock backtest engine with results
	config := btengine.BacktestConfig{
		InitialCapital: 10000.0,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000.0,
		MaxPositions:   5,
		StartDate:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:        time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Symbols:        []string{"BTC/USDT"},
	}

	engine := btengine.NewEngine(config)

	// Add some equity curve points
	engine.EquityCurve = []*btengine.EquityPoint{
		{
			Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Equity:    10000.0,
			Cash:      10000.0,
			Holdings:  0,
		},
		{
			Timestamp: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			Equity:    11520.0,
			Cash:      8000.0,
			Holdings:  3520.0,
		},
	}

	// Add some closed positions
	engine.ClosedPositions = []*btengine.ClosedPosition{
		{
			Symbol:      "BTC/USDT",
			Side:        "LONG",
			EntryTime:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			ExitTime:    time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			EntryPrice:  42000.0,
			ExitPrice:   43500.0,
			Quantity:    0.1,
			RealizedPL:  150.0,
			ReturnPct:   3.57,
			HoldingTime: 4 * 24 * time.Hour,
			Commission:  4.2,
		},
	}

	// Create metrics
	metrics := &btengine.Metrics{
		TotalReturnPct: 15.2,
		SharpeRatio:    1.8,
		MaxDrawdownPct: 8.5,
		WinRate:        62.0,
		TotalTrades:    45,
		ProfitFactor:   2.5,
		SortinoRatio:   2.1,
		CalmarRatio:    1.9,
		Expectancy:     125.5,
		WinningTrades:  28,
		LosingTrades:   17,
		AverageWin:     350.0,
		AverageLoss:    -180.0,
		LargestWin:     1200.0,
		LargestLoss:    -450.0,
	}

	// Convert to API format
	results := ConvertEngineResultsToBacktestResults(engine, metrics)

	// Verify conversion
	assert.NotNil(t, results)
	assert.Equal(t, 15.2, results.TotalReturnPct)
	assert.Equal(t, 1.8, results.SharpeRatio)
	assert.Equal(t, 8.5, results.MaxDrawdownPct)
	assert.Equal(t, 62.0, results.WinRate)
	assert.Equal(t, 45, results.TotalTrades)
	assert.Equal(t, 2.5, results.ProfitFactor)
	assert.Equal(t, 28, results.WinningTrades)
	assert.Equal(t, 17, results.LosingTrades)

	// Verify equity curve conversion
	assert.Equal(t, 2, len(results.EquityCurve))
	assert.Equal(t, "2024-01-01", results.EquityCurve[0].Date)
	assert.Equal(t, 10000.0, results.EquityCurve[0].Value)
	assert.Equal(t, "2024-06-01", results.EquityCurve[1].Date)
	assert.Equal(t, 11520.0, results.EquityCurve[1].Value)

	// Verify trades conversion
	assert.Equal(t, 1, len(results.Trades))
	assert.Equal(t, "BTC/USDT", results.Trades[0].Symbol)
	assert.Equal(t, "LONG", results.Trades[0].Side)
	assert.Equal(t, 42000.0, results.Trades[0].EntryPrice)
	assert.Equal(t, 43500.0, results.Trades[0].ExitPrice)
	assert.Equal(t, 0.1, results.Trades[0].Quantity)
	assert.Equal(t, 150.0, results.Trades[0].PnL)
	assert.Equal(t, 3.57, results.Trades[0].PnLPct)
	assert.Equal(t, 4.2, results.Trades[0].Commission)
}
