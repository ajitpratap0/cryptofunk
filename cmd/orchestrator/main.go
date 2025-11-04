package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

func main() {
	// Configure logging to stderr (important for MCP protocol - stdout is reserved)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	log.Info().Msg("Starting CryptoFunk MCP Orchestrator")

	// Load configuration
	viper.SetConfigName("orchestrator")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("CRYPTOFUNK")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("orchestrator.name", "mcp-orchestrator")
	viper.SetDefault("orchestrator.nats_url", "nats://localhost:4222")
	viper.SetDefault("orchestrator.signal_topic", "cryptofunk.agent.signals")
	viper.SetDefault("orchestrator.decision_topic", "cryptofunk.orchestrator.decisions")
	viper.SetDefault("orchestrator.heartbeat_topic", "cryptofunk.agent.heartbeat")
	viper.SetDefault("orchestrator.step_interval", "30s")
	viper.SetDefault("orchestrator.min_consensus", 0.6)
	viper.SetDefault("orchestrator.min_confidence", 0.5)
	viper.SetDefault("orchestrator.max_signal_age", "5m")
	viper.SetDefault("orchestrator.health_check_interval", "1m")
	viper.SetDefault("orchestrator.metrics_port", 8080)

	if err := viper.ReadInConfig(); err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
	} else {
		log.Info().Str("config_file", viper.ConfigFileUsed()).Msg("Loaded configuration")
	}

	// DEBUG: Check what Viper has loaded from YAML
	log.Debug().
		Str("name", viper.GetString("orchestrator.name")).
		Str("nats_url", viper.GetString("orchestrator.nats_url")).
		Str("signal_topic", viper.GetString("orchestrator.signal_topic")).
		Msg("Viper values after ReadInConfig()")

	// Override with environment variables if set
	// Note: BindEnv() doesn't work with UnmarshalKey() due to sub-viper creation
	// So we manually check env vars and use Set() to override YAML values
	if url := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_NATS_URL"); url != "" {
		viper.Set("orchestrator.nats_url", url)
	}
	if topic := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_SIGNAL_TOPIC"); topic != "" {
		viper.Set("orchestrator.signal_topic", topic)
	}
	if topic := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_DECISION_TOPIC"); topic != "" {
		viper.Set("orchestrator.decision_topic", topic)
	}
	if topic := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_HEARTBEAT_TOPIC"); topic != "" {
		viper.Set("orchestrator.heartbeat_topic", topic)
	}
	if interval := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_STEP_INTERVAL"); interval != "" {
		viper.Set("orchestrator.step_interval", interval)
	}
	if consensus := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_MIN_CONSENSUS"); consensus != "" {
		viper.Set("orchestrator.min_consensus", consensus)
	}
	if confidence := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_MIN_CONFIDENCE"); confidence != "" {
		viper.Set("orchestrator.min_confidence", confidence)
	}
	if maxAge := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_MAX_SIGNAL_AGE"); maxAge != "" {
		viper.Set("orchestrator.max_signal_age", maxAge)
	}
	if healthInterval := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_HEALTH_CHECK_INTERVAL"); healthInterval != "" {
		viper.Set("orchestrator.health_check_interval", healthInterval)
	}
	if port := os.Getenv("CRYPTOFUNK_ORCHESTRATOR_METRICS_PORT"); port != "" {
		viper.Set("orchestrator.metrics_port", port)
	}

	// DEBUG: Check what Viper has after env var overrides
	log.Debug().
		Str("name", viper.GetString("orchestrator.name")).
		Str("nats_url", viper.GetString("orchestrator.nats_url")).
		Str("signal_topic", viper.GetString("orchestrator.signal_topic")).
		Str("decision_topic", viper.GetString("orchestrator.decision_topic")).
		Str("heartbeat_topic", viper.GetString("orchestrator.heartbeat_topic")).
		Msg("Viper values after env var overrides")

	// Manually construct config struct to avoid viper.Sub() inheritance issues
	// This approach is more reliable than UnmarshalKey() or Sub() + Unmarshal()
	// because it reads each value directly from the parent viper which has both
	// YAML defaults and environment variable overrides
	config := orchestrator.OrchestratorConfig{
		Name:                viper.GetString("orchestrator.name"),
		NATSUrl:             viper.GetString("orchestrator.nats_url"),
		SignalTopic:         viper.GetString("orchestrator.signal_topic"),
		DecisionTopic:       viper.GetString("orchestrator.decision_topic"),
		HeartbeatTopic:      viper.GetString("orchestrator.heartbeat_topic"),
		StepInterval:        viper.GetDuration("orchestrator.step_interval"),
		MinConsensus:        viper.GetFloat64("orchestrator.min_consensus"),
		MinConfidence:       viper.GetFloat64("orchestrator.min_confidence"),
		MaxSignalAge:        viper.GetDuration("orchestrator.max_signal_age"),
		HealthCheckInterval: viper.GetDuration("orchestrator.health_check_interval"),
	}

	// Get metrics port
	metricsPort := viper.GetInt("orchestrator.metrics_port")

	// Log configuration
	log.Info().
		Str("name", config.Name).
		Str("nats_url", config.NATSUrl).
		Str("signal_topic", config.SignalTopic).
		Str("decision_topic", config.DecisionTopic).
		Str("heartbeat_topic", config.HeartbeatTopic).
		Dur("step_interval", config.StepInterval).
		Float64("min_consensus", config.MinConsensus).
		Float64("min_confidence", config.MinConfidence).
		Dur("max_signal_age", config.MaxSignalAge).
		Int("metrics_port", metricsPort).
		Msg("Orchestrator configuration loaded")

	// Create orchestrator
	orch, err := orchestrator.NewOrchestrator(&config, log.Logger, metricsPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create orchestrator")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize orchestrator
	if err := orch.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize orchestrator")
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start orchestrator in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := orch.Run(ctx); err != nil {
			errChan <- fmt.Errorf("orchestrator run error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case err := <-errChan:
		log.Error().Err(err).Msg("Orchestrator error")
	}

	// Initiate graceful shutdown
	log.Info().Msg("Initiating graceful shutdown...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown orchestrator
	if err := orch.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during orchestrator shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Orchestrator shutdown complete")
}
