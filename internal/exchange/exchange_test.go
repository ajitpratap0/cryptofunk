package exchange

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.OrderID)
		assert.Equal(t, OrderStatusOpen, resp.Status)

		// Get order
		order, err := exchange.GetOrder(context.Background(), resp.OrderID)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusOpen, order.Status)

		// Cancel order
		cancelledOrder, err := exchange.CancelOrder(context.Background(), resp.OrderID)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.OrderID)
		assert.Equal(t, OrderStatusFilled, resp.Status)

		// Get fills
		fills, err := exchange.GetOrderFills(context.Background(), resp.OrderID)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)

		order, err := exchange.GetOrder(context.Background(), resp.OrderID)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)

		order, err := exchange.GetOrder(context.Background(), resp.OrderID)
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

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)

		// Get fills
		fills, err := exchange.GetOrderFills(context.Background(), resp.OrderID)
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
		assert.Equal(t, 1.0, position.Quantity)              // Quantity is always positive
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

	t.Run("Place limit order via service", func(t *testing.T) {
		service.exchange.SetMarketPrice("ETHUSDT", 3000.0)

		args := map[string]interface{}{
			"symbol":   "ETHUSDT",
			"side":     "sell",
			"quantity": 1.0,
			"price":    3100.0,
		}

		result, err := service.PlaceLimitOrder(args)
		require.NoError(t, err)

		order := result.(*Order)
		assert.Equal(t, "ETHUSDT", order.Symbol)
		assert.Equal(t, OrderSideSell, order.Side)
		assert.Equal(t, OrderTypeLimit, order.Type)
		assert.Equal(t, OrderStatusOpen, order.Status)
		assert.Equal(t, 3100.0, order.Price)
	})

	t.Run("Cancel order via service", func(t *testing.T) {
		// First, place a limit order
		service.exchange.SetMarketPrice("BTCUSDT", 50000.0)
		args := map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 0.1,
			"price":    48000.0,
		}

		result, err := service.PlaceLimitOrder(args)
		require.NoError(t, err)
		order := result.(*Order)

		// Cancel it
		cancelArgs := map[string]interface{}{
			"order_id": order.ID,
		}

		cancelResult, err := service.CancelOrder(cancelArgs)
		require.NoError(t, err)

		cancelledOrder := cancelResult.(*Order)
		assert.Equal(t, OrderStatusCancelled, cancelledOrder.Status)
	})

	t.Run("Get order status via service", func(t *testing.T) {
		// Place an order
		service.exchange.SetMarketPrice("BTCUSDT", 50000.0)
		args := map[string]interface{}{
			"symbol":   "BTCUSDT",
			"side":     "buy",
			"quantity": 0.05,
		}

		result, err := service.PlaceMarketOrder(args)
		require.NoError(t, err)
		order := result.(*Order)

		// Get its status
		statusArgs := map[string]interface{}{
			"order_id": order.ID,
		}

		statusResult, err := service.GetOrderStatus(statusArgs)
		require.NoError(t, err)

		statusMap := statusResult.(map[string]interface{})
		statusOrder := statusMap["order"].(*Order)
		assert.Equal(t, order.ID, statusOrder.ID)
		assert.Equal(t, OrderStatusFilled, statusOrder.Status)
	})

	t.Run("Get positions (without session)", func(t *testing.T) {
		result, err := service.GetPositions(map[string]interface{}{})
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})
		count := resultMap["count"].(int)

		// Without an active session, there should be no positions
		assert.Equal(t, 0, count)
	})

	t.Run("Get position by symbol (without session)", func(t *testing.T) {
		args := map[string]interface{}{
			"symbol": "BTCUSDT",
		}

		result, err := service.GetPositionBySymbol(args)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})
		exists := resultMap["exists"].(bool)

		// Without session, position shouldn't exist
		assert.False(t, exists)
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

	t.Run("Close position by symbol (without position)", func(t *testing.T) {
		args := map[string]interface{}{
			"symbol": "BTCUSDT",
		}

		_, err := service.ClosePositionBySymbol(args)
		// Should return error when no position exists
		assert.Error(t, err)
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

// TestMockExchangeSessionManagement tests session management
func TestMockExchangeSessionManagement(t *testing.T) {
	exchange := NewMockExchange(nil)

	t.Run("GetSession returns nil initially", func(t *testing.T) {
		session := exchange.GetSession()
		assert.Nil(t, session)
	})

	t.Run("SetSession and GetSession", func(t *testing.T) {
		sessionID := uuid.New()
		exchange.SetSession(&sessionID)

		retrievedSession := exchange.GetSession()
		require.NotNil(t, retrievedSession)
		assert.Equal(t, sessionID, *retrievedSession)
	})

	t.Run("SetSession with nil clears session", func(t *testing.T) {
		// First set a session
		sessionID := uuid.New()
		exchange.SetSession(&sessionID)
		assert.NotNil(t, exchange.GetSession())

		// Clear it
		exchange.SetSession(nil)
		assert.Nil(t, exchange.GetSession())
	})
}

// TestConvertToDBOrder tests the convertToDBOrder function
func TestConvertToDBOrder(t *testing.T) {
	exchange := NewMockExchange(nil)
	sessionID := uuid.New()
	exchange.SetSession(&sessionID)

	t.Run("Convert filled market order", func(t *testing.T) {
		now := time.Now()
		filledAt := now
		order := &Order{
			ID:           uuid.New().String(),
			Symbol:       "BTCUSDT",
			Side:         OrderSideBuy,
			Type:         OrderTypeMarket,
			Status:       OrderStatusFilled,
			Price:        50000.0,
			Quantity:     0.1,
			FilledQty:    0.1,
			AvgFillPrice: 50100.0,
			CreatedAt:    now,
			UpdatedAt:    now,
			FilledAt:     &filledAt,
		}

		dbOrder := exchange.convertToDBOrder(order)

		assert.Equal(t, order.Symbol, dbOrder.Symbol)
		assert.Equal(t, "PAPER", dbOrder.Exchange)
		assert.Equal(t, db.OrderSideBuy, dbOrder.Side)
		assert.Equal(t, db.OrderTypeMarket, dbOrder.Type)
		assert.Equal(t, db.OrderStatusFilled, dbOrder.Status)
		assert.NotNil(t, dbOrder.Price)
		assert.Equal(t, 50000.0, *dbOrder.Price)
		assert.Equal(t, 0.1, dbOrder.Quantity)
		assert.Equal(t, 0.1, dbOrder.ExecutedQuantity)
		assert.InDelta(t, 5010.0, dbOrder.ExecutedQuoteQuantity, 1.0) // 0.1 * 50100
		assert.NotNil(t, dbOrder.SessionID)
		assert.Equal(t, sessionID, *dbOrder.SessionID)
	})

	t.Run("Convert limit order with zero price", func(t *testing.T) {
		order := &Order{
			ID:        uuid.New().String(),
			Symbol:    "ETHUSDT",
			Side:      OrderSideSell,
			Type:      OrderTypeLimit,
			Status:    OrderStatusOpen,
			Price:     0, // Zero price
			Quantity:  1.0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		dbOrder := exchange.convertToDBOrder(order)

		assert.Nil(t, dbOrder.Price) // Price should be nil when zero
		assert.Equal(t, db.OrderTypeLimit, dbOrder.Type)
		// OrderStatusOpen maps to OrderStatusPartiallyFilled in db
		assert.Equal(t, db.OrderStatusPartiallyFilled, dbOrder.Status)
	})

	t.Run("Convert order with empty ID", func(t *testing.T) {
		order := &Order{
			ID:        "", // Empty ID
			Symbol:    "BTCUSDT",
			Side:      OrderSideBuy,
			Type:      OrderTypeMarket,
			Status:    OrderStatusFilled,
			Price:     50000.0,
			Quantity:  0.1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		dbOrder := exchange.convertToDBOrder(order)

		// Should handle empty ID gracefully
		assert.NotNil(t, dbOrder)
		assert.Equal(t, order.Symbol, dbOrder.Symbol)
	})
}

// TestUpdateOrderStatusInDB tests updateOrderStatusInDB with nil db
func TestUpdateOrderStatusInDB(t *testing.T) {
	exchange := NewMockExchange(nil) // nil database

	t.Run("Should not panic with nil database", func(t *testing.T) {
		order := &Order{
			ID:           uuid.New().String(),
			Symbol:       "BTCUSDT",
			Side:         OrderSideBuy,
			Type:         OrderTypeMarket,
			Status:       OrderStatusFilled,
			FilledQty:    0.1,
			AvgFillPrice: 50000.0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		// Should not panic, just return silently when db is nil
		assert.NotPanics(t, func() {
			exchange.updateOrderStatusInDB(context.Background(), order)
		})
	})
}

// TestPersistTradeInDB tests persistTradeInDB with nil db
func TestPersistTradeInDB(t *testing.T) {
	exchange := NewMockExchange(nil) // nil database

	t.Run("Should not panic with nil database", func(t *testing.T) {
		orderID := uuid.New().String()
		order := &Order{
			ID:        orderID,
			Symbol:    "BTCUSDT",
			Side:      OrderSideBuy,
			Type:      OrderTypeMarket,
			Status:    OrderStatusFilled,
			CreatedAt: time.Now(),
		}

		// Add order to exchange
		exchange.orders[orderID] = order

		fill := Fill{
			OrderID:   orderID,
			Quantity:  0.1,
			Price:     50000.0,
			Timestamp: time.Now(),
		}

		// Should not panic, just return silently when db is nil
		assert.NotPanics(t, func() {
			exchange.persistTradeInDB(context.Background(), orderID, fill)
		})
	})
}

// TestCancelOrderEdgeCases tests additional cancel order scenarios
func TestCancelOrderEdgeCases(t *testing.T) {
	exchange := NewMockExchange(nil)
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	t.Run("Cancel non-existent order", func(t *testing.T) {
		_, err := exchange.CancelOrder(context.Background(), "non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})

	t.Run("Cancel already filled order", func(t *testing.T) {
		// Place and fill a market order
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeMarket,
			Quantity: 0.1,
		}

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusFilled, resp.Status)

		// Try to cancel
		_, err = exchange.CancelOrder(context.Background(), resp.OrderID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot cancel order in status")
	})

	t.Run("Cancel already cancelled order", func(t *testing.T) {
		// Place a limit order
		req := PlaceOrderRequest{
			Symbol:   "BTCUSDT",
			Side:     OrderSideBuy,
			Type:     OrderTypeLimit,
			Quantity: 0.1,
			Price:    45000.0,
		}

		resp, err := exchange.PlaceOrder(context.Background(), req)
		require.NoError(t, err)

		// Cancel it
		_, err = exchange.CancelOrder(context.Background(), resp.OrderID)
		require.NoError(t, err)

		// Try to cancel again
		_, err = exchange.CancelOrder(context.Background(), resp.OrderID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot cancel order in status")
	})
}

// TestGetOrderEdgeCases tests GetOrder edge cases
func TestGetOrderEdgeCases(t *testing.T) {
	exchange := NewMockExchange(nil)

	t.Run("Get non-existent order", func(t *testing.T) {
		_, err := exchange.GetOrder(context.Background(), "non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

// TestGetOrderFillsEdgeCases tests GetOrderFills edge cases
func TestGetOrderFillsEdgeCases(t *testing.T) {
	exchange := NewMockExchange(nil)

	t.Run("Get fills for non-existent order", func(t *testing.T) {
		fills, err := exchange.GetOrderFills(context.Background(), "non-existent-id")
		require.NoError(t, err)
		assert.Empty(t, fills)
	})
}

// TestMockExchangeConcurrency tests concurrent access to mock exchange
func TestMockExchangeConcurrency(t *testing.T) {
	exchange := NewMockExchange(nil)
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	t.Run("Concurrent order placement", func(t *testing.T) {
		const numOrders = 10
		done := make(chan bool, numOrders)

		for i := 0; i < numOrders; i++ {
			go func() {
				req := PlaceOrderRequest{
					Symbol:   "BTCUSDT",
					Side:     OrderSideBuy,
					Type:     OrderTypeMarket,
					Quantity: 0.01,
				}

				_, err := exchange.PlaceOrder(context.Background(), req)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < numOrders; i++ {
			<-done
		}

		// All orders should be recorded
		exchange.mu.RLock()
		orderCount := len(exchange.orders)
		exchange.mu.RUnlock()

		assert.Equal(t, numOrders, orderCount)
	})

	t.Run("Concurrent price updates and order placement", func(t *testing.T) {
		const numIterations = 20
		done := make(chan bool, numIterations*2)

		// Price updater goroutines
		for i := 0; i < numIterations; i++ {
			go func(iteration int) {
				price := 50000.0 + float64(iteration)*100
				exchange.SetMarketPrice("ETHUSDT", price)
				done <- true
			}(i)
		}

		// Order placement goroutines
		for i := 0; i < numIterations; i++ {
			go func() {
				req := PlaceOrderRequest{
					Symbol:   "ETHUSDT",
					Side:     OrderSideBuy,
					Type:     OrderTypeMarket,
					Quantity: 0.01,
				}

				_, err := exchange.PlaceOrder(context.Background(), req)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < numIterations*2; i++ {
			<-done
		}
	})
}
