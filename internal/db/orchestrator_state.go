package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// OrchestratorState represents the orchestrator's current state
type OrchestratorState struct {
	ID          int        `json:"id"`
	Paused      bool       `json:"paused"`
	PausedAt    *time.Time `json:"paused_at,omitempty"`
	ResumedAt   *time.Time `json:"resumed_at,omitempty"`
	PausedBy    *string    `json:"paused_by,omitempty"`
	PauseReason *string    `json:"pause_reason,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// GetOrchestratorState retrieves the current orchestrator state
// Returns the most recent state record
func (db *DB) GetOrchestratorState(ctx context.Context) (*OrchestratorState, error) {
	query := `
		SELECT id, paused, paused_at, resumed_at, paused_by, pause_reason, updated_at, created_at
		FROM orchestrator_state
		ORDER BY id DESC
		LIMIT 1
	`

	var state OrchestratorState
	err := db.pool.QueryRow(ctx, query).Scan(
		&state.ID,
		&state.Paused,
		&state.PausedAt,
		&state.ResumedAt,
		&state.PausedBy,
		&state.PauseReason,
		&state.UpdatedAt,
		&state.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// No state exists yet, return default (not paused)
			now := time.Now()
			return &OrchestratorState{
				ID:        0,
				Paused:    false,
				UpdatedAt: now,
				CreatedAt: now,
			}, nil
		}
		return nil, fmt.Errorf("failed to query orchestrator state: %w", err)
	}

	return &state, nil
}

// SetOrchestratorPaused updates the orchestrator state to paused
func (db *DB) SetOrchestratorPaused(ctx context.Context, pausedBy, pauseReason string) error {
	query := `
		INSERT INTO orchestrator_state (paused, paused_at, paused_by, pause_reason, updated_at, created_at)
		VALUES (TRUE, NOW(), $1, $2, NOW(), NOW())
	`

	_, err := db.pool.Exec(ctx, query, pausedBy, pauseReason)
	if err != nil {
		return fmt.Errorf("failed to set orchestrator paused: %w", err)
	}

	return nil
}

// SetOrchestratorResumed updates the orchestrator state to resumed (not paused)
func (db *DB) SetOrchestratorResumed(ctx context.Context) error {
	query := `
		INSERT INTO orchestrator_state (paused, resumed_at, updated_at, created_at)
		VALUES (FALSE, NOW(), NOW(), NOW())
	`

	_, err := db.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set orchestrator resumed: %w", err)
	}

	return nil
}

// IsTradingPaused checks if trading is currently paused
// This is a convenience method that returns just the boolean status
func (db *DB) IsTradingPaused(ctx context.Context) (bool, error) {
	state, err := db.GetOrchestratorState(ctx)
	if err != nil {
		return false, err
	}
	return state.Paused, nil
}
