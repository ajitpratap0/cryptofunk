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
	"github.com/ajitpratap0/cryptofunk/internal/llm"
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

	// LLM client for AI-powered analysis
	llmClient     *llm.Client
	promptBuilder *llm.PromptBuilder
	useLLM        bool

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

	// Initialize LLM client if enabled
	var llmClient *llm.Client
	var promptBuilder *llm.PromptBuilder
	useLLM := viper.GetBool("llm.enabled")

	if useLLM {
		llmConfig := llm.ClientConfig{
			Endpoint:    viper.GetString("llm.endpoint"),
			APIKey:      viper.GetString("llm.api_key"),
			Model:       viper.GetString("llm.primary_model"),
			Temperature: viper.GetFloat64("llm.temperature"),
			MaxTokens:   viper.GetInt("llm.max_tokens"),
			Timeout:     viper.GetDuration("llm.timeout"),
		}
		llmClient = llm.NewClient(llmConfig)
		promptBuilder = llm.NewPromptBuilder(llm.AgentTypeReversion)
		log.Info().Msg("LLM-powered mean reversion analysis enabled")
	} else {
		log.Info().Msg("Using rule-based mean reversion analysis")
	}

	return &ReversionAgent{
		BaseAgent:            baseAgent,
		natsConn:             nc,
		natsTopic:            natsTopic,
		llmClient:            llmClient,
		promptBuilder:        promptBuilder,
		useLLM:               useLLM,
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

	// Step 2.5: Detect Bollinger Band signal
	bbSignal, bbConfidence, bbReasoning := a.detectBandTouch(bollinger, currentPrice)

	log.Info().
		Str("signal", bbSignal).
		Float64("confidence", bbConfidence).
		Str("reasoning", bbReasoning).
		Msg("Bollinger Band signal detected")

	// Step 3: Calculate RSI (T086)
	rsi, err := a.calculateRSI(ctx, symbol, prices)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate RSI")
		return fmt.Errorf("RSI calculation failed: %w", err)
	}

	log.Info().
		Float64("rsi", rsi).
		Msg("RSI calculated")

	// Step 3.5: Detect RSI extremes
	rsiSignal, rsiConfidence, rsiReasoning := a.detectRSIExtreme(rsi)

	log.Info().
		Str("signal", rsiSignal).
		Float64("confidence", rsiConfidence).
		Str("reasoning", rsiReasoning).
		Msg("RSI signal detected")

	// Step 4: Generate final signal (LLM or rule-based)
	finalSignal, finalConfidence, finalReasoning := a.generateMeanReversionSignal(
		ctx, symbol, currentPrice, bollinger, rsi,
		bbSignal, bbConfidence, bbReasoning,
		rsiSignal, rsiConfidence, rsiReasoning,
	)

	log.Info().
		Str("final_signal", finalSignal).
		Float64("final_confidence", finalConfidence).
		Str("reasoning", finalReasoning).
		Msg("Mean reversion signal generated")

	// Step 4.5: Detect market regime and filter signal (T087)
	adx, err := a.calculateADX(ctx, symbol, prices)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate ADX")
		return fmt.Errorf("ADX calculation failed: %w", err)
	}

	log.Info().
		Float64("adx", adx).
		Msg("ADX calculated for regime detection")

	// Detect market regime (ranging vs trending vs volatile)
	regime := a.detectMarketRegime(adx)

	log.Info().
		Str("regime", regime.Type).
		Float64("adx", regime.ADX).
		Float64("confidence", regime.Confidence).
		Msg("Market regime detected")

	// Filter signal based on regime (mean reversion only works in ranging markets)
	filteredSignal, filteredConfidence, filteredReasoning := a.filterSignalByRegime(
		finalSignal, finalConfidence, finalReasoning, regime,
	)

	log.Info().
		Str("filtered_signal", filteredSignal).
		Float64("filtered_confidence", filteredConfidence).
		Bool("signal_changed", filteredSignal != finalSignal).
		Str("reasoning", filteredReasoning).
		Msg("Signal filtered by market regime")

	// Step 4.7: Calculate exit levels for quick exits (T088)
	stopLoss, takeProfit, riskReward := a.calculateExitLevels(filteredSignal, currentPrice)

	log.Info().
		Str("signal", filteredSignal).
		Float64("stop_loss", stopLoss).
		Float64("take_profit", takeProfit).
		Float64("risk_reward", riskReward).
		Msg("Exit levels calculated")

	// Check if risk/reward meets minimum threshold
	if filteredSignal != "HOLD" && riskReward < a.riskRewardRatio {
		log.Warn().
			Float64("risk_reward", riskReward).
			Float64("min_required", a.riskRewardRatio).
			Msg("Risk/reward ratio too low - changing signal to HOLD")

		filteredSignal = "HOLD"
		filteredConfidence = 0.3
		filteredReasoning = fmt.Sprintf("RISK/REWARD FILTER: Risk/reward ratio %.2f is below minimum %.2f. Trade rejected. %s",
			riskReward, a.riskRewardRatio, filteredReasoning)
	}

	// Step 5: Update agent beliefs with all indicator data
	a.updateBollingerBeliefs(bollinger, currentPrice)
	a.updateRSIBeliefs(rsi, rsiSignal, rsiConfidence)
	a.updateRegimeBeliefs(regime)
	a.updateExitBeliefs(stopLoss, takeProfit, riskReward)
	a.beliefs.UpdateBelief("combined_signal", finalSignal, finalConfidence, "signal_combiner")
	a.beliefs.UpdateBelief("final_signal", filteredSignal, filteredConfidence, "regime_filter")

	// Update agent state
	a.lastSignal = filteredSignal

	// Step 6: Generate full trading signal with all data (T089)
	tradingSignal := a.generateTradingSignal(
		symbol,
		filteredSignal,
		filteredConfidence,
		filteredReasoning,
		currentPrice,
		stopLoss,
		takeProfit,
		riskReward,
		bollinger,
		rsi,
		regime,
	)

	// Step 7: Publish signal to NATS for orchestrator and other agents
	if err := a.publishSignal(ctx, tradingSignal); err != nil {
		log.Error().Err(err).Msg("Failed to publish trading signal")
		return fmt.Errorf("signal publication failed: %w", err)
	}

	log.Info().
		Str("symbol", symbol).
		Str("signal", filteredSignal).
		Float64("confidence", filteredConfidence).
		Float64("overall_confidence", a.beliefs.GetConfidence()).
		Msg("Decision cycle complete - signal published")

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

// ============================================================================
// RSI EXTREMES DETECTION (T086)
// ============================================================================

// calculateRSI calculates the Relative Strength Index using MCP Technical Indicators server
func (a *ReversionAgent) calculateRSI(ctx context.Context, symbol string, priceData []float64) (float64, error) {
	log.Debug().
		Str("symbol", symbol).
		Int("data_points", len(priceData)).
		Int("period", a.rsiPeriod).
		Msg("Calculating RSI")

	// Call Technical Indicators MCP server for RSI
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_rsi", map[string]interface{}{
		"prices": priceData,
		"period": a.rsiPeriod, // Default: 14
	})
	if err != nil {
		return 0, fmt.Errorf("failed to calculate RSI: %w", err)
	}

	// Parse result - extract text content
	if len(result.Content) == 0 {
		return 0, fmt.Errorf("empty result from technical indicators server")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return 0, fmt.Errorf("invalid content type")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		return 0, fmt.Errorf("failed to parse RSI result: %w", err)
	}

	// Extract RSI values array
	rsiValues, ok := data["values"].([]interface{})
	if !ok || len(rsiValues) == 0 {
		return 0, fmt.Errorf("RSI values not found in result")
	}

	// Get current (last) RSI value
	currentRSI := rsiValues[len(rsiValues)-1].(float64)

	log.Debug().
		Float64("rsi", currentRSI).
		Msg("RSI calculated")

	return currentRSI, nil
}

// detectRSIExtreme detects oversold/overbought conditions based on RSI
// Returns: signal (BUY/SELL/HOLD), confidence (0.0-1.0), reasoning
func (a *ReversionAgent) detectRSIExtreme(rsi float64) (string, float64, string) {
	if rsi < 30 {
		// Oversold condition - price likely to revert UP
		confidence := 0.7
		if rsi < 20 {
			confidence = 0.9 // Very oversold - higher confidence
		}
		reasoning := fmt.Sprintf("RSI %.2f indicates oversold condition (< 30). Expect mean reversion upward.", rsi)
		return "BUY", confidence, reasoning
	} else if rsi > 70 {
		// Overbought condition - price likely to revert DOWN
		confidence := 0.7
		if rsi > 80 {
			confidence = 0.9 // Very overbought - higher confidence
		}
		reasoning := fmt.Sprintf("RSI %.2f indicates overbought condition (> 70). Expect mean reversion downward.", rsi)
		return "SELL", confidence, reasoning
	} else {
		// Neutral zone - no clear signal
		reasoning := fmt.Sprintf("RSI %.2f in neutral zone (30-70). No RSI-based signal.", rsi)
		return "HOLD", 0.5, reasoning
	}
}

// generateMeanReversionSignal routes to LLM or rule-based signal generation
func (a *ReversionAgent) generateMeanReversionSignal(
	ctx context.Context,
	symbol string,
	currentPrice float64,
	bollinger *BollingerIndicators,
	rsi float64,
	bbSignal string,
	bbConfidence float64,
	bbReasoning string,
	rsiSignal string,
	rsiConfidence float64,
	rsiReasoning string,
) (string, float64, string) {
	if a.useLLM && a.llmClient != nil {
		signal, confidence, reasoning, err := a.generateSignalWithLLM(
			ctx, symbol, currentPrice, bollinger, rsi,
		)
		if err != nil {
			log.Warn().Err(err).Msg("LLM request failed, falling back to rule-based analysis")
			return a.combineSignalsRuleBased(bbSignal, bbConfidence, bbReasoning, rsiSignal, rsiConfidence, rsiReasoning)
		}
		return signal, confidence, reasoning
	}
	return a.combineSignalsRuleBased(bbSignal, bbConfidence, bbReasoning, rsiSignal, rsiConfidence, rsiReasoning)
}

// generateSignalWithLLM generates a signal using LLM-powered analysis
func (a *ReversionAgent) generateSignalWithLLM(
	ctx context.Context,
	symbol string,
	currentPrice float64,
	bollinger *BollingerIndicators,
	rsi float64,
) (string, float64, string, error) {
	log.Debug().Str("symbol", symbol).Msg("Generating mean reversion signal (LLM-powered)")

	// Build market context for LLM
	indicatorMap := make(map[string]float64)
	indicatorMap["rsi"] = rsi
	indicatorMap["bollinger_upper"] = bollinger.UpperBand
	indicatorMap["bollinger_middle"] = bollinger.MiddleBand
	indicatorMap["bollinger_lower"] = bollinger.LowerBand
	indicatorMap["bollinger_bandwidth"] = bollinger.Bandwidth

	marketCtx := llm.MarketContext{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		Indicators:   indicatorMap,
		Timestamp:    time.Now(),
	}

	// Build LLM prompt
	userPrompt := a.promptBuilder.BuildMeanReversionPrompt(marketCtx, nil) // No positions yet
	systemPrompt := a.promptBuilder.GetSystemPrompt()

	// Call LLM with retry logic
	response, err := a.llmClient.CompleteWithRetry(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 2) // 2 retries

	if err != nil {
		return "", 0, "", fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse LLM response
	if len(response.Choices) == 0 {
		return "", 0, "", fmt.Errorf("LLM returned no choices")
	}

	content := response.Choices[0].Message.Content

	// Parse JSON response
	var llmSignal llm.Signal
	if err := a.llmClient.ParseJSONResponse(content, &llmSignal); err != nil {
		return "", 0, "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Validate signal
	if llmSignal.Side != "BUY" && llmSignal.Side != "SELL" && llmSignal.Side != "HOLD" {
		return "", 0, "", fmt.Errorf("invalid signal from LLM: %s", llmSignal.Side)
	}

	log.Info().
		Str("symbol", symbol).
		Str("signal", llmSignal.Side).
		Float64("confidence", llmSignal.Confidence).
		Str("reasoning", llmSignal.Reasoning).
		Msg("Generated LLM-powered mean reversion signal")

	return llmSignal.Side, llmSignal.Confidence, llmSignal.Reasoning, nil
}

// combineSignalsRuleBased combines Bollinger Band and RSI signals for confirmation (rule-based)
// Returns: final signal, combined confidence, reasoning
func (a *ReversionAgent) combineSignalsRuleBased(bbSignal string, bbConfidence float64, bbReasoning string,
	rsiSignal string, rsiConfidence float64, rsiReasoning string) (string, float64, string) {

	// Case 1: Both signals agree (strong confirmation)
	if bbSignal == rsiSignal && bbSignal != "HOLD" {
		// Both agree on BUY or SELL - high confidence
		combinedConfidence := (bbConfidence + rsiConfidence) / 2.0
		combinedConfidence = math.Min(combinedConfidence+0.1, 1.0) // Bonus for agreement
		reasoning := fmt.Sprintf("STRONG SIGNAL - Both Bollinger Bands and RSI agree on %s. Bollinger: %s. RSI: %s",
			bbSignal, bbReasoning, rsiReasoning)
		return bbSignal, combinedConfidence, reasoning
	}

	// Case 2: Signals conflict (one BUY, one SELL)
	if (bbSignal == "BUY" && rsiSignal == "SELL") || (bbSignal == "SELL" && rsiSignal == "BUY") {
		// Conflicting signals - reduce confidence, use HOLD
		reasoning := fmt.Sprintf("CONFLICTING SIGNALS - Bollinger says %s (%.2f), RSI says %s (%.2f). Holding position due to uncertainty.",
			bbSignal, bbConfidence, rsiSignal, rsiConfidence)
		return "HOLD", 0.3, reasoning
	}

	// Case 3: One signal is HOLD (use the non-HOLD signal)
	if bbSignal == "HOLD" && rsiSignal != "HOLD" {
		// RSI has signal, Bollinger is neutral
		reasoning := fmt.Sprintf("RSI signal %s (confidence: %.2f) with neutral Bollinger position. RSI: %s",
			rsiSignal, rsiConfidence, rsiReasoning)
		return rsiSignal, rsiConfidence * 0.8, reasoning // Slightly reduce confidence without full confirmation
	}
	if rsiSignal == "HOLD" && bbSignal != "HOLD" {
		// Bollinger has signal, RSI is neutral
		reasoning := fmt.Sprintf("Bollinger signal %s (confidence: %.2f) with neutral RSI. Bollinger: %s",
			bbSignal, bbConfidence, bbReasoning)
		return bbSignal, bbConfidence * 0.8, reasoning // Slightly reduce confidence without full confirmation
	}

	// Case 4: Both signals are HOLD
	reasoning := fmt.Sprintf("Both Bollinger Bands and RSI are neutral. No clear mean reversion signal. Bollinger: %s. RSI: %s",
		bbReasoning, rsiReasoning)
	return "HOLD", 0.5, reasoning
}

// updateRSIBeliefs updates the agent's belief base with RSI data
func (a *ReversionAgent) updateRSIBeliefs(rsi float64, signal string, confidence float64) {
	a.beliefs.UpdateBelief("rsi_value", rsi, 0.9, "rsi_indicator")
	a.beliefs.UpdateBelief("rsi_signal", signal, confidence, "rsi_indicator")

	// Classify RSI state
	var rsiState string
	if rsi < 20 {
		rsiState = "very_oversold"
	} else if rsi < 30 {
		rsiState = "oversold"
	} else if rsi > 80 {
		rsiState = "very_overbought"
	} else if rsi > 70 {
		rsiState = "overbought"
	} else {
		rsiState = "neutral"
	}
	a.beliefs.UpdateBelief("rsi_state", rsiState, 0.9, "rsi_indicator")

	log.Debug().
		Float64("rsi", rsi).
		Str("state", rsiState).
		Str("signal", signal).
		Float64("confidence", confidence).
		Msg("RSI beliefs updated")
}

// ============================================================================
// MARKET REGIME DETECTION (T087)
// ============================================================================

// calculateADX calculates the Average Directional Index using MCP Technical Indicators server
func (a *ReversionAgent) calculateADX(ctx context.Context, symbol string, priceData []float64) (float64, error) {
	log.Debug().
		Str("symbol", symbol).
		Int("data_points", len(priceData)).
		Msg("Calculating ADX for regime detection")

	// Call Technical Indicators MCP server for ADX
	// Note: ADX typically requires high/low/close data, but we'll use a simplified version with close prices
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_adx", map[string]interface{}{
		"prices": priceData,
		"period": 14, // Standard ADX period
	})
	if err != nil {
		return 0, fmt.Errorf("failed to calculate ADX: %w", err)
	}

	// Parse result - extract text content
	if len(result.Content) == 0 {
		return 0, fmt.Errorf("empty result from technical indicators server")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return 0, fmt.Errorf("invalid content type")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		return 0, fmt.Errorf("failed to parse ADX result: %w", err)
	}

	// Extract ADX value
	var adx float64
	if adxValue, ok := data["value"].(float64); ok {
		adx = adxValue
	} else if adxValues, ok := data["values"].([]interface{}); ok && len(adxValues) > 0 {
		// If array, get last value
		adx = adxValues[len(adxValues)-1].(float64)
	} else {
		return 0, fmt.Errorf("ADX value not found in result")
	}

	log.Debug().
		Float64("adx", adx).
		Msg("ADX calculated")

	return adx, nil
}

// detectMarketRegime detects whether the market is ranging or trending based on ADX
// Mean reversion strategies work best in ranging markets (low ADX)
// Returns: MarketRegime with type, ADX value, confidence
func (a *ReversionAgent) detectMarketRegime(adx float64) *MarketRegime {
	var regimeType string
	var confidence float64

	if adx < 20 {
		// Very weak trend - ranging market (ideal for mean reversion)
		regimeType = "ranging"
		confidence = 0.9
	} else if adx < 25 {
		// Weak trend - still ranging but less clear
		regimeType = "ranging"
		confidence = 0.7
	} else if adx < 40 {
		// Moderate trend - not ideal for mean reversion
		regimeType = "trending"
		confidence = 0.7
	} else if adx < 50 {
		// Strong trend - avoid mean reversion
		regimeType = "trending"
		confidence = 0.9
	} else {
		// Very strong trend or high volatility - definitely avoid mean reversion
		regimeType = "volatile"
		confidence = 0.95
	}

	regime := &MarketRegime{
		Type:       regimeType,
		ADX:        adx,
		Confidence: confidence,
		Timestamp:  time.Now(),
	}

	log.Debug().
		Str("regime", regimeType).
		Float64("adx", adx).
		Float64("confidence", confidence).
		Msg("Market regime detected")

	return regime
}

// filterSignalByRegime filters trading signals based on market regime
// Mean reversion only works in ranging markets - returns HOLD if trending/volatile
func (a *ReversionAgent) filterSignalByRegime(signal string, confidence float64, reasoning string, regime *MarketRegime) (string, float64, string) {
	// If already HOLD, no need to filter
	if signal == "HOLD" {
		return signal, confidence, reasoning
	}

	// Check if regime is suitable for mean reversion trading
	if regime.Type == "ranging" {
		// Good regime for mean reversion - allow signal through
		updatedReasoning := fmt.Sprintf("%s Market regime: RANGING (ADX: %.2f) - favorable for mean reversion.",
			reasoning, regime.ADX)
		return signal, confidence, updatedReasoning
	}

	// Trending or volatile market - suppress mean reversion signal
	filteredReasoning := fmt.Sprintf("REGIME FILTER: Market is %s (ADX: %.2f). Mean reversion suppressed. Original signal: %s (%.2f confidence). %s",
		regime.Type, regime.ADX, signal, confidence, reasoning)

	// Reduce confidence significantly when regime is unfavorable
	reducedConfidence := 0.2

	log.Warn().
		Str("regime", regime.Type).
		Float64("adx", regime.ADX).
		Str("original_signal", signal).
		Msg("Signal suppressed due to unfavorable market regime")

	return "HOLD", reducedConfidence, filteredReasoning
}

// updateRegimeBeliefs updates the agent's belief base with market regime data
func (a *ReversionAgent) updateRegimeBeliefs(regime *MarketRegime) {
	a.beliefs.UpdateBelief("market_regime", regime.Type, regime.Confidence, "adx_indicator")
	a.beliefs.UpdateBelief("adx_value", regime.ADX, 0.9, "adx_indicator")

	// Determine if regime is favorable for mean reversion
	isFavorable := regime.Type == "ranging"
	a.beliefs.UpdateBelief("regime_favorable", isFavorable, regime.Confidence, "adx_indicator")

	log.Debug().
		Str("regime", regime.Type).
		Float64("adx", regime.ADX).
		Bool("favorable", isFavorable).
		Msg("Regime beliefs updated")
}

// calculateExitLevels calculates stop-loss and take-profit levels for quick exits (T088)
// Mean reversion strategy uses tight stops (2%) and quick profit targets (1-2%)
// Returns: stopLoss, takeProfit, riskReward
func (a *ReversionAgent) calculateExitLevels(signal string, entryPrice float64) (float64, float64, float64) {
	if signal == "HOLD" {
		return 0, 0, 0
	}

	var stopLoss, takeProfit, risk, reward, riskReward float64

	if signal == "BUY" {
		// For BUY (long position):
		// - Entry: current price
		// - Stop-loss: 2% below entry (tight stop for mean reversion)
		// - Take-profit: 1-2% above entry (quick exit)
		stopLoss = entryPrice * (1.0 - a.stopLossPct)
		takeProfit = entryPrice * (1.0 + a.takeProfitPct)

		// Calculate risk/reward
		risk = entryPrice - stopLoss
		reward = takeProfit - entryPrice
	} else if signal == "SELL" {
		// For SELL (short position):
		// - Entry: current price
		// - Stop-loss: 2% above entry (tight stop for mean reversion)
		// - Take-profit: 1-2% below entry (quick exit)
		stopLoss = entryPrice * (1.0 + a.stopLossPct)
		takeProfit = entryPrice * (1.0 - a.takeProfitPct)

		// Calculate risk/reward
		risk = stopLoss - entryPrice
		reward = entryPrice - takeProfit
	}

	// Calculate risk/reward ratio
	if risk > 0 {
		riskReward = reward / risk
	}

	log.Debug().
		Str("signal", signal).
		Float64("entry", entryPrice).
		Float64("stop_loss", stopLoss).
		Float64("take_profit", takeProfit).
		Float64("risk", risk).
		Float64("reward", reward).
		Float64("risk_reward", riskReward).
		Msg("Exit levels calculated")

	return stopLoss, takeProfit, riskReward
}

// updateExitBeliefs updates the agent's belief base with exit level data
func (a *ReversionAgent) updateExitBeliefs(stopLoss, takeProfit, riskReward float64) {
	a.beliefs.UpdateBelief("stop_loss", stopLoss, 1.0, "risk_management")
	a.beliefs.UpdateBelief("take_profit", takeProfit, 1.0, "risk_management")
	a.beliefs.UpdateBelief("risk_reward_ratio", riskReward, 1.0, "risk_management")

	// Assess if risk/reward is favorable
	isFavorable := riskReward >= a.riskRewardRatio
	a.beliefs.UpdateBelief("risk_reward_favorable", isFavorable, 1.0, "risk_management")

	log.Debug().
		Float64("stop_loss", stopLoss).
		Float64("take_profit", takeProfit).
		Float64("risk_reward", riskReward).
		Bool("favorable", isFavorable).
		Msg("Exit beliefs updated")
}

// generateTradingSignal creates a complete ReversionSignal with all indicator data (T089)
func (a *ReversionAgent) generateTradingSignal(
	symbol string,
	signal string,
	confidence float64,
	reasoning string,
	currentPrice float64,
	stopLoss float64,
	takeProfit float64,
	riskReward float64,
	bollinger *BollingerIndicators,
	rsi float64,
	regime *MarketRegime,
) *ReversionSignal {
	// Create the signal with all collected data
	tradingSignal := &ReversionSignal{
		AgentID:        a.GetName(),
		Symbol:         symbol,
		Signal:         signal,
		Confidence:     confidence,
		Price:          currentPrice,
		StopLoss:       stopLoss,
		TakeProfit:     takeProfit,
		RiskReward:     riskReward,
		Reasoning:      reasoning,
		Timestamp:      time.Now(),
		BollingerBands: bollinger,
		RSI:            rsi,
		MarketRegime:   regime,
		Beliefs:        a.beliefs.GetAllBeliefs(), // Include full belief state for transparency
	}

	log.Info().
		Str("agent_id", tradingSignal.AgentID).
		Str("symbol", tradingSignal.Symbol).
		Str("signal", tradingSignal.Signal).
		Float64("confidence", tradingSignal.Confidence).
		Float64("price", tradingSignal.Price).
		Float64("stop_loss", tradingSignal.StopLoss).
		Float64("take_profit", tradingSignal.TakeProfit).
		Float64("risk_reward", tradingSignal.RiskReward).
		Float64("rsi", tradingSignal.RSI).
		Str("regime", tradingSignal.MarketRegime.Type).
		Int("belief_count", len(tradingSignal.Beliefs)).
		Msg("Trading signal generated")

	return tradingSignal
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
