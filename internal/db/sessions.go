package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

// TradingMode represents trading mode (database enum)
type TradingMode string

const (
	TradingModeLive  TradingMode = "LIVE"
	TradingModePaper TradingMode = "PAPER"
)

// TradingSession represents a database trading session record
type TradingSession struct {
	ID             uuid.UUID              `json:"id"`
	Mode           TradingMode            `json:"mode"`
	Symbol         string                 `json:"symbol"`
	Exchange       string                 `json:"exchange"`
	StartedAt      time.Time              `json:"started_at"`
	StoppedAt      *time.Time             `json:"stopped_at"`
	InitialCapital float64                `json:"initial_capital"`
	FinalCapital   *float64               `json:"final_capital"`
	TotalTrades    int                    `json:"total_trades"`
	WinningTrades  int                    `json:"winning_trades"`
	LosingTrades   int                    `json:"losing_trades"`
	TotalPnL       float64                `json:"total_pnl"`
	MaxDrawdown    float64                `json:"max_drawdown"`
	SharpeRatio    *float64               `json:"sharpe_ratio"`
	Config         map[string]interface{} `json:"config,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// CreateSession creates a new trading session
func (db *DB) CreateSession(ctx context.Context, session *TradingSession) error {
	query := `
		INSERT INTO trading_sessions (
			id, mode, symbol, exchange, started_at, initial_capital,
			config, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	now := time.Now()
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	session.CreatedAt = now
	session.UpdatedAt = now

	_, err := db.pool.Exec(ctx, query,
		session.ID,
		session.Mode,
		session.Symbol,
		session.Exchange,
		session.StartedAt,
		session.InitialCapital,
		session.Config,
		session.Metadata,
		session.CreatedAt,
		session.UpdatedAt,
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("session_id", session.ID.String()).
			Msg("Failed to create trading session")
		return fmt.Errorf("failed to create trading session: %w", err)
	}

	log.Info().
		Str("session_id", session.ID.String()).
		Str("mode", string(session.Mode)).
		Str("symbol", session.Symbol).
		Msg("Trading session created")

	return nil
}

// GetSession retrieves a trading session by ID
func (db *DB) GetSession(ctx context.Context, sessionID uuid.UUID) (*TradingSession, error) {
	query := `
		SELECT id, mode, symbol, exchange, started_at, stopped_at,
		       initial_capital, final_capital, total_trades, winning_trades,
		       losing_trades, total_pnl, max_drawdown, sharpe_ratio,
		       config, metadata, created_at, updated_at
		FROM trading_sessions
		WHERE id = $1
	`

	var session TradingSession
	err := db.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&session.Mode,
		&session.Symbol,
		&session.Exchange,
		&session.StartedAt,
		&session.StoppedAt,
		&session.InitialCapital,
		&session.FinalCapital,
		&session.TotalTrades,
		&session.WinningTrades,
		&session.LosingTrades,
		&session.TotalPnL,
		&session.MaxDrawdown,
		&session.SharpeRatio,
		&session.Config,
		&session.Metadata,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get trading session: %w", err)
	}

	return &session, nil
}

// UpdateSessionStats updates trading session statistics
func (db *DB) UpdateSessionStats(ctx context.Context, sessionID uuid.UUID, stats SessionStats) error {
	query := `
		UPDATE trading_sessions
		SET total_trades = $1,
		    winning_trades = $2,
		    losing_trades = $3,
		    total_pnl = $4,
		    max_drawdown = $5,
		    sharpe_ratio = $6,
		    updated_at = NOW()
		WHERE id = $7
	`

	result, err := db.pool.Exec(ctx, query,
		stats.TotalTrades,
		stats.WinningTrades,
		stats.LosingTrades,
		stats.TotalPnL,
		stats.MaxDrawdown,
		stats.SharpeRatio,
		sessionID,
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("session_id", sessionID.String()).
			Msg("Failed to update session stats")
		return fmt.Errorf("failed to update session stats: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("trading session not found: %s", sessionID.String())
	}

	log.Debug().
		Str("session_id", sessionID.String()).
		Int("total_trades", stats.TotalTrades).
		Float64("total_pnl", stats.TotalPnL).
		Msg("Session stats updated")

	return nil
}

// StopSession marks a trading session as stopped
func (db *DB) StopSession(ctx context.Context, sessionID uuid.UUID, finalCapital float64) error {
	query := `
		UPDATE trading_sessions
		SET stopped_at = NOW(),
		    final_capital = $1,
		    updated_at = NOW()
		WHERE id = $2
		AND stopped_at IS NULL
	`

	result, err := db.pool.Exec(ctx, query, finalCapital, sessionID)
	if err != nil {
		log.Error().
			Err(err).
			Str("session_id", sessionID.String()).
			Msg("Failed to stop trading session")
		return fmt.Errorf("failed to stop trading session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("trading session not found or already stopped: %s", sessionID.String())
	}

	log.Info().
		Str("session_id", sessionID.String()).
		Float64("final_capital", finalCapital).
		Msg("Trading session stopped")

	return nil
}

// ListActiveSessions retrieves all active (not stopped) trading sessions.
// It delegates to ListActiveSessionsPaginated with limit=0 (unlimited).
func (db *DB) ListActiveSessions(ctx context.Context) ([]*TradingSession, error) {
	return db.ListActiveSessionsPaginated(ctx, 0, 0)
}

// scanSessions reads all rows from a pgx.Rows result set into a slice of
// TradingSession pointers. The caller is responsible for calling rows.Close().
// This mirrors the scanOrders helper in orders.go.
func scanSessions(rows pgx.Rows) ([]*TradingSession, error) {
	var sessions []*TradingSession
	for rows.Next() {
		var session TradingSession
		err := rows.Scan(
			&session.ID,
			&session.Mode,
			&session.Symbol,
			&session.Exchange,
			&session.StartedAt,
			&session.StoppedAt,
			&session.InitialCapital,
			&session.FinalCapital,
			&session.TotalTrades,
			&session.WinningTrades,
			&session.LosingTrades,
			&session.TotalPnL,
			&session.MaxDrawdown,
			&session.SharpeRatio,
			&session.Config,
			&session.Metadata,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan session row")
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		sessions = append(sessions, &session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating session rows: %w", err)
	}
	return sessions, nil
}

// ListActiveSessionsPaginated retrieves active (not stopped) trading sessions with
// limit/offset pagination. limit=0 means no limit (returns all rows).
func (db *DB) ListActiveSessionsPaginated(ctx context.Context, limit, offset int) ([]*TradingSession, error) {
	query := `
		SELECT id, mode, symbol, exchange, started_at, stopped_at,
		       initial_capital, final_capital, total_trades, winning_trades,
		       losing_trades, total_pnl, max_drawdown, sharpe_ratio,
		       config, metadata, created_at, updated_at
		FROM trading_sessions
		WHERE stopped_at IS NULL
		ORDER BY started_at DESC
		LIMIT NULLIF($1, 0) OFFSET $2
	`

	rows, err := db.pool.Query(ctx, query, limit, offset)
	if err != nil {
		log.Error().
			Err(err).
			Int("limit", limit).
			Int("offset", offset).
			Msg("Failed to list active sessions (paginated)")
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}
	defer rows.Close()

	sessions, err := scanSessions(rows)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Int("limit", limit).
		Int("offset", offset).
		Int("count", len(sessions)).
		Msg("Listed active sessions (paginated)")

	return sessions, nil
}

// GetSessionsBySymbol retrieves all trading sessions for a specific symbol
func (db *DB) GetSessionsBySymbol(ctx context.Context, symbol string) ([]*TradingSession, error) {
	query := `
		SELECT id, mode, symbol, exchange, started_at, stopped_at,
		       initial_capital, final_capital, total_trades, winning_trades,
		       losing_trades, total_pnl, max_drawdown, sharpe_ratio,
		       config, metadata, created_at, updated_at
		FROM trading_sessions
		WHERE symbol = $1
		ORDER BY started_at DESC
	`

	rows, err := db.pool.Query(ctx, query, symbol)
	if err != nil {
		log.Error().
			Err(err).
			Str("symbol", symbol).
			Msg("Failed to get sessions by symbol")
		return nil, fmt.Errorf("failed to get sessions by symbol: %w", err)
	}
	defer rows.Close()

	sessions, err := scanSessions(rows)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Str("symbol", symbol).
		Int("count", len(sessions)).
		Msg("Retrieved sessions by symbol")

	return sessions, nil
}

// AggregateSessionStats recalculates session statistics from completed trades.
// All four counters (total_trades, winning_trades, losing_trades, total_pnl) are
// derived from closed positions (exit_time IS NOT NULL) so the win-rate denominator
// is consistent: win_rate = winning_trades / total_trades.
// Previously total_trades counted FILLED/PARTIALLY_FILLED orders while the win/loss
// counts used positions, causing a denominator mismatch.
// TODO: Executor-placed orders (via NATS decisions) bypass the API and don't carry
// session_id. Those positions won't be counted here until the order-executor MCP tool
// accepts a session_id parameter.
func (db *DB) AggregateSessionStats(ctx context.Context, sessionID uuid.UUID) error {
	query := `
		UPDATE trading_sessions SET
			total_trades = COALESCE((
				SELECT COUNT(*) FROM positions WHERE session_id = $1 AND exit_time IS NOT NULL
			), 0),
			total_pnl = COALESCE((
				SELECT SUM(realized_pnl) FROM positions WHERE session_id = $1 AND exit_time IS NOT NULL
			), 0),
			winning_trades = COALESCE((
				SELECT COUNT(*) FROM positions WHERE session_id = $1 AND exit_time IS NOT NULL AND realized_pnl > 0
			), 0),
			losing_trades = COALESCE((
				SELECT COUNT(*) FROM positions WHERE session_id = $1 AND exit_time IS NOT NULL AND realized_pnl < 0
			), 0),
			updated_at = NOW()
		WHERE id = $1
	`
	result, err := db.pool.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to aggregate session stats: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("session %s not found", sessionID)
	}
	return nil
}

// CleanupStaleOrders marks old NEW orders as CANCELED if they've been stuck
// for longer than the given duration. This handles orphaned tracking records
// from the API handler that were never executed.
// Only orders belonging to the given session are affected (session-scoped).
func (db *DB) CleanupStaleOrders(ctx context.Context, sessionID uuid.UUID, olderThan time.Duration) (int64, error) {
	query := `
		UPDATE orders SET
			status = 'CANCELED',
			canceled_at = NOW(),
			error_message = 'Cleaned up: stuck in NEW status',
			updated_at = NOW()
		WHERE status = 'NEW'
		AND session_id = $1
		AND placed_at < NOW() - $2
	`
	// pgx v5 natively maps time.Duration to PostgreSQL interval — no string conversion needed.
	result, err := db.pool.Exec(ctx, query, sessionID, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup stale orders: %w", err)
	}
	return result.RowsAffected(), nil
}

// SessionStats holds session statistics
type SessionStats struct {
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	TotalPnL      float64
	MaxDrawdown   float64
	SharpeRatio   *float64
}
