package main

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRiskAnalyzerServer_Initialize(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	// Risk-analyzer doesn't implement "initialize" method - should return error
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestRiskAnalyzerServer_ListTools(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 2, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := result["tools"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 5) // 5 tools: position_size, var, limits, sharpe, drawdown

	// Verify tool names
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool["name"].(string)
	}
	assert.Contains(t, toolNames, "calculate_position_size")
	assert.Contains(t, toolNames, "calculate_var")
	assert.Contains(t, toolNames, "check_portfolio_limits")
	assert.Contains(t, toolNames, "calculate_sharpe")
	assert.Contains(t, toolNames, "calculate_drawdown")
}

func TestCalculatePositionSize_ValidInput(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 3, resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "position_size")
	assert.Contains(t, result, "kelly_percentage")
	assert.Contains(t, result, "recommendation")
	assert.Contains(t, result, "adjusted_kelly")
}

func TestCalculatePositionSize_MissingParams(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"capital": 10000.0,
		// Missing required parameters (win_rate, avg_win, avg_loss)
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 4, resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32603, resp.Error.Code)
}

func TestCalculatePositionSize_NegativeValues(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"account_balance": -1000.0, // Negative balance
		"win_probability": 0.6,
		"win_amount":      100.0,
		"loss_amount":     50.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculatePositionSize_InvalidProbability(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"account_balance": 10000.0,
		"win_probability": 1.5, // Invalid probability > 1
		"win_amount":      100.0,
		"loss_amount":     50.0,
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
}

func TestCalculateVaR_ValidInput(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_var"
	req.Params.Arguments = map[string]interface{}{
		"returns":           []interface{}{0.01, -0.02, 0.03, -0.01, 0.02, -0.03, 0.01},
		"confidence_level":  0.95,
		"portfolio_value":   10000.0,
		"time_horizon_days": 1.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "var")
	assert.Contains(t, result, "confidence_level")
	assert.Contains(t, result, "interpretation")
}

func TestCalculateVaR_EmptyReturns(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_var"
	req.Params.Arguments = map[string]interface{}{
		"returns":           []interface{}{}, // Empty returns
		"confidence_level":  0.95,
		"portfolio_value":   10000.0,
		"time_horizon_days": 1.0,
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
}

func TestCheckPortfolioLimits_ValidInput(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{
			map[string]interface{}{
				"symbol":   "BTCUSDT",
				"quantity": 0.5,
				"value":    25000.0,
			},
		},
		"new_trade": map[string]interface{}{
			"symbol":   "ETHUSDT",
			"quantity": 1.0,
			"price":    3000.0,
			"side":     "BUY",
		},
		"limits": map[string]interface{}{
			"max_exposure":      50000.0,
			"max_concentration": 0.3,
			"max_drawdown":      0.2,
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "approved")
	assert.Contains(t, result, "violations")
	assert.Contains(t, result, "recommendation")
}

func TestCalculateSharpe_ValidInput(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns":          []interface{}{0.01, -0.02, 0.03, -0.01, 0.02, -0.03, 0.01, 0.02},
		"risk_free_rate":   0.02,
		"periods_per_year": 252.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "sharpe_ratio")
	assert.Contains(t, result, "annualized_return")
	assert.Contains(t, result, "annualized_volatility")
}

func TestCalculateDrawdown_ValidInput(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_drawdown"
	req.Params.Arguments = map[string]interface{}{
		"equity_curve": []interface{}{10000.0, 10500.0, 10200.0, 11000.0, 10800.0, 11500.0},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "max_drawdown")
	assert.Contains(t, result, "max_drawdown_percent")
	assert.Contains(t, result, "current_drawdown_percent")
}

func TestCalculateDrawdown_EmptyEquityCurve(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_drawdown"
	req.Params.Arguments = map[string]interface{}{
		"equity_curve": []interface{}{}, // Empty
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
}

func TestInvalidMethod(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "invalid_method",
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestInvalidToolName(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
	}
	req.Params.Name = "invalid_tool"
	req.Params.Arguments = map[string]interface{}{}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestMalformedJSON(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "invalid_method",
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestStdioIntegration(t *testing.T) {
	// Test that the server can handle requests via stdin/stdout simulation
	server := &MCPServer{}

	// Create request
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      16,
		Method:  "initialize",
	}

	// Encode to JSON (simulating stdin)
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err := encoder.Encode(req)
	require.NoError(t, err)

	// Decode (simulating server reading stdin)
	var decodedReq MCPRequest
	decoder := json.NewDecoder(&buf)
	err = decoder.Decode(&decodedReq)
	require.NoError(t, err)

	// Handle request
	resp := server.handleRequest(&decodedReq)

	// Encode response (simulating stdout)
	var respBuf bytes.Buffer
	respEncoder := json.NewEncoder(&respBuf)
	err = respEncoder.Encode(resp)
	require.NoError(t, err)

	// Decode response (simulating client reading stdout)
	var decodedResp MCPResponse
	respDecoder := json.NewDecoder(&respBuf)
	err = respDecoder.Decode(&decodedResp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decodedResp.JSONRPC)
	// Risk-analyzer doesn't implement "initialize" method - should return error
	assert.NotNil(t, decodedResp.Error)
	assert.Equal(t, -32601, decodedResp.Error.Code)
	assert.Contains(t, decodedResp.Error.Message, "Method not found")
}

// Edge case tests to improve coverage

func TestCalculatePositionSize_IntegerValues(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      17,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        100,   // int instead of float64
		"avg_loss":       50,    // int instead of float64
		"capital":        10000, // int instead of float64
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestCalculatePositionSize_ZeroWinRate(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      18,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.0, // Zero win rate
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestCalculatePositionSize_HighKellyFraction(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      19,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 2.0, // Very high fraction (>1) - should be rejected
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32603, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "kelly_fraction must be between 0 and 1")
}

func TestCalculateSharpe_InsufficientReturns(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns":          []interface{}{0.01}, // Only one return - produces +Inf sharpe ratio
		"risk_free_rate":   0.02,
		"periods_per_year": 252.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	// With only 1 return, std dev is 0, resulting in +Inf sharpe ratio
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "sharpe_ratio")
}

func TestCalculateSharpe_AllSameReturns(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns":          []interface{}{0.02, 0.02, 0.02, 0.02, 0.02}, // All same (zero volatility)
		"risk_free_rate":   0.02,
		"periods_per_year": 252.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestCalculateSharpe_IntegerInputs(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns":          []interface{}{1, -2, 3, -1, 2}, // Integer returns
		"risk_free_rate":   2,                              // Integer risk-free rate
		"periods_per_year": 252,                            // Integer periods
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestCheckPortfolioLimits_ExceedsExposure(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{
			map[string]interface{}{
				"symbol":   "BTCUSDT",
				"quantity": 0.5,
				"value":    25000.0,
			},
		},
		"new_trade": map[string]interface{}{
			"symbol":   "ETHUSDT",
			"quantity": 10.0,
			"price":    3000.0,
			"side":     "BUY",
		},
		"limits": map[string]interface{}{
			"max_exposure":      30000.0, // Total would be 55000, exceeds limit
			"max_concentration": 0.3,
			"max_drawdown":      0.2,
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	// Should be rejected due to max_exposure violation
	approved, ok := result["approved"].(bool)
	require.True(t, ok)
	assert.False(t, approved)
}

func TestCheckPortfolioLimits_MissingNewTrade(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      24,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{},
		// Missing "new_trade" field
		"limits": map[string]interface{}{
			"max_exposure":      50000.0,
			"max_concentration": 0.3,
			"max_drawdown":      0.2,
		},
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "new_trade is required")
}

func TestCalculateVaR_IntegerInputs(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      25,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_var"
	req.Params.Arguments = map[string]interface{}{
		"returns":           []interface{}{1, -2, 3, -1, 2}, // Integer returns
		"confidence_level":  0.95,                           // Correct decimal format (not 95)
		"portfolio_value":   10000,                          // Integer portfolio value
		"time_horizon_days": 1,                              // Integer time horizon
	}

	resp := server.handleRequest(&req)

	// Should handle integer inputs via extractFloat
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

// Additional tests to improve coverage to >80%

func TestCheckPortfolioLimits_SellSide(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      27,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{
			map[string]interface{}{
				"symbol": "BTC",
				"value":  5000.0,
			},
		},
		"new_trade": map[string]interface{}{
			"symbol":   "BTC",
			"side":     "SELL",
			"quantity": 0.1,
			"price":    50000.0,
		},
		"limits": map[string]interface{}{
			"max_exposure":      10000.0,
			"max_concentration": 0.8,
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, result["approved"].(bool))
}

func TestCheckPortfolioLimits_IntegerLimits(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      28,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{
			map[string]interface{}{
				"symbol": "ETH",
				"value":  2000,
			},
		},
		"new_trade": map[string]interface{}{
			"symbol":   "ETH",
			"side":     "BUY",
			"quantity": 1,
			"price":    2000,
		},
		"limits": map[string]interface{}{
			"max_exposure":      10000, // Integer
			"max_concentration": 0.5,
			"max_drawdown":      0.2, // Integer drawdown
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestCheckPortfolioLimits_ConcentrationViolation(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      29,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{
			map[string]interface{}{
				"symbol": "BTC",
				"value":  5000.0,
			},
		},
		"new_trade": map[string]interface{}{
			"symbol":   "BTC",
			"side":     "BUY",
			"quantity": 1.0,
			"price":    10000.0, // Large trade
		},
		"limits": map[string]interface{}{
			"max_exposure":      20000.0,
			"max_concentration": 0.5, // 50% max concentration
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.False(t, result["approved"].(bool))
	violations := result["violations"].([]string)
	assert.NotEmpty(t, violations)
	assert.Contains(t, violations[0], "concentration")
}

func TestCalculateSharpe_NegativeInfinity(t *testing.T) {
	server := &MCPServer{}

	// To get -Inf, we need constant returns (stdDev = 0) below risk-free rate
	// meanReturn = -0.01, riskFreePerPeriod = 0.02/252 = 0.0000793651
	// Since -0.01 < 0.0000793651, sharpe = -Inf
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns":          []interface{}{-0.01, -0.01, -0.01}, // Constant negative returns
		"risk_free_rate":   0.02,                               // Positive risk-free rate
		"periods_per_year": 252.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	sharpe := result["sharpe_ratio"].(float64)
	assert.True(t, math.IsInf(sharpe, -1), "Expected -Inf sharpe ratio")
}

func TestCalculateSharpe_ZeroSharpe(t *testing.T) {
	server := &MCPServer{}

	// Calculate risk-free rate per period: 0.02 / 252 ≈ 0.0000793651
	riskFreeRate := 0.02
	periodsPerYear := 252.0
	riskFreePerPeriod := riskFreeRate / periodsPerYear

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns": []interface{}{
			riskFreePerPeriod,
			riskFreePerPeriod,
			riskFreePerPeriod,
		}, // Returns exactly equal to risk-free rate
		"risk_free_rate":   riskFreeRate,
		"periods_per_year": periodsPerYear,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	sharpe := result["sharpe_ratio"].(float64)
	assert.Equal(t, 0.0, sharpe, "Expected zero sharpe ratio")
}

func TestCalculateSharpe_SubOptimal(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		// For Poor interpretation (Sharpe < 0), use negative mean return
		// Mean will be negative, so (mean - rf) will be even more negative
		"returns":          []interface{}{-0.0001, 0.0002, -0.0003, 0.0001, -0.0002},
		"risk_free_rate":   0.02,
		"periods_per_year": 252.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	interpretation := result["interpretation"].(string)
	sharpe := result["sharpe_ratio"].(float64)

	// Should be Poor (Sharpe < 0) since mean return < risk-free rate
	assert.True(t, sharpe < 0, "Expected negative Sharpe, got %f", sharpe)
	assert.Contains(t, interpretation, "Poor")
}

func TestCalculateSharpe_VeryGood(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      33,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_sharpe"
	req.Params.Arguments = map[string]interface{}{
		"returns":          []interface{}{0.005, 0.006, 0.004, 0.007, 0.005}, // Strong consistent returns
		"risk_free_rate":   0.01,
		"periods_per_year": 252.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	interpretation := result["interpretation"].(string)
	// Should be in Very Good or Excellent range (Sharpe >= 2)
	assert.True(t,
		strings.Contains(interpretation, "Very Good") ||
			strings.Contains(interpretation, "Excellent"),
		"Expected Very Good or Excellent interpretation")
}

// TestCalculatePositionSize_Int64Values tests extractFloat with int64 type
func TestCalculatePositionSize_Int64Values(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      34,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        int64(100),   // int64 type to test extractFloat int64 branch
		"avg_loss":       int64(50),    // int64 type
		"capital":        int64(10000), // int64 type
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, result["position_size"].(float64) > 0)
	assert.True(t, result["kelly_percentage"].(float64) > 0)
}

// TestCalculatePositionSize_StringValue tests extractFloat with invalid string type
func TestCalculatePositionSize_StringValue(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      35,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       "0.6", // string instead of number - triggers default case in extractFloat
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "must be a number")
}

// TestCheckPortfolioLimits_EmptyPositions tests with no current positions
func TestCheckPortfolioLimits_EmptyPositions(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      36,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{}, // Empty positions - edge case
		"new_trade": map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "BUY",
			"quantity": 0.1,
			"price":    50000.0,
		},
		"limits": map[string]interface{}{
			"max_exposure":      100000.0,
			"max_concentration": 0.5, // 50% max concentration
			"max_drawdown":      0.2,
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	// When starting with empty positions, a single new position will have 100% concentration
	// This violates the 50% max concentration limit, so trade should be rejected
	assert.False(t, result["approved"].(bool), "Trade should be rejected - single position = 100% concentration")
	violations := result["violations"].([]string)
	assert.Equal(t, 1, len(violations), "Should have 1 concentration violation")
	assert.Contains(t, violations[0], "concentration")

	// Verify concentration check details
	checks := result["checks"].(map[string]interface{})
	concentrationCheck := checks["concentration_check"].(map[string]interface{})
	assert.Equal(t, "BTCUSDT", concentrationCheck["largest_position"].(string))
	assert.Equal(t, 1.0, concentrationCheck["concentration"].(float64), "Single position = 100% concentration")
	assert.True(t, concentrationCheck["violated"].(bool))
}

// TestCalculatePositionSize_InvalidWinRate tests validation of win_rate parameter
func TestCalculatePositionSize_InvalidWinRate(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      37,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       1.5, // Invalid: > 1
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "win_rate must be between 0 and 1")
}

// TestCalculatePositionSize_InvalidAvgWin tests validation of avg_win parameter
func TestCalculatePositionSize_InvalidAvgWin(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      38,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        -10.0, // Invalid: <= 0
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "avg_win must be positive")
}

// TestCalculatePositionSize_InvalidAvgLoss tests validation of avg_loss parameter
func TestCalculatePositionSize_InvalidAvgLoss(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      39,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        100.0,
		"avg_loss":       0.0, // Invalid: <= 0
		"capital":        10000.0,
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "avg_loss must be positive")
}

// TestCalculatePositionSize_InvalidCapital tests validation of capital parameter
func TestCalculatePositionSize_InvalidCapital(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      40,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        -5000.0, // Invalid: <= 0
		"kelly_fraction": 0.5,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "capital must be positive")
}

// TestCalculatePositionSize_InvalidKellyFraction tests validation of kelly_fraction parameter
func TestCalculatePositionSize_InvalidKellyFraction(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      41,
		Method:  "tools/call",
	}
	req.Params.Name = "calculate_position_size"
	req.Params.Arguments = map[string]interface{}{
		"win_rate":       0.6,
		"avg_win":        100.0,
		"avg_loss":       50.0,
		"capital":        10000.0,
		"kelly_fraction": 1.5, // Invalid: > 1
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "kelly_fraction must be between 0 and 1")
}

// TestCheckPortfolioLimits_NegativeQuantity tests negative quantity validation
func TestCheckPortfolioLimits_NegativeQuantity(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      43,
		Method:  "tools/call",
	}
	req.Params.Name = "check_portfolio_limits"
	req.Params.Arguments = map[string]interface{}{
		"current_positions": []interface{}{},
		"new_trade": map[string]interface{}{
			"symbol":   "BTC",
			"side":     "BUY",
			"quantity": -1.0,
			"price":    50000.0,
		},
		"limits": map[string]interface{}{
			"max_exposure":      100000.0,
			"max_concentration": 0.3,
			"max_drawdown":      0.2,
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 43, resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "quantity must be positive")
}
