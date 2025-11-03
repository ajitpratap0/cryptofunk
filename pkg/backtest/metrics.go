// Performance metrics calculation for backtesting
package backtest

import (
	"fmt"
	"math"
	"time"
)

// ============================================================================
// PERFORMANCE METRICS
// ============================================================================

// Metrics holds all performance metrics for a backtest
type Metrics struct {
	// Returns
	TotalReturn      float64 `json:"total_return"`      // Total profit/loss
	TotalReturnPct   float64 `json:"total_return_pct"`  // Total return percentage
	AnnualizedReturn float64 `json:"annualized_return"` // Annualized return percentage
	CAGR             float64 `json:"cagr"`              // Compound Annual Growth Rate

	// Risk metrics
	MaxDrawdown    float64 `json:"max_drawdown"`     // Maximum drawdown in dollars
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Maximum drawdown percentage
	Volatility     float64 `json:"volatility"`       // Standard deviation of returns
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Risk-adjusted return
	SortinoRatio   float64 `json:"sortino_ratio"`    // Downside risk-adjusted return
	CalmarRatio    float64 `json:"calmar_ratio"`     // CAGR / Max Drawdown

	// Trade statistics
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`     // Percentage of winning trades
	AverageWin    float64 `json:"average_win"`  // Average profit per winning trade
	AverageLoss   float64 `json:"average_loss"` // Average loss per losing trade
	LargestWin    float64 `json:"largest_win"`
	LargestLoss   float64 `json:"largest_loss"`
	ProfitFactor  float64 `json:"profit_factor"` // Total profit / Total loss
	Expectancy    float64 `json:"expectancy"`    // Expected value per trade

	// Time statistics
	AverageHoldingTime time.Duration `json:"average_holding_time"`
	MedianHoldingTime  time.Duration `json:"median_holding_time"`
	MaxHoldingTime     time.Duration `json:"max_holding_time"`
	MinHoldingTime     time.Duration `json:"min_holding_time"`

	// Portfolio statistics
	InitialCapital float64       `json:"initial_capital"`
	FinalEquity    float64       `json:"final_equity"`
	PeakEquity     float64       `json:"peak_equity"`
	EquityLow      float64       `json:"equity_low"`
	StartDate      time.Time     `json:"start_date"`
	EndDate        time.Time     `json:"end_date"`
	Duration       time.Duration `json:"duration"`
}

// CalculateMetrics calculates all performance metrics from a backtest
func CalculateMetrics(engine *Engine) (*Metrics, error) {
	if len(engine.EquityCurve) == 0 {
		return nil, fmt.Errorf("no equity curve data")
	}

	metrics := &Metrics{
		InitialCapital: engine.InitialCapital,
		FinalEquity:    engine.GetCurrentEquity(),
		PeakEquity:     engine.PeakEquity,
		TotalTrades:    engine.TotalTrades,
		WinningTrades:  engine.WinningTrades,
		LosingTrades:   engine.LosingTrades,
		MaxDrawdown:    engine.MaxDrawdown,
		MaxDrawdownPct: engine.MaxDrawdownPct,
		StartDate:      engine.EquityCurve[0].Timestamp,
		EndDate:        engine.EquityCurve[len(engine.EquityCurve)-1].Timestamp,
	}

	metrics.Duration = metrics.EndDate.Sub(metrics.StartDate)

	// Calculate returns
	metrics.TotalReturn = metrics.FinalEquity - metrics.InitialCapital
	metrics.TotalReturnPct = (metrics.TotalReturn / metrics.InitialCapital) * 100.0

	// Calculate annualized return and CAGR
	if metrics.Duration > 0 {
		years := metrics.Duration.Hours() / 24.0 / 365.25
		if years > 0 {
			metrics.CAGR = (math.Pow(metrics.FinalEquity/metrics.InitialCapital, 1.0/years) - 1.0) * 100.0
			metrics.AnnualizedReturn = metrics.CAGR
		}
	}

	// Calculate trade statistics
	if len(engine.ClosedPositions) > 0 {
		calculateTradeStatistics(metrics, engine.ClosedPositions)
	}

	// Calculate risk metrics
	calculateRiskMetrics(metrics, engine.EquityCurve)

	// Calculate ratios
	if metrics.Volatility > 0 {
		metrics.SharpeRatio = (metrics.AnnualizedReturn - 3.0) / metrics.Volatility // Assume 3% risk-free rate
	}

	if metrics.MaxDrawdownPct > 0 {
		metrics.CalmarRatio = metrics.CAGR / metrics.MaxDrawdownPct
	}

	// Calculate Sortino ratio (downside deviation)
	calculateSortinoRatio(metrics, engine.EquityCurve)

	// Find equity low
	metrics.EquityLow = metrics.InitialCapital
	for _, point := range engine.EquityCurve {
		if point.Equity < metrics.EquityLow {
			metrics.EquityLow = point.Equity
		}
	}

	return metrics, nil
}

// calculateTradeStatistics calculates statistics from closed positions
func calculateTradeStatistics(metrics *Metrics, positions []*ClosedPosition) {
	var totalWin, totalLoss float64
	var holdingTimes []time.Duration

	for _, pos := range positions {
		holdingTimes = append(holdingTimes, pos.HoldingTime)

		if pos.RealizedPL > 0 {
			totalWin += pos.RealizedPL
			if pos.RealizedPL > metrics.LargestWin {
				metrics.LargestWin = pos.RealizedPL
			}
		} else {
			totalLoss += pos.RealizedPL
			if pos.RealizedPL < metrics.LargestLoss {
				metrics.LargestLoss = pos.RealizedPL
			}
		}
	}

	// Win rate
	if metrics.TotalTrades > 0 {
		metrics.WinRate = (float64(metrics.WinningTrades) / float64(metrics.TotalTrades)) * 100.0
	}

	// Average win/loss
	if metrics.WinningTrades > 0 {
		metrics.AverageWin = totalWin / float64(metrics.WinningTrades)
	}

	if metrics.LosingTrades > 0 {
		metrics.AverageLoss = totalLoss / float64(metrics.LosingTrades)
	}

	// Profit factor
	if totalLoss != 0 {
		metrics.ProfitFactor = totalWin / math.Abs(totalLoss)
	}

	// Expectancy (expected value per trade)
	if metrics.TotalTrades > 0 {
		winProb := float64(metrics.WinningTrades) / float64(metrics.TotalTrades)
		lossProb := float64(metrics.LosingTrades) / float64(metrics.TotalTrades)
		metrics.Expectancy = (winProb * metrics.AverageWin) + (lossProb * metrics.AverageLoss)
	}

	// Holding time statistics
	if len(holdingTimes) > 0 {
		var totalTime time.Duration
		for _, t := range holdingTimes {
			totalTime += t
		}
		metrics.AverageHoldingTime = totalTime / time.Duration(len(holdingTimes))

		// Find min/max
		metrics.MinHoldingTime = holdingTimes[0]
		metrics.MaxHoldingTime = holdingTimes[0]
		for _, t := range holdingTimes {
			if t < metrics.MinHoldingTime {
				metrics.MinHoldingTime = t
			}
			if t > metrics.MaxHoldingTime {
				metrics.MaxHoldingTime = t
			}
		}

		// Median (simplified - sort and take middle)
		// For production, use a proper median calculation
		metrics.MedianHoldingTime = metrics.AverageHoldingTime // Placeholder
	}
}

// calculateRiskMetrics calculates volatility and related metrics
func calculateRiskMetrics(metrics *Metrics, equityCurve []*EquityPoint) {
	if len(equityCurve) < 2 {
		return
	}

	// Calculate daily returns
	var returns []float64
	for i := 1; i < len(equityCurve); i++ {
		prevEquity := equityCurve[i-1].Equity
		currentEquity := equityCurve[i].Equity
		dailyReturn := (currentEquity - prevEquity) / prevEquity
		returns = append(returns, dailyReturn)
	}

	if len(returns) == 0 {
		return
	}

	// Calculate mean return
	var sumReturns float64
	for _, r := range returns {
		sumReturns += r
	}
	meanReturn := sumReturns / float64(len(returns))

	// Calculate variance
	var sumSquaredDiff float64
	for _, r := range returns {
		diff := r - meanReturn
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(returns))

	// Volatility (standard deviation) - annualized
	stdDev := math.Sqrt(variance)
	metrics.Volatility = stdDev * math.Sqrt(252) * 100.0 // Annualized, in percentage
}

// calculateSortinoRatio calculates the Sortino ratio (downside deviation)
func calculateSortinoRatio(metrics *Metrics, equityCurve []*EquityPoint) {
	if len(equityCurve) < 2 {
		return
	}

	// Calculate daily returns (only negative ones)
	var negativeReturns []float64
	for i := 1; i < len(equityCurve); i++ {
		prevEquity := equityCurve[i-1].Equity
		currentEquity := equityCurve[i].Equity
		dailyReturn := (currentEquity - prevEquity) / prevEquity

		if dailyReturn < 0 {
			negativeReturns = append(negativeReturns, dailyReturn)
		}
	}

	if len(negativeReturns) == 0 {
		metrics.SortinoRatio = 0 // No downside risk
		return
	}

	// Calculate downside deviation
	var sumSquaredNegReturns float64
	for _, r := range negativeReturns {
		sumSquaredNegReturns += r * r
	}
	downsideVariance := sumSquaredNegReturns / float64(len(negativeReturns))
	downsideDeviation := math.Sqrt(downsideVariance) * math.Sqrt(252) * 100.0 // Annualized

	if downsideDeviation > 0 {
		metrics.SortinoRatio = (metrics.AnnualizedReturn - 3.0) / downsideDeviation
	}
}

// ============================================================================
// REPORT GENERATION
// ============================================================================

// GenerateReport generates a human-readable performance report
func GenerateReport(metrics *Metrics) string {
	report := fmt.Sprintf(`
================================================================================
BACKTEST PERFORMANCE REPORT
================================================================================

OVERVIEW
--------
Period:           %s to %s (%.0f days)
Initial Capital:  $%.2f
Final Equity:     $%.2f
Peak Equity:      $%.2f
Equity Low:       $%.2f

RETURNS
-------
Total Return:     $%.2f (%.2f%%)
Annualized Return: %.2f%%
CAGR:             %.2f%%

RISK METRICS
------------
Max Drawdown:     $%.2f (%.2f%%)
Volatility:       %.2f%%
Sharpe Ratio:     %.2f
Sortino Ratio:    %.2f
Calmar Ratio:     %.2f

TRADE STATISTICS
----------------
Total Trades:     %d
Winning Trades:   %d
Losing Trades:    %d
Win Rate:         %.2f%%

Average Win:      $%.2f
Average Loss:     $%.2f
Largest Win:      $%.2f
Largest Loss:     $%.2f

Profit Factor:    %.2f
Expectancy:       $%.2f per trade

HOLDING TIMES
-------------
Average:          %s
Median:           %s
Min:              %s
Max:              %s

================================================================================
`,
		metrics.StartDate.Format("2006-01-02"),
		metrics.EndDate.Format("2006-01-02"),
		metrics.Duration.Hours()/24,
		metrics.InitialCapital,
		metrics.FinalEquity,
		metrics.PeakEquity,
		metrics.EquityLow,
		metrics.TotalReturn,
		metrics.TotalReturnPct,
		metrics.AnnualizedReturn,
		metrics.CAGR,
		metrics.MaxDrawdown,
		metrics.MaxDrawdownPct,
		metrics.Volatility,
		metrics.SharpeRatio,
		metrics.SortinoRatio,
		metrics.CalmarRatio,
		metrics.TotalTrades,
		metrics.WinningTrades,
		metrics.LosingTrades,
		metrics.WinRate,
		metrics.AverageWin,
		metrics.AverageLoss,
		metrics.LargestWin,
		metrics.LargestLoss,
		metrics.ProfitFactor,
		metrics.Expectancy,
		formatDuration(metrics.AverageHoldingTime),
		formatDuration(metrics.MedianHoldingTime),
		formatDuration(metrics.MinHoldingTime),
		formatDuration(metrics.MaxHoldingTime),
	)

	return report
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}
