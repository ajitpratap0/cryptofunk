package market

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// CachedCoinGeckoClient wraps CoinGeckoClient with Redis caching
type CachedCoinGeckoClient struct {
	client   *CoinGeckoClient
	redis    *redis.Client
	cacheTTL time.Duration
}

// NewCachedCoinGeckoClient creates a new cached CoinGecko client
func NewCachedCoinGeckoClient(client *CoinGeckoClient, redisClient *redis.Client, cacheTTL time.Duration) *CachedCoinGeckoClient {
	return &CachedCoinGeckoClient{
		client:   client,
		redis:    redisClient,
		cacheTTL: cacheTTL,
	}
}

// GetPrice fetches price with caching
func (c *CachedCoinGeckoClient) GetPrice(ctx context.Context, symbol string, vsCurrency string) (*PriceResult, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("coingecko:price:%s:%s", symbol, vsCurrency)

	// Check cache first
	cached, err := c.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit
		log.Debug().
			Str("symbol", symbol).
			Str("vs_currency", vsCurrency).
			Str("cache_key", cacheKey).
			Msg("Cache hit for GetPrice")

		var result PriceResult
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		log.Warn().Err(err).Msg("Failed to unmarshal cached price, fetching fresh")
	} else if err != redis.Nil {
		// Log cache errors but continue with API call
		log.Warn().Err(err).Msg("Redis error during cache lookup")
	}

	// Cache miss or error - fetch from CoinGecko MCP
	log.Debug().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Msg("Cache miss, fetching from CoinGecko MCP")

	result, err := c.client.GetPrice(ctx, symbol, vsCurrency)
	if err != nil {
		return nil, err
	}

	// Store in cache (async, don't block on cache write failure)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		data, err := json.Marshal(result)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to marshal price for cache")
			return
		}

		if err := c.redis.Set(cacheCtx, cacheKey, data, c.cacheTTL).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to cache price result")
		} else {
			log.Debug().
				Str("cache_key", cacheKey).
				Dur("ttl", c.cacheTTL).
				Msg("Cached price result")
		}
	}()

	return result, nil
}

// GetMarketChart fetches market chart with caching
func (c *CachedCoinGeckoClient) GetMarketChart(ctx context.Context, symbol string, days int) (*MarketChart, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("coingecko:chart:%s:%d", symbol, days)

	// Check cache first
	cached, err := c.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit
		log.Debug().
			Str("symbol", symbol).
			Int("days", days).
			Str("cache_key", cacheKey).
			Msg("Cache hit for GetMarketChart")

		var result MarketChart
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		log.Warn().Err(err).Msg("Failed to unmarshal cached market chart, fetching fresh")
	} else if err != redis.Nil {
		log.Warn().Err(err).Msg("Redis error during cache lookup")
	}

	// Cache miss or error - fetch from CoinGecko MCP
	log.Debug().
		Str("symbol", symbol).
		Int("days", days).
		Msg("Cache miss, fetching from CoinGecko MCP")

	result, err := c.client.GetMarketChart(ctx, symbol, days)
	if err != nil {
		return nil, err
	}

	// Store in cache (async)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		data, err := json.Marshal(result)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to marshal market chart for cache")
			return
		}

		// Historical data can be cached longer (e.g., 5 minutes for daily charts)
		ttl := c.cacheTTL
		if days >= 7 {
			ttl = 5 * time.Minute // Historical data changes less frequently
		}

		if err := c.redis.Set(cacheCtx, cacheKey, data, ttl).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to cache market chart")
		} else {
			log.Debug().
				Str("cache_key", cacheKey).
				Dur("ttl", ttl).
				Msg("Cached market chart")
		}
	}()

	return result, nil
}

// GetCoinInfo fetches coin info with caching
func (c *CachedCoinGeckoClient) GetCoinInfo(ctx context.Context, coinID string) (*CoinInfo, error) {
	// Generate cache key
	cacheKey := fmt.Sprintf("coingecko:info:%s", coinID)

	// Check cache first
	cached, err := c.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit
		log.Debug().
			Str("coin_id", coinID).
			Str("cache_key", cacheKey).
			Msg("Cache hit for GetCoinInfo")

		var result CoinInfo
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		log.Warn().Err(err).Msg("Failed to unmarshal cached coin info, fetching fresh")
	} else if err != redis.Nil {
		log.Warn().Err(err).Msg("Redis error during cache lookup")
	}

	// Cache miss or error - fetch from CoinGecko MCP
	log.Debug().
		Str("coin_id", coinID).
		Msg("Cache miss, fetching from CoinGecko MCP")

	result, err := c.client.GetCoinInfo(ctx, coinID)
	if err != nil {
		return nil, err
	}

	// Store in cache (async)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		data, err := json.Marshal(result)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to marshal coin info for cache")
			return
		}

		// Coin metadata changes infrequently, cache for 10 minutes
		ttl := 10 * time.Minute

		if err := c.redis.Set(cacheCtx, cacheKey, data, ttl).Err(); err != nil {
			log.Warn().Err(err).Msg("Failed to cache coin info")
		} else {
			log.Debug().
				Str("cache_key", cacheKey).
				Dur("ttl", ttl).
				Msg("Cached coin info")
		}
	}()

	return result, nil
}

// Health checks both CoinGecko and Redis health
func (c *CachedCoinGeckoClient) Health(ctx context.Context) error {
	// Check CoinGecko client health
	if err := c.client.Health(ctx); err != nil {
		return fmt.Errorf("CoinGecko client unhealthy: %w", err)
	}

	// Check Redis health
	if err := c.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis unhealthy: %w", err)
	}

	return nil
}

// InvalidateCache invalidates all cached data for a specific symbol
func (c *CachedCoinGeckoClient) InvalidateCache(ctx context.Context, symbol string) error {
	pattern := fmt.Sprintf("coingecko:*:%s*", symbol)

	iter := c.redis.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.redis.Del(ctx, iter.Val()).Err(); err != nil {
			log.Warn().
				Err(err).
				Str("key", iter.Val()).
				Msg("Failed to delete cache key")
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache invalidation failed: %w", err)
	}

	log.Info().
		Str("pattern", pattern).
		Msg("Cache invalidated")

	return nil
}

// ClearCache clears all CoinGecko cached data
func (c *CachedCoinGeckoClient) ClearCache(ctx context.Context) error {
	pattern := "coingecko:*"

	iter := c.redis.Scan(ctx, 0, pattern, 0).Iterator()
	count := 0
	for iter.Next(ctx) {
		if err := c.redis.Del(ctx, iter.Val()).Err(); err != nil {
			log.Warn().
				Err(err).
				Str("key", iter.Val()).
				Msg("Failed to delete cache key")
		} else {
			count++
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache clear failed: %w", err)
	}

	log.Info().
		Int("keys_deleted", count).
		Msg("Cache cleared")

	return nil
}
