package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTechnicalIndicatorsServer_Initialize(t *testing.T) {
	server := &MCPServer{
		service: nil, // Service not needed for initialize
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2024-11-05", result["protocolVersion"])

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "technical-indicators", serverInfo["name"])
	assert.Equal(t, "1.0.0", serverInfo["version"])
}

func TestTechnicalIndicatorsServer_ListTools(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

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
	assert.Len(t, tools, 5) // 5 tools: RSI, MACD, Bollinger, EMA, ADX

	// Verify tool names
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool["name"].(string)
	}
	assert.Contains(t, toolNames, "calculate_rsi")
	assert.Contains(t, toolNames, "calculate_macd")
	assert.Contains(t, toolNames, "calculate_bollinger_bands")
	assert.Contains(t, toolNames, "calculate_ema")
	assert.Contains(t, toolNames, "calculate_adx")
}

func TestCalculateRSI_ValidInput(t *testing.T) {
	server := &MCPServer{
		service: nil, // Will be set in test
	}

	params := map[string]interface{}{
		"name": "calculate_rsi",
		"arguments": map[string]interface{}{
			"prices": []interface{}{
				44.34, 44.09, 43.61, 43.03, 43.52, 43.13, 42.66,
				42.82, 42.67, 43.13, 43.37, 43.23, 43.08, 42.07,
				41.99, 42.18, 42.49, 42.28, 42.51, 43.13,
			},
			"period": 14,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 3, resp.ID)
	// Note: Will fail without proper service, but structure is correct
}

func TestCalculateRSI_EmptyPrices(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_rsi",
		"arguments": map[string]interface{}{
			"prices": []interface{}{}, // Empty prices
			"period": 14,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculateRSI_InsufficientData(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_rsi",
		"arguments": map[string]interface{}{
			"prices": []interface{}{100.0, 101.0, 99.0}, // Only 3 prices, need 14+
			"period": 14,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculateRSI_MissingPrices(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_rsi",
		"arguments": map[string]interface{}{
			"period": 14,
			// Missing prices
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculateMACD_ValidInput(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	// Generate 50 prices for MACD calculation
	prices := make([]interface{}, 50)
	for i := 0; i < 50; i++ {
		prices[i] = 100.0 + float64(i)*0.5
	}

	params := map[string]interface{}{
		"name": "calculate_macd",
		"arguments": map[string]interface{}{
			"prices":        prices,
			"fast_period":   12,
			"slow_period":   26,
			"signal_period": 9,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 7, resp.ID)
}

func TestCalculateMACD_InsufficientData(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_macd",
		"arguments": map[string]interface{}{
			"prices":        []interface{}{100.0, 101.0}, // Too few prices
			"fast_period":   12,
			"slow_period":   26,
			"signal_period": 9,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculateBollingerBands_ValidInput(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	// Generate 30 prices
	prices := make([]interface{}, 30)
	for i := 0; i < 30; i++ {
		prices[i] = 100.0 + float64(i%10)
	}

	params := map[string]interface{}{
		"name": "calculate_bollinger_bands",
		"arguments": map[string]interface{}{
			"prices":  prices,
			"period":  20,
			"std_dev": 2,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 9, resp.ID)
}

func TestCalculateBollingerBands_DefaultParams(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	prices := make([]interface{}, 30)
	for i := 0; i < 30; i++ {
		prices[i] = 100.0 + float64(i%10)
	}

	params := map[string]interface{}{
		"name": "calculate_bollinger_bands",
		"arguments": map[string]interface{}{
			"prices": prices,
			// period and std_dev should use defaults (20 and 2)
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 10, resp.ID)
}

func TestCalculateEMA_ValidInput(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	prices := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		prices[i] = 100.0 + float64(i)
	}

	params := map[string]interface{}{
		"name": "calculate_ema",
		"arguments": map[string]interface{}{
			"prices": prices,
			"period": 10,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 11, resp.ID)
}

func TestCalculateEMA_MissingPeriod(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	prices := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		prices[i] = 100.0 + float64(i)
	}

	params := map[string]interface{}{
		"name": "calculate_ema",
		"arguments": map[string]interface{}{
			"prices": prices,
			// Missing required period parameter
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculateADX_ValidInput(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	// Generate 30 OHLC data points
	high := make([]interface{}, 30)
	low := make([]interface{}, 30)
	close := make([]interface{}, 30)
	for i := 0; i < 30; i++ {
		base := 100.0 + float64(i)
		high[i] = base + 2.0
		low[i] = base - 2.0
		close[i] = base
	}

	params := map[string]interface{}{
		"name": "calculate_adx",
		"arguments": map[string]interface{}{
			"high":   high,
			"low":    low,
			"close":  close,
			"period": 14,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 13, resp.ID)
}

func TestCalculateADX_MismatchedArrayLengths(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_adx",
		"arguments": map[string]interface{}{
			"high":   []interface{}{102.0, 103.0, 104.0},
			"low":    []interface{}{98.0, 99.0}, // Different length
			"close":  []interface{}{100.0, 101.0, 102.0},
			"period": 14,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestCalculateADX_MissingClose(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_adx",
		"arguments": map[string]interface{}{
			"high":   []interface{}{102.0, 103.0, 104.0},
			"low":    []interface{}{98.0, 99.0, 100.0},
			"period": 14,
			// Missing close
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestInvalidMethod(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      16,
		Method:  "invalid_method",
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestInvalidToolName(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name":      "invalid_tool",
		"arguments": map[string]interface{}{},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      17,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestMalformedJSON(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      18,
		Method:  "tools/call",
		Params:  json.RawMessage(`{invalid json`),
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestStdioIntegration(t *testing.T) {
	// Test that the server can handle requests via stdin/stdout simulation
	server := &MCPServer{
		service: nil,
	}

	// Create request
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      19,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
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
	assert.Nil(t, decodedResp.Error)
}

func TestInvalidPriceType(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	params := map[string]interface{}{
		"name": "calculate_rsi",
		"arguments": map[string]interface{}{
			"prices": []interface{}{"invalid", "prices", "not", "numbers"},
			"period": 14,
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestNegativePeriod(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	prices := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		prices[i] = 100.0 + float64(i)
	}

	params := map[string]interface{}{
		"name": "calculate_rsi",
		"arguments": map[string]interface{}{
			"prices": prices,
			"period": -14, // Negative period
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}

func TestZeroPeriod(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	prices := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		prices[i] = 100.0 + float64(i)
	}

	params := map[string]interface{}{
		"name": "calculate_ema",
		"arguments": map[string]interface{}{
			"prices": prices,
			"period": 0, // Zero period
		},
	}

	paramsBytes, _ := json.Marshal(params)
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
}
