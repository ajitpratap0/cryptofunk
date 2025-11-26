package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/audit"
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
				"error":       "rate limit exceeded",
				"message":     fmt.Sprintf("Maximum %d requests per %v allowed", rl.maxRequests, rl.window),
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

// AuditLoggingMiddleware creates middleware that logs all requests to the audit log
func AuditLoggingMiddleware(auditLogger *audit.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID for correlation
		requestID := uuid.New().String()
		c.Set("request_id", requestID)

		// Record start time
		start := time.Now()

		// Extract request info
		ipAddress := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		method := c.Request.Method
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Milliseconds()

		// Get response status
		statusCode := c.Writer.Status()
		success := statusCode >= 200 && statusCode < 400

		// Determine event type based on path and method
		eventType := determineEventType(method, path)
		if eventType == "" {
			// Skip audit logging for non-critical endpoints
			return
		}

		// Determine severity
		severity := audit.SeverityInfo
		if !success {
			if statusCode >= 500 {
				severity = audit.SeverityError
			} else if statusCode >= 400 {
				severity = audit.SeverityWarning
			}
		}

		// Get error message if any
		var errorMsg string
		if !success {
			if err, exists := c.Get("error"); exists {
				errorMsg = fmt.Sprintf("%v", err)
			}
		}

		// Get user ID if authenticated (placeholder for now)
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = fmt.Sprintf("%v", userID)
		}

		// Get resource ID if available
		resource, _ := c.Get("resource_id")
		resourceStr := ""
		if resource != nil {
			resourceStr = fmt.Sprintf("%v", resource)
		}

		// Create audit event
		event := &audit.Event{
			EventType: eventType,
			Severity:  severity,
			UserID:    userIDStr,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			Resource:  resourceStr,
			Action:    fmt.Sprintf("%s %s", method, path),
			Success:   success,
			ErrorMsg:  errorMsg,
			RequestID: requestID,
			Duration:  duration,
		}

		// Log asynchronously to avoid blocking the response
		// Capture context before goroutine to avoid race condition - the request
		// context may be modified or become invalid after ServeHTTP returns
		ctx := c.Request.Context()
		go func() {
			if err := auditLogger.Log(ctx, event); err != nil {
				log.Error().Err(err).Msg("Failed to log audit event")
			}
		}()
	}
}

// determineEventType maps HTTP paths to audit event types
func determineEventType(method, path string) audit.EventType {
	// Trading control endpoints
	if path == "/api/v1/trade/start" {
		return audit.EventTypeTradingStart
	}
	if path == "/api/v1/trade/stop" {
		return audit.EventTypeTradingStop
	}
	if path == "/api/v1/trade/pause" {
		return audit.EventTypeTradingPause
	}
	if path == "/api/v1/trade/resume" {
		return audit.EventTypeTradingResume
	}

	// Order endpoints
	if path == "/api/v1/orders" && method == "POST" {
		return audit.EventTypeOrderPlaced
	}
	if method == "DELETE" && len(path) > len("/api/v1/orders/") && path[:len("/api/v1/orders/")] == "/api/v1/orders/" {
		return audit.EventTypeOrderCanceled
	}

	// Configuration endpoints
	if path == "/api/v1/config" && method == "PATCH" {
		return audit.EventTypeConfigUpdated
	}
	if path == "/api/v1/config" && method == "GET" {
		return audit.EventTypeConfigViewed
	}

	// Return empty for non-critical endpoints (health checks, status, etc.)
	return ""
}
