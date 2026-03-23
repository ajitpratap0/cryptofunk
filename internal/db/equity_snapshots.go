package db

import (
	"context"
	"time"

	"github.com/google/uuid"
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
func (db *DB) ListEquitySnapshots(ctx context.Context, sessionID uuid.UUID, limit int) ([]*EquitySnapshot, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, session_id, equity, realized_pnl, unrealized_pnl, recorded_at
		FROM equity_snapshots
		WHERE session_id = $1
		ORDER BY recorded_at ASC
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
	return snapshots, rows.Err()
}

// GetSessionIDWithMostSnapshots returns the session ID that has the highest number of
// equity snapshots among the provided session IDs. This allows callers to find the
// richest data set for drawdown/Sharpe computation with a single round-trip instead of
// issuing one query per session (N+1).
// Returns ("", nil) when sessionIDs is empty.
func (db *DB) GetSessionIDWithMostSnapshots(ctx context.Context, sessionIDs []string) (string, error) {
	if len(sessionIDs) == 0 {
		return "", nil
	}
	var sessionID string
	err := db.pool.QueryRow(ctx, `
		SELECT session_id
		FROM equity_snapshots
		WHERE session_id = ANY($1)
		GROUP BY session_id
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, sessionIDs).Scan(&sessionID)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}
