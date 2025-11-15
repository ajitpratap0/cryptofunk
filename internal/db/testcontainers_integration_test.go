package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseConnectionWithTestcontainers tests basic database connectivity using testcontainers
func TestDatabaseConnectionWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Test Ping
	err = tc.DB.Ping(ctx)
	assert.NoError(t, err)

	// Test Health
	err = tc.DB.Health(ctx)
	assert.NoError(t, err)

	// Test Pool
	pool := tc.DB.Pool()
	assert.NotNil(t, pool)
}

// TestTradingSessionCRUDWithTestcontainers tests complete CRUD operations for trading sessions
func TestTradingSessionCRUDWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		session := &db.TradingSession{
			Mode:           db.TradingModePaper,
			Symbol:         "BTC/USDT",
			Exchange:       "binance",
			StartedAt:      time.Now(),
			InitialCapital: 10000.0,
			Config: map[string]interface{}{
				"strategy": "trend_following",
				"risk":     0.02,
			},
			Metadata: map[string]interface{}{
				"test": true,
			},
		}

		err := tc.DB.CreateSession(ctx, session)
		require.NoError(t, err)

		assert.NotEqual(t, uuid.Nil, session.ID)
		assert.NotZero(t, session.CreatedAt)
		assert.NotZero(t, session.UpdatedAt)
	})

	t.Run("Read", func(t *testing.T) {
		// Create session
		session := &db.TradingSession{
			Mode:           db.TradingModePaper,
			Symbol:         "ETH/USDT",
			Exchange:       "binance",
			StartedAt:      time.Now(),
			InitialCapital: 5000.0,
		}

		err := tc.DB.CreateSession(ctx, session)
		require.NoError(t, err)

		// Read it back
		retrieved, err := tc.DB.GetSession(ctx, session.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, session.ID, retrieved.ID)
		assert.Equal(t, session.Mode, retrieved.Mode)
		assert.Equal(t, session.Symbol, retrieved.Symbol)
		assert.Equal(t, session.Exchange, retrieved.Exchange)
		assert.Equal(t, session.InitialCapital, retrieved.InitialCapital)
	})

	t.Run("Update", func(t *testing.T) {
		// Create session
		session := &db.TradingSession{
			Mode:           db.TradingModePaper,
			Symbol:         "SOL/USDT",
			Exchange:       "binance",
			StartedAt:      time.Now(),
			InitialCapital: 3000.0,
		}

		err := tc.DB.CreateSession(ctx, session)
		require.NoError(t, err)

		// Update stats
		sharpeRatio := 1.8
		err = tc.DB.UpdateSessionStats(ctx, session.ID, db.SessionStats{
			TotalTrades:   25,
			WinningTrades: 15,
			LosingTrades:  10,
			TotalPnL:      1500.0,
			MaxDrawdown:   -200.0,
			SharpeRatio:   &sharpeRatio,
		})
		require.NoError(t, err)

		// Verify update
		updated, err := tc.DB.GetSession(ctx, session.ID)
		require.NoError(t, err)

		assert.Equal(t, 25, updated.TotalTrades)
		assert.Equal(t, 15, updated.WinningTrades)
		assert.Equal(t, 10, updated.LosingTrades)
		assert.Equal(t, 1500.0, updated.TotalPnL)
		assert.Equal(t, -200.0, updated.MaxDrawdown)
		assert.NotNil(t, updated.SharpeRatio)
		assert.Equal(t, 1.8, *updated.SharpeRatio)
	})

	t.Run("Stop", func(t *testing.T) {
		// Create session
		session := &db.TradingSession{
			Mode:           db.TradingModePaper,
			Symbol:         "ADA/USDT",
			Exchange:       "binance",
			StartedAt:      time.Now(),
			InitialCapital: 2000.0,
		}

		err := tc.DB.CreateSession(ctx, session)
		require.NoError(t, err)

		// Stop it
		err = tc.DB.StopSession(ctx, session.ID, 2500.0)
		require.NoError(t, err)

		// Verify stopped
		stopped, err := tc.DB.GetSession(ctx, session.ID)
		require.NoError(t, err)

		assert.NotNil(t, stopped.StoppedAt)
		assert.NotNil(t, stopped.FinalCapital)
		assert.Equal(t, 2500.0, *stopped.FinalCapital)
	})
}

// TestOrdersCRUDWithTestcontainers tests complete CRUD operations for orders
func TestOrdersCRUDWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create a session first
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	err = tc.DB.CreateSession(ctx, session)
	require.NoError(t, err)

	t.Run("Insert", func(t *testing.T) {
		order := &db.Order{
			ID:           uuid.New(),
			SessionID:    &session.ID,
			Symbol:       "BTC/USDT",
			Exchange:     "binance",
			Side:         db.OrderSideBuy,
			Type:         db.OrderTypeMarket,
			Quantity:     0.1,
			Status:       db.OrderStatusNew,
			PlacedAt:     time.Now(),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := tc.DB.InsertOrder(ctx, order)
		require.NoError(t, err)
	})

	t.Run("Read", func(t *testing.T) {
		// Create order
		orderID := uuid.New()
		price := 50000.0
		order := &db.Order{
			ID:           orderID,
			SessionID:    &session.ID,
			Symbol:       "BTC/USDT",
			Exchange:     "binance",
			Side:         db.OrderSideSell,
			Type:         db.OrderTypeLimit,
			Quantity:     0.2,
			Price:        &price,
			Status:       db.OrderStatusNew,
			PlacedAt:     time.Now(),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := tc.DB.InsertOrder(ctx, order)
		require.NoError(t, err)

		// Read it back
		retrieved, err := tc.DB.GetOrder(ctx, orderID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, order.ID, retrieved.ID)
		assert.Equal(t, order.Symbol, retrieved.Symbol)
		assert.Equal(t, order.Side, retrieved.Side)
		assert.Equal(t, order.Type, retrieved.Type)
		assert.Equal(t, order.Quantity, retrieved.Quantity)
	})

	t.Run("Update", func(t *testing.T) {
		// Create order
		orderID := uuid.New()
		order := &db.Order{
			ID:           orderID,
			SessionID:    &session.ID,
			Symbol:       "BTC/USDT",
			Exchange:     "binance",
			Side:         db.OrderSideBuy,
			Type:         db.OrderTypeMarket,
			Quantity:     0.15,
			Status:       db.OrderStatusNew,
			PlacedAt:     time.Now(),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := tc.DB.InsertOrder(ctx, order)
		require.NoError(t, err)

		// Update to filled
		now := time.Now()
		err = tc.DB.UpdateOrderStatus(ctx, orderID, db.OrderStatusFilled, 0.15, 7200.0, &now, nil, nil)
		require.NoError(t, err)

		// Verify update
		updated, err := tc.DB.GetOrder(ctx, orderID)
		require.NoError(t, err)

		assert.Equal(t, db.OrderStatusFilled, updated.Status)
		assert.Equal(t, 0.15, updated.ExecutedQuantity)
		assert.Equal(t, 7200.0, updated.ExecutedQuoteQuantity)
		assert.NotNil(t, updated.FilledAt)
	})

	t.Run("List", func(t *testing.T) {
		// Create multiple orders
		for i := 0; i < 5; i++ {
			order := &db.Order{
				ID:           uuid.New(),
				SessionID:    &session.ID,
				Symbol:       "BTC/USDT",
				Exchange:     "binance",
				Side:         db.OrderSideBuy,
				Type:         db.OrderTypeMarket,
				Quantity:     0.1,
				Status:       db.OrderStatusNew,
				PlacedAt:     time.Now(),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			err := tc.DB.InsertOrder(ctx, order)
			require.NoError(t, err)
		}

		// List orders for session
		orders, err := tc.DB.GetOrdersBySession(ctx, session.ID)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(orders), 5)
	})
}

// TestTradesCRUDWithTestcontainers tests complete CRUD operations for trades
func TestTradesCRUDWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create session and order first
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	err = tc.DB.CreateSession(ctx, session)
	require.NoError(t, err)

	orderID := uuid.New()
	order := &db.Order{
		ID:           orderID,
		SessionID:    &session.ID,
		Symbol:       "BTC/USDT",
		Exchange:     "binance",
		Side:         db.OrderSideBuy,
		Type:         db.OrderTypeMarket,
		Quantity:     0.1,
		Status:       db.OrderStatusNew,
		PlacedAt:     time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = tc.DB.InsertOrder(ctx, order)
	require.NoError(t, err)

	t.Run("Insert", func(t *testing.T) {
		commissionAsset := "USDT"
		trade := &db.Trade{
			ID:              uuid.New(),
			OrderID:         orderID,
			Symbol:          "BTC/USDT",
			Exchange:        "binance",
			Side:            db.OrderSideBuy,
			Quantity:        0.1,
			Price:           48000.0,
			QuoteQuantity:   4800.0,
			Commission:      4.8,
			CommissionAsset: &commissionAsset,
			ExecutedAt:      time.Now(),
			IsMaker:         false,
			CreatedAt:       time.Now(),
		}

		err := tc.DB.InsertTrade(ctx, trade)
		require.NoError(t, err)
	})

	t.Run("List", func(t *testing.T) {
		// Create multiple trades
		for i := 0; i < 3; i++ {
			commissionAsset := "USDT"
			trade := &db.Trade{
				ID:              uuid.New(),
				OrderID:         orderID,
				Symbol:          "BTC/USDT",
				Exchange:        "binance",
				Side:            db.OrderSideBuy,
				Quantity:        0.01,
				Price:           48000.0 + float64(i*100),
				QuoteQuantity:   480.0 + float64(i),
				Commission:      0.48,
				CommissionAsset: &commissionAsset,
				ExecutedAt:      time.Now(),
				IsMaker:         false,
				CreatedAt:       time.Now(),
			}
			err := tc.DB.InsertTrade(ctx, trade)
			require.NoError(t, err)
		}

		// List trades for order
		trades, err := tc.DB.GetTradesByOrderID(ctx, orderID)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(trades), 3)
	})
}

// TestPositionsCRUDWithTestcontainers tests complete CRUD operations for positions
func TestPositionsCRUDWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create session first
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	err = tc.DB.CreateSession(ctx, session)
	require.NoError(t, err)

	t.Run("Create", func(t *testing.T) {
		position := &db.Position{
			SessionID:  &session.ID,
			Symbol:     "BTC/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   0.5,
			EntryPrice: 48000.0,
			EntryTime:  time.Now(),
		}

		err := tc.DB.CreatePosition(ctx, position)
		require.NoError(t, err)

		assert.NotEqual(t, uuid.Nil, position.ID)
		assert.NotZero(t, position.CreatedAt)
	})

	t.Run("Read", func(t *testing.T) {
		// Create position
		position := &db.Position{
			SessionID:  &session.ID,
			Symbol:     "ETH/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   5.0,
			EntryPrice: 3000.0,
			EntryTime:  time.Now(),
		}

		err := tc.DB.CreatePosition(ctx, position)
		require.NoError(t, err)

		// Read it back
		retrieved, err := tc.DB.GetPosition(ctx, position.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, position.ID, retrieved.ID)
		assert.Equal(t, position.Symbol, retrieved.Symbol)
		assert.Equal(t, position.Quantity, retrieved.Quantity)
		assert.Equal(t, position.EntryPrice, retrieved.EntryPrice)
	})

	t.Run("Update", func(t *testing.T) {
		// Create position
		position := &db.Position{
			SessionID:  &session.ID,
			Symbol:     "SOL/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   10.0,
			EntryPrice: 100.0,
			EntryTime:  time.Now(),
		}

		err := tc.DB.CreatePosition(ctx, position)
		require.NoError(t, err)

		// Update unrealized P&L
		err = tc.DB.UpdateUnrealizedPnL(ctx, position.ID, 110.0)
		require.NoError(t, err)

		// Verify update
		updated, err := tc.DB.GetPosition(ctx, position.ID)
		require.NoError(t, err)

		assert.NotNil(t, updated.UnrealizedPnL)
		assert.Equal(t, 100.0, *updated.UnrealizedPnL) // (110-100) * 10
	})

	t.Run("Close", func(t *testing.T) {
		// Create position
		position := &db.Position{
			SessionID:  &session.ID,
			Symbol:     "ADA/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			Quantity:   1000.0,
			EntryPrice: 0.5,
			EntryTime:  time.Now(),
		}

		err := tc.DB.CreatePosition(ctx, position)
		require.NoError(t, err)

		// Close it
		err = tc.DB.ClosePosition(ctx, position.ID, 0.6, "target_reached", 5.0)
		require.NoError(t, err)

		// Verify closed
		closed, err := tc.DB.GetPosition(ctx, position.ID)
		require.NoError(t, err)

		assert.NotNil(t, closed.ExitTime)
		assert.NotNil(t, closed.ExitPrice)
		assert.Equal(t, 0.6, *closed.ExitPrice)
		assert.NotNil(t, closed.RealizedPnL)
		// (0.6 - 0.5) * 1000 - 5 = 95
		assert.Equal(t, 95.0, *closed.RealizedPnL)
	})
}

// TestConcurrentOperationsWithTestcontainers tests thread-safety of database operations
func TestConcurrentOperationsWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create session
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	err = tc.DB.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create 50 orders concurrently
	done := make(chan bool, 50)
	errors := make(chan error, 50)

	for i := 0; i < 50; i++ {
		go func(idx int) {
			order := &db.Order{
				ID:           uuid.New(),
				SessionID:    &session.ID,
				Symbol:       "BTC/USDT",
				Exchange:     "binance",
				Side:         db.OrderSideBuy,
				Type:         db.OrderTypeMarket,
				Quantity:     0.1,
				Status:       db.OrderStatusNew,
				PlacedAt:     time.Now(),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}

			err := tc.DB.InsertOrder(ctx, order)
			if err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 50; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify all orders were created
	orders, err := tc.DB.GetOrdersBySession(ctx, session.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(orders), 50)
}
