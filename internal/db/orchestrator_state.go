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
// Uses database-level locking to prevent race conditions on concurrent pause operations
func (db *DB) SetOrchestratorPaused(ctx context.Context, pausedBy, pauseReason string) error {
	// Begin transaction for atomic state update with locking
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if commit not called

	// Lock the current state row with SELECT FOR UPDATE to prevent concurrent modifications
	// This ensures serialized access to state changes even across multiple orchestrator instances
	lockQuery := `
		SELECT id, paused
		FROM orchestrator_state
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE
	`

	var currentID int
	var currentPaused bool
	err = tx.QueryRow(ctx, lockQuery).Scan(&currentID, &currentPaused)
	if err != nil {
		return fmt.Errorf("failed to lock current state: %w", err)
	}

	// Validate state transition: can only pause if currently not paused
	if currentPaused {
		return fmt.Errorf("trading is already paused (current state locked)")
	}

	// Insert new state record with paused=true
	insertQuery := `
		INSERT INTO orchestrator_state (paused, paused_at, paused_by, pause_reason, updated_at, created_at)
		VALUES (TRUE, NOW(), $1, $2, NOW(), NOW())
	`

	_, err = tx.Exec(ctx, insertQuery, pausedBy, pauseReason)
	if err != nil {
		return fmt.Errorf("failed to insert paused state: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit pause state: %w", err)
	}

	return nil
}

// SetOrchestratorResumed updates the orchestrator state to resumed (not paused)
// Uses database-level locking to prevent race conditions on concurrent resume operations
func (db *DB) SetOrchestratorResumed(ctx context.Context) error {
	// Begin transaction for atomic state update with locking
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if commit not called

	// Lock the current state row with SELECT FOR UPDATE to prevent concurrent modifications
	// This ensures serialized access to state changes even across multiple orchestrator instances
	lockQuery := `
		SELECT id, paused
		FROM orchestrator_state
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE
	`

	var currentID int
	var currentPaused bool
	err = tx.QueryRow(ctx, lockQuery).Scan(&currentID, &currentPaused)
	if err != nil {
		return fmt.Errorf("failed to lock current state: %w", err)
	}

	// Validate state transition: can only resume if currently paused
	if !currentPaused {
		return fmt.Errorf("trading is not paused (current state locked)")
	}

	// Insert new state record with paused=false
	insertQuery := `
		INSERT INTO orchestrator_state (paused, resumed_at, updated_at, created_at)
		VALUES (FALSE, NOW(), NOW(), NOW())
	`

	_, err = tx.Exec(ctx, insertQuery)
	if err != nil {
		return fmt.Errorf("failed to insert resumed state: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit resume state: %w", err)
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
