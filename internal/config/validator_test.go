package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultValidatorOptions(t *testing.T) {
	opts := DefaultValidatorOptions()
	assert.True(t, opts.VerifyConnectivity)
	assert.False(t, opts.VerifyAPIKeys)
	assert.Equal(t, 5*time.Second, opts.Timeout)
}

func TestNewValidator(t *testing.T) {
	cfg := &Config{}
	opts := DefaultValidatorOptions()
	v := NewValidator(cfg, opts)
	assert.NotNil(t, v)
	assert.Equal(t, cfg, v.config)
}

func TestIsPlaceholderValue(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"your_api_key_here", true},
		{"changeme", true},
		{"placeholder_secret", true},
		{"example_key", true},
		{"test_value", true},
		{"sample_api_key", true},
		{"demo_key", true},
		{"sk-abc123realkey456", false},
		{"aVeryR3alS3cretK3y!", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := isPlaceholderValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateProductionRequirements_NonProd(t *testing.T) {
	os.Setenv("CRYPTOFUNK_APP_ENVIRONMENT", "development")
	defer os.Unsetenv("CRYPTOFUNK_APP_ENVIRONMENT")

	cfg := &Config{}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateProductionRequirements()
	assert.NoError(t, err)
}

func TestValidateProductionRequirements_ProdNoVault(t *testing.T) {
	os.Setenv("CRYPTOFUNK_APP_ENVIRONMENT", "production")
	os.Unsetenv("VAULT_ENABLED")
	defer os.Unsetenv("CRYPTOFUNK_APP_ENVIRONMENT")

	cfg := &Config{}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateProductionRequirements()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Vault must be enabled")
}

func TestValidateEnvironmentVariables_MinConfig(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Host: "localhost"},
		Redis:    RedisConfig{Host: "localhost"},
		NATS:     NATSConfig{URL: "nats://localhost:4222"},
	}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateEnvironmentVariables()
	assert.NoError(t, err)
}

func TestValidateEnvironmentVariables_MissingDB(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	cfg := &Config{
		Database: DatabaseConfig{},
		Redis:    RedisConfig{Host: "localhost"},
		NATS:     NATSConfig{URL: "nats://localhost:4222"},
	}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateEnvironmentVariables()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE")
}

func TestValidateAPIKeysPresence_PaperMode(t *testing.T) {
	cfg := &Config{
		Trading: TradingConfig{Mode: "paper"},
	}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateAPIKeysPresence()
	assert.NoError(t, err)
}

func TestValidateAPIKeysPresence_LiveModeEmptyKeys(t *testing.T) {
	cfg := &Config{
		Trading: TradingConfig{Mode: "live"},
		Exchanges: map[string]ExchangeConfig{
			"binance": {APIKey: "", SecretKey: ""},
		},
	}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateAPIKeysPresence()
	assert.Error(t, err)
}

func TestValidateAPIKeysPresence_LiveModePlaceholder(t *testing.T) {
	cfg := &Config{
		Trading: TradingConfig{Mode: "live"},
		Exchanges: map[string]ExchangeConfig{
			"binance": {APIKey: "your_api_key_here_placeholder", SecretKey: "your_secret_here_placeholder"},
		},
	}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateAPIKeysPresence()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "placeholder")
}

func TestValidateAPIKeysPresence_LiveModeShortKeys(t *testing.T) {
	cfg := &Config{
		Trading: TradingConfig{Mode: "live"},
		Exchanges: map[string]ExchangeConfig{
			"binance": {APIKey: "short", SecretKey: "short"},
		},
	}
	v := NewValidator(cfg, DefaultValidatorOptions())
	err := v.validateAPIKeysPresence()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestValidateStartup_NoConnectivity(t *testing.T) {
	os.Setenv("CRYPTOFUNK_APP_ENVIRONMENT", "development")
	defer os.Unsetenv("CRYPTOFUNK_APP_ENVIRONMENT")

	cfg := &Config{
		Database: DatabaseConfig{Host: "localhost"},
		Redis:    RedisConfig{Host: "localhost"},
		NATS:     NATSConfig{URL: "nats://localhost:4222"},
		Trading:  TradingConfig{Mode: "paper"},
	}
	opts := ValidatorOptions{
		VerifyConnectivity: false,
		VerifyAPIKeys:      false,
		Timeout:            1 * time.Second,
	}
	v := NewValidator(cfg, opts)

	ctx := context.Background()
	err := v.ValidateStartup(ctx)
	require.NoError(t, err)
}
