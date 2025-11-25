//nolint:goconst // MCP tool names are defined by protocol spec
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging to stderr (stdout is reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Risk Analyzer MCP Server starting...")

	// Start MCP server with stdio transport
	server := &MCPServer{}

	if err := server.Run(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

// MCPServer handles MCP protocol over stdio
type MCPServer struct{}

// Run starts the MCP server
func (s *MCPServer) Run() error {
	log.Info().Msg("MCP server ready, listening on stdio")

	// Read from stdin, process requests, write to stdout
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var request MCPRequest
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" {
				log.Info().Msg("Client disconnected")
				return nil
			}
			log.Error().Err(err).Msg("Failed to decode request")
			continue
		}

		log.Debug().
			Str("method", request.Method).
			Str("tool", request.Params.Name).
			Msg("Received request")

		// Handle request
		response := s.handleRequest(&request)

		// Send response
		if err := encoder.Encode(response); err != nil {
			log.Error().Err(err).Msg("Failed to encode response")
			return err
		}
	}
}

// MCPRequest represents an MCP tool call request
type MCPRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handleRequest routes the request to the appropriate handler
func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"listChanged": true,
				},
			},
			"serverInfo": map[string]string{
				"name":    "risk-analyzer",
				"version": "1.0.0",
			},
		}
	case "tools/list":
		resp.Result = s.listTools()
	case "tools/call":
		result, err := s.callTool(req.Params.Name, req.Params.Arguments)
		if err != nil {
			resp.Error = &MCPError{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	return resp
}

// listTools returns the list of available tools
func (s *MCPServer) listTools() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "calculate_position_size",
				"description": "Calculate optimal position size using Kelly Criterion",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"win_rate": map[string]interface{}{
							"type":        "number",
							"description": "Win rate as decimal (e.g., 0.55 for 55%)",
						},
						"avg_win": map[string]interface{}{
							"type":        "number",
							"description": "Average winning trade profit",
						},
						"avg_loss": map[string]interface{}{
							"type":        "number",
							"description": "Average losing trade loss (positive number)",
						},
						"capital": map[string]interface{}{
							"type":        "number",
							"description": "Total trading capital",
						},
						"kelly_fraction": map[string]interface{}{
							"type":        "number",
							"description": "Fraction of Kelly to use (e.g., 0.5 for half-Kelly)",
						},
					},
					"required": []string{"win_rate", "avg_win", "avg_loss", "capital", "kelly_fraction"},
				},
			},
			{
				"name":        "calculate_var",
				"description": "Calculate Value at Risk (VaR) for a return series",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"returns": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of historical returns",
						},
						"confidence_level": map[string]interface{}{
							"type":        "number",
							"description": "Confidence level (e.g., 0.95 for 95%)",
						},
					},
					"required": []string{"returns", "confidence_level"},
				},
			},
			{
				"name":        "check_portfolio_limits",
				"description": "Check if a proposed trade violates portfolio risk limits",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"current_positions": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "object"},
							"description": "Array of current positions with symbol, quantity, value",
						},
						"new_trade": map[string]interface{}{
							"type":        "object",
							"description": "Proposed trade with symbol, side, quantity, price",
						},
						"limits": map[string]interface{}{
							"type":        "object",
							"description": "Risk limits: max_exposure, max_concentration, max_drawdown",
						},
					},
					"required": []string{"current_positions", "new_trade", "limits"},
				},
			},
			{
				"name":        "calculate_sharpe",
				"description": "Calculate Sharpe ratio for a return series",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"returns": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of period returns",
						},
						"risk_free_rate": map[string]interface{}{
							"type":        "number",
							"description": "Risk-free rate (annualized)",
						},
						"periods_per_year": map[string]interface{}{
							"type":        "number",
							"description": "Number of periods per year (252 for daily, 12 for monthly)",
						},
					},
					"required": []string{"returns", "risk_free_rate", "periods_per_year"},
				},
			},
			{
				"name":        "calculate_drawdown",
				"description": "Calculate current and maximum drawdown from equity curve",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"equity_curve": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of equity values over time",
						},
					},
					"required": []string{"equity_curve"},
				},
			},
		},
	}
}

// callTool executes the specified tool
func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "calculate_position_size":
		return s.calculatePositionSize(args)
	case "calculate_var":
		return s.calculateVaR(args)
	case "check_portfolio_limits":
		return s.checkPortfolioLimits(args)
	case "calculate_sharpe":
		return s.calculateSharpe(args)
	case "calculate_drawdown":
		return s.calculateDrawdown(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
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

// calculatePositionSize implements Kelly Criterion position sizing
func (s *MCPServer) calculatePositionSize(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("calculatePositionSize called")

	// Extract parameters
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

	// Calculate Kelly Criterion
	// Formula: f* = (bp - q) / b
	// Where: b = avg_win / avg_loss (win/loss ratio)
	//        p = win_rate (probability of winning)
	//        q = 1 - win_rate (probability of losing)

	b := avgWin / avgLoss // win/loss ratio
	p := winRate
	q := 1 - winRate

	// Calculate Kelly percentage
	kellyPercentage := (b*p - q) / b

	log.Debug().
		Float64("win_rate", winRate).
		Float64("avg_win", avgWin).
		Float64("avg_loss", avgLoss).
		Float64("b_ratio", b).
		Float64("kelly_raw", kellyPercentage).
		Msg("Kelly calculation")

	// Handle edge cases
	var adjustedKelly float64
	var recommendation string

	if kellyPercentage < 0 {
		// Negative Kelly indicates a losing strategy - don't bet
		adjustedKelly = 0
		recommendation = "Negative Kelly indicates no statistical edge - position size is 0"
		log.Warn().
			Float64("kelly_percentage", kellyPercentage).
			Msg("Negative Kelly - no edge detected")
	} else if kellyPercentage > 1 {
		// Kelly > 1 is very aggressive, cap at 100%
		adjustedKelly = 1.0 * kellyFraction
		recommendation = fmt.Sprintf("Kelly > 100%% capped at 100%%, then multiplied by Kelly fraction (%.2f)", kellyFraction)
		log.Warn().
			Float64("kelly_percentage", kellyPercentage).
			Float64("adjusted", adjustedKelly).
			Msg("Kelly > 1 capped")
	} else {
		// Apply Kelly fraction for risk management
		adjustedKelly = kellyPercentage * kellyFraction
		recommendation = fmt.Sprintf("Using %.0f%% Kelly (%.2f fraction of full Kelly)", adjustedKelly*100, kellyFraction)
	}

	// Calculate position size
	positionSize := adjustedKelly * capital

	result := map[string]interface{}{
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
	}

	log.Info().
		Float64("kelly_pct", kellyPercentage*100).
		Float64("adjusted_kelly_pct", adjustedKelly*100).
		Float64("position_size", positionSize).
		Bool("positive_edge", kellyPercentage > 0).
		Msg("Position size calculated")

	return result, nil
}

func (s *MCPServer) calculateVaR(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("calculateVaR called")

	// Extract returns array
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

	// Convert to float64 slice
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

	// Extract confidence level
	confidenceLevel, err := extractFloat(args, "confidence_level")
	if err != nil {
		return nil, err
	}
	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		return nil, fmt.Errorf("confidence_level must be between 0 and 1 (got %f)", confidenceLevel)
	}

	// Sort returns in ascending order (worst losses first)
	sortedReturns := make([]float64, len(returns))
	copy(sortedReturns, returns)
	for i := 0; i < len(sortedReturns)-1; i++ {
		for j := i + 1; j < len(sortedReturns); j++ {
			if sortedReturns[i] > sortedReturns[j] {
				sortedReturns[i], sortedReturns[j] = sortedReturns[j], sortedReturns[i]
			}
		}
	}

	// Calculate VaR using historical simulation (percentile method)
	// VaR is the loss at the (1 - confidence_level) percentile
	alpha := 1 - confidenceLevel
	index := int(alpha * float64(len(sortedReturns)))
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	var_ := -sortedReturns[index] // VaR is typically expressed as a positive number (loss)

	// Calculate additional statistics
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

	// Count returns worse than VaR
	exceedances := 0
	for _, r := range returns {
		if r <= -var_ {
			exceedances++
		}
	}

	result := map[string]interface{}{
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
	}

	log.Info().
		Float64("var", var_).
		Float64("confidence", confidenceLevel).
		Int("sample_size", len(returns)).
		Int("exceedances", exceedances).
		Msg("VaR calculated")

	return result, nil
}

func (s *MCPServer) checkPortfolioLimits(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("checkPortfolioLimits called")

	// Extract current positions
	positionsRaw, ok := args["current_positions"]
	if !ok {
		return nil, fmt.Errorf("current_positions is required")
	}

	positionsArray, ok := positionsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("current_positions must be an array")
	}

	// Extract new trade
	newTradeRaw, ok := args["new_trade"]
	if !ok {
		return nil, fmt.Errorf("new_trade is required")
	}

	newTrade, ok := newTradeRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("new_trade must be an object")
	}

	// Extract limits
	limitsRaw, ok := args["limits"]
	if !ok {
		return nil, fmt.Errorf("limits is required")
	}

	limits, ok := limitsRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("limits must be an object")
	}

	// Parse new trade details
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

	// Parse limits
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

	// Calculate current portfolio metrics
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

	// Calculate new trade value
	tradeValue := tradeQuantity * tradePrice

	// Adjust for BUY/SELL
	switch tradeSide {
	case "BUY":
		positionsBySymbol[tradeSymbol] += tradeValue
	case "SELL":
		positionsBySymbol[tradeSymbol] -= tradeValue
	}

	// Calculate new total portfolio value
	newTotalValue := totalPortfolioValue
	switch tradeSide {
	case "BUY":
		newTotalValue += tradeValue
	case "SELL":
		newTotalValue -= tradeValue
	}

	// Check violations
	violations := []string{}
	checks := make(map[string]interface{})

	// Check max exposure
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

	// Check max concentration
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

	// Check max drawdown (if applicable - requires historical equity data)
	if hasMaxDrawdown {
		checks["drawdown_check"] = map[string]interface{}{
			"max_drawdown":      maxDrawdown,
			"violated":          false,
			"note":              "Drawdown check requires historical equity curve",
			"requires_tracking": true,
		}
	}

	// Determine overall approval
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

	log.Info().
		Bool("approved", approved).
		Int("violations", len(violations)).
		Float64("trade_value", tradeValue).
		Float64("portfolio_value", newTotalValue).
		Msg("Portfolio limits checked")

	return result, nil
}

func (s *MCPServer) calculateSharpe(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("calculateSharpe called")

	// Extract returns array
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

	// Convert returns to float64 slice
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

	// Extract risk-free rate
	var riskFreeRate float64
	if rfr, ok := args["risk_free_rate"]; ok {
		switch v := rfr.(type) {
		case float64:
			riskFreeRate = v
		case int:
			riskFreeRate = float64(v)
		case int64:
			riskFreeRate = float64(v)
		default:
			return nil, fmt.Errorf("risk_free_rate must be a number")
		}
	} else {
		return nil, fmt.Errorf("risk_free_rate is required")
	}

	// Extract periods per year
	var periodsPerYear float64
	if ppy, ok := args["periods_per_year"]; ok {
		switch v := ppy.(type) {
		case float64:
			periodsPerYear = v
		case int:
			periodsPerYear = float64(v)
		case int64:
			periodsPerYear = float64(v)
		default:
			return nil, fmt.Errorf("periods_per_year must be a number")
		}
	} else {
		return nil, fmt.Errorf("periods_per_year is required")
	}

	if periodsPerYear <= 0 {
		return nil, fmt.Errorf("periods_per_year must be positive (got %f)", periodsPerYear)
	}

	// Calculate mean return
	var sum float64
	for _, r := range returns {
		sum += r
	}
	meanReturn := sum / float64(len(returns))

	// Calculate standard deviation
	var varianceSum float64
	for _, r := range returns {
		diff := r - meanReturn
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(returns))
	stdDev := math.Sqrt(variance)

	// Convert annualized risk-free rate to per-period rate
	riskFreeRatePerPeriod := riskFreeRate / periodsPerYear

	// Calculate Sharpe ratio (per period)
	var sharpeRatio float64
	if stdDev == 0 {
		// If there's no volatility, handle edge case
		if meanReturn > riskFreeRatePerPeriod {
			sharpeRatio = math.Inf(1) // Positive infinity
		} else if meanReturn < riskFreeRatePerPeriod {
			sharpeRatio = math.Inf(-1) // Negative infinity
		} else {
			sharpeRatio = 0 // Exactly at risk-free rate
		}
	} else {
		sharpeRatio = (meanReturn - riskFreeRatePerPeriod) / stdDev
	}

	// Annualize Sharpe ratio
	annualizedSharpe := sharpeRatio * math.Sqrt(periodsPerYear)

	// Annualize mean return and volatility for reporting
	annualizedReturn := meanReturn * periodsPerYear
	annualizedVolatility := stdDev * math.Sqrt(periodsPerYear)

	// Determine interpretation
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

	result := map[string]interface{}{
		"sharpe_ratio":          annualizedSharpe,
		"sharpe_ratio_period":   sharpeRatio,
		"mean_return":           meanReturn,
		"annualized_return":     annualizedReturn,
		"std_dev":               stdDev,
		"annualized_volatility": annualizedVolatility,
		"risk_free_rate":        riskFreeRate,
		"periods_per_year":      periodsPerYear,
		"sample_size":           len(returns),
		"interpretation":        interpretation,
	}

	log.Info().
		Float64("sharpe_ratio", annualizedSharpe).
		Float64("mean_return", meanReturn).
		Float64("std_dev", stdDev).
		Int("sample_size", len(returns)).
		Msg("Sharpe ratio calculated")

	return result, nil
}

func (s *MCPServer) calculateDrawdown(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("calculateDrawdown called")

	// Extract equity curve array
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

	// Convert equity curve to float64 slice
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

	// Validate equity curve values are positive
	for i, val := range equityCurve {
		if val < 0 {
			return nil, fmt.Errorf("equity_curve[%d] must be non-negative (got %f)", i, val)
		}
	}

	// Calculate running maximum (peak) at each point
	runningMax := make([]float64, len(equityCurve))
	runningMax[0] = equityCurve[0]
	for i := 1; i < len(equityCurve); i++ {
		runningMax[i] = math.Max(runningMax[i-1], equityCurve[i])
	}

	// Calculate drawdown at each point (as percentage from peak)
	drawdowns := make([]float64, len(equityCurve))
	for i := 0; i < len(equityCurve); i++ {
		if runningMax[i] > 0 {
			drawdowns[i] = (runningMax[i] - equityCurve[i]) / runningMax[i]
		} else {
			drawdowns[i] = 0
		}
	}

	// Find maximum drawdown and its location
	var maxDrawdown float64
	var maxDrawdownIdx int
	var peakIdx int

	for i := 0; i < len(drawdowns); i++ {
		if drawdowns[i] > maxDrawdown {
			maxDrawdown = drawdowns[i]
			maxDrawdownIdx = i

			// Find the peak that preceded this drawdown
			for j := i - 1; j >= 0; j-- {
				if equityCurve[j] == runningMax[i] {
					peakIdx = j
					break
				}
			}
		}
	}

	// Calculate drawdown duration (periods from peak to trough)
	drawdownDuration := maxDrawdownIdx - peakIdx

	// Find recovery point (when equity returns to peak level)
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

	// Calculate recovery duration
	var recoveryDuration int
	if recovered {
		recoveryDuration = recoveryIdx - maxDrawdownIdx
	}

	// Calculate current drawdown (from most recent peak)
	currentEquity := equityCurve[len(equityCurve)-1]
	currentPeak := runningMax[len(runningMax)-1]
	var currentDrawdown float64
	if currentPeak > 0 {
		currentDrawdown = (currentPeak - currentEquity) / currentPeak
	}

	// Find current drawdown start (most recent peak)
	var currentDrawdownStart int
	for i := len(equityCurve) - 1; i >= 0; i-- {
		if equityCurve[i] == currentPeak {
			currentDrawdownStart = i
			break
		}
	}
	currentDrawdownDuration := len(equityCurve) - 1 - currentDrawdownStart

	// Calculate underwater periods (periods in drawdown)
	underwaterPeriods := 0
	for _, dd := range drawdowns {
		if dd > 0 {
			underwaterPeriods++
		}
	}
	underwaterPercentage := float64(underwaterPeriods) / float64(len(drawdowns))

	// Calculate average drawdown
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

	// Determine severity interpretation
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

	result := map[string]interface{}{
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
		"in_drawdown":               currentDrawdown > 0.001, // Small threshold for floating point
		"underwater_periods":        underwaterPeriods,
		"underwater_percentage":     underwaterPercentage,
		"avg_drawdown":              avgDrawdown,
		"avg_drawdown_percent":      avgDrawdown * 100,
		"total_periods":             len(equityCurve),
		"severity":                  severity,
	}

	log.Info().
		Float64("max_drawdown", maxDrawdown).
		Float64("current_drawdown", currentDrawdown).
		Int("drawdown_duration", drawdownDuration).
		Bool("recovered", recovered).
		Msg("Drawdown calculated")

	return result, nil
}
