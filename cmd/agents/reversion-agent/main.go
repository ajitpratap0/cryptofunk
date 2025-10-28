// Mean Reversion Agent
// Generates trading signals when price deviates from mean (Bollinger Bands + RSI extremes)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// BollingerIndicators holds Bollinger Band calculations
type BollingerIndicators struct {
	UpperBand  float64   `json:"upper_band"`  // Upper Bollinger Band (mean + k*stddev)
	MiddleBand float64   `json:"middle_band"` // Middle band (SMA)
	LowerBand  float64   `json:"lower_band"`  // Lower Bollinger Band (mean - k*stddev)
	Bandwidth  float64   `json:"bandwidth"`   // (Upper - Lower) / Middle (volatility measure)
	Position   string    `json:"position"`    // "above_upper", "below_lower", "between", "at_upper", "at_lower"
	Timestamp  time.Time `json:"timestamp"`
}

// MarketRegime represents the current market state
type MarketRegime struct {
	Type       string    `json:"type"`       // "ranging", "trending", "volatile"
	ADX        float64   `json:"adx"`        // ADX value for trend strength
	Confidence float64   `json:"confidence"` // Confidence in regime assessment
	Timestamp  time.Time `json:"timestamp"`
}

// ReversionSignal represents a mean reversion trading signal
type ReversionSignal struct {
	AgentID        string               `json:"agent_id"`
	Symbol         string               `json:"symbol"`
	Signal         string               `json:"signal"` // BUY, SELL, HOLD
	Confidence     float64              `json:"confidence"`
	Price          float64              `json:"price"`
	StopLoss       float64              `json:"stop_loss"`
	TakeProfit     float64              `json:"take_profit"`
	RiskReward     float64              `json:"risk_reward"`
	Reasoning      string               `json:"reasoning"`
	Timestamp      time.Time            `json:"timestamp"`
	BollingerBands *BollingerIndicators `json:"bollinger_bands,omitempty"`
	MarketRegime   *MarketRegime        `json:"market_regime,omitempty"`
	RSI            float64              `json:"rsi,omitempty"`
	Beliefs        map[string]*Belief   `json:"beliefs,omitempty"`
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

	// Step 1: Fetch market data
	symbols := a.getSymbolsToAnalyze()
	if len(symbols) == 0 {
		log.Warn().Msg("No symbols to analyze")
		return nil
	}

	symbol := symbols[0] // Analyze first symbol
	log.Debug().Str("symbol", symbol).Msg("Analyzing symbol for mean reversion")

	// Fetch price data (need enough candles for Bollinger Band calculations)
	prices, currentPrice, err := a.fetchPriceData(ctx, symbol)
	if err != nil {
		log.Error().Err(err).Str("symbol", symbol).Msg("Failed to fetch price data")
		return fmt.Errorf("failed to fetch market data: %w", err)
	}

	log.Debug().
		Str("symbol", symbol).
		Int("price_count", len(prices)).
		Float64("current_price", currentPrice).
		Msg("Retrieved price data")

	// Step 2: Calculate Bollinger Bands (T085)
	bollinger, err := a.calculateBollingerBands(ctx, symbol, prices)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate Bollinger Bands")
		return fmt.Errorf("Bollinger Band calculation failed: %w", err)
	}

	log.Info().
		Float64("upper_band", bollinger.UpperBand).
		Float64("middle_band", bollinger.MiddleBand).
		Float64("lower_band", bollinger.LowerBand).
		Float64("bandwidth", bollinger.Bandwidth).
		Str("position", bollinger.Position).
		Msg("Bollinger Bands calculated")

	// Step 2.5: Detect band touch and generate signal
	signal, confidence, reasoning := a.detectBandTouch(bollinger, currentPrice)

	log.Info().
		Str("signal", signal).
		Float64("confidence", confidence).
		Str("reasoning", reasoning).
		Msg("Bollinger Band signal detected")

	// Step 3: Update agent beliefs with Bollinger Band data
	a.updateBollingerBeliefs(bollinger, currentPrice)
	a.beliefs.UpdateBelief("last_signal", signal, confidence, "bollinger_bands")

	// Update agent state
	a.lastSignal = signal

	log.Debug().
		Float64("overall_confidence", a.beliefs.GetConfidence()).
		Msg("Decision cycle complete")

	// TODO: Remaining tasks:
	// - T086: Calculate RSI and detect extremes (combine with Bollinger for confirmation)
	// - T087: Detect market regime (ranging vs trending) - only trade in ranging markets
	// - T088: Implement quick exit logic (tight stops, small profit targets)
	// - T089: Generate full trading signal with risk management and publish to NATS

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

// getSymbolsToAnalyze returns the list of symbols to analyze
func (a *ReversionAgent) getSymbolsToAnalyze() []string {
	return a.symbols
}

// fetchPriceData fetches historical price data from CoinGecko
func (a *ReversionAgent) fetchPriceData(ctx context.Context, symbol string) ([]float64, float64, error) {
	// Calculate days needed for lookback candles (using hourly data)
	days := max(1, (a.lookbackCandles+23)/24)

	// Call CoinGecko MCP tool
	result, err := a.CallMCPTool(ctx, "coingecko", "get_market_chart", map[string]interface{}{
		"id":          symbol,
		"vs_currency": "usd",
		"days":        days,
		"interval":    "hourly",
	})
	if err != nil {
		return nil, 0, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content
	if len(result.Content) == 0 {
		return nil, 0, fmt.Errorf("empty result from CoinGecko")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, 0, fmt.Errorf("invalid content type")
	}

	// Parse JSON - CoinGecko returns {prices: [[timestamp, price], ...]}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	pricesRaw, ok := resultMap["prices"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("prices field not found")
	}

	// Extract close prices
	prices := make([]float64, 0, len(pricesRaw))
	var latestPrice float64

	for _, p := range pricesRaw {
		point, ok := p.([]interface{})
		if !ok || len(point) != 2 {
			continue
		}

		price, ok := point[1].(float64)
		if !ok {
			continue
		}

		prices = append(prices, price)
		latestPrice = price // Keep track of latest
	}

	if len(prices) == 0 {
		return nil, 0, fmt.Errorf("no price data available")
	}

	log.Debug().
		Str("symbol", symbol).
		Int("days", days).
		Int("candles", len(prices)).
		Float64("latest_price", latestPrice).
		Msg("Fetched price data from CoinGecko")

	return prices, latestPrice, nil
}

// ============================================================================
// BOLLINGER BAND STRATEGY (T085)
// ============================================================================

// calculateBollingerBands fetches price data and calculates Bollinger Bands using MCP server
func (a *ReversionAgent) calculateBollingerBands(ctx context.Context, symbol string, priceData []float64) (*BollingerIndicators, error) {
	log.Debug().
		Str("symbol", symbol).
		Int("data_points", len(priceData)).
		Int("period", a.bollingerPeriod).
		Float64("std_dev", a.bollingerStdDev).
		Msg("Calculating Bollinger Bands")

	// Call Technical Indicators MCP server for Bollinger Bands
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_bollinger_bands", map[string]interface{}{
		"prices": priceData,
		"period": a.bollingerPeriod,
		"k":      a.bollingerStdDev, // Number of standard deviations
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate Bollinger Bands: %w", err)
	}

	// Parse result - extract text content
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from technical indicators server")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		return nil, fmt.Errorf("failed to parse Bollinger Bands result: %w", err)
	}

	// Extract band values (last value in each array)
	upperBands := data["upper_band"].([]interface{})
	middleBands := data["middle_band"].([]interface{})
	lowerBands := data["lower_band"].([]interface{})

	if len(upperBands) == 0 || len(middleBands) == 0 || len(lowerBands) == 0 {
		return nil, fmt.Errorf("insufficient data for Bollinger Bands calculation")
	}

	// Get current (last) values
	upperBand := upperBands[len(upperBands)-1].(float64)
	middleBand := middleBands[len(middleBands)-1].(float64)
	lowerBand := lowerBands[len(lowerBands)-1].(float64)

	// Calculate bandwidth (volatility measure)
	bandwidth := 0.0
	if middleBand > 0 {
		bandwidth = (upperBand - lowerBand) / middleBand
	}

	// Detect position relative to bands
	currentPrice := priceData[len(priceData)-1]
	position := a.detectBandPosition(currentPrice, upperBand, middleBand, lowerBand)

	indicators := &BollingerIndicators{
		UpperBand:  upperBand,
		MiddleBand: middleBand,
		LowerBand:  lowerBand,
		Bandwidth:  bandwidth,
		Position:   position,
		Timestamp:  time.Now(),
	}

	log.Debug().
		Float64("upper", upperBand).
		Float64("middle", middleBand).
		Float64("lower", lowerBand).
		Float64("bandwidth", bandwidth).
		Str("position", position).
		Msg("Bollinger Bands calculated")

	return indicators, nil
}

// detectBandPosition determines where price is relative to Bollinger Bands
func (a *ReversionAgent) detectBandPosition(price, upperBand, middleBand, lowerBand float64) string {
	// Calculate thresholds (within 0.5% of band = "at band")
	touchThreshold := 0.005 // 0.5%
	upperThreshold := upperBand * (1 - touchThreshold)
	lowerThreshold := lowerBand * (1 + touchThreshold)

	if price >= upperThreshold && price <= upperBand {
		return "at_upper" // Price touching upper band (overbought signal)
	} else if price >= lowerBand && price <= lowerThreshold {
		return "at_lower" // Price touching lower band (oversold signal)
	} else if price > upperBand {
		return "above_upper" // Price above upper band (extreme overbought)
	} else if price < lowerBand {
		return "below_lower" // Price below lower band (extreme oversold)
	} else {
		return "between" // Price between bands (no signal)
	}
}

// detectBandTouch checks if current price is touching Bollinger Bands
// Returns: signal type ("BUY", "SELL", "HOLD"), confidence (0.0-1.0), reasoning
func (a *ReversionAgent) detectBandTouch(indicators *BollingerIndicators, currentPrice float64) (string, float64, string) {
	position := indicators.Position

	// Mean reversion signals:
	// - Price at lower band = oversold = BUY signal (expect bounce back up)
	// - Price at upper band = overbought = SELL signal (expect pullback down)
	// - Price between bands = no clear signal = HOLD

	switch position {
	case "at_lower", "below_lower":
		// Oversold condition - price likely to revert UP
		confidence := 0.7
		if position == "below_lower" {
			confidence = 0.8 // Higher confidence when price is extremely low
		}
		// Adjust confidence based on bandwidth (tighter bands = higher confidence)
		if indicators.Bandwidth < 0.05 { // Low volatility
			confidence += 0.1
		}
		confidence = math.Min(confidence, 1.0)

		reasoning := fmt.Sprintf("Price %.2f at/below lower Bollinger Band (%.2f). Oversold condition detected - expect mean reversion upward. Bandwidth: %.4f",
			currentPrice, indicators.LowerBand, indicators.Bandwidth)

		return "BUY", confidence, reasoning

	case "at_upper", "above_upper":
		// Overbought condition - price likely to revert DOWN
		confidence := 0.7
		if position == "above_upper" {
			confidence = 0.8 // Higher confidence when price is extremely high
		}
		// Adjust confidence based on bandwidth
		if indicators.Bandwidth < 0.05 {
			confidence += 0.1
		}
		confidence = math.Min(confidence, 1.0)

		reasoning := fmt.Sprintf("Price %.2f at/above upper Bollinger Band (%.2f). Overbought condition detected - expect mean reversion downward. Bandwidth: %.4f",
			currentPrice, indicators.UpperBand, indicators.Bandwidth)

		return "SELL", confidence, reasoning

	default: // "between"
		// Price within bands - no clear mean reversion signal
		reasoning := fmt.Sprintf("Price %.2f between Bollinger Bands (%.2f - %.2f). No mean reversion signal - awaiting band touch.",
			currentPrice, indicators.LowerBand, indicators.UpperBand)

		return "HOLD", 0.5, reasoning
	}
}

// updateBollingerBeliefs updates beliefs with Bollinger Band data
func (a *ReversionAgent) updateBollingerBeliefs(indicators *BollingerIndicators, currentPrice float64) {
	a.beliefs.UpdateBelief("bollinger_upper", indicators.UpperBand, 0.9, "bollinger_bands")
	a.beliefs.UpdateBelief("bollinger_middle", indicators.MiddleBand, 0.9, "bollinger_bands")
	a.beliefs.UpdateBelief("bollinger_lower", indicators.LowerBand, 0.9, "bollinger_bands")
	a.beliefs.UpdateBelief("bollinger_bandwidth", indicators.Bandwidth, 0.85, "bollinger_bands")
	a.beliefs.UpdateBelief("bollinger_position", indicators.Position, 0.9, "bollinger_bands")
	a.beliefs.UpdateBelief("current_price", currentPrice, 1.0, "market_data")

	// Calculate position confidence based on bandwidth
	// Tighter bands (low bandwidth) = more reliable signals
	positionConfidence := 0.7
	if indicators.Bandwidth < 0.05 {
		positionConfidence = 0.9
	} else if indicators.Bandwidth > 0.15 {
		positionConfidence = 0.5 // High volatility reduces confidence
	}
	a.beliefs.UpdateBelief("band_signal_confidence", positionConfidence, positionConfidence, "bollinger_bands")

	log.Debug().
		Str("position", indicators.Position).
		Float64("bandwidth", indicators.Bandwidth).
		Float64("confidence", positionConfidence).
		Msg("Bollinger beliefs updated")
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
