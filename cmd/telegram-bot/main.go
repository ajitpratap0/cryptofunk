package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/telegram"
)

func main() {
	// Setup logging
	setupLogging()

	log.Info().Msg("Starting CryptoFunk Telegram Bot")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Check if Telegram is enabled
	if !config.Enabled {
		log.Warn().Msg("Telegram bot is disabled in configuration")
		log.Info().Msg("Set telegram.enabled=true or TELEGRAM_ENABLED=true to enable")
		os.Exit(0)
	}

	// Validate bot token
	if config.BotToken == "" {
		log.Fatal().Msg("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	// Initialize database connection
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	log.Info().Msg("Database connection established")

	// Create bot configuration
	botConfig := &telegram.Config{
		BotToken:        config.BotToken,
		WebhookURL:      config.WebhookURL,
		PollingTimeout:  config.PollingTimeout,
		Debug:           config.Debug,
		OrchestratorURL: config.OrchestratorURL,
	}

	// Create bot instance
	bot, err := telegram.NewBot(botConfig, database.Pool())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create bot")
	}

	log.Info().
		Str("mode", getMode(config.WebhookURL)).
		Msg("Bot initialized successfully")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start bot in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := bot.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Info().Msg("Received shutdown signal")
		bot.Stop()
	case err := <-errChan:
		log.Error().Err(err).Msg("Bot error")
		bot.Stop()
		os.Exit(1)
	}

	log.Info().Msg("Telegram bot stopped gracefully")
}

// Config holds the application configuration
type Config struct {
	Enabled         bool
	BotToken        string
	WebhookURL      string
	PollingTimeout  int
	Debug           bool
	OrchestratorURL string
}

// loadConfig loads configuration from file and environment variables
func loadConfig() (*Config, error) {
	// Set config file path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Enable environment variable override
	viper.SetEnvPrefix("CRYPTOFUNK")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	log.Info().
		Str("config_file", viper.ConfigFileUsed()).
		Msg("Configuration loaded")

	// Parse Telegram configuration
	config := &Config{
		Enabled:         viper.GetBool("telegram.enabled"),
		BotToken:        viper.GetString("telegram.bot_token"),
		WebhookURL:      viper.GetString("telegram.webhook_url"),
		PollingTimeout:  viper.GetInt("telegram.polling_timeout"),
		Debug:           viper.GetBool("telegram.debug"),
		OrchestratorURL: viper.GetString("api.orchestrator_url"),
	}

	// Override with environment variables if set
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		config.BotToken = token
	}

	if enabled := os.Getenv("TELEGRAM_ENABLED"); enabled != "" {
		config.Enabled = enabled == "true" || enabled == "1"
	}

	// Set defaults
	if config.PollingTimeout == 0 {
		config.PollingTimeout = 60
	}

	if config.OrchestratorURL == "" {
		config.OrchestratorURL = "http://localhost:8081"
	}

	return config, nil
}

// setupLogging configures zerolog
func setupLogging() {
	// Use console writer for human-readable output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Set log level
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().
		Str("level", zerolog.GlobalLevel().String()).
		Msg("Logging configured")
}

// getMode returns the bot mode (polling or webhook)
func getMode(webhookURL string) string {
	if webhookURL != "" {
		return "webhook"
	}
	return "polling"
}
