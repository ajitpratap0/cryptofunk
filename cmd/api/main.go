package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/api"
	"github.com/ajitpratap0/cryptofunk/internal/audit"
	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

type APIServer struct {
	router             *gin.Engine
	db                 *db.DB
	config             *config.Config
	hub                *Hub
	port               string
	orchestratorClient *http.Client
	rateLimiter        *RateLimiterMiddleware
}

// HTTP client for orchestrator communication with timeout and connection pooling
var defaultOrchestratorClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	},
}

// NOTE: Rate limiting code moved to middleware.go for better organization

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load and validate configuration
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load or validate configuration")
	}

	// Initialize database
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()

	// Set Gin mode based on environment
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create WebSocket hub
	hub := NewHub()
	go hub.Run()

	// Create API server
	server := &APIServer{
		router:             gin.Default(),
		db:                 database,
		config:             cfg,
		hub:                hub,
		port:               getPort(),
		orchestratorClient: defaultOrchestratorClient,
	}

	// Setup middleware
	server.setupMiddleware()

	// Setup routes
	server.setupRoutes()

	// Start server
	server.start()
}

func (s *APIServer) setupMiddleware() {
	// CORS configuration - use configured origins or defaults for development
	allowedOrigins := s.config.API.AllowedOrigins
	if len(allowedOrigins) == 0 {
		// Default origins for development
		allowedOrigins = []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"}
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

		// Position routes (read-only, apply read rate limiter)
		positions := v1.Group("/positions")
		positions.Use(s.rateLimiter.ReadMiddleware())
		{
			positions.GET("", s.handleListPositions)
			positions.GET("/:symbol", s.handleGetPosition)
		}

		// Order routes (mixed read/write, apply appropriate limiters)
		orders := v1.Group("/orders")
		{
			// Read operations (higher limit)
			orders.GET("", s.rateLimiter.ReadMiddleware(), s.handleListOrders)
			orders.GET("/:id", s.rateLimiter.ReadMiddleware(), s.handleGetOrder)

			// Write operations (lower limit to prevent order spam)
			orders.POST("", s.rateLimiter.OrderMiddleware(), s.handlePlaceOrder)
			orders.DELETE("/:id", s.rateLimiter.OrderMiddleware(), s.handleCancelOrder)
		}

		// Trading control routes (critical ops, strictest rate limiting)
		trade := v1.Group("/trade")
		trade.Use(s.rateLimiter.ControlMiddleware())
		{
			trade.POST("/start", s.handleStartTrading)
			trade.POST("/stop", s.handleStopTrading)
			trade.POST("/pause", s.handlePauseTrading)
			trade.POST("/resume", s.handleResumeTrading)
		}

		// Configuration routes (admin ops, apply control rate limiter)
		config := v1.Group("/config")
		{
			config.GET("", s.rateLimiter.ReadMiddleware(), s.handleGetConfig)
			config.PATCH("", s.rateLimiter.ControlMiddleware(), s.handleUpdateConfig)
		}

		// Decision explainability routes (T307) with rate limiting
		// Search uses dedicated rate limiter for expensive vector operations
		decisionRepo := api.NewDecisionRepository(s.db.Pool())
		decisionHandler := api.NewDecisionHandler(decisionRepo)
		decisionHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.SearchMiddleware())

		// Decision feedback routes (T309) with rate limiting
		feedbackRepo := api.NewFeedbackRepository(s.db.Pool())
		feedbackHandler := api.NewFeedbackHandler(feedbackRepo)
		feedbackHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.OrderMiddleware())

		// Strategy import/export routes (T310) with rate limiting
		strategyHandler := api.NewStrategyHandler()
		strategyHandler.RegisterRoutesWithRateLimiter(v1, s.rateLimiter.ReadMiddleware(), s.rateLimiter.OrderMiddleware())
	}

	// Root endpoint
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "CryptoFunk Trading API",
			"version": config.Version,
			"status":  "running",
		})
	})
}

func (s *APIServer) start() {
	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + s.port,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Str("port", s.port).
			Str("version", config.Version).
			Msg("Starting API server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start API server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down API server...")

	// Stop rate limiter cleanup worker to prevent goroutine leak
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("API server stopped")
}

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

// Agent handlers
func (s *APIServer) handleListAgents(c *gin.Context) {
	ctx := c.Request.Context()

	// Query agent status from database
	agents, err := s.db.GetAllAgentStatuses(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve agents",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}

func (s *APIServer) handleGetAgent(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Query specific agent
	agent, err := s.db.GetAgentStatus(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent not found",
			"name":  name,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent": agent,
	})
}

func (s *APIServer) handleGetAgentStatus(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Query agent status
	agent, err := s.db.GetAgentStatus(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent not found",
			"name":  name,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":           agent.Name,
		"type":           agent.Type,
		"status":         agent.Status,
		"last_heartbeat": agent.LastHeartbeat,
		"started_at":     agent.StartedAt,
		"total_signals":  agent.TotalSignals,
		"error_count":    agent.ErrorCount,
	})
}

// Position handlers
func (s *APIServer) handleListPositions(c *gin.Context) {
	ctx := c.Request.Context()

	// Optional: filter by session_id query param
	sessionIDStr := c.Query("session_id")

	var positions []*db.Position
	var err error

	if sessionIDStr != "" {
		// Parse session ID
		sessionID, parseErr := parseUUID(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid session_id format",
			})
			return
		}
		positions, err = s.db.GetPositionsBySession(ctx, sessionID)
	} else {
		// Get all open positions (no session filter)
		positions, err = s.db.GetAllOpenPositions(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve positions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"positions": positions,
		"count":     len(positions),
	})
}

func (s *APIServer) handleGetPosition(c *gin.Context) {
	symbol := c.Param("symbol")
	ctx := c.Request.Context()

	// Optional: filter by session_id
	sessionIDStr := c.Query("session_id")

	var position *db.Position
	var err error

	if sessionIDStr != "" {
		sessionID, parseErr := parseUUID(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid session_id format",
			})
			return
		}
		position, err = s.db.GetPositionBySymbolAndSession(ctx, symbol, sessionID)
	} else {
		// Get latest position for symbol
		position, err = s.db.GetLatestPositionBySymbol(ctx, symbol)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  "position not found",
			"symbol": symbol,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"position": position,
	})
}

// Order handlers
func (s *APIServer) handleListOrders(c *gin.Context) {
	ctx := c.Request.Context()

	// Optional filters
	sessionIDStr := c.Query("session_id")
	symbol := c.Query("symbol")
	status := c.Query("status")

	var orders []*db.Order
	var err error

	if sessionIDStr != "" {
		sessionID, parseErr := parseUUID(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid session_id format",
			})
			return
		}
		orders, err = s.db.GetOrdersBySession(ctx, sessionID)
	} else if symbol != "" {
		orders, err = s.db.GetOrdersBySymbol(ctx, symbol)
	} else if status != "" {
		orders, err = s.db.GetOrdersByStatus(ctx, db.ConvertOrderStatus(status))
	} else {
		// Get recent orders (limit 100)
		orders, err = s.db.GetRecentOrders(ctx, 100)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve orders",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"count":  len(orders),
	})
}

func (s *APIServer) handleGetOrder(c *gin.Context) {
	orderIDStr := c.Param("id")
	ctx := c.Request.Context()

	orderID, err := parseUUID(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid order_id format",
		})
		return
	}

	order, err := s.db.GetOrderByID(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "order not found",
			"order_id": orderIDStr,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order": order,
	})
}

func (s *APIServer) handlePlaceOrder(c *gin.Context) {
	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Side     string  `json:"side" binding:"required,oneof=buy sell BUY SELL"`
		Type     string  `json:"type" binding:"required,oneof=market limit MARKET LIMIT"`
		Quantity float64 `json:"quantity" binding:"required,gt=0"`
		Price    float64 `json:"price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate price for limit orders
	if (req.Type == "limit" || req.Type == "LIMIT") && req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "price is required for limit orders",
		})
		return
	}

	// Create order in database
	price := &req.Price
	if req.Price == 0 {
		price = nil
	}

	order := &db.Order{
		ID:        uuid.New(),
		Symbol:    req.Symbol,
		Exchange:  "API", // Manual order via API
		Side:      db.ConvertOrderSide(req.Side),
		Type:      db.ConvertOrderType(req.Type),
		Quantity:  req.Quantity,
		Price:     price,
		Status:    db.OrderStatusNew,
		PlacedAt:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := c.Request.Context()
	if err := s.db.InsertOrder(ctx, order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create order",
		})
		return
	}

	// Broadcast order update to WebSocket clients
	if err := s.BroadcastOrderUpdate(order); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast order update")
	}

	c.JSON(http.StatusCreated, gin.H{
		"order":   order,
		"message": "Order created successfully",
	})
}

func (s *APIServer) handleCancelOrder(c *gin.Context) {
	orderIDStr := c.Param("id")
	ctx := c.Request.Context()

	orderID, err := parseUUID(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid order_id format",
		})
		return
	}

	// Get the order first
	order, err := s.db.GetOrderByID(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "order not found",
			"order_id": orderIDStr,
		})
		return
	}

	// Check if order can be cancelled
	if order.Status != db.OrderStatusNew && order.Status != db.OrderStatusPartiallyFilled {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "order cannot be cancelled",
			"status": order.Status,
		})
		return
	}

	// Update order status to cancelled
	cancelledAt := time.Now()
	err = s.db.UpdateOrderStatus(ctx, orderID, db.OrderStatusCanceled, order.ExecutedQuantity, order.ExecutedQuoteQuantity, order.FilledAt, &cancelledAt, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to cancel order",
		})
		return
	}

	// Get updated order
	order, _ = s.db.GetOrderByID(ctx, orderID)

	// Broadcast order update to WebSocket clients
	if err := s.BroadcastOrderUpdate(order); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast order cancellation")
	}

	c.JSON(http.StatusOK, gin.H{
		"order":   order,
		"message": "Order cancelled successfully",
	})
}

// Trading control handlers
func (s *APIServer) handleStartTrading(c *gin.Context) {
	var req struct {
		Symbol         string  `json:"symbol" binding:"required"`
		InitialCapital float64 `json:"initial_capital" binding:"required,gt=0"`
		Mode           string  `json:"mode" binding:"oneof=paper live PAPER LIVE"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Default to paper trading if not specified
	if req.Mode == "" {
		req.Mode = "paper"
	}

	// Create a new trading session
	session := &db.TradingSession{
		Mode:           db.TradingMode(req.Mode),
		Symbol:         req.Symbol,
		Exchange:       "PAPER", // TODO: Make configurable
		StartedAt:      time.Now(),
		InitialCapital: req.InitialCapital,
	}

	ctx := c.Request.Context()
	if err := s.db.CreateSession(ctx, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create trading session",
		})
		return
	}

	// Broadcast system status update
	metadata := map[string]interface{}{
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
		"mode":       session.Mode,
		"event":      "trading_started",
	}
	if err := s.BroadcastSystemStatus("trading_started", "Trading session started", metadata); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast trading start")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Trading started successfully",
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
		"mode":       session.Mode,
		"started_at": session.StartedAt,
	})
}

func (s *APIServer) handleStopTrading(c *gin.Context) {
	var req struct {
		SessionID    string  `json:"session_id" binding:"required"`
		FinalCapital float64 `json:"final_capital" binding:"required,gte=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := parseUUID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	ctx := c.Request.Context()
	if err := s.db.StopSession(ctx, sessionID, req.FinalCapital); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to stop trading session",
		})
		return
	}

	// Get updated session
	session, _ := s.db.GetSession(ctx, sessionID)

	// Broadcast system status update
	metadata := map[string]interface{}{
		"session_id":    session.ID.String(),
		"final_capital": req.FinalCapital,
		"total_pnl":     session.TotalPnL,
		"total_trades":  session.TotalTrades,
		"event":         "trading_stopped",
	}
	if err := s.BroadcastSystemStatus("trading_stopped", "Trading session stopped", metadata); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast trading stop")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Trading stopped successfully",
		"session_id":    session.ID.String(),
		"final_capital": req.FinalCapital,
		"total_pnl":     session.TotalPnL,
		"total_trades":  session.TotalTrades,
		"stopped_at":    session.StoppedAt,
	})
}

func (s *APIServer) handlePauseTrading(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := parseUUID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	// Get session to verify it exists
	ctx := c.Request.Context()
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": req.SessionID,
		})
		return
	}

	// Call orchestrator to pause trading with retry
	orchestratorURL := s.getOrchestratorURL()
	resp, err := s.callOrchestratorWithRetry(orchestratorURL + "/pause")
	if err != nil {
		log.Error().Err(err).Msg("Failed to call orchestrator pause endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to pause trading",
			"details": err.Error(),
		})
		return
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Error().Err(cerr).Msg("Failed to close response body")
		}
	}()

	// Check orchestrator response
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{
			"error": "orchestrator failed to pause trading",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Trading paused successfully",
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
	})
}

func (s *APIServer) handleResumeTrading(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := parseUUID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	// Get session to verify it exists
	ctx := c.Request.Context()
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": req.SessionID,
		})
		return
	}

	// Call orchestrator to resume trading with retry
	orchestratorURL := s.getOrchestratorURL()
	resp, err := s.callOrchestratorWithRetry(orchestratorURL + "/resume")
	if err != nil {
		log.Error().Err(err).Msg("Failed to call orchestrator resume endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to resume trading",
			"details": err.Error(),
		})
		return
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Error().Err(cerr).Msg("Failed to close response body")
		}
	}()

	// Check orchestrator response
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{
			"error": "orchestrator failed to resume trading",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Trading resumed successfully",
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
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

// WebSocket broadcast helpers

// BroadcastPositionUpdate broadcasts a position update to all WebSocket clients
func (s *APIServer) BroadcastPositionUpdate(position *db.Position) error {
	data := map[string]interface{}{
		"position_id":    position.ID.String(),
		"session_id":     position.SessionID,
		"symbol":         position.Symbol,
		"exchange":       position.Exchange,
		"side":           position.Side,
		"entry_price":    position.EntryPrice,
		"exit_price":     position.ExitPrice,
		"quantity":       position.Quantity,
		"entry_time":     position.EntryTime,
		"exit_time":      position.ExitTime,
		"stop_loss":      position.StopLoss,
		"take_profit":    position.TakeProfit,
		"realized_pnl":   position.RealizedPnL,
		"unrealized_pnl": position.UnrealizedPnL,
		"fees":           position.Fees,
		"entry_reason":   position.EntryReason,
		"exit_reason":    position.ExitReason,
	}

	return s.hub.Broadcast(MessageTypePositionUpdate, data)
}

// BroadcastPnLUpdate broadcasts a P&L update to all WebSocket clients
func (s *APIServer) BroadcastPnLUpdate(sessionID uuid.UUID, totalPnL, realizedPnL, unrealizedPnL float64, positions []*db.Position) error {
	data := map[string]interface{}{
		"session_id":     sessionID.String(),
		"total_pnl":      totalPnL,
		"realized_pnl":   realizedPnL,
		"unrealized_pnl": unrealizedPnL,
		"position_count": len(positions),
		"timestamp":      time.Now(),
	}

	return s.hub.Broadcast(MessageTypePositionUpdate, data)
}

// BroadcastTradeNotification broadcasts a trade (fill) notification
func (s *APIServer) BroadcastTradeNotification(trade *db.Trade) error {
	data := map[string]interface{}{
		"trade_id":          trade.ID.String(),
		"order_id":          trade.OrderID.String(),
		"exchange_trade_id": trade.ExchangeTradeID,
		"symbol":            trade.Symbol,
		"exchange":          trade.Exchange,
		"side":              trade.Side,
		"price":             trade.Price,
		"quantity":          trade.Quantity,
		"quote_quantity":    trade.QuoteQuantity,
		"commission":        trade.Commission,
		"commission_asset":  trade.CommissionAsset,
		"executed_at":       trade.ExecutedAt,
		"is_maker":          trade.IsMaker,
	}

	return s.hub.Broadcast(MessageTypeTrade, data)
}

// BroadcastOrderUpdate broadcasts an order status update
func (s *APIServer) BroadcastOrderUpdate(order *db.Order) error {
	data := map[string]interface{}{
		"order_id":                order.ID.String(),
		"session_id":              order.SessionID,
		"position_id":             order.PositionID,
		"exchange_order_id":       order.ExchangeOrderID,
		"symbol":                  order.Symbol,
		"exchange":                order.Exchange,
		"side":                    order.Side,
		"type":                    order.Type,
		"status":                  order.Status,
		"price":                   order.Price,
		"stop_price":              order.StopPrice,
		"quantity":                order.Quantity,
		"executed_quantity":       order.ExecutedQuantity,
		"executed_quote_quantity": order.ExecutedQuoteQuantity,
		"time_in_force":           order.TimeInForce,
		"placed_at":               order.PlacedAt,
		"filled_at":               order.FilledAt,
		"canceled_at":             order.CanceledAt,
		"error_message":           order.ErrorMessage,
	}

	return s.hub.Broadcast(MessageTypeOrderUpdate, data)
}

// BroadcastAgentStatus broadcasts agent status change
func (s *APIServer) BroadcastAgentStatus(agent *db.AgentStatus) error {
	data := map[string]interface{}{
		"name":           agent.Name,
		"type":           agent.Type,
		"status":         agent.Status,
		"last_heartbeat": agent.LastHeartbeat,
		"started_at":     agent.StartedAt,
		"total_signals":  agent.TotalSignals,
		"error_count":    agent.ErrorCount,
		"metadata":       agent.Metadata,
	}

	return s.hub.Broadcast(MessageTypeAgentStatus, data)
}

// BroadcastSystemStatus broadcasts system status update
func (s *APIServer) BroadcastSystemStatus(status string, message string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"status":    status,
		"message":   message,
		"metadata":  metadata,
		"timestamp": time.Now(),
	}

	return s.hub.Broadcast(MessageTypeSystemStatus, data)
}

// BroadcastDecision broadcasts a new LLM decision to all WebSocket clients
func (s *APIServer) BroadcastDecision(decision *db.LLMDecision) error {
	data := map[string]interface{}{
		"id":            decision.ID.String(),
		"session_id":    decision.SessionID,
		"decision_type": decision.DecisionType,
		"symbol":        decision.Symbol,
		"agent_name":    decision.AgentName,
		"model":         decision.Model,
		"confidence":    decision.Confidence,
		"outcome":       decision.Outcome,
		"pnl":           decision.PnL,
		"tokens_used":   decision.TokensUsed,
		"latency_ms":    decision.LatencyMs,
		"created_at":    decision.CreatedAt,
		// Truncate prompt/response for real-time updates (full details via API)
		"prompt_preview":   truncateString(decision.Prompt, 200),
		"response_preview": truncateString(decision.Response, 200),
	}

	return s.hub.Broadcast(MessageTypeDecision, data)
}

// BroadcastDecisionStats broadcasts aggregated decision statistics.
// TODO: Integrate with periodic stats updates or decision outcome events.
func (s *APIServer) BroadcastDecisionStats(stats map[string]interface{}) error {
	data := map[string]interface{}{
		"stats":     stats,
		"timestamp": time.Now(),
	}

	return s.hub.Broadcast(MessageTypeDecisionStats, data)
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

// WebSocket handler

// createWebSocketUpgrader creates an upgrader with proper origin checking
// based on the configured allowed origins and environment
func (s *APIServer) createWebSocketUpgrader() websocket.Upgrader {
	allowedOrigins := s.config.API.AllowedOrigins
	isProduction := s.config.App.Environment == "production"

	if len(allowedOrigins) == 0 {
		if isProduction {
			// In production, require explicit configuration
			log.Warn().Msg("No allowed_origins configured for WebSocket in production - all origins will be rejected")
			allowedOrigins = []string{} // Empty list = reject all
		} else {
			// Default origins for development
			allowedOrigins = []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"}
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
				// Allow in development for testing tools like curl, wscat
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

		req, err := http.NewRequestWithContext(context.Background(), "POST", url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.orchestratorClient.Do(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Str("url", url).
			Msg("Orchestrator call failed")
	}

	return nil, fmt.Errorf("orchestrator call failed after %d attempts: %w", maxRetries, lastErr)
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
