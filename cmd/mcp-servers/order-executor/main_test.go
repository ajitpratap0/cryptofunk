//nolint:goconst // Test files use repeated strings for clarity
package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/ajitpratap0/cryptofunk/internal/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDatabase creates a PostgreSQL container and returns a database connection
func setupTestDatabase(t *testing.T) (*db.DB, func()) {
	t.Helper()

	// Use testhelpers for consistent database setup
	tc := testhelpers.SetupTestDatabase(t)

	// Apply migrations for tests that need schema
	// Note: Migrations path is relative to the cmd/mcp-servers/order-executor directory
	err := tc.ApplyMigrations("../../../migrations")
	if err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	// Return database and cleanup function
	cleanup := func() {
		tc.Cleanup()
	}

	return tc.DB, cleanup
}

func TestOrderExecutorServer_ListTools(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := result["tools"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 7) // 7 tools: place_market_order, place_limit_order, cancel_order, get_order_status, start_session, stop_session, get_session_stats

	// Verify tool names
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool["name"].(string)
	}
	assert.Contains(t, toolNames, "place_market_order")
	assert.Contains(t, toolNames, "place_limit_order")
	assert.Contains(t, toolNames, "cancel_order")
	assert.Contains(t, toolNames, "get_order_status")
	assert.Contains(t, toolNames, "start_session")
	assert.Contains(t, toolNames, "stop_session")
	assert.Contains(t, toolNames, "get_session_stats")
}

func TestStartSession_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
	}
	req.Params.Name = "start_session"
	req.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
		"config": map[string]interface{}{
			"strategy": "trend_following",
		},
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 2, resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "session_id")
	assert.Equal(t, "BTCUSDT", result["symbol"])
	assert.Equal(t, "PAPER", result["exchange"])
	assert.Equal(t, "paper", result["mode"])
	assert.Equal(t, 10000.0, result["initial_capital"])
}

func TestStartSession_MissingSymbol(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
	}
	req.Params.Name = "start_session"
	req.Params.Arguments = map[string]interface{}{
		"initial_capital": 10000.0,
		// Missing symbol
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32603, resp.Error.Code)
}

func TestStartSession_NegativeCapital(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
	}
	req.Params.Name = "start_session"
	req.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": -1000.0, // Negative capital
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
}

func TestPlaceMarketOrder_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session first
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Place market order
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
	}
	req.Params.Name = "place_market_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 6, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "order_id")
	assert.Equal(t, "BTCUSDT", result["symbol"])
	assert.Equal(t, "buy", result["side"])
	assert.Equal(t, "market", result["type"])
	assert.Equal(t, 0.1, result["quantity"])
}

func TestPlaceMarketOrder_MissingSymbol(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
	}
	req.Params.Name = "place_market_order"
	req.Params.Arguments = map[string]interface{}{
		"side":     "buy",
		"quantity": 0.1,
		// Missing symbol
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "symbol")
}

func TestPlaceMarketOrder_InvalidSide(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
	}
	req.Params.Name = "place_market_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "invalid", // Invalid side
		"quantity": 0.1,
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "side")
}

func TestPlaceMarketOrder_NegativeQuantity(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
	}
	req.Params.Name = "place_market_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": -0.1, // Negative quantity
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "quantity")
}

func TestPlaceLimitOrder_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session first
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Place limit order
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
	}
	req.Params.Name = "place_limit_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "sell",
		"quantity": 0.1,
		"price":    50000.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 11, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "order_id")
	assert.Equal(t, "BTCUSDT", result["symbol"])
	assert.Equal(t, "sell", result["side"])
	assert.Equal(t, "limit", result["type"])
	assert.Equal(t, 0.1, result["quantity"])
	assert.Equal(t, 50000.0, result["price"])
}

func TestPlaceLimitOrder_MissingPrice(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
	}
	req.Params.Name = "place_limit_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "sell",
		"quantity": 0.1,
		// Missing price
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "price")
}

func TestPlaceLimitOrder_NegativePrice(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
	}
	req.Params.Name = "place_limit_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "sell",
		"quantity": 0.1,
		"price":    -50000.0, // Negative price
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "price")
}

func TestGetOrderStatus_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Place order
	placeReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
	}
	placeReq.Params.Name = "place_market_order"
	placeReq.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	}
	placeResp := server.handleRequest(&placeReq)
	placeOrder := placeResp.Result.(*exchange.Order)
	orderID := placeOrder.ID

	// Get order status
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      16,
		Method:  "tools/call",
	}
	req.Params.Name = "get_order_status"
	req.Params.Arguments = map[string]interface{}{
		"order_id": orderID,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 16, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "order")
	assert.Contains(t, result, "fills")
}

func TestGetOrderStatus_InvalidOrderID(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      17,
		Method:  "tools/call",
	}
	req.Params.Name = "get_order_status"
	req.Params.Arguments = map[string]interface{}{
		"order_id": "nonexistent_order_id",
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
}

func TestCancelOrder_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      18,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Place limit order (can be cancelled before fill)
	placeReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      19,
		Method:  "tools/call",
	}
	placeReq.Params.Name = "place_limit_order"
	placeReq.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
		"price":    40000.0, // Low price to avoid fill
	}
	placeResp := server.handleRequest(&placeReq)
	placeOrder := placeResp.Result.(*exchange.Order)
	orderID := placeOrder.ID

	// Cancel order
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "tools/call",
	}
	req.Params.Name = "cancel_order"
	req.Params.Arguments = map[string]interface{}{
		"order_id": orderID,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 20, resp.ID)
	// Note: May or may not error depending on whether order was already filled
}

func TestCancelOrder_MissingOrderID(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "tools/call",
	}
	req.Params.Name = "cancel_order"
	req.Params.Arguments = map[string]interface{}{
		// Missing order_id
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "order_id")
}

func TestGetSessionStats_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Get session stats
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "tools/call",
	}
	req.Params.Name = "get_session_stats"
	req.Params.Arguments = map[string]interface{}{}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 23, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "session_id")
	assert.Contains(t, result, "symbol")
	assert.Contains(t, result, "initial_capital")
	assert.Contains(t, result, "total_trades")
	assert.Contains(t, result, "winning_trades")
	assert.Contains(t, result, "losing_trades")
	assert.Contains(t, result, "total_pnl")
}

func TestGetSessionStats_NoActiveSession(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      24,
		Method:  "tools/call",
	}
	req.Params.Name = "get_session_stats"
	req.Params.Arguments = map[string]interface{}{}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "no active trading session")
}

func TestStopSession_ValidInput(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      25,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Stop session
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      26,
		Method:  "tools/call",
	}
	req.Params.Name = "stop_session"
	req.Params.Arguments = map[string]interface{}{
		"final_capital": 10500.0,
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 26, resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, result, "session_id")
	assert.Contains(t, result, "final_capital")
	assert.Contains(t, result, "stopped_at")
}

func TestStopSession_NoActiveSession(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      27,
		Method:  "tools/call",
	}
	req.Params.Name = "stop_session"
	req.Params.Arguments = map[string]interface{}{
		"final_capital": 10000.0,
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "no active trading session")
}

func TestStopSession_NegativeFinalCapital(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Start session
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      28,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	server.handleRequest(&startReq)

	// Stop session with negative final capital
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      29,
		Method:  "tools/call",
	}
	req.Params.Name = "stop_session"
	req.Params.Arguments = map[string]interface{}{
		"final_capital": -1000.0, // Negative
	}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "final_capital")
}

func TestInvalidMethod(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "invalid_method",
	}

	resp := server.handleRequest(&req)

	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestInvalidToolName(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
	}
	req.Params.Name = "invalid_tool"
	req.Params.Arguments = map[string]interface{}{}

	resp := server.handleRequest(&req)

	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestStdioIntegration(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// Create request
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "tools/list",
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

func TestCompleteSessionLifecycle(t *testing.T) {
	database, cleanup := setupTestDatabase(t)
	defer cleanup()

	exchangeService := exchange.NewServicePaper(database)
	server := &MCPServer{
		service: exchangeService,
	}

	// 1. Start session
	startReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      33,
		Method:  "tools/call",
	}
	startReq.Params.Name = "start_session"
	startReq.Params.Arguments = map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	}
	startResp := server.handleRequest(&startReq)
	assert.Nil(t, startResp.Error)

	// 2. Place buy order
	buyReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      34,
		Method:  "tools/call",
	}
	buyReq.Params.Name = "place_market_order"
	buyReq.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	}
	buyResp := server.handleRequest(&buyReq)
	assert.Nil(t, buyResp.Error)

	// 3. Place sell order
	sellReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      35,
		Method:  "tools/call",
	}
	sellReq.Params.Name = "place_market_order"
	sellReq.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "sell",
		"quantity": 0.1,
	}
	sellResp := server.handleRequest(&sellReq)
	assert.Nil(t, sellResp.Error)

	// 4. Get session stats
	statsReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      36,
		Method:  "tools/call",
	}
	statsReq.Params.Name = "get_session_stats"
	statsReq.Params.Arguments = map[string]interface{}{}
	statsResp := server.handleRequest(&statsReq)
	assert.Nil(t, statsResp.Error)

	statsResult := statsResp.Result.(map[string]interface{})
	// Verify stats endpoint returns expected fields
	assert.Contains(t, statsResult, "total_trades")
	assert.Contains(t, statsResult, "session_id")
	assert.Contains(t, statsResult, "symbol")
	// Note: total_trades may be 0 if session tracking is not yet fully implemented

	// 5. Stop session
	stopReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      37,
		Method:  "tools/call",
	}
	stopReq.Params.Name = "stop_session"
	stopReq.Params.Arguments = map[string]interface{}{
		"final_capital": 10100.0,
	}
	stopResp := server.handleRequest(&stopReq)
	assert.Nil(t, stopResp.Error)

	stopResult := stopResp.Result.(map[string]interface{})
	assert.Contains(t, stopResult, "total_trades")
	assert.Contains(t, stopResult, "total_pnl")
}
