package testhelpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer holds the testcontainer instance and connection details
type PostgresContainer struct {
	Container     *postgres.PostgresContainer
	ConnectionStr string
	DB            *db.DB
	cleanupFuncs  []func()
	t             *testing.T
}

// SetupTestDatabase creates a PostgreSQL testcontainer with TimescaleDB and pgvector
func SetupTestDatabase(t *testing.T) *PostgresContainer {
	t.Helper()

	ctx := context.Background()

	// Create PostgreSQL container with TimescaleDB image (includes pgvector)
	container, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg15", // TimescaleDB with PostgreSQL 15
		postgres.WithDatabase("cryptofunk_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("testpassword"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// Get connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Create test database connection
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to parse connection string: %v", err)
	}

	// Configure connection pool
	config.MaxConns = 5
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		container.Terminate(ctx)
		t.Fatalf("Failed to ping database: %v", err)
	}

	database := &db.DB{}
	database.SetPool(pool)

	tc := &PostgresContainer{
		Container:     container,
		ConnectionStr: connStr,
		DB:            database,
		cleanupFuncs:  []func(){},
		t:             t,
	}

	// Set up cleanup
	t.Cleanup(func() {
		tc.Cleanup()
	})

	return tc
}

// ApplyMigrations runs SQL migrations from the migrations directory
func (tc *PostgresContainer) ApplyMigrations(migrationsPath string) error {
	tc.t.Helper()

	ctx := context.Background()
	pool := tc.DB.Pool()

	// Read all migration files in order
	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	// Sort files to ensure they run in order (001, 002, 003, etc.)
	// This works because files are named with numeric prefixes
	sort := func(i, j int) bool {
		return filepath.Base(files[i]) < filepath.Base(files[j])
	}

	// Simple bubble sort for the file list
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if !sort(i, j) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Apply each migration in order
	for _, migrationFile := range files {
		tc.t.Logf("Applying migration: %s", filepath.Base(migrationFile))

		sqlBytes, err := os.ReadFile(migrationFile)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migrationFile, err)
		}

		schema := string(sqlBytes)

		// Execute schema
		_, err = pool.Exec(ctx, schema)
		if err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filepath.Base(migrationFile), err)
		}
	}

	return nil
}

// ApplyMigrationsLegacy provides a minimal schema if migration file is not available
func (tc *PostgresContainer) ApplyMigrationsLegacy() error {
	tc.t.Helper()

	ctx := context.Background()
	pool := tc.DB.Pool()

	schema := `
-- Enable extensions
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS vector;

-- Trading sessions table
CREATE TABLE IF NOT EXISTS trading_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mode TEXT NOT NULL,
    symbol TEXT NOT NULL,
    exchange TEXT NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    stopped_at TIMESTAMP WITH TIME ZONE,
    initial_capital DECIMAL(20, 8) NOT NULL,
    final_capital DECIMAL(20, 8),
    total_trades INTEGER DEFAULT 0,
    winning_trades INTEGER DEFAULT 0,
    losing_trades INTEGER DEFAULT 0,
    total_pnl DECIMAL(20, 8) DEFAULT 0,
    max_drawdown DECIMAL(20, 8) DEFAULT 0,
    sharpe_ratio DECIMAL(10, 4),
    config JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES trading_sessions(id),
    position_id UUID,
    exchange_order_id TEXT,
    symbol TEXT NOT NULL,
    exchange TEXT NOT NULL DEFAULT 'binance',
    side TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'NEW',
    price DECIMAL(20, 8),
    stop_price DECIMAL(20, 8),
    quantity DECIMAL(20, 8) NOT NULL,
    executed_quantity DECIMAL(20, 8) DEFAULT 0,
    executed_quote_quantity DECIMAL(20, 8) DEFAULT 0,
    time_in_force TEXT,
    placed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    filled_at TIMESTAMP WITH TIME ZONE,
    canceled_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Trades table
CREATE TABLE IF NOT EXISTS trades (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID REFERENCES orders(id),
    exchange_trade_id TEXT,
    symbol TEXT NOT NULL,
    exchange TEXT NOT NULL DEFAULT 'binance',
    side TEXT NOT NULL,
    price DECIMAL(20, 8) NOT NULL,
    quantity DECIMAL(20, 8) NOT NULL,
    quote_quantity DECIMAL(20, 8) NOT NULL DEFAULT 0,
    commission DECIMAL(20, 8) DEFAULT 0,
    commission_asset TEXT,
    executed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    is_maker BOOLEAN DEFAULT false,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Positions table
CREATE TABLE IF NOT EXISTS positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES trading_sessions(id),
    symbol TEXT NOT NULL,
    exchange TEXT NOT NULL DEFAULT 'binance',
    side TEXT NOT NULL,
    entry_price DECIMAL(20, 8) NOT NULL,
    exit_price DECIMAL(20, 8),
    quantity DECIMAL(20, 8) NOT NULL,
    entry_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    exit_time TIMESTAMP WITH TIME ZONE,
    stop_loss DECIMAL(20, 8),
    take_profit DECIMAL(20, 8),
    realized_pnl DECIMAL(20, 8),
    unrealized_pnl DECIMAL(20, 8),
    fees DECIMAL(20, 8) DEFAULT 0,
    entry_reason TEXT,
    exit_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Agent status table
CREATE TABLE IF NOT EXISTS agent_status (
    name TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_healthy BOOLEAN DEFAULT true,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Agent signals table
CREATE TABLE IF NOT EXISTS agent_signals (
    id BIGSERIAL PRIMARY KEY,
    agent_name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    signal_type TEXT NOT NULL,
    action TEXT NOT NULL,
    confidence DECIMAL(5, 4) NOT NULL,
    reasoning TEXT,
    metadata JSONB,
    context JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- LLM decisions table
CREATE TABLE IF NOT EXISTS llm_decisions (
    id BIGSERIAL PRIMARY KEY,
    agent_name TEXT NOT NULL,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens_used INTEGER,
    latency_ms INTEGER,
    decision_type TEXT,
    confidence DECIMAL(5, 4),
    metadata JSONB,
    prompt_embedding vector(1536),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Candlesticks table (hypertable)
CREATE TABLE IF NOT EXISTS candlesticks (
    time TIMESTAMP WITH TIME ZONE NOT NULL,
    symbol TEXT NOT NULL,
    exchange TEXT NOT NULL,
    timeframe TEXT NOT NULL,
    open DECIMAL(20, 8) NOT NULL,
    high DECIMAL(20, 8) NOT NULL,
    low DECIMAL(20, 8) NOT NULL,
    close DECIMAL(20, 8) NOT NULL,
    volume DECIMAL(20, 8) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance metrics table (hypertable)
CREATE TABLE IF NOT EXISTS performance_metrics (
    time TIMESTAMP WITH TIME ZONE NOT NULL,
    session_id UUID REFERENCES trading_sessions(id),
    metric_name TEXT NOT NULL,
    metric_value DECIMAL(20, 8) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Convert candlesticks to hypertable
SELECT create_hypertable('candlesticks', 'time', if_not_exists => TRUE);

-- Convert performance_metrics to hypertable
SELECT create_hypertable('performance_metrics', 'time', if_not_exists => TRUE);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_orders_session_id ON orders(session_id);
CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders(symbol);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_trades_order_id ON trades(order_id);
CREATE INDEX IF NOT EXISTS idx_trades_session_id ON trades(session_id);
CREATE INDEX IF NOT EXISTS idx_positions_session_id ON positions(session_id);
CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol);
CREATE INDEX IF NOT EXISTS idx_agent_signals_agent_name ON agent_signals(agent_name);
CREATE INDEX IF NOT EXISTS idx_agent_signals_symbol ON agent_signals(symbol);
CREATE INDEX IF NOT EXISTS idx_llm_decisions_agent_name ON llm_decisions(agent_name);
CREATE INDEX IF NOT EXISTS idx_candlesticks_symbol_time ON candlesticks(symbol, time DESC);
`

	// Execute schema
	_, err := pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// AddCleanup registers a cleanup function to be called during teardown
func (tc *PostgresContainer) AddCleanup(fn func()) {
	tc.cleanupFuncs = append(tc.cleanupFuncs, fn)
}

// Cleanup terminates the container and runs cleanup functions
func (tc *PostgresContainer) Cleanup() {
	ctx := context.Background()

	// Run cleanup functions in reverse order
	for i := len(tc.cleanupFuncs) - 1; i >= 0; i-- {
		tc.cleanupFuncs[i]()
	}

	// Close database connection
	if tc.DB != nil {
		tc.DB.Close()
	}

	// Terminate container
	if tc.Container != nil {
		if err := tc.Container.Terminate(ctx); err != nil {
			tc.t.Logf("Failed to terminate container: %v", err)
		}
	}
}

// TruncateAllTables clears all data from tables (useful for test isolation)
func (tc *PostgresContainer) TruncateAllTables() error {
	ctx := context.Background()
	pool := tc.DB.Pool()

	tables := []string{
		"trades",
		"orders",
		"positions",
		"agent_signals",
		"llm_decisions",
		"performance_metrics",
		"candlesticks",
		"agent_status",
		"trading_sessions",
	}

	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
	}

	return nil
}

// ExecuteSQL executes arbitrary SQL (useful for test setup)
func (tc *PostgresContainer) ExecuteSQL(sql string) error {
	ctx := context.Background()
	pool := tc.DB.Pool()

	_, err := pool.Exec(ctx, sql)
	return err
}
