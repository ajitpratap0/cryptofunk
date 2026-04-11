package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	"github.com/ajitpratap0/cryptofunk/internal/exchange"
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
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
	sessionMu          sync.Mutex              // Protects orderExecSession, connecting, and activeSessionID
	connecting         bool                    // Guard against concurrent reconnect
	orderExecSession   *mcp.ClientSession      // MCP session for order-executor calls
	mcpClient          *mcp.Client             // MCP client for creating/reconnecting sessions
	activeSessionID    *uuid.UUID              // Currently active trading session ID (guarded by sessionMu)
	exchange           exchange.Exchange       // Shared mock exchange instance for paper trading
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
	// Bootstrap logging before config is loaded.
	// Default to console format so development is human-readable.
	// This will be reconfigured below once the full config is available.
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load and validate configuration
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load or validate configuration")
	}

	// Reconfigure logger now that we have the full config.
	// In production (or when LOG_FORMAT=json) emit structured JSON for log aggregators.
	// In development, keep the human-readable console format.
	logFormat := "console"
	if cfg.App.Environment == envProduction || cfg.App.Environment == "prod" || os.Getenv("LOG_FORMAT") == "json" {
		logFormat = "json"
	}
	config.InitLogger(cfg.App.LogLevel, logFormat)

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

	// Bound the initial connection attempt by a dedicated 30s timeout so the
	// process fails fast when Postgres is slow or unreachable. pgxpool.New
	// only uses the context for the initial AcquireConn; the pool itself is
	// torn down by database.Close() on shutdown, not by any stored context,
	// so it's safe to cancel dbInitCtx immediately after db.New returns
	// instead of deferring — the defer pattern would keep the cancel func
	// alive for the lifetime of main() without any benefit.
	dbInitCtx, dbInitCancel := context.WithTimeout(ctx, 30*time.Second)
	database, err := db.New(dbInitCtx, &cfg.Database)
	dbInitCancel()
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

	// Select exchange implementation based on configured trading mode.
	// PAPER mode (default): MockExchange with simulated fills and slippage.
	// LIVE mode: BinanceExchange using API key/secret from config or env.
	var ex exchange.Exchange
	if strings.EqualFold(cfg.Trading.Mode, "live") {
		binanceCfg := exchange.BinanceConfig{}
		if exc, ok := cfg.Exchanges["binance"]; ok {
			binanceCfg.APIKey = exc.APIKey
			binanceCfg.SecretKey = exc.SecretKey
		}
		binanceEx, err := exchange.NewBinanceExchange(binanceCfg, database)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize Binance exchange for LIVE trading")
		}
		ex = binanceEx
		log.Warn().Msg("LIVE trading mode: real orders will be placed on Binance")
	} else {
		ex = exchange.NewMockExchange(database)
		log.Info().Str("mode", cfg.Trading.Mode).Msg("Paper trading mode: using mock exchange")
	}

	// Create router and configure trusted proxies.
	// Setting an empty slice means Gin trusts NO forwarded headers, so
	// c.ClientIP() returns the real remote address instead of a spoofable header.
	// In production behind a known reverse proxy (e.g. nginx or an AWS ALB),
	// replace the empty slice with the specific proxy IP(s):
	//   router.SetTrustedProxies([]string{"10.0.0.1"})
	router := gin.Default()
	if err := router.SetTrustedProxies([]string{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to configure trusted proxies")
	}

	// Create API server
	server := &APIServer{
		router:             router,
		db:                 database,
		config:             cfg,
		hub:                hub,
		port:               getPort(),
		orchestratorClient: defaultOrchestratorClient,
		ctx:                ctx,
		safetyGuard:        safetyGuard,
		orderExecutorURL:   getOrderExecutorURL(),
		exchange:           ex,
	}

	// Initialize MCP client for order-executor (session connects lazily on first order)
	server.mcpClient = mcp.NewClient(
		&mcp.Implementation{
			Name:    "cryptofunk-api",
			Version: config.Version,
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

	// Start metrics updater — periodically refreshes DB pool stats, trading metrics,
	// position values, and agent status from the database.
	metricsUpdater := metrics.NewUpdater(database.Pool(), 5*time.Second)
	log.Warn().Float64("initial_capital", metrics.DefaultInitialCapital).Msg("metrics/updater: initialCapital is hardcoded to 10000; return metrics will be incorrect for non-10k portfolios until this is made configurable")
	metricsUpdater.StartAsync(ctx)
	defer metricsUpdater.Stop()

	// start() blocks until a SIGTERM/SIGINT signal is received and the graceful drain completes.
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

	// Register signal handler BEFORE starting the server to avoid missing SIGTERM
	// that arrives very shortly after process startup (e.g. from K8s pod eviction).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

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

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
		os.Exit(1)
	}

	// TODO(SEC-009): wire api.WaitForAsyncKeyOps (or its exported
	// sibling when one lands) into this drain path so in-flight
	// opportunistic-rehash and last_used_at goroutines complete before
	// os.Exit instead of being killed with the process. Currently the
	// 5s per-goroutine context.WithTimeout inside the goroutines
	// bounds the DB operation, not the process lifetime — a SIGTERM
	// with a <5s drain window will abandon them. The goroutines are
	// best-effort (the next validation retries), so this is not a
	// correctness issue, just a cleanliness one.

	log.Info().Msg("API server stopped")
}
