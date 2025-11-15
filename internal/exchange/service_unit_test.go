package exchange

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewService tests creating service with different configurations
func TestNewService(t *testing.T) {
	t.Run("Create paper trading service", func(t *testing.T) {
		config := ServiceConfig{
			Mode: TradingModePaper,
		}
		service, err := NewService(nil, config)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, TradingModePaper, service.mode)
	})

	t.Run("Create with default mode (paper trading)", func(t *testing.T) {
		config := ServiceConfig{
			Mode: "",
		}
		service, err := NewService(nil, config)
		require.NoError(t, err)
		assert.NotNil(t, service)
		// Empty mode defaults to paper trading but stores empty string
		assert.Equal(t, TradingMode(""), service.mode)
	})

	t.Run("NewServicePaper helper", func(t *testing.T) {
		service := NewServicePaper(nil)
		assert.NotNil(t, service)
		assert.Equal(t, TradingModePaper, service.mode)
	})
}

// TestPlaceMarketOrder_ErrorPaths tests error handling in PlaceMarketOrder
func TestPlaceMarketOrder_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing symbol", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"side":     "buy",
			"quantity": 1.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbol")
		assert.Nil(t, result)
	})

	t.Run("Empty symbol", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "",
			"side":     "buy",
			"quantity": 1.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbol")
		assert.Nil(t, result)
	})

	t.Run("Missing side", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"quantity": 1.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "side")
		assert.Nil(t, result)
	})

	t.Run("Invalid side", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "invalid",
			"quantity": 1.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "side")
		assert.Nil(t, result)
	})

	t.Run("Missing quantity", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol": "BTCUSDT",
			"side":   "buy",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quantity")
		assert.Nil(t, result)
	})

	t.Run("Zero quantity", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 0.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quantity")
		assert.Nil(t, result)
	})

	t.Run("Negative quantity", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": -1.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quantity")
		assert.Nil(t, result)
	})

	t.Run("Invalid quantity type (string)", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": "not_a_number",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quantity")
		assert.Nil(t, result)
	})

	t.Run("Quantity as integer", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 1,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("Quantity as int64", func(t *testing.T) {
		result, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": int64(1),
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// TestPlaceLimitOrder_ErrorPaths tests error handling in PlaceLimitOrder
func TestPlaceLimitOrder_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing symbol", func(t *testing.T) {
		result, err := service.PlaceLimitOrder(map[string]interface{}{
			"side":     "buy",
			"quantity": 1.0,
			"price":    50000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbol")
		assert.Nil(t, result)
	})

	t.Run("Missing price", func(t *testing.T) {
		result, err := service.PlaceLimitOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 1.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "price")
		assert.Nil(t, result)
	})

	t.Run("Zero price", func(t *testing.T) {
		result, err := service.PlaceLimitOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 1.0,
			"price":    0.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "price")
		assert.Nil(t, result)
	})

	t.Run("Negative price", func(t *testing.T) {
		result, err := service.PlaceLimitOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 1.0,
			"price":    -50000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "price")
		assert.Nil(t, result)
	})

	t.Run("Price as integer", func(t *testing.T) {
		result, err := service.PlaceLimitOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 1.0,
			"price":    50000,
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// TestCancelOrder_ErrorPaths tests error handling in CancelOrder
func TestCancelOrder_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing order_id", func(t *testing.T) {
		result, err := service.CancelOrder(map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order_id")
		assert.Nil(t, result)
	})

	t.Run("Empty order_id", func(t *testing.T) {
		result, err := service.CancelOrder(map[string]interface{}{
			"order_id": "",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order_id")
		assert.Nil(t, result)
	})

	t.Run("Non-existent order_id", func(t *testing.T) {
		result, err := service.CancelOrder(map[string]interface{}{
			"order_id": "non_existent_order",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
		assert.Nil(t, result)
	})
}

// TestGetOrderStatus_ErrorPaths tests error handling in GetOrderStatus
func TestGetOrderStatus_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing order_id", func(t *testing.T) {
		result, err := service.GetOrderStatus(map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order_id")
		assert.Nil(t, result)
	})

	t.Run("Non-existent order_id", func(t *testing.T) {
		result, err := service.GetOrderStatus(map[string]interface{}{
			"order_id": "non_existent_order",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
		assert.Nil(t, result)
	})
}

// TestGetPositionBySymbol tests GetPositionBySymbol
func TestGetPositionBySymbol(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing symbol", func(t *testing.T) {
		result, err := service.GetPositionBySymbol(map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbol")
		assert.Nil(t, result)
	})

	t.Run("Non-existent symbol", func(t *testing.T) {
		result, err := service.GetPositionBySymbol(map[string]interface{}{
			"symbol": "NONEXISTENT",
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.False(t, resultMap["exists"].(bool))
		assert.Nil(t, resultMap["position"])
	})
}

// TestGetPositions tests GetPositions
func TestGetPositions(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Get positions (empty)", func(t *testing.T) {
		result, err := service.GetPositions(map[string]interface{}{})
		assert.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 0, resultMap["count"])
	})
}

// TestUpdatePositionPnL_ErrorPaths tests error handling in UpdatePositionPnL
func TestUpdatePositionPnL_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing prices", func(t *testing.T) {
		result, err := service.UpdatePositionPnL(map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prices")
		assert.Nil(t, result)
	})

	t.Run("Invalid prices type", func(t *testing.T) {
		result, err := service.UpdatePositionPnL(map[string]interface{}{
			"prices": "not_a_map",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prices")
		assert.Nil(t, result)
	})

	t.Run("Invalid price value", func(t *testing.T) {
		result, err := service.UpdatePositionPnL(map[string]interface{}{
			"prices": map[string]interface{}{
				"BTCUSDT": "not_a_number",
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid price")
		assert.Nil(t, result)
	})

	t.Run("Valid prices with float64", func(t *testing.T) {
		result, err := service.UpdatePositionPnL(map[string]interface{}{
			"prices": map[string]interface{}{
				"BTCUSDT": 50000.0,
				"ETHUSDT": 3000.0,
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("Valid prices with int", func(t *testing.T) {
		result, err := service.UpdatePositionPnL(map[string]interface{}{
			"prices": map[string]interface{}{
				"BTCUSDT": 50000,
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("Valid prices with int64", func(t *testing.T) {
		result, err := service.UpdatePositionPnL(map[string]interface{}{
			"prices": map[string]interface{}{
				"BTCUSDT": int64(50000),
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// TestClosePositionBySymbol_ErrorPaths tests error handling in ClosePositionBySymbol
func TestClosePositionBySymbol_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing symbol", func(t *testing.T) {
		result, err := service.ClosePositionBySymbol(map[string]interface{}{
			"exit_price": 50000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbol")
		assert.Nil(t, result)
	})

	t.Run("Missing exit_price", func(t *testing.T) {
		result, err := service.ClosePositionBySymbol(map[string]interface{}{
			"symbol": "BTCUSDT",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exit_price")
		assert.Nil(t, result)
	})

	t.Run("Non-existent position", func(t *testing.T) {
		result, err := service.ClosePositionBySymbol(map[string]interface{}{
			"symbol":     "NONEXISTENT",
			"exit_price": 50000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no open position")
		assert.Nil(t, result)
	})

	t.Run("Optional exit_reason", func(t *testing.T) {
		// Create a position first
		_, err := service.PlaceMarketOrder(map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 1.0,
		})
		require.NoError(t, err)

		// Close without exit_reason (should use default)
		result, err := service.ClosePositionBySymbol(map[string]interface{}{
			"symbol":     "BTCUSDT",
			"exit_price": 51000.0,
		})

		// This will fail because DB is nil, but we're testing parameter validation
		// The error should be about database, not about exit_reason
		if err != nil {
			assert.NotContains(t, err.Error(), "exit_reason")
		}
		_ = result
	})
}

// TestStopSession_ErrorPaths tests error handling in StopSession
func TestStopSession_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("No active session", func(t *testing.T) {
		result, err := service.StopSession(map[string]interface{}{
			"final_capital": 10000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active trading session")
		assert.Nil(t, result)
	})

	t.Run("Missing final_capital (fails on session check first)", func(t *testing.T) {
		result, err := service.StopSession(map[string]interface{}{})
		assert.Error(t, err)
		// Will fail on "no active session" before checking final_capital
		assert.Contains(t, err.Error(), "no active")
		assert.Nil(t, result)
	})

	t.Run("Negative final_capital (fails on session check first)", func(t *testing.T) {
		result, err := service.StopSession(map[string]interface{}{
			"final_capital": -1000.0,
		})
		assert.Error(t, err)
		// Will fail on "no active session" before checking final_capital
		assert.Contains(t, err.Error(), "no active")
		assert.Nil(t, result)
	})
}

// TestGetSessionStats_ErrorPaths tests error handling in GetSessionStats
func TestGetSessionStats_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("No active session", func(t *testing.T) {
		result, err := service.GetSessionStats(map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active trading session")
		assert.Nil(t, result)
	})
}

// TestStartSession_ErrorPaths tests error handling in StartSession
func TestStartSession_ErrorPaths(t *testing.T) {
	service := NewServicePaper(nil)

	t.Run("Missing symbol", func(t *testing.T) {
		result, err := service.StartSession(map[string]interface{}{
			"initial_capital": 10000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbol")
		assert.Nil(t, result)
	})

	t.Run("Missing initial_capital", func(t *testing.T) {
		result, err := service.StartSession(map[string]interface{}{
			"symbol": "BTCUSDT",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initial_capital")
		assert.Nil(t, result)
	})

	t.Run("Zero initial_capital", func(t *testing.T) {
		result, err := service.StartSession(map[string]interface{}{
			"symbol":          "BTCUSDT",
			"initial_capital": 0.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initial_capital")
		assert.Nil(t, result)
	})

	t.Run("Negative initial_capital", func(t *testing.T) {
		result, err := service.StartSession(map[string]interface{}{
			"symbol":          "BTCUSDT",
			"initial_capital": -1000.0,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initial_capital")
		assert.Nil(t, result)
	})
}
