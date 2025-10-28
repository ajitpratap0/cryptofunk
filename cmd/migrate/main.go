// Database migration CLI tool
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	_ "github.com/lib/pq"
)

func main() {
	// Parse command line flags
	command := flag.String("command", "migrate", "Command to run: migrate or status")
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "Database connection URL")
	migrationsDir := flag.String("migrations", "migrations", "Path to migrations directory")
	flag.Parse()

	// Use default DATABASE_URL if not provided
	if *dbURL == "" {
		*dbURL = "postgres://postgres:cryptofunk_dev_password@localhost:5432/cryptofunk?sslmode=disable"
	}

	// Connect to database
	database, err := sql.Open("postgres", *dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Test connection
	if err := database.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	// Set migrations directory
	db.SetMigrationsDir(*migrationsDir)

	// Create migrator
	migrator := db.NewMigrator(database)
	ctx := context.Background()

	// Execute command
	switch *command {
	case "migrate":
		if err := migrator.Migrate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := migrator.Status(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Status check failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *command)
		fmt.Fprintf(os.Stderr, "Usage: migrate -command=[migrate|status]\n")
		os.Exit(1)
	}
}
