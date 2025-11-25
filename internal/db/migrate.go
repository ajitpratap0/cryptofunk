// Package db provides database utilities including migration runner
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

// migrationsDir will be set by the caller
var migrationsDir string

// SetMigrationsDir sets the directory containing migration files
func SetMigrationsDir(dir string) {
	migrationsDir = dir
}

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	SQL         string
	Filename    string
}

// Migrator handles database migrations
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a new migration runner
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// ensureSchemaVersionTable creates the schema_version table if it doesn't exist
func (m *Migrator) ensureSchemaVersionTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW(),
			description TEXT
		);
	`
	_, err := m.db.ExecContext(ctx, query)
	return err
}

// getCurrentVersion returns the current schema version
func (m *Migrator) getCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// loadMigrations loads all migration files from the migrations directory
func (m *Migrator) loadMigrations() ([]Migration, error) {
	var migrations []Migration

	// Read directory entries
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, entry := range entries {
		// Skip directories, non-SQL files, and DOWN migrations
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		// Skip DOWN migration files (e.g., 001_initial_schema_down.sql)
		if strings.HasSuffix(entry.Name(), "_down.sql") {
			continue
		}

		// Read file content
		// Validate that the file path is within the migrations directory to prevent directory traversal
		filePath := filepath.Join(migrationsDir, entry.Name())
		cleanPath := filepath.Clean(filePath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(migrationsDir)) {
			return nil, fmt.Errorf("invalid migration file path: %s", entry.Name())
		}
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}

		filename := entry.Name()
		var version int
		var description string

		// Parse filename format: 001_description.sql
		if _, err := fmt.Sscanf(filename, "%d_%s", &version, &description); err != nil {
			return nil, fmt.Errorf("invalid migration filename format: %s (expected: NNN_description.sql)", filename)
		}

		// Extract description from filename (remove .sql extension)
		description = strings.TrimSuffix(description, ".sql")
		description = strings.ReplaceAll(description, "_", " ")

		migrations = append(migrations, Migration{
			Version:     version,
			Description: description,
			SQL:         string(content),
			Filename:    filename,
		})
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate(ctx context.Context) error {
	// Ensure schema_version table exists
	if err := m.ensureSchemaVersionTable(ctx); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	// Load all migrations
	migrations, err := m.loadMigrations()
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		fmt.Println("No migrations found")
		return nil
	}

	// Filter migrations that need to be applied
	pendingMigrations := []Migration{}
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	if len(pendingMigrations) == 0 {
		fmt.Printf("Database is up to date (version %d)\n", currentVersion)
		return nil
	}

	fmt.Printf("Current schema version: %d\n", currentVersion)
	fmt.Printf("Found %d pending migration(s)\n", len(pendingMigrations))

	// Apply each migration in a transaction
	for _, migration := range pendingMigrations {
		if err := m.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
	}

	// Get final version
	finalVersion, _ := m.getCurrentVersion(ctx)
	fmt.Printf("Migration complete. Current version: %d\n", finalVersion)

	return nil
}

// applyMigration applies a single migration
func (m *Migrator) applyMigration(ctx context.Context, migration Migration) error {
	fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description)

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // Rollback on error - commit overrides if successful

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration in schema_version table
	_, err = tx.ExecContext(ctx,
		"INSERT INTO schema_version (version, description) VALUES ($1, $2) ON CONFLICT (version) DO NOTHING",
		migration.Version,
		migration.Description,
	)
	if err != nil {
		return fmt.Errorf("failed to record migration version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("✓ Migration %d applied successfully\n", migration.Version)

	return nil
}

// Status shows the current migration status
func (m *Migrator) Status(ctx context.Context) error {
	if err := m.ensureSchemaVersionTable(ctx); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return err
	}

	fmt.Printf("Current schema version: %d\n", currentVersion)
	fmt.Printf("Available migrations: %d\n", len(migrations))
	fmt.Println("\nMigration history:")
	fmt.Println("VERSION | STATUS  | DESCRIPTION")
	fmt.Println("--------|---------|-----------------------------------")

	for _, migration := range migrations {
		status := "pending"
		if migration.Version <= currentVersion {
			status = "applied"
		}
		fmt.Printf("%-7d | %-7s | %s\n", migration.Version, status, migration.Description)
	}

	return nil
}
