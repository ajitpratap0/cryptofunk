package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"

	"github.com/ajitpratap0/cryptofunk/internal/risk"
	"github.com/ajitpratap0/cryptofunk/internal/vault"
)

// DB wraps the PostgreSQL connection pool
type DB struct {
	pool           *pgxpool.Pool
	circuitBreaker *risk.CircuitBreakerManager
}

// New creates a new database connection pool.
// It first tries to get credentials from Vault, then falls back to DATABASE_URL env var.
func New(ctx context.Context) (*DB, error) {
	var databaseURL string

	// Try to get database URL from Vault first
	if vaultClient, err := vault.NewClientFromEnv(); err == nil {
		if dbConfig, err := vaultClient.GetDatabaseConfig(ctx); err == nil {
			databaseURL = dbConfig.ConnectionString()
			log.Info().Msg("Database credentials loaded from Vault")
		} else {
			log.Debug().Err(err).Msg("Could not load database config from Vault, falling back to env")
		}
	}

	// Fall back to environment variable
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}

	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set and Vault credentials not available")
	}

	// Configure connection pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Set pool configuration
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().Msg("Database connection pool created successfully")

	return &DB{
		pool:           pool,
		circuitBreaker: risk.NewCircuitBreakerManager(),
	}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
		log.Info().Msg("Database connection pool closed")
	}
}

// Ping checks the database connection
func (db *DB) Ping(ctx context.Context) error {
	if db.pool == nil {
		return fmt.Errorf("database connection pool is nil")
	}
	return db.pool.Ping(ctx)
}

// Pool returns the underlying connection pool
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Health checks database connectivity
func (db *DB) Health(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// SetPool sets the connection pool (used by tests)
func (db *DB) SetPool(pool *pgxpool.Pool) {
	db.pool = pool
}

// ExecuteWithCircuitBreaker executes a database operation with circuit breaker protection
// This wraps database calls to prevent cascading failures during database outages
func (db *DB) ExecuteWithCircuitBreaker(operation func() (interface{}, error)) (interface{}, error) {
	if db.circuitBreaker == nil {
		// Fallback if circuit breaker is not initialized
		return operation()
	}

	result, err := db.circuitBreaker.Database().Execute(operation)
	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			db.circuitBreaker.Metrics().RecordRequest("database", false)
			return nil, fmt.Errorf("database circuit breaker is open, service unavailable")
		}
		db.circuitBreaker.Metrics().RecordRequest("database", false)
		return nil, err
	}

	db.circuitBreaker.Metrics().RecordRequest("database", true)
	return result, nil
}

// GetCircuitBreaker returns the circuit breaker manager for this database
// This allows external code to use the same circuit breaker instance
func (db *DB) GetCircuitBreaker() *risk.CircuitBreakerManager {
	return db.circuitBreaker
}

// SetCircuitBreaker sets a custom circuit breaker manager
// This is useful for sharing circuit breakers across components
func (db *DB) SetCircuitBreaker(cb *risk.CircuitBreakerManager) {
	db.circuitBreaker = cb
}
