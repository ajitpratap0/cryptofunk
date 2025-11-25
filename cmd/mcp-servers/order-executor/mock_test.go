package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ajitpratap0/cryptofunk/internal/exchange"
)

// TestCallTool_PlaceMarketOrder tests calling place_market_order tool
func TestCallTool_PlaceMarketOrder(t *testing.T) {
	// Use paper trading service for testing (nil database is OK for these tests)
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	result, err := server.callTool("place_market_order", map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// The result is an exchange.Order struct
	order, ok := result.(*exchange.Order)
	assert.True(t, ok, "Result should be an *exchange.Order")
	assert.NotEmpty(t, order.ID, "Order should have an ID")
	assert.Equal(t, "BTCUSDT", order.Symbol)
	assert.Equal(t, exchange.OrderSideBuy, order.Side)
	assert.Equal(t, exchange.OrderTypeMarket, order.Type)
	assert.Equal(t, 0.1, order.Quantity)
	// Market orders in paper trading fill immediately
	assert.Equal(t, exchange.OrderStatusFilled, order.Status)
}

// TestCallTool_PlaceLimitOrder tests calling place_limit_order tool
func TestCallTool_PlaceLimitOrder(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	result, err := server.callTool("place_limit_order", map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
		"price":    50000.0,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// The result is an exchange.Order struct
	order, ok := result.(*exchange.Order)
	assert.True(t, ok, "Result should be an *exchange.Order")
	assert.NotEmpty(t, order.ID, "Order should have an ID")
	assert.Equal(t, "BTCUSDT", order.Symbol)
	assert.Equal(t, exchange.OrderSideBuy, order.Side)
	assert.Equal(t, exchange.OrderTypeLimit, order.Type)
	assert.Equal(t, 0.1, order.Quantity)
	assert.Equal(t, 50000.0, order.Price)
	// Limit orders in paper trading may or may not fill immediately
	assert.Contains(t, []exchange.OrderStatus{
		exchange.OrderStatusPending,
		exchange.OrderStatusOpen,
		exchange.OrderStatusFilled,
	}, order.Status)
}

// TestCallTool_CancelOrder tests calling cancel_order tool
func TestCallTool_CancelOrder(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	// First place a limit order (these don't fill immediately, so we can cancel)
	placeResult, err := server.callTool("place_limit_order", map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
		"price":    40000.0, // Below market to ensure it doesn't fill
	})
	assert.NoError(t, err)
	placedOrder := placeResult.(*exchange.Order)

	// Now cancel it
	result, err := server.callTool("cancel_order", map[string]interface{}{
		"order_id": placedOrder.ID,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// The result is an exchange.Order struct
	order, ok := result.(*exchange.Order)
	assert.True(t, ok, "Result should be an *exchange.Order")
	assert.Equal(t, placedOrder.ID, order.ID)
	assert.Equal(t, exchange.OrderStatusCancelled, order.Status)
}

// TestCallTool_GetOrderStatus tests calling get_order_status tool
func TestCallTool_GetOrderStatus(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	// First place an order
	placeResult, err := server.callTool("place_market_order", map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	})
	assert.NoError(t, err)
	placedOrder := placeResult.(*exchange.Order)

	// Now get its status
	result, err := server.callTool("get_order_status", map[string]interface{}{
		"order_id": placedOrder.ID,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// The result is a map with "order" and "fills" keys
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")
	assert.NotNil(t, resultMap["order"], "Result should contain 'order' field")

	// Verify the order data
	order, ok := resultMap["order"].(*exchange.Order)
	assert.True(t, ok, "Order should be an *exchange.Order")
	assert.Equal(t, placedOrder.ID, order.ID)
	assert.Equal(t, "BTCUSDT", order.Symbol)
}

// TestCallTool_StartSession tests calling start_session tool
// Note: This test is skipped as it requires database integration testing
func TestCallTool_StartSession(t *testing.T) {
	t.Skip("StartSession requires database - use integration tests instead")

	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	result, err := server.callTool("start_session", map[string]interface{}{
		"symbol":          "BTCUSDT",
		"initial_capital": 10000.0,
	})

	// This would panic with nil database
	_ = result
	_ = err
}

// TestCallTool_StopSession tests calling stop_session tool
// Note: This test expects failure because nil database is used
func TestCallTool_StopSession(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	result, err := server.callTool("stop_session", map[string]interface{}{
		"final_capital": 10500.0,
	})

	// StopSession requires an active session, which requires database
	assert.Error(t, err, "StopSession should fail with no active session")
	assert.Contains(t, err.Error(), "no active trading session")
	assert.Nil(t, result)
}

// TestCallTool_GetSessionStats tests calling get_session_stats tool
// Note: This test expects failure because nil database is used
func TestCallTool_GetSessionStats(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	result, err := server.callTool("get_session_stats", map[string]interface{}{})

	// GetSessionStats requires an active session, which requires database
	assert.Error(t, err, "GetSessionStats should fail with no active session")
	assert.Contains(t, err.Error(), "no active trading session")
	assert.Nil(t, result)
}

// TestCallTool_UnknownTool tests calling an unknown tool
func TestCallTool_UnknownTool(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	result, err := server.callTool("unknown_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

// TestHandleRequest_ToolsCall_Success tests successful tools/call handling
func TestHandleRequest_ToolsCall_Success(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
	}
	req.Params.Name = "place_market_order"
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	}

	resp := server.handleRequest(req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	// Result should be an Order struct
	order, ok := resp.Result.(*exchange.Order)
	assert.True(t, ok, "Result should be an *exchange.Order")
	assert.NotEmpty(t, order.ID, "Order should have an ID")
	assert.Equal(t, "BTCUSDT", order.Symbol)
}

// TestHandleRequest_ToolsCall_UnknownTool tests tools/call with unknown tool
func TestHandleRequest_ToolsCall_UnknownTool(t *testing.T) {
	service := exchange.NewServicePaper(nil)
	server := &MCPServer{
		service: service,
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
	}
	req.Params.Name = "unknown_tool"
	req.Params.Arguments = map[string]interface{}{}

	resp := server.handleRequest(req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Nil(t, resp.Result)
	assert.Equal(t, -32603, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

// TestPaperTradingService tests the paper trading service directly
func TestPaperTradingService(t *testing.T) {
	service := exchange.NewServicePaper(nil)

	t.Run("PlaceMarketOrder", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 0.1,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)

		order, ok := result.(*exchange.Order)
		assert.True(t, ok, "Result should be an *exchange.Order")
		assert.Equal(t, "BTCUSDT", order.Symbol)
		assert.Equal(t, exchange.OrderSideBuy, order.Side)
	})

	t.Run("PlaceLimitOrder", func(t *testing.T) {
		result, err := service.PlaceLimitOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "sell",
			"quantity": 0.1,
			"price":    50000.0,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)

		order, ok := result.(*exchange.Order)
		assert.True(t, ok, "Result should be an *exchange.Order")
		assert.Equal(t, "BTCUSDT", order.Symbol)
		assert.Equal(t, exchange.OrderSideSell, order.Side)
		assert.Equal(t, 50000.0, order.Price)
	})

	t.Run("InvalidParameters", func(t *testing.T) {
		// Missing symbol
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"side":     "buy",
			"quantity": 0.1,
		})
		assert.Error(t, err, "Should fail with missing symbol")
		assert.Nil(t, result)

		// Invalid side
		result, err = service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "invalid",
			"quantity": 0.1,
		})
		assert.Error(t, err, "Should fail with invalid side")
		assert.Nil(t, result)

		// Invalid quantity
		result, err = service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": -0.1,
		})
		assert.Error(t, err, "Should fail with negative quantity")
		assert.Nil(t, result)
	})
}
