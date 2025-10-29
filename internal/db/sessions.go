package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	ID             uuid.UUID
	Mode           TradingMode
	Symbol         string
	Exchange       string
	StartedAt      time.Time
	StoppedAt      *time.Time
	InitialCapital float64
	FinalCapital   *float64
	TotalTrades    int
	WinningTrades  int
	LosingTrades   int
	TotalPnL       float64
	MaxDrawdown    float64
	SharpeRatio    *float64
	Config         map[string]interface{}
	Metadata       map[string]interface{}
	CreatedAt      time.Time
	UpdatedAt      time.Time
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

// SessionStats holds session statistics
type SessionStats struct {
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	TotalPnL      float64
	MaxDrawdown   float64
	SharpeRatio   *float64
}
