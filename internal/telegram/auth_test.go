package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserSettings_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	mock.ExpectQuery("SELECT").WithArgs(int64(999)).WillReturnError(pgx.ErrNoRows)

	settings, err := getUserSettings(t.Context(), bot, 999)
	assert.Error(t, err)
	assert.Nil(t, settings)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetActiveSessions_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	rows := pgxmock.NewRows([]string{"symbol", "mode", "started_at", "total_trades", "pnl_percent"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	sessions, err := getActiveSessions(t.Context(), bot)
	assert.NoError(t, err)
	assert.Empty(t, sessions)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOpenPositions_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	rows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "quantity",
		"stop_loss", "take_profit", "current_price",
		"unrealized_pnl", "pnl_percent",
	})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	positions, err := getOpenPositions(t.Context(), bot)
	assert.NoError(t, err)
	assert.Empty(t, positions)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRecentDecisions_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	rows := pgxmock.NewRows([]string{
		"agent_name", "decision", "symbol", "confidence", "reasoning", "created_at",
	})
	mock.ExpectQuery("SELECT").WithArgs(5).WillReturnRows(rows)

	decisions, err := getRecentDecisions(t.Context(), bot, 5)
	assert.NoError(t, err)
	assert.Empty(t, decisions)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVerifiedUsers_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"chat_id"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	chatIDs, err := GetVerifiedUsers(t.Context(), mock)
	assert.NoError(t, err)
	assert.Empty(t, chatIDs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueueAlert_NilMetadata(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectExec("INSERT INTO telegram_alert_queue").
		WithArgs(int64(123), "Alert", "Msg", "info", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = QueueAlert(t.Context(), mock, 123, "Alert", "Msg", "info", nil)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLastTradeTime(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	expected := time.Date(2026, 2, 13, 10, 30, 0, 0, time.UTC)

	rows := pgxmock.NewRows([]string{"closed_at"}).AddRow(expected)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := getLastTradeTime(context.Background(), bot)
	assert.NoError(t, err)
	assert.Equal(t, expected, *result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLastTradeTime_NoTrades(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	mock.ExpectQuery("SELECT").WillReturnError(pgx.ErrNoRows)

	result, err := getLastTradeTime(context.Background(), bot)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSessionPnL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}

	// Session query
	sessionRows := pgxmock.NewRows([]string{
		"initial_capital", "realized_pnl", "total_trades", "winning_trades", "losing_trades",
	}).AddRow(10000.0, 500.0, 20, 12, 8)
	mock.ExpectQuery("SELECT").WillReturnRows(sessionRows)

	// Open positions query
	posRows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "quantity",
		"stop_loss", "take_profit", "current_price",
		"unrealized_pnl", "pnl_percent",
	}).AddRow("BTCUSDT", "LONG", 50000.0, 0.1, 49000.0, 52000.0, 51000.0, 100.0, 2.0)
	mock.ExpectQuery("SELECT").WillReturnRows(posRows)

	// Avg win/loss query
	avgRows := pgxmock.NewRows([]string{"avg_win", "avg_loss"}).AddRow(75.0, -40.0)
	mock.ExpectQuery("SELECT").WillReturnRows(avgRows)

	report, err := getSessionPnL(t.Context(), bot)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 10000.0, report.InitialCapital)
	assert.Equal(t, 500.0, report.RealizedPnL)
	assert.Equal(t, 100.0, report.UnrealizedPnL)
	assert.Equal(t, 600.0, report.TotalPnL)
	assert.Equal(t, 10600.0, report.CurrentValue)
	assert.Equal(t, 6.0, report.ReturnPercent)
	assert.Equal(t, 60.0, report.WinRate)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSessionPnL_NoPositions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}

	sessionRows := pgxmock.NewRows([]string{
		"initial_capital", "realized_pnl", "total_trades", "winning_trades", "losing_trades",
	}).AddRow(5000.0, 0.0, 0, 0, 0)
	mock.ExpectQuery("SELECT").WillReturnRows(sessionRows)

	posRows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "quantity",
		"stop_loss", "take_profit", "current_price",
		"unrealized_pnl", "pnl_percent",
	})
	mock.ExpectQuery("SELECT").WillReturnRows(posRows)

	report, err := getSessionPnL(t.Context(), bot)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, report.TotalPnL)
	assert.Equal(t, 5000.0, report.CurrentValue)
	assert.Equal(t, 0.0, report.WinRate)
	assert.NoError(t, mock.ExpectationsWereMet())
}
