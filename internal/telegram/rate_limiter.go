package telegram

import (
	"sync"
	"time"
)

// RateLimiter implements a per-user rate limiter using a sliding window
type RateLimiter struct {
	mu         sync.Mutex
	requests   map[int64][]time.Time
	maxPerMin  int
	cleanupTTL time.Duration
}

// NewRateLimiter creates a new rate limiter with the given max requests per minute
func NewRateLimiter(maxPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		requests:   make(map[int64][]time.Time),
		maxPerMin:  maxPerMinute,
		cleanupTTL: 5 * time.Minute,
	}
	go rl.cleanup()
	return rl
}

// Allow checks if a user is allowed to make a request
func (rl *RateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	// Filter to only recent requests
	recent := make([]time.Time, 0)
	for _, t := range rl.requests[userID] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.maxPerMin {
		rl.requests[userID] = recent
		return false
	}

	rl.requests[userID] = append(recent, now)
	return true
}

// cleanup periodically removes old entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupTTL)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-time.Minute)
		for userID, times := range rl.requests {
			recent := make([]time.Time, 0)
			for _, t := range times {
				if t.After(cutoff) {
					recent = append(recent, t)
				}
			}
			if len(recent) == 0 {
				delete(rl.requests, userID)
			} else {
				rl.requests[userID] = recent
			}
		}
		rl.mu.Unlock()
	}
}
