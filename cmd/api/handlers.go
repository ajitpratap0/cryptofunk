package main

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// Health check handler
func (s *APIServer) handleHealth(c *gin.Context) {
	// Check database connection
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"error":   "database connection failed",
			"version": config.Version,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": config.Version,
		"uptime":  time.Since(startTime).String(),
	})
}

// System status handler
func (s *APIServer) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "operational",
		"version": config.Version,
		"uptime":  time.Since(startTime).String(),
		"components": gin.H{
			"database":  "healthy",
			"api":       "healthy",
			"websocket": "healthy",
		},
		"websocket": gin.H{
			"connected_clients": s.hub.ClientCount(),
		},
	})
}

// Config handlers
func (s *APIServer) handleGetConfig(c *gin.Context) {
	// Return sanitized configuration (no API keys, passwords, or secrets)
	sanitized := s.sanitizeConfig(s.config)

	c.JSON(http.StatusOK, gin.H{
		"config": sanitized,
	})
}

func (s *APIServer) handleUpdateConfig(c *gin.Context) {
	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if stderrors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Apply updates to safe configuration fields
	updates := make(map[string]interface{})
	errors := []string{}

	// Allow updating trading configuration
	if tradingMode, ok := req["trading_mode"].(string); ok {
		if tradingMode == "paper" || tradingMode == "live" {
			s.config.Trading.Mode = tradingMode
			updates["trading_mode"] = tradingMode
		} else {
			errors = append(errors, "trading_mode must be 'paper' or 'live'")
		}
	}

	if initialCapital, ok := req["initial_capital"].(float64); ok {
		if initialCapital > 0 {
			s.config.Trading.InitialCapital = initialCapital
			updates["initial_capital"] = initialCapital
		} else {
			errors = append(errors, "initial_capital must be positive")
		}
	}

	if maxPositions, ok := req["max_positions"].(float64); ok {
		if maxPositions > 0 {
			s.config.Trading.MaxPositions = int(maxPositions)
			updates["max_positions"] = int(maxPositions)
		} else {
			errors = append(errors, "max_positions must be positive")
		}
	}

	// Allow updating risk configuration
	if maxPositionSize, ok := req["max_position_size"].(float64); ok {
		if maxPositionSize > 0 && maxPositionSize <= 1 {
			s.config.Risk.MaxPositionSize = maxPositionSize
			updates["max_position_size"] = maxPositionSize
		} else {
			errors = append(errors, "max_position_size must be between 0 and 1")
		}
	}

	if maxDailyLoss, ok := req["max_daily_loss"].(float64); ok {
		if maxDailyLoss > 0 && maxDailyLoss <= 1 {
			s.config.Risk.MaxDailyLoss = maxDailyLoss
			updates["max_daily_loss"] = maxDailyLoss
		} else {
			errors = append(errors, "max_daily_loss must be between 0 and 1")
		}
	}

	if maxDrawdown, ok := req["max_drawdown"].(float64); ok {
		if maxDrawdown > 0 && maxDrawdown <= 1 {
			s.config.Risk.MaxDrawdown = maxDrawdown
			updates["max_drawdown"] = maxDrawdown
		} else {
			errors = append(errors, "max_drawdown must be between 0 and 1")
		}
	}

	if defaultStopLoss, ok := req["default_stop_loss"].(float64); ok {
		if defaultStopLoss > 0 && defaultStopLoss <= 1 {
			s.config.Risk.DefaultStopLoss = defaultStopLoss
			updates["default_stop_loss"] = defaultStopLoss
		} else {
			errors = append(errors, "default_stop_loss must be between 0 and 1")
		}
	}

	if defaultTakeProfit, ok := req["default_take_profit"].(float64); ok {
		if defaultTakeProfit > 0 {
			s.config.Risk.DefaultTakeProfit = defaultTakeProfit
			updates["default_take_profit"] = defaultTakeProfit
		} else {
			errors = append(errors, "default_take_profit must be positive")
		}
	}

	if minConfidence, ok := req["min_confidence"].(float64); ok {
		if minConfidence >= 0 && minConfidence <= 1 {
			s.config.Risk.MinConfidence = minConfidence
			updates["min_confidence"] = minConfidence
		} else {
			errors = append(errors, "min_confidence must be between 0 and 1")
		}
	}

	if llmApprovalRequired, ok := req["llm_approval_required"].(bool); ok {
		s.config.Risk.LLMApprovalRequired = llmApprovalRequired
		updates["llm_approval_required"] = llmApprovalRequired
	}

	// Allow updating LLM configuration (safe fields only)
	if temperature, ok := req["llm_temperature"].(float64); ok {
		if temperature >= 0 && temperature <= 2 {
			s.config.LLM.Temperature = temperature
			updates["llm_temperature"] = temperature
		} else {
			errors = append(errors, "llm_temperature must be between 0 and 2")
		}
	}

	if maxTokens, ok := req["llm_max_tokens"].(float64); ok {
		if maxTokens > 0 {
			s.config.LLM.MaxTokens = int(maxTokens)
			updates["llm_max_tokens"] = int(maxTokens)
		} else {
			errors = append(errors, "llm_max_tokens must be positive")
		}
	}

	// Check for any validation errors
	if len(errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation failed",
			"errors": errors,
		})
		return
	}

	// If no updates were made
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "no valid configuration fields to update",
			"message": "See documentation for updatable fields",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully",
		"updates": updates,
		"note":    "Changes are in-memory only and will reset on server restart",
	})
}

// sanitizeConfig removes sensitive information from config
func (s *APIServer) sanitizeConfig(cfg *config.Config) map[string]interface{} {
	return map[string]interface{}{
		"app": map[string]interface{}{
			"name":        cfg.App.Name,
			"version":     cfg.App.Version,
			"environment": cfg.App.Environment,
			"log_level":   cfg.App.LogLevel,
		},
		"database": map[string]interface{}{
			"host":      cfg.Database.Host,
			"port":      cfg.Database.Port,
			"database":  cfg.Database.Database,
			"ssl_mode":  cfg.Database.SSLMode,
			"pool_size": cfg.Database.PoolSize,
			// Omit: user, password
		},
		"redis": map[string]interface{}{
			"host": cfg.Redis.Host,
			"port": cfg.Redis.Port,
			"db":   cfg.Redis.DB,
			// Omit: password
		},
		"nats": map[string]interface{}{
			"url":              cfg.NATS.URL,
			"enable_jetstream": cfg.NATS.EnableJetStream,
		},
		"llm": map[string]interface{}{
			"gateway":        cfg.LLM.Gateway,
			"endpoint":       cfg.LLM.Endpoint,
			"primary_model":  cfg.LLM.PrimaryModel,
			"fallback_model": cfg.LLM.FallbackModel,
			"temperature":    cfg.LLM.Temperature,
			"max_tokens":     cfg.LLM.MaxTokens,
			"enable_caching": cfg.LLM.EnableCaching,
			"timeout":        cfg.LLM.Timeout,
		},
		"trading": map[string]interface{}{
			"mode":             cfg.Trading.Mode,
			"symbols":          cfg.Trading.Symbols,
			"exchange":         cfg.Trading.Exchange,
			"initial_capital":  cfg.Trading.InitialCapital,
			"max_positions":    cfg.Trading.MaxPositions,
			"default_quantity": cfg.Trading.DefaultQuantity,
		},
		"risk": map[string]interface{}{
			"max_position_size":     cfg.Risk.MaxPositionSize,
			"max_daily_loss":        cfg.Risk.MaxDailyLoss,
			"max_drawdown":          cfg.Risk.MaxDrawdown,
			"default_stop_loss":     cfg.Risk.DefaultStopLoss,
			"default_take_profit":   cfg.Risk.DefaultTakeProfit,
			"llm_approval_required": cfg.Risk.LLMApprovalRequired,
			"min_confidence":        cfg.Risk.MinConfidence,
		},
		"api": map[string]interface{}{
			"host": cfg.API.Host,
			"port": cfg.API.Port,
		},
		"monitoring": map[string]interface{}{
			"prometheus_port": cfg.Monitoring.PrometheusPort,
			"enable_metrics":  cfg.Monitoring.EnableMetrics,
		},
		"mcp": map[string]interface{}{
			"external": map[string]interface{}{
				"coingecko": map[string]interface{}{
					"enabled":   cfg.MCP.External.CoinGecko.Enabled,
					"name":      cfg.MCP.External.CoinGecko.Name,
					"transport": cfg.MCP.External.CoinGecko.Transport,
					"cache_ttl": cfg.MCP.External.CoinGecko.CacheTTL,
				},
			},
			"internal": map[string]interface{}{
				"order_executor": map[string]interface{}{
					"enabled":   cfg.MCP.Internal.OrderExecutor.Enabled,
					"name":      cfg.MCP.Internal.OrderExecutor.Name,
					"transport": cfg.MCP.Internal.OrderExecutor.Transport,
				},
				"risk_analyzer": map[string]interface{}{
					"enabled":   cfg.MCP.Internal.RiskAnalyzer.Enabled,
					"name":      cfg.MCP.Internal.RiskAnalyzer.Name,
					"transport": cfg.MCP.Internal.RiskAnalyzer.Transport,
				},
				"technical_indicators": map[string]interface{}{
					"enabled":   cfg.MCP.Internal.TechnicalIndicators.Enabled,
					"name":      cfg.MCP.Internal.TechnicalIndicators.Name,
					"transport": cfg.MCP.Internal.TechnicalIndicators.Transport,
				},
				"market_data": map[string]interface{}{
					"enabled":   cfg.MCP.Internal.MarketData.Enabled,
					"name":      cfg.MCP.Internal.MarketData.Name,
					"transport": cfg.MCP.Internal.MarketData.Transport,
				},
			},
		},
		// Omit exchanges entirely (contains API keys and secrets)
	}
}

// WebSocket handler

// createWebSocketUpgrader creates an upgrader with proper origin checking
// based on the configured allowed origins and environment
func (s *APIServer) createWebSocketUpgrader() websocket.Upgrader {
	allowedOrigins := s.config.API.AllowedOrigins
	isProduction := s.config.App.Environment == envProduction

	if len(allowedOrigins) == 0 {
		if isProduction {
			// In production, require explicit configuration
			log.Warn().Msg("No allowed_origins configured for WebSocket in production - all origins will be rejected")
			allowedOrigins = []string{} // Empty list = reject all
		} else {
			// Default origins for development
			allowedOrigins = []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:3333", "http://localhost:5173", "http://localhost:8080"}
		}
	}

	// Validate production configuration
	if isProduction {
		for _, origin := range allowedOrigins {
			if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
				log.Warn().Str("origin", origin).Msg("WARNING: localhost origin configured in production WebSocket")
			}
			if !strings.HasPrefix(origin, "https://") {
				log.Warn().Str("origin", origin).Msg("WARNING: non-HTTPS origin configured in production WebSocket")
			}
		}
	}

	// Create a map for O(1) lookup
	originMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		originMap[origin] = true
	}

	return websocket.Upgrader{
		ReadBufferSize:  4096, // Increased for real-time trading data
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")

			// In production, reject requests with no origin header
			// This prevents non-browser clients from bypassing origin validation
			if origin == "" {
				if isProduction {
					log.Warn().
						Str("remote_addr", r.RemoteAddr).
						Str("path", r.URL.Path).
						Msg("WebSocket connection rejected - missing origin header in production")
					return false
				}
				// In development mode, allow connections without Origin header.
				// This enables testing with non-browser tools like wscat, Postman,
				// curl, or other CLI clients that don't send Origin headers.
				// In production, this check is more strict and requires a valid Origin
				// from the allowed domains list to prevent CSRF attacks.
				return true
			}

			// Check if origin is in allowed list
			allowed := originMap[origin]
			if !allowed {
				log.Warn().
					Str("origin", origin).
					Str("remote_addr", r.RemoteAddr).
					Msg("WebSocket connection rejected - origin not in allowed list")
			}
			return allowed
		},
	}
}

func (s *APIServer) handleWebSocket(c *gin.Context) {
	// Create upgrader with configured origins
	upgrader := s.createWebSocketUpgrader()

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	// Create new client
	client := &Client{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	// Register client with hub
	client.hub.register <- client

	// Start client's goroutines
	go client.writePump()
	go client.readPump()

	log.Info().
		Str("remote_addr", c.Request.RemoteAddr).
		Msg("WebSocket client connected")
}
