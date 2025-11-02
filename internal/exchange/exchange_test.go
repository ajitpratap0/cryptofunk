package exchange

import (
	"context"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockExchangeOrderLifecycle tests the complete order lifecycle
func TestMockExchangeOrderLifecycle(t *testing.T) {
	// Create mock exchange without database
	exchange := NewMockExchange(nil)

	// Set market price
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	// Set session
	sessionID := uuid.New()
	exchange.SetSession(&sessionID)

	t.Run("Place market buy order", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 0.1,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.OrderID)
		assert.Equal(t, OrderStatusFilled, resp.Status)
	})

	t.Run("Place limit sell order", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideSell,
			Type:     OrderTypeLimit,
			Quantity: 0.05,
			Price:    51000.0,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.OrderID)
		assert.Equal(t, OrderStatusOpen, resp.Status)

		// Get order
		order, err := exchange.GetOrder(resp.OrderID)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusOpen, order.Status)

		// Cancel order
		cancelledOrder, err := exchange.CancelOrder(resp.OrderID)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusCancelled, cancelledOrder.Status)
	})

	t.Run("Place market sell order", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideSell,
			Type:     OrderTypeMarket,
			Quantity: 0.02,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.OrderID)
		assert.Equal(t, OrderStatusFilled, resp.Status)

		// Get fills
		fills, err := exchange.GetOrderFills(resp.OrderID)
		require.NoError(t, err)
		assert.NotEmpty(t, fills)

		totalQty := 0.0
		for _, fill := range fills {
			totalQty += fill.Quantity
		}
		assert.InDelta(t, 0.02, totalQty, 0.0001)
	})
}

// TestMockExchangeValidation tests order validation
func TestMockExchangeValidation(t *testing.T) {
	exchange := NewMockExchange(nil)
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	t.Run("Empty symbol", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 0.1,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err) // No error, but status is rejected
		assert.Equal(t, OrderStatusRejected, resp.Status)
	})

	t.Run("Invalid side", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSide("INVALID"),
			Type:     OrderTypeMarket,
			Quantity: 0.1,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusRejected, resp.Status)
	})

	t.Run("Zero quantity", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 0,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusRejected, resp.Status)
	})

	t.Run("Limit order without price", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeLimit,
			Quantity: 0.1,
			Price:    0,
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusRejected, resp.Status)
	})
}

// TestMockExchangeSlippage tests slippage simulation
func TestMockExchangeSlippage(t *testing.T) {
	exchange := NewMockExchange(nil)
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	t.Run("Small order has minimal slippage", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 0.01, // Small quantity
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)

		order, err := exchange.GetOrder(resp.OrderID)
		require.NoError(t, err)

		// Slippage should be minimal (< 0.1%)
		slippage := (order.AvgFillPrice - 50000.0) / 50000.0 * 100
		assert.Less(t, slippage, 0.1)
	})

	t.Run("Large order has more slippage", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 5.0, // Large quantity
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)

		order, err := exchange.GetOrder(resp.OrderID)
		require.NoError(t, err)

		// Large orders should have more slippage than small orders
		slippage := (order.AvgFillPrice - 50000.0) / 50000.0 * 100
		assert.Greater(t, slippage, 0.05, "Large order slippage should be greater than 0.05%")
	})
}

// TestMockExchangePartialFills tests partial fill simulation
func TestMockExchangePartialFills(t *testing.T) {
	exchange := NewMockExchange(nil)
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	t.Run("Large order gets partial fills", func(t *testing.T) {
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 10.0, // Very large quantity
		}

		resp, err := exchange.PlaceOrder(req)
		require.NoError(t, err)

		// Get fills
		fills, err := exchange.GetOrderFills(resp.OrderID)
		require.NoError(t, err)

		// Should have multiple fills
		assert.Greater(t, len(fills), 1, "Large order should have multiple fills")

		// Total filled quantity should match requested
		totalQty := 0.0
		for _, fill := range fills {
			totalQty += fill.Quantity
		}
		assert.InDelta(t, 10.0, totalQty, 0.001)
	})
}

// TestPositionManager tests position tracking and P&L calculation
func TestPositionManager(t *testing.T) {
	// Create position manager without database
	pm := NewPositionManager(nil)

	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	t.Run("Open long position", func(t *testing.T) {
		// Create buy order
		order := &Order{
			ID:           uuid.New().String(),
			Symbol:       "BTCUSDT",
			Side:         OrderSideBuy,
			Type:         OrderTypeMarket,
			Quantity:     0.1,
			Price:        50000.0,
			Status:       OrderStatusFilled,
			FilledQty:    0.1,
			AvgFillPrice: 50000.0,
			CreatedAt:    time.Now(),
		}

		fills := []Fill{
			{
				OrderID:   order.ID,
				Quantity:  0.1,
				Price:     50000.0,
				Timestamp: time.Now(),
			},
		}

		ctx := context.Background()
		err := pm.OnOrderFilled(ctx, order, fills)
		require.NoError(t, err)

		// Check position
		position, exists := pm.GetPosition("BTCUSDT")
		require.True(t, exists)
		assert.Equal(t, 0.1, position.Quantity)
		assert.Equal(t, 50000.0, position.EntryPrice)
	})

	t.Run("Update unrealized P&L", func(t *testing.T) {
		ctx := context.Background()
		prices := map[string]float64{
			"BTCUSDT": 55000.0, // Price went up
		}

		err := pm.UpdateUnrealizedPnL(ctx, prices)
		require.NoError(t, err)

		totalPnL := pm.GetTotalUnrealizedPnL()
		// Profit: (55000 - 50000) * 0.1 = 500
		assert.InDelta(t, 500.0, totalPnL, 1.0)
	})

	t.Run("Close position", func(t *testing.T) {
		// Create sell order to close
		order := &Order{
			ID:           uuid.New().String(),
			Symbol:       "BTCUSDT",
			Side:         OrderSideSell,
			Type:         OrderTypeMarket,
			Quantity:     0.1,
			Price:        55000.0,
			Status:       OrderStatusFilled,
			FilledQty:    0.1,
			AvgFillPrice: 55000.0,
			CreatedAt:    time.Now(),
		}

		fills := []Fill{
			{
				OrderID:   order.ID,
				Quantity:  0.1,
				Price:     55000.0,
				Timestamp: time.Now(),
			},
		}

		ctx := context.Background()
		err := pm.OnOrderFilled(ctx, order, fills)
		require.NoError(t, err)

		// Position should be closed
		_, exists := pm.GetPosition("BTCUSDT")
		assert.False(t, exists)

		// Check realized P&L was recorded
		totalPnL := pm.GetTotalUnrealizedPnL()
		assert.Equal(t, 0.0, totalPnL) // No open positions
	})
}

// TestPositionManagerShortPosition tests short position handling
func TestPositionManagerShortPosition(t *testing.T) {
	pm := NewPositionManager(nil)

	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	t.Run("Open short position", func(t *testing.T) {
		// Create sell order (short)
		order := &Order{
			ID:           uuid.New().String(),
			Symbol:       "ETHUSDT",
			Side:         OrderSideSell,
			Type:         OrderTypeMarket,
			Quantity:     1.0,
			Price:        3000.0,
			Status:       OrderStatusFilled,
			FilledQty:    1.0,
			AvgFillPrice: 3000.0,
			CreatedAt:    time.Now(),
		}

		fills := []Fill{
			{
				OrderID:   order.ID,
				Quantity:  1.0,
				Price:     3000.0,
				Timestamp: time.Now(),
			},
		}

		ctx := context.Background()
		err := pm.OnOrderFilled(ctx, order, fills)
		require.NoError(t, err)

		// Check position
		position, exists := pm.GetPosition("ETHUSDT")
		require.True(t, exists)
		assert.Equal(t, 1.0, position.Quantity) // Quantity is always positive
		assert.Equal(t, db.PositionSideShort, position.Side) // Side indicates SHORT
		assert.Equal(t, 3000.0, position.EntryPrice)
	})

	t.Run("P&L for short position", func(t *testing.T) {
		prices := map[string]float64{
			"ETHUSDT": 2800.0, // Price went down (profit for short)
		}

		err := pm.UpdateUnrealizedPnL(context.Background(), prices)
		require.NoError(t, err)

		totalPnL := pm.GetTotalUnrealizedPnL()
		// Profit: (3000 - 2800) * 1.0 = 200
		assert.InDelta(t, 200.0, totalPnL, 1.0)
	})
}

// TestServiceIntegration tests the service layer integration
func TestServiceIntegration(t *testing.T) {
	// Create service with paper trading mode
	service := NewServicePaper(nil)

	t.Run("Place market order via service", func(t *testing.T) {
		// Set market price first
		service.exchange.SetMarketPrice("BTCUSDT", 50000.0)

		args := map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 0.1,
		}

		result, err := service.PlaceMarketOrder(args)
		require.NoError(t, err)

		order := result.(*Order)
		assert.Equal(t, "BTCUSDT", order.Symbol)
		assert.Equal(t, OrderSideBuy, order.Side)
		assert.Equal(t, OrderStatusFilled, order.Status)
	})

	t.Run("Get positions (without session)", func(t *testing.T) {
		result, err := service.GetPositions(map[string]interface{}{})
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})
		count := resultMap["count"].(int)

		// Without an active session, there should be no positions
		assert.Equal(t, 0, count)
	})

	t.Run("Update position P&L (without session)", func(t *testing.T) {
		args := map[string]interface{}{
			"prices": map[string]interface{}{
				"BTCUSDT": 55000.0,
			},
		}

		result, err := service.UpdatePositionPnL(args)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})
		totalPnL := resultMap["total_unrealized_pnl"].(float64)

		// Without positions, P&L should be 0
		assert.Equal(t, 0.0, totalPnL)
	})
}

// TestRetryLogic tests the retry mechanism
func TestRetryLogic(t *testing.T) {
	t.Run("isRetryableError identifies retryable errors", func(t *testing.T) {
		retryableErrors := []string{
			"connection refused",
			"connection reset",
			"timeout",
			"429 Too Many Requests",
			"500 Internal Server Error",
			"503 Service Unavailable",
		}

		for _, errMsg := range retryableErrors {
			err := &mockError{msg: errMsg}
			assert.True(t, isRetryableError(err), "Error should be retryable: %s", errMsg)
		}
	})

	t.Run("isRetryableError rejects non-retryable errors", func(t *testing.T) {
		nonRetryableErrors := []string{
			"invalid API key",
			"insufficient balance",
			"400 Bad Request",
			"401 Unauthorized",
		}

		for _, errMsg := range nonRetryableErrors {
			err := &mockError{msg: errMsg}
			assert.False(t, isRetryableError(err), "Error should not be retryable: %s", errMsg)
		}
	})

	t.Run("retryWithBackoff succeeds on first try", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}

		err := retryWithBackoff(operation, "test_operation")
		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("retryWithBackoff retries on transient errors", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return &mockError{msg: "connection refused"}
			}
			return nil
		}

		err := retryWithBackoff(operation, "test_operation")
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("retryWithBackoff fails after max retries", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return &mockError{msg: "503 Service Unavailable"}
		}

		err := retryWithBackoff(operation, "test_operation")
		assert.Error(t, err)
		assert.Equal(t, maxRetries+1, attempts)
	})

	t.Run("retryWithBackoff does not retry non-retryable errors", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return &mockError{msg: "invalid API key"}
		}

		err := retryWithBackoff(operation, "test_operation")
		assert.Error(t, err)
		assert.Equal(t, 1, attempts)
	})
}

// mockError is a simple error type for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
