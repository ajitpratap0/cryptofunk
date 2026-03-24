package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EquitySnapshot records account equity at a point in time.
// One row is written after each paper trade (open or close).
type EquitySnapshot struct {
	ID            uuid.UUID `db:"id"`
	SessionID     uuid.UUID `db:"session_id"`
	Equity        float64   `db:"equity"`
	RealizedPnL   float64   `db:"realized_pnl"`
	UnrealizedPnL float64   `db:"unrealized_pnl"`
	RecordedAt    time.Time `db:"recorded_at"`
}

// InsertEquitySnapshot writes a new equity snapshot for the given session.
// Best-effort: callers should log on error but not fail the trade.
func (db *DB) InsertEquitySnapshot(ctx context.Context, sessionID uuid.UUID, equity, realizedPnL, unrealizedPnL float64) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO equity_snapshots (session_id, equity, realized_pnl, unrealized_pnl)
		VALUES ($1, $2, $3, $4)
	`, sessionID, equity, realizedPnL, unrealizedPnL)
	return err
}

// ListEquitySnapshots returns up to limit snapshots for the session, oldest-first.
// Returns an empty slice (not nil) when no snapshots exist.
// Internally queries DESC to select the most recent N rows, then reverses the result
// so callers always receive chronological (ASC) order — critical for drawdown/Sharpe calculations.
func (db *DB) ListEquitySnapshots(ctx context.Context, sessionID uuid.UUID, limit int) ([]*EquitySnapshot, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, session_id, equity, realized_pnl, unrealized_pnl, recorded_at
		FROM equity_snapshots
		WHERE session_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snapshots := make([]*EquitySnapshot, 0)
	for rows.Next() {
		s := &EquitySnapshot{}
		if err := rows.Scan(&s.ID, &s.SessionID, &s.Equity, &s.RealizedPnL, &s.UnrealizedPnL, &s.RecordedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	// Reverse so the caller receives chronological (oldest-first) order.
	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}
	return snapshots, rows.Err()
}

// GetSessionIDWithMostSnapshots returns the session ID that has the highest number of
// equity snapshots among the provided session IDs. This allows callers to find the
// richest data set for drawdown/Sharpe computation with a single round-trip instead of
// issuing one query per session (N+1).
// Returns (uuid.UUID{}, nil) when sessionIDs is empty.
func (db *DB) GetSessionIDWithMostSnapshots(ctx context.Context, sessionIDs []uuid.UUID) (uuid.UUID, error) {
	if len(sessionIDs) == 0 {
		return uuid.UUID{}, nil
	}
	var sessionID uuid.UUID
	err := db.pool.QueryRow(ctx, `
		SELECT session_id
		FROM equity_snapshots
		WHERE session_id = ANY($1::uuid[])
		GROUP BY session_id
		ORDER BY MAX(recorded_at) DESC
		LIMIT 1
	`, sessionIDs).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, nil // no snapshots yet — normal initial state
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return sessionID, nil
}
