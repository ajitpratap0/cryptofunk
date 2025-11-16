package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// RateLimiterConfig defines rate limiting configuration for different endpoint types
type RateLimiterConfig struct {
	// Global limits (applies to all endpoints)
	GlobalMaxRequests int
	GlobalWindow      time.Duration

	// Control endpoints (start/stop/pause/resume trading)
	ControlMaxRequests int
	ControlWindow      time.Duration

	// Order endpoints (place/cancel orders)
	OrderMaxRequests int
	OrderWindow      time.Duration

	// Read-only endpoints (list positions, orders, agents)
	ReadMaxRequests int
	ReadWindow      time.Duration

	// Enable/disable rate limiting
	Enabled bool
}

// DefaultRateLimiterConfig returns the default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		// Global: 100 requests per minute per IP
		GlobalMaxRequests: 100,
		GlobalWindow:      time.Minute,

		// Control endpoints: 10 requests per minute (prevent DOS on critical ops)
		ControlMaxRequests: 10,
		ControlWindow:      time.Minute,

		// Order endpoints: 30 requests per minute (allow trading activity)
		OrderMaxRequests: 30,
		OrderWindow:      time.Minute,

		// Read endpoints: 60 requests per minute (allow monitoring)
		ReadMaxRequests: 60,
		ReadWindow:      time.Minute,

		Enabled: true,
	}
}

// rateLimiterEntry tracks request timestamps for an IP address
type rateLimiterEntry struct {
	requests []time.Time
	mu       sync.Mutex
}

// RateLimiter implements token bucket rate limiting per IP address
type RateLimiter struct {
	entries     sync.Map // map[string]*rateLimiterEntry
	maxRequests int
	window      time.Duration
	name        string // For logging
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(name string, maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		maxRequests: maxRequests,
		window:      window,
		name:        name,
	}
}

// allow checks if a request from the given IP is allowed
func (rl *RateLimiter) allow(ip string) bool {
	now := time.Now()

	// Get or create entry for this IP
	val, _ := rl.entries.LoadOrStore(ip, &rateLimiterEntry{
		requests: make([]time.Time, 0, rl.maxRequests),
	})
	entry := val.(*rateLimiterEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Remove expired requests (outside the time window)
	cutoff := now.Add(-rl.window)
	validRequests := make([]time.Time, 0, len(entry.requests))
	for _, req := range entry.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	entry.requests = validRequests

	// Check if we're at the limit
	if len(entry.requests) >= rl.maxRequests {
		log.Warn().
			Str("ip", ip).
			Str("limiter", rl.name).
			Int("requests", len(entry.requests)).
			Int("max", rl.maxRequests).
			Dur("window", rl.window).
			Msg("Rate limit exceeded")
		return false
	}

	// Add this request
	entry.requests = append(entry.requests, now)
	return true
}

// Middleware returns a Gin middleware that applies rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": fmt.Sprintf("Maximum %d requests per %v allowed", rl.maxRequests, rl.window),
				"retry_after": rl.window.Seconds(),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimiterMiddleware manages multiple rate limiters
type RateLimiterMiddleware struct {
	global  *RateLimiter
	control *RateLimiter
	order   *RateLimiter
	read    *RateLimiter
	enabled bool
}

// NewRateLimiterMiddleware creates a new rate limiter middleware with the given config
func NewRateLimiterMiddleware(config *RateLimiterConfig) *RateLimiterMiddleware {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	return &RateLimiterMiddleware{
		global:  NewRateLimiter("global", config.GlobalMaxRequests, config.GlobalWindow),
		control: NewRateLimiter("control", config.ControlMaxRequests, config.ControlWindow),
		order:   NewRateLimiter("order", config.OrderMaxRequests, config.OrderWindow),
		read:    NewRateLimiter("read", config.ReadMaxRequests, config.ReadWindow),
		enabled: config.Enabled,
	}
}

// GlobalMiddleware returns middleware that applies global rate limiting to all requests
func (rlm *RateLimiterMiddleware) GlobalMiddleware() gin.HandlerFunc {
	if !rlm.enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return rlm.global.Middleware()
}

// ControlMiddleware returns middleware for control endpoints (start/stop/pause/resume)
func (rlm *RateLimiterMiddleware) ControlMiddleware() gin.HandlerFunc {
	if !rlm.enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return rlm.control.Middleware()
}

// OrderMiddleware returns middleware for order endpoints (place/cancel)
func (rlm *RateLimiterMiddleware) OrderMiddleware() gin.HandlerFunc {
	if !rlm.enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return rlm.order.Middleware()
}

// ReadMiddleware returns middleware for read-only endpoints (list/get)
func (rlm *RateLimiterMiddleware) ReadMiddleware() gin.HandlerFunc {
	if !rlm.enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return rlm.read.Middleware()
}

// CleanupOldEntries removes stale IP entries from all rate limiters (call periodically)
func (rlm *RateLimiterMiddleware) CleanupOldEntries() {
	now := time.Now()

	cleanupLimiter := func(limiter *RateLimiter) {
		limiter.entries.Range(func(key, value interface{}) bool {
			entry := value.(*rateLimiterEntry)
			entry.mu.Lock()
			cutoff := now.Add(-limiter.window * 2) // Keep entries for 2x window
			hasValidRequests := false
			for _, req := range entry.requests {
				if req.After(cutoff) {
					hasValidRequests = true
					break
				}
			}
			entry.mu.Unlock()

			if !hasValidRequests {
				limiter.entries.Delete(key)
			}
			return true
		})
	}

	cleanupLimiter(rlm.global)
	cleanupLimiter(rlm.control)
	cleanupLimiter(rlm.order)
	cleanupLimiter(rlm.read)
}

// StartCleanupWorker starts a background goroutine that periodically cleans up old entries
func (rlm *RateLimiterMiddleware) StartCleanupWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			rlm.CleanupOldEntries()
			log.Debug().Msg("Rate limiter cleanup completed")
		}
	}()
}
