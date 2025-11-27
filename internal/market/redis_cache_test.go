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
					t.Error("Expected nil cache")
				}
			} else {
				if cache == nil {
					t.Fatal("Expected non-nil cache")
				}
				if cache.ttl == 0 {
					t.Error("Expected non-zero TTL")
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
		t.Error("Expected cache miss")
	}
	if price != 0 {
		t.Errorf("Expected price 0, got %f", price)
	}

	// Set price
	err = cache.Set(ctx, "bitcoin", "usd", 50000.0)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Test cache hit
	price, found = cache.Get(ctx, "bitcoin", "usd")
	if !found {
		t.Error("Expected cache hit")
	}
	if price != 50000.0 {
		t.Errorf("Expected price 50000.0, got %f", price)
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

	// Set with custom TTL
	err = cache.SetWithTTL(ctx, "ethereum", "usd", 3000.0, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to set cache with TTL: %v", err)
	}

	// Should be cached
	price, found := cache.Get(ctx, "ethereum", "usd")
	if !found {
		t.Error("Expected cache hit")
	}
	if price != 3000.0 {
		t.Errorf("Expected price 3000.0, got %f", price)
	}

	// Advance time in miniredis
	mr.FastForward(2 * time.Second)

	// Should be expired
	_, found = cache.Get(ctx, "ethereum", "usd")
	if found {
		t.Error("Expected cache miss after expiration")
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

	// Set price
	err = cache.Set(ctx, "bitcoin", "usd", 50000.0)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Verify it's cached
	_, found := cache.Get(ctx, "bitcoin", "usd")
	if !found {
		t.Error("Expected cache hit")
	}

	// Delete
	err = cache.Delete(ctx, "bitcoin", "usd")
	if err != nil {
		t.Fatalf("Failed to delete cache: %v", err)
	}

	// Should be gone
	_, found = cache.Get(ctx, "bitcoin", "usd")
	if found {
		t.Error("Expected cache miss after delete")
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
		err = cache.Set(ctx, s.symbol, s.currency, s.price)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}
	}

	// Verify all are cached
	for _, s := range symbols {
		_, found := cache.Get(ctx, s.symbol, s.currency)
		if !found {
			t.Errorf("Expected cache hit for %s", s.symbol)
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
			t.Errorf("Expected cache miss for %s after clear", s.symbol)
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
		t.Errorf("Expected health check to pass: %v", err)
	}

	// Close Redis
	mr.Close()

	// Health check should fail
	err = cache.Health(ctx)
	if err == nil {
		t.Error("Expected health check to fail after Redis close")
	}
}

func TestRedisPriceCache_NilSafety(t *testing.T) {
	var cache *RedisPriceCache
	ctx := context.Background()

	// All methods should handle nil cache gracefully
	price, found := cache.Get(ctx, "bitcoin", "usd")
	if found {
		t.Error("Expected false for nil cache")
	}
	if price != 0 {
		t.Errorf("Expected price 0, got %f", price)
	}

	err := cache.Set(ctx, "bitcoin", "usd", 50000.0)
	if err == nil {
		t.Error("Expected error for nil cache Set")
	}

	err = cache.Delete(ctx, "bitcoin", "usd")
	if err == nil {
		t.Error("Expected error for nil cache Delete")
	}

	err = cache.Clear(ctx)
	if err == nil {
		t.Error("Expected error for nil cache Clear")
	}

	err = cache.Health(ctx)
	if err == nil {
		t.Error("Expected error for nil cache Health")
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
		t.Error("Expected cache miss on Redis failure")
	}
	if price != 0 {
		t.Errorf("Expected price 0, got %f", price)
	}

	// Set should return error but not panic
	err := cache.Set(ctx, "bitcoin", "usd", 50000.0)
	if err == nil {
		t.Error("Expected error when Redis is unavailable")
	}
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

	// Set price
	err = cache.Set(ctx, "bitcoin", "usd", 50000.0)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Check key format in Redis
	expectedKey := "cryptofunk:price:bitcoin:usd"
	exists, err := client.Exists(ctx, expectedKey).Result()
	if err != nil {
		t.Fatalf("Failed to check key existence: %v", err)
	}
	if exists != 1 {
		t.Errorf("Expected key %s to exist", expectedKey)
	}
}
