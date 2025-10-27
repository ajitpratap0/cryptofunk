package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	App        AppConfig                 `mapstructure:"app"`
	Database   DatabaseConfig            `mapstructure:"database"`
	Redis      RedisConfig               `mapstructure:"redis"`
	NATS       NATSConfig                `mapstructure:"nats"`
	LLM        LLMConfig                 `mapstructure:"llm"`
	Trading    TradingConfig             `mapstructure:"trading"`
	Risk       RiskConfig                `mapstructure:"risk"`
	Exchanges  map[string]ExchangeConfig `mapstructure:"exchanges"`
	API        APIConfig                 `mapstructure:"api"`
	Monitoring MonitoringConfig          `mapstructure:"monitoring"`
}

// AppConfig contains application-level settings
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"` // development, staging, production
	LogLevel    string `mapstructure:"log_level"`
}

// DatabaseConfig contains PostgreSQL/TimescaleDB settings
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
	PoolSize int    `mapstructure:"pool_size"`
}

// RedisConfig contains Redis settings
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// NATSConfig contains NATS messaging settings
type NATSConfig struct {
	URL             string `mapstructure:"url"`
	EnableJetStream bool   `mapstructure:"enable_jetstream"`
}

// LLMConfig contains LLM gateway settings
type LLMConfig struct {
	Gateway       string  `mapstructure:"gateway"`        // "bifrost"
	Endpoint      string  `mapstructure:"endpoint"`       // "http://localhost:8080/v1/chat/completions"
	PrimaryModel  string  `mapstructure:"primary_model"`  // "claude-sonnet-4-20250514"
	FallbackModel string  `mapstructure:"fallback_model"` // "gpt-4-turbo"
	Temperature   float64 `mapstructure:"temperature"`    // 0.7
	MaxTokens     int     `mapstructure:"max_tokens"`     // 2000
	EnableCaching bool    `mapstructure:"enable_caching"` // true
	Timeout       int     `mapstructure:"timeout"`        // 30000 (ms)
}

// TradingConfig contains trading settings
type TradingConfig struct {
	Mode            string   `mapstructure:"mode"`             // "paper" or "live"
	Symbols         []string `mapstructure:"symbols"`          // ["BTCUSDT", "ETHUSDT"]
	Exchange        string   `mapstructure:"exchange"`         // "binance"
	InitialCapital  float64  `mapstructure:"initial_capital"`  // 10000.0
	MaxPositions    int      `mapstructure:"max_positions"`    // 3
	DefaultQuantity float64  `mapstructure:"default_quantity"` // 0.01
}

// RiskConfig contains risk management settings
type RiskConfig struct {
	MaxPositionSize     float64 `mapstructure:"max_position_size"`     // 0.1 (10% of portfolio)
	MaxDailyLoss        float64 `mapstructure:"max_daily_loss"`        // 0.02 (2%)
	MaxDrawdown         float64 `mapstructure:"max_drawdown"`          // 0.1 (10%)
	DefaultStopLoss     float64 `mapstructure:"default_stop_loss"`     // 0.02 (2%)
	DefaultTakeProfit   float64 `mapstructure:"default_take_profit"`   // 0.05 (5%)
	LLMApprovalRequired bool    `mapstructure:"llm_approval_required"` // true
	MinConfidence       float64 `mapstructure:"min_confidence"`        // 0.7
}

// ExchangeConfig contains exchange-specific settings
type ExchangeConfig struct {
	APIKey      string `mapstructure:"api_key"`
	SecretKey   string `mapstructure:"secret_key"`
	Testnet     bool   `mapstructure:"testnet"`
	RateLimitMS int    `mapstructure:"rate_limit_ms"`
}

// APIConfig contains REST API settings
type APIConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// MonitoringConfig contains monitoring settings
type MonitoringConfig struct {
	PrometheusPort int  `mapstructure:"prometheus_port"`
	EnableMetrics  bool `mapstructure:"enable_metrics"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// Enable environment variable overrides
	v.AutomaticEnv()
	v.SetEnvPrefix("CRYPTOFUNK")

	// Set defaults
	setDefaults(v)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; using defaults and environment variables
	}

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "CryptoFunk")
	v.SetDefault("app.version", "0.1.0")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.log_level", "info")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.database", "cryptofunk")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.pool_size", 10)

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)

	// NATS defaults
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.enable_jetstream", true)

	// LLM defaults
	v.SetDefault("llm.gateway", "bifrost")
	v.SetDefault("llm.endpoint", "http://localhost:8080/v1/chat/completions")
	v.SetDefault("llm.primary_model", "claude-sonnet-4-20250514")
	v.SetDefault("llm.fallback_model", "gpt-4-turbo")
	v.SetDefault("llm.temperature", 0.7)
	v.SetDefault("llm.max_tokens", 2000)
	v.SetDefault("llm.enable_caching", true)
	v.SetDefault("llm.timeout", 30000)

	// Trading defaults
	v.SetDefault("trading.mode", "paper")
	v.SetDefault("trading.symbols", []string{"BTCUSDT", "ETHUSDT"})
	v.SetDefault("trading.exchange", "binance")
	v.SetDefault("trading.initial_capital", 10000.0)
	v.SetDefault("trading.max_positions", 3)
	v.SetDefault("trading.default_quantity", 0.01)

	// Risk defaults
	v.SetDefault("risk.max_position_size", 0.1)
	v.SetDefault("risk.max_daily_loss", 0.02)
	v.SetDefault("risk.max_drawdown", 0.1)
	v.SetDefault("risk.default_stop_loss", 0.02)
	v.SetDefault("risk.default_take_profit", 0.05)
	v.SetDefault("risk.llm_approval_required", true)
	v.SetDefault("risk.min_confidence", 0.7)

	// API defaults
	v.SetDefault("api.host", "0.0.0.0")
	v.SetDefault("api.port", 8081)

	// Monitoring defaults
	v.SetDefault("monitoring.prometheus_port", 9100)
	v.SetDefault("monitoring.enable_metrics", true)
}

// validate validates the configuration
func validate(cfg *Config) error {
	// Validate trading mode
	if cfg.Trading.Mode != "paper" && cfg.Trading.Mode != "live" {
		return fmt.Errorf("invalid trading mode: %s (must be 'paper' or 'live')", cfg.Trading.Mode)
	}

	// Validate symbols
	if len(cfg.Trading.Symbols) == 0 {
		return fmt.Errorf("at least one trading symbol must be configured")
	}

	// Validate capital
	if cfg.Trading.InitialCapital <= 0 {
		return fmt.Errorf("initial capital must be positive")
	}

	// Validate risk parameters
	if cfg.Risk.MaxPositionSize <= 0 || cfg.Risk.MaxPositionSize > 1 {
		return fmt.Errorf("max position size must be between 0 and 1")
	}

	if cfg.Risk.MinConfidence < 0 || cfg.Risk.MinConfidence > 1 {
		return fmt.Errorf("min confidence must be between 0 and 1")
	}

	// Validate LLM config
	if cfg.LLM.Endpoint == "" {
		return fmt.Errorf("LLM endpoint must be configured")
	}

	return nil
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// GetRedisAddr returns the Redis address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetAPIAddr returns the API server address
func (c *APIConfig) GetAPIAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetTimeout returns the LLM timeout as time.Duration
func (c *LLMConfig) GetTimeout() time.Duration {
	return time.Duration(c.Timeout) * time.Millisecond
}
