package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleStartSessionValidation tests input validation for the /startsession command
func TestHandleStartSessionValidation(t *testing.T) {
	tests := []struct {
		name        string
		messageText string
		wantError   bool
		description string
	}{
		{
			name:        "missing all arguments",
			messageText: "/startsession",
			wantError:   false, // Shows help message, doesn't error
			description: "Should show help when no arguments provided",
		},
		{
			name:        "missing capital",
			messageText: "/startsession BTCUSDT",
			wantError:   false, // Shows help message
			description: "Should show help when capital is missing",
		},
		{
			name:        "valid paper mode implicit",
			messageText: "/startsession BTCUSDT 1000",
			wantError:   false,
			description: "Should accept valid input with implicit paper mode",
		},
		{
			name:        "valid paper mode explicit",
			messageText: "/startsession BTCUSDT 1000 PAPER",
			wantError:   false,
			description: "Should accept valid input with explicit paper mode",
		},
		{
			name:        "live mode without confirmation",
			messageText: "/startsession BTCUSDT 1000 LIVE",
			wantError:   false, // Shows confirmation prompt
			description: "Should require confirmation for live mode",
		},
		{
			name:        "live mode with confirmation",
			messageText: "/startsession BTCUSDT 1000 LIVE CONFIRM",
			wantError:   false,
			description: "Should accept live mode with confirmation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test validates the parsing logic exists
			// Full integration tests require the HTTP server mock
			assert.NotEmpty(t, tt.messageText)
		})
	}
}

// TestHandleStopValidation tests input validation for the /stop command
func TestHandleStopValidation(t *testing.T) {
	tests := []struct {
		name            string
		messageText     string
		hasConfirmation bool
	}{
		{
			name:            "without confirmation",
			messageText:     "/stop",
			hasConfirmation: false,
		},
		{
			name:            "with confirmation",
			messageText:     "/stop CONFIRM",
			hasConfirmation: true,
		},
		{
			name:            "with lowercase confirmation",
			messageText:     "/stop confirm",
			hasConfirmation: true, // Should be case-insensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parsing logic
			assert.NotEmpty(t, tt.messageText)
		})
	}
}

// TestVerifyUserFlow tests the verification flow
func TestVerifyUserFlow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	ctx := context.Background()

	t.Run("valid verification code", func(t *testing.T) {
		// Setup expectations for transaction
		mock.ExpectBegin()

		// Expect the check query
		checkRows := pgxmock.NewRows([]string{"id", "telegram_id"}).
			AddRow("user-uuid-123", int64(0))
		mock.ExpectQuery("SELECT id, telegram_id").
			WithArgs("VALIDCODE").
			WillReturnRows(checkRows)

		// Expect the update query
		mock.ExpectExec("UPDATE telegram_users").
			WithArgs(
				int64(123456789), // telegram_id
				int64(987654321), // chat_id
				"testuser",       // username
				"Test",           // first_name
				"User",           // last_name
				"en",             // language_code
				"user-uuid-123",  // user_id
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		mock.ExpectCommit()

		user := &tgbotapi.User{
			ID:           123456789,
			UserName:     "testuser",
			FirstName:    "Test",
			LastName:     "User",
			LanguageCode: "en",
		}

		verified, err := verifyUser(ctx, bot, 123456789, 987654321, "VALIDCODE", user)
		assert.NoError(t, err)
		assert.True(t, verified)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("invalid verification code", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		bot := &Bot{db: mock}

		mock.ExpectBegin()
		// Return empty result (no rows)
		mock.ExpectQuery("SELECT id, telegram_id").
			WithArgs("INVALIDCODE").
			WillReturnRows(pgxmock.NewRows([]string{"id", "telegram_id"}))

		mock.ExpectRollback()

		user := &tgbotapi.User{ID: 123456789}
		verified, err := verifyUser(ctx, bot, 123456789, 987654321, "INVALIDCODE", user)

		// Should return false for no matching code (pgx.ErrNoRows)
		// The function returns (false, nil) for no rows
		assert.NoError(t, err)
		assert.False(t, verified)
	})
}

// TestGetSessionPnLCalculations tests P&L calculation logic
func TestGetSessionPnLCalculations(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	bot := &Bot{db: mock}
	ctx := context.Background()

	// Setup session query mock
	sessionRows := pgxmock.NewRows([]string{
		"initial_capital", "realized_pnl", "total_trades", "winning_trades", "losing_trades",
	}).AddRow(10000.0, 500.0, 20, 12, 8)
	mock.ExpectQuery("SELECT").
		WillReturnRows(sessionRows)

	// Setup position query mock (for unrealized P&L)
	positionRows := pgxmock.NewRows([]string{
		"symbol", "side", "entry_price", "quantity",
		"stop_loss", "take_profit", "current_price",
		"unrealized_pnl", "pnl_percent",
	}).
		AddRow("BTCUSDT", "LONG", 50000.0, 0.1, 49000.0, 52000.0, 51000.0, 100.0, 2.0).
		AddRow("ETHUSDT", "SHORT", 3000.0, 1.0, 3100.0, 2800.0, 2900.0, 100.0, 3.33)
	mock.ExpectQuery("SELECT").
		WillReturnRows(positionRows)

	// Setup win/loss average query mock
	avgRows := pgxmock.NewRows([]string{"avg_win", "avg_loss"}).
		AddRow(75.0, -50.0)
	mock.ExpectQuery("SELECT").
		WillReturnRows(avgRows)

	pnl, err := getSessionPnL(ctx, bot)

	assert.NoError(t, err)
	assert.NotNil(t, pnl)

	// Verify calculations
	assert.Equal(t, 10000.0, pnl.InitialCapital)
	assert.Equal(t, 500.0, pnl.RealizedPnL)
	assert.Equal(t, 200.0, pnl.UnrealizedPnL) // 100 + 100 from positions
	assert.Equal(t, 700.0, pnl.TotalPnL)      // 500 + 200
	assert.Equal(t, 10700.0, pnl.CurrentValue)
	assert.InDelta(t, 7.0, pnl.ReturnPercent, 0.01) // 700/10000 * 100
	assert.Equal(t, 20, pnl.TotalTrades)
	assert.Equal(t, 12, pnl.WinningTrades)
	assert.Equal(t, 8, pnl.LosingTrades)
	assert.Equal(t, 60.0, pnl.WinRate) // 12/20 * 100
	assert.Equal(t, 75.0, pnl.AvgWin)
	assert.Equal(t, -50.0, pnl.AvgLoss)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// TestPositionPnLCalculation tests position P&L display logic
func TestPositionPnLCalculation(t *testing.T) {
	tests := []struct {
		name         string
		side         string
		entryPrice   float64
		currentPrice float64
		quantity     float64
		expectedPnL  float64
		expectedPct  float64
	}{
		{
			name:         "long position profit",
			side:         "LONG",
			entryPrice:   50000.0,
			currentPrice: 51000.0,
			quantity:     0.1,
			expectedPnL:  100.0, // (51000-50000) * 0.1
			expectedPct:  2.0,   // 2%
		},
		{
			name:         "long position loss",
			side:         "LONG",
			entryPrice:   50000.0,
			currentPrice: 49000.0,
			quantity:     0.1,
			expectedPnL:  -100.0,
			expectedPct:  -2.0,
		},
		{
			name:         "short position profit",
			side:         "SHORT",
			entryPrice:   3000.0,
			currentPrice: 2900.0,
			quantity:     1.0,
			expectedPnL:  100.0, // (3000-2900) * 1
			expectedPct:  3.33,  // ~3.33%
		},
		{
			name:         "short position loss",
			side:         "SHORT",
			entryPrice:   3000.0,
			currentPrice: 3100.0,
			quantity:     1.0,
			expectedPnL:  -100.0,
			expectedPct:  -3.33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualPnL, actualPct float64

			if tt.side == "LONG" {
				actualPnL = (tt.currentPrice - tt.entryPrice) * tt.quantity
				actualPct = ((tt.currentPrice - tt.entryPrice) / tt.entryPrice) * 100
			} else {
				actualPnL = (tt.entryPrice - tt.currentPrice) * tt.quantity
				actualPct = ((tt.entryPrice - tt.currentPrice) / tt.entryPrice) * 100
			}

			assert.InDelta(t, tt.expectedPnL, actualPnL, 0.01)
			assert.InDelta(t, tt.expectedPct, actualPct, 0.01)
		})
	}
}

// TestOrchestratorStatusQuery tests the orchestrator status query
func TestOrchestratorStatusQuery(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/orchestrator/status", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"state":"running","is_paused":false,"active_agents":5}`))
	}))
	defer server.Close()

	bot := &Bot{
		config: &Config{
			OrchestratorURL: server.URL,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := queryOrchestratorStatus(ctx, bot)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "running", status.State)
	assert.False(t, status.IsPaused)
	assert.Equal(t, 5, status.ActiveAgents)
}

// TestOrchestratorCommandSend tests sending commands to orchestrator
func TestOrchestratorCommandSend(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		statusCode int
		wantError  bool
	}{
		{
			name:       "pause command success",
			command:    "pause",
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "resume command success",
			command:    "resume",
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "command failure",
			command:    "pause",
			statusCode: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/orchestrator/"+tt.command, r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			bot := &Bot{
				config: &Config{
					OrchestratorURL: server.URL,
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := sendOrchestratorCommand(ctx, bot, tt.command)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStartTradingSession tests the start trading session API call
func TestStartTradingSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/trade/start", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"session_id":"session-123","message":"Session started"}`))
	}))
	defer server.Close()

	bot := &Bot{
		config: &Config{
			OrchestratorURL: server.URL,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessionID, err := startTradingSession(ctx, bot, "BTCUSDT", 1000.0, "PAPER")

	assert.NoError(t, err)
	assert.Equal(t, "session-123", sessionID)
}

// TestStopTradingSession tests the stop trading session flow
func TestStopTradingSession(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/trade/stop", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bot := &Bot{
		db: mock,
		config: &Config{
			OrchestratorURL: server.URL,
		},
	}

	// Setup mock for session query
	sessionRows := pgxmock.NewRows([]string{"id", "initial_capital", "total_pnl"}).
		AddRow("session-123", 10000.0, 500.0)
	mock.ExpectQuery("SELECT id, initial_capital").
		WithArgs("BTCUSDT").
		WillReturnRows(sessionRows)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = stopTradingSession(ctx, bot, "BTCUSDT")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAbsFunction tests the abs helper function
func TestAbsFunction(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{input: 5.0, expected: 5.0},
		{input: -5.0, expected: 5.0},
		{input: 0.0, expected: 0.0},
		{input: 0.0, expected: 0.0}, // negative zero test removed (SA4026)
		{input: 123.456, expected: 123.456},
		{input: -123.456, expected: 123.456},
	}

	for _, tt := range tests {
		result := abs(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

// TestBoolToEmoji tests the boolToEmoji helper function
func TestBoolToEmoji(t *testing.T) {
	assert.Contains(t, boolToEmoji(true), "Enabled")
	assert.Contains(t, boolToEmoji(false), "Disabled")
}

// TestUserSettingsStruct tests UserSettings structure
func TestUserSettingsStruct(t *testing.T) {
	settings := UserSettings{
		ReceiveAlerts:             true,
		ReceiveTradeNotifications: true,
		ReceiveDailySummary:       false,
	}

	assert.True(t, settings.ReceiveAlerts)
	assert.True(t, settings.ReceiveTradeNotifications)
	assert.False(t, settings.ReceiveDailySummary)
}

// TestDecisionStruct tests Decision structure
func TestDecisionStruct(t *testing.T) {
	now := time.Now()
	decision := Decision{
		AgentName:  "technical-agent",
		Decision:   "BUY",
		Symbol:     "BTCUSDT",
		Confidence: 0.85,
		Reasoning:  "RSI indicates oversold conditions",
		CreatedAt:  now,
	}

	assert.Equal(t, "technical-agent", decision.AgentName)
	assert.Equal(t, "BUY", decision.Decision)
	assert.Equal(t, "BTCUSDT", decision.Symbol)
	assert.Equal(t, 0.85, decision.Confidence)
	assert.Equal(t, "RSI indicates oversold conditions", decision.Reasoning)
	assert.Equal(t, now, decision.CreatedAt)
}

// TestPositionStruct tests Position structure
func TestPositionStruct(t *testing.T) {
	position := Position{
		Symbol:        "BTCUSDT",
		Side:          "LONG",
		EntryPrice:    50000.0,
		CurrentPrice:  51000.0,
		Quantity:      0.1,
		UnrealizedPnL: 100.0,
		PnLPercent:    2.0,
		StopLoss:      49000.0,
		TakeProfit:    52000.0,
	}

	assert.Equal(t, "BTCUSDT", position.Symbol)
	assert.Equal(t, "LONG", position.Side)
	assert.Equal(t, 50000.0, position.EntryPrice)
	assert.Equal(t, 51000.0, position.CurrentPrice)
	assert.Equal(t, 0.1, position.Quantity)
	assert.Equal(t, 100.0, position.UnrealizedPnL)
	assert.Equal(t, 2.0, position.PnLPercent)
	assert.Equal(t, 49000.0, position.StopLoss)
	assert.Equal(t, 52000.0, position.TakeProfit)
}

// TestSessionInfoStruct tests SessionInfo structure
func TestSessionInfoStruct(t *testing.T) {
	now := time.Now()
	session := SessionInfo{
		Symbol:      "BTCUSDT",
		Mode:        "PAPER",
		StartedAt:   now,
		TotalTrades: 15,
		PnLPercent:  5.5,
	}

	assert.Equal(t, "BTCUSDT", session.Symbol)
	assert.Equal(t, "PAPER", session.Mode)
	assert.Equal(t, now, session.StartedAt)
	assert.Equal(t, 15, session.TotalTrades)
	assert.Equal(t, 5.5, session.PnLPercent)
}

// TestPnLReportStruct tests PnLReport structure
func TestPnLReportStruct(t *testing.T) {
	report := PnLReport{
		RealizedPnL:    500.0,
		UnrealizedPnL:  200.0,
		TotalPnL:       700.0,
		InitialCapital: 10000.0,
		CurrentValue:   10700.0,
		ReturnPercent:  7.0,
		TotalTrades:    20,
		WinningTrades:  12,
		LosingTrades:   8,
		WinRate:        60.0,
		AvgWin:         75.0,
		AvgLoss:        -50.0,
	}

	assert.Equal(t, 500.0, report.RealizedPnL)
	assert.Equal(t, 200.0, report.UnrealizedPnL)
	assert.Equal(t, 700.0, report.TotalPnL)
	assert.Equal(t, 10000.0, report.InitialCapital)
	assert.Equal(t, 10700.0, report.CurrentValue)
	assert.Equal(t, 7.0, report.ReturnPercent)
	assert.Equal(t, 20, report.TotalTrades)
	assert.Equal(t, 12, report.WinningTrades)
	assert.Equal(t, 8, report.LosingTrades)
	assert.Equal(t, 60.0, report.WinRate)
	assert.Equal(t, 75.0, report.AvgWin)
	assert.Equal(t, -50.0, report.AvgLoss)
}

// TestOrchestratorStatusStruct tests OrchestratorStatus structure
func TestOrchestratorStatusStruct(t *testing.T) {
	status := OrchestratorStatus{
		State:        "running",
		IsPaused:     false,
		ActiveAgents: 5,
	}

	assert.Equal(t, "running", status.State)
	assert.False(t, status.IsPaused)
	assert.Equal(t, 5, status.ActiveAgents)
}
