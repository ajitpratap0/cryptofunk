// Package orchestrator coordinates multiple trading agents via weighted voting and consensus
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

// AgentSignal represents a signal received from an agent
type AgentSignal struct {
	AgentName  string                 `json:"agent_name"`
	AgentType  string                 `json:"agent_type"`
	Symbol     string                 `json:"symbol"`
	Signal     string                 `json:"signal"` // BUY, SELL, HOLD
	Confidence float64                `json:"confidence"`
	Reasoning  string                 `json:"reasoning"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AgentSession tracks an active agent connection
type AgentSession struct {
	Name            string
	Type            string
	Enabled         bool
	Weight          float64 // Voting weight (0.0-1.0)
	LastHeartbeat   time.Time
	LastSignal      time.Time
	SignalCount     int64
	ErrorCount      int64
	HealthStatus    string // HEALTHY, DEGRADED, UNHEALTHY
	PerformanceData map[string]interface{}
}

// DecisionContext holds information for making a trading decision
type DecisionContext struct {
	Symbol        string
	Signals       []*AgentSignal
	Timestamp     time.Time
	MinConsensus  float64 // Minimum agreement threshold (0.0-1.0)
	MinConfidence float64 // Minimum confidence threshold (0.0-1.0)
}

// TradingDecision represents the final orchestrator decision
type TradingDecision struct {
	Symbol              string                 `json:"symbol"`
	Action              string                 `json:"action"` // BUY, SELL, HOLD
	Confidence          float64                `json:"confidence"`
	Consensus           float64                `json:"consensus"` // 0.0-1.0
	ParticipatingAgents int                    `json:"participating_agents"`
	VotingResults       map[string]float64     `json:"voting_results"` // action -> weighted score
	Reasoning           string                 `json:"reasoning"`
	Timestamp           time.Time              `json:"timestamp"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// OrchestratorConfig holds orchestrator configuration
type OrchestratorConfig struct {
	Name                string        `json:"name" yaml:"name"`
	NATSUrl             string        `json:"nats_url" yaml:"nats_url"`
	SignalTopic         string        `json:"signal_topic" yaml:"signal_topic"`       // Topic to subscribe for agent signals
	DecisionTopic       string        `json:"decision_topic" yaml:"decision_topic"`   // Topic to publish decisions
	HeartbeatTopic      string        `json:"heartbeat_topic" yaml:"heartbeat_topic"` // Topic for agent heartbeats
	StepInterval        time.Duration `json:"step_interval" yaml:"step_interval"`     // Decision-making interval
	MinConsensus        float64       `json:"min_consensus" yaml:"min_consensus"`     // Minimum consensus threshold
	MinConfidence       float64       `json:"min_confidence" yaml:"min_confidence"`   // Minimum confidence threshold
	MaxSignalAge        time.Duration `json:"max_signal_age" yaml:"max_signal_age"`   // Discard signals older than this
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
}

// OrchestratorMetrics holds Prometheus metrics for orchestrator
type OrchestratorMetrics struct {
	DecisionsTotal   prometheus.Counter
	DecisionDuration prometheus.Histogram
	SignalsReceived  prometheus.Counter
	SignalsProcessed prometheus.Counter
	SignalsDropped   prometheus.Counter
	ActiveAgents     prometheus.Gauge
	ConsensusScore   prometheus.Histogram
	VotingDuration   prometheus.Histogram
}

// Global metrics instance (singleton pattern to avoid Prometheus registration conflicts)
var (
	orchestratorMetricsInstance *OrchestratorMetrics
	orchestratorMetricsOnce     sync.Once
)

// getOrCreateOrchestratorMetrics returns the singleton metrics instance
// Uses sync.Once to ensure metrics are registered only once globally
func getOrCreateOrchestratorMetrics() *OrchestratorMetrics {
	orchestratorMetricsOnce.Do(func() {
		orchestratorMetricsInstance = &OrchestratorMetrics{
			DecisionsTotal: promauto.NewCounter(prometheus.CounterOpts{
				Name: "orchestrator_decisions_total",
				Help: "Total number of trading decisions made",
			}),
			DecisionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
				Name:    "orchestrator_decision_duration_seconds",
				Help:    "Duration of decision-making process",
				Buckets: prometheus.DefBuckets,
			}),
			SignalsReceived: promauto.NewCounter(prometheus.CounterOpts{
				Name: "orchestrator_signals_received_total",
				Help: "Total number of agent signals received",
			}),
			SignalsProcessed: promauto.NewCounter(prometheus.CounterOpts{
				Name: "orchestrator_signals_processed_total",
				Help: "Total number of agent signals processed",
			}),
			SignalsDropped: promauto.NewCounter(prometheus.CounterOpts{
				Name: "orchestrator_signals_dropped_total",
				Help: "Total number of agent signals dropped (expired or invalid)",
			}),
			ActiveAgents: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "orchestrator_active_agents",
				Help: "Number of currently active agents",
			}),
			ConsensusScore: promauto.NewHistogram(prometheus.HistogramOpts{
				Name:    "orchestrator_consensus_score",
				Help:    "Consensus score for decisions",
				Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			}),
			VotingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
				Name:    "orchestrator_voting_duration_seconds",
				Help:    "Duration of weighted voting calculation",
				Buckets: prometheus.DefBuckets,
			}),
		}
	})
	return orchestratorMetricsInstance
}

// Orchestrator coordinates multiple trading agents
type Orchestrator struct {
	// Configuration
	config *OrchestratorConfig
	log    zerolog.Logger

	// Agent Registry
	agents      map[string]*AgentSession // agent_name -> session
	agentsMutex sync.RWMutex

	// NATS Connection
	natsConn     *nats.Conn
	signalSub    *nats.Subscription
	heartbeatSub *nats.Subscription

	// Signal Buffer (recent signals for decision making)
	signalBuffer      []*AgentSignal
	signalBufferMutex sync.RWMutex

	// State
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Trading control
	paused      bool
	pausedMutex sync.RWMutex

	// Metrics
	metrics       *OrchestratorMetrics
	metricsServer *metrics.Server
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config *OrchestratorConfig, log zerolog.Logger, metricsPort int) (*Orchestrator, error) {
	// Create metrics (singleton pattern to avoid Prometheus registration conflicts)
	orchestratorMetrics := getOrCreateOrchestratorMetrics()

	// Create logger for orchestrator
	orchestratorLog := log.With().Str("component", "orchestrator").Logger()

	// Create metrics server
	metricsServer := metrics.NewServer(metricsPort, orchestratorLog)

	// Note: NATS connection is deferred to Initialize() to allow unit tests
	// to create orchestrator without requiring NATS server

	return &Orchestrator{
		config:        config,
		log:           orchestratorLog,
		agents:        make(map[string]*AgentSession),
		natsConn:      nil, // Will be set in Initialize()
		signalBuffer:  make([]*AgentSignal, 0),
		metrics:       orchestratorMetrics,
		metricsServer: metricsServer,
	}, nil
}

// Initialize sets up the orchestrator
func (o *Orchestrator) Initialize(ctx context.Context) error {
	o.log.Info().Msg("Initializing orchestrator")

	// Create cancellable context
	o.ctx, o.cancel = context.WithCancel(ctx)

	// Connect to NATS (if not already connected)
	if o.natsConn == nil {
		nc, err := nats.Connect(o.config.NATSUrl)
		if err != nil {
			return fmt.Errorf("failed to connect to NATS: %w", err)
		}
		o.natsConn = nc
		o.log.Info().Str("nats_url", o.config.NATSUrl).Msg("Connected to NATS")
	}

	// Subscribe to agent signals
	signalSub, err := o.natsConn.Subscribe(o.config.SignalTopic, o.handleSignal)
	if err != nil {
		return fmt.Errorf("failed to subscribe to signal topic: %w", err)
	}
	o.signalSub = signalSub

	o.log.Info().Str("topic", o.config.SignalTopic).Msg("Subscribed to agent signals")

	// Subscribe to agent heartbeats
	heartbeatSub, err := o.natsConn.Subscribe(o.config.HeartbeatTopic, o.handleHeartbeat)
	if err != nil {
		return fmt.Errorf("failed to subscribe to heartbeat topic: %w", err)
	}
	o.heartbeatSub = heartbeatSub

	o.log.Info().Str("topic", o.config.HeartbeatTopic).Msg("Subscribed to agent heartbeats")

	// Start metrics server
	if o.metricsServer != nil {
		if err := o.metricsServer.Start(); err != nil {
			o.log.Error().Err(err).Msg("Failed to start metrics server")
		} else {
			o.log.Info().Msg("Metrics server started successfully")

			// Register control endpoints
			o.metricsServer.RegisterHandler("/pause", o.handlePauseRequest)
			o.metricsServer.RegisterHandler("/resume", o.handleResumeRequest)
			o.metricsServer.RegisterHandler("/status", o.handleStatusRequest)
			o.log.Info().Msg("Control endpoints registered")
		}
	}

	// Start health check routine
	o.wg.Add(1)
	go o.healthCheckLoop()

	o.log.Info().Msg("Orchestrator initialized successfully")
	return nil
}

// Run starts the orchestrator's main decision loop
func (o *Orchestrator) Run(ctx context.Context) error {
	o.log.Info().Msg("Starting orchestrator run loop")

	ticker := time.NewTicker(o.config.StepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.log.Info().Msg("Orchestrator run loop stopped by context")
			return ctx.Err()
		case <-o.ctx.Done():
			o.log.Info().Msg("Orchestrator run loop stopped by internal context")
			return o.ctx.Err()
		case <-ticker.C:
			if err := o.makeDecision(ctx); err != nil {
				o.log.Error().Err(err).Msg("Error making decision")
				// Continue running despite errors
			}
		}
	}
}

// handleSignal processes incoming agent signals
func (o *Orchestrator) handleSignal(msg *nats.Msg) {
	o.metrics.SignalsReceived.Inc()

	var signal AgentSignal
	if err := json.Unmarshal(msg.Data, &signal); err != nil {
		o.log.Error().Err(err).Msg("Failed to unmarshal agent signal")
		o.metrics.SignalsDropped.Inc()
		return
	}

	// Validate signal age
	if time.Since(signal.Timestamp) > o.config.MaxSignalAge {
		o.log.Warn().
			Str("agent", signal.AgentName).
			Dur("age", time.Since(signal.Timestamp)).
			Msg("Signal too old, dropping")
		o.metrics.SignalsDropped.Inc()
		return
	}

	// Update agent session
	o.updateAgentSession(signal.AgentName, signal.AgentType, &signal)

	// Add to signal buffer
	o.signalBufferMutex.Lock()
	o.signalBuffer = append(o.signalBuffer, &signal)
	o.signalBufferMutex.Unlock()

	o.metrics.SignalsProcessed.Inc()

	o.log.Debug().
		Str("agent", signal.AgentName).
		Str("symbol", signal.Symbol).
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Msg("Received agent signal")
}

// handleHeartbeat processes agent heartbeat messages
func (o *Orchestrator) handleHeartbeat(msg *nats.Msg) {
	var heartbeat struct {
		AgentName string    `json:"agent_name"`
		AgentType string    `json:"agent_type"`
		Timestamp time.Time `json:"timestamp"`
		Status    string    `json:"status"`
	}

	if err := json.Unmarshal(msg.Data, &heartbeat); err != nil {
		o.log.Error().Err(err).Msg("Failed to unmarshal heartbeat")
		return
	}

	o.agentsMutex.Lock()
	if session, exists := o.agents[heartbeat.AgentName]; exists {
		session.LastHeartbeat = heartbeat.Timestamp
		session.HealthStatus = heartbeat.Status
	} else {
		// Register new agent
		o.agents[heartbeat.AgentName] = &AgentSession{
			Name:            heartbeat.AgentName,
			Type:            heartbeat.AgentType,
			Enabled:         true,
			Weight:          o.getDefaultWeight(heartbeat.AgentType),
			LastHeartbeat:   heartbeat.Timestamp,
			HealthStatus:    heartbeat.Status,
			PerformanceData: make(map[string]interface{}),
		}
		o.log.Info().
			Str("agent", heartbeat.AgentName).
			Str("type", heartbeat.AgentType).
			Msg("Registered new agent")
	}
	o.agentsMutex.Unlock()

	o.updateActiveAgentsMetric()
}

// updateAgentSession updates or creates an agent session from a signal
func (o *Orchestrator) updateAgentSession(name, agentType string, signal *AgentSignal) {
	o.agentsMutex.Lock()
	defer o.agentsMutex.Unlock()

	if session, exists := o.agents[name]; exists {
		session.LastSignal = signal.Timestamp
		session.SignalCount++
	} else {
		// Register new agent from signal
		o.agents[name] = &AgentSession{
			Name:            name,
			Type:            agentType,
			Enabled:         true,
			Weight:          o.getDefaultWeight(agentType),
			LastSignal:      signal.Timestamp,
			SignalCount:     1,
			HealthStatus:    "UNKNOWN",
			PerformanceData: make(map[string]interface{}),
		}
		o.log.Info().
			Str("agent", name).
			Str("type", agentType).
			Msg("Registered new agent from signal")
	}

	o.updateActiveAgentsMetricLocked()
}

// getDefaultWeight returns the default voting weight for an agent type
func (o *Orchestrator) getDefaultWeight(agentType string) float64 {
	// Default weights based on agent type
	// These can be overridden by configuration or learned over time
	weights := map[string]float64{
		"technical": 0.25, // Technical analysis agents
		"orderbook": 0.20, // Order book analysis agents
		"sentiment": 0.15, // Sentiment analysis agents
		"trend":     0.30, // Trend following strategy agents
		"reversion": 0.25, // Mean reversion strategy agents
		"arbitrage": 0.20, // Arbitrage strategy agents
		"risk":      1.00, // Risk agent has veto power
	}

	if weight, exists := weights[agentType]; exists {
		return weight
	}
	return 0.20 // Default weight for unknown agent types
}

// makeDecision performs the decision-making cycle
func (o *Orchestrator) makeDecision(ctx context.Context) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		o.metrics.DecisionDuration.Observe(duration.Seconds())
	}()

	// Check if trading is paused
	o.pausedMutex.RLock()
	isPaused := o.paused
	o.pausedMutex.RUnlock()

	if isPaused {
		o.log.Debug().Msg("Trading is paused, skipping decision making")
		return nil
	}

	// Get recent signals grouped by symbol
	symbolSignals := o.getRecentSignalsBySymbol()

	// Make decision for each symbol with signals
	for symbol, signals := range symbolSignals {
		if len(signals) == 0 {
			continue
		}

		decision := o.calculateDecision(&DecisionContext{
			Symbol:        symbol,
			Signals:       signals,
			Timestamp:     time.Now(),
			MinConsensus:  o.config.MinConsensus,
			MinConfidence: o.config.MinConfidence,
		})

		// Publish all decisions including HOLD (needed for monitoring and testing)
		o.publishDecision(decision)
		o.metrics.DecisionsTotal.Inc()
		o.metrics.ConsensusScore.Observe(decision.Consensus)

		o.log.Info().
			Str("symbol", decision.Symbol).
			Str("action", decision.Action).
			Float64("confidence", decision.Confidence).
			Float64("consensus", decision.Consensus).
			Int("agents", decision.ParticipatingAgents).
			Msg("Trading decision made")
	}

	// Clean old signals from buffer
	o.cleanOldSignals()

	return nil
}

// getRecentSignalsBySymbol groups recent signals by trading symbol
func (o *Orchestrator) getRecentSignalsBySymbol() map[string][]*AgentSignal {
	o.signalBufferMutex.RLock()
	defer o.signalBufferMutex.RUnlock()

	result := make(map[string][]*AgentSignal)
	now := time.Now()

	for _, signal := range o.signalBuffer {
		// Only include signals within the decision window
		if now.Sub(signal.Timestamp) <= o.config.StepInterval {
			result[signal.Symbol] = append(result[signal.Symbol], signal)
		}
	}

	return result
}

// calculateDecision implements weighted voting and consensus logic
func (o *Orchestrator) calculateDecision(ctx *DecisionContext) *TradingDecision {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		o.metrics.VotingDuration.Observe(duration.Seconds())
	}()

	// Initialize voting scores
	votingScores := map[string]float64{
		"BUY":  0.0,
		"SELL": 0.0,
		"HOLD": 0.0,
	}

	totalWeight := 0.0
	participatingAgents := 0
	var reasoning []string

	o.agentsMutex.RLock()
	for _, signal := range ctx.Signals {
		// Get agent session for weight
		session, exists := o.agents[signal.AgentName]
		if !exists || !session.Enabled {
			continue
		}

		// Calculate weighted vote
		weight := session.Weight
		confidence := signal.Confidence
		vote := weight * confidence

		votingScores[signal.Signal] += vote
		totalWeight += weight

		participatingAgents++
		reasoning = append(reasoning, fmt.Sprintf("%s(%s): %.2f confidence",
			signal.AgentName, signal.Signal, signal.Confidence))
	}
	o.agentsMutex.RUnlock()

	// Find winning action
	maxScore := 0.0
	winningAction := "HOLD"
	for action, score := range votingScores {
		if score > maxScore {
			maxScore = score
			winningAction = action
		}
	}

	// Calculate consensus (agreement level)
	consensus := 0.0
	if totalWeight > 0 {
		consensus = maxScore / totalWeight
	}

	// Calculate final confidence
	confidence := maxScore / totalWeight

	// Check thresholds
	if consensus < ctx.MinConsensus || confidence < ctx.MinConfidence {
		winningAction = "HOLD"
		reasoning = append(reasoning, fmt.Sprintf(
			"Insufficient consensus (%.2f < %.2f) or confidence (%.2f < %.2f)",
			consensus, ctx.MinConsensus, confidence, ctx.MinConfidence))
	}

	return &TradingDecision{
		Symbol:              ctx.Symbol,
		Action:              winningAction,
		Confidence:          confidence,
		Consensus:           consensus,
		ParticipatingAgents: participatingAgents,
		VotingResults:       votingScores,
		Reasoning:           fmt.Sprintf("Weighted voting: %v", reasoning),
		Timestamp:           ctx.Timestamp,
	}
}

// publishDecision publishes a trading decision to NATS
func (o *Orchestrator) publishDecision(decision *TradingDecision) error {
	data, err := json.Marshal(decision)
	if err != nil {
		return fmt.Errorf("failed to marshal decision: %w", err)
	}

	if err := o.natsConn.Publish(o.config.DecisionTopic, data); err != nil {
		return fmt.Errorf("failed to publish decision: %w", err)
	}

	return nil
}

// cleanOldSignals removes expired signals from buffer
func (o *Orchestrator) cleanOldSignals() {
	o.signalBufferMutex.Lock()
	defer o.signalBufferMutex.Unlock()

	now := time.Now()
	validSignals := make([]*AgentSignal, 0)

	for _, signal := range o.signalBuffer {
		if now.Sub(signal.Timestamp) <= o.config.MaxSignalAge {
			validSignals = append(validSignals, signal)
		}
	}

	o.signalBuffer = validSignals
}

// healthCheckLoop periodically checks agent health
func (o *Orchestrator) healthCheckLoop() {
	defer o.wg.Done()

	ticker := time.NewTicker(o.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.checkAgentHealth()
		}
	}
}

// checkAgentHealth evaluates agent health status
func (o *Orchestrator) checkAgentHealth() {
	o.agentsMutex.Lock()
	defer o.agentsMutex.Unlock()

	now := time.Now()
	healthyCount := 0

	for _, session := range o.agents {
		timeSinceHeartbeat := now.Sub(session.LastHeartbeat)
		timeSinceSignal := now.Sub(session.LastSignal)

		// Determine health status
		previousStatus := session.HealthStatus

		if timeSinceHeartbeat > 5*time.Minute {
			session.HealthStatus = "UNHEALTHY"
			session.Enabled = false
		} else if timeSinceSignal > 10*time.Minute {
			session.HealthStatus = "DEGRADED"
		} else {
			session.HealthStatus = "HEALTHY"
			healthyCount++
		}

		// Log status changes
		if session.HealthStatus != previousStatus {
			o.log.Warn().
				Str("agent", session.Name).
				Str("old_status", previousStatus).
				Str("new_status", session.HealthStatus).
				Dur("since_heartbeat", timeSinceHeartbeat).
				Dur("since_signal", timeSinceSignal).
				Msg("Agent health status changed")
		}
	}

	o.updateActiveAgentsMetricLocked()
}

// updateActiveAgentsMetric updates the active agents gauge
func (o *Orchestrator) updateActiveAgentsMetric() {
	o.agentsMutex.RLock()
	defer o.agentsMutex.RUnlock()

	activeCount := 0
	for _, session := range o.agents {
		if session.Enabled && session.HealthStatus == "HEALTHY" {
			activeCount++
		}
	}

	o.metrics.ActiveAgents.Set(float64(activeCount))
}

// updateActiveAgentsMetricLocked updates the active agents gauge
// ASSUMES: caller already holds o.agentsMutex lock (read or write)
func (o *Orchestrator) updateActiveAgentsMetricLocked() {
	activeCount := 0
	for _, session := range o.agents {
		if session.Enabled && session.HealthStatus == "HEALTHY" {
			activeCount++
		}
	}

	o.metrics.ActiveAgents.Set(float64(activeCount))
}

// Pause pauses all trading decision-making
func (o *Orchestrator) Pause() error {
	o.pausedMutex.Lock()
	defer o.pausedMutex.Unlock()

	if o.paused {
		return fmt.Errorf("trading is already paused")
	}

	o.paused = true
	o.log.Info().Msg("Trading paused")

	// Broadcast pause event to all agents via NATS
	if o.natsConn != nil {
		pauseEvent := map[string]interface{}{
			"event":     "trading_paused",
			"timestamp": time.Now(),
			"reason":    "manual_pause",
		}

		data, err := json.Marshal(pauseEvent)
		if err != nil {
			o.log.Error().Err(err).Msg("Failed to marshal pause event")
			return fmt.Errorf("failed to marshal pause event: %w", err)
		}

		// Publish to control topic
		topic := "cryptofunk.orchestrator.control"
		if err := o.natsConn.Publish(topic, data); err != nil {
			o.log.Error().Err(err).Str("topic", topic).Msg("Failed to publish pause event")
			return fmt.Errorf("failed to publish pause event: %w", err)
		}

		o.log.Info().Str("topic", topic).Msg("Pause event broadcast to agents")
	}

	return nil
}

// Resume resumes trading decision-making
func (o *Orchestrator) Resume() error {
	o.pausedMutex.Lock()
	defer o.pausedMutex.Unlock()

	if !o.paused {
		return fmt.Errorf("trading is not paused")
	}

	o.paused = false
	o.log.Info().Msg("Trading resumed")

	// Broadcast resume event to all agents via NATS
	if o.natsConn != nil {
		resumeEvent := map[string]interface{}{
			"event":     "trading_resumed",
			"timestamp": time.Now(),
		}

		data, err := json.Marshal(resumeEvent)
		if err != nil {
			o.log.Error().Err(err).Msg("Failed to marshal resume event")
			return fmt.Errorf("failed to marshal resume event: %w", err)
		}

		// Publish to control topic
		topic := "cryptofunk.orchestrator.control"
		if err := o.natsConn.Publish(topic, data); err != nil {
			o.log.Error().Err(err).Str("topic", topic).Msg("Failed to publish resume event")
			return fmt.Errorf("failed to publish resume event: %w", err)
		}

		o.log.Info().Str("topic", topic).Msg("Resume event broadcast to agents")
	}

	return nil
}

// IsPaused returns whether trading is currently paused
func (o *Orchestrator) IsPaused() bool {
	o.pausedMutex.RLock()
	defer o.pausedMutex.RUnlock()
	return o.paused
}

// HTTP handlers for control endpoints

func (o *Orchestrator) handlePauseRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := o.Pause(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Trading paused successfully",
		"paused":    true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"success":   true,
	})
}

func (o *Orchestrator) handleResumeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := o.Resume(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Trading resumed successfully",
		"paused":    false,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"success":   true,
	})
}

func (o *Orchestrator) handleStatusRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	o.pausedMutex.RLock()
	isPaused := o.paused
	o.pausedMutex.RUnlock()

	o.agentsMutex.RLock()
	activeAgents := len(o.agents)
	o.agentsMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"paused":        isPaused,
		"active_agents": activeAgents,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

// Shutdown gracefully stops the orchestrator
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	o.log.Info().Msg("Shutting down orchestrator")

	// Cancel internal context
	if o.cancel != nil {
		o.cancel()
	}

	// Unsubscribe from NATS
	if o.signalSub != nil {
		if err := o.signalSub.Unsubscribe(); err != nil {
			o.log.Error().Err(err).Msg("Error unsubscribing from signals")
		}
	}
	if o.heartbeatSub != nil {
		if err := o.heartbeatSub.Unsubscribe(); err != nil {
			o.log.Error().Err(err).Msg("Error unsubscribing from heartbeats")
		}
	}

	// Close NATS connection
	if o.natsConn != nil {
		o.natsConn.Close()
		o.log.Info().Msg("NATS connection closed")
	}

	// Shutdown metrics server
	if o.metricsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := o.metricsServer.Shutdown(shutdownCtx); err != nil {
			o.log.Error().Err(err).Msg("Error shutting down metrics server")
		} else {
			o.log.Info().Msg("Metrics server shutdown successfully")
		}
	}

	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		o.log.Info().Msg("Orchestrator shutdown complete")
	case <-ctx.Done():
		o.log.Warn().Msg("Orchestrator shutdown timeout")
		return ctx.Err()
	}

	return nil
}

// GetAgentSessions returns a copy of current agent sessions (for monitoring)
func (o *Orchestrator) GetAgentSessions() map[string]*AgentSession {
	o.agentsMutex.RLock()
	defer o.agentsMutex.RUnlock()

	sessions := make(map[string]*AgentSession)
	for name, session := range o.agents {
		// Create a copy to avoid race conditions
		sessionCopy := *session
		sessions[name] = &sessionCopy
	}

	return sessions
}
