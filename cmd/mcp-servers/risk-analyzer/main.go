//nolint:goconst // MCP tool names are defined by protocol spec
package main

import (
	"context"
	"fmt"
	"math"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
)

const (
	serverName = "risk-analyzer"
)

var serverVersion = config.Version

func main() {
	// Setup logging to stderr (stdout is reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Risk Analyzer MCP Server starting...")

	logger := log.With().Str("server", serverName).Logger()

	srv := mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  logger,
	})

	registerTools(srv)

	if err := srv.Run(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func registerTools(srv *mcpserver.Server) {
	srv.AddToolRaw(
		mcpserver.NewTool("calculate_position_size", "Calculate optimal position size using Kelly Criterion", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"win_rate":       map[string]interface{}{"type": "number", "description": "Win rate as decimal (e.g., 0.55 for 55%)"},
				"avg_win":        map[string]interface{}{"type": "number", "description": "Average winning trade profit"},
				"avg_loss":       map[string]interface{}{"type": "number", "description": "Average losing trade loss (positive number)"},
				"capital":        map[string]interface{}{"type": "number", "description": "Total trading capital"},
				"kelly_fraction": map[string]interface{}{"type": "number", "description": "Fraction of Kelly to use (e.g., 0.5 for half-Kelly)"},
			},
			"required": []string{"win_rate", "avg_win", "avg_loss", "capital", "kelly_fraction"},
		}),
		mcpserver.WrapLegacyHandler(calculatePositionSize),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_var", "Calculate Value at Risk (VaR) for a return series", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"returns":          map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of historical returns"},
				"confidence_level": map[string]interface{}{"type": "number", "description": "Confidence level (e.g., 0.95 for 95%)"},
			},
			"required": []string{"returns", "confidence_level"},
		}),
		mcpserver.WrapLegacyHandler(calculateVaR),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("check_portfolio_limits", "Check if a proposed trade violates portfolio risk limits", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"current_positions": map[string]interface{}{"type": "array", "items": map[string]string{"type": "object"}, "description": "Array of current positions with symbol, quantity, value"},
				"new_trade":         map[string]interface{}{"type": "object", "description": "Proposed trade with symbol, side, quantity, price"},
				"limits":            map[string]interface{}{"type": "object", "description": "Risk limits: max_exposure, max_concentration, max_drawdown"},
			},
			"required": []string{"current_positions", "new_trade", "limits"},
		}),
		mcpserver.WrapLegacyHandler(checkPortfolioLimits),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_sharpe", "Calculate Sharpe ratio for a return series", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"returns":          map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of period returns"},
				"risk_free_rate":   map[string]interface{}{"type": "number", "description": "Risk-free rate (annualized)"},
				"periods_per_year": map[string]interface{}{"type": "number", "description": "Number of periods per year (252 for daily, 12 for monthly)"},
			},
			"required": []string{"returns", "risk_free_rate", "periods_per_year"},
		}),
		mcpserver.WrapLegacyHandler(calculateSharpe),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_drawdown", "Calculate current and maximum drawdown from equity curve", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"equity_curve": map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of equity values over time"},
			},
			"required": []string{"equity_curve"},
		}),
		mcpserver.WrapLegacyHandler(calculateDrawdown),
	)
}

// extractFloat extracts a float64 from the args map
func extractFloat(args map[string]interface{}, key string) (float64, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func calculatePositionSize(_ context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("calculatePositionSize called")

	winRate, err := extractFloat(args, "win_rate")
	if err != nil {
		return nil, err
	}
	if winRate < 0 || winRate > 1 {
		return nil, fmt.Errorf("win_rate must be between 0 and 1 (got %f)", winRate)
	}

	avgWin, err := extractFloat(args, "avg_win")
	if err != nil {
		return nil, err
	}
	if avgWin <= 0 {
		return nil, fmt.Errorf("avg_win must be positive (got %f)", avgWin)
	}

	avgLoss, err := extractFloat(args, "avg_loss")
	if err != nil {
		return nil, err
	}
	if avgLoss <= 0 {
		return nil, fmt.Errorf("avg_loss must be positive (got %f)", avgLoss)
	}

	capital, err := extractFloat(args, "capital")
	if err != nil {
		return nil, err
	}
	if capital <= 0 {
		return nil, fmt.Errorf("capital must be positive (got %f)", capital)
	}

	kellyFraction, err := extractFloat(args, "kelly_fraction")
	if err != nil {
		return nil, err
	}
	if kellyFraction <= 0 || kellyFraction > 1 {
		return nil, fmt.Errorf("kelly_fraction must be between 0 and 1 (got %f)", kellyFraction)
	}

	b := avgWin / avgLoss
	p := winRate
	q := 1 - winRate

	kellyPercentage := (b*p - q) / b

	var adjustedKelly float64
	var recommendation string

	if kellyPercentage < 0 {
		adjustedKelly = 0
		recommendation = "Negative Kelly indicates no statistical edge - position size is 0"
	} else if kellyPercentage > 1 {
		adjustedKelly = 1.0 * kellyFraction
		recommendation = fmt.Sprintf("Kelly > 100%% capped at 100%%, then multiplied by Kelly fraction (%.2f)", kellyFraction)
	} else {
		adjustedKelly = kellyPercentage * kellyFraction
		recommendation = fmt.Sprintf("Using %.0f%% Kelly (%.2f fraction of full Kelly)", adjustedKelly*100, kellyFraction)
	}

	positionSize := adjustedKelly * capital

	return map[string]interface{}{
		"kelly_percentage":  kellyPercentage,
		"adjusted_kelly":    adjustedKelly,
		"position_size":     positionSize,
		"capital":           capital,
		"kelly_fraction":    kellyFraction,
		"recommendation":    recommendation,
		"edge_ratio":        b,
		"win_rate":          winRate,
		"loss_rate":         q,
		"has_positive_edge": kellyPercentage > 0,
	}, nil
}

func calculateVaR(_ context.Context, args map[string]interface{}) (interface{}, error) {
	returnsRaw, ok := args["returns"]
	if !ok {
		return nil, fmt.Errorf("returns is required")
	}

	returnsArray, ok := returnsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("returns must be an array")
	}

	if len(returnsArray) == 0 {
		return nil, fmt.Errorf("returns array cannot be empty")
	}

	returns := make([]float64, len(returnsArray))
	for i, v := range returnsArray {
		switch val := v.(type) {
		case float64:
			returns[i] = val
		case int:
			returns[i] = float64(val)
		case int64:
			returns[i] = float64(val)
		default:
			return nil, fmt.Errorf("returns[%d] must be a number", i)
		}
	}

	confidenceLevel, err := extractFloat(args, "confidence_level")
	if err != nil {
		return nil, err
	}
	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		return nil, fmt.Errorf("confidence_level must be between 0 and 1 (got %f)", confidenceLevel)
	}

	sortedReturns := make([]float64, len(returns))
	copy(sortedReturns, returns)
	for i := 0; i < len(sortedReturns)-1; i++ {
		for j := i + 1; j < len(sortedReturns); j++ {
			if sortedReturns[i] > sortedReturns[j] {
				sortedReturns[i], sortedReturns[j] = sortedReturns[j], sortedReturns[i]
			}
		}
	}

	alpha := 1 - confidenceLevel
	index := int(alpha * float64(len(sortedReturns)))
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	var_ := -sortedReturns[index]

	var sumReturns float64
	for _, r := range returns {
		sumReturns += r
	}
	meanReturn := sumReturns / float64(len(returns))

	var sumSquaredDiff float64
	for _, r := range returns {
		diff := r - meanReturn
		sumSquaredDiff += diff * diff
	}
	stdDev := 0.0
	if len(returns) > 1 {
		variance := sumSquaredDiff / float64(len(returns)-1)
		if variance > 0 {
			stdDev = math.Sqrt(variance)
		}
	}

	exceedances := 0
	for _, r := range returns {
		if r <= -var_ {
			exceedances++
		}
	}

	return map[string]interface{}{
		"var":              var_,
		"confidence_level": confidenceLevel,
		"sample_size":      len(returns),
		"mean_return":      meanReturn,
		"std_dev":          stdDev,
		"exceedances":      exceedances,
		"exceedance_rate":  float64(exceedances) / float64(len(returns)),
		"worst_return":     sortedReturns[0],
		"best_return":      sortedReturns[len(sortedReturns)-1],
		"interpretation":   fmt.Sprintf("With %.0f%% confidence, maximum expected loss is %.4f", confidenceLevel*100, var_),
	}, nil
}

func checkPortfolioLimits(_ context.Context, args map[string]interface{}) (interface{}, error) {
	positionsRaw, ok := args["current_positions"]
	if !ok {
		return nil, fmt.Errorf("current_positions is required")
	}

	positionsArray, ok := positionsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("current_positions must be an array")
	}

	newTradeRaw, ok := args["new_trade"]
	if !ok {
		return nil, fmt.Errorf("new_trade is required")
	}

	newTrade, ok := newTradeRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("new_trade must be an object")
	}

	limitsRaw, ok := args["limits"]
	if !ok {
		return nil, fmt.Errorf("limits is required")
	}

	limits, ok := limitsRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("limits must be an object")
	}

	tradeSymbol, ok := newTrade["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("new_trade.symbol is required and must be a string")
	}

	tradeSide, ok := newTrade["side"].(string)
	if !ok {
		return nil, fmt.Errorf("new_trade.side is required and must be a string")
	}

	var tradeQuantity, tradePrice float64
	if qty, ok := newTrade["quantity"]; ok {
		switch v := qty.(type) {
		case float64:
			tradeQuantity = v
		case int:
			tradeQuantity = float64(v)
		case int64:
			tradeQuantity = float64(v)
		default:
			return nil, fmt.Errorf("new_trade.quantity must be a number")
		}
	} else {
		return nil, fmt.Errorf("new_trade.quantity is required")
	}

	if price, ok := newTrade["price"]; ok {
		switch v := price.(type) {
		case float64:
			tradePrice = v
		case int:
			tradePrice = float64(v)
		case int64:
			tradePrice = float64(v)
		default:
			return nil, fmt.Errorf("new_trade.price must be a number")
		}
	} else {
		return nil, fmt.Errorf("new_trade.price is required")
	}

	if tradeQuantity <= 0 {
		return nil, fmt.Errorf("new_trade.quantity must be positive (got %f)", tradeQuantity)
	}
	if tradePrice <= 0 {
		return nil, fmt.Errorf("new_trade.price must be positive (got %f)", tradePrice)
	}

	var maxExposure, maxConcentration, maxDrawdown float64
	var hasMaxExposure, hasMaxConcentration, hasMaxDrawdown bool

	if exp, ok := limits["max_exposure"]; ok {
		hasMaxExposure = true
		switch v := exp.(type) {
		case float64:
			maxExposure = v
		case int:
			maxExposure = float64(v)
		case int64:
			maxExposure = float64(v)
		default:
			return nil, fmt.Errorf("limits.max_exposure must be a number")
		}
	}

	if conc, ok := limits["max_concentration"]; ok {
		hasMaxConcentration = true
		switch v := conc.(type) {
		case float64:
			maxConcentration = v
		case int:
			maxConcentration = float64(v)
		case int64:
			maxConcentration = float64(v)
		default:
			return nil, fmt.Errorf("limits.max_concentration must be a number")
		}
		if maxConcentration <= 0 || maxConcentration > 1 {
			return nil, fmt.Errorf("limits.max_concentration must be between 0 and 1 (got %f)", maxConcentration)
		}
	}

	if dd, ok := limits["max_drawdown"]; ok {
		hasMaxDrawdown = true
		switch v := dd.(type) {
		case float64:
			maxDrawdown = v
		case int:
			maxDrawdown = float64(v)
		case int64:
			maxDrawdown = float64(v)
		default:
			return nil, fmt.Errorf("limits.max_drawdown must be a number")
		}
	}

	var totalPortfolioValue float64
	positionsBySymbol := make(map[string]float64)

	for _, posRaw := range positionsArray {
		pos, ok := posRaw.(map[string]interface{})
		if !ok {
			continue
		}

		var posValue float64
		if val, ok := pos["value"]; ok {
			switch v := val.(type) {
			case float64:
				posValue = v
			case int:
				posValue = float64(v)
			case int64:
				posValue = float64(v)
			}
		}

		totalPortfolioValue += posValue

		if sym, ok := pos["symbol"].(string); ok {
			positionsBySymbol[sym] += posValue
		}
	}

	tradeValue := tradeQuantity * tradePrice

	switch tradeSide {
	case "BUY":
		positionsBySymbol[tradeSymbol] += tradeValue
	case "SELL":
		positionsBySymbol[tradeSymbol] -= tradeValue
	}

	newTotalValue := totalPortfolioValue
	switch tradeSide {
	case "BUY":
		newTotalValue += tradeValue
	case "SELL":
		newTotalValue -= tradeValue
	}

	violations := []string{}
	checks := make(map[string]interface{})

	if hasMaxExposure {
		exposureViolation := newTotalValue > maxExposure
		checks["exposure_check"] = map[string]interface{}{
			"current_exposure": newTotalValue,
			"max_exposure":     maxExposure,
			"violated":         exposureViolation,
			"utilization":      newTotalValue / maxExposure,
		}
		if exposureViolation {
			violations = append(violations, fmt.Sprintf("Total exposure %.2f exceeds maximum %.2f", newTotalValue, maxExposure))
		}
	}

	if hasMaxConcentration && newTotalValue > 0 {
		var maxSymbolExposure float64
		var maxSymbol string
		for sym, val := range positionsBySymbol {
			if val > maxSymbolExposure {
				maxSymbolExposure = val
				maxSymbol = sym
			}
		}

		concentration := maxSymbolExposure / newTotalValue
		concentrationViolation := concentration > maxConcentration

		checks["concentration_check"] = map[string]interface{}{
			"largest_position":    maxSymbol,
			"position_value":      maxSymbolExposure,
			"concentration":       concentration,
			"max_concentration":   maxConcentration,
			"violated":            concentrationViolation,
			"concentration_ratio": concentration / maxConcentration,
		}

		if concentrationViolation {
			violations = append(violations, fmt.Sprintf("Position concentration %.2f%% exceeds maximum %.2f%%", concentration*100, maxConcentration*100))
		}
	}

	if hasMaxDrawdown {
		checks["drawdown_check"] = map[string]interface{}{
			"max_drawdown":      maxDrawdown,
			"violated":          false,
			"note":              "Drawdown check requires historical equity curve",
			"requires_tracking": true,
		}
	}

	approved := len(violations) == 0

	result := map[string]interface{}{
		"approved":            approved,
		"violations":          violations,
		"checks":              checks,
		"trade_value":         tradeValue,
		"current_portfolio":   totalPortfolioValue,
		"projected_portfolio": newTotalValue,
		"position_count":      len(positionsBySymbol),
		"recommendation":      "",
	}

	if approved {
		result["recommendation"] = "Trade approved - all risk limits satisfied"
	} else {
		result["recommendation"] = fmt.Sprintf("Trade rejected - %d violation(s) detected", len(violations))
	}

	return result, nil
}

func calculateSharpe(_ context.Context, args map[string]interface{}) (interface{}, error) {
	returnsRaw, ok := args["returns"]
	if !ok {
		return nil, fmt.Errorf("returns is required")
	}

	returnsArray, ok := returnsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("returns must be an array")
	}

	if len(returnsArray) == 0 {
		return nil, fmt.Errorf("returns array cannot be empty")
	}

	returns := make([]float64, len(returnsArray))
	for i, r := range returnsArray {
		switch v := r.(type) {
		case float64:
			returns[i] = v
		case int:
			returns[i] = float64(v)
		case int64:
			returns[i] = float64(v)
		default:
			return nil, fmt.Errorf("returns[%d] must be a number", i)
		}
	}

	riskFreeRate, err := extractFloat(args, "risk_free_rate")
	if err != nil {
		return nil, err
	}

	periodsPerYear, err := extractFloat(args, "periods_per_year")
	if err != nil {
		return nil, err
	}
	if periodsPerYear <= 0 {
		return nil, fmt.Errorf("periods_per_year must be positive (got %f)", periodsPerYear)
	}

	var sum float64
	for _, r := range returns {
		sum += r
	}
	meanReturn := sum / float64(len(returns))

	var varianceSum float64
	for _, r := range returns {
		diff := r - meanReturn
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(returns))
	stdDev := math.Sqrt(variance)

	riskFreeRatePerPeriod := riskFreeRate / periodsPerYear

	var sharpeRatio float64
	if stdDev == 0 {
		if meanReturn > riskFreeRatePerPeriod {
			sharpeRatio = math.Inf(1)
		} else if meanReturn < riskFreeRatePerPeriod {
			sharpeRatio = math.Inf(-1)
		} else {
			sharpeRatio = 0
		}
	} else {
		sharpeRatio = (meanReturn - riskFreeRatePerPeriod) / stdDev
	}

	annualizedSharpe := sharpeRatio * math.Sqrt(periodsPerYear)
	annualizedReturn := meanReturn * periodsPerYear
	annualizedVolatility := stdDev * math.Sqrt(periodsPerYear)

	var interpretation string
	switch {
	case annualizedSharpe < 0:
		interpretation = "Poor - returns below risk-free rate"
	case annualizedSharpe < 1:
		interpretation = "Sub-optimal - excess return doesn't adequately compensate for risk"
	case annualizedSharpe < 2:
		interpretation = "Good - adequate risk-adjusted returns"
	case annualizedSharpe < 3:
		interpretation = "Very Good - strong risk-adjusted returns"
	default:
		interpretation = "Excellent - exceptional risk-adjusted returns"
	}

	// Sanitize Inf/NaN for JSON serialization
	sanitize := func(v float64) interface{} {
		if math.IsInf(v, 1) {
			return "Infinity"
		}
		if math.IsInf(v, -1) {
			return "-Infinity"
		}
		if math.IsNaN(v) {
			return "NaN"
		}
		return v
	}

	return map[string]interface{}{
		"sharpe_ratio":          sanitize(annualizedSharpe),
		"sharpe_ratio_period":   sanitize(sharpeRatio),
		"mean_return":           meanReturn,
		"annualized_return":     annualizedReturn,
		"std_dev":               stdDev,
		"annualized_volatility": annualizedVolatility,
		"risk_free_rate":        riskFreeRate,
		"periods_per_year":      periodsPerYear,
		"sample_size":           len(returns),
		"interpretation":        interpretation,
	}, nil
}

func calculateDrawdown(_ context.Context, args map[string]interface{}) (interface{}, error) {
	equityCurveRaw, ok := args["equity_curve"]
	if !ok {
		return nil, fmt.Errorf("equity_curve is required")
	}

	equityCurveArray, ok := equityCurveRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("equity_curve must be an array")
	}

	if len(equityCurveArray) == 0 {
		return nil, fmt.Errorf("equity_curve array cannot be empty")
	}

	equityCurve := make([]float64, len(equityCurveArray))
	for i, e := range equityCurveArray {
		switch v := e.(type) {
		case float64:
			equityCurve[i] = v
		case int:
			equityCurve[i] = float64(v)
		case int64:
			equityCurve[i] = float64(v)
		default:
			return nil, fmt.Errorf("equity_curve[%d] must be a number", i)
		}
	}

	for i, val := range equityCurve {
		if val < 0 {
			return nil, fmt.Errorf("equity_curve[%d] must be non-negative (got %f)", i, val)
		}
	}

	runningMax := make([]float64, len(equityCurve))
	runningMax[0] = equityCurve[0]
	for i := 1; i < len(equityCurve); i++ {
		runningMax[i] = math.Max(runningMax[i-1], equityCurve[i])
	}

	drawdowns := make([]float64, len(equityCurve))
	for i := 0; i < len(equityCurve); i++ {
		if runningMax[i] > 0 {
			drawdowns[i] = (runningMax[i] - equityCurve[i]) / runningMax[i]
		}
	}

	var maxDrawdown float64
	var maxDrawdownIdx int
	var peakIdx int

	for i := 0; i < len(drawdowns); i++ {
		if drawdowns[i] > maxDrawdown {
			maxDrawdown = drawdowns[i]
			maxDrawdownIdx = i
			for j := i - 1; j >= 0; j-- {
				if equityCurve[j] == runningMax[i] {
					peakIdx = j
					break
				}
			}
		}
	}

	drawdownDuration := maxDrawdownIdx - peakIdx

	recoveryIdx := -1
	var recovered bool
	if maxDrawdownIdx < len(equityCurve)-1 {
		peakValue := runningMax[maxDrawdownIdx]
		for i := maxDrawdownIdx + 1; i < len(equityCurve); i++ {
			if equityCurve[i] >= peakValue {
				recoveryIdx = i
				recovered = true
				break
			}
		}
	}

	var recoveryDuration int
	if recovered {
		recoveryDuration = recoveryIdx - maxDrawdownIdx
	}

	currentEquity := equityCurve[len(equityCurve)-1]
	currentPeak := runningMax[len(runningMax)-1]
	var currentDrawdown float64
	if currentPeak > 0 {
		currentDrawdown = (currentPeak - currentEquity) / currentPeak
	}

	var currentDrawdownStart int
	for i := len(equityCurve) - 1; i >= 0; i-- {
		if equityCurve[i] == currentPeak {
			currentDrawdownStart = i
			break
		}
	}
	currentDrawdownDuration := len(equityCurve) - 1 - currentDrawdownStart

	underwaterPeriods := 0
	for _, dd := range drawdowns {
		if dd > 0 {
			underwaterPeriods++
		}
	}
	underwaterPercentage := float64(underwaterPeriods) / float64(len(drawdowns))

	var sumDrawdowns float64
	var nonZeroDrawdowns int
	for _, dd := range drawdowns {
		if dd > 0 {
			sumDrawdowns += dd
			nonZeroDrawdowns++
		}
	}
	var avgDrawdown float64
	if nonZeroDrawdowns > 0 {
		avgDrawdown = sumDrawdowns / float64(nonZeroDrawdowns)
	}

	var severity string
	switch {
	case maxDrawdown < 0.05:
		severity = "Minimal - very low risk"
	case maxDrawdown < 0.10:
		severity = "Low - acceptable risk for conservative strategies"
	case maxDrawdown < 0.20:
		severity = "Moderate - typical for balanced strategies"
	case maxDrawdown < 0.30:
		severity = "High - aggressive strategy with substantial risk"
	default:
		severity = "Severe - very high risk, significant capital impairment"
	}

	return map[string]interface{}{
		"max_drawdown":              maxDrawdown,
		"max_drawdown_percent":      maxDrawdown * 100,
		"max_drawdown_peak_idx":     peakIdx,
		"max_drawdown_trough_idx":   maxDrawdownIdx,
		"max_drawdown_duration":     drawdownDuration,
		"recovered":                 recovered,
		"recovery_idx":              recoveryIdx,
		"recovery_duration":         recoveryDuration,
		"current_drawdown":          currentDrawdown,
		"current_drawdown_percent":  currentDrawdown * 100,
		"current_drawdown_start":    currentDrawdownStart,
		"current_drawdown_duration": currentDrawdownDuration,
		"in_drawdown":               currentDrawdown > 0.001,
		"underwater_periods":        underwaterPeriods,
		"underwater_percentage":     underwaterPercentage,
		"avg_drawdown":              avgDrawdown,
		"avg_drawdown_percent":      avgDrawdown * 100,
		"total_periods":             len(equityCurve),
		"severity":                  severity,
	}, nil
}
