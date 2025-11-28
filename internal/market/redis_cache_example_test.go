package market_test

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ajitpratap0/cryptofunk/internal/market"
)

// ExampleRedisPriceCache demonstrates basic usage of RedisPriceCache
func ExampleRedisPriceCache() {
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Create price cache with 60 second TTL
	cache := market.NewRedisPriceCache(redisClient, 60*time.Second)

	ctx := context.Background()

	// Set a price (fire-and-forget, no error returned)
	cache.Set(ctx, "bitcoin", "usd", 50000.0)

	// Get the cached price
	price, found := cache.Get(ctx, "bitcoin", "usd")
	if found {
		fmt.Printf("Bitcoin price: $%.2f\n", price)
	} else {
		fmt.Println("Price not found in cache")
	}

	// Check cache health
	if err := cache.Health(ctx); err != nil {
		fmt.Printf("Cache unhealthy: %v\n", err)
	}
}

// ExampleCoinGeckoClient_withRedisCache demonstrates CoinGecko client with Redis caching
func ExampleCoinGeckoClient_withRedisCache() {
	ctx := context.Background()

	// Create Redis client (optional)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Create Redis cache
	cache := market.NewRedisPriceCache(redisClient, 60*time.Second)

	// Create CoinGecko client with Redis cache
	client, err := market.NewCoinGeckoClientWithOptions(market.CoinGeckoClientOptions{
		MCPURL:             "https://mcp.api.coingecko.com/mcp",
		APIKey:             "", // Optional
		Timeout:            30 * time.Second,
		RateLimit:          50,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		EnableRateLimiting: true,
		Cache:              cache, // Enable Redis caching
	})
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	// First call hits the API (cache miss)
	result, err := client.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		fmt.Printf("Failed to get price: %v\n", err)
		return
	}
	fmt.Printf("Bitcoin price (from API): $%.2f\n", result.Price)

	// Second call within TTL hits the cache (no API call)
	result, err = client.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		fmt.Printf("Failed to get price: %v\n", err)
		return
	}
	fmt.Printf("Bitcoin price (from cache): $%.2f\n", result.Price)
}

// ExampleCoinGeckoClient_withoutRedisCache demonstrates CoinGecko client without Redis
func ExampleCoinGeckoClient_withoutRedisCache() {
	ctx := context.Background()

	// Create CoinGecko client WITHOUT Redis cache
	// System falls back to in-memory singleflight deduplication
	client, err := market.NewCoinGeckoClient("")
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	// Fetch price (no caching, but singleflight prevents concurrent duplicate requests)
	result, err := client.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		fmt.Printf("Failed to get price: %v\n", err)
		return
	}
	fmt.Printf("Bitcoin price: $%.2f\n", result.Price)
}

// ExampleCoinGeckoClient_SetCache demonstrates adding cache to existing client
func ExampleCoinGeckoClient_SetCache() {
	ctx := context.Background()

	// Create client without cache initially
	client, err := market.NewCoinGeckoClient("")
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	// Fetch price without cache
	result, err := client.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		fmt.Printf("Failed to get price: %v\n", err)
		return
	}
	fmt.Printf("Bitcoin price (no cache): $%.2f\n", result.Price)

	// Later, add Redis cache to the same client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	cache := market.NewRedisPriceCache(redisClient, 60*time.Second)
	client.SetCache(cache)

	// Now subsequent calls use cache
	result, err = client.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		fmt.Printf("Failed to get price: %v\n", err)
		return
	}
	fmt.Printf("Bitcoin price (with cache): $%.2f\n", result.Price)
}

// ExampleRedisPriceCache_customTTL demonstrates using custom TTL per price
func ExampleRedisPriceCache_customTTL() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create cache with default 60s TTL
	cache := market.NewRedisPriceCache(redisClient, 60*time.Second)

	ctx := context.Background()

	// Cache stable coins with longer TTL (5 minutes) - fire-and-forget
	cache.SetWithTTL(ctx, "usdt", "usd", 1.0, 5*time.Minute)

	// Cache volatile coins with shorter TTL (30 seconds) - fire-and-forget
	cache.SetWithTTL(ctx, "bitcoin", "usd", 50000.0, 30*time.Second)

	fmt.Println("Prices cached with custom TTLs")
}

// ExampleRedisPriceCache_cacheManagement demonstrates cache management operations
func ExampleRedisPriceCache_cacheManagement() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	cache := market.NewRedisPriceCache(redisClient, 60*time.Second)
	ctx := context.Background()

	// Cache multiple prices
	cache.Set(ctx, "bitcoin", "usd", 50000.0)
	cache.Set(ctx, "ethereum", "usd", 3000.0)

	// Delete specific price
	if err := cache.Delete(ctx, "bitcoin", "usd"); err != nil {
		fmt.Printf("Failed to delete: %v\n", err)
	}

	// Clear all cached prices
	if err := cache.Clear(ctx); err != nil {
		fmt.Printf("Failed to clear: %v\n", err)
	}

	fmt.Println("Cache management completed")
}
