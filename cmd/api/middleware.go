package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/audit"
)

// HTTP method constants to satisfy goconst linter
const (
	httpMethodGET    = "GET"
	httpMethodPOST   = "POST"
	httpMethodPATCH  = "PATCH"
	httpMethodDELETE = "DELETE"
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

	// Search endpoints (expensive vector operations)
	// Separate limit for semantic search which is CPU/memory intensive
	SearchMaxRequests int
	SearchWindow      time.Duration

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

		// Search endpoints: 20 requests per minute (vector search is expensive)
		SearchMaxRequests: 20,
		SearchWindow:      time.Minute,

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

// rateLimitInfo contains information about the current rate limit state
type rateLimitInfo struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// check checks if a request from the given IP is allowed and returns rate limit info
func (rl *RateLimiter) check(ip string) rateLimitInfo {
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
	var oldestRequest time.Time
	for _, req := range entry.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
			if oldestRequest.IsZero() || req.Before(oldestRequest) {
				oldestRequest = req
			}
		}
	}
	entry.requests = validRequests

	// Calculate reset time (when the oldest request expires)
	resetAt := now.Add(rl.window)
	if !oldestRequest.IsZero() {
		resetAt = oldestRequest.Add(rl.window)
	}

	// Check if we're at the limit
	if len(entry.requests) >= rl.maxRequests {
		log.Warn().
			Str("ip", ip).
			Str("limiter", rl.name).
			Int("requests", len(entry.requests)).
			Int("max", rl.maxRequests).
			Dur("window", rl.window).
			Msg("Rate limit exceeded")
		return rateLimitInfo{
			Allowed:   false,
			Limit:     rl.maxRequests,
			Remaining: 0,
			ResetAt:   resetAt,
		}
	}

	// Add this request
	entry.requests = append(entry.requests, now)
	return rateLimitInfo{
		Allowed:   true,
		Limit:     rl.maxRequests,
		Remaining: rl.maxRequests - len(entry.requests),
		ResetAt:   resetAt,
	}
}

// allow checks if a request from the given IP is allowed (backwards compatible)
func (rl *RateLimiter) allow(ip string) bool {
	return rl.check(ip).Allowed
}

// Middleware returns a Gin middleware that applies rate limiting
// Adds standard rate limit headers to all responses:
//   - X-RateLimit-Limit: Maximum requests allowed in the window
//   - X-RateLimit-Remaining: Requests remaining in current window
//   - X-RateLimit-Reset: Unix timestamp when the rate limit resets
//   - Retry-After: Seconds until the rate limit resets (only on 429)
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		info := rl.check(ip)

		// Always set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetAt.Unix()))

		if !info.Allowed {
			retryAfter := int(time.Until(info.ResetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"message":     fmt.Sprintf("Maximum %d requests per %v allowed", rl.maxRequests, rl.window),
				"retry_after": retryAfter,
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
	search  *RateLimiter
	enabled bool

	// Cleanup worker management
	stopChan chan struct{}
	doneChan chan struct{}
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
		search:  NewRateLimiter("search", config.SearchMaxRequests, config.SearchWindow),
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

// SearchMiddleware returns middleware for search endpoints (expensive vector operations)
func (rlm *RateLimiterMiddleware) SearchMiddleware() gin.HandlerFunc {
	if !rlm.enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return rlm.search.Middleware()
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
	cleanupLimiter(rlm.search)
}

// StartCleanupWorker starts a background goroutine that periodically cleans up old entries.
// Call Stop() to gracefully shutdown the worker and prevent goroutine leaks.
func (rlm *RateLimiterMiddleware) StartCleanupWorker(interval time.Duration) {
	rlm.stopChan = make(chan struct{})
	rlm.doneChan = make(chan struct{})

	ticker := time.NewTicker(interval)
	go func() {
		defer close(rlm.doneChan)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rlm.CleanupOldEntries()
				log.Debug().Msg("Rate limiter cleanup completed")
			case <-rlm.stopChan:
				log.Debug().Msg("Rate limiter cleanup worker stopping")
				return
			}
		}
	}()
}

// Stop gracefully shuts down the cleanup worker goroutine.
// This should be called during server shutdown to prevent goroutine leaks.
func (rlm *RateLimiterMiddleware) Stop() {
	if rlm.stopChan == nil {
		return // Worker was never started
	}

	// Signal the worker to stop
	close(rlm.stopChan)

	// Wait for the worker to finish with a timeout
	select {
	case <-rlm.doneChan:
		log.Info().Msg("Rate limiter cleanup worker stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Warn().Msg("Rate limiter cleanup worker did not stop in time")
	}
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
	if path == "/api/v1/orders" && method == httpMethodPOST {
		return audit.EventTypeOrderPlaced
	}
	if method == httpMethodDELETE && len(path) > len("/api/v1/orders/") && path[:len("/api/v1/orders/")] == "/api/v1/orders/" {
		return audit.EventTypeOrderCanceled
	}

	// Configuration endpoints
	if path == "/api/v1/config" && method == httpMethodPATCH {
		return audit.EventTypeConfigUpdated
	}
	if path == "/api/v1/config" && method == httpMethodGET {
		return audit.EventTypeConfigViewed
	}

	// Decision explainability endpoints
	if strings.HasPrefix(path, "/api/v1/decisions") {
		// POST /decisions/search
		if path == "/api/v1/decisions/search" && method == httpMethodPOST {
			return audit.EventTypeDecisionSearched
		}
		// GET /decisions/stats
		if path == "/api/v1/decisions/stats" && method == httpMethodGET {
			return audit.EventTypeDecisionStatsAccessed
		}
		// GET /decisions/:id/similar
		if strings.HasSuffix(path, "/similar") && method == httpMethodGET {
			return audit.EventTypeDecisionSimilarAccessed
		}
		// GET /decisions/:id (single decision)
		if method == httpMethodGET && path != "/api/v1/decisions" && !strings.HasSuffix(path, "/similar") && !strings.HasSuffix(path, "/stats") {
			return audit.EventTypeDecisionViewed
		}
		// GET /decisions (list)
		if path == "/api/v1/decisions" && method == httpMethodGET {
			return audit.EventTypeDecisionListAccessed
		}
	}

	// Return empty for non-critical endpoints (health checks, status, etc.)
	return ""
}
