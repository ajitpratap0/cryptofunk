package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// mockRedisClient is a simple mock for testing RedisMetrics without a real Redis connection
// Note: This test will be skipped if Redis is not available (following graceful degradation pattern)

func TestNewRedisMetrics(t *testing.T) {
	// Create a Redis client (will fail to connect, but that's okay for constructor test)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rm := NewRedisMetrics(client)

	assert.NotNil(t, rm)
	assert.Equal(t, client, rm.client)
	assert.Equal(t, int64(0), rm.hits)
	assert.Equal(t, int64(0), rm.misses)
}

func TestRedisMetrics_Client(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rm := NewRedisMetrics(client)

	// Client() should return the underlying client
	assert.Equal(t, client, rm.Client())
}

func TestRedisMetrics_ResetStats(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rm := NewRedisMetrics(client)

	// Set some values
	rm.hits = 100
	rm.misses = 50

	// Reset
	rm.ResetStats()

	assert.Equal(t, int64(0), rm.hits)
	assert.Equal(t, int64(0), rm.misses)
}

func TestRedisMetrics_UpdateHitRate(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rm := NewRedisMetrics(client)

	// Test with no hits/misses
	assert.NotPanics(t, func() {
		rm.updateHitRate()
	})

	// Test with some hits
	rm.hits = 80
	rm.misses = 20

	assert.NotPanics(t, func() {
		rm.updateHitRate()
	})

	// Test with all hits
	rm.hits = 100
	rm.misses = 0

	assert.NotPanics(t, func() {
		rm.updateHitRate()
	})

	// Test with all misses
	rm.hits = 0
	rm.misses = 100

	assert.NotPanics(t, func() {
		rm.updateHitRate()
	})
}

// Integration tests - these require a real Redis instance
// They will be skipped if Redis is not available

func TestRedisMetrics_Get_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use a test database
	})

	ctx := context.Background()

	// Check if Redis is available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	// Clean up test key
	testKey := "test:metrics:get"
	client.Del(ctx, testKey)

	// Test cache miss
	_, err := rm.Get(ctx, testKey)
	assert.Error(t, err)
	assert.Equal(t, redis.Nil, err)
	assert.Equal(t, int64(0), rm.hits)
	assert.Equal(t, int64(1), rm.misses)

	// Set a value
	client.Set(ctx, testKey, "test-value", time.Minute)

	// Reset stats
	rm.ResetStats()

	// Test cache hit
	val, err := rm.Get(ctx, testKey)
	assert.NoError(t, err)
	assert.Equal(t, "test-value", val)
	assert.Equal(t, int64(1), rm.hits)
	assert.Equal(t, int64(0), rm.misses)

	// Clean up
	client.Del(ctx, testKey)
}

func TestRedisMetrics_Set_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	testKey := "test:metrics:set"
	client.Del(ctx, testKey)

	// Test set
	err := rm.Set(ctx, testKey, "test-value", time.Minute)
	assert.NoError(t, err)

	// Verify value was set
	val, err := client.Get(ctx, testKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "test-value", val)

	// Clean up
	client.Del(ctx, testKey)
}

func TestRedisMetrics_Del_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	testKey := "test:metrics:del"

	// Set a value first
	client.Set(ctx, testKey, "test-value", time.Minute)

	// Test delete
	err := rm.Del(ctx, testKey)
	assert.NoError(t, err)

	// Verify key was deleted
	_, err = client.Get(ctx, testKey).Result()
	assert.Error(t, err)
	assert.Equal(t, redis.Nil, err)
}

func TestRedisMetrics_Exists_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	testKey := "test:metrics:exists"
	client.Del(ctx, testKey)

	// Test non-existent key
	count, err := rm.Exists(ctx, testKey)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Set a value
	client.Set(ctx, testKey, "test-value", time.Minute)

	// Test existing key
	count, err = rm.Exists(ctx, testKey)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Clean up
	client.Del(ctx, testKey)
}

func TestRedisMetrics_Expire_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	testKey := "test:metrics:expire"

	// Set a value
	client.Set(ctx, testKey, "test-value", 0) // No expiration

	// Set expiration
	err := rm.Expire(ctx, testKey, time.Second)
	assert.NoError(t, err)

	// Verify TTL is set
	ttl, err := client.TTL(ctx, testKey).Result()
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, time.Second)

	// Clean up
	client.Del(ctx, testKey)
}

func TestRedisMetrics_HitRateCalculation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	testKey1 := "test:metrics:hit1"
	testKey2 := "test:metrics:hit2"

	// Clean up
	client.Del(ctx, testKey1, testKey2)

	// Set one key
	client.Set(ctx, testKey1, "value1", time.Minute)

	// Reset stats
	rm.ResetStats()

	// Generate 2 hits and 1 miss
	_, _ = rm.Get(ctx, testKey1) // hit - error ignored for test stats
	_, _ = rm.Get(ctx, testKey1) // hit - error ignored for test stats
	_, _ = rm.Get(ctx, testKey2) // miss - error ignored for test stats

	// Verify stats
	assert.Equal(t, int64(2), rm.hits)
	assert.Equal(t, int64(1), rm.misses)

	// Clean up
	client.Del(ctx, testKey1, testKey2)
}

func TestRedisMetrics_MultipleKeys_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	defer func() { _ = client.Close() }() // Test cleanup

	rm := NewRedisMetrics(client)

	keys := []string{"test:multi:1", "test:multi:2", "test:multi:3"}

	// Set multiple keys
	for i, key := range keys {
		err := rm.Set(ctx, key, i, time.Minute)
		assert.NoError(t, err)
	}

	// Delete multiple keys
	err := rm.Del(ctx, keys...)
	assert.NoError(t, err)

	// Verify all deleted
	for _, key := range keys {
		_, err := client.Get(ctx, key).Result()
		assert.Error(t, err)
	}
}
