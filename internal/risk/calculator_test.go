package risk

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadHistoricalPrices tests loading historical prices from database
func TestLoadHistoricalPrices(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	// Mock data
	rows := pgxmock.NewRows([]string{"close", "open_time"}).
		AddRow(100.0, time.Now().Add(-3*24*time.Hour)).
		AddRow(105.0, time.Now().Add(-2*24*time.Hour)).
		AddRow(110.0, time.Now().Add(-1*24*time.Hour)).
		AddRow(115.0, time.Now())

	mock.ExpectQuery("SELECT close, open_time FROM candlesticks").
		WithArgs("BTC/USDT", "1d", 30).
		WillReturnRows(rows)

	ctx := context.Background()
	histData, err := calculator.LoadHistoricalPrices(ctx, "BTC/USDT", "1d", 30)

	require.NoError(t, err)
	assert.Equal(t, 4, len(histData.Prices))
	assert.Equal(t, 3, len(histData.Returns)) // Returns is prices - 1
	assert.Equal(t, 100.0, histData.Prices[0])
	assert.Equal(t, 115.0, histData.Prices[3])

	// Check returns calculation: (105-100)/100 = 0.05
	assert.InDelta(t, 0.05, histData.Returns[0], 0.001)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestLoadHistoricalPricesNoData tests when no data is found
func TestLoadHistoricalPricesNoData(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"close", "open_time"})
	mock.ExpectQuery("SELECT close, open_time FROM candlesticks").
		WithArgs("BTC/USDT", "1d", 30).
		WillReturnRows(rows)

	ctx := context.Background()
	_, err = calculator.LoadHistoricalPrices(ctx, "BTC/USDT", "1d", 30)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no historical prices found")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetCurrentPrice tests getting the most recent price
func TestGetCurrentPrice(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"close"}).AddRow(50000.0)
	mock.ExpectQuery("SELECT close FROM candlesticks").
		WithArgs("BTC/USDT", "1h").
		WillReturnRows(rows)

	ctx := context.Background()
	price, err := calculator.GetCurrentPrice(ctx, "BTC/USDT", "1h")

	require.NoError(t, err)
	assert.Equal(t, 50000.0, price)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCalculateWinRate tests win rate calculation from positions
func TestCalculateWinRate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"winning_trades", "losing_trades", "total_trades", "avg_win", "avg_loss"}).
		AddRow(int64(60), int64(40), int64(100), 250.0, 100.0)

	mock.ExpectQuery("SELECT(.+)FROM positions").
		WithArgs("BTC/USDT").
		WillReturnRows(rows)

	ctx := context.Background()
	winRateData, err := calculator.CalculateWinRate(ctx, "BTC/USDT")

	require.NoError(t, err)
	assert.Equal(t, 0.6, winRateData.WinRate) // 60/100
	assert.Equal(t, int64(60), winRateData.WinningTrades)
	assert.Equal(t, int64(40), winRateData.LosingTrades)
	assert.Equal(t, int64(100), winRateData.TotalTrades)
	assert.Equal(t, 250.0, winRateData.AvgWin)
	assert.Equal(t, 100.0, winRateData.AvgLoss)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCalculateWinRateNoData tests win rate with no historical trades
func TestCalculateWinRateNoData(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"winning_trades", "losing_trades", "total_trades", "avg_win", "avg_loss"}).
		AddRow(int64(0), int64(0), int64(0), 0.0, 0.0)

	mock.ExpectQuery("SELECT(.+)FROM positions").
		WithArgs("BTC/USDT").
		WillReturnRows(rows)

	ctx := context.Background()
	winRateData, err := calculator.CalculateWinRate(ctx, "BTC/USDT")

	require.NoError(t, err)
	// Should return defaults when no data
	assert.Equal(t, 0.55, winRateData.WinRate)
	assert.Equal(t, 200.0, winRateData.AvgWin)
	assert.Equal(t, 100.0, winRateData.AvgLoss)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestLoadEquityCurve tests loading equity curve from performance_metrics
func TestLoadEquityCurve(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"total_value", "metric_time"}).
		AddRow(10000.0, time.Now().Add(-3*24*time.Hour)).
		AddRow(10500.0, time.Now().Add(-2*24*time.Hour)).
		AddRow(11000.0, time.Now().Add(-1*24*time.Hour)).
		AddRow(10800.0, time.Now())

	mock.ExpectQuery("SELECT total_value, metric_time FROM performance_metrics").
		WithArgs(30).
		WillReturnRows(rows)

	ctx := context.Background()
	perfData, err := calculator.LoadEquityCurve(ctx, nil, 30)

	require.NoError(t, err)
	assert.Equal(t, 4, len(perfData.EquityCurve))
	assert.Equal(t, 3, len(perfData.Returns))
	assert.Equal(t, 11000.0, perfData.PeakEquity)

	// First return: (10500-10000)/10000 = 0.05
	assert.InDelta(t, 0.05, perfData.Returns[0], 0.001)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestLoadEquityCurveEmpty tests loading equity curve with no data
func TestLoadEquityCurveEmpty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"total_value", "metric_time"})
	mock.ExpectQuery("SELECT total_value, metric_time FROM performance_metrics").
		WithArgs(30).
		WillReturnRows(rows)

	ctx := context.Background()
	perfData, err := calculator.LoadEquityCurve(ctx, nil, 30)

	require.NoError(t, err)
	assert.Equal(t, 0, len(perfData.EquityCurve))
	assert.Equal(t, 0, len(perfData.Returns))
	assert.Equal(t, 0.0, perfData.PeakEquity)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCalculateSharpeRatio tests Sharpe ratio calculation
func TestCalculateSharpeRatio(t *testing.T) {
	calculator := NewCalculator(nil) // No DB needed for this test

	// Create mock returns (daily returns)
	returns := []float64{0.01, 0.02, -0.01, 0.015, 0.005, -0.005, 0.02, 0.01}
	riskFreeRate := 0.03 // 3% annual risk-free rate

	sharpe, err := calculator.CalculateSharpeRatio(returns, riskFreeRate)

	require.NoError(t, err)
	assert.Greater(t, sharpe, 0.0) // Should be positive with positive returns

	// With these positive returns, annualized Sharpe should be > 0
	t.Logf("Calculated Sharpe ratio: %.4f", sharpe)
}

// TestCalculateSharpeRatioEmpty tests Sharpe ratio with no returns
func TestCalculateSharpeRatioEmpty(t *testing.T) {
	calculator := NewCalculator(nil)

	returns := []float64{}
	riskFreeRate := 0.03

	_, err := calculator.CalculateSharpeRatio(returns, riskFreeRate)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "returns array is empty")
}

// TestCalculateSharpeRatioZeroStdDev tests Sharpe ratio with zero volatility
func TestCalculateSharpeRatioZeroStdDev(t *testing.T) {
	calculator := NewCalculator(nil)

	// All same returns = zero standard deviation
	returns := []float64{0.01, 0.01, 0.01, 0.01}
	riskFreeRate := 0.03

	_, err := calculator.CalculateSharpeRatio(returns, riskFreeRate)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "standard deviation is zero")
}

// TestDetectMarketRegime tests market regime detection
func TestDetectMarketRegime(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	// Create bullish trend data (prices going up)
	now := time.Now()
	rows := pgxmock.NewRows([]string{"close", "open_time"})
	for i := 0; i < 30; i++ {
		price := 40000.0 + float64(i*500) // Increasing prices
		rows.AddRow(price, now.Add(time.Duration(-29+i)*24*time.Hour))
	}

	mock.ExpectQuery("SELECT close, open_time FROM candlesticks").
		WithArgs("BTC/USDT", "1d", 30).
		WillReturnRows(rows)

	ctx := context.Background()
	regimeData, err := calculator.DetectMarketRegime(ctx, "BTC/USDT", 30)

	require.NoError(t, err)
	assert.Equal(t, "bullish", regimeData.Regime)
	assert.Greater(t, regimeData.ShortMA, 0.0)
	assert.Greater(t, regimeData.LongMA, 0.0)
	assert.Greater(t, regimeData.TrendStrength, 0.0) // Positive trend

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDetectMarketRegimeInsufficientData tests regime detection with too little data
func TestDetectMarketRegimeInsufficientData(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	calculator := NewCalculator(mock)

	rows := pgxmock.NewRows([]string{"close", "open_time"}).
		AddRow(50000.0, time.Now())

	mock.ExpectQuery("SELECT close, open_time FROM candlesticks").
		WithArgs("BTC/USDT", "1d", 30).
		WillReturnRows(rows)

	ctx := context.Background()
	_, err = calculator.DetectMarketRegime(ctx, "BTC/USDT", 30)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCalculateVaRFromReturns tests Value at Risk calculation
func TestCalculateVaRFromReturns(t *testing.T) {
	calculator := NewCalculator(nil)

	// Create returns with some losses
	returns := []float64{
		0.02, 0.01, -0.03, 0.015, -0.02, 0.01, -0.01, 0.02,
		-0.04, 0.01, 0.005, -0.015, 0.02, -0.005, 0.03,
	}
	confidenceLevel := 0.95

	varValue, cvarValue, err := calculator.CalculateVaR(returns, confidenceLevel)

	require.NoError(t, err)
	assert.Greater(t, varValue, 0.0) // VaR should be positive for losses
	assert.GreaterOrEqual(t, cvarValue, varValue) // CVaR should be >= VaR

	t.Logf("VaR (95%%): %.4f, CVaR: %.4f", varValue, cvarValue)
}

// TestCalculateVaRFromReturnsEmpty tests VaR with no returns
func TestCalculateVaRFromReturnsEmpty(t *testing.T) {
	calculator := NewCalculator(nil)

	returns := []float64{}
	confidenceLevel := 0.95

	_, _, err := calculator.CalculateVaR(returns, confidenceLevel)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "returns array is empty")
}

// TestCalculateVaRFromReturnsInvalidConfidence tests VaR with invalid confidence level
func TestCalculateVaRFromReturnsInvalidConfidence(t *testing.T) {
	calculator := NewCalculator(nil)

	returns := []float64{0.01, 0.02, -0.01}

	// Test confidence level > 1
	_, _, err := calculator.CalculateVaR(returns, 1.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "confidence level must be between 0 and 1")

	// Test confidence level <= 0
	_, _, err = calculator.CalculateVaR(returns, 0.0)
	assert.Error(t, err)
}

// TestCalculateDrawdownFromEquity tests drawdown calculation
func TestCalculateDrawdownFromEquity(t *testing.T) {
	calculator := NewCalculator(nil)

	// Create equity curve with drawdown
	equityCurve := []float64{
		10000, 11000, 12000, 11000, 10500, 11500, 12500, 11800,
	}

	currentDD, maxDD, peakEquity := calculator.CalculateDrawdown(equityCurve)

	assert.Greater(t, peakEquity, 0.0)
	assert.Equal(t, 12500.0, peakEquity) // Peak is 12500
	assert.Greater(t, maxDD, 0.0)        // There was a drawdown

	// Current drawdown: (12500 - 11800) / 12500 = 0.056 or 5.6%
	assert.InDelta(t, 0.056, currentDD, 0.01)

	// Max drawdown: from 12000 to 10500 = (12000-10500)/12000 = 0.125 or 12.5%
	assert.Greater(t, maxDD, 0.10)

	t.Logf("Current DD: %.2f%%, Max DD: %.2f%%, Peak: %.2f", currentDD*100, maxDD*100, peakEquity)
}

// TestCalculateDrawdownFromEquityEmpty tests drawdown with empty equity curve
func TestCalculateDrawdownFromEquityEmpty(t *testing.T) {
	calculator := NewCalculator(nil)

	equityCurve := []float64{}

	currentDD, maxDD, peakEquity := calculator.CalculateDrawdown(equityCurve)

	assert.Equal(t, 0.0, currentDD)
	assert.Equal(t, 0.0, maxDD)
	assert.Equal(t, 0.0, peakEquity)
}

// TestCalculateDrawdownFromEquityNoDrawdown tests equity curve with no drawdown
func TestCalculateDrawdownFromEquityNoDrawdown(t *testing.T) {
	calculator := NewCalculator(nil)

	// Steadily increasing equity
	equityCurve := []float64{10000, 11000, 12000, 13000, 14000}

	currentDD, maxDD, peakEquity := calculator.CalculateDrawdown(equityCurve)

	assert.Equal(t, 0.0, currentDD)
	assert.Equal(t, 0.0, maxDD)
	assert.Equal(t, 14000.0, peakEquity)
}

// TestCalculateStdDev tests standard deviation calculation
func TestCalculateStdDev(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	stdDev := calculateStdDev(values)

	// Known standard deviation for this dataset
	expectedStdDev := 2.0
	assert.InDelta(t, expectedStdDev, stdDev, 0.1)
}

// TestCalculateStdDevEmpty tests standard deviation with empty slice
func TestCalculateStdDevEmpty(t *testing.T) {
	values := []float64{}
	stdDev := calculateStdDev(values)
	assert.Equal(t, 0.0, stdDev)
}

// TestCalculateMovingAverage tests moving average calculation
func TestCalculateMovingAverage(t *testing.T) {
	values := []float64{10, 12, 14, 16, 18, 20, 22, 24}
	period := 3

	ma := calculateMovingAverage(values, period)

	// Last 3 values: 20, 22, 24 -> average = 22
	assert.Equal(t, 22.0, ma)
}

// TestCalculateMovingAverageInsufficientData tests MA with insufficient data
func TestCalculateMovingAverageInsufficientData(t *testing.T) {
	values := []float64{10, 12}
	period := 5

	ma := calculateMovingAverage(values, period)
	assert.Equal(t, 0.0, ma)
}

// TestSortReturns tests sorting of returns
func TestSortReturns(t *testing.T) {
	returns := []float64{0.05, -0.02, 0.01, -0.05, 0.03}
	sortReturns(returns)

	// Should be sorted in ascending order
	assert.Equal(t, -0.05, returns[0])
	assert.Equal(t, -0.02, returns[1])
	assert.Equal(t, 0.01, returns[2])
	assert.Equal(t, 0.03, returns[3])
	assert.Equal(t, 0.05, returns[4])
}

// TestSortReturnsEmpty tests sorting empty slice
func TestSortReturnsEmpty(t *testing.T) {
	returns := []float64{}
	sortReturns(returns)
	assert.Equal(t, 0, len(returns))
}

// TestSortReturnsSingle tests sorting single element
func TestSortReturnsSingle(t *testing.T) {
	returns := []float64{0.05}
	sortReturns(returns)
	assert.Equal(t, 1, len(returns))
	assert.Equal(t, 0.05, returns[0])
}
