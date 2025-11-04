package metrics

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisMetrics wraps a Redis client and instruments operations
type RedisMetrics struct {
	client *redis.Client
	hits   int64
	misses int64
}

// NewRedisMetrics creates a new instrumented Redis client
func NewRedisMetrics(client *redis.Client) *RedisMetrics {
	return &RedisMetrics{
		client: client,
		hits:   0,
		misses: 0,
	}
}

// Get performs a Redis GET and records metrics
func (rm *RedisMetrics) Get(ctx context.Context, key string) (string, error) {
	RecordRedisOperation("get")

	val, err := rm.client.Get(ctx, key).Result()
	if err == redis.Nil {
		rm.misses++
		rm.updateHitRate()
		return "", err
	} else if err != nil {
		return "", err
	}

	rm.hits++
	rm.updateHitRate()
	return val, nil
}

// Set performs a Redis SET and records metrics
func (rm *RedisMetrics) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	RecordRedisOperation("set")
	return rm.client.Set(ctx, key, value, expiration).Err()
}

// Del performs a Redis DEL and records metrics
func (rm *RedisMetrics) Del(ctx context.Context, keys ...string) error {
	RecordRedisOperation("del")
	return rm.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist and records metrics
func (rm *RedisMetrics) Exists(ctx context.Context, keys ...string) (int64, error) {
	RecordRedisOperation("exists")
	return rm.client.Exists(ctx, keys...).Result()
}

// Expire sets key expiration and records metrics
func (rm *RedisMetrics) Expire(ctx context.Context, key string, expiration time.Duration) error {
	RecordRedisOperation("expire")
	return rm.client.Expire(ctx, key, expiration).Err()
}

// Client returns the underlying Redis client
func (rm *RedisMetrics) Client() *redis.Client {
	return rm.client
}

// updateHitRate updates the cache hit rate metric
func (rm *RedisMetrics) updateHitRate() {
	total := rm.hits + rm.misses
	if total > 0 {
		hitRate := float64(rm.hits) / float64(total)
		RedisCacheHitRate.Set(hitRate)
	}
}

// ResetStats resets hit/miss statistics
func (rm *RedisMetrics) ResetStats() {
	rm.hits = 0
	rm.misses = 0
	RedisCacheHitRate.Set(0)
}
