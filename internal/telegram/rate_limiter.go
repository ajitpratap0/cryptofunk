package telegram

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for Telegram commands
type RateLimiter struct {
	mu            sync.RWMutex
	userBuckets   map[int64]*tokenBucket
	maxTokens     int           // Maximum tokens in bucket
	refillRate    time.Duration // How often to add a token
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// tokenBucket represents a token bucket for a single user
type tokenBucket struct {
	tokens       int
	lastRefill   time.Time
	lastActivity time.Time
}

// RateLimiterConfig holds configuration for the rate limiter
type RateLimiterConfig struct {
	MaxTokens     int           // Maximum requests allowed in burst (default: 10)
	RefillRate    time.Duration // Time to refill one token (default: 3 seconds)
	CleanupPeriod time.Duration // How often to clean up stale entries (default: 5 minutes)
	InactivityTTL time.Duration // Remove bucket after inactivity (default: 1 hour)
}

// DefaultRateLimiterConfig returns sensible defaults for rate limiting
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		MaxTokens:     10,              // Allow 10 commands in burst
		RefillRate:    3 * time.Second, // Refill 1 token every 3 seconds (~20 cmds/min sustained)
		CleanupPeriod: 5 * time.Minute,
		InactivityTTL: 1 * time.Hour,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	rl := &RateLimiter{
		userBuckets: make(map[int64]*tokenBucket),
		maxTokens:   config.MaxTokens,
		refillRate:  config.RefillRate,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine
	rl.cleanupTicker = time.NewTicker(config.CleanupPeriod)
	go rl.cleanupLoop(config.InactivityTTL)

	return rl
}

// Allow checks if a request from the given user should be allowed
// Returns true if allowed, false if rate limited
func (rl *RateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	bucket, exists := rl.userBuckets[userID]
	if !exists {
		// Create new bucket with full tokens
		rl.userBuckets[userID] = &tokenBucket{
			tokens:       rl.maxTokens - 1, // Consume one token for this request
			lastRefill:   now,
			lastActivity: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > rl.maxTokens {
			bucket.tokens = rl.maxTokens
		}
		bucket.lastRefill = now
	}

	bucket.lastActivity = now

	// Check if we have tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// GetWaitTime returns how long the user needs to wait before making another request
func (rl *RateLimiter) GetWaitTime(userID int64) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	bucket, exists := rl.userBuckets[userID]
	if !exists || bucket.tokens > 0 {
		return 0
	}

	// Calculate time until next token
	elapsed := time.Since(bucket.lastRefill)
	remaining := rl.refillRate - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset resets the rate limiter for a specific user (useful for testing)
func (rl *RateLimiter) Reset(userID int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.userBuckets, userID)
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
	rl.cleanupTicker.Stop()
}

// cleanupLoop periodically removes inactive user buckets to prevent memory leaks
func (rl *RateLimiter) cleanupLoop(inactivityTTL time.Duration) {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.cleanup(inactivityTTL)
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes buckets that haven't been accessed recently
func (rl *RateLimiter) cleanup(inactivityTTL time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for userID, bucket := range rl.userBuckets {
		if now.Sub(bucket.lastActivity) > inactivityTTL {
			delete(rl.userBuckets, userID)
		}
	}
}

// Stats returns current statistics about the rate limiter
func (rl *RateLimiter) Stats() RateLimiterStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return RateLimiterStats{
		ActiveUsers: len(rl.userBuckets),
		MaxTokens:   rl.maxTokens,
		RefillRate:  rl.refillRate,
	}
}

// RateLimiterStats contains statistics about the rate limiter
type RateLimiterStats struct {
	ActiveUsers int
	MaxTokens   int
	RefillRate  time.Duration
}
