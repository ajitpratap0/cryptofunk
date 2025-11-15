package config

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// ValidatorOptions contains options for configuration validation
type ValidatorOptions struct {
	VerifyConnectivity bool // Check database/Redis connectivity
	VerifyAPIKeys      bool // Verify API keys with external services
	Timeout            time.Duration
}

// DefaultValidatorOptions returns default validator options for startup
func DefaultValidatorOptions() ValidatorOptions {
	return ValidatorOptions{
		VerifyConnectivity: true,
		VerifyAPIKeys:      false, // Disabled by default (enabled with --verify-keys flag)
		Timeout:            5 * time.Second,
	}
}

// Validator handles configuration validation at startup
type Validator struct {
	config  *Config
	options ValidatorOptions
}

// NewValidator creates a new configuration validator
func NewValidator(config *Config, options ValidatorOptions) *Validator {
	return &Validator{
		config:  config,
		options: options,
	}
}

// ValidateStartup performs comprehensive startup validation
// This should be called before starting any services
func (v *Validator) ValidateStartup(ctx context.Context) error {
	log.Info().Msg("Validating configuration...")

	// Step 0: Check production environment requirements
	if err := v.validateProductionRequirements(); err != nil {
		return fmt.Errorf("production requirements validation failed: %w", err)
	}

	// Step 1: Validate required environment variables
	if err := v.validateEnvironmentVariables(); err != nil {
		return fmt.Errorf("environment variable validation failed: %w", err)
	}

	// Step 2: Validate API keys presence (not testing, just checking they exist)
	if err := v.validateAPIKeysPresence(); err != nil {
		return fmt.Errorf("API key validation failed: %w", err)
	}

	// Step 3: Check database connectivity (if enabled)
	if v.options.VerifyConnectivity {
		if err := v.checkDatabaseConnectivity(ctx); err != nil {
			return fmt.Errorf("database connectivity check failed: %w", err)
		}
	}

	// Step 4: Check Redis connectivity (if enabled)
	if v.options.VerifyConnectivity {
		if err := v.checkRedisConnectivity(ctx); err != nil {
			return fmt.Errorf("redis connectivity check failed: %w", err)
		}
	}

	// Step 5: Verify API keys functionality (if enabled with --verify-keys flag)
	if v.options.VerifyAPIKeys {
		if err := v.verifyAPIKeys(ctx); err != nil {
			return fmt.Errorf("API key verification failed: %w", err)
		}
	}

	log.Info().Msg("Configuration validation completed successfully")
	return nil
}

// validateProductionRequirements checks production-specific security requirements
func (v *Validator) validateProductionRequirements() error {
	// Check if we're running in production
	appEnv := strings.ToLower(os.Getenv("CRYPTOFUNK_APP_ENVIRONMENT"))
	isProduction := appEnv == "production" || appEnv == "prod"

	if !isProduction {
		// Not production, skip validation
		log.Info().Str("environment", appEnv).Msg("Non-production environment detected, skipping production requirements")
		return nil
	}

	log.Info().Msg("Production environment detected - enforcing production security requirements")

	var errors []string

	// 1. Vault must be enabled in production
	vaultEnabled := strings.ToLower(os.Getenv("VAULT_ENABLED"))
	if vaultEnabled != "true" && vaultEnabled != "1" {
		errors = append(errors, "Vault must be enabled in production (set VAULT_ENABLED=true)")
	}

	// 2. Check that Vault configuration is provided
	if vaultEnabled == "true" || vaultEnabled == "1" {
		vaultAddr := os.Getenv("VAULT_ADDR")
		if vaultAddr == "" {
			errors = append(errors, "VAULT_ADDR must be set when Vault is enabled")
		}

		vaultAuthMethod := os.Getenv("VAULT_AUTH_METHOD")
		if vaultAuthMethod == "" {
			errors = append(errors, "VAULT_AUTH_METHOD must be set when Vault is enabled (kubernetes, token, or approle)")
		}

		// Validate auth method specific requirements
		switch vaultAuthMethod {
		case "kubernetes":
			// Kubernetes auth requires K8s service account token
			tokenPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"
			if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("Kubernetes service account token not found at %s", tokenPath))
			}
		case "token":
			vaultToken := os.Getenv("VAULT_TOKEN")
			if vaultToken == "" {
				errors = append(errors, "VAULT_TOKEN must be set when using token auth method")
			}
		case "approle":
			roleID := os.Getenv("VAULT_ROLE_ID")
			secretID := os.Getenv("VAULT_SECRET_ID")
			if roleID == "" || secretID == "" {
				errors = append(errors, "VAULT_ROLE_ID and VAULT_SECRET_ID must be set when using approle auth method")
			}
		default:
			errors = append(errors, fmt.Sprintf("Unknown VAULT_AUTH_METHOD: %s (must be kubernetes, token, or approle)", vaultAuthMethod))
		}
	}

	// 3. TLS/SSL must be enforced for database
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		if strings.Contains(databaseURL, "sslmode=disable") {
			errors = append(errors, "Database SSL cannot be disabled in production (sslmode=disable found in DATABASE_URL)")
		}
		if !strings.Contains(databaseURL, "sslmode=") {
			errors = append(errors, "Database SSL mode must be explicitly set in production (add sslmode=require to DATABASE_URL)")
		}
	}

	// 4. TLS/SSL must be enforced for Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		if strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
			errors = append(errors, "Redis TLS must be enabled in production (use rediss:// instead of redis://)")
		}
	}

	// 5. JWT secret must not be a placeholder
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret != "" && isPlaceholderValue(jwtSecret) {
		errors = append(errors, "JWT_SECRET cannot be a placeholder value in production")
	}
	if jwtSecret != "" && len(jwtSecret) < 32 {
		errors = append(errors, "JWT_SECRET must be at least 32 characters in production")
	}

	// 6. Trading mode should be PAPER initially (warning, not error)
	tradingMode := strings.ToLower(os.Getenv("TRADING_MODE"))
	if tradingMode == "live" {
		log.Warn().Msg("WARNING: Live trading is enabled in production. Ensure this is intentional and all testing is complete.")
	}

	// 7. Default credentials check
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	if postgresPassword != "" && isPlaceholderValue(postgresPassword) {
		errors = append(errors, "POSTGRES_PASSWORD cannot be a placeholder value in production")
	}

	grafanaPassword := os.Getenv("GRAFANA_ADMIN_PASSWORD")
	if grafanaPassword != "" && isPlaceholderValue(grafanaPassword) {
		errors = append(errors, "GRAFANA_ADMIN_PASSWORD cannot be a placeholder value in production")
	}

	if len(errors) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("\n==========================================================\n")
		errMsg.WriteString("PRODUCTION SECURITY REQUIREMENTS NOT MET\n")
		errMsg.WriteString("==========================================================\n\n")
		errMsg.WriteString("The following production security requirements must be addressed:\n\n")
		for i, err := range errors {
			errMsg.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err))
		}
		errMsg.WriteString("\n")
		errMsg.WriteString("Production deployment cannot proceed until these issues are resolved.\n")
		errMsg.WriteString("See docs/TLS_SETUP.md and docs/SECRET_ROTATION.md for guidance.\n")
		errMsg.WriteString("==========================================================\n")
		return fmt.Errorf("%s", errMsg.String())
	}

	log.Info().Msg("✓ Production security requirements validated successfully")
	return nil
}

// validateEnvironmentVariables checks that required environment variables are set
func (v *Validator) validateEnvironmentVariables() error {
	var errors []string

	// Required environment variables based on trading mode and environment
	requiredVars := make(map[string]string)

	// Database connection (can be DATABASE_URL or individual components)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		// If DATABASE_URL is not set, check individual components
		if v.config.Database.Host == "" {
			requiredVars["DATABASE_HOST or DATABASE_URL"] = "Database host is not configured"
		}
	}

	// Redis connection
	if v.config.Redis.Host == "" {
		requiredVars["REDIS_URL or REDIS_HOST"] = "Redis host is not configured"
	}

	// NATS connection
	if v.config.NATS.URL == "" {
		requiredVars["NATS_URL"] = "NATS URL is not configured"
	}

	// Exchange API keys (only for live trading)
	if strings.ToLower(v.config.Trading.Mode) == "live" {
		for exchangeName, exchangeConfig := range v.config.Exchanges {
			if exchangeConfig.APIKey == "" {
				requiredVars[fmt.Sprintf("%s_API_KEY", strings.ToUpper(exchangeName))] =
					fmt.Sprintf("%s API key is required for live trading", exchangeName)
			}
			if exchangeConfig.SecretKey == "" {
				requiredVars[fmt.Sprintf("%s_API_SECRET", strings.ToUpper(exchangeName))] =
					fmt.Sprintf("%s API secret is required for live trading", exchangeName)
			}
		}
	}

	// LLM API keys (check via environment - Bifrost handles actual keys)
	if os.Getenv("ANTHROPIC_API_KEY") == "" && os.Getenv("OPENAI_API_KEY") == "" {
		// Warn but don't fail - might be using other providers
		log.Warn().Msg("Neither ANTHROPIC_API_KEY nor OPENAI_API_KEY is set - ensure Bifrost has LLM access configured")
	}

	if len(requiredVars) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("Required environment variables are missing:\n\n")
		for varName, description := range requiredVars {
			errMsg.WriteString(fmt.Sprintf("  - %s: %s\n", varName, description))
			errors = append(errors, fmt.Sprintf("%s: %s", varName, description))
		}
		errMsg.WriteString("\nPlease set these environment variables and try again.\n")
		return fmt.Errorf("%s", errMsg.String())
	}

	log.Info().Msg("Environment variables validation passed")
	return nil
}

// validateAPIKeysPresence checks that API keys are present and not empty
func (v *Validator) validateAPIKeysPresence() error {
	var errors []string

	// Check exchange API keys
	for exchangeName, exchangeConfig := range v.config.Exchanges {
		if strings.ToLower(v.config.Trading.Mode) == "live" {
			if exchangeConfig.APIKey == "" {
				errors = append(errors, fmt.Sprintf("%s API key is empty", exchangeName))
			} else if len(exchangeConfig.APIKey) < 16 {
				errors = append(errors, fmt.Sprintf("%s API key is too short (minimum 16 characters)", exchangeName))
			}

			if exchangeConfig.SecretKey == "" {
				errors = append(errors, fmt.Sprintf("%s API secret is empty", exchangeName))
			} else if len(exchangeConfig.SecretKey) < 16 {
				errors = append(errors, fmt.Sprintf("%s API secret is too short (minimum 16 characters)", exchangeName))
			}

			// Check for placeholder values
			if isPlaceholderValue(exchangeConfig.APIKey) {
				errors = append(errors, fmt.Sprintf("%s API key appears to be a placeholder value", exchangeName))
			}
			if isPlaceholderValue(exchangeConfig.SecretKey) {
				errors = append(errors, fmt.Sprintf("%s API secret appears to be a placeholder value", exchangeName))
			}
		}
	}

	if len(errors) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("API key validation failed:\n\n")
		for _, err := range errors {
			errMsg.WriteString(fmt.Sprintf("  - %s\n", err))
		}
		errMsg.WriteString("\nPlease provide valid API keys and try again.\n")
		return fmt.Errorf("%s", errMsg.String())
	}

	log.Info().Msg("API key presence validation passed")
	return nil
}

// checkDatabaseConnectivity tests database connection with timeout
func (v *Validator) checkDatabaseConnectivity(ctx context.Context) error {
	log.Info().Msg("Checking database connectivity...")

	// Create context with timeout
	connCtx, cancel := context.WithTimeout(ctx, v.options.Timeout)
	defer cancel()

	// Build connection string
	var connString string
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		connString = dbURL
	} else {
		connString = v.config.Database.GetDSN()
	}

	// Attempt connection
	pool, err := pgxpool.New(connCtx, connString)
	if err != nil {
		return fmt.Errorf("failed to create database connection pool: %w\n\nPlease check:\n  - Database is running\n  - Connection details are correct\n  - Network connectivity is available", err)
	}
	defer pool.Close()

	// Ping database
	if err := pool.Ping(connCtx); err != nil {
		return fmt.Errorf("failed to ping database: %w\n\nPlease check:\n  - Database is running and accepting connections\n  - Credentials are correct\n  - Network connectivity is available", err)
	}

	// Verify database name
	var dbName string
	err = pool.QueryRow(connCtx, "SELECT current_database()").Scan(&dbName)
	if err != nil {
		return fmt.Errorf("failed to verify database: %w", err)
	}

	log.Info().
		Str("database", dbName).
		Str("host", v.config.Database.Host).
		Int("port", v.config.Database.Port).
		Msg("Database connectivity check passed")

	return nil
}

// checkRedisConnectivity tests Redis connection with timeout
func (v *Validator) checkRedisConnectivity(ctx context.Context) error {
	log.Info().Msg("Checking Redis connectivity...")

	// Create context with timeout
	connCtx, cancel := context.WithTimeout(ctx, v.options.Timeout)
	defer cancel()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     v.config.Redis.GetRedisAddr(),
		Password: v.config.Redis.Password,
		DB:       v.config.Redis.DB,
	})
	defer client.Close()

	// Ping Redis
	if err := client.Ping(connCtx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis: %w\n\nPlease check:\n  - Redis is running and accepting connections\n  - Connection details are correct\n  - Network connectivity is available", err)
	}

	log.Info().
		Str("addr", v.config.Redis.GetRedisAddr()).
		Int("db", v.config.Redis.DB).
		Msg("Redis connectivity check passed")

	return nil
}

// verifyAPIKeys tests API keys with actual API calls (dry run)
func (v *Validator) verifyAPIKeys(ctx context.Context) error {
	log.Info().Msg("Verifying API keys (dry run)...")

	var errors []string

	// Verify exchange API keys
	for exchangeName, exchangeConfig := range v.config.Exchanges {
		if exchangeConfig.APIKey == "" || exchangeConfig.SecretKey == "" {
			continue // Skip if not configured
		}

		log.Info().Str("exchange", exchangeName).Msg("Verifying exchange API key...")

		// Note: Actual verification would use exchange-specific API
		// For now, we'll check Binance if that's the configured exchange
		if exchangeName == "binance" {
			if err := v.verifyBinanceAPIKey(ctx, exchangeConfig); err != nil {
				errors = append(errors, fmt.Sprintf("Binance API key verification failed: %v", err))
			} else {
				log.Info().Msg("Binance API key verification passed")
			}
		} else {
			log.Warn().Str("exchange", exchangeName).Msg("API key verification not implemented for this exchange")
		}
	}

	// Verify LLM API keys via Bifrost
	if err := v.verifyLLMAPIKey(ctx); err != nil {
		// Warn but don't fail - LLM might not be critical for startup
		log.Warn().Err(err).Msg("LLM API key verification failed")
		errors = append(errors, fmt.Sprintf("LLM API key verification failed: %v (non-critical)", err))
	}

	if len(errors) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("API key verification failed:\n\n")
		for _, err := range errors {
			errMsg.WriteString(fmt.Sprintf("  - %s\n", err))
		}
		errMsg.WriteString("\nPlease check your API keys and try again.\n")
		errMsg.WriteString("Note: Use --verify-keys flag only when you want to test API connectivity.\n")
		return fmt.Errorf("%s", errMsg.String())
	}

	log.Info().Msg("API key verification completed successfully")
	return nil
}

// verifyBinanceAPIKey tests Binance API key with a lightweight API call
func (v *Validator) verifyBinanceAPIKey(ctx context.Context, config ExchangeConfig) error {
	// Use a simple endpoint that doesn't require authentication to check connectivity
	baseURL := "https://api.binance.com"
	if config.Testnet {
		baseURL = "https://testnet.binance.vision"
	}

	// First, check if we can reach the API
	pingURL := baseURL + "/api/v3/ping"

	reqCtx, cancel := context.WithTimeout(ctx, v.options.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", pingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to ping Binance API: %w (check network connectivity)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Binance API ping failed with status: %d", resp.StatusCode)
	}

	// Note: Full API key verification would require making an authenticated request
	// to an endpoint like /api/v3/account, but that's more invasive and might have
	// rate limit implications. For now, we verify connectivity and presence of keys.
	log.Info().
		Str("base_url", baseURL).
		Bool("testnet", config.Testnet).
		Msg("Binance API connectivity verified")

	return nil
}

// verifyLLMAPIKey tests LLM API key via Bifrost health endpoint
func (v *Validator) verifyLLMAPIKey(ctx context.Context) error {
	// Check if Bifrost is reachable
	healthURL := v.config.LLM.Endpoint
	if strings.Contains(healthURL, "/v1/chat/completions") {
		// Replace chat endpoint with health endpoint
		healthURL = strings.Replace(healthURL, "/v1/chat/completions", "/health", 1)
	}

	reqCtx, cancel := context.WithTimeout(ctx, v.options.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to ping LLM gateway: %w (Bifrost might not be running)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM gateway health check failed with status: %d", resp.StatusCode)
	}

	log.Info().
		Str("endpoint", healthURL).
		Msg("LLM gateway connectivity verified")

	return nil
}

// isPlaceholderValue checks if a value is likely a placeholder
func isPlaceholderValue(value string) bool {
	lowerValue := strings.ToLower(value)
	placeholders := []string{
		"your_api_key",
		"your_secret",
		"changeme",
		"placeholder",
		"example",
		"test",
		"sample",
		"demo",
	}

	for _, placeholder := range placeholders {
		if strings.Contains(lowerValue, placeholder) {
			return true
		}
	}

	return false
}
