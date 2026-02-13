package telegram

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	// Test with nil config (uses defaults)
	rl := NewRateLimiter(nil)
	require.NotNil(t, rl)
	defer rl.Stop()

	stats := rl.Stats()
	assert.Equal(t, 10, stats.MaxTokens)
	assert.Equal(t, 3*time.Second, stats.RefillRate)
	assert.Equal(t, 0, stats.ActiveUsers)
}

func TestNewRateLimiter_CustomConfig(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     5,
		RefillRate:    1 * time.Second,
		CleanupPeriod: 10 * time.Minute,
		InactivityTTL: 2 * time.Hour,
	}

	rl := NewRateLimiter(config)
	require.NotNil(t, rl)
	defer rl.Stop()

	stats := rl.Stats()
	assert.Equal(t, 5, stats.MaxTokens)
	assert.Equal(t, 1*time.Second, stats.RefillRate)
}

func TestRateLimiter_Allow(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     3,
		RefillRate:    100 * time.Millisecond,
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	userID := int64(123456789)

	// First 3 requests should be allowed (burst)
	assert.True(t, rl.Allow(userID), "First request should be allowed")
	assert.True(t, rl.Allow(userID), "Second request should be allowed")
	assert.True(t, rl.Allow(userID), "Third request should be allowed")

	// Fourth request should be rate limited
	assert.False(t, rl.Allow(userID), "Fourth request should be rate limited")

	// Wait for token refill
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again after refill
	assert.True(t, rl.Allow(userID), "Request after refill should be allowed")
}

func TestRateLimiter_MultipleUsers(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     2,
		RefillRate:    1 * time.Second,
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	user1 := int64(111111111)
	user2 := int64(222222222)

	// Each user should have their own bucket
	assert.True(t, rl.Allow(user1))
	assert.True(t, rl.Allow(user2))
	assert.True(t, rl.Allow(user1))
	assert.True(t, rl.Allow(user2))

	// Both should be rate limited now
	assert.False(t, rl.Allow(user1))
	assert.False(t, rl.Allow(user2))

	stats := rl.Stats()
	assert.Equal(t, 2, stats.ActiveUsers)
}

func TestRateLimiter_GetWaitTime(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     1,
		RefillRate:    500 * time.Millisecond,
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	userID := int64(123456789)

	// Before any requests, wait time should be 0
	assert.Equal(t, time.Duration(0), rl.GetWaitTime(userID))

	// Use up the token
	assert.True(t, rl.Allow(userID))

	// Now wait time should be positive
	waitTime := rl.GetWaitTime(userID)
	assert.True(t, waitTime > 0, "Wait time should be positive after exhausting tokens")
	assert.True(t, waitTime <= 500*time.Millisecond, "Wait time should be less than refill rate")
}

func TestRateLimiter_Reset(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     1,
		RefillRate:    1 * time.Hour, // Long refill to ensure we're rate limited
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	userID := int64(123456789)

	// Use up the token
	assert.True(t, rl.Allow(userID))
	assert.False(t, rl.Allow(userID))

	// Reset the user
	rl.Reset(userID)

	// Should be allowed again
	assert.True(t, rl.Allow(userID))
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     3,
		RefillRate:    50 * time.Millisecond,
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	userID := int64(123456789)

	// Use up all tokens
	assert.True(t, rl.Allow(userID))
	assert.True(t, rl.Allow(userID))
	assert.True(t, rl.Allow(userID))
	assert.False(t, rl.Allow(userID))

	// Wait for multiple tokens to refill
	time.Sleep(150 * time.Millisecond) // Should refill ~3 tokens

	// Should have multiple tokens available now
	assert.True(t, rl.Allow(userID))
	assert.True(t, rl.Allow(userID))
}

func TestRateLimiter_Cleanup(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     5,
		RefillRate:    1 * time.Second,
		CleanupPeriod: 50 * time.Millisecond,
		InactivityTTL: 100 * time.Millisecond,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	userID := int64(123456789)

	// Create a bucket
	assert.True(t, rl.Allow(userID))

	stats := rl.Stats()
	assert.Equal(t, 1, stats.ActiveUsers)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Bucket should be cleaned up
	stats = rl.Stats()
	assert.Equal(t, 0, stats.ActiveUsers)
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	config := DefaultRateLimiterConfig()

	assert.Equal(t, 10, config.MaxTokens)
	assert.Equal(t, 3*time.Second, config.RefillRate)
	assert.Equal(t, 5*time.Minute, config.CleanupPeriod)
	assert.Equal(t, 1*time.Hour, config.InactivityTTL)
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     100,
		RefillRate:    1 * time.Second,
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Test concurrent access doesn't cause race conditions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(userID int64) {
			for j := 0; j < 50; j++ {
				rl.Allow(userID)
				rl.GetWaitTime(userID)
			}
			done <- true
		}(int64(i))
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := rl.Stats()
	assert.Equal(t, 10, stats.ActiveUsers)
}

func TestRateLimiter_MaxTokensCap(t *testing.T) {
	config := &RateLimiterConfig{
		MaxTokens:     3,
		RefillRate:    10 * time.Millisecond,
		CleanupPeriod: 1 * time.Hour,
		InactivityTTL: 1 * time.Hour,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	userID := int64(123456789)

	// Use one token
	assert.True(t, rl.Allow(userID))

	// Wait for more tokens than max to refill
	time.Sleep(100 * time.Millisecond)

	// Should still only have max tokens available
	assert.True(t, rl.Allow(userID))
	assert.True(t, rl.Allow(userID))
	assert.True(t, rl.Allow(userID))
	assert.False(t, rl.Allow(userID)) // Should be limited after max
}
