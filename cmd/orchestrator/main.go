package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

func main() {
	// Parse command-line flags
	verifyKeys := flag.Bool("verify-keys", false, "Verify API keys and secrets, then exit")
	flag.Parse()

	// Configure logging to stderr (important for MCP protocol - stdout is reserved)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	// If --verify-keys flag is set, verify keys and exit
	if *verifyKeys {
		os.Exit(verifyAPIKeys())
	}

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

// verifyAPIKeys verifies all configured API keys and secrets
// Returns 0 if all keys are valid, 1 if any keys are invalid or missing
func verifyAPIKeys() int {
	log.Info().Msg("Verifying API keys and secrets...")

	// Load main configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return 1
	}

	allValid := true
	keysChecked := 0

	// Verify Exchange API Keys
	if len(cfg.Exchanges) > 0 {
		log.Info().Msg("Checking exchange API keys...")
		for exchangeName, exchangeConfig := range cfg.Exchanges {
			keysChecked++

			// Check if keys are present
			if exchangeConfig.APIKey == "" {
				log.Warn().Str("exchange", exchangeName).Msg("❌ API key not configured")
				allValid = false
				continue
			}
			if exchangeConfig.SecretKey == "" {
				log.Warn().Str("exchange", exchangeName).Msg("❌ Secret key not configured")
				allValid = false
				continue
			}

			// Check for placeholder values
			placeholders := []string{"YOUR_API_KEY", "changeme", "test_api_key", ""}
			isPlaceholder := false
			for _, placeholder := range placeholders {
				if exchangeConfig.APIKey == placeholder || exchangeConfig.SecretKey == placeholder {
					isPlaceholder = true
					break
				}
			}

			if isPlaceholder {
				log.Warn().
					Str("exchange", exchangeName).
					Msg("❌ API keys appear to be placeholder values")
				allValid = false
				continue
			}

			// For paper trading, keys don't need to be validated against the exchange
			if cfg.Trading.Mode == "paper" || cfg.Trading.Mode == "PAPER" {
				log.Info().
					Str("exchange", exchangeName).
					Str("mode", cfg.Trading.Mode).
					Msg("✓ Exchange keys configured (paper trading mode - not validated against exchange)")
				continue
			}

			// For live trading, we should validate against the exchange
			// However, this requires actual exchange API calls which may fail for various reasons
			// (network, rate limits, etc.) so we just check for presence and format
			log.Info().
				Str("exchange", exchangeName).
				Str("mode", cfg.Trading.Mode).
				Int("key_length", len(exchangeConfig.APIKey)).
				Msg("✓ Exchange API keys configured (live mode - validation requires exchange connection)")
		}
	} else {
		log.Warn().Msg("No exchanges configured")
	}

	// Verify LLM Configuration
	log.Info().Msg("Checking LLM configuration...")
	keysChecked++

	if cfg.LLM.Endpoint == "" {
		log.Error().Msg("❌ LLM endpoint not configured")
		allValid = false
	} else if cfg.LLM.Gateway == "" {
		log.Error().Msg("❌ LLM gateway not configured")
		allValid = false
	} else if cfg.LLM.PrimaryModel == "" {
		log.Error().Msg("❌ LLM primary model not configured")
		allValid = false
	} else {
		log.Info().
			Str("gateway", cfg.LLM.Gateway).
			Str("endpoint", cfg.LLM.Endpoint).
			Str("model", cfg.LLM.PrimaryModel).
			Msg("✓ LLM configuration present (endpoint validation requires live connection)")
	}

	// Verify Database Configuration
	log.Info().Msg("Checking database configuration...")
	keysChecked++

	if cfg.Database.Host == "" {
		log.Error().Msg("❌ Database host not configured")
		allValid = false
	} else if cfg.Database.Database == "" {
		log.Error().Msg("❌ Database name not configured")
		allValid = false
	} else {
		// Check password for non-development environments
		if cfg.App.Environment != "development" && cfg.Database.Password == "" {
			log.Warn().
				Str("environment", cfg.App.Environment).
				Msg("❌ Database password not configured (required for non-development environments)")
			allValid = false
		}

		// Check for placeholder passwords
		if cfg.App.Environment == "production" {
			placeholders := []string{"changeme", "changeme_in_production", "postgres", "password"}
			for _, placeholder := range placeholders {
				if cfg.Database.Password == placeholder {
					log.Error().
						Str("password", placeholder).
						Msg("❌ Database password is a common placeholder value (SECURITY RISK)")
					allValid = false
					break
				}
			}
		}

		if allValid {
			log.Info().
				Str("host", cfg.Database.Host).
				Str("database", cfg.Database.Database).
				Str("ssl_mode", cfg.Database.SSLMode).
				Msg("✓ Database configuration present")
		}
	}

	// Summary
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if allValid {
		log.Info().
			Int("keys_checked", keysChecked).
			Msg("✅ All API keys and configuration verified successfully")
		log.Info().Msg("System is ready to start")
		return 0
	} else {
		log.Error().
			Int("keys_checked", keysChecked).
			Msg("❌ Some API keys or configuration are invalid or missing")
		log.Error().Msg("Please fix the above issues before starting the system")
		return 1
	}
}
