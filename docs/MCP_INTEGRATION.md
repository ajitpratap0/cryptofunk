# CryptoFunk MCP Integration Guide

**Version**: 1.0
**Last Updated**: 2025-10-27
**Status**: Production Ready

---

## Table of Contents

1. [Overview](#overview)
2. [Why Hybrid MCP Architecture?](#why-hybrid-mcp-architecture)
3. [CoinGecko MCP Server](#coingecko-mcp-server)
4. [Custom MCP Servers](#custom-mcp-servers)
5. [Configuration](#configuration)
6. [Integration Steps](#integration-steps)
7. [Usage Examples](#usage-examples)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)

---

## Overview

CryptoFunk uses a **hybrid MCP architecture** that combines:

1. **External MCP Servers** - Third-party servers (like CoinGecko) for market data
2. **Custom MCP Servers** - Internal servers for execution, risk, and indicators

This approach provides the best of both worlds:

- **Rapid development** - Leverage existing MCP servers
- **Full control** - Custom logic where needed
- **Reduced maintenance** - External servers maintained by providers

---

## Why Hybrid MCP Architecture?

### Time Savings

| Task | Build from Scratch | Use CoinGecko MCP | Savings |
|------|-------------------|-------------------|---------|
| Market data API integration | 12 hours | 1 hour | 11 hours |
| Multi-exchange aggregation | 8 hours | 0 hours | 8 hours |
| Historical data fetching | 6 hours | 0 hours | 6 hours |
| WebSocket streaming | 4 hours | 0 hours | 4 hours |
| Error handling & retries | 3 hours | 0 hours | 3 hours |
| **Total** | **33 hours** | **1 hour** | **32 hours** |

### Feature Comparison

| Feature | CoinGecko MCP | Custom Build |
|---------|---------------|--------------|
| **Market Data Tools** | 76+ pre-built | Need to build all |
| **Multi-Exchange** | Built-in | Manual integration |
| **Historical Data** | Instant access | Need to collect |
| **Maintenance** | CoinGecko handles | Our responsibility |
| **Cost** | Free tier + Pro | Infrastructure only |
| **Reliability** | CoinGecko SLA | Our ops |
| **Setup Time** | <1 hour | Days/weeks |

---

## CoinGecko MCP Server

### What is CoinGecko MCP?

CoinGecko MCP is an external MCP server provided by CoinGecko that gives access to comprehensive cryptocurrency market data through 76+ tools.

**Server URL**: `https://mcp.api.coingecko.com/mcp`
**Transport**: HTTP Streaming
**Authentication**: None required for free tier
**Rate Limits**: 100 requests/minute (free tier), higher with Pro

### Available Tools (76+)

#### Price Data

- `get_price` - Current cryptocurrency prices
- `get_price_by_id` - Price for specific coin
- `get_simple_price` - Simple price query with multiple vs currencies
- `get_token_price` - Token price by contract address

#### Market Data

- `get_market_chart` - Historical market data (price, volume, market cap)
- `get_market_chart_range` - Market data for specific date range
- `get_ohlc` - OHLCV candlestick data
- `get_coin_market_data` - Comprehensive market statistics

#### Coin Information

- `get_coin_info` - Detailed coin information
- `get_coin_by_id` - Coin data by ID
- `get_coin_tickers` - Trading pair tickers
- `get_coin_history` - Historical snapshots

#### Market Trends

- `get_trending` - Trending coins
- `get_top_gainers` - Top gaining coins
- `get_top_losers` - Top losing coins
- `get_recently_added` - Recently listed coins

#### Categories & Search

- `search_coins` - Search for cryptocurrencies
- `list_coins` - List all supported coins
- `get_categories` - Market categories
- `get_category_coins` - Coins in specific category

#### NFT Data

- `get_nft_collection` - NFT collection data
- `get_nft_market` - NFT marketplace statistics
- `list_nft_collections` - All NFT collections

#### DeFi Data

- `get_defi_pools` - DeFi liquidity pools
- `get_exchanges` - Exchange information
- `get_exchange_tickers` - Exchange trading pairs

**Full list**: See CoinGecko MCP documentation at <https://mcp.api.coingecko.com>

### Benefits

✅ **76+ pre-built tools** - No need to implement
✅ **Multi-exchange aggregation** - Prices from multiple sources
✅ **Historical data** - Years of historical OHLCV data
✅ **No API key required** - Free tier available
✅ **Maintained by CoinGecko** - Always up-to-date
✅ **Reliable** - Enterprise-grade SLA
✅ **Global CDN** - Low latency worldwide

### Limitations

⚠️ **Rate limits** - Free tier: 100 req/min (Pro tier available)
⚠️ **External dependency** - Requires internet connection
⚠️ **Limited to provided tools** - Can't customize logic
⚠️ **Latency** - External API call overhead

**Solution**: Use Redis caching layer to mitigate rate limits and latency.

---

## Custom MCP Servers

These are internal servers we build for functionality not available externally.

### 1. Order Executor Server

**Purpose**: Execute trades on exchanges
**Why Custom**: Needs direct exchange API for actual trading
**Binary**: `./bin/order-executor-server`
**Transport**: stdio

**Tools**:

- `place_market_order(symbol, side, quantity)` → order_id
- `place_limit_order(symbol, side, quantity, price)` → order_id
- `cancel_order(order_id)` → status
- `get_order_status(order_id)` → order
- `get_positions()` → positions[]

**Technology**: CCXT for unified exchange API

### 2. Risk Analyzer Server

**Purpose**: Portfolio risk management
**Why Custom**: Custom risk rules and calculations
**Binary**: `./bin/risk-analyzer-server`
**Transport**: stdio

**Tools**:

- `calculate_position_size(win_rate, capital, kelly_fraction)` → size
- `calculate_var(returns[], confidence)` → var_value
- `check_portfolio_limits(positions, new_trade, limits)` → approved
- `calculate_sharpe(returns[], risk_free_rate)` → sharpe
- `calculate_drawdown(equity_curve[])` → drawdown

**Technology**: Custom Go implementation

### 3. Technical Indicators Server

**Purpose**: Technical analysis calculations
**Why Custom**: Specialized indicator calculations
**Binary**: `./bin/technical-indicators-server`
**Transport**: stdio

**Tools**:

- `calculate_rsi(prices[], period)` → rsi_value
- `calculate_macd(prices[], fast, slow, signal)` → macd_result
- `calculate_bollinger_bands(prices[], period, std_devs)` → bands
- `calculate_ema(prices[], period)` → ema_value
- `calculate_adx(high[], low[], close[], period)` → adx_value
- `detect_patterns(candlesticks[])` → patterns[]

**Technology**: cinar/indicator library (60+ indicators)

### 4. Market Data Server (Optional)

**Purpose**: Binance-specific features
**Why Optional**: CoinGecko MCP covers 90% of use cases
**Binary**: `./bin/market-data-server`
**Transport**: stdio

**Use Case**: Only enable if you need:

- Exchange-specific order book depth
- Real-time WebSocket from specific exchange
- Binance-specific endpoints

**Default**: Disabled (CoinGecko MCP is primary data source)

---

## Configuration

### MCP Configuration Structure

**File**: `configs/config.yaml`

```yaml
mcp:
  # External MCP Servers
  external:
    coingecko:
      enabled: true
      name: "CoinGecko MCP"
      url: "https://mcp.api.coingecko.com/mcp"
      transport: "http_streaming"
      description: "76+ market data tools"
      cache_ttl: 60  # seconds
      rate_limit:
        enabled: true
        requests_per_minute: 100

  # Internal (Custom) MCP Servers
  internal:
    order_executor:
      enabled: true
      name: "Order Executor"
      command: "./bin/order-executor-server"
      transport: "stdio"
      description: "Execute orders on exchanges"
      env:
        EXCHANGE: "binance"
        MODE: "paper"  # paper or live

    risk_analyzer:
      enabled: true
      name: "Risk Analyzer"
      command: "./bin/risk-analyzer-server"
      transport: "stdio"
      description: "Portfolio risk management"

    technical_indicators:
      enabled: true
      name: "Technical Indicators"
      command: "./bin/technical-indicators-server"
      transport: "stdio"
      description: "Technical analysis indicators"

    market_data:
      enabled: false  # Disabled by default
      name: "Market Data (Binance)"
      command: "./bin/market-data-server"
      transport: "stdio"
      note: "CoinGecko MCP is primary data source"
```

### Go Configuration Structs

**File**: `internal/config/config.go`

```go
type MCPConfig struct {
    External MCPExternalServers
    Internal MCPInternalServers
}

type MCPExternalServerConfig struct {
    Enabled     bool
    Name        string
    URL         string
    Transport   string  // "http_streaming"
    CacheTTL    int     // seconds
    RateLimit   MCPRateLimitConfig
    Tools       []string
}

type MCPInternalServerConfig struct {
    Enabled     bool
    Name        string
    Command     string  // path to binary
    Transport   string  // "stdio"
    Args        []string
    Env         map[string]string
    Tools       []string
}
```

---

## Integration Steps

### Step 1: Configure CoinGecko MCP

**Edit** `configs/config.yaml`:

```yaml
mcp:
  external:
    coingecko:
      enabled: true
      url: "https://mcp.api.coingecko.com/mcp"
      cache_ttl: 60
      rate_limit:
        enabled: true
        requests_per_minute: 100  # Adjust for your tier
```

### Step 2: Create MCP Client Connection

```go
// internal/market/coingecko.go
package market

import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CoinGeckoClient struct {
    client *mcp.Client
}

func NewCoinGeckoClient(url string) (*CoinGeckoClient, error) {
    client, err := mcp.NewClient(mcp.ClientConfig{
        ServerURL: url,
        Transport: mcp.TransportHTTPStreaming,
    })
    if err != nil {
        return nil, err
    }

    return &CoinGeckoClient{client: client}, nil
}

func (c *CoinGeckoClient) GetPrice(symbol string, vsCurrency string) (float64, error) {
    result, err := c.client.CallTool("get_price", map[string]any{
        "ids": symbol,
        "vs_currencies": vsCurrency,
    })
    if err != nil {
        return 0, err
    }

    // Parse result
    price := result.(map[string]any)[symbol].(map[string]any)[vsCurrency].(float64)
    return price, nil
}

func (c *CoinGeckoClient) GetMarketChart(symbol string, days int) (*MarketChart, error) {
    result, err := c.client.CallTool("get_market_chart", map[string]any{
        "id": symbol,
        "vs_currency": "usd",
        "days": days,
    })
    if err != nil {
        return nil, err
    }

    // Parse and return
    return parseMarketChart(result), nil
}
```

### Step 3: Add Caching Layer

```go
// internal/market/cache.go
package market

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type CachedCoinGeckoClient struct {
    client    *CoinGeckoClient
    redis     *redis.Client
    cacheTTL  time.Duration
}

func (c *CachedCoinGeckoClient) GetPrice(ctx context.Context, symbol string) (float64, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("price:%s", symbol)
    cached, err := c.redis.Get(ctx, cacheKey).Result()
    if err == nil {
        var price float64
        json.Unmarshal([]byte(cached), &price)
        return price, nil
    }

    // Cache miss - fetch from CoinGecko MCP
    price, err := c.client.GetPrice(symbol, "usd")
    if err != nil {
        return 0, err
    }

    // Store in cache
    data, _ := json.Marshal(price)
    c.redis.Set(ctx, cacheKey, data, c.cacheTTL)

    return price, nil
}
```

### Step 4: Integrate with Analysis Agents

```go
// cmd/agents/technical-agent/main.go
package main

import (
    "github.com/ajitpratap0/cryptofunk/internal/market"
)

func main() {
    // Connect to CoinGecko MCP
    coinGecko, err := market.NewCoinGeckoClient(
        "https://mcp.api.coingecko.com/mcp",
    )
    if err != nil {
        log.Fatal(err)
    }

    // Connect to Technical Indicators MCP Server
    indicators, err := mcp.NewClient(mcp.ClientConfig{
        Command: "./bin/technical-indicators-server",
        Transport: mcp.TransportStdio,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Use both servers
    prices, _ := coinGecko.GetMarketChart("bitcoin", 30)
    rsi, _ := indicators.CallTool("calculate_rsi", map[string]any{
        "prices": prices.ClosePrices,
        "period": 14,
    })

    // Generate signal based on analysis...
}
```

---

## Usage Examples

### Example 1: Get Current Bitcoin Price

```go
// Using CoinGecko MCP
price, err := coinGecko.GetPrice("bitcoin", "usd")
fmt.Printf("BTC Price: $%.2f\n", price)
```

### Example 2: Fetch Historical OHLCV Data

```go
// Get 30 days of historical data
chart, err := coinGecko.GetMarketChart("ethereum", 30)

// Extract OHLCV
for _, candle := range chart.Prices {
    fmt.Printf("Time: %v, Price: %.2f\n", candle.Time, candle.Price)
}

// Store in TimescaleDB for backtesting
db.StoreCandlesticks("ETHUSD", chart.ToCandlesticks())
```

### Example 3: Multi-Exchange Price Comparison

```go
// CoinGecko aggregates prices from multiple exchanges
tickers, err := coinGecko.CallTool("get_coin_tickers", map[string]any{
    "id": "bitcoin",
})

for _, ticker := range tickers {
    fmt.Printf("Exchange: %s, Price: $%.2f, Volume: $%.2f\n",
        ticker.Exchange, ticker.Last, ticker.Volume)
}
```

### Example 4: Technical Analysis Pipeline

```go
// 1. Get market data from CoinGecko MCP
prices, err := coinGecko.GetMarketChart("bitcoin", 100)

// 2. Calculate indicators using Technical Indicators MCP Server
rsi, _ := indicators.CallTool("calculate_rsi", map[string]any{
    "prices": prices.ClosePrices,
    "period": 14,
})

macd, _ := indicators.CallTool("calculate_macd", map[string]any{
    "prices": prices.ClosePrices,
    "fast": 12,
    "slow": 26,
    "signal": 9,
})

// 3. Use LLM for analysis (via Bifrost)
decision := llm.Analyze(prices, rsi, macd)

// 4. Execute if approved by Risk Agent
if riskAgent.Approve(decision) {
    orderExecutor.CallTool("place_market_order", decision)
}
```

---

## Best Practices

### 1. Caching Strategy

**Always cache CoinGecko responses:**

```go
// Cache with appropriate TTL based on data type
cacheTTL := map[string]time.Duration{
    "price":        60 * time.Second,   // Prices change frequently
    "market_chart": 5 * time.Minute,    // Historical data less frequent
    "coin_info":    30 * time.Minute,   // Metadata rarely changes
}
```

**Benefits**:

- Reduce API calls (stay within rate limits)
- Lower latency (Redis is faster than external API)
- Cost savings (if using Pro tier)

### 2. Rate Limit Management

```go
type RateLimiter struct {
    limiter *rate.Limiter
}

func (r *RateLimiter) WaitIfNeeded(ctx context.Context) error {
    return r.limiter.Wait(ctx)
}

// Before each CoinGecko MCP call
rateLimiter.WaitIfNeeded(ctx)
price, err := coinGecko.GetPrice("bitcoin", "usd")
```

### 3. Error Handling

```go
func (c *CoinGeckoClient) GetPriceWithRetry(symbol string, maxRetries int) (float64, error) {
    var price float64
    var err error

    for i := 0; i < maxRetries; i++ {
        price, err = c.GetPrice(symbol, "usd")
        if err == nil {
            return price, nil
        }

        // Exponential backoff
        time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second)
    }

    return 0, fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}
```

### 4. Fallback Strategy

```go
// Try CoinGecko first, fallback to custom market data server if needed
price, err := coinGecko.GetPrice("bitcoin", "usd")
if err != nil {
    // Fallback to Binance-specific server
    price, err = marketDataServer.CallTool("get_current_price", map[string]any{
        "symbol": "BTCUSDT",
    })
}
```

### 5. Data Persistence

```go
// Sync historical data to TimescaleDB periodically
func syncHistoricalData(coinGecko *CoinGeckoClient, db *Database) {
    for _, symbol := range []string{"bitcoin", "ethereum", "binancecoin"} {
        // Get 365 days of data
        chart, err := coinGecko.GetMarketChart(symbol, 365)
        if err != nil {
            log.Error("Failed to fetch data", "symbol", symbol, "error", err)
            continue
        }

        // Store in TimescaleDB
        err = db.StoreCandlesticks(symbol, chart.ToCandlesticks())
        if err != nil {
            log.Error("Failed to store data", "symbol", symbol, "error", err)
        }
    }
}

// Run daily
ticker := time.NewTicker(24 * time.Hour)
go func() {
    for range ticker.C {
        syncHistoricalData(coinGecko, db)
    }
}()
```

---

## Troubleshooting

### Issue 1: Connection Failed to CoinGecko MCP

**Error**: `failed to connect to https://mcp.api.coingecko.com/mcp`

**Solutions**:

```bash
# 1. Check internet connectivity
ping mcp.api.coingecko.com

# 2. Test endpoint directly
curl https://mcp.api.coingecko.com/mcp

# 3. Check firewall rules
# 4. Verify proxy settings if behind corporate firewall
```

### Issue 2: Rate Limit Exceeded

**Error**: `429 Too Many Requests`

**Solutions**:

```yaml
# 1. Enable caching in config.yaml
mcp:
  external:
    coingecko:
      cache_ttl: 120  # Increase to 2 minutes

# 2. Reduce request frequency
rate_limit:
  requests_per_minute: 50  # Lower than limit

# 3. Upgrade to CoinGecko Pro (if budget allows)

# 4. Batch requests where possible
```

### Issue 3: Slow Response Times

**Problem**: CoinGecko MCP calls taking >2 seconds

**Solutions**:

```go
// 1. Add timeout to client
client := mcp.NewClient(mcp.ClientConfig{
    ServerURL: coinGeckoURL,
    Timeout:   5 * time.Second,  // Set reasonable timeout
})

// 2. Use aggressive caching
cacheTTL := 5 * time.Minute  // Cache longer

// 3. Parallel requests for multiple symbols
var wg sync.WaitGroup
prices := make(map[string]float64)
mu := sync.Mutex{}

for _, symbol := range symbols {
    wg.Add(1)
    go func(s string) {
        defer wg.Done()
        price, _ := coinGecko.GetPrice(s, "usd")
        mu.Lock()
        prices[s] = price
        mu.Unlock()
    }(symbol)
}
wg.Wait()
```

### Issue 4: Custom MCP Server Won't Start

**Error**: `failed to start order-executor-server`

**Solutions**:

```bash
# 1. Check binary exists and is executable
ls -la ./bin/order-executor-server
chmod +x ./bin/order-executor-server

# 2. Test binary directly
./bin/order-executor-server

# 3. Check logs
tail -f /var/log/cryptofunk/order-executor-server.log

# 4. Verify environment variables
echo $BINANCE_API_KEY
```

### Issue 5: Data Inconsistency

**Problem**: CoinGecko prices differ from exchange prices

**Explanation**: CoinGecko aggregates prices from multiple exchanges, so there may be slight differences.

**Solutions**:

```go
// 1. Use specific exchange ticker
tickers, _ := coinGecko.CallTool("get_coin_tickers", map[string]any{
    "id": "bitcoin",
    "exchange_ids": "binance",  // Filter by exchange
})

// 2. For trading, use exchange-specific API
// CoinGecko for analysis, Binance API for execution
analysisPrice := coinGecko.GetPrice("bitcoin", "usd")
executionPrice := binanceAPI.GetPrice("BTCUSDT")
```

---

## Next Steps

1. **Review Architecture**: See [ARCHITECTURE.md](ARCHITECTURE.md) for hybrid MCP design
2. **Implementation**: Follow [TASKS.md](../TASKS.md) Phase 2.1 for integration steps
3. **Testing**: Test CoinGecko MCP connection:

   ```bash
   go run cmd/test-mcp-client/main.go
   ```

4. **Build Agents**: Integrate CoinGecko MCP into analysis agents (Phase 3)

---

## Resources

- **CoinGecko MCP Docs**: <https://mcp.api.coingecko.com>
- **Official MCP Go SDK**: <https://github.com/modelcontextprotocol/go-sdk>
- **CoinGecko API Docs**: <https://www.coingecko.com/en/api/documentation>
- **CryptoFunk Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **CryptoFunk Tasks**: [TASKS.md](../TASKS.md)

---

**Conclusion**: By using CoinGecko MCP, we save **32+ hours** of development time while gaining access to comprehensive market data from a reliable, maintained external service. This lets us focus on building the unique value of CryptoFunk: intelligent trading logic, risk management, and LLM-powered decision-making.
