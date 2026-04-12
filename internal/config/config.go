package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	App           AppConfig                 `mapstructure:"app"`
	Database      DatabaseConfig            `mapstructure:"database"`
	Redis         RedisConfig               `mapstructure:"redis"`
	NATS          NATSConfig                `mapstructure:"nats"`
	LLM           LLMConfig                 `mapstructure:"llm"`
	MCP           MCPConfig                 `mapstructure:"mcp"`
	Trading       TradingConfig             `mapstructure:"trading"`
	Risk          RiskConfig                `mapstructure:"risk"`
	Exchanges     map[string]ExchangeConfig `mapstructure:"exchanges"`
	Polymarket    PolymarketConfig          `mapstructure:"polymarket"`
	API           APIConfig                 `mapstructure:"api"`
	Monitoring    MonitoringConfig          `mapstructure:"monitoring"`
	Notifications NotificationsConfig       `mapstructure:"notifications"`
}

// PolymarketConfig contains Polymarket-specific settings
type PolymarketConfig struct {
	PrivateKey string `mapstructure:"private_key"`    // Ethereum private key for signing
	Mode       string `mapstructure:"mode"`           // "paper" or "live"
	FunderAddr string `mapstructure:"funder_address"` // Optional funder address
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
	Enabled     bool     `mapstructure:"enabled"`
	Name        string   `mapstructure:"name"`
	URL         string   `mapstructure:"url"`       // HTTP endpoint, e.g. "http://localhost:8090/mcp"
	Transport   string   `mapstructure:"transport"` // "http" (Streamable HTTP)
	Description string   `mapstructure:"description"`
	Tools       []string `mapstructure:"tools"`
	Note        string   `mapstructure:"note"` // optional note
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
	CommissionRate  float64  `mapstructure:"commission_rate"`  // 0.001 (0.1%) — paper trade taker fee
	// SlippageBuyFactor multiplies the buy price to simulate slippage (e.g. 1.001 = 0.1% slippage).
	// Defaults to 1.001 via Viper. Set to 1.0 to disable buy slippage.
	SlippageBuyFactor float64 `mapstructure:"slippage_buy_factor"`
	// SlippageSellFactor multiplies the sell price to simulate slippage (e.g. 0.999 = 0.1% slippage).
	// Defaults to 0.999 via Viper. Set to 1.0 to disable sell slippage.
	SlippageSellFactor float64 `mapstructure:"slippage_sell_factor"`
}

// RiskConfig contains risk management settings
type RiskConfig struct {
	MaxPositionSize     float64              `mapstructure:"max_position_size"`     // 0.1 (10% of portfolio)
	MaxDailyLoss        float64              `mapstructure:"max_daily_loss"`        // 0.02 (2%)
	MaxDrawdown         float64              `mapstructure:"max_drawdown"`          // 0.1 (10%)
	DefaultStopLoss     float64              `mapstructure:"default_stop_loss"`     // 0.02 (2%)
	DefaultTakeProfit   float64              `mapstructure:"default_take_profit"`   // 0.05 (5%)
	LLMApprovalRequired bool                 `mapstructure:"llm_approval_required"` // true
	MinConfidence       float64              `mapstructure:"min_confidence"`        // 0.7
	CircuitBreaker      CircuitBreakerConfig `mapstructure:"circuit_breaker"`       // Circuit breaker thresholds
	SafetyGuard         SafetyGuardConfig    `mapstructure:"safety_guard"`          // Live trading safety guards

	// Dashboard circuit breaker thresholds — used by the /risk/circuit-breakers endpoint.
	// These are expressed in absolute units: dollars for MaxDailyLossDollars,
	// percentage points for MaxDrawdownPct, and a count for MaxTradeCount.
	MaxDailyLossDollars float64 `mapstructure:"max_daily_loss_dollars"` // 5000.0 (USD)
	MaxDrawdownPct      float64 `mapstructure:"max_drawdown_pct"`       // 10.0 (percent)
	MaxTradeCount       int     `mapstructure:"max_trade_count"`        // 100
}

// SafetyGuardConfig contains safety guard settings for live trading protection
type SafetyGuardConfig struct {
	Enabled              bool               `mapstructure:"enabled"`                // Enable/disable safety guards
	MaxDailyDrawdown     float64            `mapstructure:"max_daily_drawdown"`     // Max daily drawdown before halting (e.g., 0.05 = 5%)
	MaxPositionSize      float64            `mapstructure:"max_position_size"`      // Max single position as fraction of capital (e.g., 0.1 = 10%)
	MaxTotalExposure     float64            `mapstructure:"max_total_exposure"`     // Max total exposure as fraction of capital (e.g., 0.5 = 50%)
	MaxDailyTrades       int                `mapstructure:"max_daily_trades"`       // Max trades per day (0 = unlimited)
	MaxConsecutiveLosses int                `mapstructure:"max_consecutive_losses"` // Consecutive losses before circuit breaker (e.g., 3)
	CooldownPeriod       string             `mapstructure:"cooldown_period"`        // Duration to wait after circuit breaker trips (e.g., "1h")
	MaxOrderValue        float64            `mapstructure:"max_order_value"`        // Max single order value in base currency (e.g., 10000.0)
	MinOrderInterval     string             `mapstructure:"min_order_interval"`     // Min time between orders (e.g., "5s")
	RequireConfirmation  bool               `mapstructure:"require_confirmation"`   // Require confirmation for large trades
	LargeTradeThreshold  float64            `mapstructure:"large_trade_threshold"`  // Threshold for "large trade" (fraction of capital)
	TradingHours         TradingHoursConfig `mapstructure:"trading_hours"`          // Trading hours restriction
}

// TradingHoursConfig contains trading hours restriction settings
type TradingHoursConfig struct {
	Enabled  bool   `mapstructure:"enabled"`  // Enable/disable trading hours restriction
	Start    string `mapstructure:"start"`    // Start time in HH:MM format (e.g., "09:00")
	End      string `mapstructure:"end"`      // End time in HH:MM format (e.g., "16:00")
	Timezone string `mapstructure:"timezone"` // Timezone (e.g., "America/New_York", "UTC")
}

// CircuitBreakerConfig contains circuit breaker settings for different service types
type CircuitBreakerConfig struct {
	Exchange CircuitBreakerSettings `mapstructure:"exchange"` // Exchange circuit breaker settings
	LLM      CircuitBreakerSettings `mapstructure:"llm"`      // LLM circuit breaker settings
	Database CircuitBreakerSettings `mapstructure:"database"` // Database circuit breaker settings
}

// CircuitBreakerSettings contains individual circuit breaker configuration
type CircuitBreakerSettings struct {
	MinRequests     uint32  `mapstructure:"min_requests"`       // Minimum requests before tripping
	FailureRatio    float64 `mapstructure:"failure_ratio"`      // Failure ratio threshold (0.0-1.0)
	OpenTimeout     string  `mapstructure:"open_timeout"`       // How long circuit stays open (duration string)
	HalfOpenMaxReqs uint32  `mapstructure:"half_open_max_reqs"` // Max requests in half-open state
	CountInterval   string  `mapstructure:"count_interval"`     // Window for counting failures (duration string)
}

// ExchangeConfig contains exchange-specific settings
type ExchangeConfig struct {
	APIKey      string    `mapstructure:"api_key"`
	SecretKey   string    `mapstructure:"secret_key"`
	Testnet     bool      `mapstructure:"testnet"`
	RateLimitMS int       `mapstructure:"rate_limit_ms"`
	Fees        FeeConfig `mapstructure:"fees"`
}

// FeeConfig contains exchange fee structure
type FeeConfig struct {
	Maker        float64 `mapstructure:"maker"`         // Maker fee percentage (e.g., 0.001 = 0.1%)
	Taker        float64 `mapstructure:"taker"`         // Taker fee percentage (e.g., 0.001 = 0.1%)
	BaseSlippage float64 `mapstructure:"base_slippage"` // Base slippage percentage (e.g., 0.0005 = 0.05%)
	MarketImpact float64 `mapstructure:"market_impact"` // Market impact per unit (e.g., 0.0001 = 0.01%)
	MaxSlippage  float64 `mapstructure:"max_slippage"`  // Maximum slippage percentage (e.g., 0.003 = 0.3%)
	Withdrawal   float64 `mapstructure:"withdrawal"`    // Withdrawal fee percentage (optional)
}

// APIConfig contains REST API settings
type APIConfig struct {
	Host            string     `mapstructure:"host"`
	Port            int        `mapstructure:"port"`
	OrchestratorURL string     `mapstructure:"orchestrator_url"`
	AllowedOrigins  []string   `mapstructure:"allowed_origins"` // CORS allowed origins
	Auth            AuthConfig `mapstructure:"auth"`            // Authentication configuration
}

// AuthConfig contains API authentication settings
type AuthConfig struct {
	Enabled             bool   `mapstructure:"enabled"`               // Enable API key authentication
	HeaderName          string `mapstructure:"header_name"`           // Header name for API key (default: X-API-Key)
	RequireHTTPS        bool   `mapstructure:"require_https"`         // Require HTTPS in production
	TrustForwardedProto bool   `mapstructure:"trust_forwarded_proto"` // Trust X-Forwarded-Proto from reverse proxy (default false)
	// KeyPepper is an HMAC-SHA256 pepper mixed with each API key before
	// hashing for storage (SEC-009 / #123). Leave empty for local dev
	// (legacy raw SHA-256). In production set via
	// CRYPTOFUNK_API_AUTH_KEY_PEPPER to a long random string from your
	// secret manager. Rotating the pepper invalidates every HMAC-hashed
	// key — legacy sha256 rows continue to work until rehashed.
	//
	// The json:"-" / yaml:"-" tags prevent the pepper from leaking if
	// the config struct is ever marshaled (e.g. a debug dump handler,
	// viper.WriteConfigAs, or a test harness that logs the full cfg).
	// The pepper should only flow from env → viper → in-memory struct;
	// it must never appear in logs, request bodies, or config dumps.
	KeyPepper string `mapstructure:"key_pepper" json:"-" yaml:"-"`
}

// MonitoringConfig contains monitoring settings
type MonitoringConfig struct {
	PrometheusPort int  `mapstructure:"prometheus_port"`
	EnableMetrics  bool `mapstructure:"enable_metrics"`
}

// NotificationsConfig contains notification settings for Email and Slack
type NotificationsConfig struct {
	Enabled bool                        `mapstructure:"enabled"` // Enable/disable notifications globally
	Email   EmailNotifConfig            `mapstructure:"email"`   // Email notification settings
	Slack   SlackNotifConfig            `mapstructure:"slack"`   // Slack notification settings
	Events  map[string]NotifEventConfig `mapstructure:"events"`  // Event routing configuration
}

// NotifEventConfig contains configuration for a specific notification event type
type NotifEventConfig struct {
	Enabled  bool     `mapstructure:"enabled"`  // Enable/disable this event type
	Channels []string `mapstructure:"channels"` // Channels to send to: "email", "slack"
	Priority string   `mapstructure:"priority"` // Priority: "high" or "normal"
}

// EmailNotifConfig contains email notification settings
type EmailNotifConfig struct {
	Enabled            bool     `mapstructure:"enabled"`               // Enable/disable email notifications
	Host               string   `mapstructure:"host"`                  // SMTP server host
	Port               int      `mapstructure:"port"`                  // SMTP server port
	Username           string   `mapstructure:"username"`              // SMTP username
	Password           string   `mapstructure:"password"`              // SMTP password
	FromAddress        string   `mapstructure:"from_address"`          // Sender email address
	FromName           string   `mapstructure:"from_name"`             // Sender name
	Recipients         []string `mapstructure:"recipients"`            // Default recipient email addresses
	UseTLS             bool     `mapstructure:"use_tls"`               // Use TLS from start (port 465)
	UseStartTLS        bool     `mapstructure:"use_starttls"`          // Use STARTTLS upgrade (port 587)
	SkipVerify         bool     `mapstructure:"skip_verify"`           // Skip TLS certificate verification
	AuthRequired       bool     `mapstructure:"auth_required"`         // Whether SMTP auth is required
	RateLimitPerMinute int      `mapstructure:"rate_limit_per_minute"` // Max emails per minute
	RetryAttempts      int      `mapstructure:"retry_attempts"`        // Number of retry attempts
	RetryDelaySeconds  int      `mapstructure:"retry_delay_seconds"`   // Delay between retries in seconds
}

// SlackNotifConfig contains Slack notification settings
type SlackNotifConfig struct {
	Enabled            bool   `mapstructure:"enabled"`               // Enable/disable Slack notifications
	WebhookURL         string `mapstructure:"webhook_url"`           // Slack incoming webhook URL
	Channel            string `mapstructure:"channel"`               // Default channel (optional)
	Username           string `mapstructure:"username"`              // Bot username
	IconEmoji          string `mapstructure:"icon_emoji"`            // Bot icon emoji
	IconURL            string `mapstructure:"icon_url"`              // Bot icon URL (overrides emoji)
	RateLimitPerMinute int    `mapstructure:"rate_limit_per_minute"` // Max messages per minute
	RetryAttempts      int    `mapstructure:"retry_attempts"`        // Number of retry attempts
	RetryDelaySeconds  int    `mapstructure:"retry_delay_seconds"`   // Delay between retries in seconds
	TimeoutSeconds     int    `mapstructure:"timeout_seconds"`       // HTTP request timeout
}

// Load loads configuration from file and environment variables
// If Vault is enabled, secrets are loaded from Vault; otherwise falls back to environment variables
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
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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

	// Allow Polymarket env var overrides
	if pk := os.Getenv("POLYMARKET_PRIVATE_KEY"); pk != "" {
		cfg.Polymarket.PrivateKey = pk
	}
	if pm := os.Getenv("POLYMARKET_MODE"); pm != "" {
		cfg.Polymarket.Mode = pm
	}
	if fa := os.Getenv("POLYMARKET_FUNDER_ADDRESS"); fa != "" {
		cfg.Polymarket.FunderAddr = fa
	}

	// Allow EXCHANGE_MODE env var as a convenient alias for trading.mode
	if em := os.Getenv("EXCHANGE_MODE"); em != "" {
		mode := strings.ToLower(em)
		log.Info().Str("exchange_mode", mode).Msg("Overriding trading mode from EXCHANGE_MODE env var")
		cfg.Trading.Mode = mode
	}

	// Load secrets from Vault if enabled (with fallback to env vars)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	vaultCfg := GetVaultConfigFromEnv()
	if vaultCfg.Enabled {
		log.Info().Msg("Vault integration enabled - loading secrets from Vault")
		if err := LoadSecretsFromVault(ctx, &cfg, vaultCfg); err != nil {
			log.Warn().Err(err).Msg("Failed to load secrets from Vault - falling back to environment variables")
			// Continue with env vars as fallback
		}
	} else {
		log.Info().Msg("Vault integration disabled - using environment variables for secrets")
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

	// MCP defaults - Internal servers (Streamable HTTP transport)
	v.SetDefault("mcp.internal.order_executor.enabled", true)
	v.SetDefault("mcp.internal.order_executor.name", "Order Executor")
	v.SetDefault("mcp.internal.order_executor.url", "http://localhost:8091/mcp")
	v.SetDefault("mcp.internal.order_executor.transport", "http")

	v.SetDefault("mcp.internal.risk_analyzer.enabled", true)
	v.SetDefault("mcp.internal.risk_analyzer.name", "Risk Analyzer")
	v.SetDefault("mcp.internal.risk_analyzer.url", "http://localhost:8092/mcp")
	v.SetDefault("mcp.internal.risk_analyzer.transport", "http")

	v.SetDefault("mcp.internal.technical_indicators.enabled", true)
	v.SetDefault("mcp.internal.technical_indicators.name", "Technical Indicators")
	v.SetDefault("mcp.internal.technical_indicators.url", "http://localhost:8093/mcp")
	v.SetDefault("mcp.internal.technical_indicators.transport", "http")

	v.SetDefault("mcp.internal.market_data.enabled", false)
	v.SetDefault("mcp.internal.market_data.name", "Market Data (Binance)")
	v.SetDefault("mcp.internal.market_data.url", "http://localhost:8090/mcp")
	v.SetDefault("mcp.internal.market_data.transport", "http")

	v.SetDefault("mcp.internal.polymarket.enabled", true)
	v.SetDefault("mcp.internal.polymarket.name", "Polymarket")
	v.SetDefault("mcp.internal.polymarket.url", "http://localhost:8094/mcp")
	v.SetDefault("mcp.internal.polymarket.transport", "http")

	// Trading defaults
	v.SetDefault("trading.mode", "paper")
	v.SetDefault("trading.symbols", []string{"BTCUSDT", "ETHUSDT"})
	v.SetDefault("trading.exchange", "binance")
	v.SetDefault("trading.initial_capital", 10000.0)
	v.SetDefault("trading.max_positions", 3)
	v.SetDefault("trading.default_quantity", 0.01)
	v.SetDefault("trading.commission_rate", 0.001)      // 0.1% taker fee (Binance standard tier)
	v.SetDefault("trading.slippage_buy_factor", 1.001)  // 0.1% adverse slippage on paper buys
	v.SetDefault("trading.slippage_sell_factor", 0.999) // 0.1% adverse slippage on paper sells

	// Risk defaults
	v.SetDefault("risk.max_position_size", 0.1)
	v.SetDefault("risk.max_daily_loss", 0.02)
	v.SetDefault("risk.max_drawdown", 0.1)
	v.SetDefault("risk.default_stop_loss", 0.02)
	v.SetDefault("risk.default_take_profit", 0.05)
	v.SetDefault("risk.llm_approval_required", true)
	v.SetDefault("risk.min_confidence", 0.7)

	// Dashboard circuit breaker threshold defaults (absolute units)
	v.SetDefault("risk.max_daily_loss_dollars", 5000.0)
	v.SetDefault("risk.max_drawdown_pct", 10.0)
	v.SetDefault("risk.max_trade_count", 100)

	// Circuit Breaker defaults - aligned with configs/circuit_breakers.yaml
	// Exchange
	v.SetDefault("risk.circuit_breaker.exchange.min_requests", 5)
	v.SetDefault("risk.circuit_breaker.exchange.failure_ratio", 0.6)
	v.SetDefault("risk.circuit_breaker.exchange.open_timeout", "60s")
	v.SetDefault("risk.circuit_breaker.exchange.half_open_max_reqs", 2)
	v.SetDefault("risk.circuit_breaker.exchange.count_interval", "10s")

	// LLM
	v.SetDefault("risk.circuit_breaker.llm.min_requests", 3)
	v.SetDefault("risk.circuit_breaker.llm.failure_ratio", 0.6)
	v.SetDefault("risk.circuit_breaker.llm.open_timeout", "30s")
	v.SetDefault("risk.circuit_breaker.llm.half_open_max_reqs", 2)
	v.SetDefault("risk.circuit_breaker.llm.count_interval", "10s")

	// Database
	v.SetDefault("risk.circuit_breaker.database.min_requests", 5)
	v.SetDefault("risk.circuit_breaker.database.failure_ratio", 0.6)
	v.SetDefault("risk.circuit_breaker.database.open_timeout", "15s")
	v.SetDefault("risk.circuit_breaker.database.half_open_max_reqs", 5)
	v.SetDefault("risk.circuit_breaker.database.count_interval", "10s")

	// Safety Guard defaults - Live trading protection
	v.SetDefault("risk.safety_guard.enabled", true)               // Enabled by default
	v.SetDefault("risk.safety_guard.max_daily_drawdown", 0.05)    // 5% max daily drawdown
	v.SetDefault("risk.safety_guard.max_position_size", 0.1)      // 10% max position size
	v.SetDefault("risk.safety_guard.max_total_exposure", 0.5)     // 50% max total exposure
	v.SetDefault("risk.safety_guard.max_daily_trades", 50)        // 50 trades per day
	v.SetDefault("risk.safety_guard.max_consecutive_losses", 3)   // 3 consecutive losses trigger circuit breaker
	v.SetDefault("risk.safety_guard.cooldown_period", "1h")       // 1 hour cooldown after circuit breaker
	v.SetDefault("risk.safety_guard.max_order_value", 10000.0)    // $10,000 max order value
	v.SetDefault("risk.safety_guard.min_order_interval", "5s")    // 5 seconds between orders
	v.SetDefault("risk.safety_guard.require_confirmation", true)  // Require confirmation for large trades
	v.SetDefault("risk.safety_guard.large_trade_threshold", 0.05) // 5% of capital is "large trade"

	// Trading Hours defaults - Optional time-based trading restrictions
	v.SetDefault("risk.safety_guard.trading_hours.enabled", false)               // Disabled by default
	v.SetDefault("risk.safety_guard.trading_hours.start", "09:00")               // Default market open
	v.SetDefault("risk.safety_guard.trading_hours.end", "16:00")                 // Default market close
	v.SetDefault("risk.safety_guard.trading_hours.timezone", "America/New_York") // Default to Eastern Time

	// API defaults
	v.SetDefault("api.host", "0.0.0.0")
	v.SetDefault("api.port", 8081)
	v.SetDefault("api.orchestrator_url", "http://localhost:8081")

	// API Authentication defaults
	// Authentication is disabled by default for development/testing
	// For production deployments, set api.auth.enabled = true in config.yaml
	v.SetDefault("api.auth.enabled", false)
	v.SetDefault("api.auth.header_name", "X-API-Key")
	v.SetDefault("api.auth.require_https", true)
	v.SetDefault("api.auth.trust_forwarded_proto", false)
	v.SetDefault("api.auth.key_pepper", "")

	// Polymarket defaults
	v.SetDefault("polymarket.mode", "paper")

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

	// Notifications defaults
	v.SetDefault("notifications.enabled", true)

	// Email notification defaults
	v.SetDefault("notifications.email.enabled", false) // Disabled by default until configured
	v.SetDefault("notifications.email.port", 587)
	v.SetDefault("notifications.email.from_name", "CryptoFunk Trading")
	v.SetDefault("notifications.email.use_tls", false)
	v.SetDefault("notifications.email.use_starttls", true)
	v.SetDefault("notifications.email.skip_verify", false)
	v.SetDefault("notifications.email.auth_required", true)
	v.SetDefault("notifications.email.rate_limit_per_minute", 30)
	v.SetDefault("notifications.email.retry_attempts", 3)
	v.SetDefault("notifications.email.retry_delay_seconds", 5)

	// Slack notification defaults
	v.SetDefault("notifications.slack.enabled", false) // Disabled by default until configured
	v.SetDefault("notifications.slack.username", "CryptoFunk Bot")
	v.SetDefault("notifications.slack.icon_emoji", ":chart_with_upwards_trend:")
	v.SetDefault("notifications.slack.rate_limit_per_minute", 30)
	v.SetDefault("notifications.slack.retry_attempts", 3)
	v.SetDefault("notifications.slack.retry_delay_seconds", 2)
	v.SetDefault("notifications.slack.timeout_seconds", 30)

	// Event routing defaults - which events go to which channels
	// trade_executed: sent to both email and slack (high priority trades)
	v.SetDefault("notifications.events.trade_executed.enabled", true)
	v.SetDefault("notifications.events.trade_executed.channels", []string{"email", "slack"})
	v.SetDefault("notifications.events.trade_executed.priority", "high")

	// position_closed: sent to slack only (frequent updates)
	v.SetDefault("notifications.events.position_closed.enabled", true)
	v.SetDefault("notifications.events.position_closed.channels", []string{"slack"})
	v.SetDefault("notifications.events.position_closed.priority", "normal")

	// safety_guard: sent to both (critical alerts)
	v.SetDefault("notifications.events.safety_guard.enabled", true)
	v.SetDefault("notifications.events.safety_guard.channels", []string{"email", "slack"})
	v.SetDefault("notifications.events.safety_guard.priority", "high")

	// system_error: sent to both (critical errors)
	v.SetDefault("notifications.events.system_error.enabled", true)
	v.SetDefault("notifications.events.system_error.channels", []string{"email", "slack"})
	v.SetDefault("notifications.events.system_error.priority", "high")

	// daily_summary: sent to email only (daily digest)
	v.SetDefault("notifications.events.daily_summary.enabled", true)
	v.SetDefault("notifications.events.daily_summary.channels", []string{"email"})
	v.SetDefault("notifications.events.daily_summary.priority", "normal")

	// pnl_alert: sent to slack for quick visibility
	v.SetDefault("notifications.events.pnl_alert.enabled", true)
	v.SetDefault("notifications.events.pnl_alert.channels", []string{"slack"})
	v.SetDefault("notifications.events.pnl_alert.priority", "high")

	// circuit_breaker: sent to both (critical)
	v.SetDefault("notifications.events.circuit_breaker.enabled", true)
	v.SetDefault("notifications.events.circuit_breaker.channels", []string{"email", "slack"})
	v.SetDefault("notifications.events.circuit_breaker.priority", "high")

	// consensus_failure: sent to slack (informational)
	v.SetDefault("notifications.events.consensus_failure.enabled", true)
	v.SetDefault("notifications.events.consensus_failure.channels", []string{"slack"})
	v.SetDefault("notifications.events.consensus_failure.priority", "normal")
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

// ParseDuration parses a duration string and returns the duration or zero if invalid
func (s *CircuitBreakerSettings) ParseDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, nil
	}
	return time.ParseDuration(durationStr)
}

// GetOpenTimeout returns the OpenTimeout as time.Duration, returns zero on parse error
func (s *CircuitBreakerSettings) GetOpenTimeout() time.Duration {
	duration, _ := s.ParseDuration(s.OpenTimeout)
	return duration
}

// GetCountInterval returns the CountInterval as time.Duration, returns zero on parse error
func (s *CircuitBreakerSettings) GetCountInterval() time.Duration {
	duration, _ := s.ParseDuration(s.CountInterval)
	return duration
}

// ToRiskSettings converts CircuitBreakerSettings to risk.CircuitBreakerConfigSettings format
// This enables the config package to provide settings to the risk package without import cycles
type RiskCircuitBreakerSettings struct {
	MinRequests     uint32
	FailureRatio    float64
	OpenTimeout     string
	HalfOpenMaxReqs uint32
	CountInterval   string
}

// ToRiskSettings converts CircuitBreakerSettings to RiskCircuitBreakerSettings
func (s *CircuitBreakerSettings) ToRiskSettings() *RiskCircuitBreakerSettings {
	return &RiskCircuitBreakerSettings{
		MinRequests:     s.MinRequests,
		FailureRatio:    s.FailureRatio,
		OpenTimeout:     s.OpenTimeout,
		HalfOpenMaxReqs: s.HalfOpenMaxReqs,
		CountInterval:   s.CountInterval,
	}
}

// GetExchangeRiskSettings returns exchange circuit breaker settings for the risk package
func (c *CircuitBreakerConfig) GetExchangeRiskSettings() *RiskCircuitBreakerSettings {
	return c.Exchange.ToRiskSettings()
}

// GetLLMRiskSettings returns LLM circuit breaker settings for the risk package
func (c *CircuitBreakerConfig) GetLLMRiskSettings() *RiskCircuitBreakerSettings {
	return c.LLM.ToRiskSettings()
}

// GetDatabaseRiskSettings returns database circuit breaker settings for the risk package
func (c *CircuitBreakerConfig) GetDatabaseRiskSettings() *RiskCircuitBreakerSettings {
	return c.Database.ToRiskSettings()
}

// IsConfigured returns true if the circuit breaker settings have valid values
func (s *RiskCircuitBreakerSettings) IsConfigured() bool {
	return s.MinRequests > 0 || s.FailureRatio > 0 || s.OpenTimeout != "" || s.HalfOpenMaxReqs > 0
}

// GetCooldownPeriod returns the cooldown period as time.Duration
func (c *SafetyGuardConfig) GetCooldownPeriod() time.Duration {
	if c.CooldownPeriod == "" {
		return time.Hour // Default 1 hour
	}
	duration, err := time.ParseDuration(c.CooldownPeriod)
	if err != nil {
		return time.Hour // Default on parse error
	}
	return duration
}

// GetMinOrderInterval returns the minimum order interval as time.Duration
func (c *SafetyGuardConfig) GetMinOrderInterval() time.Duration {
	if c.MinOrderInterval == "" {
		return 5 * time.Second // Default 5 seconds
	}
	duration, err := time.ParseDuration(c.MinOrderInterval)
	if err != nil {
		return 5 * time.Second // Default on parse error
	}
	return duration
}

// IsLargeOrder returns true if the order value exceeds the large trade threshold
func (c *SafetyGuardConfig) IsLargeOrder(orderValue, capital float64) bool {
	if capital <= 0 || c.LargeTradeThreshold <= 0 {
		return false
	}
	return orderValue/capital >= c.LargeTradeThreshold
}

// IsWithinTradingHours checks if the current time is within configured trading hours
// Returns true if trading hours are disabled or if within the configured window
func (c *TradingHoursConfig) IsWithinTradingHours(now time.Time) bool {
	if !c.Enabled {
		return true // Trading hours restriction is disabled
	}

	// Parse timezone
	loc := time.UTC // Default to UTC
	if c.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(c.Timezone)
		if err != nil {
			// If timezone parsing fails, default to UTC and log would happen in caller
			loc = time.UTC
		}
	}

	// Convert current time to configured timezone
	localTime := now.In(loc)

	// Parse start and end times
	startTime, err := time.Parse("15:04", c.Start)
	if err != nil {
		return true // Invalid start time, allow trading
	}
	endTime, err := time.Parse("15:04", c.End)
	if err != nil {
		return true // Invalid end time, allow trading
	}

	// Create today's start and end times in the local timezone
	todayStart := time.Date(localTime.Year(), localTime.Month(), localTime.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, loc)
	todayEnd := time.Date(localTime.Year(), localTime.Month(), localTime.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, loc)

	// Handle overnight trading (e.g., start: 20:00, end: 04:00)
	if todayEnd.Before(todayStart) || todayEnd.Equal(todayStart) {
		// Trading window spans midnight
		// Check if current time is after start OR before end
		return localTime.After(todayStart) || localTime.Before(todayEnd) || localTime.Equal(todayStart)
	}

	// Normal trading window (same day)
	return (localTime.After(todayStart) || localTime.Equal(todayStart)) && localTime.Before(todayEnd)
}

// GetLocation returns the configured timezone location
func (c *TradingHoursConfig) GetLocation() *time.Location {
	if c.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}
