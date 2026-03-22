package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/api"
	"github.com/ajitpratap0/cryptofunk/internal/audit"
	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
	"github.com/ajitpratap0/cryptofunk/internal/safety"
)

func (s *APIServer) setupMiddleware() {
	// Limit request bodies to 1 MB to prevent OOM attacks
	s.router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1 MB
		c.Next()
	})

	// Security headers middleware (applies to all responses)
	s.router.Use(securityHeadersMiddleware())

	// CORS configuration - use configured origins or defaults for development
	allowedOrigins := s.config.API.AllowedOrigins
	isProduction := s.config.App.Environment == envProduction

	if len(allowedOrigins) == 0 {
		if isProduction {
			// In production, require explicit configuration
			log.Warn().Msg("No allowed_origins configured for CORS in production - rejecting all origins")
			allowedOrigins = []string{} // Empty list = reject all
		} else {
			// Default origins for development
			allowedOrigins = []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:3333", "http://localhost:5173", "http://localhost:8080"}
		}
	}

	// In production, filter out localhost origins for security
	if isProduction {
		filteredOrigins := make([]string, 0, len(allowedOrigins))
		for _, origin := range allowedOrigins {
			if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
				log.Warn().
					Str("origin", origin).
					Msg("SECURITY WARNING: Blocking localhost origin in production CORS configuration")
			} else {
				filteredOrigins = append(filteredOrigins, origin)
			}
		}
		allowedOrigins = filteredOrigins

		// Log startup warning if no valid origins remain
		if len(allowedOrigins) == 0 {
			log.Error().Msg("SECURITY ERROR: No valid CORS origins configured in production after filtering localhost - all CORS requests will be rejected")
		}
	}

	config := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	// Add CORS middleware
	s.router.Use(cors.New(config))

	// Prometheus metrics middleware (before request logger to capture all requests)
	s.router.Use(metrics.GinMiddleware())

	// Audit logging middleware (logs security-relevant events)
	auditLogger := audit.NewLogger(s.db.Pool(), true)
	s.router.Use(AuditLoggingMiddleware(auditLogger))

	// Request logging middleware
	s.router.Use(requestLogger())

	// Recovery middleware
	s.router.Use(gin.Recovery())
}

func (s *APIServer) setupRoutes() {
	// Initialize comprehensive rate limiting middleware
	s.rateLimiter = NewRateLimiterMiddleware(DefaultRateLimiterConfig())

	// Start cleanup worker to remove stale IP entries (runs every 5 minutes)
	s.rateLimiter.StartCleanupWorker(5 * time.Minute)

	// Initialize API key store for authentication
	// Auth is disabled by default for development - enable via config.yaml: api.auth.enabled = true
	s.apiKeyStore = api.NewAPIKeyStore(s.db.Pool(), s.config.API.Auth.Enabled)

	isProduction := s.config.App.Environment == envProduction

	if s.config.API.Auth.Enabled {
		log.Info().
			Bool("require_https", s.config.API.Auth.RequireHTTPS).
			Str("header_name", s.config.API.Auth.HeaderName).
			Msg("API key authentication enabled")
	} else {
		// Always warn when authentication is disabled - security risk regardless of environment
		// Production warning is more severe, but all environments should be aware
		if isProduction {
			log.Error().
				Str("environment", s.config.App.Environment).
				Msg("CRITICAL: API authentication is DISABLED in production - all endpoints including trading control (pause/resume/start/stop) are unprotected. This is a security vulnerability. Enable via api.auth.enabled=true in config.yaml")
		} else {
			log.Warn().
				Str("environment", s.config.App.Environment).
				Msg("API authentication is disabled - all endpoints including trading control are unprotected. This is acceptable for development. Enable via api.auth.enabled=true for production.")
		}
	}

	// Apply global rate limiting to all API requests
	s.router.Use(s.rateLimiter.GlobalMiddleware())

	// Prometheus metrics endpoint (no API prefix, no rate limiting)
	s.router.GET("/metrics", gin.WrapH(metrics.Handler()))

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Health and status (no additional rate limiting beyond global)
		v1.GET("/health", s.handleHealth)
		v1.GET("/status", s.handleStatus)

		// WebSocket endpoint (no rate limiting - uses connection limits)
		v1.GET("/ws", s.handleWebSocket)

		// Agent routes (read-only, apply read rate limiter)
		agents := v1.Group("/agents")
		agents.Use(s.rateLimiter.ReadMiddleware())
		{
			agents.GET("", s.handleListAgents)
			agents.GET("/:name", s.handleGetAgent)
			agents.GET("/:name/status", s.handleGetAgentStatus)
		}

		// Session routes (read-only, apply read rate limiter)
		sessions := v1.Group("/sessions")
		sessions.Use(s.rateLimiter.ReadMiddleware())
		{
			sessions.GET("", s.handleListSessions)
			sessions.GET("/:id", s.handleGetSession)
		}

		// Position routes (read-only, apply read rate limiter)
		positions := v1.Group("/positions")
		positions.Use(s.rateLimiter.ReadMiddleware())
		{
			positions.GET("", s.handleListPositions)
			positions.GET("/:symbol", s.handleGetPosition)
		}

		// Create authentication config for protected endpoints
		authConfig := &api.AuthConfig{
			Enabled:      s.config.API.Auth.Enabled,
			HeaderName:   s.config.API.Auth.HeaderName,
			RequireHTTPS: s.config.API.Auth.RequireHTTPS,
		}
		if authConfig.HeaderName == "" {
			authConfig.HeaderName = "X-API-Key" // Default header name
		}

		// Order routes (mixed read/write, apply appropriate limiters + authentication)
		orders := v1.Group("/orders")
		if s.config.API.Auth.Enabled {
			orders.Use(api.AuthMiddleware(s.apiKeyStore, authConfig))
		}
		{
			// Read operations (higher limit)
			orders.GET("", s.rateLimiter.ReadMiddleware(), s.handleListOrders)
			orders.GET("/:id", s.rateLimiter.ReadMiddleware(), s.handleGetOrder)

			// Write operations (lower limit to prevent order spam)
			orders.POST("", s.rateLimiter.OrderMiddleware(), s.handlePlaceOrder)
			orders.DELETE("/:id", s.rateLimiter.OrderMiddleware(), s.handleCancelOrder)
		}

		// Trading control routes (critical ops, strictest rate limiting + authentication)
		// These endpoints control trading operations and require authentication when enabled
		trade := v1.Group("/trade")
		trade.Use(s.rateLimiter.ControlMiddleware())

		// Add authentication middleware for critical trading operations
		// When auth is enabled, these endpoints require a valid API key
		// This protects against unauthorized trading control
		if s.config.API.Auth.Enabled {
			trade.Use(api.AuthMiddleware(s.apiKeyStore, authConfig))
		}

		{
			trade.POST("", s.handlePaperTrade)
			trade.POST("/start", s.handleStartTrading)
			trade.POST("/stop", s.handleStopTrading)
			trade.POST("/pause", s.handlePauseTrading)
			trade.POST("/resume", s.handleResumeTrading)
		}

		// Configuration routes (admin ops, apply control rate limiter)
		configGroup := v1.Group("/config")
		{
			configGroup.GET("", s.rateLimiter.ReadMiddleware(), s.handleGetConfig)
			configGroup.PATCH("", s.rateLimiter.ControlMiddleware(), api.AuthMiddleware(s.apiKeyStore, authConfig), s.handleUpdateConfig)
		}

		// Decision explainability routes (T307) with rate limiting and optional auth
		// Decision routes require authentication when auth is enabled
		decisionsAuth := api.AuthMiddleware(s.apiKeyStore, authConfig)

		decisionRepo := api.NewDecisionRepository(s.db.Pool())
		decisionHandler := api.NewDecisionHandler(decisionRepo)
		decisionHandler.RegisterRoutesWithRateLimiterAndAuth(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.SearchMiddleware(), decisionsAuth)

		// Decision feedback routes (T309) with rate limiting
		feedbackRepo := api.NewFeedbackRepository(s.db.Pool())
		feedbackHandler := api.NewFeedbackHandler(feedbackRepo)
		feedbackHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.OrderMiddleware())

		// Strategy import/export routes (T310) with rate limiting
		strategyHandler := api.NewStrategyHandler()
		strategyHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.OrderMiddleware())

		// Backtest routes (T312) with rate limiting
		// Backtesting operations can be computationally expensive, so we apply stricter rate limits
		backtestHandler := api.NewBacktestHandler(s.db.Pool())
		backtestHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.OrderMiddleware())

		// Dashboard routes (TC-002) with rate limiting
		// Provides user dashboard with trading control, positions, P&L, and system status
		// Use an HTTP client to the orchestrator so the dashboard can report agent counts
		// even though the API and orchestrator run as separate processes.
		orchestratorURL := s.getOrchestratorURL()
		orchClient := api.NewOrchestratorClient(orchestratorURL)
		dashboardHandler := api.NewDashboardHandlerWithOrchestrator(s.db, orchClient, config.Version)
		dashboardHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.ControlMiddleware())

		// Trades routes — fill records from the trades table
		tradesHandler := api.NewTradesHandler(s.db)
		tradesHandler.RegisterRoutes(v1, s.rateLimiter.ReadMiddleware(), api.AuthMiddleware(s.apiKeyStore, authConfig))

		// TB-006: API Key Management routes
		// These endpoints allow users to manage their API keys (create, rotate, revoke)
		// All key management operations require authentication
		keysHandler := api.NewKeysHandler(s.db.Pool())
		s.keyManager = keysHandler.GetKeyManager()

		// Start the expired key cleanup worker (runs every hour)
		s.keyManager.StartCleanupWorker(s.ctx, time.Hour)

		keysHandler.RegisterRoutesWithRateLimiter(
			v1,
			api.AuthMiddleware(s.apiKeyStore, authConfig),
			s.rateLimiter.ReadMiddleware(),
			s.rateLimiter.OrderMiddleware(),
		)

		// Unified cross-platform portfolio routes
		unifiedHandler := api.NewUnifiedHandler(s.db)
		unifiedHandler.RegisterRoutes(v1, s.rateLimiter.ReadMiddleware())

		// Polymarket paper trading routes
		polymarketHandler := api.NewPolymarketHandler(s.db)
		polymarketHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.OrderMiddleware())

		// Risk metrics routes
		riskHandler := api.NewRiskHandler(s.db, &s.config.Risk)
		riskHandler.RegisterRoutes(v1, s.rateLimiter.ReadMiddleware(), api.AuthMiddleware(s.apiKeyStore, authConfig))

		// Performance routes
		perfHandler := api.NewPerformanceHandler(s.db)
		perfHandler.RegisterRoutes(v1, s.rateLimiter.ReadMiddleware(), api.AuthMiddleware(s.apiKeyStore, authConfig))

		// Decision analytics and outcome resolution routes
		decisionAnalyticsHandler := api.NewDecisionAnalyticsHandler(s.db)
		decisionAnalyticsHandler.RegisterRoutes(v1, s.rateLimiter.ReadMiddleware(), api.AuthMiddleware(s.apiKeyStore, authConfig))

		// TC-003: Safety guard routes
		safety.RegisterRoutes(v1, s.safetyGuard)
	}

	// Root endpoint
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":    "CryptoFunk Trading API",
			"version": config.Version,
			"status":  "running",
		})
	})
}
