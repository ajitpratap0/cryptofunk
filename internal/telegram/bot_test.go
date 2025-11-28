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

func TestNewBot(t *testing.T) {
	t.Skip("Skipping test that requires actual Telegram API token")
}

func TestUpdateLastInteraction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	// Create a bot with the mock pool interface converted properly
	bot := &Bot{
		db: mock,
	}

	telegramID := int64(123456789)
	chatID := int64(987654321)

	// Expect the insert/update query
	mock.ExpectExec("INSERT INTO telegram_users").
		WithArgs(telegramID, chatID).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = bot.updateLastInteraction(telegramID, chatID)
	assert.NoError(t, err)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestIsUserVerified(t *testing.T) {
	tests := []struct {
		name       string
		telegramID int64
		setupMock  func(mock pgxmock.PgxPoolIface)
		want       bool
		wantError  bool
	}{
		{
			name:       "verified user",
			telegramID: 123456789,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"is_verified"}).AddRow(true)
				mock.ExpectQuery("SELECT is_verified").
					WithArgs(int64(123456789)).
					WillReturnRows(rows)
			},
			want:      true,
			wantError: false,
		},
		{
			name:       "unverified user",
			telegramID: 987654321,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"is_verified"}).AddRow(false)
				mock.ExpectQuery("SELECT is_verified").
					WithArgs(int64(987654321)).
					WillReturnRows(rows)
			},
			want:      false,
			wantError: false,
		},
		{
			name:       "user not found",
			telegramID: 111111111,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT is_verified").
					WithArgs(int64(111111111)).
					WillReturnError(pgx.ErrNoRows)
			},
			want:      false,
			wantError: false, // Returns false when user not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			bot := &Bot{db: mock}
			tt.setupMock(mock)

			ctx := context.Background()
			verified, err := isUserVerified(ctx, bot, tt.telegramID)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, verified)
			}
		})
	}
}

func TestGetUserSettings(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	telegramID := int64(123456789)

	// Setup mock expectation
	rows := pgxmock.NewRows([]string{
		"receive_alerts",
		"receive_trade_notifications",
		"receive_daily_summary",
	}).AddRow(true, true, false)

	mock.ExpectQuery("SELECT receive_alerts").
		WithArgs(telegramID).
		WillReturnRows(rows)

	ctx := context.Background()
	settings, err := getUserSettings(ctx, bot, telegramID)

	assert.NoError(t, err)
	assert.NotNil(t, settings)
	assert.True(t, settings.ReceiveAlerts)
	assert.True(t, settings.ReceiveTradeNotifications)
	assert.False(t, settings.ReceiveDailySummary)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestGetActiveSessions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}

	// Setup mock expectation
	now := time.Now()
	rows := pgxmock.NewRows([]string{
		"symbol", "mode", "started_at", "total_trades", "pnl_percent",
	}).
		AddRow("BTCUSDT", "PAPER", now, 10, 5.5).
		AddRow("ETHUSDT", "PAPER", now.Add(-1*time.Hour), 5, -2.3)

	mock.ExpectQuery("SELECT symbol, mode").
		WillReturnRows(rows)

	ctx := context.Background()
	sessions, err := getActiveSessions(ctx, bot)

	assert.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Equal(t, "BTCUSDT", sessions[0].Symbol)
	assert.Equal(t, "PAPER", sessions[0].Mode)
	assert.Equal(t, 10, sessions[0].TotalTrades)
	assert.Equal(t, 5.5, sessions[0].PnLPercent)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestGetOpenPositions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}

	// Setup mock expectation
	rows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "quantity",
		"stop_loss", "take_profit", "current_price",
		"unrealized_pnl", "pnl_percent",
	}).
		AddRow("BTCUSDT", "LONG", 50000.0, 0.1,
			49000.0, 52000.0, 51000.0,
			100.0, 2.0)

	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	ctx := context.Background()
	positions, err := getOpenPositions(ctx, bot)

	assert.NoError(t, err)
	assert.Len(t, positions, 1)
	assert.Equal(t, "BTCUSDT", positions[0].Symbol)
	assert.Equal(t, "LONG", positions[0].Side)
	assert.Equal(t, 50000.0, positions[0].EntryPrice)
	assert.Equal(t, 51000.0, positions[0].CurrentPrice)
	assert.Equal(t, 100.0, positions[0].UnrealizedPnL)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestGetRecentDecisions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}

	// Setup mock expectation
	now := time.Now()
	rows := pgxmock.NewRows([]string{
		"agent_name", "decision", "symbol", "confidence", "reasoning", "created_at",
	}).
		AddRow("technical-agent", "BUY", "BTCUSDT", 0.85, "RSI oversold", now).
		AddRow("trend-agent", "HOLD", "ETHUSDT", 0.65, "Sideways trend", now.Add(-5*time.Minute))

	mock.ExpectQuery("SELECT").
		WithArgs(5).
		WillReturnRows(rows)

	ctx := context.Background()
	decisions, err := getRecentDecisions(ctx, bot, 5)

	assert.NoError(t, err)
	assert.Len(t, decisions, 2)
	assert.Equal(t, "technical-agent", decisions[0].AgentName)
	assert.Equal(t, "BUY", decisions[0].Decision)
	assert.Equal(t, "BTCUSDT", decisions[0].Symbol)
	assert.Equal(t, 0.85, decisions[0].Confidence)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestGenerateVerificationCode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	// Setup mock expectation
	rows := pgxmock.NewRows([]string{"verification_code"}).
		AddRow("ABC123")

	mock.ExpectQuery("INSERT INTO telegram_users").
		WillReturnRows(rows)

	ctx := context.Background()
	code, err := GenerateVerificationCode(ctx, mock)

	assert.NoError(t, err)
	assert.Equal(t, "ABC123", code)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestGetVerifiedUsers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	// Setup mock expectation
	rows := pgxmock.NewRows([]string{"chat_id"}).
		AddRow(int64(123456789)).
		AddRow(int64(987654321))

	mock.ExpectQuery("SELECT chat_id").
		WillReturnRows(rows)

	ctx := context.Background()
	chatIDs, err := GetVerifiedUsers(ctx, mock)

	assert.NoError(t, err)
	assert.Len(t, chatIDs, 2)
	assert.Contains(t, chatIDs, int64(123456789))
	assert.Contains(t, chatIDs, int64(987654321))

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestQueueAlert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	chatID := int64(123456789)
	title := "Test Alert"
	message := "This is a test alert"
	severity := "INFO"
	metadata := map[string]interface{}{
		"test_key": "test_value",
	}

	// Setup mock expectation
	mock.ExpectExec("INSERT INTO telegram_alert_queue").
		WithArgs(chatID, title, message, severity, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	ctx := context.Background()
	err = QueueAlert(ctx, mock, chatID, title, message, severity, metadata)

	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestBotConfig(t *testing.T) {
	config := &Config{
		BotToken:        "test_token",
		WebhookURL:      "https://example.com/webhook",
		PollingTimeout:  60,
		Debug:           true,
		OrchestratorURL: "http://localhost:8081",
	}

	assert.Equal(t, "test_token", config.BotToken)
	assert.Equal(t, "https://example.com/webhook", config.WebhookURL)
	assert.Equal(t, 60, config.PollingTimeout)
	assert.True(t, config.Debug)
	assert.Equal(t, "http://localhost:8081", config.OrchestratorURL)
}
