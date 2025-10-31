package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PositionSide represents the side of a position
type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
	PositionSideFlat  PositionSide = "FLAT"
)

// Position represents a trading position
type Position struct {
	ID            uuid.UUID    `db:"id"`
	SessionID     *uuid.UUID   `db:"session_id"`
	Symbol        string       `db:"symbol"`
	Exchange      string       `db:"exchange"`
	Side          PositionSide `db:"side"`
	EntryPrice    float64      `db:"entry_price"`
	ExitPrice     *float64     `db:"exit_price"`
	Quantity      float64      `db:"quantity"`
	EntryTime     time.Time    `db:"entry_time"`
	ExitTime      *time.Time   `db:"exit_time"`
	StopLoss      *float64     `db:"stop_loss"`
	TakeProfit    *float64     `db:"take_profit"`
	RealizedPnL   *float64     `db:"realized_pnl"`
	UnrealizedPnL *float64     `db:"unrealized_pnl"`
	Fees          float64      `db:"fees"`
	EntryReason   *string      `db:"entry_reason"`
	ExitReason    *string      `db:"exit_reason"`
	Metadata      interface{}  `db:"metadata"`
	CreatedAt     time.Time    `db:"created_at"`
	UpdatedAt     time.Time    `db:"updated_at"`
}

// CreatePosition inserts a new position into the database
func (db *DB) CreatePosition(ctx context.Context, position *Position) error {
	query := `
		INSERT INTO positions (
			id, session_id, symbol, exchange, side, entry_price, quantity,
			entry_time, stop_loss, take_profit, entry_reason, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	if position.ID == uuid.Nil {
		position.ID = uuid.New()
	}
	if position.CreatedAt.IsZero() {
		position.CreatedAt = time.Now()
	}
	if position.UpdatedAt.IsZero() {
		position.UpdatedAt = time.Now()
	}

	_, err := db.pool.Exec(ctx, query,
		position.ID,
		position.SessionID,
		position.Symbol,
		position.Exchange,
		position.Side,
		position.EntryPrice,
		position.Quantity,
		position.EntryTime,
		position.StopLoss,
		position.TakeProfit,
		position.EntryReason,
		position.Metadata,
		position.CreatedAt,
		position.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create position: %w", err)
	}

	return nil
}

// UpdatePosition updates an existing position
func (db *DB) UpdatePosition(ctx context.Context, position *Position) error {
	query := `
		UPDATE positions
		SET
			exit_price = $2,
			exit_time = $3,
			realized_pnl = $4,
			unrealized_pnl = $5,
			fees = $6,
			exit_reason = $7,
			metadata = $8,
			updated_at = $9
		WHERE id = $1
	`

	position.UpdatedAt = time.Now()

	result, err := db.pool.Exec(ctx, query,
		position.ID,
		position.ExitPrice,
		position.ExitTime,
		position.RealizedPnL,
		position.UnrealizedPnL,
		position.Fees,
		position.ExitReason,
		position.Metadata,
		position.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update position: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("position not found: %s", position.ID)
	}

	return nil
}

// GetPosition retrieves a position by ID
func (db *DB) GetPosition(ctx context.Context, id uuid.UUID) (*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE id = $1
	`

	var position Position
	err := db.pool.QueryRow(ctx, query, id).Scan(
		&position.ID,
		&position.SessionID,
		&position.Symbol,
		&position.Exchange,
		&position.Side,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryTime,
		&position.ExitTime,
		&position.StopLoss,
		&position.TakeProfit,
		&position.RealizedPnL,
		&position.UnrealizedPnL,
		&position.Fees,
		&position.EntryReason,
		&position.ExitReason,
		&position.Metadata,
		&position.CreatedAt,
		&position.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("position not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get position: %w", err)
	}

	return &position, nil
}

// GetOpenPositions retrieves all open positions for a session
func (db *DB) GetOpenPositions(ctx context.Context, sessionID uuid.UUID) ([]*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE session_id = $1 AND exit_time IS NULL
		ORDER BY entry_time DESC
	`

	rows, err := db.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open positions: %w", err)
	}
	defer rows.Close()

	var positions []*Position
	for rows.Next() {
		var position Position
		err := rows.Scan(
			&position.ID,
			&position.SessionID,
			&position.Symbol,
			&position.Exchange,
			&position.Side,
			&position.EntryPrice,
			&position.ExitPrice,
			&position.Quantity,
			&position.EntryTime,
			&position.ExitTime,
			&position.StopLoss,
			&position.TakeProfit,
			&position.RealizedPnL,
			&position.UnrealizedPnL,
			&position.Fees,
			&position.EntryReason,
			&position.ExitReason,
			&position.Metadata,
			&position.CreatedAt,
			&position.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, &position)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating positions: %w", err)
	}

	return positions, nil
}

// ClosePosition closes a position with exit price and reason
func (db *DB) ClosePosition(ctx context.Context, id uuid.UUID, exitPrice float64, exitReason string, fees float64) error {
	// Get the position first to calculate realized P&L
	position, err := db.GetPosition(ctx, id)
	if err != nil {
		return err
	}

	if position.ExitTime != nil {
		return fmt.Errorf("position already closed: %s", id)
	}

	// Calculate realized P&L
	var realizedPnL float64
	if position.Side == PositionSideLong {
		// LONG: profit when exit price > entry price
		realizedPnL = (exitPrice - position.EntryPrice) * position.Quantity
	} else {
		// SHORT: profit when exit price < entry price
		realizedPnL = (position.EntryPrice - exitPrice) * position.Quantity
	}

	// Subtract fees
	realizedPnL -= fees

	// Update position
	now := time.Now()
	query := `
		UPDATE positions
		SET
			exit_price = $2,
			exit_time = $3,
			realized_pnl = $4,
			unrealized_pnl = 0,
			fees = fees + $5,
			exit_reason = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := db.pool.Exec(ctx, query,
		id,
		exitPrice,
		now,
		realizedPnL,
		fees,
		exitReason,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("position not found: %s", id)
	}

	return nil
}

// UpdateUnrealizedPnL updates the unrealized P&L for an open position
func (db *DB) UpdateUnrealizedPnL(ctx context.Context, id uuid.UUID, currentPrice float64) error {
	// Get the position
	position, err := db.GetPosition(ctx, id)
	if err != nil {
		return err
	}

	if position.ExitTime != nil {
		return fmt.Errorf("cannot update unrealized P&L for closed position: %s", id)
	}

	// Calculate unrealized P&L
	var unrealizedPnL float64
	if position.Side == PositionSideLong {
		unrealizedPnL = (currentPrice - position.EntryPrice) * position.Quantity
	} else {
		unrealizedPnL = (position.EntryPrice - currentPrice) * position.Quantity
	}

	// Update position
	query := `
		UPDATE positions
		SET
			unrealized_pnl = $2,
			updated_at = $3
		WHERE id = $1
	`

	_, err = db.pool.Exec(ctx, query, id, unrealizedPnL, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update unrealized P&L: %w", err)
	}

	return nil
}

// ConvertPositionSide converts a string to PositionSide
func ConvertPositionSide(side string) PositionSide {
	switch side {
	case "LONG", "long", "buy", "BUY":
		return PositionSideLong
	case "SHORT", "short", "sell", "SELL":
		return PositionSideShort
	default:
		return PositionSideFlat
	}
}
