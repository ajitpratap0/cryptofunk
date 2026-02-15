package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Helper functions

var startTime = time.Now()

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func getPort() string {
	// Try environment variable first
	if port := os.Getenv("API_PORT"); port != "" {
		return port
	}

	// Default port
	return "8080"
}

func (s *APIServer) getOrchestratorURL() string {
	// Try environment variable first (highest priority)
	if url := os.Getenv("ORCHESTRATOR_URL"); url != "" {
		return url
	}

	// Use configured URL (from config.yaml)
	if s.config != nil && s.config.API.OrchestratorURL != "" {
		return s.config.API.OrchestratorURL
	}

	// Fallback to default URL (orchestrator metrics server on port 8081)
	return "http://localhost:8081"
}

// callOrchestratorWithRetry calls the orchestrator endpoint with retry logic
func (s *APIServer) callOrchestratorWithRetry(url string) (*http.Response, error) {
	const maxRetries = 3
	const retryDelay = 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(attempt)) // Exponential backoff
			log.Debug().
				Int("attempt", attempt+1).
				Int("max_retries", maxRetries).
				Str("url", url).
				Msg("Retrying orchestrator call")
		}

		parentCtx := s.ctx
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		reqCtx, reqCancel := context.WithTimeout(parentCtx, 10*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, "POST", url, nil)
		if err != nil {
			reqCancel()
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.orchestratorClient.Do(req)
		if err == nil {
			reqCancel()
			return resp, nil
		}

		reqCancel()
		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Str("url", url).
			Msg("Orchestrator call failed")
	}

	return nil, fmt.Errorf("orchestrator call failed after %d attempts: %w", maxRetries, lastErr)
}

// truncateString truncates a string to maxLen and adds "..." if truncated.
// For maxLen < 4, returns the first maxLen characters without "...".
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// requestLogger logs each HTTP request
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log after request
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		logEvent := log.Info()
		if statusCode >= 400 {
			logEvent = log.Warn()
		}
		if statusCode >= 500 {
			logEvent = log.Error()
		}

		logEvent.
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Int("status", statusCode).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("HTTP request")
	}
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS protection in older browsers (most modern browsers ignore this)
		c.Header("X-XSS-Protection", "1; mode=block")

		c.Next()
	}
}
