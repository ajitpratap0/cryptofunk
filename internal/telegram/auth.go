package telegram

import (
	"context"
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
)

// isUserVerified checks if a Telegram user is verified
func isUserVerified(ctx context.Context, bot *Bot, telegramID int64) (bool, error) {
	query := `
		SELECT is_verified
		FROM telegram_users
		WHERE telegram_id = $1
	`

	var isVerified bool
	err := bot.db.QueryRow(ctx, query, telegramID).Scan(&isVerified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check verification status: %w", err)
	}

	return isVerified, nil
}

// verifyUser verifies a user with a verification code
func verifyUser(ctx context.Context, bot *Bot, telegramID int64, chatID int64, code string, user *tgbotapi.User) (bool, error) {
	// Start a transaction
	tx, err := bot.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check if verification code is valid and not expired
	checkQuery := `
		SELECT id, telegram_id
		FROM telegram_users
		WHERE verification_code = $1
		AND verification_expires_at > CURRENT_TIMESTAMP
		AND is_verified = false
		FOR UPDATE
	`

	var userID string
	var existingTelegramID int64
	err = tx.QueryRow(ctx, checkQuery, code).Scan(&userID, &existingTelegramID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil // Invalid or expired code
		}
		return false, fmt.Errorf("failed to check verification code: %w", err)
	}

	// Update the user record with Telegram details
	updateQuery := `
		UPDATE telegram_users
		SET telegram_id = $1,
		    chat_id = $2,
		    telegram_username = $3,
		    first_name = $4,
		    last_name = $5,
		    language_code = $6,
		    is_verified = true,
		    verification_code = NULL,
		    verification_expires_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $7
	`

	_, err = tx.Exec(ctx, updateQuery,
		telegramID,
		chatID,
		user.UserName,
		user.FirstName,
		user.LastName,
		user.LanguageCode,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("failed to update user: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return true, nil
}

// getUserSettings retrieves user notification settings
func getUserSettings(ctx context.Context, bot *Bot, telegramID int64) (*UserSettings, error) {
	query := `
		SELECT receive_alerts, receive_trade_notifications, receive_daily_summary
		FROM telegram_users
		WHERE telegram_id = $1 AND is_verified = true
	`

	var settings UserSettings
	err := bot.db.QueryRow(ctx, query, telegramID).Scan(
		&settings.ReceiveAlerts,
		&settings.ReceiveTradeNotifications,
		&settings.ReceiveDailySummary,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user settings: %w", err)
	}

	return &settings, nil
}

// getActiveSessions retrieves active trading sessions
func getActiveSessions(ctx context.Context, bot *Bot) ([]SessionInfo, error) {
	query := `
		SELECT symbol, mode, started_at, total_trades,
		       COALESCE((total_pnl / NULLIF(initial_capital, 0)) * 100, 0) as pnl_percent
		FROM trading_sessions
		WHERE stopped_at IS NULL
		ORDER BY started_at DESC
		LIMIT 10
	`

	rows, err := bot.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var session SessionInfo
		err := rows.Scan(
			&session.Symbol,
			&session.Mode,
			&session.StartedAt,
			&session.TotalTrades,
			&session.PnLPercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// getOpenPositions retrieves currently open positions
func getOpenPositions(ctx context.Context, bot *Bot) ([]Position, error) {
	query := `
		SELECT
			p.symbol,
			p.side::text,
			p.entry_price,
			p.quantity,
			p.stop_loss,
			p.take_profit,
			COALESCE(c.close, p.entry_price) as current_price,
			CASE
				WHEN p.side = 'LONG' THEN (COALESCE(c.close, p.entry_price) - p.entry_price) * p.quantity
				WHEN p.side = 'SHORT' THEN (p.entry_price - COALESCE(c.close, p.entry_price)) * p.quantity
				ELSE 0
			END as unrealized_pnl,
			CASE
				WHEN p.side = 'LONG' THEN ((COALESCE(c.close, p.entry_price) - p.entry_price) / NULLIF(p.entry_price, 0)) * 100
				WHEN p.side = 'SHORT' THEN ((p.entry_price - COALESCE(c.close, p.entry_price)) / NULLIF(p.entry_price, 0)) * 100
				ELSE 0
			END as pnl_percent
		FROM positions p
		LEFT JOIN LATERAL (
			SELECT close
			FROM candlesticks
			WHERE symbol = p.symbol
			AND interval = '1m'
			ORDER BY open_time DESC
			LIMIT 1
		) c ON true
		WHERE p.closed_at IS NULL
		ORDER BY p.opened_at DESC
	`

	rows, err := bot.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query positions: %w", err)
	}
	defer rows.Close()

	var positions []Position
	for rows.Next() {
		var pos Position
		err := rows.Scan(
			&pos.Symbol,
			&pos.Side,
			&pos.EntryPrice,
			&pos.Quantity,
			&pos.StopLoss,
			&pos.TakeProfit,
			&pos.CurrentPrice,
			&pos.UnrealizedPnL,
			&pos.PnLPercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, pos)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating positions: %w", err)
	}

	return positions, nil
}

// getSessionPnL calculates P&L for current sessions
func getSessionPnL(ctx context.Context, bot *Bot) (*PnLReport, error) {
	// Get realized P&L from active sessions
	sessionQuery := `
		SELECT
			COALESCE(SUM(initial_capital), 0) as initial_capital,
			COALESCE(SUM(total_pnl), 0) as realized_pnl,
			COALESCE(SUM(total_trades), 0) as total_trades,
			COALESCE(SUM(winning_trades), 0) as winning_trades,
			COALESCE(SUM(losing_trades), 0) as losing_trades
		FROM trading_sessions
		WHERE stopped_at IS NULL
	`

	var report PnLReport
	err := bot.db.QueryRow(ctx, sessionQuery).Scan(
		&report.InitialCapital,
		&report.RealizedPnL,
		&report.TotalTrades,
		&report.WinningTrades,
		&report.LosingTrades,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query session P&L: %w", err)
	}

	// Get unrealized P&L from open positions
	positions, err := getOpenPositions(ctx, bot)
	if err != nil {
		return nil, err
	}

	report.UnrealizedPnL = 0
	for _, pos := range positions {
		report.UnrealizedPnL += pos.UnrealizedPnL
	}

	// Calculate totals
	report.TotalPnL = report.RealizedPnL + report.UnrealizedPnL
	report.CurrentValue = report.InitialCapital + report.TotalPnL

	if report.InitialCapital > 0 {
		report.ReturnPercent = (report.TotalPnL / report.InitialCapital) * 100
	}

	if report.TotalTrades > 0 {
		report.WinRate = (float64(report.WinningTrades) / float64(report.TotalTrades)) * 100
	}

	// Calculate average win/loss
	if report.WinningTrades > 0 || report.LosingTrades > 0 {
		winSumQuery := `
			SELECT
				COALESCE(AVG(CASE WHEN pnl > 0 THEN pnl ELSE NULL END), 0) as avg_win,
				COALESCE(AVG(CASE WHEN pnl < 0 THEN pnl ELSE NULL END), 0) as avg_loss
			FROM trades
			WHERE session_id IN (
				SELECT id FROM trading_sessions WHERE stopped_at IS NULL
			)
		`

		err = bot.db.QueryRow(ctx, winSumQuery).Scan(&report.AvgWin, &report.AvgLoss)
		if err != nil {
			// Non-critical error, continue with zeros
			report.AvgWin = 0
			report.AvgLoss = 0
		}
	}

	return &report, nil
}

// getRecentDecisions retrieves recent agent decisions
func getRecentDecisions(ctx context.Context, bot *Bot, limit int) ([]Decision, error) {
	query := `
		SELECT
			agent_name,
			decision,
			symbol,
			confidence,
			COALESCE(reasoning, '') as reasoning,
			created_at
		FROM llm_decisions
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := bot.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer rows.Close()

	var decisions []Decision
	for rows.Next() {
		var decision Decision
		err := rows.Scan(
			&decision.AgentName,
			&decision.Decision,
			&decision.Symbol,
			&decision.Confidence,
			&decision.Reasoning,
			&decision.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan decision: %w", err)
		}
		decisions = append(decisions, decision)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating decisions: %w", err)
	}

	return decisions, nil
}

// GenerateVerificationCode generates a verification code for a new Telegram user
func GenerateVerificationCode(ctx context.Context, db DBPool) (string, error) {
	query := `
		INSERT INTO telegram_users (verification_code, verification_expires_at)
		VALUES (generate_verification_code(), CURRENT_TIMESTAMP + INTERVAL '1 hour')
		RETURNING verification_code
	`

	var code string
	err := db.QueryRow(ctx, query).Scan(&code)
	if err != nil {
		return "", fmt.Errorf("failed to generate verification code: %w", err)
	}

	return code, nil
}

// GetVerifiedUsers retrieves all verified Telegram users
func GetVerifiedUsers(ctx context.Context, db DBPool) ([]int64, error) {
	query := `
		SELECT chat_id
		FROM telegram_users
		WHERE is_verified = true
		AND receive_alerts = true
	`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query verified users: %w", err)
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, fmt.Errorf("failed to scan chat ID: %w", err)
		}
		chatIDs = append(chatIDs, chatID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating verified users: %w", err)
	}

	return chatIDs, nil
}

// QueueAlert adds an alert to the Telegram alert queue
func QueueAlert(ctx context.Context, db DBPool, chatID int64, title, message, severity string, metadata map[string]interface{}) error {
	query := `
		INSERT INTO telegram_alert_queue (telegram_user_id, chat_id, title, message, severity, metadata)
		SELECT id, $1, $2, $3, $4, $5
		FROM telegram_users
		WHERE chat_id = $1 AND is_verified = true
	`

	var metadataJSON []byte
	var err error
	if metadata != nil {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err = db.Exec(ctx, query, chatID, title, message, severity, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to queue alert: %w", err)
	}

	return nil
}
