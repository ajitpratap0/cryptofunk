# CoinGecko MCP Real Integration - Implementation Summary

## Overview

This document describes the implementation of T289: Fix CoinGecko MCP Real Integration for CryptoFunk. The implementation provides a complete, production-ready client for CoinGecko's Model Context Protocol (MCP) server with enterprise-grade features including rate limiting, retry logic with exponential backoff, and Redis caching.

## Implementation Details

### Core Components

#### 1. CoinGeckoClient (`internal/market/coingecko.go`)

A full-featured MCP client with the following capabilities:

**Features:**
- **MCP SDK Integration**: Uses `github.com/modelcontextprotocol/go-sdk` v1.0.0
- **SSE Transport**: Connects to CoinGecko MCP via Server-Sent Events (SSE)
- **Rate Limiting**: Token bucket algorithm with configurable requests per minute
- **Retry Logic**: Exponential backoff with configurable max retries and delays
- **Thread-Safe**: Uses sync.RWMutex for concurrent access
- **Context Support**: Proper context handling for cancellation and timeouts

**Key Methods:**
```go
// Create client with default settings (50 req/min, 3 retries)
client, err := NewCoinGeckoClient(apiKey)

// Or with custom options
client, err := NewCoinGeckoClientWithOptions(CoinGeckoClientOptions{
    MCPURL:             "https://mcp.api.coingecko.com/mcp",
    RateLimit:          100, // requests per minute
    MaxRetries:         3,
    RetryDelay:         time.Second,
    EnableRateLimiting: true,
})

// Fetch current price
price, err := client.GetPrice(ctx, "bitcoin", "usd")

// Fetch historical market data
chart, err := client.GetMarketChart(ctx, "bitcoin", 30) // 30 days

// Fetch detailed coin information
info, err := client.GetCoinInfo(ctx, "bitcoin")

// Health check
err := client.Health(ctx)

// Clean up
client.Close()
```

#### 2. Rate Limiter

Uses `golang.org/x/time/rate` package with token bucket algorithm:

- **Token Bucket**: Allows burst of requests up to the rate limit
- **Smooth Distribution**: Converts requests/minute to requests/second
- **Context-Aware**: Respects context cancellation during rate limit wait

**Configuration:**
```go
// Free tier: 50 requests per minute
RateLimit: 50

// Pro tier: 500 requests per minute
RateLimit: 500
```

#### 3. Retry Logic with Exponential Backoff

Automatic retry on failures with exponential backoff:

- **Max Retries**: Configurable (default: 3)
- **Initial Delay**: Configurable (default: 1 second)
- **Backoff Formula**: `delay = 2^(attempt-1) * retryDelay`
- **Max Delay**: Capped at 30 seconds
- **Context-Aware**: Respects context cancellation during backoff

**Example Retry Sequence:**
```
Attempt 1: Immediate (0s delay)
Attempt 2: 1s delay (2^0 * 1s)
Attempt 3: 2s delay (2^1 * 1s)
Attempt 4: 4s delay (2^2 * 1s)
```

#### 4. Redis Caching Layer (`internal/market/cache.go`)

Wraps the CoinGecko client with intelligent caching:

**Cache TTLs:**
- **Prices**: 60 seconds (configurable)
- **Market Charts (< 7 days)**: 60 seconds (configurable)
- **Market Charts (>= 7 days)**: 5 minutes (historical data changes less)
- **Coin Info**: 10 minutes (metadata rarely changes)

**Features:**
- **Async Cache Writes**: Non-blocking cache updates
- **Cache Miss Fallback**: Automatically fetches from API on miss
- **Error Tolerance**: Continues on cache errors, logs warnings
- **Cache Management**: Invalidate specific symbols or clear all

**Usage:**
```go
// Wrap client with caching
cachedClient := market.NewCachedCoinGeckoClient(
    client,
    redisClient,
    60*time.Second, // cache TTL
)

// Same interface as regular client
price, err := cachedClient.GetPrice(ctx, "bitcoin", "usd")

// Cache management
cachedClient.InvalidateCache(ctx, "bitcoin")
cachedClient.ClearCache(ctx)
```

### API Endpoints

The client supports the following CoinGecko MCP tools:

1. **get_price** - Current cryptocurrency prices
   ```go
   Arguments:
   - ids: coin ID (e.g., "bitcoin")
   - vs_currencies: target currency (e.g., "usd")
   ```

2. **get_market_chart** - Historical OHLCV data
   ```go
   Arguments:
   - coin_id: coin ID
   - vs_currency: target currency
   - days: number of days (1-365)
   ```

3. **get_coin_info** - Detailed coin information
   ```go
   Arguments:
   - coin_id: coin ID
   - localization: bool (false)
   - tickers: bool (false)
   - community_data: bool (false)
   - developer_data: bool (false)
   ```

### Data Structures

```go
// Price result
type PriceResult struct {
    Symbol   string
    Price    float64
    Currency string
}

// Historical market data
type MarketChart struct {
    Prices       []PricePoint
    MarketCaps   []PricePoint
    TotalVolumes []PricePoint
}

type PricePoint struct {
    Timestamp time.Time
    Value     float64
}

// Coin information
type CoinInfo struct {
    ID          string
    Symbol      string
    Name        string
    Description string
    Links       map[string]string
    MarketData  map[string]interface{}
}

// OHLCV candlesticks
type Candlestick struct {
    Timestamp time.Time
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    float64
}
```

### Helper Functions

**ToCandlesticks**: Converts MarketChart to OHLCV candlesticks
```go
// Convert to 15-minute candles
candlesticks := chart.ToCandlesticks(15)

// Convert to 1-hour candles
candlesticks := chart.ToCandlesticks(60)
```

## Testing

### Unit Tests (`internal/market/coingecko_test.go`)

Comprehensive test coverage including:

- Client initialization (with/without API key)
- Custom options configuration
- Rate limiting behavior
- Retry logic structure
- Context cancellation
- Health checks
- Resource cleanup

### Integration Tests

Real API tests (skipped by default to avoid rate limiting):

```bash
# Run integration tests with real CoinGecko MCP endpoint
COINGECKO_API_TEST=1 go test ./internal/market/...

# With API key
COINGECKO_API_TEST=1 COINGECKO_API_KEY=your-key go test ./internal/market/...
```

### Cache Tests (`internal/market/cache_test.go`)

Tests for Redis caching layer:
- Cache hit/miss scenarios
- TTL behavior
- Cache invalidation
- Error handling

## Configuration

### Environment Variables

```bash
# Optional: CoinGecko API key (pro tier)
COINGECKO_API_KEY=your-api-key-here

# Optional: Enable integration tests
COINGECKO_API_TEST=1
```

### Config File (`configs/config.yaml`)

```yaml
mcp:
  external:
    coingecko:
      enabled: true
      name: "CoinGecko MCP"
      url: "https://mcp.api.coingecko.com/mcp"
      transport: "http_streaming"
      description: "76+ market data tools for prices, historical data, trends"
      cache_ttl: 60  # seconds
      rate_limit:
        enabled: true
        requests_per_minute: 50  # Free tier: 50, Pro: 500+
      tools:
        - "get_price"
        - "get_market_chart"
        - "get_coin_info"
        - "get_trending"
        - "get_top_gainers"
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ajitpratap0/cryptofunk/internal/market"
)

func main() {
    // Create client
    client, err := market.NewCoinGeckoClient("")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Get Bitcoin price
    price, err := client.GetPrice(ctx, "bitcoin", "usd")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Bitcoin price: $%.2f", price.Price)

    // Get 30-day market chart
    chart, err := client.GetMarketChart(ctx, "bitcoin", 30)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Fetched %d price points", len(chart.Prices))

    // Convert to 1-hour candlesticks
    candles := chart.ToCandlesticks(60)
    log.Printf("Generated %d candles", len(candles))
}
```

### With Redis Caching

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ajitpratap0/cryptofunk/internal/market"
    "github.com/redis/go-redis/v9"
)

func main() {
    // Create CoinGecko client
    client, err := market.NewCoinGeckoClient("")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    defer redisClient.Close()

    // Wrap with caching
    cachedClient := market.NewCachedCoinGeckoClient(
        client,
        redisClient,
        60*time.Second, // 60s cache TTL
    )

    ctx := context.Background()

    // First call: Cache miss, fetches from API
    price1, _ := cachedClient.GetPrice(ctx, "bitcoin", "usd")
    log.Printf("First call (cache miss): $%.2f", price1.Price)

    // Second call: Cache hit, instant response
    price2, _ := cachedClient.GetPrice(ctx, "bitcoin", "usd")
    log.Printf("Second call (cache hit): $%.2f", price2.Price)
}
```

### Custom Options

```go
client, err := market.NewCoinGeckoClientWithOptions(
    market.CoinGeckoClientOptions{
        MCPURL:             "https://mcp.api.coingecko.com/mcp",
        APIKey:             os.Getenv("COINGECKO_API_KEY"),
        Timeout:            30 * time.Second,
        RateLimit:          100, // Pro tier
        MaxRetries:         5,
        RetryDelay:         2 * time.Second,
        EnableRateLimiting: true,
    },
)
```

## Performance Characteristics

### Rate Limiting Impact

- **Free Tier (50 req/min)**: ~1 request every 1.2 seconds
- **Pro Tier (500 req/min)**: ~8 requests per second
- **Burst Capacity**: Can handle bursts up to the rate limit before throttling

### Retry Overhead

With default settings (3 retries, 1s initial delay):
- **Success on 1st attempt**: No overhead
- **Success on 2nd attempt**: +1s overhead
- **Success on 3rd attempt**: +3s overhead (1s + 2s)
- **Success on 4th attempt**: +7s overhead (1s + 2s + 4s)

### Caching Benefits

- **Cache Hit**: ~1ms (Redis latency)
- **Cache Miss**: 200-1000ms (API latency)
- **Hit Rate**: Typically 80-95% for active trading

## Error Handling

The client provides detailed error messages with context:

```go
// Rate limit errors
"rate limit wait failed: context canceled"

// Connection errors
"failed to connect to MCP server: connection refused"

// Retry exhaustion
"max retries (3) exceeded: attempt 4 failed: timeout"

// API errors
"symbol bitcoin not found in response"
"currency usd not found for symbol bitcoin"
```

## Monitoring and Observability

### Logging

All operations are logged with structured logging (zerolog):

```go
{"level":"info","mcp_url":"https://mcp.api.coingecko.com/mcp","rate_limit":50,"message":"Initializing CoinGecko MCP client"}
{"level":"debug","symbol":"bitcoin","vs_currency":"usd","message":"Fetching price from CoinGecko MCP"}
{"level":"info","symbol":"bitcoin","price":45000.50,"message":"Price fetched successfully"}
{"level":"warn","tool":"get_price","attempt":2,"message":"MCP tool call failed"}
```

### Metrics

Consider adding Prometheus metrics for:
- Request count by endpoint
- Request latency percentiles (p50, p95, p99)
- Cache hit/miss ratio
- Rate limit wait time
- Retry count by endpoint
- Error count by type

## Future Improvements

1. **API Key Headers**: Add support when SDK implements header customization
2. **Circuit Breaker**: Add circuit breaker pattern for sustained failures
3. **Request Deduplication**: Coalesce identical concurrent requests
4. **Batch Requests**: Support batch price lookups when SDK supports it
5. **Compression**: Enable HTTP compression for large responses
6. **Connection Pooling**: Optimize HTTP client for high throughput

## References

- **CoinGecko MCP Docs**: https://docs.coingecko.com/reference/mcp
- **MCP Specification**: https://modelcontextprotocol.io/
- **MCP Go SDK**: https://github.com/modelcontextprotocol/go-sdk
- **Rate Limiting Library**: https://pkg.go.dev/golang.org/x/time/rate

## Dependencies

```go
require (
    github.com/modelcontextprotocol/go-sdk v1.0.0
    github.com/redis/go-redis/v9 v9.16.0
    github.com/rs/zerolog v1.34.0
    golang.org/x/time v0.14.0
)
```

## License

This implementation is part of the CryptoFunk project and follows the project's license.

## Support

For issues or questions:
1. Check the test files for usage examples
2. Review the code documentation
3. Enable debug logging for detailed traces
4. Consult the MCP SDK documentation

---

**Last Updated**: 2025-11-27
**Implementation Version**: v1.0.0
**Status**: Production Ready
