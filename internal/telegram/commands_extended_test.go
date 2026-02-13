package telegram

import (
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRecentTrades(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "exit_price", "quantity",
		"pnl", "pnl_percent", "opened_at", "closed_at", "session_id",
	}).
		AddRow("BTCUSDT", "LONG", 50000.0, 51000.0, 0.1, 100.0, 2.0, now.Add(-1*time.Hour), now, "sess-1").
		AddRow("ETHUSDT", "SHORT", 3000.0, 2900.0, 1.0, 100.0, 3.33, now.Add(-2*time.Hour), now.Add(-1*time.Hour), "sess-1")

	mock.ExpectQuery("SELECT").WithArgs(10).WillReturnRows(rows)

	trades, err := getRecentTrades(t.Context(), bot, 10)
	assert.NoError(t, err)
	assert.Len(t, trades, 2)
	assert.Equal(t, "BTCUSDT", trades[0].Symbol)
	assert.Equal(t, "LONG", trades[0].Side)
	assert.Equal(t, 100.0, trades[0].PnL)
	assert.Equal(t, "ETHUSDT", trades[1].Symbol)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetBalance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}

	// Balance query
	balanceRows := pgxmock.NewRows([]string{"initial_capital", "total_balance", "total_pnl"}).
		AddRow(10000.0, 10500.0, 500.0)
	mock.ExpectQuery("SELECT").WillReturnRows(balanceRows)

	// Open positions query (called by getBalance -> getOpenPositions)
	posRows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "quantity",
		"stop_loss", "take_profit", "current_price",
		"unrealized_pnl", "pnl_percent",
	})
	mock.ExpectQuery("SELECT").WillReturnRows(posRows)

	balance, err := getBalance(t.Context(), bot)
	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, 10000.0, balance.InitialCapital)
	assert.Equal(t, 10500.0, balance.TotalBalance)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAgentsFromDB(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	now := time.Now()

	rows := pgxmock.NewRows([]string{"agent_name", "decisions", "last_run"}).
		AddRow("technical-agent", 42, now).
		AddRow("trend-agent", 18, now.Add(-10*time.Minute))

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	agents, err := getAgentsFromDB(t.Context(), bot)
	assert.NoError(t, err)
	assert.Len(t, agents, 2)
	assert.Equal(t, "technical-agent", agents[0].Name)
	assert.Equal(t, 42, agents[0].Decisions)
	assert.Equal(t, "unknown", agents[0].Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLatestSignals(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	now := time.Now()

	rows := pgxmock.NewRows([]string{"agent_name", "symbol", "decision", "confidence", "price", "created_at"}).
		AddRow("technical-agent", "BTCUSDT", "BUY", 0.85, 50000.0, now).
		AddRow("trend-agent", "ETHUSDT", "SELL", 0.72, 3000.0, now.Add(-5*time.Minute))

	mock.ExpectQuery("SELECT").WithArgs(10).WillReturnRows(rows)

	signals, err := getLatestSignals(t.Context(), bot, 10)
	assert.NoError(t, err)
	assert.Len(t, signals, 2)
	assert.Equal(t, "BUY", signals[0].Signal)
	assert.Equal(t, 0.85, signals[0].Confidence)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25*time.Hour + 15*time.Minute, "1d 1h 15m"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, formatDuration(tt.d))
	}
}
