package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/api"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Info().Msg("Starting CryptoFunk API Server")

	// Create context that listens for interrupt signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Read configuration from environment
	host := os.Getenv("API_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := 8080
	if portStr := os.Getenv("API_PORT"); portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			log.Warn().Str("port", portStr).Msg("Invalid API_PORT, using default 8080")
		}
	}

	// Initialize database connection
	database, err := db.New(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize database, continuing without DB")
		// Continue without database for now - some endpoints will work
		database = nil
	}
	defer func() {
		if database != nil {
			database.Close()
		}
	}()

	// Create API server
	config := api.Config{
		Host: host,
		Port: port,
		DB:   database,
	}

	server := api.NewServer(config)

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start()
	}()

	// Wait for interrupt signal or server error
	select {
	case err := <-serverErrors:
		log.Error().Err(err).Msg("Server error")
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	}

	// Graceful shutdown
	log.Info().Msg("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Failed to stop server gracefully")
		os.Exit(1)
	}

	log.Info().Msg("Server stopped successfully")
}
