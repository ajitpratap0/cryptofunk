package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/paper"
)

// ---------------------------------------------------------------------------
// TestPolymarketPaperSuite_MultiplePositions
//
// Verifies that a single engine session can hold multiple open positions
// across different markets and different sides of the same market, and that
// the portfolio balance reflects all deductions correctly.
// ---------------------------------------------------------------------------

func TestPolymarketPaperSuite_MultiplePositions(t *testing.T) {
	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	database := tc.DB
	ctx := context.Background()

	engine, err := paper.NewDBPaperEngine(database)
	require.NoError(t, err)

	// Seed markets so FK constraints are satisfied (db_engine upserts on Buy,
	// so explicit seeding is optional — but doing it here makes the test
	// self-documenting).
	for _, m := range []db.PolymarketMarket{
		{ID: "market-A", Question: "Market A question?", Active: true},
		{ID: "market-B", Question: "Market B question?", Active: true},
	} {
		require.NoError(t, database.UpsertPolymarketMarket(ctx, &m))
	}

	// Buy YES on market-A at 0.6, $20
	tradeA1, err := engine.Buy("market-A", "Market A question?", paper.YES, 20.0, 0.6)
	require.NoError(t, err, "buy YES on market-A")
	assert.Equal(t, "BUY", tradeA1.Action)

	// Buy NO on market-A at 0.4, $15 — same market, different side
	tradeA2, err := engine.Buy("market-A", "Market A question?", paper.NO, 15.0, 0.4)
	require.NoError(t, err, "buy NO on market-A")
	assert.Equal(t, "BUY", tradeA2.Action)

	// Buy YES on market-B at 0.7, $10
	tradeB, err := engine.Buy("market-B", "Market B question?", paper.YES, 10.0, 0.7)
	require.NoError(t, err, "buy YES on market-B")
	assert.Equal(t, "BUY", tradeB.Action)

	// GetPortfolio — 3 open positions
	portfolio := engine.GetPortfolio()

	// Balance: 100 - 20 - 15 - 10 = 55
	assert.InDelta(t, 55.0, portfolio.Balance, 0.01,
		"balance should be 100-20-15-10 = 55 after three buys")

	assert.Len(t, portfolio.Positions, 3,
		"portfolio should have exactly 3 open positions")

	// Verify each position key exists
	assert.Contains(t, portfolio.Positions, "market-A:YES", "missing market-A:YES position")
	assert.Contains(t, portfolio.Positions, "market-A:NO", "missing market-A:NO position")
	assert.Contains(t, portfolio.Positions, "market-B:YES", "missing market-B:YES position")

	// Verify shares for each position
	posAYES := portfolio.Positions["market-A:YES"]
	require.NotNil(t, posAYES)
	assert.InDelta(t, 20.0/0.6, posAYES.Shares, 0.01)
	assert.InDelta(t, 20.0, posAYES.CostBasis, 0.01)

	posANO := portfolio.Positions["market-A:NO"]
	require.NotNil(t, posANO)
	assert.InDelta(t, 15.0/0.4, posANO.Shares, 0.01)
	assert.InDelta(t, 15.0, posANO.CostBasis, 0.01)

	posBYES := portfolio.Positions["market-B:YES"]
	require.NotNil(t, posBYES)
	assert.InDelta(t, 10.0/0.7, posBYES.Shares, 0.01)
	assert.InDelta(t, 10.0, posBYES.CostBasis, 0.01)

	// Also verify via DB directly
	dbPositions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	assert.Len(t, dbPositions, 3, "DB should reflect 3 open positions")
}

// ---------------------------------------------------------------------------
// TestPolymarketPaperSuite_PnL_Calculation
//
// Validates that partial sells produce correct realized P&L and balance updates.
// Buy 100 shares at 0.3 ($30), sell 50 at 0.7 (proceeds = $35).
// Expected balance after: (100-30) + 35 = $105.
// ---------------------------------------------------------------------------

func TestPolymarketPaperSuite_PnL_Calculation(t *testing.T) {
	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	database := tc.DB
	ctx := context.Background()

	engine, err := paper.NewDBPaperEngine(database)
	require.NoError(t, err)

	// Buy YES on market-X at 0.3 with $30 → 100 shares
	buyTrade, err := engine.Buy("market-X", "Will X happen?", paper.YES, 30.0, 0.3)
	require.NoError(t, err)
	require.InDelta(t, 100.0, buyTrade.Shares, 0.01, "should have 100 shares after buy")

	// Balance after buy: 100 - 30 = 70
	assert.InDelta(t, 70.0, engine.GetBalance(), 0.01)

	// Sell 50 shares at 0.7 → proceeds = 50 * 0.7 = $35
	sellTrade, err := engine.Sell("market-X", paper.YES, 50.0, 0.7)
	require.NoError(t, err)
	assert.Equal(t, "SELL", sellTrade.Action)
	assert.InDelta(t, 35.0, sellTrade.Amount, 0.01, "sell proceeds should be 50*0.7=35")

	// Balance after sell: 70 + 35 = 105
	assert.InDelta(t, 105.0, engine.GetBalance(), 0.01,
		"balance should be 70 + sell_proceeds(35) = 105")

	// Portfolio should still have ~50 shares remaining
	portfolio := engine.GetPortfolio()
	require.Len(t, portfolio.Positions, 1, "one position should still be open")
	remaining := portfolio.Positions["market-X:YES"]
	require.NotNil(t, remaining)
	assert.InDelta(t, 50.0, remaining.Shares, 0.01, "should have ~50 shares remaining")

	// Realized P&L: we sold at 0.7 vs bought at 0.3 — so this partial sell was profitable.
	// sell proceeds (35) > proportional cost basis (50/100 * 30 = 15) → positive PnL = 20.
	// Verify via DB: query trades to confirm both BUY and SELL are recorded with correct Action.
	trades, err := database.GetPolymarketTrades(ctx, 100)
	require.NoError(t, err)
	require.Len(t, trades, 2, "should have BUY + SELL trade")

	// GetPolymarketTrades returns newest-first
	actions := map[string]bool{}
	for _, tr := range trades {
		actions[tr.Action] = true
		if tr.Action == "SELL" {
			assert.InDelta(t, 35.0, tr.Amount, 0.01, "SELL trade amount should be 35")
			assert.InDelta(t, 0.7, tr.Price, 0.001, "SELL trade price should be 0.7")
		}
		if tr.Action == "BUY" {
			assert.InDelta(t, 30.0, tr.Amount, 0.01, "BUY trade amount should be 30")
			assert.InDelta(t, 0.3, tr.Price, 0.001, "BUY trade price should be 0.3")
		}
	}
	assert.True(t, actions["BUY"], "BUY trade should be recorded")
	assert.True(t, actions["SELL"], "SELL trade should be recorded")
}

// ---------------------------------------------------------------------------
// TestPolymarketPaperSuite_SessionIsolation
//
// Two independent DBPaperEngine instances (sessions A and B) should each see
// only their own positions. Session B created after session A's buy should
// start with full DefaultBalance and empty positions.
// ---------------------------------------------------------------------------

func TestPolymarketPaperSuite_SessionIsolation(t *testing.T) {
	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	database := tc.DB

	// Engine 1: session A — make a purchase
	engine1, err := paper.NewDBPaperEngine(database)
	require.NoError(t, err)

	_, err = engine1.Buy("iso-market", "Isolation test market?", paper.YES, 40.0, 0.5)
	require.NoError(t, err)
	assert.InDelta(t, 60.0, engine1.GetBalance(), 0.01, "engine1 balance after buy")

	// Engine 2: session B — brand new session, no purchases
	engine2, err := paper.NewDBPaperEngine(database)
	require.NoError(t, err)

	// engine2 should start at DefaultBalance
	assert.InDelta(t, paper.DefaultBalance, engine2.GetBalance(), 0.01,
		"engine2 should start with full DefaultBalance")

	// engine2.GetPortfolio() should have no positions for its own session
	portfolio2 := engine2.GetPortfolio()
	assert.InDelta(t, paper.DefaultBalance, portfolio2.Balance, 0.01,
		"engine2 portfolio balance should be DefaultBalance")

	// engine2 should not see engine1's position in its own portfolio
	// (GetPortfolio only returns positions belonging to the engine's session)
	assert.NotContains(t, portfolio2.Positions, "iso-market:YES",
		"engine2 should not see engine1's position")

	// engine1's portfolio should still be intact
	portfolio1 := engine1.GetPortfolio()
	assert.InDelta(t, 60.0, portfolio1.Balance, 0.01, "engine1 balance unchanged")
	assert.Contains(t, portfolio1.Positions, "iso-market:YES",
		"engine1 should still have its position")
}

// ---------------------------------------------------------------------------
// TestPolymarketPaperSuite_DBConsistency
//
// Verifies that:
//   - A buy creates a polymarket_positions row with status="OPEN"
//   - Selling all shares sets status="CLOSED" with a realized_pnl value
//   - Reset closes all positions and creates a new session
//   - Trade records carry the correct Action field ("BUY" / "SELL")
// ---------------------------------------------------------------------------

func TestPolymarketPaperSuite_DBConsistency(t *testing.T) {
	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	database := tc.DB
	ctx := context.Background()

	engine, err := paper.NewDBPaperEngine(database)
	require.NoError(t, err)

	// --- Step 1: BUY ---
	_, err = engine.Buy("consistency-market", "Consistency test?", paper.YES, 50.0, 0.5)
	require.NoError(t, err)

	// Query DB directly to verify position row
	openPositions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, openPositions, 1, "DB should have exactly 1 open position after buy")

	pos := openPositions[0]
	assert.Equal(t, "consistency-market", pos.MarketID)
	assert.Equal(t, "YES", pos.Side)
	assert.Equal(t, "OPEN", pos.Status)
	assert.InDelta(t, 100.0, pos.Shares, 0.01, "should have 100 shares (50/0.5)")
	assert.Nil(t, pos.RealizedPnl, "realized PnL should be nil for open position")

	// Verify BUY trade record
	tradesAfterBuy, err := database.GetPolymarketTrades(ctx, 100)
	require.NoError(t, err)
	require.Len(t, tradesAfterBuy, 1, "should have 1 trade after buy")
	assert.Equal(t, "BUY", tradesAfterBuy[0].Action, "trade action should be BUY")
	assert.InDelta(t, 50.0, tradesAfterBuy[0].Amount, 0.01)
	assert.InDelta(t, 0.5, tradesAfterBuy[0].Price, 0.001)

	// --- Step 2: SELL ALL shares ---
	sellShares := pos.Shares
	_, err = engine.Sell("consistency-market", paper.YES, sellShares, 0.8)
	require.NoError(t, err)

	// Position should now be CLOSED in DB
	openAfterSell, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	assert.Len(t, openAfterSell, 0, "no open positions after selling all shares")

	// Fetch the closed position and verify PnL
	closedPos, err := database.GetPolymarketPosition(ctx, pos.ID)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", closedPos.Status)
	require.NotNil(t, closedPos.RealizedPnl, "realized_pnl should be set after close")
	// PnL = sell_proceeds - cost_basis = (100 * 0.8) - 50 = 30
	assert.InDelta(t, 30.0, *closedPos.RealizedPnl, 0.01,
		"realized PnL should be 30 (100 shares * 0.8 price - $50 cost)")

	// Verify SELL trade record
	tradesAfterSell, err := database.GetPolymarketTrades(ctx, 100)
	require.NoError(t, err)
	require.Len(t, tradesAfterSell, 2, "should have BUY + SELL trades")

	// GetPolymarketTrades returns DESC by timestamp — most recent first
	foundBuy, foundSell := false, false
	for _, tr := range tradesAfterSell {
		switch tr.Action {
		case "BUY":
			foundBuy = true
			assert.InDelta(t, 50.0, tr.Amount, 0.01)
		case "SELL":
			foundSell = true
			assert.InDelta(t, sellShares*0.8, tr.Amount, 0.01)
		}
	}
	assert.True(t, foundBuy, "BUY trade record must exist")
	assert.True(t, foundSell, "SELL trade record must exist")

	// --- Step 3: Reset ---
	// First open another position so Reset has something to close
	_, err = engine.Buy("reset-market", "Will reset happen?", paper.NO, 20.0, 0.4)
	require.NoError(t, err)

	openBeforeReset, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, openBeforeReset, 1, "should have 1 open position before reset")

	err = engine.Reset()
	require.NoError(t, err)

	// After reset: no open positions, balance restored
	openAfterReset, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	assert.Len(t, openAfterReset, 0, "Reset should close all open positions")

	assert.InDelta(t, paper.DefaultBalance, engine.GetBalance(), 0.01,
		"Reset should restore DefaultBalance")

	// The position that existed before reset should now be CLOSED
	closedAfterReset, err := database.GetPolymarketPosition(ctx, openBeforeReset[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", closedAfterReset.Status,
		"Reset should have closed the previously open position")
}
