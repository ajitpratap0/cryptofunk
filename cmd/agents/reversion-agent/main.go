// Mean Reversion Agent
// Generates trading signals when price deviates from mean (Bollinger Bands + RSI extremes)
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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// ============================================================================
// BELIEF SYSTEM (BDI ARCHITECTURE)
// ============================================================================

// Belief represents a single belief in the agent's belief base
type Belief struct {
	Key        string      `json:"key"`        // Belief identifier
	Value      interface{} `json:"value"`      // Belief value (flexible type)
	Confidence float64     `json:"confidence"` // Confidence level (0.0-1.0)
	Timestamp  time.Time   `json:"timestamp"`  // Last updated
	Source     string      `json:"source"`     // Source of belief (RSI, Bollinger, market_data, etc.)
}

// BeliefBase manages the agent's beliefs with thread-safe operations
type BeliefBase struct {
	beliefs map[string]*Belief
	mutex   sync.RWMutex
}

// NewBeliefBase creates a new belief base
func NewBeliefBase() *BeliefBase {
	return &BeliefBase{
		beliefs: make(map[string]*Belief),
	}
}

// UpdateBelief creates or updates a belief
func (bb *BeliefBase) UpdateBelief(key string, value interface{}, confidence float64, source string) {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	bb.beliefs[key] = &Belief{
		Key:        key,
		Value:      value,
		Confidence: confidence,
		Timestamp:  time.Now(),
		Source:     source,
	}
}

// GetBelief retrieves a single belief
func (bb *BeliefBase) GetBelief(key string) (*Belief, bool) {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	belief, exists := bb.beliefs[key]
	return belief, exists
}

// GetAllBeliefs returns a copy of all beliefs
func (bb *BeliefBase) GetAllBeliefs() map[string]*Belief {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	// Return a copy to avoid race conditions
	copy := make(map[string]*Belief, len(bb.beliefs))
	for k, v := range bb.beliefs {
		belief := *v // Copy the belief
		copy[k] = &belief
	}
	return copy
}

// GetConfidence calculates overall confidence (average of all beliefs)
func (bb *BeliefBase) GetConfidence() float64 {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	if len(bb.beliefs) == 0 {
		return 0.0
	}

	total := 0.0
	for _, belief := range bb.beliefs {
		total += belief.Confidence
	}
	return total / float64(len(bb.beliefs))
}

// ============================================================================
// MEAN REVERSION AGENT
// ============================================================================

// ReversionAgent implements a mean reversion trading strategy
type ReversionAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn  *nats.Conn
	natsTopic string

	// Strategy configuration
	symbols              []string
	rsiPeriod            int
	rsiOversold          float64
	rsiOverbought        float64
	bollingerPeriod      int
	bollingerStdDev      float64
	volumeSpikeThreshold float64
	lookbackCandles      int

	// Risk Management
	stopLossPct     float64
	takeProfitPct   float64
	riskRewardRatio float64

	// Voting
	confidenceThreshold float64
	voteWeight          float64

	// BDI Architecture
	beliefs *BeliefBase

	// State
	lastSignal string
	mutex      sync.RWMutex
}

// ReversionSignal represents a mean reversion trading signal
type ReversionSignal struct {
	AgentID    string             `json:"agent_id"`
	Symbol     string             `json:"symbol"`
	Signal     string             `json:"signal"` // BUY, SELL, HOLD
	Confidence float64            `json:"confidence"`
	Price      float64            `json:"price"`
	StopLoss   float64            `json:"stop_loss"`
	TakeProfit float64            `json:"take_profit"`
	RiskReward float64            `json:"risk_reward"`
	Reasoning  string             `json:"reasoning"`
	Timestamp  time.Time          `json:"timestamp"`
	Beliefs    map[string]*Belief `json:"beliefs,omitempty"`
}

// NewReversionAgent creates a new Mean Reversion Agent
func NewReversionAgent(config *agents.AgentConfig, log zerolog.Logger, metricsPort int) (*ReversionAgent, error) {
	baseAgent := agents.NewBaseAgent(config, log, metricsPort)

	// Extract strategy configuration
	agentConfig := config.Config

	// Read NATS configuration
	natsURL := viper.GetString("nats.url")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	natsTopic := viper.GetString("communication.nats.topics.reversion_signals")
	if natsTopic == "" {
		natsTopic = "agents.strategy.reversion"
	}

	// Connect to NATS
	log.Info().Str("url", natsURL).Str("topic", natsTopic).Msg("Connecting to NATS")
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info().Msg("Successfully connected to NATS")

	// Extract strategy parameters from config (with defaults)
	rsiPeriod := getIntFromConfig(agentConfig, "rsi_period", 14)
	rsiOversold := getFloatFromConfig(agentConfig, "entry_conditions.rsi_oversold", 30.0)
	rsiOverbought := getFloatFromConfig(agentConfig, "entry_conditions.rsi_overbought", 70.0)
	bollingerPeriod := getIntFromConfig(agentConfig, "bollinger_period", 20)
	bollingerStdDev := getFloatFromConfig(agentConfig, "bollinger_std_dev", 2.0)
	volumeSpikeThreshold := getFloatFromConfig(agentConfig, "entry_conditions.volume_spike", 1.5)
	lookbackCandles := getIntFromConfig(agentConfig, "lookback_candles", 100)

	// Extract risk management configuration
	stopLossPct := getFloatFromConfig(agentConfig, "exit_conditions.stop_loss_pct", 0.02)
	takeProfitPct := getFloatFromConfig(agentConfig, "exit_conditions.take_profit_pct", 0.03)
	riskRewardRatio := getFloatFromConfig(agentConfig, "min_risk_reward", 1.5)

	// Extract voting parameters
	confidenceThreshold := getFloatFromConfig(agentConfig, "confidence_threshold", 0.70)
	voteWeight := getFloatFromConfig(agentConfig, "vote_weight", 0.3)

	// Extract symbols to analyze
	symbols := getStringSliceFromConfig(agentConfig, "symbols", []string{"bitcoin", "ethereum"})

	return &ReversionAgent{
		BaseAgent:            baseAgent,
		natsConn:             nc,
		natsTopic:            natsTopic,
		symbols:              symbols,
		rsiPeriod:            rsiPeriod,
		rsiOversold:          rsiOversold,
		rsiOverbought:        rsiOverbought,
		bollingerPeriod:      bollingerPeriod,
		bollingerStdDev:      bollingerStdDev,
		volumeSpikeThreshold: volumeSpikeThreshold,
		lookbackCandles:      lookbackCandles,
		stopLossPct:          stopLossPct,
		takeProfitPct:        takeProfitPct,
		riskRewardRatio:      riskRewardRatio,
		confidenceThreshold:  confidenceThreshold,
		voteWeight:           voteWeight,
		beliefs:              NewBeliefBase(),
		lastSignal:           "HOLD",
	}, nil
}

// Step executes one decision cycle
func (a *ReversionAgent) Step(ctx context.Context) error {
	// Call parent Step to handle metrics
	if err := a.BaseAgent.Step(ctx); err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	log.Debug().Msg("Executing mean reversion strategy step")

	// TODO: Implement decision cycle in subsequent tasks:
	// - T085: Fetch price data and calculate Bollinger Bands
	// - T086: Calculate RSI and detect extremes
	// - T087: Detect market regime (ranging vs trending)
	// - T088: Generate mean reversion signals
	// - T089: Apply risk management and publish signals

	// For now, just update basic beliefs
	a.updateBasicBeliefs()

	log.Debug().
		Float64("overall_confidence", a.beliefs.GetConfidence()).
		Msg("Decision cycle complete")

	return nil
}

// updateBasicBeliefs updates the agent's belief base with current observations
func (a *ReversionAgent) updateBasicBeliefs() {
	// Update agent state belief
	a.beliefs.UpdateBelief("agent_state", "initializing", 1.0, "internal")
	a.beliefs.UpdateBelief("last_signal", a.lastSignal, 1.0, "agent_state")
	a.beliefs.UpdateBelief("strategy", "mean_reversion", 1.0, "config")

	// Market beliefs will be added in subsequent tasks (T085-T089)
}

// publishSignal publishes a trading signal to NATS
func (a *ReversionAgent) publishSignal(ctx context.Context, signal *ReversionSignal) error {
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	topic := a.natsTopic
	if err := a.natsConn.Publish(topic, data); err != nil {
		return fmt.Errorf("failed to publish signal: %w", err)
	}

	log.Info().
		Str("symbol", signal.Symbol).
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Str("topic", topic).
		Msg("Published trading signal")

	return nil
}

// ============================================================================
// CONFIG HELPERS
// ============================================================================

// getIntFromConfig extracts an integer value from nested config map
func getIntFromConfig(config map[string]interface{}, key string, defaultVal int) int {
	if val, ok := config[key]; ok {
		if intVal, ok := val.(int); ok {
			return intVal
		}
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
	}
	return defaultVal
}

// getFloatFromConfig extracts a float value from nested config map
func getFloatFromConfig(config map[string]interface{}, key string, defaultVal float64) float64 {
	// Handle nested keys like "risk_management.stop_loss_pct"
	keys := parseKey(key)
	val := config
	for _, k := range keys[:len(keys)-1] {
		if v, ok := val[k].(map[string]interface{}); ok {
			val = v
		} else {
			return defaultVal
		}
	}

	lastKey := keys[len(keys)-1]
	if v, ok := val[lastKey]; ok {
		if floatVal, ok := v.(float64); ok {
			return floatVal
		}
		if intVal, ok := v.(int); ok {
			return float64(intVal)
		}
	}
	return defaultVal
}

// getStringSliceFromConfig extracts a string slice from config map
func getStringSliceFromConfig(config map[string]interface{}, key string, defaultVal []string) []string {
	if val, ok := config[key]; ok {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
		if slice, ok := val.([]string); ok {
			return slice
		}
	}
	return defaultVal
}

// parseKey splits a key like "risk_management.stop_loss_pct" into []string{"risk_management", "stop_loss_pct"}
func parseKey(key string) []string {
	result := []string{}
	current := ""
	for _, char := range key {
		if char == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	// Configure logging to stderr (stdout reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load agent configuration
	agentCfg, err := config.LoadAgentConfig("configs/agents.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load agent configuration")
	}

	// Get mean reversion agent config
	reversionConfig, ok := agentCfg.StrategyAgents["mean_reversion"]
	if !ok {
		log.Fatal().Msg("Mean reversion agent configuration not found")
	}

	if !reversionConfig.Enabled {
		log.Warn().Msg("Mean reversion agent is disabled in configuration")
		return
	}

	// Parse step interval
	stepInterval, err := time.ParseDuration(reversionConfig.StepInterval)
	if err != nil {
		log.Fatal().Err(err).Str("step_interval", reversionConfig.StepInterval).Msg("Invalid step interval")
	}

	// Convert to agents.AgentConfig format
	baseConfig := &agents.AgentConfig{
		Name:         reversionConfig.Name,
		Type:         reversionConfig.Type,
		Version:      reversionConfig.Version,
		MCPServers:   convertMCPServers(reversionConfig.MCPServers),
		Config:       reversionConfig.Config,
		StepInterval: stepInterval,
		Enabled:      reversionConfig.Enabled,
	}

	// Get metrics port from global config
	metricsPort := agentCfg.Global.MetricsPort
	if metricsPort == 0 {
		metricsPort = 9101
	}

	// Create agent logger
	agentLogger := log.With().Str("agent", "reversion-agent").Logger()

	// Create agent
	agent, err := NewReversionAgent(baseConfig, agentLogger, metricsPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Mean Reversion Agent")
	}
	defer agent.natsConn.Close()

	// Initialize agent (connect to MCP servers)
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	log.Info().Str("name", agent.GetName()).Msg("Mean Reversion Agent started")

	// Run agent (includes decision cycle loop)
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	// Start agent run loop in background
	go func() {
		if err := agent.Run(runCtx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("Agent run loop error")
		}
	}()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Info().Msg("Shutdown signal received, gracefully stopping...")

	// Cancel run context
	runCancel()

	// Shutdown agent
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := agent.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during agent shutdown")
	}

	log.Info().Msg("Mean Reversion Agent stopped")
}

// convertMCPServers converts config.MCPServerConnection to agents.MCPServerConfig
func convertMCPServers(servers []config.MCPServerConnection) []agents.MCPServerConfig {
	result := make([]agents.MCPServerConfig, len(servers))
	for i, server := range servers {
		result[i] = agents.MCPServerConfig{
			Name:    server.Name,
			Type:    server.Type,
			Command: server.Command,
		}
	}
	return result
}
