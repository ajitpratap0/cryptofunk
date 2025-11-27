package market

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RedisPriceCache provides Redis-based caching for cryptocurrency price data
type RedisPriceCache struct {
	client *redis.Client
	ttl    time.Duration
}

// PriceCacheEntry represents a cached price with metadata
type PriceCacheEntry struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
	Timestamp time.Time `json:"timestamp"`
}

// NewRedisPriceCache creates a new Redis-based price cache
// If client is nil, returns nil (optional Redis support)
func NewRedisPriceCache(client *redis.Client, ttl time.Duration) *RedisPriceCache {
	if client == nil {
		return nil
	}

	if ttl == 0 {
		ttl = 60 * time.Second // Default TTL: 60 seconds
	}

	return &RedisPriceCache{
		client: client,
		ttl:    ttl,
	}
}

// Get retrieves a price from cache
// Returns the price and true if found, or 0 and false if not found or on error
func (c *RedisPriceCache) Get(ctx context.Context, symbol string, currency string) (float64, bool) {
	if c == nil || c.client == nil {
		return 0, false
	}

	key := c.buildKey(symbol, currency)

	// Use a short timeout for cache operations to prevent blocking
	cacheCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	cached, err := c.client.Get(cacheCtx, key).Result()
	if err != nil {
		if err != redis.Nil {
			// Log error but don't fail - cache miss is acceptable
			log.Debug().
				Err(err).
				Str("key", key).
				Msg("Redis get error - treating as cache miss")
		}
		return 0, false
	}

	var entry PriceCacheEntry
	if err := json.Unmarshal([]byte(cached), &entry); err != nil {
		log.Warn().
			Err(err).
			Str("key", key).
			Msg("Failed to unmarshal cached price")
		return 0, false
	}

	log.Debug().
		Str("symbol", symbol).
		Str("currency", currency).
		Float64("price", entry.Price).
		Time("cached_at", entry.Timestamp).
		Msg("Cache hit for price")

	return entry.Price, true
}

// Set stores a price in cache with the configured TTL
func (c *RedisPriceCache) Set(ctx context.Context, symbol string, currency string, price float64) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("cache not initialized")
	}

	key := c.buildKey(symbol, currency)

	entry := PriceCacheEntry{
		Symbol:    symbol,
		Price:     price,
		Currency:  currency,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal price entry: %w", err)
	}

	// Use a short timeout for cache operations
	cacheCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := c.client.Set(cacheCtx, key, data, c.ttl).Err(); err != nil {
		// Log but don't fail the operation - cache failure should be graceful
		log.Warn().
			Err(err).
			Str("key", key).
			Msg("Failed to cache price")
		return err
	}

	log.Debug().
		Str("symbol", symbol).
		Str("currency", currency).
		Float64("price", price).
		Dur("ttl", c.ttl).
		Msg("Cached price")

	return nil
}

// SetWithTTL stores a price in cache with a custom TTL
func (c *RedisPriceCache) SetWithTTL(ctx context.Context, symbol string, currency string, price float64, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("cache not initialized")
	}

	key := c.buildKey(symbol, currency)

	entry := PriceCacheEntry{
		Symbol:    symbol,
		Price:     price,
		Currency:  currency,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal price entry: %w", err)
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := c.client.Set(cacheCtx, key, data, ttl).Err(); err != nil {
		log.Warn().
			Err(err).
			Str("key", key).
			Msg("Failed to cache price with custom TTL")
		return err
	}

	log.Debug().
		Str("symbol", symbol).
		Str("currency", currency).
		Float64("price", price).
		Dur("ttl", ttl).
		Msg("Cached price with custom TTL")

	return nil
}

// Delete removes a price from cache
func (c *RedisPriceCache) Delete(ctx context.Context, symbol string, currency string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("cache not initialized")
	}

	key := c.buildKey(symbol, currency)

	cacheCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := c.client.Del(cacheCtx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete cache key: %w", err)
	}

	log.Debug().
		Str("symbol", symbol).
		Str("currency", currency).
		Msg("Deleted price from cache")

	return nil
}

// Clear removes all price cache entries
func (c *RedisPriceCache) Clear(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("cache not initialized")
	}

	pattern := c.buildKeyPattern()

	cacheCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	iter := c.client.Scan(cacheCtx, 0, pattern, 0).Iterator()
	count := 0

	for iter.Next(cacheCtx) {
		if err := c.client.Del(cacheCtx, iter.Val()).Err(); err != nil {
			log.Warn().
				Err(err).
				Str("key", iter.Val()).
				Msg("Failed to delete cache key")
		} else {
			count++
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache scan error: %w", err)
	}

	log.Info().
		Int("keys_deleted", count).
		Msg("Cleared price cache")

	return nil
}

// Health checks if the Redis connection is healthy
func (c *RedisPriceCache) Health(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("cache not initialized")
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := c.client.Ping(cacheCtx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// buildKey creates a Redis key for a symbol/currency pair
func (c *RedisPriceCache) buildKey(symbol string, currency string) string {
	return fmt.Sprintf("cryptofunk:price:%s:%s", symbol, currency)
}

// buildKeyPattern creates a Redis key pattern for scanning
func (c *RedisPriceCache) buildKeyPattern() string {
	return "cryptofunk:price:*"
}
