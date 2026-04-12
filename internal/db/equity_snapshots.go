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

// InsertEquitySnapshotTx writes an equity snapshot inside an existing
// transaction. DB-008 (#132): the paper-trade handler previously called
// InsertEquitySnapshot outside the trade transaction, creating a torn-
// write window where the snapshot could read concurrently-modified
// session state. This Tx variant lets the snapshot participate in the
// same RepeatableRead transaction as the trade itself.
func (db *DB) InsertEquitySnapshotTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, equity, realizedPnL, unrealizedPnL float64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO equity_snapshots (session_id, equity, realized_pnl, unrealized_pnl)
		VALUES ($1, $2, $3, $4)
	`, sessionID, equity, realizedPnL, unrealizedPnL)
	return err
}

// ListEquitySnapshots returns the most-recent [limit] equity snapshots for the given
// session, in chronological order (oldest-to-newest among the returned records).
// When limit=0 all snapshots are returned (no LIMIT clause is added; LIMIT 0 in
// PostgreSQL returns zero rows, not all rows). Note: because we fetch with ORDER BY DESC,
// only the TAIL of the session's history is returned when limit>0, not the oldest rows.
// Use this for recent-trend calculations (drawdown, Sharpe on recent returns).
// Returns an empty slice (not nil) when no snapshots exist.
func (db *DB) ListEquitySnapshots(ctx context.Context, sessionID uuid.UUID, limit int) ([]*EquitySnapshot, error) {
	// most-recent N rows fetched in reverse-chronological order then reversed to chronological.
	query := `
		SELECT id, session_id, equity, realized_pnl, unrealized_pnl, recorded_at
		FROM equity_snapshots
		WHERE session_id = $1
		ORDER BY recorded_at DESC`
	args := []interface{}{sessionID}
	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}

	rows, err := db.pool.Query(ctx, query, args...)
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

// GetSessionIDWithMostSnapshots returns the session ID that has the most equity snapshot
// data points (highest COUNT(*)) among the provided session IDs. This allows callers to
// find the richest data set for drawdown/Sharpe computation with a single round-trip
// instead of issuing one query per session (N+1).
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
		ORDER BY COUNT(*) DESC, MAX(recorded_at) DESC
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
