package risk

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// Service provides risk management calculations
type Service struct {
	// Add any necessary dependencies here
}

// NewService creates a new risk service
func NewService() *Service {
	return &Service{}
}

// CalculatePositionSize calculates optimal position size using Kelly Criterion
// Kelly Criterion: f = (p * (b + 1) - 1) / b
// where: f = fraction to bet, p = win probability, b = win/loss ratio
func (s *Service) CalculatePositionSize(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CalculatePositionSize called")

	// Extract parameters
	winRate, ok := args["win_rate"].(float64)
	if !ok {
		return nil, fmt.Errorf("win_rate must be a number")
	}

	avgWin, ok := args["avg_win"].(float64)
	if !ok {
		return nil, fmt.Errorf("avg_win must be a number")
	}

	avgLoss, ok := args["avg_loss"].(float64)
	if !ok {
		return nil, fmt.Errorf("avg_loss must be a number")
	}

	capital, ok := args["capital"].(float64)
	if !ok {
		return nil, fmt.Errorf("capital must be a number")
	}

	// Kelly fraction (default to 0.25 for quarter Kelly)
	kellyFraction := 0.25
	if frac, ok := args["kelly_fraction"].(float64); ok {
		kellyFraction = frac
	}

	// Validate inputs
	if winRate < 0 || winRate > 1 {
		return nil, fmt.Errorf("win_rate must be between 0 and 1")
	}
	if avgWin <= 0 {
		return nil, fmt.Errorf("avg_win must be positive")
	}
	if avgLoss <= 0 {
		return nil, fmt.Errorf("avg_loss must be positive")
	}
	if capital <= 0 {
		return nil, fmt.Errorf("capital must be positive")
	}
	if kellyFraction < 0 || kellyFraction > 1 {
		return nil, fmt.Errorf("kelly_fraction must be between 0 and 1")
	}

	// Calculate Kelly Criterion
	// f = (p * b - q) / b, where b = avg_win/avg_loss, q = 1-p
	b := avgWin / avgLoss
	q := 1 - winRate
	kellyPercent := (winRate*b - q) / b

	// Apply Kelly fraction to be more conservative
	adjustedKelly := kellyPercent * kellyFraction

	// Ensure non-negative position size
	if adjustedKelly < 0 {
		adjustedKelly = 0
	}

	// Calculate position size
	positionSize := capital * adjustedKelly

	return &PositionSizeResult{
		PositionSize:    positionSize,
		KellyPercent:    kellyPercent * 100,
		AdjustedPercent: adjustedKelly * 100,
		Recommendation:  getPositionRecommendation(adjustedKelly),
	}, nil
}

// CalculateVaR calculates Value at Risk using historical simulation method
// VaR represents the maximum expected loss at a given confidence level
func (s *Service) CalculateVaR(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CalculateVaR called")

	// Extract returns array
	returnsRaw, ok := args["returns"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("returns must be an array")
	}

	// Convert to float64 slice
	returns := make([]float64, len(returnsRaw))
	for i, r := range returnsRaw {
		val, ok := r.(float64)
		if !ok {
			return nil, fmt.Errorf("all returns must be numbers")
		}
		returns[i] = val
	}

	if len(returns) == 0 {
		return nil, fmt.Errorf("returns array cannot be empty")
	}

	// Get confidence level (default 0.95 for 95%)
	confidenceLevel := 0.95
	if cl, ok := args["confidence_level"].(float64); ok {
		confidenceLevel = cl
	}

	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		return nil, fmt.Errorf("confidence_level must be between 0 and 1")
	}

	// Calculate VaR using historical simulation
	// Sort returns in ascending order
	sortedReturns := make([]float64, len(returns))
	copy(sortedReturns, returns)
	sortFloat64s(sortedReturns)

	// Find the percentile corresponding to (1 - confidence_level)
	// For 95% confidence, we look at the 5th percentile (worst 5% of returns)
	percentile := 1 - confidenceLevel
	index := int(float64(len(sortedReturns)) * percentile)
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	varValue := -sortedReturns[index] // VaR is positive for losses

	// Calculate CVaR (Conditional VaR / Expected Shortfall)
	// This is the average of all losses worse than VaR
	var cvarSum float64
	cvarCount := 0
	for i := 0; i <= index; i++ {
		cvarSum += sortedReturns[i]
		cvarCount++
	}
	cvarValue := -cvarSum / float64(cvarCount)

	return &VaRResult{
		VaR:             varValue,
		CVaR:            cvarValue,
		ConfidenceLevel: confidenceLevel * 100,
		Interpretation:  getVaRInterpretation(varValue),
	}, nil
}

// CheckPortfolioLimits checks if a trade violates portfolio risk limits
func (s *Service) CheckPortfolioLimits(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CheckPortfolioLimits called")

	// Extract current positions
	positionsRaw, ok := args["current_positions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("current_positions must be an array")
	}

	// Extract new trade
	newTradeRaw, ok := args["new_trade"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("new_trade must be an object")
	}

	// Extract limits configuration
	limitsRaw, ok := args["limits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("limits must be an object")
	}

	// Parse limits
	limits := parseLimits(limitsRaw)

	// Parse current positions
	positions := parsePositions(positionsRaw)

	// Parse new trade
	newTrade := parseTrade(newTradeRaw)

	// Calculate current portfolio metrics
	totalExposure := calculateTotalExposure(positions)
	symbolExposure := calculateSymbolExposure(positions, newTrade.Symbol)

	// Check if new trade would violate limits
	violations := []string{}

	// Check max position size
	if newTrade.Size > limits.MaxPositionSize {
		violations = append(violations, fmt.Sprintf("Position size %.2f exceeds limit %.2f",
			newTrade.Size, limits.MaxPositionSize))
	}

	// Check max portfolio exposure
	newTotalExposure := totalExposure + newTrade.Size
	if newTotalExposure > limits.MaxTotalExposure {
		violations = append(violations, fmt.Sprintf("Total exposure %.2f would exceed limit %.2f",
			newTotalExposure, limits.MaxTotalExposure))
	}

	// Check max concentration per symbol
	newSymbolExposure := symbolExposure + newTrade.Size
	maxSymbolExposure := limits.MaxTotalExposure * limits.MaxConcentration
	if newSymbolExposure > maxSymbolExposure {
		violations = append(violations, fmt.Sprintf("Symbol concentration %.2f%% would exceed limit %.2f%%",
			(newSymbolExposure/limits.MaxTotalExposure)*100, limits.MaxConcentration*100))
	}

	// Check max open positions
	if len(positions) >= limits.MaxOpenPositions {
		violations = append(violations, fmt.Sprintf("Already at maximum %d open positions",
			limits.MaxOpenPositions))
	}

	// Determine approval
	approved := len(violations) == 0
	reason := "Trade approved"
	if !approved {
		reason = fmt.Sprintf("Trade rejected: %d violations", len(violations))
	}

	return &PortfolioLimitsResult{
		Approved:         approved,
		Reason:           reason,
		Violations:       violations,
		CurrentMetrics:   getCurrentMetrics(positions, totalExposure, limits),
		ProjectedMetrics: getProjectedMetrics(positions, newTrade, newTotalExposure, limits),
	}, nil
}

// CalculateSharpe calculates Sharpe ratio
// Sharpe Ratio = (Average Return - Risk-Free Rate) / Standard Deviation of Returns
func (s *Service) CalculateSharpe(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CalculateSharpe called")

	// Extract returns array
	returnsRaw, ok := args["returns"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("returns must be an array")
	}

	// Convert to float64 slice
	returns := make([]float64, len(returnsRaw))
	for i, r := range returnsRaw {
		val, ok := r.(float64)
		if !ok {
			return nil, fmt.Errorf("all returns must be numbers")
		}
		returns[i] = val
	}

	if len(returns) == 0 {
		return nil, fmt.Errorf("returns array cannot be empty")
	}

	// Get risk-free rate (default 0.03 for 3%)
	riskFreeRate := 0.03
	if rfr, ok := args["risk_free_rate"].(float64); ok {
		riskFreeRate = rfr
	}

	// Calculate average return
	var sum float64
	for _, r := range returns {
		sum += r
	}
	avgReturn := sum / float64(len(returns))

	// Calculate standard deviation
	var varianceSum float64
	for _, r := range returns {
		diff := r - avgReturn
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(returns))
	stdDev := sqrt(variance)

	// Avoid division by zero
	if stdDev == 0 {
		return nil, fmt.Errorf("standard deviation is zero - cannot calculate Sharpe ratio")
	}

	// Calculate Sharpe ratio
	// Assuming returns are already in the same frequency as risk-free rate
	sharpeRatio := (avgReturn - riskFreeRate) / stdDev

	// Annualize if needed (assuming daily returns)
	// Multiply by sqrt(252) for daily to annual conversion
	annualizedSharpe := sharpeRatio * sqrt(252)

	return &SharpeResult{
		SharpeRatio:      sharpeRatio,
		AnnualizedSharpe: annualizedSharpe,
		AvgReturn:        avgReturn * 100,    // Convert to percentage
		StdDev:           stdDev * 100,       // Convert to percentage
		RiskFreeRate:     riskFreeRate * 100, // Convert to percentage
		Interpretation:   getSharpeInterpretation(annualizedSharpe),
	}, nil
}

// CalculateDrawdown calculates drawdown metrics from equity curve
// Drawdown = (Peak - Trough) / Peak
// Maximum Drawdown = largest peak-to-trough decline
func (s *Service) CalculateDrawdown(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CalculateDrawdown called")

	// Extract equity curve
	equityCurveRaw, ok := args["equity_curve"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("equity_curve must be an array")
	}

	// Convert to float64 slice
	equityCurve := make([]float64, len(equityCurveRaw))
	for i, v := range equityCurveRaw {
		val, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("all equity values must be numbers")
		}
		equityCurve[i] = val
	}

	if len(equityCurve) == 0 {
		return nil, fmt.Errorf("equity_curve cannot be empty")
	}

	// Track peak, current drawdown, and maximum drawdown
	var peak float64 = equityCurve[0]
	var maxDrawdown float64 = 0
	var currentDrawdown float64 = 0
	var maxDrawdownStart, maxDrawdownEnd int
	var currentDrawdownStart int

	// Track all drawdown periods for recovery time analysis
	var drawdownPeriods []DrawdownPeriod

	inDrawdown := false
	var currentPeriod DrawdownPeriod

	for i, equity := range equityCurve {
		// Update peak if we have a new high
		if equity > peak {
			// If we were in drawdown, record the recovery
			if inDrawdown {
				currentPeriod.EndIndex = i - 1
				currentPeriod.Duration = currentPeriod.EndIndex - currentPeriod.StartIndex + 1
				drawdownPeriods = append(drawdownPeriods, currentPeriod)
				inDrawdown = false
			}
			peak = equity
			currentDrawdown = 0
		} else {
			// Calculate current drawdown
			currentDrawdown = (peak - equity) / peak

			// Start a new drawdown period if not already in one
			if !inDrawdown {
				inDrawdown = true
				currentPeriod = DrawdownPeriod{
					StartIndex: i,
					StartValue: peak,
				}
			}

			// Update maximum drawdown
			if currentDrawdown > maxDrawdown {
				maxDrawdown = currentDrawdown
				maxDrawdownStart = currentDrawdownStart
				maxDrawdownEnd = i
			}
		}

		// Update current drawdown start if needed
		if currentDrawdown > 0 && (i == 0 || equityCurve[i-1] >= peak) {
			currentDrawdownStart = i
		}
	}

	// If still in drawdown at the end, record it
	if inDrawdown {
		currentPeriod.EndIndex = len(equityCurve) - 1
		currentPeriod.Duration = currentPeriod.EndIndex - currentPeriod.StartIndex + 1
		currentPeriod.Drawdown = currentDrawdown
		drawdownPeriods = append(drawdownPeriods, currentPeriod)
	}

	// Calculate average drawdown
	var totalDrawdown float64
	for _, period := range drawdownPeriods {
		totalDrawdown += period.Drawdown
	}
	avgDrawdown := 0.0
	if len(drawdownPeriods) > 0 {
		avgDrawdown = totalDrawdown / float64(len(drawdownPeriods))
	}

	// Calculate average recovery time
	var totalRecoveryTime int
	recoveredCount := 0
	for _, period := range drawdownPeriods {
		if period.EndIndex < len(equityCurve)-1 { // Only count recovered drawdowns
			totalRecoveryTime += period.Duration
			recoveredCount++
		}
	}
	avgRecoveryTime := 0
	if recoveredCount > 0 {
		avgRecoveryTime = totalRecoveryTime / recoveredCount
	}

	currentEquity := equityCurve[len(equityCurve)-1]
	isInDrawdown := currentEquity < peak

	return &DrawdownResult{
		CurrentDrawdown:  currentDrawdown * 100, // Convert to percentage
		MaxDrawdown:      maxDrawdown * 100,     // Convert to percentage
		AvgDrawdown:      avgDrawdown * 100,     // Convert to percentage
		Peak:             peak,
		CurrentEquity:    currentEquity,
		DrawdownCount:    len(drawdownPeriods),
		AvgRecoveryTime:  avgRecoveryTime,
		IsInDrawdown:     isInDrawdown,
		MaxDrawdownStart: maxDrawdownStart,
		MaxDrawdownEnd:   maxDrawdownEnd,
		Interpretation:   getDrawdownInterpretation(maxDrawdown),
	}, nil
}

// PositionSizeResult represents the result of position size calculation
type PositionSizeResult struct {
	PositionSize    float64 `json:"position_size"`
	KellyPercent    float64 `json:"kelly_percent"`
	AdjustedPercent float64 `json:"adjusted_percent"`
	Recommendation  string  `json:"recommendation"`
}

// VaRResult represents the result of VaR calculation
type VaRResult struct {
	VaR             float64 `json:"var"`
	CVaR            float64 `json:"cvar"`
	ConfidenceLevel float64 `json:"confidence_level"`
	Interpretation  string  `json:"interpretation"`
}

// SharpeResult represents the result of Sharpe ratio calculation
type SharpeResult struct {
	SharpeRatio      float64 `json:"sharpe_ratio"`
	AnnualizedSharpe float64 `json:"annualized_sharpe"`
	AvgReturn        float64 `json:"avg_return"`
	StdDev           float64 `json:"std_dev"`
	RiskFreeRate     float64 `json:"risk_free_rate"`
	Interpretation   string  `json:"interpretation"`
}

// DrawdownResult represents the result of drawdown calculation
type DrawdownResult struct {
	CurrentDrawdown  float64 `json:"current_drawdown"`
	MaxDrawdown      float64 `json:"max_drawdown"`
	AvgDrawdown      float64 `json:"avg_drawdown"`
	Peak             float64 `json:"peak"`
	CurrentEquity    float64 `json:"current_equity"`
	DrawdownCount    int     `json:"drawdown_count"`
	AvgRecoveryTime  int     `json:"avg_recovery_time"`
	IsInDrawdown     bool    `json:"is_in_drawdown"`
	MaxDrawdownStart int     `json:"max_drawdown_start"`
	MaxDrawdownEnd   int     `json:"max_drawdown_end"`
	Interpretation   string  `json:"interpretation"`
}

// DrawdownPeriod represents a single drawdown period
type DrawdownPeriod struct {
	StartIndex int
	EndIndex   int
	StartValue float64
	Drawdown   float64
	Duration   int
}

// getPositionRecommendation provides interpretation of Kelly percentage
func getPositionRecommendation(kellyPercent float64) string {
	percent := kellyPercent * 100

	if percent <= 0 {
		return "No position recommended - negative edge"
	} else if percent <= 2 {
		return "Very small position - minimal edge"
	} else if percent <= 5 {
		return "Conservative position - moderate edge"
	} else if percent <= 10 {
		return "Standard position - good edge"
	} else if percent <= 20 {
		return "Large position - strong edge (monitor risk)"
	} else {
		return "Warning: Very large position suggested - verify calculations and consider further reducing Kelly fraction"
	}
}

// getVaRInterpretation provides interpretation of VaR value
func getVaRInterpretation(varValue float64) string {
	varPercent := varValue * 100

	if varPercent <= 1 {
		return "Very low risk - minimal downside"
	} else if varPercent <= 3 {
		return "Low risk - acceptable for conservative portfolios"
	} else if varPercent <= 5 {
		return "Moderate risk - typical for balanced strategies"
	} else if varPercent <= 10 {
		return "Elevated risk - monitor closely"
	} else if varPercent <= 20 {
		return "High risk - consider position sizing reduction"
	} else {
		return "Very high risk - significant capital at risk"
	}
}

// getSharpeInterpretation provides interpretation of Sharpe ratio
func getSharpeInterpretation(sharpe float64) string {
	if sharpe < 0 {
		return "Poor - negative risk-adjusted returns"
	} else if sharpe < 1.0 {
		return "Sub-optimal - returns below acceptable risk-adjusted level"
	} else if sharpe < 2.0 {
		return "Good - acceptable risk-adjusted returns"
	} else if sharpe < 3.0 {
		return "Very good - strong risk-adjusted returns"
	} else {
		return "Excellent - exceptional risk-adjusted returns"
	}
}

// getDrawdownInterpretation provides interpretation of maximum drawdown
func getDrawdownInterpretation(maxDrawdown float64) string {
	maxDDPercent := maxDrawdown * 100

	if maxDDPercent <= 5 {
		return "Excellent - very low drawdown"
	} else if maxDDPercent <= 10 {
		return "Good - acceptable drawdown for most strategies"
	} else if maxDDPercent <= 20 {
		return "Moderate - typical for aggressive strategies"
	} else if maxDDPercent <= 30 {
		return "High - monitor position sizing and risk"
	} else if maxDDPercent <= 50 {
		return "Very high - significant capital at risk"
	} else {
		return "Extreme - unacceptable drawdown level"
	}
}

// sqrt calculates the square root of a number using Newton's method
func sqrt(x float64) float64 {
	if x < 0 {
		return 0 // Return 0 for negative numbers
	}
	if x == 0 {
		return 0
	}

	// Newton's method for square root
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// PortfolioLimitsResult represents the result of portfolio limits check
type PortfolioLimitsResult struct {
	Approved         bool                   `json:"approved"`
	Reason           string                 `json:"reason"`
	Violations       []string               `json:"violations"`
	CurrentMetrics   map[string]interface{} `json:"current_metrics"`
	ProjectedMetrics map[string]interface{} `json:"projected_metrics"`
}

// Position represents a trading position
type Position struct {
	Symbol string
	Size   float64
}

// Trade represents a proposed trade
type Trade struct {
	Symbol string
	Size   float64
}

// RiskLimits represents portfolio risk limits
type RiskLimits struct {
	MaxPositionSize  float64
	MaxTotalExposure float64
	MaxConcentration float64 // As fraction of total exposure
	MaxOpenPositions int
}

// parseLimits extracts risk limits from raw data
func parseLimits(raw map[string]interface{}) RiskLimits {
	limits := RiskLimits{
		MaxPositionSize:  10000,  // Default
		MaxTotalExposure: 100000, // Default
		MaxConcentration: 0.2,    // Default 20%
		MaxOpenPositions: 10,     // Default
	}

	if v, ok := raw["max_position_size"].(float64); ok {
		limits.MaxPositionSize = v
	}
	if v, ok := raw["max_total_exposure"].(float64); ok {
		limits.MaxTotalExposure = v
	}
	if v, ok := raw["max_concentration"].(float64); ok {
		limits.MaxConcentration = v
	}
	if v, ok := raw["max_open_positions"].(float64); ok {
		limits.MaxOpenPositions = int(v)
	}

	return limits
}

// parsePositions extracts positions from raw data
func parsePositions(raw []interface{}) []Position {
	positions := []Position{}
	for _, p := range raw {
		posMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		symbol, _ := posMap["symbol"].(string)
		size, _ := posMap["size"].(float64)

		positions = append(positions, Position{
			Symbol: symbol,
			Size:   size,
		})
	}
	return positions
}

// parseTrade extracts trade from raw data
func parseTrade(raw map[string]interface{}) Trade {
	symbol, _ := raw["symbol"].(string)
	size, _ := raw["size"].(float64)

	return Trade{
		Symbol: symbol,
		Size:   size,
	}
}

// calculateTotalExposure sums all position sizes
func calculateTotalExposure(positions []Position) float64 {
	total := 0.0
	for _, p := range positions {
		total += p.Size
	}
	return total
}

// calculateSymbolExposure sums exposure for a specific symbol
func calculateSymbolExposure(positions []Position, symbol string) float64 {
	total := 0.0
	for _, p := range positions {
		if p.Symbol == symbol {
			total += p.Size
		}
	}
	return total
}

// getCurrentMetrics returns current portfolio metrics
func getCurrentMetrics(positions []Position, totalExposure float64, limits RiskLimits) map[string]interface{} {
	utilizationPercent := 0.0
	if limits.MaxTotalExposure > 0 {
		utilizationPercent = (totalExposure / limits.MaxTotalExposure) * 100
	}

	return map[string]interface{}{
		"open_positions":      len(positions),
		"total_exposure":      totalExposure,
		"utilization_percent": utilizationPercent,
	}
}

// getProjectedMetrics returns projected metrics after trade
func getProjectedMetrics(positions []Position, trade Trade, newTotalExposure float64, limits RiskLimits) map[string]interface{} {
	newPositionCount := len(positions)

	// Check if this would be a new position
	isNewPosition := true
	for _, p := range positions {
		if p.Symbol == trade.Symbol {
			isNewPosition = false
			break
		}
	}

	if isNewPosition {
		newPositionCount++
	}

	utilizationPercent := 0.0
	if limits.MaxTotalExposure > 0 {
		utilizationPercent = (newTotalExposure / limits.MaxTotalExposure) * 100
	}

	return map[string]interface{}{
		"open_positions":      newPositionCount,
		"total_exposure":      newTotalExposure,
		"utilization_percent": utilizationPercent,
	}
}

// sortFloat64s sorts a float64 slice in ascending order (simple bubble sort for small arrays)
func sortFloat64s(arr []float64) {
	n := len(arr)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if arr[j] > arr[j+1] {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}
