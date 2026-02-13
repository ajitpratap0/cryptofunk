package paper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "portfolio.json")
	e := &Engine{
		FilePath: tmp,
		Portfolio: &Portfolio{
			Balance:   100.0,
			Positions: make(map[string]*Position),
			NextID:    1,
		},
	}
	return e
}

func TestBuy(t *testing.T) {
	e := testEngine(t)

	trade, err := e.Buy("market1", "Will X happen?", YES, 10, 0.60)
	require.NoError(t, err)
	assert.Equal(t, "BUY", trade.Action)
	assert.InDelta(t, 16.666, trade.Shares, 0.01)
	assert.InDelta(t, 90.0, e.Portfolio.Balance, 0.01)

	pos := e.Portfolio.Positions["market1:YES"]
	require.NotNil(t, pos)
	assert.InDelta(t, 16.666, pos.Shares, 0.01)
	assert.InDelta(t, 0.60, pos.AvgPrice, 0.01)
}

func TestBuyInsufficientBalance(t *testing.T) {
	e := testEngine(t)
	_, err := e.Buy("m1", "Q", YES, 200, 0.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
}

func TestSell(t *testing.T) {
	e := testEngine(t)

	_, err := e.Buy("m1", "Q", YES, 10, 0.50)
	require.NoError(t, err)
	// Bought 20 shares at 0.50

	trade, err := e.Sell("m1", YES, 10, 0.70)
	require.NoError(t, err)
	assert.Equal(t, "SELL", trade.Action)
	assert.InDelta(t, 7.0, trade.Amount, 0.01)

	// Balance: 90 + 7 = 97
	assert.InDelta(t, 97.0, e.Portfolio.Balance, 0.01)

	// Still have 10 shares
	pos := e.Portfolio.Positions["m1:YES"]
	require.NotNil(t, pos)
	assert.InDelta(t, 10.0, pos.Shares, 0.01)
}

func TestSellAll(t *testing.T) {
	e := testEngine(t)

	_, err := e.Buy("m1", "Q", YES, 10, 0.50)
	require.NoError(t, err)

	_, err = e.Sell("m1", YES, 20, 0.80)
	require.NoError(t, err)

	// Position should be removed
	_, exists := e.Portfolio.Positions["m1:YES"]
	assert.False(t, exists)
}

func TestSellNoPosition(t *testing.T) {
	e := testEngine(t)
	_, err := e.Sell("m1", YES, 10, 0.5)
	assert.Error(t, err)
}

func TestPnL(t *testing.T) {
	p := &Position{Shares: 20, CostBasis: 10, AvgPrice: 0.50}
	assert.InDelta(t, 4.0, p.PnL(0.70), 0.01) // 20*0.70 - 10 = 4
	assert.InDelta(t, -4.0, p.PnL(0.30), 0.01)
}

func TestReset(t *testing.T) {
	e := testEngine(t)
	e.Buy("m1", "Q", YES, 50, 0.5)
	require.NoError(t, e.Reset())
	assert.InDelta(t, 100.0, e.Portfolio.Balance, 0.01)
	assert.Empty(t, e.Portfolio.Positions)
	assert.Empty(t, e.Portfolio.Trades)
}

func TestSaveLoad(t *testing.T) {
	e := testEngine(t)
	e.Buy("m1", "Q", NO, 20, 0.40)
	require.NoError(t, e.Save())

	e2 := &Engine{FilePath: e.FilePath}
	require.NoError(t, e2.Load())
	assert.InDelta(t, 80.0, e2.Portfolio.Balance, 0.01)
	assert.Len(t, e2.Portfolio.Positions, 1)
}

func TestLoadNonExistent(t *testing.T) {
	e := &Engine{FilePath: filepath.Join(os.TempDir(), "nonexistent_test.json")}
	err := e.Load()
	assert.Error(t, err)
}
