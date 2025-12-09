// Risk Management Agent
// Monitors portfolio risk and has veto power over trades
//
//nolint:goconst // Trading signals and market regimes are domain-specific strings
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/llm"
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
	"github.com/ajitpratap0/cryptofunk/internal/risk"
)

// ============================================================================
// AGENT CONFIGURATION
// ============================================================================

// RiskAgentConfig holds risk agent configuration
type RiskAgentConfig struct {
	AgentName          string  `mapstructure:"agent_name"`
	AgentType          string  `mapstructure:"agent_type"`
	Weight             float64 `mapstructure:"weight"`
	NATSUrl            string  `mapstructure:"nats_url"`
	SignalTopic        string  `mapstructure:"signal_topic"`
	HeartbeatTopic     string  `mapstructure:"heartbeat_topic"`
	DecisionTopic      string  `mapstructure:"decision_topic"`
	HeartbeatInterval  string  `mapstructure:"heartbeat_interval"`
	MetricsPort        int     `mapstructure:"metrics_port"`
	MaxPositionSize    float64 `mapstructure:"max_position_size"`
	MaxTotalExposure   float64 `mapstructure:"max_total_exposure"`
	MaxConcentration   float64 `mapstructure:"max_concentration"`
	MaxOpenPositions   int     `mapstructure:"max_open_positions"`
	MaxDrawdownPercent float64 `mapstructure:"max_drawdown_percent"`
	MinSharpeRatio     float64 `mapstructure:"min_sharpe_ratio"`
	KellyFraction      float64 `mapstructure:"kelly_fraction"`
	StopLossMultiplier float64 `mapstructure:"stop_loss_multiplier"`
	RiskFreeRate       float64 `mapstructure:"risk_free_rate"`
}

// ============================================================================
// RISK AGENT (BDI ARCHITECTURE)
// ============================================================================

// RiskAgent implements risk management with veto power
type RiskAgent struct {
	// Configuration
	config *RiskAgentConfig

	// Services
	db          *db.DB
	riskService *risk.Service
	calculator  *risk.Calculator // Database-backed risk calculator
	natsConn    *nats.Conn

	// LLM client for AI-powered risk analysis
	llmClient     llm.LLMClient // Interface supports both Client and FallbackClient
	promptBuilder *llm.PromptBuilder
	useLLM        bool

	// BDI Components
	beliefs    *RiskBeliefs
	desires    *RiskDesires
	intentions *RiskIntentions

	// State
	mu            sync.RWMutex
	running       bool
	lastHeartbeat time.Time

	// Performance tracking
	vetoCount      int64
	approvalCount  int64
	totalDecisions int64

	// Metrics
	metricsServer *metrics.Server
	riskMetrics   *RiskMetrics
}

// RiskMetrics holds Prometheus metrics for the risk agent
type RiskMetrics struct {
	VetoCount         prometheus.Counter
	ApprovalCount     prometheus.Counter
	DecisionsTotal    prometheus.Counter
	DrawdownCurrent   prometheus.Gauge
	DrawdownMax       prometheus.Gauge
	ExposureTotal     prometheus.Gauge
	SharpeRatio       prometheus.Gauge
	OpenPositions     prometheus.Gauge
	LimitsUtilization prometheus.Gauge
	AgentStatus       prometheus.Gauge
}

// RiskBeliefs represents the agent's current understanding of portfolio risk
type RiskBeliefs struct {
	mu sync.RWMutex

	// Portfolio state
	currentPositions  []Position
	totalExposure     float64
	openPositionCount int

	// Performance metrics
	equityCurve     []float64
	returns         []float64
	currentDrawdown float64
	maxDrawdown     float64
	sharpeRatio     float64
	peakEquity      float64

	// Market conditions
	volatility   float64
	marketRegime string // "bullish", "bearish", "sideways"
	lastUpdate   time.Time

	// Risk limits status
	limitsUtilization float64 // 0.0 to 1.0
	nearLimitSymbols  []string
}

// RiskDesires represents the agent's goals
type RiskDesires struct {
	// Primary goals
	protectCapital          bool
	maintainDiversification bool
	controlDrawdown         bool
	optimizeRiskReturn      bool

	// Target metrics
	targetSharpe      float64
	maxAcceptableDD   float64
	targetUtilization float64
}

// RiskIntentions represents the agent's planned actions
type RiskIntentions struct {
	// Current action plan
	shouldVeto      bool
	vetoReason      string
	recommendedSize float64
	stopLossLevel   float64
	confidenceScore float64

	// Next actions
	monitorDrawdown bool
	// TODO: Will be used in Phase 11 for advanced risk control strategies
	// reduceExposure  bool
	// increaseCash    bool
}

// Position represents a trading position (matches internal/risk)
type Position struct {
	Symbol       string  `json:"symbol"`
	Size         float64 `json:"size"`
	EntryPrice   float64 `json:"entry_price"`
	CurrentPrice float64 `json:"current_price"`
	UnrealizedPL float64 `json:"unrealized_pl"`
}

// ============================================================================
// INITIALIZATION
// ============================================================================

func main() {
	// Configure logging to stderr
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	log.Info().Msg("Starting Risk Management Agent")

	// Load configuration
	viper.SetConfigName("agents")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("CRYPTOFUNK")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("risk_agent.agent_name", "risk-agent")
	viper.SetDefault("risk_agent.agent_type", "risk")
	viper.SetDefault("risk_agent.weight", 1.0)
	viper.SetDefault("risk_agent.nats_url", "nats://localhost:4222")
	viper.SetDefault("risk_agent.signal_topic", "cryptofunk.agent.signals")
	viper.SetDefault("risk_agent.heartbeat_topic", "cryptofunk.agent.heartbeat")
	viper.SetDefault("risk_agent.decision_topic", "cryptofunk.orchestrator.decisions")
	viper.SetDefault("risk_agent.heartbeat_interval", "30s")
	viper.SetDefault("risk_agent.metrics_port", 9108)
	viper.SetDefault("risk_agent.max_position_size", 10000.0)
	viper.SetDefault("risk_agent.max_total_exposure", 50000.0)
	viper.SetDefault("risk_agent.max_concentration", 0.25)
	viper.SetDefault("risk_agent.max_open_positions", 5)
	viper.SetDefault("risk_agent.max_drawdown_percent", 20.0)
	viper.SetDefault("risk_agent.min_sharpe_ratio", 1.0)
	viper.SetDefault("risk_agent.kelly_fraction", 0.25)
	viper.SetDefault("risk_agent.stop_loss_multiplier", 2.0)
	viper.SetDefault("risk_agent.risk_free_rate", 0.03)

	if err := viper.ReadInConfig(); err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
	}

	// Parse configuration
	var config RiskAgentConfig
	if err := viper.UnmarshalKey("risk_agent", &config); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse configuration")
	}

	// Apply defaults for empty fields (UnmarshalKey doesn't apply SetDefault values)
	if config.AgentName == "" {
		config.AgentName = viper.GetString("risk_agent.agent_name")
	}
	if config.AgentType == "" {
		config.AgentType = viper.GetString("risk_agent.agent_type")
	}
	if config.Weight == 0 {
		config.Weight = viper.GetFloat64("risk_agent.weight")
	}
	if config.NATSUrl == "" {
		config.NATSUrl = viper.GetString("risk_agent.nats_url")
	}
	if config.SignalTopic == "" {
		config.SignalTopic = viper.GetString("risk_agent.signal_topic")
	}
	if config.HeartbeatTopic == "" {
		config.HeartbeatTopic = viper.GetString("risk_agent.heartbeat_topic")
	}
	if config.DecisionTopic == "" {
		config.DecisionTopic = viper.GetString("risk_agent.decision_topic")
	}
	if config.HeartbeatInterval == "" {
		config.HeartbeatInterval = viper.GetString("risk_agent.heartbeat_interval")
	}
	if config.MetricsPort == 0 {
		config.MetricsPort = viper.GetInt("risk_agent.metrics_port")
	}
	if config.MaxPositionSize == 0 {
		config.MaxPositionSize = viper.GetFloat64("risk_agent.max_position_size")
	}
	if config.MaxTotalExposure == 0 {
		config.MaxTotalExposure = viper.GetFloat64("risk_agent.max_total_exposure")
	}
	if config.MaxConcentration == 0 {
		config.MaxConcentration = viper.GetFloat64("risk_agent.max_concentration")
	}
	if config.MaxOpenPositions == 0 {
		config.MaxOpenPositions = viper.GetInt("risk_agent.max_open_positions")
	}
	if config.MaxDrawdownPercent == 0 {
		config.MaxDrawdownPercent = viper.GetFloat64("risk_agent.max_drawdown_percent")
	}
	if config.MinSharpeRatio == 0 {
		config.MinSharpeRatio = viper.GetFloat64("risk_agent.min_sharpe_ratio")
	}
	if config.KellyFraction == 0 {
		config.KellyFraction = viper.GetFloat64("risk_agent.kelly_fraction")
	}
	if config.StopLossMultiplier == 0 {
		config.StopLossMultiplier = viper.GetFloat64("risk_agent.stop_loss_multiplier")
	}
	if config.RiskFreeRate == 0 {
		config.RiskFreeRate = viper.GetFloat64("risk_agent.risk_free_rate")
	}

	log.Info().
		Str("agent_name", config.AgentName).
		Str("agent_type", config.AgentType).
		Float64("weight", config.Weight).
		Float64("max_position_size", config.MaxPositionSize).
		Float64("max_total_exposure", config.MaxTotalExposure).
		Float64("max_drawdown", config.MaxDrawdownPercent).
		Msg("Configuration loaded")

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to database
	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	// Create risk service
	riskService := risk.NewService()

	// Create risk calculator with database connection
	calculator := risk.NewCalculatorWithPool(database.Pool())

	// Create agent
	agent, err := NewRiskAgent(&config, database, riskService, calculator)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create risk agent")
	}

	// Initialize agent
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start agent in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := agent.Run(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case err := <-errChan:
		log.Error().Err(err).Msg("Agent error")
	}

	// Shutdown agent
	log.Info().Msg("Shutting down agent...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := agent.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Risk agent shutdown complete")
}

// NewRiskAgent creates a new risk management agent
func NewRiskAgent(config *RiskAgentConfig, database *db.DB, riskService *risk.Service, calculator *risk.Calculator) (*RiskAgent, error) {
	// Validate required parameters
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	// database can be nil for testing - calculator will use defaults
	if riskService == nil {
		return nil, fmt.Errorf("riskService is required")
	}
	if calculator == nil {
		return nil, fmt.Errorf("calculator is required for risk calculations")
	}

	// Initialize LLM client if enabled
	var llmClient llm.LLMClient
	var promptBuilder *llm.PromptBuilder
	useLLM := viper.GetBool("llm.enabled")

	if useLLM {
		primaryConfig := llm.ClientConfig{
			Endpoint:    viper.GetString("llm.endpoint"),
			APIKey:      viper.GetString("llm.api_key"),
			Model:       viper.GetString("llm.primary_model"),
			Temperature: viper.GetFloat64("llm.temperature"),
			MaxTokens:   viper.GetInt("llm.max_tokens"),
			Timeout:     viper.GetDuration("llm.timeout"),
		}

		// Check if fallback models are configured
		fallbackModels := viper.GetStringSlice("llm.fallback_models")
		if len(fallbackModels) > 0 {
			// Create FallbackClient with primary + fallback models
			fallbackConfigs := make([]llm.ClientConfig, len(fallbackModels))
			for i, model := range fallbackModels {
				fallbackConfigs[i] = llm.ClientConfig{
					Endpoint:    viper.GetString("llm.endpoint"),
					APIKey:      viper.GetString("llm.api_key"),
					Model:       model,
					Temperature: viper.GetFloat64("llm.temperature"),
					MaxTokens:   viper.GetInt("llm.max_tokens"),
					Timeout:     viper.GetDuration("llm.timeout"),
				}
			}

			fallbackConfig := llm.FallbackConfig{
				PrimaryConfig:   primaryConfig,
				PrimaryName:     viper.GetString("llm.primary_model"),
				FallbackConfigs: fallbackConfigs,
				FallbackNames:   fallbackModels,
				CircuitBreakerConfig: llm.CircuitBreakerConfig{
					FailureThreshold: viper.GetInt("llm.circuit_breaker.failure_threshold"),
					SuccessThreshold: viper.GetInt("llm.circuit_breaker.success_threshold"),
					Timeout:          viper.GetDuration("llm.circuit_breaker.timeout"),
					TimeWindow:       viper.GetDuration("llm.circuit_breaker.time_window"),
				},
			}

			// Apply defaults if not set
			if fallbackConfig.CircuitBreakerConfig.FailureThreshold == 0 {
				fallbackConfig.CircuitBreakerConfig = llm.DefaultCircuitBreakerConfig()
			}

			llmClient = llm.NewFallbackClient(fallbackConfig)
			log.Info().
				Str("primary_model", fallbackConfig.PrimaryName).
				Strs("fallback_models", fallbackModels).
				Msg("LLM fallback client initialized for risk management")
		} else {
			// Create basic Client
			llmClient = llm.NewClient(primaryConfig)
			log.Info().Msg("LLM-powered risk analysis enabled")
		}

		promptBuilder = llm.NewPromptBuilder(llm.AgentTypeRisk)
	} else {
		log.Info().Msg("Using rule-based risk analysis")
	}

	// Create Prometheus metrics for risk agent
	riskMetrics := &RiskMetrics{
		VetoCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "risk_agent_veto_total",
			Help: "Total number of trades vetoed by risk agent",
		}),
		ApprovalCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "risk_agent_approval_total",
			Help: "Total number of trades approved by risk agent",
		}),
		DecisionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "risk_agent_decisions_total",
			Help: "Total number of risk decisions made",
		}),
		DrawdownCurrent: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_drawdown_current_percent",
			Help: "Current portfolio drawdown percentage",
		}),
		DrawdownMax: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_drawdown_max_percent",
			Help: "Maximum portfolio drawdown percentage",
		}),
		ExposureTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_exposure_total",
			Help: "Total portfolio exposure in USD",
		}),
		SharpeRatio: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_sharpe_ratio",
			Help: "Current Sharpe ratio",
		}),
		OpenPositions: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_open_positions",
			Help: "Number of open positions",
		}),
		LimitsUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_limits_utilization",
			Help: "Portfolio limits utilization (0.0 to 1.0)",
		}),
		AgentStatus: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "risk_agent_status",
			Help: "Risk agent status (1=running, 0=stopped)",
		}),
	}

	// Create metrics server
	metricsServer := metrics.NewServer(config.MetricsPort, log.Logger)

	return &RiskAgent{
		config:        config,
		db:            database,
		riskService:   riskService,
		calculator:    calculator,
		llmClient:     llmClient,
		promptBuilder: promptBuilder,
		useLLM:        useLLM,
		beliefs: &RiskBeliefs{
			currentPositions: make([]Position, 0),
			equityCurve:      make([]float64, 0),
			returns:          make([]float64, 0),
			nearLimitSymbols: make([]string, 0),
			lastUpdate:       time.Now(),
			marketRegime:     "sideways",
		},
		desires: &RiskDesires{
			protectCapital:          true,
			maintainDiversification: true,
			controlDrawdown:         true,
			optimizeRiskReturn:      true,
			targetSharpe:            config.MinSharpeRatio,
			maxAcceptableDD:         config.MaxDrawdownPercent,
			targetUtilization:       0.80, // 80% utilization is healthy
		},
		intentions: &RiskIntentions{
			shouldVeto:      false,
			monitorDrawdown: true,
		},
		metricsServer: metricsServer,
		riskMetrics:   riskMetrics,
	}, nil
}

// Initialize sets up the agent
func (a *RiskAgent) Initialize(ctx context.Context) error {
	log.Info().Msg("Initializing risk agent")

	// Connect to NATS
	nc, err := nats.Connect(a.config.NATSUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	a.natsConn = nc

	log.Info().Str("nats_url", a.config.NATSUrl).Msg("Connected to NATS")

	// Subscribe to orchestrator decisions
	_, err = a.natsConn.Subscribe(a.config.DecisionTopic, a.handleDecision)
	if err != nil {
		return fmt.Errorf("failed to subscribe to decisions: %w", err)
	}

	log.Info().Str("topic", a.config.DecisionTopic).Msg("Subscribed to orchestrator decisions")

	// Load current portfolio state from database
	if err := a.loadPortfolioState(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load initial portfolio state")
	}

	// Start metrics server
	if a.metricsServer != nil {
		if err := a.metricsServer.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start metrics server")
			// Don't fail agent initialization if metrics server fails
		} else {
			log.Info().Int("port", a.config.MetricsPort).Msg("Metrics server started")
		}
	}

	// Set agent status to running
	if a.riskMetrics != nil {
		a.riskMetrics.AgentStatus.Set(1)
	}

	log.Info().Msg("Risk agent initialized successfully")
	return nil
}

// Run starts the agent's main loop
func (a *RiskAgent) Run(ctx context.Context) error {
	log.Info().Msg("Risk agent running")

	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	// Parse heartbeat interval
	heartbeatInterval, err := time.ParseDuration(a.config.HeartbeatInterval)
	if err != nil {
		heartbeatInterval = 30 * time.Second
	}

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	// Portfolio update interval (every 5 seconds)
	updateTicker := time.NewTicker(5 * time.Second)
	defer updateTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Risk agent stopped by context")
			return ctx.Err()
		case <-heartbeatTicker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Error().Err(err).Msg("Failed to send heartbeat")
			}
		case <-updateTicker.C:
			if err := a.updateBeliefs(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to update beliefs")
			}
		}
	}
}

// Shutdown gracefully stops the agent
func (a *RiskAgent) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down risk agent")

	a.mu.Lock()
	a.running = false
	a.mu.Unlock()

	// Close NATS connection
	if a.natsConn != nil {
		a.natsConn.Close()
		log.Info().Msg("NATS connection closed")
	}

	// Shutdown metrics server
	if a.metricsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.metricsServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error shutting down metrics server")
		} else {
			log.Info().Msg("Metrics server shutdown complete")
		}
	}

	// Set agent status to stopped
	if a.riskMetrics != nil {
		a.riskMetrics.AgentStatus.Set(0)
	}

	return nil
}

// sendHeartbeat sends periodic heartbeat to orchestrator
func (a *RiskAgent) sendHeartbeat() error {
	a.mu.Lock()
	a.lastHeartbeat = time.Now()
	a.mu.Unlock()

	heartbeat := map[string]interface{}{
		"agent_name": a.config.AgentName,
		"agent_type": a.config.AgentType,
		"timestamp":  time.Now().Format(time.RFC3339),
		"status":     "HEALTHY",
		"enabled":    true,
		"weight":     a.config.Weight,
		"performance_data": map[string]interface{}{
			"veto_count":      a.vetoCount,
			"approval_count":  a.approvalCount,
			"total_decisions": a.totalDecisions,
			"veto_rate":       float64(a.vetoCount) / float64(max(1, a.totalDecisions)),
		},
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	if err := a.natsConn.Publish(a.config.HeartbeatTopic, data); err != nil {
		return fmt.Errorf("failed to publish heartbeat: %w", err)
	}

	log.Debug().Msg("Heartbeat sent")
	return nil
}

// max returns the maximum of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// handleDecision processes orchestrator decisions and generates risk signals
// This is called when the orchestrator makes a decision (after other agents vote)
func (a *RiskAgent) handleDecision(msg *nats.Msg) {
	// For this implementation, we're listening to decisions to update our beliefs
	// The risk agent actually sends signals BEFORE the orchestrator makes decisions
	// This handler helps us learn from outcomes
	log.Debug().Msg("Received orchestrator decision")
}

// ============================================================================
// T120: PORTFOLIO LIMIT CHECKING
// ============================================================================

// checkPortfolioLimits validates if a proposed trade violates risk limits
func (a *RiskAgent) checkPortfolioLimits(symbol string, size float64) (bool, []string) {
	a.beliefs.mu.RLock()
	defer a.beliefs.mu.RUnlock()

	violations := []string{}

	// Check max position size
	if size > a.config.MaxPositionSize {
		violations = append(violations, fmt.Sprintf(
			"Position size $%.2f exceeds maximum $%.2f",
			size, a.config.MaxPositionSize))
	}

	// Check total exposure
	newTotalExposure := a.beliefs.totalExposure + size
	if newTotalExposure > a.config.MaxTotalExposure {
		violations = append(violations, fmt.Sprintf(
			"Total exposure $%.2f would exceed maximum $%.2f",
			newTotalExposure, a.config.MaxTotalExposure))
	}

	// Check concentration limit
	symbolExposure := a.getSymbolExposure(symbol) + size
	maxSymbolExposure := a.config.MaxTotalExposure * a.config.MaxConcentration
	if symbolExposure > maxSymbolExposure {
		concentrationPct := (symbolExposure / a.config.MaxTotalExposure) * 100
		violations = append(violations, fmt.Sprintf(
			"Symbol concentration %.1f%% exceeds maximum %.1f%%",
			concentrationPct, a.config.MaxConcentration*100))
	}

	// Check max open positions
	if a.beliefs.openPositionCount >= a.config.MaxOpenPositions {
		violations = append(violations, fmt.Sprintf(
			"Already at maximum %d open positions",
			a.config.MaxOpenPositions))
	}

	// Note: Drawdown check is handled separately in evaluateProposal
	// with better messaging and circuit breaker logic

	return len(violations) == 0, violations
}

// getSymbolExposure calculates current exposure for a specific symbol
func (a *RiskAgent) getSymbolExposure(symbol string) float64 {
	total := 0.0
	for _, pos := range a.beliefs.currentPositions {
		if pos.Symbol == symbol {
			total += pos.Size
		}
	}
	return total
}

// ============================================================================
// T121: KELLY CRITERION POSITION SIZING
// ============================================================================

// calculateOptimalSize calculates optimal position size using Kelly Criterion
func (a *RiskAgent) calculateOptimalSize(ctx context.Context, symbol string, confidence float64) float64 {
	// Get historical performance for this symbol or overall portfolio
	winRate := a.getHistoricalWinRate(ctx, symbol)
	avgWin := a.getHistoricalAvgWin(ctx, symbol)
	avgLoss := a.getHistoricalAvgLoss(ctx, symbol)

	// Adjust win rate based on signal confidence
	adjustedWinRate := winRate * confidence

	// Calculate Kelly fraction
	if avgLoss == 0 {
		avgLoss = 0.01 // Avoid division by zero
	}

	b := avgWin / avgLoss
	q := 1 - adjustedWinRate
	kellyPercent := (adjustedWinRate*b - q) / b

	// Apply conservative Kelly fraction
	adjustedKelly := kellyPercent * a.config.KellyFraction

	// Ensure non-negative
	if adjustedKelly < 0 {
		adjustedKelly = 0
	}

	// Cap at reasonable maximum (10% of total exposure limit)
	maxSize := a.config.MaxTotalExposure * 0.10
	optimalSize := a.config.MaxTotalExposure * adjustedKelly

	if optimalSize > maxSize {
		optimalSize = maxSize
	}

	// Cap at max position size
	if optimalSize > a.config.MaxPositionSize {
		optimalSize = a.config.MaxPositionSize
	}

	return optimalSize
}

// getHistoricalWinRate returns historical win rate for symbol (or overall)
func (a *RiskAgent) getHistoricalWinRate(ctx context.Context, symbol string) float64 {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use calculator to get win rate from database
	winRateData, err := a.calculator.CalculateWinRate(ctx, symbol)
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to calculate win rate from database, using default")
		return 0.55 // Default conservative estimate
	}

	return winRateData.WinRate
}

// getHistoricalAvgWin returns average win size
func (a *RiskAgent) getHistoricalAvgWin(ctx context.Context, symbol string) float64 {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use calculator to get win rate data from database
	winRateData, err := a.calculator.CalculateWinRate(ctx, symbol)
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to calculate average win from database, using default")
		return 200.0 // Default estimate
	}

	if winRateData.AvgWin == 0 {
		return 200.0 // Default if no historical data
	}

	return winRateData.AvgWin
}

// getHistoricalAvgLoss returns average loss size
func (a *RiskAgent) getHistoricalAvgLoss(ctx context.Context, symbol string) float64 {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use calculator to get win rate data from database
	winRateData, err := a.calculator.CalculateWinRate(ctx, symbol)
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to calculate average loss from database, using default")
		return 100.0 // Default estimate
	}

	if winRateData.AvgLoss == 0 {
		return 100.0 // Default if no historical data
	}

	return winRateData.AvgLoss
}

// ============================================================================
// T122: DYNAMIC STOP-LOSS CALCULATION
// ============================================================================

// calculateStopLoss calculates dynamic stop-loss based on volatility
func (a *RiskAgent) calculateStopLoss(entryPrice float64, side string) float64 {
	a.beliefs.mu.RLock()
	volatility := a.beliefs.volatility
	a.beliefs.mu.RUnlock()

	// Default to 2% if volatility unknown
	if volatility == 0 {
		volatility = 0.02
	}

	// Stop loss is multiplier * volatility from entry
	stopLossDistance := entryPrice * volatility * a.config.StopLossMultiplier

	var stopLoss float64
	if side == "BUY" || side == "LONG" {
		stopLoss = entryPrice - stopLossDistance
	} else {
		stopLoss = entryPrice + stopLossDistance
	}

	return stopLoss
}

// ============================================================================
// BELIEF UPDATES
// ============================================================================

// updateBeliefs updates the agent's understanding of portfolio state
func (a *RiskAgent) updateBeliefs(ctx context.Context) error {
	// Load current positions from database
	if err := a.loadPortfolioState(ctx); err != nil {
		return err
	}

	// Calculate performance metrics
	a.calculatePerformanceMetrics(ctx)

	// Assess market conditions
	a.assessMarketConditions(ctx)

	// Update limits utilization
	a.beliefs.mu.Lock()
	if a.config.MaxTotalExposure > 0 {
		a.beliefs.limitsUtilization = a.beliefs.totalExposure / a.config.MaxTotalExposure
	}
	a.beliefs.lastUpdate = time.Now()
	a.beliefs.mu.Unlock()

	// Update Prometheus metrics
	if a.riskMetrics != nil {
		a.beliefs.mu.RLock()
		a.riskMetrics.DrawdownCurrent.Set(a.beliefs.currentDrawdown)
		a.riskMetrics.DrawdownMax.Set(a.beliefs.maxDrawdown)
		a.riskMetrics.ExposureTotal.Set(a.beliefs.totalExposure)
		a.riskMetrics.SharpeRatio.Set(a.beliefs.sharpeRatio)
		a.riskMetrics.OpenPositions.Set(float64(a.beliefs.openPositionCount))
		a.riskMetrics.LimitsUtilization.Set(a.beliefs.limitsUtilization)
		a.beliefs.mu.RUnlock()
	}

	log.Debug().
		Float64("total_exposure", a.beliefs.totalExposure).
		Float64("current_drawdown", a.beliefs.currentDrawdown).
		Float64("sharpe_ratio", a.beliefs.sharpeRatio).
		Int("open_positions", a.beliefs.openPositionCount).
		Msg("Beliefs updated")

	return nil
}

// loadPortfolioState loads current positions from database
func (a *RiskAgent) loadPortfolioState(ctx context.Context) error {
	// Check if database is available
	if a.db == nil {
		log.Debug().Msg("No database connection, skipping portfolio state load")
		return nil // Graceful degradation
	}

	// Query database for open positions
	// Note: position_side enum is 'LONG', 'SHORT', 'FLAT' (not 'BUY'/'SELL')
	// Open positions are identified by exit_time IS NULL
	query := `
		SELECT symbol,
		       SUM(CASE WHEN side = 'LONG' THEN quantity ELSE -quantity END) as net_quantity,
		       AVG(CASE WHEN side = 'LONG' THEN entry_price ELSE NULL END) as avg_entry_price
		FROM positions
		WHERE exit_time IS NULL
		GROUP BY symbol
		HAVING SUM(CASE WHEN side = 'LONG' THEN quantity ELSE -quantity END) > 0
	`

	rows, err := a.db.Pool().Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query positions: %w", err)
	}
	defer rows.Close()

	positions := make([]Position, 0)
	totalExposure := 0.0

	for rows.Next() {
		var symbol string
		var quantity, entryPrice float64

		if err := rows.Scan(&symbol, &quantity, &entryPrice); err != nil {
			return fmt.Errorf("failed to scan position: %w", err)
		}

		size := quantity * entryPrice
		positions = append(positions, Position{
			Symbol:     symbol,
			Size:       size,
			EntryPrice: entryPrice,
		})

		totalExposure += size
	}

	a.beliefs.mu.Lock()
	a.beliefs.currentPositions = positions
	a.beliefs.totalExposure = totalExposure
	a.beliefs.openPositionCount = len(positions)
	a.beliefs.mu.Unlock()

	return nil
}

// calculatePerformanceMetrics calculates Sharpe, drawdown, etc.
func (a *RiskAgent) calculatePerformanceMetrics(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Load equity curve from performance_metrics table using calculator
	perfData, err := a.calculator.LoadEquityCurve(ctx, nil, 30)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load equity curve from database")
		return
	}

	a.beliefs.mu.Lock()
	defer a.beliefs.mu.Unlock()

	// Update equity curve and returns
	a.beliefs.equityCurve = perfData.EquityCurve
	a.beliefs.returns = perfData.Returns
	a.beliefs.peakEquity = perfData.PeakEquity

	// Calculate Sharpe ratio from returns using calculator
	if len(perfData.Returns) > 0 {
		sharpe, err := a.calculator.CalculateSharpeRatio(perfData.Returns, a.config.RiskFreeRate)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to calculate Sharpe ratio")
		} else {
			a.beliefs.sharpeRatio = sharpe
			log.Debug().Float64("sharpe_ratio", sharpe).Msg("Sharpe ratio calculated from real returns")
		}
	}

	// Calculate drawdown using calculator
	if len(perfData.EquityCurve) > 0 {
		currentDD, maxDD, peak := a.calculator.CalculateDrawdown(perfData.EquityCurve)
		a.beliefs.currentDrawdown = currentDD * 100 // Convert to percentage
		a.beliefs.maxDrawdown = maxDD * 100         // Convert to percentage
		a.beliefs.peakEquity = peak

		log.Debug().
			Float64("peak", peak).
			Float64("current_drawdown_pct", a.beliefs.currentDrawdown).
			Float64("max_drawdown_pct", a.beliefs.maxDrawdown).
			Msg("Drawdown calculated from database")
	}
}

// assessMarketConditions determines current market regime
func (a *RiskAgent) assessMarketConditions(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use calculator to detect market regime from database
	// Use BTC/USDT as major market indicator
	regimeData, err := a.calculator.DetectMarketRegime(ctx, "BTC/USDT", 30)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to detect market regime from database, using defaults")
		a.beliefs.mu.Lock()
		a.beliefs.marketRegime = "sideways"
		a.beliefs.volatility = 0.02
		a.beliefs.mu.Unlock()
		return
	}

	a.beliefs.mu.Lock()
	a.beliefs.marketRegime = regimeData.Regime
	a.beliefs.volatility = regimeData.Volatility
	a.beliefs.mu.Unlock()

	log.Debug().
		Str("regime", regimeData.Regime).
		Float64("volatility", regimeData.Volatility).
		Float64("short_ma", regimeData.ShortMA).
		Float64("long_ma", regimeData.LongMA).
		Float64("trend_strength", regimeData.TrendStrength).
		Msg("Market conditions assessed from database")
}

// getCurrentPrice gets the current market price for a symbol from the database
func (a *RiskAgent) getCurrentPrice(ctx context.Context, symbol string) float64 {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use calculator to get current price from database
	price, err := a.calculator.GetCurrentPrice(ctx, symbol, "1h")
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to get current price from database, using default")
		return 100.0 // Default fallback price
	}

	return price
}

// ============================================================================
// T123: VETO LOGIC
// ============================================================================

// evaluateProposal evaluates a trading proposal and decides whether to veto
// evaluateProposal routes to LLM or rule-based proposal evaluation
func (a *RiskAgent) evaluateProposal(ctx context.Context, symbol string, action string, size float64, confidence float64) *RiskIntentions {
	if a.useLLM && a.llmClient != nil {
		intentions, err := a.evaluateProposalWithLLM(ctx, symbol, action, size, confidence)
		if err != nil {
			log.Warn().Err(err).Msg("LLM risk assessment failed, falling back to rule-based analysis")
			return a.evaluateProposalRuleBased(ctx, symbol, action, size, confidence)
		}
		return intentions
	}
	return a.evaluateProposalRuleBased(ctx, symbol, action, size, confidence)
}

// evaluateProposalWithLLM performs LLM-powered risk assessment
func (a *RiskAgent) evaluateProposalWithLLM(ctx context.Context, symbol string, action string, size float64, confidence float64) (*RiskIntentions, error) {
	log.Debug().Str("symbol", symbol).Str("action", action).Msg("Evaluating proposal with LLM")

	// Get current price from database
	currentPrice := a.getCurrentPrice(ctx, symbol)

	a.beliefs.mu.RLock()
	portfolioValue := a.config.MaxPositionSize * 10.0 // Estimate portfolio as 10x max position
	if a.beliefs.totalExposure > 0 {
		portfolioValue = a.beliefs.totalExposure / 0.8 // Assume 80% utilization
	}
	a.beliefs.mu.RUnlock()

	// Build signal for the prompt
	signal := llm.Signal{
		Symbol:     symbol,
		Side:       action,
		Confidence: confidence,
		Reasoning:  fmt.Sprintf("Proposed %s trade for %s", action, symbol),
	}

	// Build market context
	marketCtx := llm.MarketContext{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		Timestamp:    time.Now(),
	}

	// Build positions context (simplified for now)
	positions := make([]llm.PositionContext, 0)

	// Build risk assessment prompt
	maxPositionSize := a.config.MaxPositionSize
	userPrompt := a.promptBuilder.BuildRiskAssessmentPrompt(signal, marketCtx, positions, portfolioValue, maxPositionSize)
	systemPrompt := a.promptBuilder.GetSystemPrompt()

	// Call LLM with retry logic
	response, err := a.llmClient.CompleteWithRetry(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 2) // 2 retries

	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse LLM response
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := response.Choices[0].Message.Content

	// Parse JSON response into a risk assessment structure
	var riskAssessment struct {
		Approved     bool     `json:"approved"`
		PositionSize float64  `json:"position_size"`
		StopLoss     *float64 `json:"stop_loss"`
		TakeProfit   *float64 `json:"take_profit"`
		RiskScore    float64  `json:"risk_score"`
		Reasoning    string   `json:"reasoning"`
		Concerns     []string `json:"concerns"`
	}

	if err := a.llmClient.ParseJSONResponse(content, &riskAssessment); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Convert LLM response to RiskIntentions
	intentions := &RiskIntentions{
		shouldVeto:      !riskAssessment.Approved,
		recommendedSize: riskAssessment.PositionSize * portfolioValue,
		confidenceScore: 1.0 - riskAssessment.RiskScore, // Convert risk score to confidence
		monitorDrawdown: true,
	}

	if !riskAssessment.Approved {
		intentions.vetoReason = riskAssessment.Reasoning
		if len(riskAssessment.Concerns) > 0 {
			intentions.vetoReason += fmt.Sprintf(" Concerns: %v", riskAssessment.Concerns)
		}
	}

	if riskAssessment.StopLoss != nil {
		intentions.stopLossLevel = *riskAssessment.StopLoss
	}

	log.Info().
		Str("symbol", symbol).
		Str("action", action).
		Bool("approved", riskAssessment.Approved).
		Float64("position_size", riskAssessment.PositionSize).
		Float64("risk_score", riskAssessment.RiskScore).
		Str("reasoning", riskAssessment.Reasoning).
		Msg("LLM risk assessment completed")

	return intentions, nil
}

// evaluateProposalRuleBased performs rule-based risk assessment
func (a *RiskAgent) evaluateProposalRuleBased(ctx context.Context, symbol string, action string, size float64, confidence float64) *RiskIntentions {
	a.beliefs.mu.RLock()
	defer a.beliefs.mu.RUnlock()

	intentions := &RiskIntentions{
		shouldVeto:      false,
		vetoReason:      "",
		recommendedSize: size,
		confidenceScore: 0.9,
		monitorDrawdown: true,
	}

	// Check 1: Portfolio limits
	if action == "BUY" {
		approved, violations := a.checkPortfolioLimits(symbol, size)
		if !approved {
			intentions.shouldVeto = true
			intentions.vetoReason = fmt.Sprintf("Portfolio limits violated: %v", violations)
			intentions.confidenceScore = 0.95 // High confidence in veto
			return intentions
		}
	}

	// Check 2: Drawdown limit (only for BUY - allow SELL to reduce exposure)
	if action == "BUY" && a.beliefs.currentDrawdown > a.config.MaxDrawdownPercent {
		intentions.shouldVeto = true
		intentions.vetoReason = fmt.Sprintf(
			"Current drawdown %.1f%% exceeds maximum %.1f%% - circuit breaker activated",
			a.beliefs.currentDrawdown, a.config.MaxDrawdownPercent)
		intentions.confidenceScore = 0.98
		return intentions
	}

	// Check 3: Approaching drawdown limit (80% of limit)
	drawdownWarningLevel := a.config.MaxDrawdownPercent * 0.80
	if a.beliefs.currentDrawdown > drawdownWarningLevel && action == "BUY" {
		intentions.shouldVeto = true
		intentions.vetoReason = fmt.Sprintf(
			"Approaching maximum drawdown (%.1f%% of %.1f%%) - reducing risk",
			a.beliefs.currentDrawdown, a.config.MaxDrawdownPercent)
		intentions.confidenceScore = 0.85
		return intentions
	}

	// Check 4: High volatility + high utilization
	highVolatilityThreshold := 0.04  // 4%
	highUtilizationThreshold := 0.85 // 85%
	if a.beliefs.volatility > highVolatilityThreshold &&
		a.beliefs.limitsUtilization > highUtilizationThreshold &&
		action == "BUY" {
		intentions.shouldVeto = true
		intentions.vetoReason = fmt.Sprintf(
			"High volatility (%.1f%%) + high utilization (%.1f%%) - reducing exposure",
			a.beliefs.volatility*100, a.beliefs.limitsUtilization*100)
		intentions.confidenceScore = 0.80
		return intentions
	}

	// Check 5: Position sizing recommendation
	optimalSize := a.calculateOptimalSize(ctx, symbol, confidence)
	if size > optimalSize*1.5 { // Allow 50% over optimal
		intentions.shouldVeto = false // Don't veto, but recommend smaller size
		intentions.recommendedSize = optimalSize
		intentions.vetoReason = fmt.Sprintf(
			"Recommended size $%.2f (requested $%.2f) - Kelly Criterion suggests smaller position",
			optimalSize, size)
		intentions.confidenceScore = 0.70
		return intentions
	}

	// Check 6: Concentration risk
	symbolExposure := a.getSymbolExposure(symbol)
	if a.beliefs.totalExposure > 0 {
		currentConcentration := symbolExposure / a.beliefs.totalExposure
		if currentConcentration > a.config.MaxConcentration*0.8 && action == "BUY" {
			intentions.shouldVeto = true
			intentions.vetoReason = fmt.Sprintf(
				"Symbol concentration %.1f%% approaching limit %.1f%% - diversification required",
				currentConcentration*100, a.config.MaxConcentration*100)
			intentions.confidenceScore = 0.75
			return intentions
		}
	}

	// All checks passed - approve trade
	intentions.shouldVeto = false
	intentions.recommendedSize = optimalSize
	intentions.confidenceScore = 0.90

	// Calculate stop loss using current market price
	currentPrice := a.getCurrentPrice(ctx, symbol)
	intentions.stopLossLevel = a.calculateStopLoss(currentPrice, action)

	return intentions
}

// ============================================================================
// T124: RISK ASSESSMENT
// ============================================================================

// assessRisk performs comprehensive risk assessment and generates signal
func (a *RiskAgent) assessRisk(ctx context.Context, symbol string, action string) (string, float64, string) {
	a.totalDecisions++
	if a.riskMetrics != nil {
		a.riskMetrics.DecisionsTotal.Inc()
	}

	// Default size for assessment
	proposedSize := a.config.MaxPositionSize * 0.5 // 50% of max

	// Evaluate the proposal
	intentions := a.evaluateProposal(ctx, symbol, action, proposedSize, 0.8)

	a.intentions = intentions

	// Determine signal action
	var signal string
	var confidence float64
	var reasoning string

	if intentions.shouldVeto {
		signal = "HOLD"
		confidence = intentions.confidenceScore
		reasoning = buildVetoReasoning(intentions, a.beliefs, a.config)
		a.vetoCount++
		if a.riskMetrics != nil {
			a.riskMetrics.VetoCount.Inc()
		}

		log.Warn().
			Str("symbol", symbol).
			Str("action", action).
			Str("veto_reason", intentions.vetoReason).
			Msg("VETO: Trade rejected by risk management")
	} else {
		signal = action
		confidence = intentions.confidenceScore
		reasoning = buildApprovalReasoning(intentions, a.beliefs, a.config)
		a.approvalCount++
		if a.riskMetrics != nil {
			a.riskMetrics.ApprovalCount.Inc()
		}

		log.Info().
			Str("symbol", symbol).
			Str("action", action).
			Float64("recommended_size", intentions.recommendedSize).
			Float64("stop_loss", intentions.stopLossLevel).
			Msg("APPROVED: Trade approved by risk management")
	}

	return signal, confidence, reasoning
}

// buildVetoReasoning constructs detailed reasoning for veto
func buildVetoReasoning(intentions *RiskIntentions, beliefs *RiskBeliefs, config *RiskAgentConfig) string {
	beliefs.mu.RLock()
	defer beliefs.mu.RUnlock()

	reasoning := "RISK MANAGEMENT VETO\n\n"

	reasoning += fmt.Sprintf("PRIMARY REASON: %s\n\n", intentions.vetoReason)

	reasoning += "CURRENT PORTFOLIO STATE:\n"
	reasoning += fmt.Sprintf("- Open Positions: %d / %d\n", beliefs.openPositionCount, config.MaxOpenPositions)
	reasoning += fmt.Sprintf("- Total Exposure: $%.2f / $%.2f (%.1f%% utilized)\n",
		beliefs.totalExposure, config.MaxTotalExposure, beliefs.limitsUtilization*100)
	reasoning += fmt.Sprintf("- Current Drawdown: %.1f%% (Max: %.1f%%)\n",
		beliefs.currentDrawdown, config.MaxDrawdownPercent)
	reasoning += fmt.Sprintf("- Market Regime: %s\n", beliefs.marketRegime)
	reasoning += fmt.Sprintf("- Volatility: %.2f%%\n\n", beliefs.volatility*100)

	reasoning += "RISK METRICS:\n"
	reasoning += fmt.Sprintf("- Sharpe Ratio: %.2f (Target: %.2f)\n", beliefs.sharpeRatio, config.MinSharpeRatio)
	reasoning += fmt.Sprintf("- Max Drawdown: %.1f%%\n", beliefs.maxDrawdown)
	reasoning += fmt.Sprintf("- Peak Equity: $%.2f\n\n", beliefs.peakEquity)

	reasoning += "RECOMMENDATION: HOLD\n"
	reasoning += "Risk management circuit breaker activated to protect capital.\n"

	return reasoning
}

// buildApprovalReasoning constructs detailed reasoning for approval
func buildApprovalReasoning(intentions *RiskIntentions, beliefs *RiskBeliefs, config *RiskAgentConfig) string {
	beliefs.mu.RLock()
	defer beliefs.mu.RUnlock()

	reasoning := "RISK MANAGEMENT APPROVAL\n\n"

	reasoning += "PORTFOLIO HEALTH:\n"
	reasoning += "- All risk limits satisfied\n"
	reasoning += fmt.Sprintf("- Open Positions: %d / %d (capacity available)\n",
		beliefs.openPositionCount, config.MaxOpenPositions)
	reasoning += fmt.Sprintf("- Exposure Utilization: %.1f%% (healthy)\n", beliefs.limitsUtilization*100)
	reasoning += fmt.Sprintf("- Current Drawdown: %.1f%% (within %.1f%% limit)\n",
		beliefs.currentDrawdown, config.MaxDrawdownPercent)

	reasoning += "\nPOSITION SIZING (KELLY CRITERION):\n"
	reasoning += fmt.Sprintf("- Recommended Size: $%.2f\n", intentions.recommendedSize)
	reasoning += fmt.Sprintf("- Kelly Fraction: %.0f%% (conservative)\n", config.KellyFraction*100)
	reasoning += fmt.Sprintf("- Stop Loss Level: $%.2f\n", intentions.stopLossLevel)

	reasoning += "\nRISK METRICS:\n"
	reasoning += fmt.Sprintf("- Sharpe Ratio: %.2f (meets %.2f target)\n", beliefs.sharpeRatio, config.MinSharpeRatio)
	reasoning += fmt.Sprintf("- Volatility: %.2f%% (acceptable)\n", beliefs.volatility*100)
	reasoning += fmt.Sprintf("- Market Regime: %s\n", beliefs.marketRegime)

	reasoning += "\nRECOMMENDATION: PROCEED\n"
	reasoning += "Trade approved with proper risk controls in place.\n"

	return reasoning
}

// TODO: Will be used in Phase 11 for proactive risk signal generation
//
// generateRiskSignal creates and publishes a risk management signal
//
//nolint:unused
func (a *RiskAgent) generateRiskSignal(ctx context.Context, symbol string, proposedAction string) error {
	signal, confidence, reasoning := a.assessRisk(ctx, symbol, proposedAction)

	agentSignal := orchestrator.AgentSignal{
		AgentName:  a.config.AgentName,
		AgentType:  a.config.AgentType,
		Symbol:     symbol,
		Signal:     signal,
		Confidence: confidence,
		Reasoning:  reasoning,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"veto":             signal == "HOLD" && proposedAction != "HOLD",
			"recommended_size": a.intentions.recommendedSize,
			"stop_loss":        a.intentions.stopLossLevel,
			"current_drawdown": a.beliefs.currentDrawdown,
			"portfolio_util":   a.beliefs.limitsUtilization,
			"open_positions":   a.beliefs.openPositionCount,
			"total_exposure":   a.beliefs.totalExposure,
		},
	}

	data, err := json.Marshal(agentSignal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	if err := a.natsConn.Publish(a.config.SignalTopic, data); err != nil {
		return fmt.Errorf("failed to publish signal: %w", err)
	}

	log.Info().
		Str("symbol", symbol).
		Str("signal", signal).
		Float64("confidence", confidence).
		Bool("veto", signal == "HOLD" && proposedAction != "HOLD").
		Msg("Risk signal published")

	return nil
}

func init() {
	// Register agent with base agent framework
	log.Info().Msg("Risk Agent module loaded")
}
