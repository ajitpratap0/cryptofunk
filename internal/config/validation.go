package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Configuration validation failed with %d error(s):\n\n", len(ve)))
	for i, err := range ve {
		sb.WriteString(fmt.Sprintf("  %d. %s: %s\n", i+1, err.Field, err.Message))
	}
	sb.WriteString("\nPlease fix the above errors and try again.\n")
	return sb.String()
}

// Validate performs comprehensive configuration validation
func (c *Config) Validate() error {
	var errors ValidationErrors

	// Validate App configuration
	errors = append(errors, c.validateApp()...)

	// Validate Database configuration
	errors = append(errors, c.validateDatabase()...)

	// Validate Redis configuration
	errors = append(errors, c.validateRedis()...)

	// Validate NATS configuration
	errors = append(errors, c.validateNATS()...)

	// Validate LLM configuration
	errors = append(errors, c.validateLLM()...)

	// Validate Trading configuration
	errors = append(errors, c.validateTrading()...)

	// Validate Risk configuration
	errors = append(errors, c.validateRisk()...)

	// Validate Exchange configuration
	errors = append(errors, c.validateExchanges()...)

	// Validate API configuration
	errors = append(errors, c.validateAPI()...)

	// Validate environment-specific requirements
	errors = append(errors, c.validateEnvironmentRequirements()...)

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (c *Config) validateApp() ValidationErrors {
	var errors ValidationErrors

	if c.App.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "app.name",
			Message: "Application name is required",
		})
	}

	if c.App.Environment == "" {
		errors = append(errors, ValidationError{
			Field:   "app.environment",
			Message: "Environment is required (development, staging, or production)",
		})
	} else {
		validEnvs := []string{"development", "staging", "production"}
		valid := false
		for _, env := range validEnvs {
			if c.App.Environment == env {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, ValidationError{
				Field:   "app.environment",
				Message: fmt.Sprintf("Invalid environment '%s'. Must be one of: %v", c.App.Environment, validEnvs),
			})
		}
	}

	if c.App.LogLevel == "" {
		errors = append(errors, ValidationError{
			Field:   "app.log_level",
			Message: "Log level is required (debug, info, warn, error)",
		})
	}

	return errors
}

func (c *Config) validateDatabase() ValidationErrors {
	var errors ValidationErrors

	if c.Database.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "database.host",
			Message: "Database host is required",
		})
	}

	if c.Database.Port == 0 {
		errors = append(errors, ValidationError{
			Field:   "database.port",
			Message: "Database port is required",
		})
	} else if c.Database.Port < 1 || c.Database.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "database.port",
			Message: fmt.Sprintf("Invalid port %d. Must be between 1-65535", c.Database.Port),
		})
	}

	if c.Database.User == "" {
		errors = append(errors, ValidationError{
			Field:   "database.user",
			Message: "Database user is required",
		})
	}

	if c.Database.Database == "" {
		errors = append(errors, ValidationError{
			Field:   "database.database",
			Message: "Database name is required",
		})
	}

	// Warn about missing password in non-development environments
	if c.Database.Password == "" && c.App.Environment != "development" {
		errors = append(errors, ValidationError{
			Field:   "database.password",
			Message: "Database password is required in non-development environments",
		})
	}

	if c.Database.PoolSize < 1 {
		errors = append(errors, ValidationError{
			Field:   "database.pool_size",
			Message: "Database pool size must be at least 1",
		})
	}

	return errors
}

func (c *Config) validateRedis() ValidationErrors {
	var errors ValidationErrors

	if c.Redis.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "redis.host",
			Message: "Redis host is required",
		})
	}

	if c.Redis.Port == 0 {
		errors = append(errors, ValidationError{
			Field:   "redis.port",
			Message: "Redis port is required",
		})
	} else if c.Redis.Port < 1 || c.Redis.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "redis.port",
			Message: fmt.Sprintf("Invalid port %d. Must be between 1-65535", c.Redis.Port),
		})
	}

	return errors
}

func (c *Config) validateNATS() ValidationErrors {
	var errors ValidationErrors

	if c.NATS.URL == "" {
		errors = append(errors, ValidationError{
			Field:   "nats.url",
			Message: "NATS URL is required",
		})
	} else if !strings.HasPrefix(c.NATS.URL, "nats://") {
		errors = append(errors, ValidationError{
			Field:   "nats.url",
			Message: "NATS URL must start with 'nats://'",
		})
	}

	return errors
}

func (c *Config) validateLLM() ValidationErrors {
	var errors ValidationErrors

	if c.LLM.Gateway == "" {
		errors = append(errors, ValidationError{
			Field:   "llm.gateway",
			Message: "LLM gateway is required",
		})
	}

	if c.LLM.Endpoint == "" {
		errors = append(errors, ValidationError{
			Field:   "llm.endpoint",
			Message: "LLM endpoint is required",
		})
	}

	if c.LLM.PrimaryModel == "" {
		errors = append(errors, ValidationError{
			Field:   "llm.primary_model",
			Message: "LLM primary model is required",
		})
	}

	if c.LLM.Temperature < 0 || c.LLM.Temperature > 2 {
		errors = append(errors, ValidationError{
			Field:   "llm.temperature",
			Message: fmt.Sprintf("Invalid temperature %.2f. Must be between 0-2", c.LLM.Temperature),
		})
	}

	if c.LLM.MaxTokens < 1 {
		errors = append(errors, ValidationError{
			Field:   "llm.max_tokens",
			Message: "LLM max_tokens must be at least 1",
		})
	}

	if c.LLM.Timeout < 1000 {
		errors = append(errors, ValidationError{
			Field:   "llm.timeout",
			Message: "LLM timeout must be at least 1000ms",
		})
	}

	return errors
}

func (c *Config) validateTrading() ValidationErrors {
	var errors ValidationErrors

	if c.Trading.Mode == "" {
		errors = append(errors, ValidationError{
			Field:   "trading.mode",
			Message: "Trading mode is required (paper or live)",
		})
	} else {
		validModes := []string{"paper", "live", "PAPER", "LIVE"}
		valid := false
		for _, mode := range validModes {
			if c.Trading.Mode == mode {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, ValidationError{
				Field:   "trading.mode",
				Message: fmt.Sprintf("Invalid trading mode '%s'. Must be 'paper' or 'live'", c.Trading.Mode),
			})
		}
	}

	if len(c.Trading.Symbols) == 0 {
		errors = append(errors, ValidationError{
			Field:   "trading.symbols",
			Message: "At least one trading symbol is required",
		})
	}

	if c.Trading.Exchange == "" {
		errors = append(errors, ValidationError{
			Field:   "trading.exchange",
			Message: "Exchange is required",
		})
	}

	if c.Trading.InitialCapital <= 0 {
		errors = append(errors, ValidationError{
			Field:   "trading.initial_capital",
			Message: "Initial capital must be greater than 0",
		})
	}

	if c.Trading.MaxPositions < 1 {
		errors = append(errors, ValidationError{
			Field:   "trading.max_positions",
			Message: "Max positions must be at least 1",
		})
	}

	if c.Trading.DefaultQuantity <= 0 {
		errors = append(errors, ValidationError{
			Field:   "trading.default_quantity",
			Message: "Default quantity must be greater than 0",
		})
	}

	return errors
}

func (c *Config) validateRisk() ValidationErrors {
	var errors ValidationErrors

	if c.Risk.MaxPositionSize <= 0 || c.Risk.MaxPositionSize > 1 {
		errors = append(errors, ValidationError{
			Field:   "risk.max_position_size",
			Message: fmt.Sprintf("Invalid max_position_size %.2f. Must be between 0-1 (representing percentage)", c.Risk.MaxPositionSize),
		})
	}

	if c.Risk.MaxDailyLoss <= 0 || c.Risk.MaxDailyLoss > 1 {
		errors = append(errors, ValidationError{
			Field:   "risk.max_daily_loss",
			Message: fmt.Sprintf("Invalid max_daily_loss %.2f. Must be between 0-1", c.Risk.MaxDailyLoss),
		})
	}

	if c.Risk.MaxDrawdown <= 0 || c.Risk.MaxDrawdown > 1 {
		errors = append(errors, ValidationError{
			Field:   "risk.max_drawdown",
			Message: fmt.Sprintf("Invalid max_drawdown %.2f. Must be between 0-1", c.Risk.MaxDrawdown),
		})
	}

	if c.Risk.MinConfidence < 0 || c.Risk.MinConfidence > 1 {
		errors = append(errors, ValidationError{
			Field:   "risk.min_confidence",
			Message: fmt.Sprintf("Invalid min_confidence %.2f. Must be between 0-1", c.Risk.MinConfidence),
		})
	}

	return errors
}

func (c *Config) validateExchanges() ValidationErrors {
	var errors ValidationErrors

	if len(c.Exchanges) == 0 {
		errors = append(errors, ValidationError{
			Field:   "exchanges",
			Message: "At least one exchange must be configured",
		})
	}

	for exchangeName, exchangeConfig := range c.Exchanges {
		// Check if API key is present for live trading
		if strings.ToLower(c.Trading.Mode) == "live" && exchangeConfig.APIKey == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("exchanges.%s.api_key", exchangeName),
				Message: "API key is required for live trading",
			})
		}

		if strings.ToLower(c.Trading.Mode) == "live" && exchangeConfig.SecretKey == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("exchanges.%s.secret_key", exchangeName),
				Message: "Secret key is required for live trading",
			})
		}

		// Validate rate limit
		if exchangeConfig.RateLimitMS < 0 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("exchanges.%s.rate_limit_ms", exchangeName),
				Message: "Rate limit must be non-negative",
			})
		}
	}

	return errors
}

func (c *Config) validateAPI() ValidationErrors {
	var errors ValidationErrors

	if c.API.Port == 0 {
		errors = append(errors, ValidationError{
			Field:   "api.port",
			Message: "API port is required",
		})
	} else if c.API.Port < 1 || c.API.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "api.port",
			Message: fmt.Sprintf("Invalid port %d. Must be between 1-65535", c.API.Port),
		})
	}

	return errors
}

func (c *Config) validateEnvironmentRequirements() ValidationErrors {
	var errors ValidationErrors

	// Production-specific validations
	if c.App.Environment == "production" {
		// Validate production secrets strength
		secretErrors := ValidateProductionSecrets(c)
		errors = append(errors, secretErrors...)

		// Ensure no testnet in production
		for exchangeName, exchangeConfig := range c.Exchanges {
			if exchangeConfig.Testnet {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("exchanges.%s.testnet", exchangeName),
					Message: "Testnet mode must be disabled in production",
				})
			}
		}

		// Note: Paper trading in production might be intentional for testing
		// Not enforcing live trading mode as a hard requirement

		// Ensure SSL for database in production
		if c.Database.SSLMode == "disable" {
			errors = append(errors, ValidationError{
				Field:   "database.ssl_mode",
				Message: "SSL must be enabled for database in production",
			})
		}
	}

	// Check critical environment variables
	criticalEnvVars := []string{
		"DATABASE_URL", // Can be constructed from config, but should be set
	}

	for _, envVar := range criticalEnvVars {
		if os.Getenv(envVar) == "" && c.App.Environment == "production" {
			// DATABASE_URL is optional if database config is complete
			if envVar == "DATABASE_URL" {
				// Check if database config is complete
				if c.Database.Host != "" && c.Database.Database != "" {
					continue // Config is complete, no need for DATABASE_URL
				}
			}

			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("env.%s", envVar),
				Message: fmt.Sprintf("Environment variable %s is required in production", envVar),
			})
		}
	}

	return errors
}

// ValidateAndLoad loads and validates configuration
// Returns the loaded config and any validation errors
// configPath can be empty to use default config locations
func ValidateAndLoad(configPath string) (*Config, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validation is already called within Load(), but we can call it again
	// for explicit validation if Load() is modified in the future
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
