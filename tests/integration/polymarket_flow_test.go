package integration

import (
	"context"
	"testing"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/paper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPolymarketFullFlow validates the complete Polymarket paper trading flow:
// insert market → buy → verify position & trade → sell → verify close & P&L.
func TestPolymarketFullFlow(t *testing.T) {
	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	defer tc.Cleanup()

	database := tc.DB
	ctx := context.Background()

	// Step 1: Insert a market into polymarket_markets
	market := &db.PolymarketMarket{
		ID:       "test-condition-id-123",
		Question: "Will BTC exceed $100k by March 2026?",
		Active:   true,
		YesPrice: 0.65,
		NoPrice:  0.35,
		Volume:   500000,
	}
	err := database.UpsertPolymarketMarket(ctx, market)
	require.NoError(t, err, "should insert market")

	// Verify market was stored
	fetched, err := database.GetPolymarketMarket(ctx, market.ID)
	require.NoError(t, err)
	assert.Equal(t, market.Question, fetched.Question)
	assert.Equal(t, 0.65, fetched.YesPrice)

	// Step 2: Create a DBPaperEngine and execute a paper buy
	engine, err := paper.NewDBPaperEngine(database)
	require.NoError(t, err, "should create DB paper engine")

	buyTrade, err := engine.Buy(market.ID, market.Question, paper.YES, 100.0, 0.65)
	require.NoError(t, err, "should execute paper buy")
	assert.Equal(t, "BUY", buyTrade.Action)
	assert.Equal(t, 100.0, buyTrade.Amount)
	assert.InDelta(t, 100.0/0.65, buyTrade.Shares, 0.01)

	// Step 3: Verify position appears in polymarket_positions
	positions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	require.Len(t, positions, 1, "should have exactly one open position")
	assert.Equal(t, market.ID, positions[0].MarketID)
	assert.Equal(t, "YES", positions[0].Side)
	assert.Equal(t, "OPEN", positions[0].Status)
	assert.InDelta(t, 100.0/0.65, positions[0].Shares, 0.01)

	// Step 4: Verify trade recorded in polymarket_trades
	trades, err := database.GetPolymarketTrades(ctx, 100)
	require.NoError(t, err)
	require.Len(t, trades, 1, "should have exactly one trade")
	assert.Equal(t, "BUY", trades[0].Action)
	assert.Equal(t, 0.65, trades[0].Price)

	// Step 5: Check portfolio summary
	summary, err := database.GetPolymarketPortfolioSummary(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.PositionCount)
	assert.InDelta(t, 100.0, summary.TotalCostBasis, 0.01)

	portfolio := engine.GetPortfolio()
	assert.InDelta(t, 900.0, portfolio.Balance, 0.01) // 1000 - 100

	// Step 6: Execute a sell (sell all shares at a higher price)
	sellPrice := 0.80
	sellShares := positions[0].Shares
	sellTrade, err := engine.Sell(market.ID, paper.YES, sellShares, sellPrice)
	require.NoError(t, err, "should execute paper sell")
	assert.Equal(t, "SELL", sellTrade.Action)

	// Step 7: Verify position closed and P&L calculated
	openPositions, err := database.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	assert.Len(t, openPositions, 0, "position should be closed")

	// Verify the closed position has realized PnL
	closedPos, err := database.GetPolymarketPosition(ctx, positions[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", closedPos.Status)
	assert.NotNil(t, closedPos.RealizedPnl)

	// P&L = sell proceeds - cost basis = (shares * 0.80) - 100
	expectedPnl := sellShares*sellPrice - 100.0
	assert.InDelta(t, expectedPnl, *closedPos.RealizedPnl, 0.01)

	// Verify balance updated correctly
	expectedBalance := 900.0 + sellShares*sellPrice
	assert.InDelta(t, expectedBalance, engine.GetBalance(), 0.01)

	// Verify we now have 2 trades total (buy + sell)
	allTrades, err := database.GetPolymarketTrades(ctx, 100)
	require.NoError(t, err)
	assert.Len(t, allTrades, 2)
}
