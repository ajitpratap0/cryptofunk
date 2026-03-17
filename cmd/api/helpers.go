package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// Helper functions

var startTime = time.Now()

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// setActiveSessionID sets the active trading session ID (thread-safe).
func (s *APIServer) setActiveSessionID(id *uuid.UUID) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.activeSessionID = id
}

func getPort() string {
	// Try environment variable first
	if port := os.Getenv("API_PORT"); port != "" {
		return port
	}

	// Default port
	return "8080"
}

func getOrderExecutorURL() string {
	// Use the standard CRYPTOFUNK_* env var prefix (per project convention in CLAUDE.md)
	if url := os.Getenv("CRYPTOFUNK_MCP_INTERNAL_ORDER_EXECUTOR_URL"); url != "" {
		return url
	}
	// Default: order-executor MCP server on port 8091
	return "http://localhost:8091/mcp"
}

// connectOrderExecutor creates or reconnects the MCP session to the order-executor.
// Thread-safe: acquires sessionMu internally. Uses a "connecting" guard to prevent
// concurrent reconnect attempts from leaking sessions.
func (s *APIServer) connectOrderExecutor() error {
	if s.mcpClient == nil {
		return fmt.Errorf("MCP client not initialized")
	}

	s.sessionMu.Lock()
	// Guard: if another goroutine is already connecting, wait and reuse its result
	if s.connecting {
		s.sessionMu.Unlock()
		// Brief spin — concurrent connect is rare and fast (10s timeout)
		for i := 0; i < 100; i++ {
			time.Sleep(100 * time.Millisecond)
			s.sessionMu.Lock()
			if !s.connecting {
				s.sessionMu.Unlock()
				return nil // other goroutine finished
			}
			s.sessionMu.Unlock()
		}
		return fmt.Errorf("timed out waiting for concurrent reconnect")
	}
	s.connecting = true
	// Close stale session if one exists
	old := s.orderExecSession
	s.orderExecSession = nil
	s.sessionMu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			log.Debug().Err(err).Msg("Error closing stale order-executor session")
		}
	}

	// Connect outside the lock (network I/O can be slow).
	// Use s.ctx so reconnect respects server shutdown signals.
	parentCtx := s.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	connCtx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()
	transport := &mcp.StreamableClientTransport{Endpoint: s.orderExecutorURL}
	session, err := s.mcpClient.Connect(connCtx, transport, nil)

	s.sessionMu.Lock()
	s.connecting = false
	if err == nil {
		s.orderExecSession = session
	}
	s.sessionMu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to connect to order-executor: %w", err)
	}
	log.Info().Str("url", s.orderExecutorURL).Msg("Connected to order-executor MCP server")
	return nil
}

// executeOrder sends an order to the order-executor MCP server via the MCP SDK session.
// Thread-safe: uses sessionMu to protect session access and reconnect.
func (s *APIServer) executeOrder(ctx context.Context, symbol string, side string, orderType string, quantity float64, price float64) error {
	// Reconnect if session is nil (first call or after previous failure)
	s.sessionMu.Lock()
	needsConnect := s.orderExecSession == nil
	s.sessionMu.Unlock()

	if needsConnect {
		if err := s.connectOrderExecutor(); err != nil {
			return fmt.Errorf("order-executor unavailable: %w", err)
		}
	}

	s.sessionMu.Lock()
	session := s.orderExecSession
	s.sessionMu.Unlock()
	if session == nil {
		return fmt.Errorf("order-executor session not available after connect")
	}

	// Determine tool name and build arguments
	toolName := "place_market_order"
	args := map[string]interface{}{
		"symbol":   symbol,
		"side":     strings.ToLower(side),
		"quantity": quantity,
	}
	if strings.EqualFold(orderType, "LIMIT") {
		toolName = "place_limit_order"
		args["price"] = price
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		// Transport error — session is stale/broken. Reconnect and retry once
		// instead of failing this call and only succeeding on the next one.
		s.sessionMu.Lock()
		s.orderExecSession = nil
		s.sessionMu.Unlock()

		log.Warn().Err(err).Msg("MCP session broken, reconnecting and retrying")
		if reconnErr := s.connectOrderExecutor(); reconnErr != nil {
			return fmt.Errorf("order execution failed (reconnect also failed): %w", err)
		}

		// Retry with fresh session
		s.sessionMu.Lock()
		session = s.orderExecSession
		s.sessionMu.Unlock()
		if session == nil {
			return fmt.Errorf("order execution failed: session nil after reconnect")
		}

		retryCtx, retryCancel := context.WithTimeout(ctx, 30*time.Second)
		defer retryCancel()
		result, err = session.CallTool(retryCtx, &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		})
		if err != nil {
			s.sessionMu.Lock()
			s.orderExecSession = nil
			s.sessionMu.Unlock()
			return fmt.Errorf("order execution failed after retry: %w", err)
		}
	}

	// Tool-level error (session still valid — don't nil it out)
	if result.IsError {
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				return fmt.Errorf("order-executor error: %s", textContent.Text)
			}
		}
		return fmt.Errorf("order-executor error: tool returned error result")
	}

	return nil
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
