package risk

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// PoolInterface defines the interface for database pool operations
type PoolInterface interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// Calculator provides database-backed risk calculations
type Calculator struct {
	pool PoolInterface
}

// NewCalculator creates a new risk calculator with database connection
func NewCalculator(pool PoolInterface) *Calculator {
	return &Calculator{
		pool: pool,
	}
}

// NewCalculatorWithPool creates a new risk calculator with pgxpool.Pool
func NewCalculatorWithPool(pool *pgxpool.Pool) *Calculator {
	return &Calculator{
		pool: pool,
	}
}

// HistoricalData holds historical market data for risk calculations
type HistoricalData struct {
	Prices  []float64
	Returns []float64
	Times   []time.Time
}

// PerformanceData holds portfolio performance data
type PerformanceData struct {
	EquityCurve []float64
	Returns     []float64
	PeakEquity  float64
	Timestamps  []time.Time
}

// WinRateData holds win rate statistics
type WinRateData struct {
	WinRate       float64
	WinningTrades int64
	LosingTrades  int64
	TotalTrades   int64
	AvgWin        float64
	AvgLoss       float64
}

// MarketRegimeData holds market regime information
type MarketRegimeData struct {
	Regime        string // "bullish", "bearish", "sideways"
	Volatility    float64
	ShortMA       float64
	LongMA        float64
	TrendStrength float64
}

// ============================================================================
// HISTORICAL PRICE DATA
// ============================================================================

// LoadHistoricalPrices loads historical prices from candlesticks table
// Uses TimescaleDB hypertable for efficient time-series queries
func (c *Calculator) LoadHistoricalPrices(ctx context.Context, symbol string, interval string, days int) (*HistoricalData, error) {
	// Return error if no pool available
	if c.pool == nil {
		return nil, fmt.Errorf("no database pool available")
	}

	query := `
		SELECT close, open_time
		FROM candlesticks
		WHERE symbol = $1
			AND interval = $2
			AND open_time >= NOW() - INTERVAL '1 day' * $3
		ORDER BY open_time ASC
	`

	rows, err := c.pool.Query(ctx, query, symbol, interval, days)
	if err != nil {
		return nil, fmt.Errorf("failed to query historical prices: %w", err)
	}
	defer rows.Close()

	var prices []float64
	var times []time.Time

	for rows.Next() {
		var price float64
		var openTime time.Time
		if err := rows.Scan(&price, &openTime); err != nil {
			return nil, fmt.Errorf("failed to scan price row: %w", err)
		}
		prices = append(prices, price)
		times = append(times, openTime)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating price rows: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no historical prices found for %s", symbol)
	}

	// Calculate returns from prices
	returns := make([]float64, 0, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] > 0 {
			ret := (prices[i] - prices[i-1]) / prices[i-1]
			returns = append(returns, ret)
		}
	}

	log.Debug().
		Str("symbol", symbol).
		Int("data_points", len(prices)).
		Int("returns", len(returns)).
		Msg("Historical prices loaded from database")

	return &HistoricalData{
		Prices:  prices,
		Returns: returns,
		Times:   times,
	}, nil
}

// GetCurrentPrice gets the most recent price for a symbol
func (c *Calculator) GetCurrentPrice(ctx context.Context, symbol string, interval string) (float64, error) {
	// Return error if no pool available
	if c.pool == nil {
		return 0, fmt.Errorf("no database pool available")
	}

	query := `
		SELECT close
		FROM candlesticks
		WHERE symbol = $1
			AND interval = $2
		ORDER BY open_time DESC
		LIMIT 1
	`

	var price float64
	err := c.pool.QueryRow(ctx, query, symbol, interval).Scan(&price)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("no price data found for symbol %s with interval %s", symbol, interval)
		}
		return 0, fmt.Errorf("failed to get current price: %w", err)
	}

	return price, nil
}

// ============================================================================
// WIN RATE CALCULATIONS
// ============================================================================

// CalculateWinRate calculates historical win rate from positions table
// Analyzes closed positions to determine win/loss statistics
func (c *Calculator) CalculateWinRate(ctx context.Context, symbol string) (*WinRateData, error) {
	// Return defaults if no pool available (for testing)
	if c.pool == nil {
		log.Warn().Str("symbol", symbol).Msg("No database pool available, using default win rate")
		return &WinRateData{
			WinRate:       0.55,
			WinningTrades: 0,
			LosingTrades:  0,
			TotalTrades:   0,
			AvgWin:        200.0,
			AvgLoss:       100.0,
		}, nil
	}

	query := `
		SELECT
			COUNT(*) FILTER (WHERE realized_pnl > 0) AS winning_trades,
			COUNT(*) FILTER (WHERE realized_pnl < 0) AS losing_trades,
			COUNT(*) AS total_trades,
			COALESCE(AVG(realized_pnl) FILTER (WHERE realized_pnl > 0), 0) AS avg_win,
			COALESCE(ABS(AVG(realized_pnl) FILTER (WHERE realized_pnl < 0)), 0) AS avg_loss
		FROM positions
		WHERE exit_time IS NOT NULL
			AND realized_pnl IS NOT NULL
	`

	args := []interface{}{}
	if symbol != "" {
		query += " AND symbol = $1"
		args = append(args, symbol)
	}

	var winningTrades, losingTrades, totalTrades int64
	var avgWin, avgLoss float64

	err := c.pool.QueryRow(ctx, query, args...).Scan(
		&winningTrades,
		&losingTrades,
		&totalTrades,
		&avgWin,
		&avgLoss,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate win rate: %w", err)
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades)
	}

	// Use defaults if no historical data
	if totalTrades == 0 {
		log.Warn().Str("symbol", symbol).Msg("No historical trades found, using default win rate")
		return &WinRateData{
			WinRate:       0.55, // Default 55% win rate
			WinningTrades: 0,
			LosingTrades:  0,
			TotalTrades:   0,
			AvgWin:        200.0, // Default $200 average win
			AvgLoss:       100.0, // Default $100 average loss
		}, nil
	}

	log.Debug().
		Str("symbol", symbol).
		Int64("winning", winningTrades).
		Int64("losing", losingTrades).
		Float64("win_rate", winRate).
		Float64("avg_win", avgWin).
		Float64("avg_loss", avgLoss).
		Msg("Win rate calculated from database")

	return &WinRateData{
		WinRate:       winRate,
		WinningTrades: winningTrades,
		LosingTrades:  losingTrades,
		TotalTrades:   totalTrades,
		AvgWin:        avgWin,
		AvgLoss:       avgLoss,
	}, nil
}

// ============================================================================
// EQUITY CURVE AND PERFORMANCE METRICS
// ============================================================================

// LoadEquityCurve loads equity curve from performance_metrics table
// Uses TimescaleDB hypertable for efficient time-series queries
func (c *Calculator) LoadEquityCurve(ctx context.Context, sessionID *string, days int) (*PerformanceData, error) {
	// Return empty data if no pool available (for testing)
	if c.pool == nil {
		log.Warn().Msg("No database pool available, returning empty equity curve")
		return &PerformanceData{
			EquityCurve: []float64{},
			Returns:     []float64{},
			PeakEquity:  0,
			Timestamps:  []time.Time{},
		}, nil
	}

	query := `
		SELECT total_value, metric_time
		FROM performance_metrics
		WHERE metric_time >= NOW() - INTERVAL '1 day' * $1
	`

	args := []interface{}{days}
	if sessionID != nil && *sessionID != "" {
		query += " AND session_id = $2"
		args = append(args, *sessionID)
	}

	query += " ORDER BY metric_time ASC"

	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query equity curve: %w", err)
	}
	defer rows.Close()

	var equityCurve []float64
	var timestamps []time.Time
	var peakEquity float64

	for rows.Next() {
		var totalValue float64
		var metricTime time.Time
		if err := rows.Scan(&totalValue, &metricTime); err != nil {
			return nil, fmt.Errorf("failed to scan equity row: %w", err)
		}
		equityCurve = append(equityCurve, totalValue)
		timestamps = append(timestamps, metricTime)
		if totalValue > peakEquity {
			peakEquity = totalValue
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating equity rows: %w", err)
	}

	if len(equityCurve) == 0 {
		log.Warn().Msg("No equity curve data found, returning empty")
		return &PerformanceData{
			EquityCurve: []float64{},
			Returns:     []float64{},
			PeakEquity:  0,
			Timestamps:  []time.Time{},
		}, nil
	}

	// Calculate returns from equity curve
	returns := make([]float64, 0, len(equityCurve)-1)
	for i := 1; i < len(equityCurve); i++ {
		if equityCurve[i-1] > 0 {
			ret := (equityCurve[i] - equityCurve[i-1]) / equityCurve[i-1]
			returns = append(returns, ret)
		}
	}

	log.Debug().
		Int("data_points", len(equityCurve)).
		Float64("peak_equity", peakEquity).
		Int("returns", len(returns)).
		Msg("Equity curve loaded from database")

	return &PerformanceData{
		EquityCurve: equityCurve,
		Returns:     returns,
		PeakEquity:  peakEquity,
		Timestamps:  timestamps,
	}, nil
}

// ============================================================================
// SHARPE RATIO CALCULATION
// ============================================================================

// CalculateSharpeRatio calculates Sharpe ratio from real returns
// Sharpe Ratio = (Mean Return - Risk-Free Rate) / Standard Deviation
func (c *Calculator) CalculateSharpeRatio(returns []float64, riskFreeRate float64) (float64, error) {
	if len(returns) == 0 {
		return 0, fmt.Errorf("returns array is empty")
	}

	// Calculate mean return
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	meanReturn := sum / float64(len(returns))

	// Calculate standard deviation using sample variance (Bessel's correction)
	variance := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		variance += diff * diff
	}
	// Use N-1 for sample variance (Bessel's correction)
	if len(returns) > 1 {
		variance /= float64(len(returns) - 1)
	} else {
		variance /= float64(len(returns))
	}
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0, fmt.Errorf("standard deviation is zero")
	}

	// Annualize (assuming daily returns)
	// Trading days per year: 252
	annualizedReturn := meanReturn * 252.0
	annualizedStdDev := stdDev * math.Sqrt(252.0)

	// Sharpe ratio = (Return - RiskFreeRate) / StdDev
	sharpe := (annualizedReturn - riskFreeRate) / annualizedStdDev

	log.Debug().
		Float64("mean_return", meanReturn).
		Float64("std_dev", stdDev).
		Float64("annualized_return", annualizedReturn).
		Float64("annualized_std_dev", annualizedStdDev).
		Float64("sharpe_ratio", sharpe).
		Msg("Sharpe ratio calculated from real returns")

	return sharpe, nil
}

// CalculateSharpeFromEquity calculates Sharpe ratio directly from equity curve
func (c *Calculator) CalculateSharpeFromEquity(ctx context.Context, sessionID *string, days int, riskFreeRate float64) (float64, error) {
	perfData, err := c.LoadEquityCurve(ctx, sessionID, days)
	if err != nil {
		return 0, fmt.Errorf("failed to load equity curve: %w", err)
	}

	if len(perfData.Returns) == 0 {
		return 0, fmt.Errorf("no returns available")
	}

	return c.CalculateSharpeRatio(perfData.Returns, riskFreeRate)
}

// ============================================================================
// MARKET REGIME DETECTION
// ============================================================================

// DetectMarketRegime detects market regime using 30-day rolling volatility
// Uses moving averages and volatility to determine bullish/bearish/sideways
func (c *Calculator) DetectMarketRegime(ctx context.Context, symbol string, days int) (*MarketRegimeData, error) {
	// Load historical prices for the symbol
	histData, err := c.LoadHistoricalPrices(ctx, symbol, "1d", days)
	if err != nil {
		return nil, fmt.Errorf("failed to load historical data: %w", err)
	}

	if len(histData.Prices) < 20 {
		return nil, fmt.Errorf("insufficient data for regime detection (need 20+ days, got %d)", len(histData.Prices))
	}

	// Calculate volatility (standard deviation of returns)
	volatility := calculateStdDev(histData.Returns)

	// Calculate moving averages
	shortMA := calculateMovingAverage(histData.Prices, 10) // 10-day MA
	longMA := calculateMovingAverage(histData.Prices, 20)  // 20-day MA

	// Determine trend and regime
	prices := histData.Prices
	currentPrice := prices[len(prices)-1]
	startPrice := prices[0]

	// Calculate price trend with zero-check to avoid division by zero
	priceTrend := 0.0
	if startPrice > 0 {
		priceTrend = (currentPrice - startPrice) / startPrice
	}

	maTrend := 0.0
	if longMA > 0 {
		maTrend = (shortMA - longMA) / longMA
	}

	// Calculate trend strength (combination of price trend and MA trend)
	trendStrength := (priceTrend + maTrend) / 2.0

	// Determine regime based on trends and volatility
	var regime string
	if maTrend > 0.02 && priceTrend > 0 {
		regime = "bullish"
	} else if maTrend < -0.02 && priceTrend < 0 {
		regime = "bearish"
	} else {
		regime = "sideways"
	}

	// High volatility can override trend signals
	if volatility > 0.05 { // 5% daily volatility is very high
		if regime == "sideways" {
			regime = "volatile_sideways"
		}
	}

	log.Info().
		Str("symbol", symbol).
		Str("regime", regime).
		Float64("volatility", volatility).
		Float64("short_ma", shortMA).
		Float64("long_ma", longMA).
		Float64("trend_strength", trendStrength).
		Msg("Market regime detected from database")

	return &MarketRegimeData{
		Regime:        regime,
		Volatility:    volatility,
		ShortMA:       shortMA,
		LongMA:        longMA,
		TrendStrength: trendStrength,
	}, nil
}

// ============================================================================
// VALUE AT RISK (VAR) CALCULATION
// ============================================================================

// CalculateVaR calculates Value at Risk from historical returns
// VaR represents the maximum expected loss at a given confidence level
// Uses historical simulation method with the 95th percentile
func (c *Calculator) CalculateVaR(returns []float64, confidenceLevel float64) (float64, float64, error) {
	if len(returns) == 0 {
		return 0, 0, fmt.Errorf("returns array is empty")
	}

	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		return 0, 0, fmt.Errorf("confidence level must be between 0 and 1")
	}

	// Sort returns in ascending order
	sortedReturns := make([]float64, len(returns))
	copy(sortedReturns, returns)
	sortReturns(sortedReturns)

	// Find the percentile corresponding to (1 - confidence_level)
	// For 95% confidence, we look at the 5th percentile (worst 5% of returns)
	percentile := 1 - confidenceLevel
	index := int(float64(len(sortedReturns)) * percentile)
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	// VaR is the negative of the percentile return (positive for losses)
	varValue := -sortedReturns[index]

	// Calculate CVaR (Conditional VaR / Expected Shortfall)
	// This is the average of all losses worse than VaR
	var cvarSum float64
	cvarCount := 0
	for i := 0; i <= index; i++ {
		cvarSum += sortedReturns[i]
		cvarCount++
	}
	cvarValue := 0.0
	if cvarCount > 0 {
		cvarValue = -cvarSum / float64(cvarCount)
	}

	log.Debug().
		Int("returns_count", len(returns)).
		Float64("confidence_level", confidenceLevel).
		Float64("var", varValue).
		Float64("cvar", cvarValue).
		Msg("VaR calculated from historical returns")

	return varValue, cvarValue, nil
}

// CalculateVaRFromEquity calculates VaR from equity curve returns
func (c *Calculator) CalculateVaRFromEquity(ctx context.Context, sessionID *string, days int, confidenceLevel float64) (float64, float64, error) {
	perfData, err := c.LoadEquityCurve(ctx, sessionID, days)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to load equity curve: %w", err)
	}

	if len(perfData.Returns) == 0 {
		return 0, 0, fmt.Errorf("no returns available")
	}

	return c.CalculateVaR(perfData.Returns, confidenceLevel)
}

// CalculateVaRFromPrices calculates VaR from historical price returns
func (c *Calculator) CalculateVaRFromPrices(ctx context.Context, symbol string, interval string, days int, confidenceLevel float64) (float64, float64, error) {
	histData, err := c.LoadHistoricalPrices(ctx, symbol, interval, days)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to load historical prices: %w", err)
	}

	if len(histData.Returns) == 0 {
		return 0, 0, fmt.Errorf("no returns available")
	}

	return c.CalculateVaR(histData.Returns, confidenceLevel)
}

// ============================================================================
// DRAWDOWN CALCULATIONS
// ============================================================================

// CalculateDrawdown calculates current and maximum drawdown from equity curve
func (c *Calculator) CalculateDrawdown(equityCurve []float64) (currentDD float64, maxDD float64, peakEquity float64) {
	if len(equityCurve) == 0 {
		return 0, 0, 0
	}

	peak := equityCurve[0]
	currentEquity := equityCurve[len(equityCurve)-1]

	for _, equity := range equityCurve {
		if equity > peak {
			peak = equity
		}

		if peak > 0 {
			dd := (peak - equity) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
	}

	// Calculate current drawdown
	if currentEquity < peak && peak > 0 {
		currentDD = (peak - currentEquity) / peak
	}

	return currentDD, maxDD, peak
}

// CalculateDrawdownFromDB calculates drawdown from database equity curve
func (c *Calculator) CalculateDrawdownFromDB(ctx context.Context, sessionID *string, days int) (currentDD float64, maxDD float64, peakEquity float64, err error) {
	perfData, err := c.LoadEquityCurve(ctx, sessionID, days)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to load equity curve: %w", err)
	}

	if len(perfData.EquityCurve) == 0 {
		return 0, 0, 0, nil
	}

	currentDD, maxDD, peakEquity = c.CalculateDrawdown(perfData.EquityCurve)
	return currentDD, maxDD, peakEquity, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// calculateStdDev calculates standard deviation of a slice
func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance using sample variance (Bessel's correction)
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	// Use N-1 for sample variance (Bessel's correction)
	if len(values) > 1 {
		variance /= float64(len(values) - 1)
	} else {
		variance /= float64(len(values))
	}

	return math.Sqrt(variance)
}

// calculateMovingAverage calculates simple moving average
func calculateMovingAverage(values []float64, period int) float64 {
	if len(values) < period || period <= 0 {
		return 0
	}

	// Use most recent 'period' values
	sum := 0.0
	start := len(values) - period
	for i := start; i < len(values); i++ {
		sum += values[i]
	}

	return sum / float64(period)
}

// sortReturns sorts returns in ascending order using stdlib slices.Sort
func sortReturns(returns []float64) {
	slices.Sort(returns)
}
