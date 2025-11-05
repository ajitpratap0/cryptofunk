//nolint:goconst // Test files use repeated strings for clarity
package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getValidConfig returns a valid configuration for testing
func getValidConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:        "CryptoFunk",
			Version:     "1.0.0",
			Environment: "development",
			LogLevel:    "info",
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "secure_password",
			Database: "cryptofunk",
			SSLMode:  "disable",
			PoolSize: 10,
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: 6379,
			DB:   0,
		},
		NATS: NATSConfig{
			URL:             "nats://localhost:4222",
			EnableJetStream: true,
		},
		LLM: LLMConfig{
			Gateway:       "bifrost",
			Endpoint:      "http://localhost:8080/v1/chat/completions",
			PrimaryModel:  "claude-sonnet-4",
			FallbackModel: "gpt-4-turbo",
			Temperature:   0.7,
			MaxTokens:     2000,
			EnableCaching: true,
			Timeout:       30000,
		},
		Trading: TradingConfig{
			Mode:            "paper",
			Symbols:         []string{"BTC/USDT", "ETH/USDT"},
			Exchange:        "binance",
			InitialCapital:  10000.0,
			MaxPositions:    3,
			DefaultQuantity: 0.01,
		},
		Risk: RiskConfig{
			MaxPositionSize:     0.1,
			MaxDailyLoss:        0.02,
			MaxDrawdown:         0.1,
			DefaultStopLoss:     0.02,
			DefaultTakeProfit:   0.05,
			LLMApprovalRequired: true,
			MinConfidence:       0.7,
		},
		Exchanges: map[string]ExchangeConfig{
			"binance": {
				APIKey:      "test_api_key",
				SecretKey:   "test_secret_key",
				Testnet:     true,
				RateLimitMS: 100,
			},
		},
		API: APIConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			OrchestratorURL: "http://localhost:8081",
		},
		Monitoring: MonitoringConfig{
			PrometheusPort: 9100,
			EnableMetrics:  true,
		},
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := getValidConfig()
	err := cfg.Validate()
	assert.NoError(t, err, "Valid configuration should not produce errors")
}

func TestValidateApp(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing app name",
			modify: func(c *Config) {
				c.App.Name = ""
			},
			expectError: "app.name",
		},
		{
			name: "missing environment",
			modify: func(c *Config) {
				c.App.Environment = ""
			},
			expectError: "app.environment",
		},
		{
			name: "invalid environment",
			modify: func(c *Config) {
				c.App.Environment = "invalid_env"
			},
			expectError: "Invalid environment",
		},
		{
			name: "missing log level",
			modify: func(c *Config) {
				c.App.LogLevel = ""
			},
			expectError: "app.log_level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateDatabase(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing host",
			modify: func(c *Config) {
				c.Database.Host = ""
			},
			expectError: "database.host",
		},
		{
			name: "missing port",
			modify: func(c *Config) {
				c.Database.Port = 0
			},
			expectError: "database.port",
		},
		{
			name: "invalid port - too high",
			modify: func(c *Config) {
				c.Database.Port = 70000
			},
			expectError: "Invalid port",
		},
		{
			name: "invalid port - negative",
			modify: func(c *Config) {
				c.Database.Port = -1
			},
			expectError: "Invalid port",
		},
		{
			name: "missing user",
			modify: func(c *Config) {
				c.Database.User = ""
			},
			expectError: "database.user",
		},
		{
			name: "missing database name",
			modify: func(c *Config) {
				c.Database.Database = ""
			},
			expectError: "database.database",
		},
		{
			name: "missing password in production",
			modify: func(c *Config) {
				c.App.Environment = "production"
				c.Database.Password = ""
			},
			expectError: "password is required",
		},
		{
			name: "invalid pool size",
			modify: func(c *Config) {
				c.Database.PoolSize = 0
			},
			expectError: "pool size must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateRedis(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing host",
			modify: func(c *Config) {
				c.Redis.Host = ""
			},
			expectError: "redis.host",
		},
		{
			name: "missing port",
			modify: func(c *Config) {
				c.Redis.Port = 0
			},
			expectError: "redis.port",
		},
		{
			name: "invalid port",
			modify: func(c *Config) {
				c.Redis.Port = 70000
			},
			expectError: "Invalid port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateNATS(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing URL",
			modify: func(c *Config) {
				c.NATS.URL = ""
			},
			expectError: "nats.url",
		},
		{
			name: "invalid URL format",
			modify: func(c *Config) {
				c.NATS.URL = "http://localhost:4222"
			},
			expectError: "must start with 'nats://'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateLLM(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing gateway",
			modify: func(c *Config) {
				c.LLM.Gateway = ""
			},
			expectError: "llm.gateway",
		},
		{
			name: "missing endpoint",
			modify: func(c *Config) {
				c.LLM.Endpoint = ""
			},
			expectError: "llm.endpoint",
		},
		{
			name: "missing primary model",
			modify: func(c *Config) {
				c.LLM.PrimaryModel = ""
			},
			expectError: "llm.primary_model",
		},
		{
			name: "invalid temperature - too low",
			modify: func(c *Config) {
				c.LLM.Temperature = -0.1
			},
			expectError: "Invalid temperature",
		},
		{
			name: "invalid temperature - too high",
			modify: func(c *Config) {
				c.LLM.Temperature = 2.5
			},
			expectError: "Invalid temperature",
		},
		{
			name: "invalid max_tokens",
			modify: func(c *Config) {
				c.LLM.MaxTokens = 0
			},
			expectError: "max_tokens must be at least 1",
		},
		{
			name: "invalid timeout",
			modify: func(c *Config) {
				c.LLM.Timeout = 500
			},
			expectError: "timeout must be at least 1000ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateTrading(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing mode",
			modify: func(c *Config) {
				c.Trading.Mode = ""
			},
			expectError: "trading.mode",
		},
		{
			name: "invalid mode",
			modify: func(c *Config) {
				c.Trading.Mode = "invalid_mode"
			},
			expectError: "Invalid trading mode",
		},
		{
			name: "no symbols",
			modify: func(c *Config) {
				c.Trading.Symbols = []string{}
			},
			expectError: "At least one trading symbol",
		},
		{
			name: "missing exchange",
			modify: func(c *Config) {
				c.Trading.Exchange = ""
			},
			expectError: "trading.exchange",
		},
		{
			name: "invalid initial capital - zero",
			modify: func(c *Config) {
				c.Trading.InitialCapital = 0
			},
			expectError: "Initial capital must be greater than 0",
		},
		{
			name: "invalid initial capital - negative",
			modify: func(c *Config) {
				c.Trading.InitialCapital = -1000
			},
			expectError: "Initial capital must be greater than 0",
		},
		{
			name: "invalid max positions",
			modify: func(c *Config) {
				c.Trading.MaxPositions = 0
			},
			expectError: "Max positions must be at least 1",
		},
		{
			name: "invalid default quantity",
			modify: func(c *Config) {
				c.Trading.DefaultQuantity = 0
			},
			expectError: "Default quantity must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateRisk(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "invalid max_position_size - too low",
			modify: func(c *Config) {
				c.Risk.MaxPositionSize = 0
			},
			expectError: "Invalid max_position_size",
		},
		{
			name: "invalid max_position_size - too high",
			modify: func(c *Config) {
				c.Risk.MaxPositionSize = 1.5
			},
			expectError: "Invalid max_position_size",
		},
		{
			name: "invalid max_daily_loss - too low",
			modify: func(c *Config) {
				c.Risk.MaxDailyLoss = 0
			},
			expectError: "Invalid max_daily_loss",
		},
		{
			name: "invalid max_daily_loss - too high",
			modify: func(c *Config) {
				c.Risk.MaxDailyLoss = 1.5
			},
			expectError: "Invalid max_daily_loss",
		},
		{
			name: "invalid max_drawdown - too low",
			modify: func(c *Config) {
				c.Risk.MaxDrawdown = 0
			},
			expectError: "Invalid max_drawdown",
		},
		{
			name: "invalid max_drawdown - too high",
			modify: func(c *Config) {
				c.Risk.MaxDrawdown = 1.5
			},
			expectError: "Invalid max_drawdown",
		},
		{
			name: "invalid min_confidence - too low",
			modify: func(c *Config) {
				c.Risk.MinConfidence = -0.1
			},
			expectError: "Invalid min_confidence",
		},
		{
			name: "invalid min_confidence - too high",
			modify: func(c *Config) {
				c.Risk.MinConfidence = 1.5
			},
			expectError: "Invalid min_confidence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateExchanges(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "no exchanges configured",
			modify: func(c *Config) {
				c.Exchanges = map[string]ExchangeConfig{}
			},
			expectError: "At least one exchange must be configured",
		},
		{
			name: "missing API key in live mode",
			modify: func(c *Config) {
				c.Trading.Mode = "live"
				c.Exchanges["binance"] = ExchangeConfig{
					APIKey:      "",
					SecretKey:   "secret",
					Testnet:     false,
					RateLimitMS: 100,
				}
			},
			expectError: "API key is required for live trading",
		},
		{
			name: "missing secret key in live mode",
			modify: func(c *Config) {
				c.Trading.Mode = "live"
				c.Exchanges["binance"] = ExchangeConfig{
					APIKey:      "key",
					SecretKey:   "",
					Testnet:     false,
					RateLimitMS: 100,
				}
			},
			expectError: "Secret key is required for live trading",
		},
		{
			name: "invalid rate limit",
			modify: func(c *Config) {
				c.Exchanges["binance"] = ExchangeConfig{
					APIKey:      "key",
					SecretKey:   "secret",
					Testnet:     true,
					RateLimitMS: -1,
				}
			},
			expectError: "Rate limit must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateAPI(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "missing port",
			modify: func(c *Config) {
				c.API.Port = 0
			},
			expectError: "api.port",
		},
		{
			name: "invalid port - too high",
			modify: func(c *Config) {
				c.API.Port = 70000
			},
			expectError: "Invalid port",
		},
		{
			name: "invalid port - negative",
			modify: func(c *Config) {
				c.API.Port = -1
			},
			expectError: "Invalid port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidateEnvironmentRequirements(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name: "testnet enabled in production",
			modify: func(c *Config) {
				c.App.Environment = "production"
				c.Exchanges["binance"] = ExchangeConfig{
					APIKey:      "key",
					SecretKey:   "secret",
					Testnet:     true,
					RateLimitMS: 100,
				}
			},
			expectError: "Testnet mode must be disabled in production",
		},
		{
			name: "SSL disabled in production",
			modify: func(c *Config) {
				c.App.Environment = "production"
				c.Database.SSLMode = "disable"
			},
			expectError: "SSL must be enabled for database in production",
		},
		{
			name: "DATABASE_URL missing in production with incomplete config",
			modify: func(c *Config) {
				c.App.Environment = "production"
				c.Database.Host = ""
				// DATABASE_URL not set
				_ = os.Unsetenv("DATABASE_URL") // Test env cleanup
			},
			expectError: "DATABASE_URL is required in production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestValidationErrors_Error(t *testing.T) {
	errors := ValidationErrors{
		{Field: "field1", Message: "error message 1"},
		{Field: "field2", Message: "error message 2"},
		{Field: "field3", Message: "error message 3"},
	}

	errMsg := errors.Error()

	// Check error message structure
	assert.Contains(t, errMsg, "Configuration validation failed with 3 error(s)")
	assert.Contains(t, errMsg, "1. field1: error message 1")
	assert.Contains(t, errMsg, "2. field2: error message 2")
	assert.Contains(t, errMsg, "3. field3: error message 3")
	assert.Contains(t, errMsg, "Please fix the above errors and try again")
}

func TestValidationErrors_Empty(t *testing.T) {
	errors := ValidationErrors{}
	assert.Equal(t, "", errors.Error())
}

func TestValidateAndLoad(t *testing.T) {
	// Create a temporary config file with invalid configuration
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }() // Test cleanup

	// Write invalid config (missing required fields)
	invalidConfig := `
app:
  name: ""
  environment: "development"
  log_level: "info"
trading:
  mode: "paper"
  symbols: []
  exchange: "binance"
`
	_, err = tmpfile.WriteString(invalidConfig)
	require.NoError(t, err)
	_ = tmpfile.Close() // Test cleanup

	// Try to load - should fail validation
	_, err = Load(tmpfile.Name())
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "app.name") || strings.Contains(err.Error(), "symbols"))
}

func TestValidateCaseInsensitiveTradingMode(t *testing.T) {
	tests := []struct {
		mode  string
		valid bool
	}{
		{"paper", true},
		{"PAPER", true},
		{"live", true},
		{"LIVE", true},
		{"Paper", false}, // Mixed case not explicitly handled, so should fail
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := getValidConfig()
			cfg.Trading.Mode = tt.mode
			err := cfg.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
