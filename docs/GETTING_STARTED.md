# Getting Started with CryptoFunk

This guide will walk you through setting up your development environment and building your first MCP server and agent.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Initial Setup](#initial-setup)
3. [Phase 1: Project Foundation](#phase-1-project-foundation)
4. [Building Your First MCP Server](#building-your-first-mcp-server)
5. [Building Your First Agent](#building-your-first-agent)
6. [Testing the Communication](#testing-the-communication)
7. [Next Steps](#next-steps)

---

## Prerequisites

### Required Software

```bash
# Go 1.21 or higher
go version

# Docker and Docker Compose
docker --version
docker-compose --version

# PostgreSQL client (for database management)
psql --version

# Optional but recommended
make --version
git --version
```

### API Keys

1. **Binance Testnet** (for development)
   - Sign up at: https://testnet.binance.vision/
   - Create API key and secret
   - No real funds required

2. **News API** (optional, for sentiment agent)
   - Sign up at: https://newsapi.org/
   - Free tier available

---

## Initial Setup

### 1. Project Initialization

```bash
# Navigate to your project directory
cd /Users/ajitpratapsingh/dev/cryptofunk

# Initialize Go module
go mod init github.com/ajitpratap0/cryptofunk

# Add the official MCP Go SDK
go get github.com/modelcontextprotocol/go-sdk

# Add other core dependencies
go get github.com/spf13/viper          # Configuration
go get github.com/rs/zerolog           # Logging
go get github.com/adshao/go-binance/v2 # Binance API
go get github.com/redis/go-redis/v9    # Redis client
go get github.com/jackc/pgx/v5         # PostgreSQL driver
```

### 2. Create Project Structure

```bash
# Create directory structure
mkdir -p cmd/{orchestrator,api,mcp-servers/{market-data,technical-indicators,risk-analyzer,order-executor},agents/{technical-agent,orderbook-agent,trend-agent,reversion-agent,risk-agent}}

mkdir -p internal/{orchestrator,mcpserver,mcpclient,agents,indicators,exchange/binance,models,config}

mkdir -p pkg/{events,utils}

mkdir -p configs
mkdir -p deployments/{docker,k8s}
mkdir -p scripts
mkdir -p migrations
mkdir -p docs
mkdir -p test/e2e

# Create go files
touch cmd/orchestrator/main.go
touch internal/config/config.go
```

### 3. Setup Environment

```bash
# Create .env file
cat > .env << 'EOF'
# Exchange API Keys
BINANCE_API_KEY=your_testnet_api_key
BINANCE_SECRET_KEY=your_testnet_secret_key

# Database
POSTGRES_PASSWORD=cryptofunk_dev
POSTGRES_USER=postgres
POSTGRES_DB=cryptofunk

# Redis
REDIS_PASSWORD=redis_dev

# Application
APP_ENV=development
LOG_LEVEL=debug

# News API (optional)
NEWS_API_KEY=your_news_api_key
EOF

# IMPORTANT: Add .env to .gitignore
echo ".env" >> .gitignore
```

### 4. Create Configuration File

```bash
cat > configs/config.yaml << 'EOF'
app:
  name: "cryptofunk"
  version: "0.1.0"
  environment: "development"
  log_level: "debug"

trading:
  mode: "paper"
  symbols:
    - "BTCUSDT"
  update_interval: "60s"

exchange:
  binance:
    api_key: "${BINANCE_API_KEY}"
    secret_key: "${BINANCE_SECRET_KEY}"
    testnet: true
    base_url: "https://testnet.binance.vision"

database:
  postgres:
    host: "localhost"
    port: 5432
    database: "cryptofunk"
    user: "postgres"
    password: "${POSTGRES_PASSWORD}"
    ssl_mode: "disable"

  redis:
    host: "localhost"
    port: 6379
    password: "${REDIS_PASSWORD}"
    db: 0

logging:
  format: "console"
  level: "debug"
  output: "stdout"
EOF
```

### 5. Setup Docker Services

```bash
cat > docker-compose.yml << 'EOF'
version: '3.9'

services:
  postgres:
    image: timescale/timescaledb:latest-pg15
    environment:
      POSTGRES_DB: cryptofunk
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "--raw", "incr", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

volumes:
  postgres_data:
  redis_data:
EOF

# Start services
docker-compose up -d

# Verify services are running
docker-compose ps
```

### 6. Setup Database Schema

```bash
cat > migrations/001_initial_schema.sql << 'EOF'
-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Candlesticks table
CREATE TABLE IF NOT EXISTS candlesticks (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    interval VARCHAR(10) NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    open DECIMAL(20, 8) NOT NULL,
    high DECIMAL(20, 8) NOT NULL,
    low DECIMAL(20, 8) NOT NULL,
    close DECIMAL(20, 8) NOT NULL,
    volume DECIMAL(20, 8) NOT NULL,
    close_time TIMESTAMPTZ NOT NULL,
    quote_volume DECIMAL(20, 8),
    trades INTEGER,
    CONSTRAINT candlesticks_unique UNIQUE (symbol, interval, open_time)
);

-- Convert to hypertable
SELECT create_hypertable('candlesticks', 'open_time', if_not_exists => TRUE);

-- Create index for faster queries
CREATE INDEX IF NOT EXISTS idx_candlesticks_symbol_interval
    ON candlesticks (symbol, interval, open_time DESC);

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(100) UNIQUE NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    type VARCHAR(20) NOT NULL,
    quantity DECIMAL(20, 8) NOT NULL,
    price DECIMAL(20, 8),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    filled_quantity DECIMAL(20, 8) DEFAULT 0,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders (symbol, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);

-- Trades table
CREATE TABLE IF NOT EXISTS trades (
    id BIGSERIAL PRIMARY KEY,
    trade_id VARCHAR(100) UNIQUE NOT NULL,
    order_id VARCHAR(100) REFERENCES orders(order_id),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    quantity DECIMAL(20, 8) NOT NULL,
    price DECIMAL(20, 8) NOT NULL,
    commission DECIMAL(20, 8) DEFAULT 0,
    pnl DECIMAL(20, 8),
    executed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades (symbol, executed_at DESC);

-- Positions table
CREATE TABLE IF NOT EXISTS positions (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL UNIQUE,
    side VARCHAR(10) NOT NULL,
    quantity DECIMAL(20, 8) NOT NULL,
    entry_price DECIMAL(20, 8) NOT NULL,
    current_price DECIMAL(20, 8),
    unrealized_pnl DECIMAL(20, 8) DEFAULT 0,
    realized_pnl DECIMAL(20, 8) DEFAULT 0,
    opened_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);

-- Agent signals table
CREATE TABLE IF NOT EXISTS agent_signals (
    id BIGSERIAL PRIMARY KEY,
    agent_name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    signal VARCHAR(20) NOT NULL,
    confidence DECIMAL(5, 4),
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_signals_agent ON agent_signals (agent_name, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_signals_symbol ON agent_signals (symbol, created_at DESC);

-- Trading sessions table
CREATE TABLE IF NOT EXISTS trading_sessions (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID UNIQUE NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    decision VARCHAR(20),
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON trading_sessions (status, started_at DESC);
EOF

# Run migrations
docker exec -i $(docker-compose ps -q postgres) psql -U postgres -d cryptofunk < migrations/001_initial_schema.sql

# Verify tables created
docker exec -it $(docker-compose ps -q postgres) psql -U postgres -d cryptofunk -c "\dt"
```

---

## Phase 1: Project Foundation

### Configuration Management

Create `internal/config/config.go`:

```go
package config

import (
    "fmt"
    "os"
    "strings"

    "github.com/spf13/viper"
)

type Config struct {
    App      AppConfig
    Trading  TradingConfig
    Exchange ExchangeConfig
    Database DatabaseConfig
    Logging  LoggingConfig
}

type AppConfig struct {
    Name        string
    Version     string
    Environment string
    LogLevel    string
}

type TradingConfig struct {
    Mode           string
    Symbols        []string
    UpdateInterval string
}

type ExchangeConfig struct {
    Binance BinanceConfig
}

type BinanceConfig struct {
    APIKey    string
    SecretKey string
    Testnet   bool
    BaseURL   string
}

type DatabaseConfig struct {
    Postgres PostgresConfig
    Redis    RedisConfig
}

type PostgresConfig struct {
    Host     string
    Port     int
    Database string
    User     string
    Password string
    SSLMode  string
}

type RedisConfig struct {
    Host     string
    Port     int
    Password string
    DB       int
}

type LoggingConfig struct {
    Format string
    Level  string
    Output string
}

func Load() (*Config, error) {
    // Set config file
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./configs")
    viper.AddConfigPath(".")

    // Enable environment variable override
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    viper.AutomaticEnv()

    // Read config
    if err := viper.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    // Unmarshal into struct
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }

    // Override with environment variables
    cfg.Exchange.Binance.APIKey = getEnv("BINANCE_API_KEY", cfg.Exchange.Binance.APIKey)
    cfg.Exchange.Binance.SecretKey = getEnv("BINANCE_SECRET_KEY", cfg.Exchange.Binance.SecretKey)
    cfg.Database.Postgres.Password = getEnv("POSTGRES_PASSWORD", cfg.Database.Postgres.Password)
    cfg.Database.Redis.Password = getEnv("REDIS_PASSWORD", cfg.Database.Redis.Password)

    return &cfg, nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### Logging Setup

Create `internal/config/logger.go`:

```go
package config

import (
    "os"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func SetupLogger(cfg LoggingConfig) {
    // Set log level
    level, err := zerolog.ParseLevel(cfg.Level)
    if err != nil {
        level = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(level)

    // Set format
    if cfg.Format == "console" {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
    }

    log.Info().
        Str("level", level.String()).
        Str("format", cfg.Format).
        Msg("Logger initialized")
}
```

---

## Building Your First MCP Server

### Market Data Server

Create `cmd/mcp-servers/market-data/main.go`:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/adshao/go-binance/v2"
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/rs/zerolog/log"
)

type MarketDataServer struct {
    client *binance.Client
}

// GetCurrentPrice tool input/output
type GetPriceInput struct {
    Symbol string `json:"symbol" jsonschema:"required,description=Trading pair symbol (e.g. BTCUSDT)"`
}

type GetPriceOutput struct {
    Symbol string  `json:"symbol"`
    Price  float64 `json:"price"`
    Time   string  `json:"time"`
}

func (s *MarketDataServer) getCurrentPrice(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetPriceInput,
) (*mcp.CallToolResult, GetPriceOutput, error) {
    log.Info().Str("symbol", input.Symbol).Msg("Getting current price")

    // Get ticker price from Binance
    prices, err := s.client.NewListPricesService().Symbol(input.Symbol).Do(ctx)
    if err != nil {
        log.Error().Err(err).Msg("Failed to get price from Binance")
        return nil, GetPriceOutput{}, fmt.Errorf("failed to get price: %w", err)
    }

    if len(prices) == 0 {
        return nil, GetPriceOutput{}, fmt.Errorf("no price found for symbol %s", input.Symbol)
    }

    // Parse price
    var price float64
    fmt.Sscanf(prices[0].Price, "%f", &price)

    output := GetPriceOutput{
        Symbol: input.Symbol,
        Price:  price,
        Time:   time.Now().Format(time.RFC3339),
    }

    log.Info().
        Str("symbol", input.Symbol).
        Float64("price", price).
        Msg("Price retrieved")

    return nil, output, nil
}

// GetCandlesticks tool
type GetCandlesticksInput struct {
    Symbol   string `json:"symbol" jsonschema:"required,description=Trading pair symbol"`
    Interval string `json:"interval" jsonschema:"required,description=Time interval (1m, 5m, 15m, 1h, 4h, 1d)"`
    Limit    int    `json:"limit" jsonschema:"description=Number of candles (default 100, max 1000)"`
}

type Candlestick struct {
    OpenTime  int64   `json:"open_time"`
    Open      float64 `json:"open"`
    High      float64 `json:"high"`
    Low       float64 `json:"low"`
    Close     float64 `json:"close"`
    Volume    float64 `json:"volume"`
    CloseTime int64   `json:"close_time"`
}

type GetCandlesticksOutput struct {
    Symbol       string        `json:"symbol"`
    Interval     string        `json:"interval"`
    Candlesticks []Candlestick `json:"candlesticks"`
}

func (s *MarketDataServer) getCandlesticks(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetCandlesticksInput,
) (*mcp.CallToolResult, GetCandlesticksOutput, error) {
    log.Info().
        Str("symbol", input.Symbol).
        Str("interval", input.Interval).
        Int("limit", input.Limit).
        Msg("Getting candlesticks")

    if input.Limit == 0 {
        input.Limit = 100
    }

    // Get klines from Binance
    klines, err := s.client.NewKlinesService().
        Symbol(input.Symbol).
        Interval(input.Interval).
        Limit(input.Limit).
        Do(ctx)
    if err != nil {
        return nil, GetCandlesticksOutput{}, fmt.Errorf("failed to get candlesticks: %w", err)
    }

    // Convert to our format
    candlesticks := make([]Candlestick, len(klines))
    for i, k := range klines {
        var open, high, low, close, volume float64
        fmt.Sscanf(k.Open, "%f", &open)
        fmt.Sscanf(k.High, "%f", &high)
        fmt.Sscanf(k.Low, "%f", &low)
        fmt.Sscanf(k.Close, "%f", &close)
        fmt.Sscanf(k.Volume, "%f", &volume)

        candlesticks[i] = Candlestick{
            OpenTime:  k.OpenTime,
            Open:      open,
            High:      high,
            Low:       low,
            Close:     close,
            Volume:    volume,
            CloseTime: k.CloseTime,
        }
    }

    output := GetCandlesticksOutput{
        Symbol:       input.Symbol,
        Interval:     input.Interval,
        Candlesticks: candlesticks,
    }

    log.Info().
        Str("symbol", input.Symbol).
        Int("count", len(candlesticks)).
        Msg("Candlesticks retrieved")

    return nil, output, nil
}

func main() {
    // Get API credentials from environment
    apiKey := os.Getenv("BINANCE_API_KEY")
    secretKey := os.Getenv("BINANCE_SECRET_KEY")

    if apiKey == "" || secretKey == "" {
        log.Fatal().Msg("BINANCE_API_KEY and BINANCE_SECRET_KEY must be set")
    }

    // Create Binance client (testnet)
    binance.UseTestnet = true
    client := binance.NewClient(apiKey, secretKey)

    // Create market data server
    marketServer := &MarketDataServer{
        client: client,
    }

    // Create MCP server
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "market-data-server",
        Version: "0.1.0",
    }, nil)

    // Add tools
    mcp.AddTool(server, &mcp.Tool{
        Name:        "get_current_price",
        Description: "Get the current ticker price for a trading pair",
    }, marketServer.getCurrentPrice)

    mcp.AddTool(server, &mcp.Tool{
        Name:        "get_candlesticks",
        Description: "Get historical candlestick data for a trading pair",
    }, marketServer.getCandlesticks)

    log.Info().Msg("Market Data Server starting...")

    // Run server over stdio
    transport := mcp.NewStdioTransport()
    if err := server.Run(transport); err != nil {
        log.Fatal().Err(err).Msg("Server failed")
    }
}
```

### Build and Test

```bash
# Build the server
go build -o bin/market-data-server cmd/mcp-servers/market-data/main.go

# Test it manually (it reads from stdin)
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/market-data-server

# You should see the list of available tools
```

---

## Building Your First Agent

### Technical Analysis Agent

Create `cmd/agents/technical-agent/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/rs/zerolog/log"
)

type TechnicalAgent struct {
    marketClient *mcp.Client
}

type Signal struct {
    Symbol     string             `json:"symbol"`
    Signal     string             `json:"signal"` // BUY, SELL, HOLD
    Confidence float64            `json:"confidence"`
    Indicators map[string]float64 `json:"indicators"`
    Timestamp  string             `json:"timestamp"`
}

func (a *TechnicalAgent) Analyze(ctx context.Context, symbol string) (*Signal, error) {
    log.Info().Str("symbol", symbol).Msg("Starting technical analysis")

    // 1. Get current price
    priceResp, err := a.marketClient.CallTool(ctx, &mcp.CallToolRequest{
        Params: mcp.CallToolRequestParams{
            Name: "get_current_price",
            Arguments: map[string]any{
                "symbol": symbol,
            },
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get price: %w", err)
    }

    // Parse price response
    var priceData struct {
        Symbol string  `json:"symbol"`
        Price  float64 `json:"price"`
    }
    if err := json.Unmarshal([]byte(priceResp.Content[0].Text), &priceData); err != nil {
        return nil, fmt.Errorf("failed to parse price: %w", err)
    }

    log.Info().
        Str("symbol", symbol).
        Float64("price", priceData.Price).
        Msg("Current price retrieved")

    // 2. Get candlesticks for analysis
    candlesResp, err := a.marketClient.CallTool(ctx, &mcp.CallToolRequest{
        Params: mcp.CallToolRequestParams{
            Name: "get_candlesticks",
            Arguments: map[string]any{
                "symbol":   symbol,
                "interval": "1h",
                "limit":    50,
            },
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get candlesticks: %w", err)
    }

    // Parse candlesticks
    var candleData struct {
        Symbol       string `json:"symbol"`
        Interval     string `json:"interval"`
        Candlesticks []struct {
            Close float64 `json:"close"`
        } `json:"candlesticks"`
    }
    if err := json.Unmarshal([]byte(candlesResp.Content[0].Text), &candleData); err != nil {
        return nil, fmt.Errorf("failed to parse candlesticks: %w", err)
    }

    log.Info().
        Str("symbol", symbol).
        Int("candles", len(candleData.Candlesticks)).
        Msg("Candlesticks retrieved")

    // 3. Simple analysis (just an example - calculate simple moving average)
    var sum float64
    for _, candle := range candleData.Candlesticks {
        sum += candle.Close
    }
    sma := sum / float64(len(candleData.Candlesticks))

    // Determine signal
    var signal string
    var confidence float64

    if priceData.Price > sma*1.02 { // Price 2% above SMA
        signal = "BUY"
        confidence = 0.7
    } else if priceData.Price < sma*0.98 { // Price 2% below SMA
        signal = "SELL"
        confidence = 0.7
    } else {
        signal = "HOLD"
        confidence = 0.5
    }

    result := &Signal{
        Symbol:     symbol,
        Signal:     signal,
        Confidence: confidence,
        Indicators: map[string]float64{
            "price": priceData.Price,
            "sma":   sma,
        },
        Timestamp: time.Now().Format(time.RFC3339),
    }

    log.Info().
        Str("symbol", symbol).
        Str("signal", signal).
        Float64("confidence", confidence).
        Msg("Analysis complete")

    return result, nil
}

func main() {
    log.Info().Msg("Technical Agent starting...")

    // Create MCP client to connect to market data server
    client := mcp.NewClient(nil)

    // Connect to market data server via stdio
    // In production, this path would come from config
    transport := mcp.NewCommandTransport("./bin/market-data-server")

    // Run client in background
    go func() {
        if err := client.Run(transport); err != nil {
            log.Fatal().Err(err).Msg("Client connection failed")
        }
    }()

    // Give client time to connect
    time.Sleep(2 * time.Second)

    // Create agent
    agent := &TechnicalAgent{
        marketClient: client,
    }

    ctx := context.Background()

    // Setup graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Main analysis loop
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    // Run analysis immediately
    symbol := "BTCUSDT"
    signal, err := agent.Analyze(ctx, symbol)
    if err != nil {
        log.Error().Err(err).Msg("Analysis failed")
    } else {
        // Print signal as JSON
        signalJSON, _ := json.MarshalIndent(signal, "", "  ")
        fmt.Println(string(signalJSON))
    }

    // Continue running and analyzing periodically
    for {
        select {
        case <-ticker.C:
            signal, err := agent.Analyze(ctx, symbol)
            if err != nil {
                log.Error().Err(err).Msg("Analysis failed")
                continue
            }

            signalJSON, _ := json.MarshalIndent(signal, "", "  ")
            fmt.Println(string(signalJSON))

        case <-sigChan:
            log.Info().Msg("Shutting down...")
            return
        }
    }
}
```

---

## Testing the Communication

### 1. Build Both Components

```bash
# Build market data server
go build -o bin/market-data-server cmd/mcp-servers/market-data/main.go

# Build technical agent
go build -o bin/technical-agent cmd/agents/technical-agent/main.go
```

### 2. Set Environment Variables

```bash
# Load from .env
export $(cat .env | grep -v '^#' | xargs)
```

### 3. Run the Agent

The agent will automatically start the market data server as a subprocess:

```bash
./bin/technical-agent
```

You should see output like:

```json
{
  "symbol": "BTCUSDT",
  "signal": "HOLD",
  "confidence": 0.5,
  "indicators": {
    "price": 43250.50,
    "sma": 43150.20
  },
  "timestamp": "2025-10-27T10:30:00Z"
}
```

### 4. Monitor Logs

The agent will continue running and analyze every 60 seconds. Watch the logs:

```bash
# In the terminal you'll see:
# - MCP client connecting to server
# - Tool calls being made
# - Signals being generated
```

---

## Next Steps

### Immediate Next Steps

1. **Add More Indicators** to the technical agent
   - RSI calculation
   - MACD calculation
   - Bollinger Bands

2. **Create Technical Indicators Server**
   - Separate MCP server for indicators
   - Add RSI, MACD, Bollinger tools
   - Technical agent uses this server

3. **Add Database Storage**
   - Store signals in PostgreSQL
   - Store candlesticks for offline analysis

4. **Create More Agents**
   - Order Book Agent
   - Trend Strategy Agent

### Phase 2: Building More MCP Servers

Follow the pattern you've learned:

1. Create new server in `cmd/mcp-servers/`
2. Define input/output types with `jsonschema` tags
3. Implement handler functions
4. Use `mcp.AddTool()` to register
5. Run with `mcp.NewStdioTransport()`

### Phase 3: Building the Orchestrator

The orchestrator will:
1. Connect to multiple agents as MCP clients
2. Aggregate their signals
3. Apply voting logic
4. Make final decisions

---

## Troubleshooting

### "Cannot connect to MCP server"

Check that the server binary path is correct:
```go
transport := mcp.NewCommandTransport("./bin/market-data-server")
```

### "Binance API error"

Verify your testnet credentials:
```bash
echo $BINANCE_API_KEY
echo $BINANCE_SECRET_KEY
```

Test directly:
```bash
curl "https://testnet.binance.vision/api/v3/ticker/price?symbol=BTCUSDT"
```

### "Database connection failed"

Check Docker containers are running:
```bash
docker-compose ps
docker-compose logs postgres
```

Test connection:
```bash
psql -h localhost -U postgres -d cryptofunk
```

---

## Resources

- [MCP Go SDK Documentation](https://github.com/modelcontextprotocol/go-sdk)
- [Binance API Documentation](https://binance-docs.github.io/apidocs/testnet/en/)
- [Go by Example](https://gobyexample.com/)
- [Zerolog Documentation](https://github.com/rs/zerolog)

---

## What You've Built

Congratulations! You've built:

✅ Project structure with proper organization
✅ Configuration management with environment variables
✅ Database schema with TimescaleDB
✅ Your first MCP server (Market Data)
✅ Your first MCP agent (Technical Analysis)
✅ Communication between agent and server via MCP
✅ Basic technical analysis with SMA

You're now ready to expand this foundation into a full trading system!

---

**Next**: Read [ARCHITECTURE.md](../ARCHITECTURE.md) for the complete system design, then proceed with implementing more MCP servers and agents according to the implementation phases.
