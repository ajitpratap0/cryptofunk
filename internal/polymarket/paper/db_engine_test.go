package paper

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// setupDB creates a testcontainer-backed *db.DB with all migrations applied.
// It skips the test if Docker is unavailable.
func setupDB(t *testing.T) *db.DB {
	t.Helper()

	// Skip if Docker is unavailable
	cmd := exec.CommandContext(context.Background(), "docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Skip("Docker not available, skipping DB integration test")
	}

	tc := testhelpers.SetupTestDatabase(t)

	// Apply all migrations (path relative to this file's location)
	err := tc.ApplyMigrations("../../../migrations")
	require.NoError(t, err, "migrations should apply cleanly")

	return tc.DB
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_NewEngine
// ---------------------------------------------------------------------------

func TestDBPaperEngine_NewEngine(t *testing.T) {
	database := setupDB(t)

	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err, "NewDBPaperEngine should succeed")

	// Initial balance must equal DefaultBalance
	assert.InDelta(t, DefaultBalance, engine.GetBalance(), 0.001)

	// Session must exist in DB with correct mode and exchange
	ctx := context.Background()
	session, err := database.GetSession(ctx, engine.sessionID)
	require.NoError(t, err, "session should be retrievable from DB")
	assert.Equal(t, db.TradingModePaper, session.Mode)
	assert.Equal(t, "polymarket", session.Exchange)
	assert.InDelta(t, DefaultBalance, session.InitialCapital, 0.001)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Buy_Basic
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Buy_Basic(t *testing.T) {
	database := setupDB(t)

	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	trade, err := engine.Buy("market-1", "Will X happen?", YES, 30.0, 0.6)
	require.NoError(t, err, "Buy should succeed")

	// Trade fields
	assert.Equal(t, "BUY", trade.Action)
	assert.InDelta(t, 30.0, trade.Amount, 0.001)
	assert.InDelta(t, 0.6, trade.Price, 0.0001)
	expectedShares := 30.0 / 0.6
	assert.InDelta(t, expectedShares, trade.Shares, 0.001)
	assert.Equal(t, "market-1", trade.MarketID)
	assert.Equal(t, YES, trade.Side)

	// Balance reduced
	assert.InDelta(t, DefaultBalance-30.0, engine.GetBalance(), 0.001)

	// Position in DB
	ctx := context.Background()
	positions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	pos := positions[0]
	assert.Equal(t, "market-1", pos.MarketID)
	assert.Equal(t, "YES", pos.Side)
	assert.Equal(t, "OPEN", pos.Status)
	assert.InDelta(t, expectedShares, pos.Shares, 0.001)
	assert.InDelta(t, 30.0, pos.CostBasis, 0.001)
	assert.InDelta(t, 0.6, pos.AvgPrice, 0.0001)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Buy_Validations
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Buy_Validations(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	tests := []struct {
		name        string
		amount      float64
		price       float64
		errContains string
	}{
		{"zero amount", 0, 0.5, "amount must be positive"},
		{"negative amount", -5, 0.5, "amount must be positive"},
		{"price zero", 10, 0, "price must be between 0 and 1"},
		{"price one", 10, 1.0, "price must be between 0 and 1"},
		{"price above one", 10, 1.5, "price must be between 0 and 1"},
		{"insufficient balance", 200, 0.5, "insufficient balance"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine.Buy("market-x", "Q", YES, tc.amount, tc.price)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}

	// Balance should be unchanged after all failed attempts
	assert.InDelta(t, DefaultBalance, engine.GetBalance(), 0.001)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Buy_AverageIn
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Buy_AverageIn(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	// First buy: $20 at price 0.60 → 33.33 shares
	trade1, err := engine.Buy("market-1", "Q?", YES, 20.0, 0.6)
	require.NoError(t, err)
	shares1 := trade1.Shares // 33.33...

	// Second buy same market: $20 at price 0.40 → 50 shares
	trade2, err := engine.Buy("market-1", "Q?", YES, 20.0, 0.4)
	require.NoError(t, err)
	shares2 := trade2.Shares // 50

	// Balance: 100 - 20 - 20 = 60
	assert.InDelta(t, 60.0, engine.GetBalance(), 0.001)

	// DB should have exactly 1 open position (averaged in)
	ctx := context.Background()
	positions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, positions, 1, "should still be one position after averaging in")

	pos := positions[0]
	totalShares := shares1 + shares2
	assert.InDelta(t, totalShares, pos.Shares, 0.001)
	assert.InDelta(t, 40.0, pos.CostBasis, 0.001) // $20 + $20

	// Average price = total cost / total shares = 40 / totalShares
	expectedAvgPrice := 40.0 / totalShares
	assert.InDelta(t, expectedAvgPrice, pos.AvgPrice, 0.001)

	// Trades table should have 2 BUY records
	trades, err := database.GetPolymarketTrades(ctx, 100)
	require.NoError(t, err)
	assert.Len(t, trades, 2)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Sell_Basic
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Sell_Basic(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	// Buy $50 at 0.5 → 100 shares, balance = 50
	_, err = engine.Buy("market-1", "Q?", YES, 50.0, 0.5)
	require.NoError(t, err)
	assert.InDelta(t, 50.0, engine.GetBalance(), 0.001)

	// Sell 50 shares at price 0.8 → proceeds = 40
	sellTrade, err := engine.Sell("market-1", YES, 50.0, 0.8)
	require.NoError(t, err)

	assert.Equal(t, "SELL", sellTrade.Action)
	assert.InDelta(t, 40.0, sellTrade.Amount, 0.001) // 50 * 0.8
	assert.InDelta(t, 0.8, sellTrade.Price, 0.0001)
	assert.InDelta(t, 50.0, sellTrade.Shares, 0.001)

	// Balance: 50 + 40 = 90
	assert.InDelta(t, 90.0, engine.GetBalance(), 0.001)

	// Position still open with 50 shares remaining
	ctx := context.Background()
	positions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, positions, 1, "position should remain open")
	assert.InDelta(t, 50.0, positions[0].Shares, 0.001)
	assert.Equal(t, "OPEN", positions[0].Status)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Sell_Closes_Position
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Sell_Closes_Position(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	// Buy $50 at 0.5 → 100 shares
	_, err = engine.Buy("market-1", "Q?", YES, 50.0, 0.5)
	require.NoError(t, err)

	ctx := context.Background()
	positionsBefore, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, positionsBefore, 1)
	posID := positionsBefore[0].ID
	allShares := positionsBefore[0].Shares // should be 100

	// Sell all 100 shares at 0.8
	sellTrade, err := engine.Sell("market-1", YES, allShares, 0.8)
	require.NoError(t, err)
	assert.Equal(t, "SELL", sellTrade.Action)

	// Expected P&L = proceeds - cost = (100 * 0.8) - 50 = 30
	expectedPnL := allShares*0.8 - 50.0

	// No open positions
	openPositions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	assert.Empty(t, openPositions, "position should be closed after selling all shares")

	// Closed position has realized P&L
	closedPos, err := database.GetPolymarketPosition(ctx, posID)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", closedPos.Status)
	require.NotNil(t, closedPos.RealizedPnl)
	assert.InDelta(t, expectedPnL, *closedPos.RealizedPnl, 0.01)

	// Balance: 50 (remaining after buy) + 80 (sell proceeds) = 130
	expectedBalance := 50.0 + allShares*0.8
	assert.InDelta(t, expectedBalance, engine.GetBalance(), 0.001)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Sell_Errors
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Sell_Errors(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	t.Run("sell nonexistent position", func(t *testing.T) {
		_, err := engine.Sell("no-such-market", YES, 10.0, 0.5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no position")
	})

	t.Run("sell more shares than held", func(t *testing.T) {
		// Buy 20 shares ($10 at 0.5)
		_, err := engine.Buy("market-sell-err", "Q?", YES, 10.0, 0.5)
		require.NoError(t, err)

		// Try to sell 100 shares (have only ~20)
		_, err = engine.Sell("market-sell-err", YES, 100.0, 0.5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient shares")
	})
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_GetPortfolio
// ---------------------------------------------------------------------------

func TestDBPaperEngine_GetPortfolio(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	// Buy two different markets
	_, err = engine.Buy("market-A", "Q A?", YES, 20.0, 0.5)
	require.NoError(t, err)
	_, err = engine.Buy("market-B", "Q B?", NO, 15.0, 0.3)
	require.NoError(t, err)

	portfolio := engine.GetPortfolio()

	// Balance: 100 - 20 - 15 = 65
	assert.InDelta(t, 65.0, portfolio.Balance, 0.001)

	// Two positions
	assert.Len(t, portfolio.Positions, 2)
	assert.Contains(t, portfolio.Positions, "market-A:YES")
	assert.Contains(t, portfolio.Positions, "market-B:NO")

	// Two trades
	assert.Len(t, portfolio.Trades, 2)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_Reset
// ---------------------------------------------------------------------------

func TestDBPaperEngine_Reset(t *testing.T) {
	database := setupDB(t)
	engine, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	// Buy a position to reduce balance
	_, err = engine.Buy("market-1", "Q?", YES, 40.0, 0.5)
	require.NoError(t, err)
	assert.InDelta(t, 60.0, engine.GetBalance(), 0.001)

	oldSessionID := engine.sessionID

	// Reset the engine
	require.NoError(t, engine.Reset())

	// Balance back to default
	assert.InDelta(t, DefaultBalance, engine.GetBalance(), 0.001)

	// No open positions in DB
	ctx := context.Background()
	openPositions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	assert.Empty(t, openPositions, "all positions should be closed after reset")

	// New session created
	assert.NotEqual(t, oldSessionID, engine.sessionID, "reset should create a new session")

	// New session retrievable from DB
	newSession, err := database.GetSession(ctx, engine.sessionID)
	require.NoError(t, err)
	assert.Equal(t, db.TradingModePaper, newSession.Mode)
	assert.InDelta(t, DefaultBalance, newSession.InitialCapital, 0.001)
}

// ---------------------------------------------------------------------------
// TestDBPaperEngine_WithSession
// ---------------------------------------------------------------------------

func TestDBPaperEngine_WithSession(t *testing.T) {
	database := setupDB(t)

	// Create engine1 and buy a position
	engine1, err := NewDBPaperEngine(database)
	require.NoError(t, err)

	_, err = engine1.Buy("market-1", "Q?", YES, 30.0, 0.6)
	require.NoError(t, err)

	sessionID := engine1.sessionID
	balanceAfterBuy := engine1.GetBalance() // should be 70

	// Create engine2 reusing the same session
	engine2, err := NewDBPaperEngineWithSession(database, sessionID)
	require.NoError(t, err)

	// engine2 should reflect the reduced balance (cost basis deducted)
	// balance = InitialCapital (100) - TotalCostBasis (30) = 70
	assert.InDelta(t, balanceAfterBuy, engine2.GetBalance(), 0.001,
		"engine2 reusing the session should see the same reduced balance")
}
