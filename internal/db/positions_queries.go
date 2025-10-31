package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListPositions returns positions with optional filters
func (db *DB) ListPositions(ctx context.Context, sessionID *uuid.UUID, symbol *string, openOnly bool, limit int, offset int) ([]*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	if sessionID != nil {
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, sessionID)
		argCount++
	}

	if symbol != nil {
		query += fmt.Sprintf(" AND symbol = $%d", argCount)
		args = append(args, *symbol)
		argCount++
	}

	if openOnly {
		query += " AND exit_time IS NULL"
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
		argCount++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query positions: %w", err)
	}
	defer rows.Close()

	var positions []*Position
	for rows.Next() {
		pos := &Position{}
		err := rows.Scan(
			&pos.ID,
			&pos.SessionID,
			&pos.Symbol,
			&pos.Exchange,
			&pos.Side,
			&pos.EntryPrice,
			&pos.ExitPrice,
			&pos.Quantity,
			&pos.EntryTime,
			&pos.ExitTime,
			&pos.StopLoss,
			&pos.TakeProfit,
			&pos.RealizedPnL,
			&pos.UnrealizedPnL,
			&pos.Fees,
			&pos.EntryReason,
			&pos.ExitReason,
			&pos.Metadata,
			&pos.CreatedAt,
			&pos.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, pos)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating positions: %w", err)
	}

	return positions, nil
}

// GetPositionBySymbol returns an open position for a specific symbol and session
func (db *DB) GetPositionBySymbol(ctx context.Context, sessionID uuid.UUID, symbol string) (*Position, error) {
	positions, err := db.ListPositions(ctx, &sessionID, &symbol, true, 1, 0)
	if err != nil {
		return nil, err
	}

	if len(positions) == 0 {
		return nil, fmt.Errorf("no open position found for symbol %s", symbol)
	}

	return positions[0], nil
}

// CountPositions returns the total count of positions matching the criteria
func (db *DB) CountPositions(ctx context.Context, sessionID *uuid.UUID, openOnly bool) (int, error) {
	query := "SELECT COUNT(*) FROM positions WHERE 1=1"

	args := []interface{}{}
	argCount := 1

	if sessionID != nil {
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, sessionID)
		argCount++
	}

	if openOnly {
		query += " AND exit_time IS NULL"
	}

	var count int
	err := db.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count positions: %w", err)
	}

	return count, nil
}
