package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/strategy"
)

// StrategyRepository handles database operations for strategies
type StrategyRepository struct {
	db *DB
}

// NewStrategyRepository creates a new strategy repository
func NewStrategyRepository(db *DB) *StrategyRepository {
	return &StrategyRepository{db: db}
}

// StrategyRecord represents a strategy record in the database
type StrategyRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	SchemaVersion string    `json:"schema_version"`
	Config        []byte    `json:"config"`
	IsActive      bool      `json:"is_active"`
	Author        string    `json:"author"`
	Version       string    `json:"version"`
	Tags          []string  `json:"tags"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Save creates or updates a strategy in the database
func (r *StrategyRepository) Save(ctx context.Context, s *strategy.StrategyConfig) error {
	if r.db == nil || r.db.pool == nil {
		return fmt.Errorf("database connection not available")
	}

	configJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy config: %w", err)
	}

	// Generate ID if not present
	if s.Metadata.ID == "" {
		s.Metadata.ID = uuid.New().String()
	}

	query := `
		INSERT INTO strategies (id, name, description, schema_version, config, is_active, author, version, tags, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			schema_version = EXCLUDED.schema_version,
			config = EXCLUDED.config,
			is_active = EXCLUDED.is_active,
			author = EXCLUDED.author,
			version = EXCLUDED.version,
			tags = EXCLUDED.tags,
			source = EXCLUDED.source,
			updated_at = EXCLUDED.updated_at
	`

	_, err = r.db.pool.Exec(ctx, query,
		s.Metadata.ID,
		s.Metadata.Name,
		s.Metadata.Description,
		s.Metadata.SchemaVersion,
		configJSON,
		false, // is_active - set separately
		s.Metadata.Author,
		s.Metadata.Version,
		s.Metadata.Tags,
		s.Metadata.Source,
		s.Metadata.CreatedAt,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save strategy: %w", err)
	}

	log.Debug().
		Str("strategy_id", s.Metadata.ID).
		Str("strategy_name", s.Metadata.Name).
		Msg("Strategy saved to database")

	return nil
}

// GetByID retrieves a strategy by its ID
func (r *StrategyRepository) GetByID(ctx context.Context, id string) (*strategy.StrategyConfig, error) {
	if r.db == nil || r.db.pool == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	query := `SELECT config FROM strategies WHERE id = $1`

	var configJSON []byte
	err := r.db.pool.QueryRow(ctx, query, id).Scan(&configJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("strategy not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get strategy: %w", err)
	}

	var s strategy.StrategyConfig
	if err := json.Unmarshal(configJSON, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategy config: %w", err)
	}

	return &s, nil
}

// GetActive retrieves the currently active strategy
func (r *StrategyRepository) GetActive(ctx context.Context) (*strategy.StrategyConfig, error) {
	if r.db == nil || r.db.pool == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	query := `SELECT config FROM strategies WHERE is_active = TRUE LIMIT 1`

	var configJSON []byte
	err := r.db.pool.QueryRow(ctx, query).Scan(&configJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No active strategy
		}
		return nil, fmt.Errorf("failed to get active strategy: %w", err)
	}

	var s strategy.StrategyConfig
	if err := json.Unmarshal(configJSON, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategy config: %w", err)
	}

	return &s, nil
}

// SetActive sets a strategy as the active strategy
func (r *StrategyRepository) SetActive(ctx context.Context, id string) error {
	if r.db == nil || r.db.pool == nil {
		return fmt.Errorf("database connection not available")
	}

	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Deactivate all strategies first
	_, err = tx.Exec(ctx, `UPDATE strategies SET is_active = FALSE WHERE is_active = TRUE`)
	if err != nil {
		return fmt.Errorf("failed to deactivate strategies: %w", err)
	}

	// Activate the specified strategy
	result, err := tx.Exec(ctx, `UPDATE strategies SET is_active = TRUE, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to activate strategy: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("strategy not found: %s", id)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("strategy_id", id).
		Msg("Strategy set as active")

	return nil
}

// SaveAndActivate saves a strategy and sets it as active
func (r *StrategyRepository) SaveAndActivate(ctx context.Context, s *strategy.StrategyConfig) error {
	if r.db == nil || r.db.pool == nil {
		return fmt.Errorf("database connection not available")
	}

	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	configJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy config: %w", err)
	}

	// Generate ID if not present
	if s.Metadata.ID == "" {
		s.Metadata.ID = uuid.New().String()
	}

	// Deactivate all strategies first
	_, err = tx.Exec(ctx, `UPDATE strategies SET is_active = FALSE WHERE is_active = TRUE`)
	if err != nil {
		return fmt.Errorf("failed to deactivate strategies: %w", err)
	}

	// Insert or update the strategy as active
	query := `
		INSERT INTO strategies (id, name, description, schema_version, config, is_active, author, version, tags, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			schema_version = EXCLUDED.schema_version,
			config = EXCLUDED.config,
			is_active = TRUE,
			author = EXCLUDED.author,
			version = EXCLUDED.version,
			tags = EXCLUDED.tags,
			source = EXCLUDED.source,
			updated_at = EXCLUDED.updated_at
	`

	_, err = tx.Exec(ctx, query,
		s.Metadata.ID,
		s.Metadata.Name,
		s.Metadata.Description,
		s.Metadata.SchemaVersion,
		configJSON,
		s.Metadata.Author,
		s.Metadata.Version,
		s.Metadata.Tags,
		s.Metadata.Source,
		s.Metadata.CreatedAt,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save and activate strategy: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("strategy_id", s.Metadata.ID).
		Str("strategy_name", s.Metadata.Name).
		Msg("Strategy saved and activated")

	return nil
}

// List retrieves all strategies with optional filtering
func (r *StrategyRepository) List(ctx context.Context, limit, offset int) ([]*strategy.StrategyConfig, error) {
	if r.db == nil || r.db.pool == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT config FROM strategies
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategies: %w", err)
	}
	defer rows.Close()

	var strategies []*strategy.StrategyConfig
	for rows.Next() {
		var configJSON []byte
		if err := rows.Scan(&configJSON); err != nil {
			return nil, fmt.Errorf("failed to scan strategy: %w", err)
		}

		var s strategy.StrategyConfig
		if err := json.Unmarshal(configJSON, &s); err != nil {
			return nil, fmt.Errorf("failed to unmarshal strategy: %w", err)
		}
		strategies = append(strategies, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating strategies: %w", err)
	}

	return strategies, nil
}

// Delete removes a strategy from the database
func (r *StrategyRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.pool == nil {
		return fmt.Errorf("database connection not available")
	}

	result, err := r.db.pool.Exec(ctx, `DELETE FROM strategies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete strategy: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("strategy not found: %s", id)
	}

	log.Info().
		Str("strategy_id", id).
		Msg("Strategy deleted from database")

	return nil
}

// SaveHistory saves a version of the strategy to history
func (r *StrategyRepository) SaveHistory(ctx context.Context, strategyID string, s *strategy.StrategyConfig, changedBy, reason string) error {
	if r.db == nil || r.db.pool == nil {
		return fmt.Errorf("database connection not available")
	}

	configJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy config: %w", err)
	}

	query := `
		INSERT INTO strategy_history (strategy_id, config, version, changed_by, change_reason)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = r.db.pool.Exec(ctx, query, strategyID, configJSON, s.Metadata.Version, changedBy, reason)
	if err != nil {
		return fmt.Errorf("failed to save strategy history: %w", err)
	}

	return nil
}

// GetHistory retrieves version history for a strategy
func (r *StrategyRepository) GetHistory(ctx context.Context, strategyID string, limit int) ([]*strategy.StrategyConfig, error) {
	if r.db == nil || r.db.pool == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT config FROM strategy_history
		WHERE strategy_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.pool.Query(ctx, query, strategyID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy history: %w", err)
	}
	defer rows.Close()

	var history []*strategy.StrategyConfig
	for rows.Next() {
		var configJSON []byte
		if err := rows.Scan(&configJSON); err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}

		var s strategy.StrategyConfig
		if err := json.Unmarshal(configJSON, &s); err != nil {
			return nil, fmt.Errorf("failed to unmarshal history: %w", err)
		}
		history = append(history, &s)
	}

	return history, nil
}
