package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/api"
	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/safety"
)

const (
	envProduction = "production"
)

type APIServer struct {
	router             *gin.Engine
	db                 *db.DB
	config             *config.Config
	hub                *Hub
	port               string
	orchestratorClient *http.Client
	rateLimiter        *RateLimiterMiddleware
	apiKeyStore        *api.APIKeyStore
	keyManager         api.KeyManagerInterface // TB-006: API key lifecycle management
	ctx                context.Context         // Server lifecycle context for background workers
	safetyGuard        *safety.Guard           // TC-003: Safety guard
	orderExecutorURL   string                  // MCP endpoint for order-executor server
	sessionMu          sync.Mutex              // Protects orderExecSession and connecting
	connecting         bool                    // Guard against concurrent reconnect
	orderExecSession   *mcp.ClientSession      // MCP session for order-executor calls
	mcpClient          *mcp.Client             // MCP client for creating/reconnecting sessions
	activeSessionID    *uuid.UUID              // Currently active trading session ID
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

	// CRITICAL SECURITY CHECK: Prevent production deployment with API auth disabled
	// This validation MUST happen before any server initialization
	isProduction := cfg.App.Environment == envProduction || cfg.App.Environment == "prod"
	if isProduction && !cfg.API.Auth.Enabled {
		log.Fatal().
			Str("environment", cfg.App.Environment).
			Bool("auth_enabled", cfg.API.Auth.Enabled).
			Msg("CRITICAL SECURITY ERROR: Cannot start API server in production with authentication disabled. " +
				"This would expose all trading control endpoints (start/stop/pause/resume) without protection. " +
				"Set api.auth.enabled=true in config.yaml or change environment to non-production.")
		os.Exit(1)
	}

	// Initialize database with signal-derived context
	ctx, ctxCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer ctxCancel()
	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()

	// Set Gin mode based on environment
	if cfg.App.Environment == envProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create WebSocket hub
	hub := NewHub()
	go hub.Run()

	// TC-003: Initialize safety guard
	safetyLimits := safety.NewLimitsConfig()
	if cfgPath := os.Getenv("SAFETY_CONFIG_PATH"); cfgPath != "" {
		if err := safetyLimits.LoadFromFile(cfgPath); err != nil {
			log.Warn().Err(err).Msg("Failed to load safety config file, using defaults")
		}
	}
	safetyLimits.LoadFromEnv()
	safetyMonitor := safety.NewMonitor(0) // portfolio value updated at runtime
	safetyGuard := safety.NewGuard(safetyLimits, safetyMonitor)

	// Create API server
	server := &APIServer{
		router:             gin.Default(),
		db:                 database,
		config:             cfg,
		hub:                hub,
		port:               getPort(),
		orchestratorClient: defaultOrchestratorClient,
		ctx:                ctx,
		safetyGuard:        safetyGuard,
		orderExecutorURL:   getOrderExecutorURL(),
	}

	// Initialize MCP client for order-executor (session connects lazily on first order)
	server.mcpClient = mcp.NewClient(
		&mcp.Implementation{
			Name:    "cryptofunk-api",
			Version: "1.0.0",
		},
		nil,
	)
	// Attempt initial connection (non-fatal if order-executor isn't ready yet)
	if err := server.connectOrderExecutor(); err != nil {
		log.Warn().Err(err).Str("url", server.orderExecutorURL).
			Msg("Order-executor not available at startup — will retry on first order")
	}

	// Setup middleware
	server.setupMiddleware()

	// Setup routes
	server.setupRoutes()

	// Start server
	server.start()
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

	// TB-006: Stop key manager cleanup worker
	if s.keyManager != nil {
		s.keyManager.StopCleanupWorker()
	}

	// Close MCP session (capture under lock, close outside)
	s.sessionMu.Lock()
	session := s.orderExecSession
	s.orderExecSession = nil
	s.sessionMu.Unlock()
	if session != nil {
		if err := session.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close order-executor MCP session")
		}
	}

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("API server stopped")
}
