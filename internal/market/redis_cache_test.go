package market

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNewRedisPriceCache(t *testing.T) {
	tests := []struct {
		name        string
		client      *redis.Client
		ttl         time.Duration
		shouldBeNil bool
	}{
		{
			name:        "nil client returns nil",
			client:      nil,
			ttl:         60 * time.Second,
			shouldBeNil: true,
		},
		{
			name:        "valid client with TTL",
			client:      &redis.Client{},
			ttl:         60 * time.Second,
			shouldBeNil: false,
		},
		{
			name:        "valid client with zero TTL uses default",
			client:      &redis.Client{},
			ttl:         0,
			shouldBeNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewRedisPriceCache(tt.client, tt.ttl)
			if tt.shouldBeNil {
				if cache != nil {
					t.Errorf("Expected nil cache for nil client, but got non-nil cache")
				}
			} else {
				if cache == nil {
					t.Fatal("Expected non-nil cache for valid client, but got nil")
				}
				if cache.ttl == 0 {
					t.Errorf("Expected non-zero TTL, but got zero (input TTL: %v)", tt.ttl)
				}
			}
		})
	}
}

func TestRedisPriceCache_GetSet(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Test cache miss
	price, found := cache.Get(ctx, "bitcoin", "usd")
	if found {
		t.Error("Expected cache miss for uncached key 'bitcoin:usd', but found=true")
	}
	const tolerance = 1e-9
	if price != 0 {
		t.Errorf("Expected price 0.0 for cache miss, got %.10f", price)
	}

	// Set price (fire-and-forget, no error returned)
	cache.Set(ctx, "bitcoin", "usd", 50000.0)

	// Test cache hit
	price, found = cache.Get(ctx, "bitcoin", "usd")
	if !found {
		t.Error("Expected cache hit for key 'bitcoin:usd' after Set, but found=false")
	}
	expectedPrice := 50000.0
	if price < expectedPrice-tolerance || price > expectedPrice+tolerance {
		t.Errorf("Expected price %.2f (±%.2e), got %.10f", expectedPrice, tolerance, price)
	}
}

func TestRedisPriceCache_SetWithTTL(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Set with custom TTL (fire-and-forget, no error returned)
	cache.SetWithTTL(ctx, "ethereum", "usd", 3000.0, 1*time.Second)

	// Should be cached
	price, found := cache.Get(ctx, "ethereum", "usd")
	if !found {
		t.Error("Expected cache hit for 'ethereum:usd' immediately after SetWithTTL, but found=false")
	}
	const tolerance = 1e-9
	expectedPrice := 3000.0
	if price < expectedPrice-tolerance || price > expectedPrice+tolerance {
		t.Errorf("Expected price %.2f (±%.2e), got %.10f", expectedPrice, tolerance, price)
	}

	// Advance time in miniredis
	mr.FastForward(2 * time.Second)

	// Should be expired
	_, found = cache.Get(ctx, "ethereum", "usd")
	if found {
		t.Error("Expected cache miss for 'ethereum:usd' after 2 seconds (TTL=1s), but found=true")
	}
}

func TestRedisPriceCache_Delete(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Set price (fire-and-forget, no error returned)
	cache.Set(ctx, "bitcoin", "usd", 50000.0)

	// Verify it's cached
	_, found := cache.Get(ctx, "bitcoin", "usd")
	if !found {
		t.Error("Expected cache hit for 'bitcoin:usd' after Set, but found=false")
	}

	// Delete
	err = cache.Delete(ctx, "bitcoin", "usd")
	if err != nil {
		t.Fatalf("Failed to delete cache for 'bitcoin:usd': %v", err)
	}

	// Should be gone
	_, found = cache.Get(ctx, "bitcoin", "usd")
	if found {
		t.Error("Expected cache miss for 'bitcoin:usd' after Delete, but found=true")
	}
}

func TestRedisPriceCache_Clear(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Set multiple prices
	symbols := []struct {
		symbol   string
		currency string
		price    float64
	}{
		{"bitcoin", "usd", 50000.0},
		{"ethereum", "usd", 3000.0},
		{"cardano", "usd", 1.5},
	}

	for _, s := range symbols {
		// Set prices (fire-and-forget, no error returned)
		cache.Set(ctx, s.symbol, s.currency, s.price)
	}

	// Verify all are cached
	for _, s := range symbols {
		_, found := cache.Get(ctx, s.symbol, s.currency)
		if !found {
			t.Errorf("Expected cache hit for '%s:%s' after Set, but found=false", s.symbol, s.currency)
		}
	}

	// Clear cache
	err = cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify all are gone
	for _, s := range symbols {
		_, found := cache.Get(ctx, s.symbol, s.currency)
		if found {
			t.Errorf("Expected cache miss for '%s:%s' after Clear, but found=true", s.symbol, s.currency)
		}
	}
}

func TestRedisPriceCache_Health(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Health check should pass
	err = cache.Health(ctx)
	if err != nil {
		t.Errorf("Expected health check to pass for running Redis, but got error: %v", err)
	}

	// Close Redis
	mr.Close()

	// Health check should fail
	err = cache.Health(ctx)
	if err == nil {
		t.Error("Expected health check to fail after Redis close, but got no error")
	}
}

func TestRedisPriceCache_NilSafety(t *testing.T) {
	var cache *RedisPriceCache
	ctx := context.Background()

	// All methods should handle nil cache gracefully
	price, found := cache.Get(ctx, "bitcoin", "usd")
	if found {
		t.Error("Expected found=false for nil cache Get, but got found=true")
	}
	if price != 0 {
		t.Errorf("Expected price 0.0 for nil cache Get, got %.10f", price)
	}

	// Set should not panic for nil cache (fire-and-forget, no error returned)
	cache.Set(ctx, "bitcoin", "usd", 50000.0)

	err := cache.Delete(ctx, "bitcoin", "usd")
	if err == nil {
		t.Error("Expected error for nil cache Delete, but got no error")
	} else if err.Error() == "" {
		t.Error("Expected descriptive error message for nil cache Delete, but got empty string")
	}

	err = cache.Clear(ctx)
	if err == nil {
		t.Error("Expected error for nil cache Clear, but got no error")
	} else if err.Error() == "" {
		t.Error("Expected descriptive error message for nil cache Clear, but got empty string")
	}

	err = cache.Health(ctx)
	if err == nil {
		t.Error("Expected error for nil cache Health, but got no error")
	} else if err.Error() == "" {
		t.Error("Expected descriptive error message for nil cache Health, but got empty string")
	}
}

func TestRedisPriceCache_RedisFailureGraceful(t *testing.T) {
	// Create a client pointing to non-existent Redis
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:9999", // Non-existent Redis
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Get should return cache miss (not panic)
	price, found := cache.Get(ctx, "bitcoin", "usd")
	if found {
		t.Error("Expected found=false (cache miss) when Redis is unavailable, but got found=true")
	}
	if price != 0 {
		t.Errorf("Expected price 0.0 when Redis is unavailable, got %.10f", price)
	}

	// Set should not panic even when Redis is unavailable (fire-and-forget)
	// It logs the error internally but doesn't return it
	cache.Set(ctx, "bitcoin", "usd", 50000.0)
	// Test passes if no panic occurs
}

func TestRedisPriceCache_KeyFormat(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisPriceCache(client, 60*time.Second)
	ctx := context.Background()

	// Set price (fire-and-forget, no error returned)
	cache.Set(ctx, "bitcoin", "usd", 50000.0)

	// Check key format in Redis
	expectedKey := "cryptofunk:price:bitcoin:usd"
	exists, err := client.Exists(ctx, expectedKey).Result()
	if err != nil {
		t.Fatalf("Failed to check existence of key %q: %v", expectedKey, err)
	}
	if exists != 1 {
		t.Errorf("Expected key %q to exist in Redis (exists=1), but got exists=%d", expectedKey, exists)
	}
}
