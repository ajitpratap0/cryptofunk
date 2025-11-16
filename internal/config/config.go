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
	MCP        MCPConfig                 `mapstructure:"mcp"`
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

// MCPConfig contains MCP server configuration (hybrid architecture)
type MCPConfig struct {
	External MCPExternalServers `mapstructure:"external"` // External MCP servers (CoinGecko, etc.)
	Internal MCPInternalServers `mapstructure:"internal"` // Custom MCP servers
}

// MCPExternalServers contains configuration for external MCP servers
type MCPExternalServers struct {
	CoinGecko MCPExternalServerConfig `mapstructure:"coingecko"`
}

// MCPInternalServers contains configuration for custom MCP servers
type MCPInternalServers struct {
	OrderExecutor       MCPInternalServerConfig `mapstructure:"order_executor"`
	RiskAnalyzer        MCPInternalServerConfig `mapstructure:"risk_analyzer"`
	TechnicalIndicators MCPInternalServerConfig `mapstructure:"technical_indicators"`
	MarketData          MCPInternalServerConfig `mapstructure:"market_data"`
}

// MCPExternalServerConfig contains configuration for an external MCP server
type MCPExternalServerConfig struct {
	Enabled     bool               `mapstructure:"enabled"`
	Name        string             `mapstructure:"name"`
	URL         string             `mapstructure:"url"`
	Transport   string             `mapstructure:"transport"` // "http_streaming"
	Description string             `mapstructure:"description"`
	CacheTTL    int                `mapstructure:"cache_ttl"` // seconds
	RateLimit   MCPRateLimitConfig `mapstructure:"rate_limit"`
	Tools       []string           `mapstructure:"tools"`
}

// MCPInternalServerConfig contains configuration for a custom MCP server
type MCPInternalServerConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Name        string            `mapstructure:"name"`
	Command     string            `mapstructure:"command"`   // path to binary
	Transport   string            `mapstructure:"transport"` // "stdio"
	Description string            `mapstructure:"description"`
	Args        []string          `mapstructure:"args"`
	Env         map[string]string `mapstructure:"env"`
	Tools       []string          `mapstructure:"tools"`
	Note        string            `mapstructure:"note"` // optional note
}

// MCPRateLimitConfig contains rate limit settings
type MCPRateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerMinute int  `mapstructure:"requests_per_minute"`
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
	APIKey      string     `mapstructure:"api_key"`
	SecretKey   string     `mapstructure:"secret_key"`
	Testnet     bool       `mapstructure:"testnet"`
	RateLimitMS int        `mapstructure:"rate_limit_ms"`
	Fees        FeeConfig  `mapstructure:"fees"`
}

// FeeConfig contains exchange fee structure
type FeeConfig struct {
	Maker           float64 `mapstructure:"maker"`              // Maker fee percentage (e.g., 0.001 = 0.1%)
	Taker           float64 `mapstructure:"taker"`              // Taker fee percentage (e.g., 0.001 = 0.1%)
	BaseSlippage    float64 `mapstructure:"base_slippage"`      // Base slippage percentage (e.g., 0.0005 = 0.05%)
	MarketImpact    float64 `mapstructure:"market_impact"`      // Market impact per unit (e.g., 0.0001 = 0.01%)
	MaxSlippage     float64 `mapstructure:"max_slippage"`       // Maximum slippage percentage (e.g., 0.003 = 0.3%)
	Withdrawal      float64 `mapstructure:"withdrawal"`         // Withdrawal fee percentage (optional)
}

// APIConfig contains REST API settings
type APIConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	OrchestratorURL string `mapstructure:"orchestrator_url"`
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

	// Validate configuration using comprehensive validation
	if err := cfg.Validate(); err != nil {
		return nil, err
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

	// MCP defaults - External servers
	v.SetDefault("mcp.external.coingecko.enabled", true)
	v.SetDefault("mcp.external.coingecko.name", "CoinGecko MCP")
	v.SetDefault("mcp.external.coingecko.url", "https://mcp.api.coingecko.com/mcp")
	v.SetDefault("mcp.external.coingecko.transport", "http_streaming")
	v.SetDefault("mcp.external.coingecko.cache_ttl", 60)
	v.SetDefault("mcp.external.coingecko.rate_limit.enabled", true)
	v.SetDefault("mcp.external.coingecko.rate_limit.requests_per_minute", 100)

	// MCP defaults - Internal servers
	v.SetDefault("mcp.internal.order_executor.enabled", true)
	v.SetDefault("mcp.internal.order_executor.name", "Order Executor")
	v.SetDefault("mcp.internal.order_executor.command", "./bin/order-executor-server")
	v.SetDefault("mcp.internal.order_executor.transport", "stdio")

	v.SetDefault("mcp.internal.risk_analyzer.enabled", true)
	v.SetDefault("mcp.internal.risk_analyzer.name", "Risk Analyzer")
	v.SetDefault("mcp.internal.risk_analyzer.command", "./bin/risk-analyzer-server")
	v.SetDefault("mcp.internal.risk_analyzer.transport", "stdio")

	v.SetDefault("mcp.internal.technical_indicators.enabled", true)
	v.SetDefault("mcp.internal.technical_indicators.name", "Technical Indicators")
	v.SetDefault("mcp.internal.technical_indicators.command", "./bin/technical-indicators-server")
	v.SetDefault("mcp.internal.technical_indicators.transport", "stdio")

	v.SetDefault("mcp.internal.market_data.enabled", false)
	v.SetDefault("mcp.internal.market_data.name", "Market Data (Binance)")
	v.SetDefault("mcp.internal.market_data.command", "./bin/market-data-server")
	v.SetDefault("mcp.internal.market_data.transport", "stdio")

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
	v.SetDefault("api.orchestrator_url", "http://localhost:8081")

	// Monitoring defaults
	v.SetDefault("monitoring.prometheus_port", 9100)
	v.SetDefault("monitoring.enable_metrics", true)

	// Exchange fee defaults (Binance-like structure)
	v.SetDefault("exchanges.binance.fees.maker", 0.001)          // 0.1% maker fee
	v.SetDefault("exchanges.binance.fees.taker", 0.001)          // 0.1% taker fee
	v.SetDefault("exchanges.binance.fees.base_slippage", 0.0005) // 0.05% base slippage
	v.SetDefault("exchanges.binance.fees.market_impact", 0.0001) // 0.01% market impact
	v.SetDefault("exchanges.binance.fees.max_slippage", 0.003)   // 0.3% max slippage
	v.SetDefault("exchanges.binance.fees.withdrawal", 0.0)       // No withdrawal fee by default
}

// Note: Comprehensive validation is now in validation.go
// The Config.Validate() method is called during Load()

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

// GetOrchestratorURL returns the orchestrator URL
func (c *APIConfig) GetOrchestratorURL() string {
	return c.OrchestratorURL
}

// GetTimeout returns the LLM timeout as time.Duration
func (c *LLMConfig) GetTimeout() time.Duration {
	return time.Duration(c.Timeout) * time.Millisecond
}
