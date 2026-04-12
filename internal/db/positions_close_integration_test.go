package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// TestClosePosition covers the single-UPDATE close added for PERF-003
// (#136). The happy path collapses to one round-trip; the error path
// does a targeted GetPosition lookup only when rowsAffected=0 to preserve
// the precise error messages. We verify:
//  1. Happy path closes a LONG position and computes realized_pnl
//     server-side via CASE on side.
//  2. Happy path closes a SHORT position (opposite sign).
//  3. Double-close attempts return "already closed" (not "not found").
//  4. Closing a non-existent ID returns "not found".
//  5. Fees are added to the existing fees field, not overwritten.
func TestClosePosition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if os.Getenv("SKIP_TESTCONTAINER_TESTS") == "true" {
		t.Skip("Skipping testcontainer test (SKIP_TESTCONTAINER_TESTS=true)")
	}

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	require.NoError(t, tc.DB.CreateSession(ctx, session))

	t.Run("close LONG computes realized_pnl from CASE", func(t *testing.T) {
		pos := &db.Position{
			ID:         uuid.New(),
			SessionID:  &session.ID,
			Symbol:     "BTC/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			EntryPrice: 50000.0,
			Quantity:   0.2,
			EntryTime:  time.Now(),
			Fees:       1.0,
		}
		require.NoError(t, tc.DB.CreatePosition(ctx, pos))

		// Exit at 51000 on a LONG, quantity 0.2, entry fees 1.0, close fees 0.5:
		// #218: realized_pnl = (51000 - 50000) * 0.2 - (1.0 + 0.5) = 198.5
		// (deducts the full accumulated fee total, not just the close fee)
		require.NoError(t, tc.DB.ClosePosition(ctx, pos.ID, 51000.0, "take_profit", 0.5))

		got, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)
		require.NotNil(t, got.ExitTime, "exit_time should be set")
		require.NotNil(t, got.RealizedPnL, "realized_pnl should be populated")
		assert.InDelta(t, 198.5, *got.RealizedPnL, 0.0001)
		assert.InDelta(t, 1.5, got.Fees, 0.0001, "fees should accumulate (1.0 + 0.5)")
		require.NotNil(t, got.ExitPrice)
		assert.InDelta(t, 51000.0, *got.ExitPrice, 0.0001)
		require.NotNil(t, got.ExitReason)
		assert.Equal(t, "take_profit", *got.ExitReason)
	})

	t.Run("close SHORT computes realized_pnl with inverted sign", func(t *testing.T) {
		pos := &db.Position{
			ID:         uuid.New(),
			SessionID:  &session.ID,
			Symbol:     "ETH/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideShort,
			EntryPrice: 3000.0,
			Quantity:   1.0,
			EntryTime:  time.Now(),
		}
		require.NoError(t, tc.DB.CreatePosition(ctx, pos))

		// Exit at 2900 on a SHORT, quantity 1.0, fees 0:
		// realized_pnl = (3000 - 2900) * 1.0 - 0 = 100.0
		require.NoError(t, tc.DB.ClosePosition(ctx, pos.ID, 2900.0, "stop_loss", 0.0))

		got, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)
		require.NotNil(t, got.RealizedPnL)
		assert.InDelta(t, 100.0, *got.RealizedPnL, 0.0001)
	})

	t.Run("double-close returns already closed", func(t *testing.T) {
		pos := &db.Position{
			ID:         uuid.New(),
			SessionID:  &session.ID,
			Symbol:     "SOL/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			EntryPrice: 100.0,
			Quantity:   10.0,
			EntryTime:  time.Now(),
		}
		require.NoError(t, tc.DB.CreatePosition(ctx, pos))
		require.NoError(t, tc.DB.ClosePosition(ctx, pos.ID, 110.0, "take_profit", 0.0))

		// Second close should fail with a precise "already closed" error.
		err := tc.DB.ClosePosition(ctx, pos.ID, 120.0, "take_profit", 0.0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already closed",
			"double-close should return 'already closed', not 'not found'")
	})

	t.Run("close non-existent returns not found", func(t *testing.T) {
		err := tc.DB.ClosePosition(ctx, uuid.New(), 100.0, "manual", 0.0)
		require.Error(t, err)
		assert.ErrorContains(t, err, "not found",
			"closing a non-existent position should surface a 'not found' error")
	})
}
