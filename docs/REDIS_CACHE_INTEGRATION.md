# Redis Cache Integration for CoinGecko Price Data

## Overview

The CoinGecko client now supports optional Redis-based caching for cryptocurrency price data. This feature reduces API calls, improves response times, and helps stay within rate limits.

## Key Features

- **Optional**: System works with or without Redis - graceful degradation
- **Configurable TTL**: Default 60 seconds, customizable per operation
- **Thread-safe**: Uses mutex protection for concurrent access
- **Fault-tolerant**: Cache failures don't break the application
- **Singleflight protection**: Prevents cache stampede even without Redis
- **Structured data**: JSON serialization with timestamps

## Architecture

```
┌─────────────────────────────────────────┐
│     CoinGecko Client Request            │
└────────────────┬────────────────────────┘
                 │
                 ▼
         ┌───────────────┐
         │ Redis Cache?  │
         └───────┬───────┘
                 │
        ┌────────┴────────┐
        │                 │
    Cache Hit         Cache Miss
        │                 │
        ▼                 ▼
    Return Price   ┌──────────────┐
                   │ Singleflight │
                   └──────┬───────┘
                          │
                          ▼
                  ┌───────────────┐
                  │  MCP API Call │
                  └───────┬───────┘
                          │
                   ┌──────┴──────┐
                   │             │
                   ▼             ▼
           Cache Result    Return Price
```

## Usage Examples

### Basic Usage with Redis Cache

```go
import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/ajitpratap0/cryptofunk/internal/market"
)

// Create Redis client
redisClient := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

// Create price cache with 60 second TTL
cache := market.NewRedisPriceCache(redisClient, 60*time.Second)

// Create CoinGecko client with cache
client, err := market.NewCoinGeckoClientWithOptions(market.CoinGeckoClientOptions{
    MCPURL:             "https://mcp.api.coingecko.com/mcp",
    Timeout:            30 * time.Second,
    RateLimit:          50,
    MaxRetries:         3,
    EnableRateLimiting: true,
    Cache:              cache, // Enable Redis caching
})

// Use client normally
ctx := context.Background()
result, err := client.GetPrice(ctx, "bitcoin", "usd")
// First call: API hit + cache store
// Second call within 60s: Cache hit (no API call)
```

### Without Redis Cache (Default Behavior)

```go
// Create client without cache - still gets singleflight protection
client, err := market.NewCoinGeckoClient("")

// Works exactly as before, just without persistent caching
result, err := client.GetPrice(ctx, "bitcoin", "usd")
```

### Adding Cache to Existing Client

```go
// Start without cache
client, err := market.NewCoinGeckoClient("")

// Later, add Redis cache
redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
cache := market.NewRedisPriceCache(redisClient, 60*time.Second)
client.SetCache(cache)

// Now uses cache
result, err := client.GetPrice(ctx, "bitcoin", "usd")
```

### Custom TTL per Price

```go
cache := market.NewRedisPriceCache(redisClient, 60*time.Second)

// Cache stable coins longer (5 minutes)
cache.SetWithTTL(ctx, "usdt", "usd", 1.0, 5*time.Minute)

// Cache volatile coins shorter (30 seconds)
cache.SetWithTTL(ctx, "bitcoin", "usd", 50000.0, 30*time.Second)
```

### Cache Management

```go
// Delete specific price
cache.Delete(ctx, "bitcoin", "usd")

// Clear all cached prices
cache.Clear(ctx)

// Check cache health
if err := cache.Health(ctx); err != nil {
    log.Printf("Cache unhealthy: %v", err)
}
```

## Configuration

### Redis Connection

Use the existing Redis configuration in `configs/config.yaml`:

```yaml
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
```

### Cache TTL

The default TTL is 60 seconds, configurable at cache creation:

```go
// 60 second TTL
cache := market.NewRedisPriceCache(redisClient, 60*time.Second)

// 5 minute TTL
cache := market.NewRedisPriceCache(redisClient, 5*time.Minute)

// Use default (60s) by passing 0
cache := market.NewRedisPriceCache(redisClient, 0)
```

## Redis Key Format

Cached prices use the key format: `cryptofunk:price:{symbol}:{currency}`

Examples:
- `cryptofunk:price:bitcoin:usd`
- `cryptofunk:price:ethereum:usd`
- `cryptofunk:price:cardano:eur`

## Error Handling

The cache is designed to fail gracefully:

1. **Redis Unavailable**: Falls back to API calls (logged as warnings)
2. **Cache Miss**: Normal flow, fetches from API
3. **Serialization Error**: Logs warning, continues without caching
4. **Timeout**: 500ms timeout on cache operations to prevent blocking

## Performance Characteristics

### With Redis Cache
- **Cache Hit**: ~1-2ms (Redis GET)
- **Cache Miss**: ~100-500ms (API call + cache store)
- **Concurrent Requests**: Deduplicated via singleflight

### Without Redis Cache
- **Every Request**: ~100-500ms (API call)
- **Concurrent Requests**: Still deduplicated via singleflight

## Testing

All cache functionality is fully tested:

```bash
# Run cache tests
go test -v -run TestRedisPriceCache ./internal/market/

# Run all market tests
go test -v ./internal/market/ -short
```

## Implementation Files

- `/internal/market/redis_cache.go` - RedisPriceCache implementation
- `/internal/market/redis_cache_test.go` - Comprehensive tests
- `/internal/market/redis_cache_example_test.go` - Usage examples
- `/internal/market/coingecko.go` - Updated with cache integration

## Monitoring

Cache operations are logged:

```json
{"level":"debug","symbol":"bitcoin","currency":"usd","price":50000,"time":"...","message":"Cached price"}
{"level":"debug","symbol":"bitcoin","currency":"usd","price":50000,"cached_at":"...","message":"Cache hit for price"}
{"level":"warn","error":"...","key":"...","message":"Failed to cache price in Redis - continuing anyway"}
```

## Migration Guide

Existing code continues to work without changes. To enable Redis caching:

1. Ensure Redis is running
2. Update client creation to include cache:
   ```go
   cache := market.NewRedisPriceCache(redisClient, 60*time.Second)
   client, _ := market.NewCoinGeckoClientWithOptions(opts)
   client.SetCache(cache)
   ```
3. Done! No other changes needed.

## Benefits

1. **Reduced API Calls**: 60s cache reduces calls by ~95% for frequently accessed prices
2. **Rate Limit Protection**: Stay well under CoinGecko's rate limits
3. **Improved Latency**: Cache hits are 50-100x faster than API calls
4. **Cost Savings**: Fewer API calls = lower costs for paid tiers
5. **Resilience**: Singleflight prevents stampedes even without Redis
6. **Zero Breaking Changes**: Completely optional and backward compatible
