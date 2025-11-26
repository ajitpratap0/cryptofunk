package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// TestOrderHelperMethodsWithTestcontainers tests uncovered order helper methods
func TestOrderHelperMethodsWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create test session
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	err = tc.DB.CreateSession(ctx, session)
	require.NoError(t, err)

	t.Run("GetOrdersBySymbol", func(t *testing.T) {
		// Create orders with different symbols
		btcOrder1 := &db.Order{
			ID:        uuid.New(),
			SessionID: &session.ID,
			Symbol:    "BTC/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideBuy,
			Type:      db.OrderTypeMarket,
			Quantity:  0.1,
			Status:    db.OrderStatusNew,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tc.DB.InsertOrder(ctx, btcOrder1)
		require.NoError(t, err)

		btcOrder2 := &db.Order{
			ID:        uuid.New(),
			SessionID: &session.ID,
			Symbol:    "BTC/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideSell,
			Type:      db.OrderTypeLimit,
			Quantity:  0.2,
			Status:    db.OrderStatusFilled,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err = tc.DB.InsertOrder(ctx, btcOrder2)
		require.NoError(t, err)

		ethOrder := &db.Order{
			ID:        uuid.New(),
			SessionID: &session.ID,
			Symbol:    "ETH/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideBuy,
			Type:      db.OrderTypeMarket,
			Quantity:  1.0,
			Status:    db.OrderStatusNew,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err = tc.DB.InsertOrder(ctx, ethOrder)
		require.NoError(t, err)

		// Test retrieval by symbol
		btcOrders, err := tc.DB.GetOrdersBySymbol(ctx, "BTC/USDT")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(btcOrders), 2, "Should have at least 2 BTC orders")

		// Verify all returned orders have correct symbol
		for _, order := range btcOrders {
			assert.Equal(t, "BTC/USDT", order.Symbol)
		}

		ethOrders, err := tc.DB.GetOrdersBySymbol(ctx, "ETH/USDT")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(ethOrders), 1, "Should have at least 1 ETH order")

		for _, order := range ethOrders {
			assert.Equal(t, "ETH/USDT", order.Symbol)
		}
	})

	t.Run("GetOrdersByStatus", func(t *testing.T) {
		// Create orders with different statuses
		newOrder := &db.Order{
			ID:        uuid.New(),
			SessionID: &session.ID,
			Symbol:    "SOL/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideBuy,
			Type:      db.OrderTypeMarket,
			Quantity:  10.0,
			Status:    db.OrderStatusNew,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tc.DB.InsertOrder(ctx, newOrder)
		require.NoError(t, err)

		filledOrder := &db.Order{
			ID:        uuid.New(),
			SessionID: &session.ID,
			Symbol:    "SOL/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideSell,
			Type:      db.OrderTypeMarket,
			Quantity:  5.0,
			Status:    db.OrderStatusFilled,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err = tc.DB.InsertOrder(ctx, filledOrder)
		require.NoError(t, err)

		canceledOrder := &db.Order{
			ID:        uuid.New(),
			SessionID: &session.ID,
			Symbol:    "SOL/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideBuy,
			Type:      db.OrderTypeLimit,
			Quantity:  15.0,
			Status:    db.OrderStatusCanceled,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err = tc.DB.InsertOrder(ctx, canceledOrder)
		require.NoError(t, err)

		// Test retrieval by status
		newOrders, err := tc.DB.GetOrdersByStatus(ctx, db.OrderStatusNew)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(newOrders), 1, "Should have at least 1 NEW order")
		for _, order := range newOrders {
			assert.Equal(t, db.OrderStatusNew, order.Status)
		}

		filledOrders, err := tc.DB.GetOrdersByStatus(ctx, db.OrderStatusFilled)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(filledOrders), 1, "Should have at least 1 FILLED order")
		for _, order := range filledOrders {
			assert.Equal(t, db.OrderStatusFilled, order.Status)
		}

		canceledOrders, err := tc.DB.GetOrdersByStatus(ctx, db.OrderStatusCanceled)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(canceledOrders), 1, "Should have at least 1 CANCELED order")
		for _, order := range canceledOrders {
			assert.Equal(t, db.OrderStatusCanceled, order.Status)
		}
	})

	t.Run("GetRecentOrders", func(t *testing.T) {
		// Create multiple orders with different timestamps
		for i := 0; i < 10; i++ {
			order := &db.Order{
				ID:        uuid.New(),
				SessionID: &session.ID,
				Symbol:    "ADA/USDT",
				Exchange:  "binance",
				Side:      db.OrderSideBuy,
				Type:      db.OrderTypeMarket,
				Quantity:  100.0,
				Status:    db.OrderStatusNew,
				PlacedAt:  time.Now().Add(time.Duration(-i) * time.Minute),
				CreatedAt: time.Now().Add(time.Duration(-i) * time.Minute),
				UpdatedAt: time.Now().Add(time.Duration(-i) * time.Minute),
			}
			err := tc.DB.InsertOrder(ctx, order)
			require.NoError(t, err)
		}

		// Test limit parameter
		limit := 5
		recentOrders, err := tc.DB.GetRecentOrders(ctx, limit)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(recentOrders), limit, "Should return at least the limited number of orders")

		// Verify orders are sorted by created_at DESC (most recent first)
		for i := 1; i < len(recentOrders) && i < limit; i++ {
			assert.True(t, recentOrders[i-1].CreatedAt.After(recentOrders[i].CreatedAt) ||
				recentOrders[i-1].CreatedAt.Equal(recentOrders[i].CreatedAt),
				"Orders should be sorted by created_at DESC")
		}
	})

	t.Run("GetOrderByID", func(t *testing.T) {
		// Create an order
		orderID := uuid.New()
		price := 45000.0
		order := &db.Order{
			ID:        orderID,
			SessionID: &session.ID,
			Symbol:    "BTC/USDT",
			Exchange:  "binance",
			Side:      db.OrderSideBuy,
			Type:      db.OrderTypeLimit,
			Quantity:  0.5,
			Price:     &price,
			Status:    db.OrderStatusNew,
			PlacedAt:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tc.DB.InsertOrder(ctx, order)
		require.NoError(t, err)

		// Test GetOrderByID (alias for GetOrder)
		retrieved, err := tc.DB.GetOrderByID(ctx, orderID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, orderID, retrieved.ID)
		assert.Equal(t, "BTC/USDT", retrieved.Symbol)
		assert.Equal(t, db.OrderSideBuy, retrieved.Side)
		assert.Equal(t, db.OrderTypeLimit, retrieved.Type)
		assert.Equal(t, 0.5, retrieved.Quantity)
		assert.NotNil(t, retrieved.Price)
		assert.Equal(t, 45000.0, *retrieved.Price)
	})
}

// TestPositionHelperMethodsWithTestcontainers tests uncovered position helper methods
func TestPositionHelperMethodsWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create test sessions
	session1 := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	err = tc.DB.CreateSession(ctx, session1)
	require.NoError(t, err)

	session2 := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "ETH/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 5000.0,
	}
	err = tc.DB.CreateSession(ctx, session2)
	require.NoError(t, err)

	t.Run("GetOpenPositions", func(t *testing.T) {
		// Create open positions for session1
		openPos1 := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "BTC/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   0.5,
			EntryPrice: 48000.0,
			EntryTime:  time.Now(),
		}
		err := tc.DB.CreatePosition(ctx, openPos1)
		require.NoError(t, err)

		openPos2 := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "ETH/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideShort,
			Quantity:   2.0,
			EntryPrice: 3000.0,
			EntryTime:  time.Now(),
		}
		err = tc.DB.CreatePosition(ctx, openPos2)
		require.NoError(t, err)

		// Create a closed position for session1
		closedPos := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "SOL/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   10.0,
			EntryPrice: 100.0,
			EntryTime:  time.Now().Add(-1 * time.Hour),
		}
		err = tc.DB.CreatePosition(ctx, closedPos)
		require.NoError(t, err)
		err = tc.DB.ClosePosition(ctx, closedPos.ID, 110.0, "take_profit", 1.0)
		require.NoError(t, err)

		// Test GetOpenPositions for session1
		openPositions, err := tc.DB.GetOpenPositions(ctx, session1.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, len(openPositions), "Should have exactly 2 open positions")

		// Verify all returned positions are open
		for _, pos := range openPositions {
			assert.Nil(t, pos.ExitTime, "Open position should have nil exit time")
			assert.Equal(t, session1.ID, *pos.SessionID)
		}

		// Verify positions are sorted by entry_time DESC
		if len(openPositions) >= 2 {
			assert.True(t, openPositions[0].EntryTime.After(openPositions[1].EntryTime) ||
				openPositions[0].EntryTime.Equal(openPositions[1].EntryTime),
				"Positions should be sorted by entry_time DESC")
		}
	})

	t.Run("GetPositionsBySession", func(t *testing.T) {
		// Create positions for session2
		pos1 := &db.Position{
			SessionID:  &session2.ID,
			Symbol:     "ETH/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   5.0,
			EntryPrice: 2900.0,
			EntryTime:  time.Now(),
		}
		err := tc.DB.CreatePosition(ctx, pos1)
		require.NoError(t, err)

		pos2 := &db.Position{
			SessionID:  &session2.ID,
			Symbol:     "BNB/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   20.0,
			EntryPrice: 300.0,
			EntryTime:  time.Now().Add(-30 * time.Minute),
		}
		err = tc.DB.CreatePosition(ctx, pos2)
		require.NoError(t, err)

		// Close one position
		err = tc.DB.ClosePosition(ctx, pos2.ID, 310.0, "take_profit", 0.5)
		require.NoError(t, err)

		// Test GetPositionsBySession (should return both open and closed)
		allPositions, err := tc.DB.GetPositionsBySession(ctx, session2.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, len(allPositions), "Should return all positions (open and closed)")

		// Verify session ID
		for _, pos := range allPositions {
			assert.Equal(t, session2.ID, *pos.SessionID)
		}

		// Count open vs closed
		openCount := 0
		closedCount := 0
		for _, pos := range allPositions {
			if pos.ExitTime == nil {
				openCount++
			} else {
				closedCount++
			}
		}
		assert.Equal(t, 1, openCount, "Should have 1 open position")
		assert.Equal(t, 1, closedCount, "Should have 1 closed position")
	})

	t.Run("GetAllOpenPositions", func(t *testing.T) {
		// This should return open positions from all sessions
		allOpenPositions, err := tc.DB.GetAllOpenPositions(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allOpenPositions), 3, "Should have at least 3 open positions across all sessions")

		// Verify all are open
		for _, pos := range allOpenPositions {
			assert.Nil(t, pos.ExitTime, "All positions should be open")
		}

		// Verify sorted by entry_time DESC
		for i := 1; i < len(allOpenPositions); i++ {
			assert.True(t, allOpenPositions[i-1].EntryTime.After(allOpenPositions[i].EntryTime) ||
				allOpenPositions[i-1].EntryTime.Equal(allOpenPositions[i].EntryTime),
				"Positions should be sorted by entry_time DESC")
		}
	})

	t.Run("GetPositionBySymbolAndSession", func(t *testing.T) {
		// Create a specific position
		pos := &db.Position{
			SessionID:   &session1.ID,
			Symbol:      "MATIC/USDT",
			Exchange:    "binance",
			Side:        db.PositionSideLong,
			Quantity:    1000.0,
			EntryPrice:  0.8,
			EntryTime:   time.Now(),
			EntryReason: strPtr("breakout_signal"),
		}
		err := tc.DB.CreatePosition(ctx, pos)
		require.NoError(t, err)

		// Test retrieval by symbol and session
		retrieved, err := tc.DB.GetPositionBySymbolAndSession(ctx, "MATIC/USDT", session1.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, "MATIC/USDT", retrieved.Symbol)
		assert.Equal(t, session1.ID, *retrieved.SessionID)
		assert.Equal(t, 1000.0, retrieved.Quantity)
		assert.Equal(t, 0.8, retrieved.EntryPrice)
		assert.NotNil(t, retrieved.EntryReason)
		assert.Equal(t, "breakout_signal", *retrieved.EntryReason)
	})

	t.Run("GetLatestPositionBySymbol", func(t *testing.T) {
		// Create multiple positions for same symbol across different sessions
		now := time.Now()

		oldPos := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "LINK/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   50.0,
			EntryPrice: 15.0,
			EntryTime:  now.Add(-2 * time.Hour),
		}
		err := tc.DB.CreatePosition(ctx, oldPos)
		require.NoError(t, err)

		// Close the old position
		err = tc.DB.ClosePosition(ctx, oldPos.ID, 16.0, "take_profit", 0.1)
		require.NoError(t, err)

		// Create latest open position
		latestPos := &db.Position{
			SessionID:  &session2.ID,
			Symbol:     "LINK/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideShort,
			Quantity:   75.0,
			EntryPrice: 16.5,
			EntryTime:  now.Add(-30 * time.Minute),
		}
		err = tc.DB.CreatePosition(ctx, latestPos)
		require.NoError(t, err)

		// Test GetLatestPositionBySymbol
		retrieved, err := tc.DB.GetLatestPositionBySymbol(ctx, "LINK/USDT")
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, "LINK/USDT", retrieved.Symbol)
		assert.Equal(t, latestPos.ID, retrieved.ID, "Should return the most recent open position")
		assert.Equal(t, 75.0, retrieved.Quantity)
		assert.Equal(t, 16.5, retrieved.EntryPrice)
		assert.Nil(t, retrieved.ExitTime, "Latest position should be open")
	})

	t.Run("UpdatePositionQuantity", func(t *testing.T) {
		// Create a position
		pos := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "AVAX/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   100.0,
			EntryPrice: 20.0,
			EntryTime:  time.Now(),
			// Note: Fees field is not inserted by CreatePosition, defaults to 0
		}
		err := tc.DB.CreatePosition(ctx, pos)
		require.NoError(t, err)

		// Update quantity (partial close scenario)
		newQuantity := 60.0
		additionalFees := 5.0
		err = tc.DB.UpdatePositionQuantity(ctx, pos.ID, newQuantity, additionalFees)
		require.NoError(t, err)

		// Verify update
		updated, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)

		assert.Equal(t, 60.0, updated.Quantity, "Quantity should be updated")
		assert.Equal(t, 5.0, updated.Fees, "Fees should be accumulated (0 + 5)")
		assert.Nil(t, updated.ExitTime, "Position should still be open")
	})

	t.Run("UpdatePositionAveraging", func(t *testing.T) {
		// Create a position
		pos := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "DOT/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   50.0,
			EntryPrice: 10.0,
			EntryTime:  time.Now(),
			// Note: Fees field is not inserted by CreatePosition, defaults to 0
		}
		err := tc.DB.CreatePosition(ctx, pos)
		require.NoError(t, err)

		// Add to position (averaging scenario)
		// Original: 50 @ 10.0 = 500
		// Adding: 50 @ 12.0 = 600
		// Average: 100 @ 11.0 = 1100
		newEntryPrice := 11.0
		newQuantity := 100.0
		additionalFees := 3.0
		err = tc.DB.UpdatePositionAveraging(ctx, pos.ID, newEntryPrice, newQuantity, additionalFees)
		require.NoError(t, err)

		// Verify update
		updated, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)

		assert.Equal(t, 11.0, updated.EntryPrice, "Entry price should be averaged")
		assert.Equal(t, 100.0, updated.Quantity, "Quantity should be updated")
		assert.Equal(t, 3.0, updated.Fees, "Fees should be accumulated (0 + 3)")
	})

	t.Run("PartialClosePosition", func(t *testing.T) {
		// Create a position
		pos := &db.Position{
			SessionID:  &session1.ID,
			Symbol:     "ATOM/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   200.0,
			EntryPrice: 8.0,
			EntryTime:  time.Now(),
		}
		err := tc.DB.CreatePosition(ctx, pos)
		require.NoError(t, err)

		// Partially close the position
		closeQuantity := 80.0
		exitPrice := 9.0
		exitReason := "partial_take_profit"
		fees := 2.0

		closedPortion, err := tc.DB.PartialClosePosition(ctx, pos.ID, closeQuantity, exitPrice, exitReason, fees)
		require.NoError(t, err)
		require.NotNil(t, closedPortion)

		// Verify closed portion
		assert.Equal(t, 80.0, closedPortion.Quantity, "Closed portion should have correct quantity")
		assert.NotNil(t, closedPortion.ExitPrice)
		assert.Equal(t, 9.0, *closedPortion.ExitPrice)
		assert.NotNil(t, closedPortion.ExitTime, "Closed portion should have exit time")
		assert.NotNil(t, closedPortion.RealizedPnL)
		// (9 - 8) * 80 - 2 = 78
		assert.Equal(t, 78.0, *closedPortion.RealizedPnL)

		// Verify original position is reduced
		remaining, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)
		assert.Equal(t, 120.0, remaining.Quantity, "Remaining quantity should be 200 - 80 = 120")
		assert.Nil(t, remaining.ExitTime, "Remaining position should still be open")
	})
}

// TestConversionFunctionsWithTestcontainers tests conversion helper functions
func TestConversionFunctionsWithTestcontainers(t *testing.T) {
	// Note: These don't require database, but we include them here for completeness
	// and to match the pattern of other integration tests

	t.Run("ConvertOrderSide", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected db.OrderSide
		}{
			{"BUY", db.OrderSideBuy},
			{"buy", db.OrderSideBuy},
			{"Buy", db.OrderSideBuy},
			{"SELL", db.OrderSideSell},
			{"sell", db.OrderSideSell},
			{"Sell", db.OrderSideSell},
			{"unknown", db.OrderSideBuy}, // Default case
		}

		for _, tc := range testCases {
			result := db.ConvertOrderSide(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})

	t.Run("ConvertOrderType", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected db.OrderType
		}{
			{"MARKET", db.OrderTypeMarket},
			{"market", db.OrderTypeMarket},
			{"Market", db.OrderTypeMarket},
			{"LIMIT", db.OrderTypeLimit},
			{"limit", db.OrderTypeLimit},
			{"Limit", db.OrderTypeLimit},
			{"unknown", db.OrderTypeMarket}, // Default case
		}

		for _, tc := range testCases {
			result := db.ConvertOrderType(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})

	t.Run("ConvertOrderStatus", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected db.OrderStatus
		}{
			{"NEW", db.OrderStatusNew},
			{"new", db.OrderStatusNew},
			{"PENDING", db.OrderStatusNew},
			{"pending", db.OrderStatusNew},
			{"PARTIALLY_FILLED", db.OrderStatusPartiallyFilled},
			{"partially_filled", db.OrderStatusPartiallyFilled},
			{"OPEN", db.OrderStatusPartiallyFilled},
			{"open", db.OrderStatusPartiallyFilled},
			{"FILLED", db.OrderStatusFilled},
			{"filled", db.OrderStatusFilled},
			{"CANCELED", db.OrderStatusCanceled},
			{"canceled", db.OrderStatusCanceled},
			{"CANCELLED", db.OrderStatusCanceled},
			{"cancelled", db.OrderStatusCanceled},
			{"REJECTED", db.OrderStatusRejected},
			{"rejected", db.OrderStatusRejected},
			{"unknown", db.OrderStatusNew}, // Default case
		}

		for _, tc := range testCases {
			result := db.ConvertOrderStatus(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})

	t.Run("ConvertPositionSide", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected db.PositionSide
		}{
			{"LONG", db.PositionSideLong},
			{"long", db.PositionSideLong},
			{"buy", db.PositionSideLong},
			{"BUY", db.PositionSideLong},
			{"SHORT", db.PositionSideShort},
			{"short", db.PositionSideShort},
			{"sell", db.PositionSideShort},
			{"SELL", db.PositionSideShort},
			{"unknown", db.PositionSideFlat}, // Default case
		}

		for _, tc := range testCases {
			result := db.ConvertPositionSide(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
