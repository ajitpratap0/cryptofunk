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

// TestBulkUpdateUnrealizedPnL covers the hot-path bulk updater added for
// PERF-002 (#135). We verify:
//  1. An empty slice is a no-op and returns nil without hitting the DB.
//  2. Multiple open positions are updated in a single round-trip and the
//     post-update values match what we passed in.
//  3. Closed positions (exit_time IS NOT NULL) are skipped — their
//     unrealized_pnl is not mutated by the bulk update.
//  4. Unknown IDs in the updates slice are silently skipped without error.
func TestBulkUpdateUnrealizedPnL(t *testing.T) {
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

	// Create test session
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}
	require.NoError(t, tc.DB.CreateSession(ctx, session))

	t.Run("empty slice is a no-op", func(t *testing.T) {
		err := tc.DB.BulkUpdateUnrealizedPnL(ctx, nil)
		assert.NoError(t, err)
		err = tc.DB.BulkUpdateUnrealizedPnL(ctx, []db.UnrealizedPnLUpdate{})
		assert.NoError(t, err)
	})

	t.Run("updates multiple open positions in one round-trip", func(t *testing.T) {
		// Create three open positions on DISTINCT symbols. The unique
		// partial index `idx_positions_open_session_symbol_uniq`
		// (migration 019) forbids two OPEN rows with the same
		// (session_id, symbol) — a single trader can hold at most one
		// open position per pair per session — so the previous
		// hardcoded "BTC/USDT" loop violated the constraint on the
		// second insert. Three different symbols still exercise the
		// bulk update path while satisfying the uniqueness invariant.
		symbols := []string{"BTC/USDT", "ETH/USDT", "SOL/USDT"}
		positions := make([]*db.Position, len(symbols))
		for i, sym := range symbols {
			positions[i] = &db.Position{
				ID:         uuid.New(),
				SessionID:  &session.ID,
				Symbol:     sym,
				Exchange:   "binance",
				Side:       db.PositionSideLong,
				EntryPrice: 50000.0,
				Quantity:   0.1 * float64(i+1),
				EntryTime:  time.Now(),
			}
			require.NoError(t, tc.DB.CreatePosition(ctx, positions[i]))
		}

		updates := []db.UnrealizedPnLUpdate{
			{ID: positions[0].ID, UnrealizedPnL: 100.0},
			{ID: positions[1].ID, UnrealizedPnL: 250.5},
			{ID: positions[2].ID, UnrealizedPnL: -75.25},
		}
		require.NoError(t, tc.DB.BulkUpdateUnrealizedPnL(ctx, updates))

		for _, want := range updates {
			got, err := tc.DB.GetPosition(ctx, want.ID)
			require.NoError(t, err)
			require.NotNil(t, got.UnrealizedPnL, "unrealized_pnl should be populated")
			assert.InDelta(t, want.UnrealizedPnL, *got.UnrealizedPnL, 0.0001,
				"position %s unrealized_pnl should match bulk-updated value", want.ID)
		}
	})

	t.Run("closed positions are skipped", func(t *testing.T) {
		// Open + close a position with unrealized_pnl=0 (from ClosePosition).
		// Use AVAX/USDT to avoid colliding with the ETH/USDT open
		// position left by the "updates multiple..." subtest above.
		pos := &db.Position{
			ID:         uuid.New(),
			SessionID:  &session.ID,
			Symbol:     "AVAX/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			EntryPrice: 3000.0,
			Quantity:   1.0,
			EntryTime:  time.Now(),
		}
		require.NoError(t, tc.DB.CreatePosition(ctx, pos))
		require.NoError(t, tc.DB.ClosePosition(ctx, pos.ID, 3100.0, "take_profit", 0.1))

		// Attempt to bulk-update the closed position — should be skipped
		// silently by the WHERE exit_time IS NULL filter.
		updates := []db.UnrealizedPnLUpdate{
			{ID: pos.ID, UnrealizedPnL: 999.0},
		}
		require.NoError(t, tc.DB.BulkUpdateUnrealizedPnL(ctx, updates))

		got, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)
		require.NotNil(t, got.UnrealizedPnL)
		assert.InDelta(t, 0.0, *got.UnrealizedPnL, 0.0001,
			"closed position's unrealized_pnl should remain 0, not the bulk-update value")
	})

	t.Run("unknown IDs are silently skipped", func(t *testing.T) {
		// Mix one real open position with an ID that doesn't exist in
		// the DB. Use DOT/USDT to avoid colliding with SOL/USDT left
		// open by the "updates multiple..." subtest above.
		pos := &db.Position{
			ID:         uuid.New(),
			SessionID:  &session.ID,
			Symbol:     "DOT/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideLong,
			EntryPrice: 100.0,
			Quantity:   5.0,
			EntryTime:  time.Now(),
		}
		require.NoError(t, tc.DB.CreatePosition(ctx, pos))

		updates := []db.UnrealizedPnLUpdate{
			{ID: pos.ID, UnrealizedPnL: 42.0},
			{ID: uuid.New(), UnrealizedPnL: 999.0}, // not in DB
		}
		err := tc.DB.BulkUpdateUnrealizedPnL(ctx, updates)
		assert.NoError(t, err, "unknown IDs should not produce an error")

		got, err := tc.DB.GetPosition(ctx, pos.ID)
		require.NoError(t, err)
		require.NotNil(t, got.UnrealizedPnL)
		assert.InDelta(t, 42.0, *got.UnrealizedPnL, 0.0001,
			"known position should still be updated even when mixed with unknown IDs")
	})
}
