// Trend Following Agent
// Generates trading signals using EMA crossover and trend strength (ADX)
//
//nolint:goconst // Trading signals are domain-specific strings
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
	"github.com/ajitpratap0/cryptofunk/internal/llm"
)

// TrendAgent performs trend following strategy using EMA crossovers and ADX
type TrendAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn  *nats.Conn
	natsTopic string

	// LLM client for AI-powered analysis
	llmClient     llm.LLMClient // Interface supports both Client and FallbackClient
	promptBuilder *llm.PromptBuilder
	useLLM        bool // Enable/disable LLM reasoning

	// Strategy configuration
	symbols         []string
	fastEMAPeriod   int
	slowEMAPeriod   int
	adxPeriod       int
	adxThreshold    float64 // Minimum ADX to consider trend strong
	lookbackCandles int     // Number of candles to fetch for analysis

	// Risk management configuration
	stopLossPct     float64 // Stop-loss percentage (e.g., 0.02 for 2%)
	takeProfitPct   float64 // Take-profit percentage (e.g., 0.03 for 3%)
	trailingStopPct float64 // Trailing stop percentage (e.g., 0.015 for 1.5%)
	useTrailingStop bool    // Whether to use trailing stop-loss
	riskRewardRatio float64 // Minimum risk/reward ratio (e.g., 2.0)

	// BDI (Belief-Desire-Intention) architecture
	beliefs *BeliefBase // Agent's beliefs about market state

	// Cached indicator values
	currentIndicators *TrendIndicators

	// Strategy state
	lastCrossover  string // "bullish", "bearish", "none"
	lastSignal     string // Last signal generated
	lastSignalTime time.Time
	entryPrice     float64 // Entry price for trailing stop calculation
	highestPrice   float64 // Highest price since entry (for long trailing stop)
	lowestPrice    float64 // Lowest price since entry (for short trailing stop)
}

// TrendIndicators holds all calculated trend indicators
type TrendIndicators struct {
	FastEMA   float64   `json:"fast_ema"`
	SlowEMA   float64   `json:"slow_ema"`
	ADX       float64   `json:"adx"`
	Trend     string    `json:"trend"`    // "uptrend", "downtrend", "ranging"
	Strength  string    `json:"strength"` // "strong", "weak"
	Timestamp time.Time `json:"timestamp"`
}

// TrendSignal represents a trend-following trading signal
type TrendSignal struct {
	Timestamp    time.Time          `json:"timestamp"`
	Symbol       string             `json:"symbol"`
	Signal       string             `json:"signal"`     // "BUY", "SELL", "HOLD"
	Confidence   float64            `json:"confidence"` // 0.0 to 1.0
	Indicators   *TrendIndicators   `json:"indicators"`
	Reasoning    string             `json:"reasoning"`
	Price        float64            `json:"price"`
	StopLoss     float64            `json:"stop_loss,omitempty"`     // Calculated stop-loss price
	TakeProfit   float64            `json:"take_profit,omitempty"`   // Calculated take-profit price
	TrailingStop float64            `json:"trailing_stop,omitempty"` // Current trailing stop price
	RiskReward   float64            `json:"risk_reward,omitempty"`   // Actual risk/reward ratio
	Beliefs      map[string]*Belief `json:"beliefs,omitempty"`       // Current beliefs for transparency
}

// Belief represents a single belief in the agent's belief base
// Part of BDI (Belief-Desire-Intention) architecture
type Belief struct {
	Key        string      `json:"key"`        // Belief identifier (e.g., "trend_direction", "trend_strength")
	Value      interface{} `json:"value"`      // Belief value (can be string, number, bool, etc.)
	Confidence float64     `json:"confidence"` // Confidence level (0.0 to 1.0)
	Timestamp  time.Time   `json:"timestamp"`  // When belief was last updated
	Source     string      `json:"source"`     // Source of belief (e.g., "EMA", "ADX", "price_action")
}

// BeliefBase represents the agent's beliefs about the market
// Implements basic BDI (Belief-Desire-Intention) architecture
type BeliefBase struct {
	beliefs map[string]*Belief // Map of belief key -> belief
	mutex   sync.RWMutex       // Thread-safe access
}

// NewBeliefBase creates a new belief base
func NewBeliefBase() *BeliefBase {
	return &BeliefBase{
		beliefs: make(map[string]*Belief),
	}
}

// UpdateBelief updates or creates a belief
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

// GetBelief retrieves a belief by key
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

	beliefs := make(map[string]*Belief, len(bb.beliefs))
	for k, v := range bb.beliefs {
		beliefs[k] = v
	}
	return beliefs
}

// GetConfidence returns overall confidence (average of all beliefs)
func (bb *BeliefBase) GetConfidence() float64 {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	if len(bb.beliefs) == 0 {
		return 0.0
	}

	var total float64
	for _, belief := range bb.beliefs {
		total += belief.Confidence
	}
	return total / float64(len(bb.beliefs))
}

// NewTrendAgent creates a new trend following agent
func NewTrendAgent(config *agents.AgentConfig, log zerolog.Logger, metricsPort int) (*TrendAgent, error) {
	baseAgent := agents.NewBaseAgent(config, log, metricsPort)

	// Extract strategy configuration
	agentConfig := config.Config

	// Read NATS configuration
	natsURL := viper.GetString("nats.url")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	natsTopic := viper.GetString("communication.nats.topics.trend_signals")
	if natsTopic == "" {
		natsTopic = "agents.strategy.trend"
	}

	// Connect to NATS
	log.Info().Str("url", natsURL).Str("topic", natsTopic).Msg("Connecting to NATS")
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info().Msg("Successfully connected to NATS")

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
				Msg("LLM fallback client initialized for trend following")
		} else {
			// Create basic Client
			llmClient = llm.NewClient(primaryConfig)
			log.Info().
				Str("endpoint", primaryConfig.Endpoint).
				Str("model", primaryConfig.Model).
				Msg("LLM client initialized for trend following")
		}

		promptBuilder = llm.NewPromptBuilder(llm.AgentTypeTrend)
	} else {
		log.Info().Msg("LLM reasoning disabled - using rule-based analysis only")
	}

	// Extract EMA periods from config
	fastEMA := getIntFromConfig(agentConfig, "fast_ema_period", 9)
	slowEMA := getIntFromConfig(agentConfig, "slow_ema_period", 21)
	adxPeriod := getIntFromConfig(agentConfig, "adx_period", 14)
	adxThreshold := getFloatFromConfig(agentConfig, "adx_threshold", 25.0)
	lookback := getIntFromConfig(agentConfig, "lookback_candles", 100)

	// Extract risk management configuration
	stopLossPct := getFloatFromConfig(agentConfig, "risk_management.stop_loss_pct", 0.02)          // 2%
	takeProfitPct := getFloatFromConfig(agentConfig, "risk_management.take_profit_pct", 0.03)      // 3%
	trailingStopPct := getFloatFromConfig(agentConfig, "risk_management.trailing_stop_pct", 0.015) // 1.5%
	riskRewardRatio := getFloatFromConfig(agentConfig, "risk_management.min_risk_reward", 2.0)     // 2:1

	// Check if trailing stop is enabled (default true)
	useTrailingStop := true
	if val, ok := agentConfig["risk_management"].(map[string]interface{}); ok {
		if enabled, ok := val["use_trailing_stop"].(bool); ok {
			useTrailingStop = enabled
		}
	}

	return &TrendAgent{
		BaseAgent:       baseAgent,
		natsConn:        nc,
		natsTopic:       natsTopic,
		llmClient:       llmClient,
		promptBuilder:   promptBuilder,
		useLLM:          useLLM,
		fastEMAPeriod:   fastEMA,
		slowEMAPeriod:   slowEMA,
		adxPeriod:       adxPeriod,
		adxThreshold:    adxThreshold,
		lookbackCandles: lookback,
		stopLossPct:     stopLossPct,
		takeProfitPct:   takeProfitPct,
		trailingStopPct: trailingStopPct,
		useTrailingStop: useTrailingStop,
		riskRewardRatio: riskRewardRatio,
		beliefs:         NewBeliefBase(), // Initialize BDI belief system
		lastCrossover:   "none",
		lastSignal:      "HOLD",
	}, nil
}

// Step performs a single decision cycle - trend analysis
func (a *TrendAgent) Step(ctx context.Context) error {
	// Call parent Step to handle metrics
	if err := a.BaseAgent.Step(ctx); err != nil {
		return err
	}

	log.Debug().Msg("Executing trend following strategy step")

	// Step 1: Fetch market data
	symbols := a.getSymbolsToAnalyze()
	if len(symbols) == 0 {
		log.Warn().Msg("No symbols to analyze")
		return nil
	}

	symbol := symbols[0] // Analyze first symbol
	log.Debug().Str("symbol", symbol).Msg("Analyzing symbol for trend")

	// Fetch price data (need enough candles for EMA calculations)
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

	// Step 2: Calculate trend indicators (EMA crossover, ADX)
	indicators, err := a.calculateTrendIndicators(ctx, prices)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate trend indicators")
		return fmt.Errorf("indicator calculation failed: %w", err)
	}

	a.currentIndicators = indicators

	log.Info().
		Float64("fast_ema", indicators.FastEMA).
		Float64("slow_ema", indicators.SlowEMA).
		Float64("adx", indicators.ADX).
		Str("trend", indicators.Trend).
		Str("strength", indicators.Strength).
		Msg("Trend indicators calculated")

	// Step 2.5: Update agent beliefs (BDI architecture)
	a.updateBeliefs(symbol, indicators, currentPrice)

	// Step 3: Generate trading signal from trend analysis
	signal, err := a.generateTrendSignal(ctx, symbol, indicators, currentPrice)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate signal")
		return fmt.Errorf("signal generation failed: %w", err)
	}

	log.Info().
		Str("symbol", signal.Symbol).
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Str("reasoning", signal.Reasoning).
		Msg("Trend signal generated")

	// Update agent state
	a.lastSignal = signal.Signal
	a.lastSignalTime = signal.Timestamp

	// Step 4: Publish signal to NATS
	if err := a.publishSignal(ctx, signal); err != nil {
		log.Error().Err(err).Msg("Failed to publish signal to NATS")
	}

	return nil
}

// calculateTrendIndicators calculates EMA and ADX indicators
func (a *TrendAgent) calculateTrendIndicators(ctx context.Context, prices []float64) (*TrendIndicators, error) {
	indicators := &TrendIndicators{
		Timestamp: time.Now(),
	}

	// Calculate Fast EMA
	fastEMA, err := a.callCalculateEMA(ctx, prices, a.fastEMAPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate fast EMA: %w", err)
	}
	indicators.FastEMA = fastEMA

	// Calculate Slow EMA
	slowEMA, err := a.callCalculateEMA(ctx, prices, a.slowEMAPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate slow EMA: %w", err)
	}
	indicators.SlowEMA = slowEMA

	// Calculate ADX for trend strength
	// Note: ADX requires high, low, close data, but for now we'll use a simplified version
	// In production, fetch full OHLCV data and pass to ADX calculation
	adx, err := a.callCalculateADX(ctx, prices)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate ADX, using default")
		indicators.ADX = 0 // Default to 0 if ADX calculation fails
	} else {
		indicators.ADX = adx
	}

	// Determine trend direction from EMA crossover
	if indicators.FastEMA > indicators.SlowEMA {
		indicators.Trend = "uptrend"

		// Check if this is a new crossover
		if a.lastCrossover != "bullish" {
			log.Info().Msg("Detected bullish EMA crossover (golden cross)")
			a.lastCrossover = "bullish"
		}
	} else if indicators.FastEMA < indicators.SlowEMA {
		indicators.Trend = "downtrend"

		// Check if this is a new crossover
		if a.lastCrossover != "bearish" {
			log.Info().Msg("Detected bearish EMA crossover (death cross)")
			a.lastCrossover = "bearish"
		}
	} else {
		indicators.Trend = "ranging"
		a.lastCrossover = "none"
	}

	// Determine trend strength from ADX
	if indicators.ADX >= a.adxThreshold {
		indicators.Strength = "strong"
	} else {
		indicators.Strength = "weak"
	}

	return indicators, nil
}

// generateTrendSignal generates a trading signal based on trend indicators
// Routes to LLM-powered or rule-based analysis depending on configuration
func (a *TrendAgent) generateTrendSignal(ctx context.Context, symbol string, indicators *TrendIndicators, currentPrice float64) (*TrendSignal, error) {
	if a.useLLM && a.llmClient != nil {
		return a.generateSignalWithLLM(ctx, symbol, indicators, currentPrice)
	}
	return a.generateTrendSignalRuleBased(ctx, symbol, indicators, currentPrice)
}

// generateSignalWithLLM generates a trading signal using LLM-powered analysis
func (a *TrendAgent) generateSignalWithLLM(ctx context.Context, symbol string, indicators *TrendIndicators, currentPrice float64) (*TrendSignal, error) {
	log.Debug().Str("symbol", symbol).Msg("Generating trend following signal (LLM-powered)")

	// Build market context for LLM
	indicatorMap := make(map[string]float64)
	indicatorMap["fast_ema"] = indicators.FastEMA
	indicatorMap["slow_ema"] = indicators.SlowEMA
	indicatorMap["adx"] = indicators.ADX

	marketCtx := llm.MarketContext{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		Indicators:   indicatorMap,
		Timestamp:    time.Now(),
	}

	// Build LLM prompt
	userPrompt := a.promptBuilder.BuildTrendFollowingPrompt(marketCtx, nil) // No historical data yet
	systemPrompt := a.promptBuilder.GetSystemPrompt()

	// Call LLM with retry logic
	response, err := a.llmClient.CompleteWithRetry(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 2) // 2 retries

	if err != nil {
		log.Warn().Err(err).Msg("LLM request failed, falling back to rule-based analysis")
		return a.generateTrendSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	// Parse LLM response
	if len(response.Choices) == 0 {
		log.Warn().Msg("LLM returned no choices, falling back to rule-based analysis")
		return a.generateTrendSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	content := response.Choices[0].Message.Content

	// Parse JSON response
	var llmSignal llm.Signal
	if err := a.llmClient.ParseJSONResponse(content, &llmSignal); err != nil {
		log.Warn().Err(err).Msg("Failed to parse LLM response, falling back to rule-based analysis")
		return a.generateTrendSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	// Validate signal
	if llmSignal.Side != "BUY" && llmSignal.Side != "SELL" && llmSignal.Side != "HOLD" {
		log.Warn().Str("signal", llmSignal.Side).Msg("Invalid signal from LLM, falling back to rule-based analysis")
		return a.generateTrendSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	// Calculate risk management levels
	var stopLoss, takeProfit, riskReward, trailingStop float64
	if llmSignal.Side == "BUY" || llmSignal.Side == "SELL" {
		stopLoss = a.calculateStopLoss(currentPrice, llmSignal.Side)
		takeProfit = a.calculateTakeProfit(currentPrice, llmSignal.Side)
		riskReward = a.calculateRiskReward(currentPrice, stopLoss, takeProfit)

		// Verify risk/reward ratio
		if riskReward < a.riskRewardRatio {
			log.Debug().
				Float64("risk_reward", riskReward).
				Float64("min_required", a.riskRewardRatio).
				Msg("Risk/reward ratio too low - converting to HOLD")
			llmSignal.Side = "HOLD"
			llmSignal.Confidence = 0.3
			llmSignal.Reasoning = fmt.Sprintf("%s (but risk/reward %.2f < %.2f required)", llmSignal.Reasoning, riskReward, a.riskRewardRatio)
			stopLoss = 0
			takeProfit = 0
			riskReward = 0
		} else {
			trailingStop = a.updateTrailingStop(currentPrice, llmSignal.Side)
		}
	}

	// Create trend signal from LLM response
	trendSignal := &TrendSignal{
		Timestamp:    time.Now(),
		Symbol:       symbol,
		Signal:       llmSignal.Side,
		Confidence:   llmSignal.Confidence,
		Indicators:   indicators,
		Reasoning:    llmSignal.Reasoning,
		Price:        currentPrice,
		StopLoss:     stopLoss,
		TakeProfit:   takeProfit,
		TrailingStop: trailingStop,
		RiskReward:   riskReward,
		Beliefs:      a.beliefs.GetAllBeliefs(),
	}

	log.Info().
		Str("symbol", symbol).
		Str("signal", llmSignal.Side).
		Float64("confidence", llmSignal.Confidence).
		Str("reasoning", llmSignal.Reasoning).
		Msg("Generated LLM-powered trend signal")

	return trendSignal, nil
}

// generateTrendSignalRuleBased generates a trading signal using rule-based analysis
func (a *TrendAgent) generateTrendSignalRuleBased(ctx context.Context, symbol string, indicators *TrendIndicators, currentPrice float64) (*TrendSignal, error) {
	log.Debug().Str("symbol", symbol).Msg("Generating trend following signal (rule-based)")

	var signal string
	var confidence float64
	var reasoning string

	// Trend following strategy logic:
	// BUY: Fast EMA crosses above Slow EMA (golden cross) + strong trend (ADX > threshold)
	// SELL: Fast EMA crosses below Slow EMA (death cross) + strong trend (ADX > threshold)
	// HOLD: Weak trend (ADX < threshold) or no clear crossover

	if indicators.Strength == "strong" {
		// Strong trend detected
		switch indicators.Trend {
		case "uptrend":
			signal = "BUY"
			// Confidence based on EMA separation and ADX strength
			emaDiff := indicators.FastEMA - indicators.SlowEMA
			emaPercent := (emaDiff / indicators.SlowEMA) * 100

			// Normalize ADX to 0-1 range (ADX typically 0-100)
			adxConfidence := math.Min(indicators.ADX/100, 1.0)

			// Normalize EMA difference (assume 2% separation = full confidence)
			emaConfidence := math.Min(math.Abs(emaPercent)/2.0, 1.0)

			// Weighted average: 60% ADX, 40% EMA separation
			confidence = (adxConfidence * 0.6) + (emaConfidence * 0.4)

			reasoning = fmt.Sprintf(
				"Strong uptrend: Fast EMA (%.2f) > Slow EMA (%.2f), ADX=%.2f (>%.0f)",
				indicators.FastEMA, indicators.SlowEMA, indicators.ADX, a.adxThreshold,
			)
		case "downtrend":
			signal = "SELL"
			emaDiff := indicators.SlowEMA - indicators.FastEMA
			emaPercent := (emaDiff / indicators.SlowEMA) * 100

			adxConfidence := math.Min(indicators.ADX/100, 1.0)
			emaConfidence := math.Min(math.Abs(emaPercent)/2.0, 1.0)
			confidence = (adxConfidence * 0.6) + (emaConfidence * 0.4)

			reasoning = fmt.Sprintf(
				"Strong downtrend: Fast EMA (%.2f) < Slow EMA (%.2f), ADX=%.2f (>%.0f)",
				indicators.FastEMA, indicators.SlowEMA, indicators.ADX, a.adxThreshold,
			)
		default:
			signal = "HOLD"
			confidence = 0.3
			reasoning = "Strong trend but EMAs converged - waiting for clear direction"
		}
	} else {
		// Weak trend or ranging market
		signal = "HOLD"
		confidence = 0.2
		reasoning = fmt.Sprintf(
			"Weak trend: ADX=%.2f (<%.0f) - insufficient trend strength",
			indicators.ADX, a.adxThreshold,
		)
	}

	// Calculate risk management levels for BUY/SELL signals
	var stopLoss, takeProfit, riskReward, trailingStop float64
	if signal == "BUY" || signal == "SELL" {
		stopLoss = a.calculateStopLoss(currentPrice, signal)
		takeProfit = a.calculateTakeProfit(currentPrice, signal)
		riskReward = a.calculateRiskReward(currentPrice, stopLoss, takeProfit)

		// Verify risk/reward ratio meets minimum threshold
		if riskReward < a.riskRewardRatio {
			log.Debug().
				Float64("risk_reward", riskReward).
				Float64("min_required", a.riskRewardRatio).
				Msg("Risk/reward ratio too low - converting to HOLD")
			signal = "HOLD"
			confidence = 0.3
			reasoning = fmt.Sprintf("%s (but risk/reward %.2f < %.2f required)", reasoning, riskReward, a.riskRewardRatio)
			// Clear risk management fields for HOLD signals
			stopLoss = 0
			takeProfit = 0
			riskReward = 0
		} else {
			// Calculate trailing stop for active positions
			trailingStop = a.updateTrailingStop(currentPrice, signal)
		}
	}

	return &TrendSignal{
		Timestamp:    time.Now(),
		Symbol:       symbol,
		Signal:       signal,
		Confidence:   confidence,
		Indicators:   indicators,
		Reasoning:    reasoning,
		Price:        currentPrice,
		StopLoss:     stopLoss,
		TakeProfit:   takeProfit,
		TrailingStop: trailingStop,
		RiskReward:   riskReward,
		Beliefs:      a.beliefs.GetAllBeliefs(), // Include current beliefs for transparency
	}, nil
}

// calculateStopLoss calculates the stop-loss price based on entry price and signal direction
func (a *TrendAgent) calculateStopLoss(entryPrice float64, signal string) float64 {
	switch signal {
	case "BUY":
		// For long positions, stop-loss is below entry price
		return entryPrice * (1.0 - a.stopLossPct)
	case "SELL":
		// For short positions, stop-loss is above entry price
		return entryPrice * (1.0 + a.stopLossPct)
	}
	return 0
}

// calculateTakeProfit calculates the take-profit price based on entry price and signal direction
func (a *TrendAgent) calculateTakeProfit(entryPrice float64, signal string) float64 {
	switch signal {
	case "BUY":
		// For long positions, take-profit is above entry price
		return entryPrice * (1.0 + a.takeProfitPct)
	case "SELL":
		// For short positions, take-profit is below entry price
		return entryPrice * (1.0 - a.takeProfitPct)
	}
	return 0
}

// calculateRiskReward calculates the risk/reward ratio for a trade
func (a *TrendAgent) calculateRiskReward(entryPrice, stopLoss, takeProfit float64) float64 {
	risk := math.Abs(entryPrice - stopLoss)
	reward := math.Abs(takeProfit - entryPrice)

	if risk == 0 {
		return 0
	}

	return reward / risk
}

// updateTrailingStop updates the trailing stop-loss level based on current price
// Returns the new trailing stop price, or 0 if trailing stop is disabled or position not active
func (a *TrendAgent) updateTrailingStop(currentPrice float64, signal string) float64 {
	if !a.useTrailingStop {
		return 0
	}

	// Initialize position tracking on new signal
	if signal == "BUY" && a.lastSignal != "BUY" {
		a.entryPrice = currentPrice
		a.highestPrice = currentPrice
		a.lowestPrice = 0
		log.Debug().
			Float64("entry_price", currentPrice).
			Msg("Initialized long position for trailing stop")
	} else if signal == "SELL" && a.lastSignal != "SELL" {
		a.entryPrice = currentPrice
		a.lowestPrice = currentPrice
		a.highestPrice = 0
		log.Debug().
			Float64("entry_price", currentPrice).
			Msg("Initialized short position for trailing stop")
	}

	// Calculate trailing stop based on position type
	switch signal {
	case "BUY":
		// For long positions, track highest price and trail below it
		if currentPrice > a.highestPrice {
			a.highestPrice = currentPrice
			log.Debug().
				Float64("new_high", currentPrice).
				Msg("Updated highest price for trailing stop")
		}

		trailingStop := a.highestPrice * (1.0 - a.trailingStopPct)

		// Check if trailing stop is hit
		if currentPrice <= trailingStop {
			log.Info().
				Float64("current_price", currentPrice).
				Float64("trailing_stop", trailingStop).
				Float64("entry_price", a.entryPrice).
				Float64("profit_pct", ((currentPrice-a.entryPrice)/a.entryPrice)*100).
				Msg("Trailing stop hit for long position")
		}

		return trailingStop

	case "SELL":
		// For short positions, track lowest price and trail above it
		if a.lowestPrice == 0 || currentPrice < a.lowestPrice {
			a.lowestPrice = currentPrice
			log.Debug().
				Float64("new_low", currentPrice).
				Msg("Updated lowest price for trailing stop")
		}

		trailingStop := a.lowestPrice * (1.0 + a.trailingStopPct)

		// Check if trailing stop is hit
		if currentPrice >= trailingStop {
			log.Info().
				Float64("current_price", currentPrice).
				Float64("trailing_stop", trailingStop).
				Float64("entry_price", a.entryPrice).
				Float64("profit_pct", ((a.entryPrice-currentPrice)/a.entryPrice)*100).
				Msg("Trailing stop hit for short position")
		}

		return trailingStop
	}

	return 0
}

// resetPositionTracking resets position tracking state (called when position is closed)
func (a *TrendAgent) resetPositionTracking() {
	a.entryPrice = 0
	a.highestPrice = 0
	a.lowestPrice = 0
	log.Debug().Msg("Position tracking reset")
}

// updateBeliefs updates the agent's beliefs based on current market observations
// This implements the BDI architecture's belief update mechanism
func (a *TrendAgent) updateBeliefs(symbol string, indicators *TrendIndicators, currentPrice float64) {
	log.Debug().Msg("Updating agent beliefs from trend indicators")

	// Update trend direction belief
	trendConfidence := 0.5 // Base confidence
	switch indicators.Strength {
	case "strong":
		trendConfidence = 0.8
	case "weak":
		trendConfidence = 0.4
	}

	a.beliefs.UpdateBelief(
		"trend_direction",
		indicators.Trend, // "uptrend", "downtrend", "ranging"
		trendConfidence,
		"EMA_crossover",
	)

	// Update trend strength belief
	adxConfidence := math.Min(indicators.ADX/100.0, 1.0) // Normalize ADX (0-100) to 0-1
	a.beliefs.UpdateBelief(
		"trend_strength",
		indicators.Strength, // "strong", "weak"
		adxConfidence,
		"ADX",
	)

	// Update Fast EMA belief
	a.beliefs.UpdateBelief(
		"fast_ema",
		indicators.FastEMA,
		0.9, // EMA values are reliable
		"EMA",
	)

	// Update Slow EMA belief
	a.beliefs.UpdateBelief(
		"slow_ema",
		indicators.SlowEMA,
		0.9, // EMA values are reliable
		"EMA",
	)

	// Update ADX value belief
	a.beliefs.UpdateBelief(
		"adx_value",
		indicators.ADX,
		adxConfidence,
		"ADX",
	)

	// Update position state belief
	positionState := "none"
	switch a.lastSignal {
	case "BUY":
		positionState = "long"
	case "SELL":
		positionState = "short"
	}

	a.beliefs.UpdateBelief(
		"position_state",
		positionState,
		1.0, // Position state is always known
		"agent_state",
	)

	// Update current price belief
	a.beliefs.UpdateBelief(
		"current_price",
		currentPrice,
		1.0, // Price data is always reliable
		"market_data",
	)

	// Update symbol belief
	a.beliefs.UpdateBelief(
		"symbol",
		symbol,
		1.0,
		"config",
	)

	// Log belief update summary
	log.Debug().
		Str("trend_direction", indicators.Trend).
		Str("trend_strength", indicators.Strength).
		Float64("adx", indicators.ADX).
		Str("position_state", positionState).
		Float64("overall_confidence", a.beliefs.GetConfidence()).
		Int("belief_count", len(a.beliefs.GetAllBeliefs())).
		Msg("Beliefs updated successfully")
}

// callCalculateEMA calls the Technical Indicators MCP server to calculate EMA
func (a *TrendAgent) callCalculateEMA(ctx context.Context, prices []float64, period int) (float64, error) {
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_ema", map[string]interface{}{
		"prices": prices,
		"period": period,
	})
	if err != nil {
		return 0, fmt.Errorf("MCP call failed: %w", err)
	}

	// Check for errors
	if result.IsError {
		return 0, fmt.Errorf("MCP tool returned error")
	}

	// Try StructuredContent first (more direct)
	if result.StructuredContent != nil {
		resultMap, ok := result.StructuredContent.(map[string]interface{})
		if ok {
			value, err := extractFloat64(resultMap, "value")
			if err != nil {
				return 0, fmt.Errorf("failed to extract EMA value: %w", err)
			}
			log.Debug().Int("period", period).Float64("ema", value).Msg("EMA calculated")
			return value, nil
		}
	}

	// Fall back to parsing Content as JSON
	if len(result.Content) == 0 {
		return 0, fmt.Errorf("empty result content")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return 0, fmt.Errorf("expected TextContent, got %T", result.Content[0])
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return 0, fmt.Errorf("failed to parse JSON content: %w", err)
	}

	value, err := extractFloat64(resultMap, "value")
	if err != nil {
		return 0, fmt.Errorf("failed to extract EMA value: %w", err)
	}

	log.Debug().Int("period", period).Float64("ema", value).Msg("EMA calculated")
	return value, nil
}

// callCalculateADX calls the Technical Indicators MCP server to calculate ADX
func (a *TrendAgent) callCalculateADX(ctx context.Context, prices []float64) (float64, error) {
	// Note: Full ADX requires high, low, close data
	// For now, use a simplified version with close prices only
	// In production, fetch OHLCV and pass proper data

	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_adx", map[string]interface{}{
		"prices": prices,
		"period": a.adxPeriod,
	})
	if err != nil {
		return 0, fmt.Errorf("MCP call failed: %w", err)
	}

	// Check for errors
	if result.IsError {
		return 0, fmt.Errorf("MCP tool returned error")
	}

	// Try StructuredContent first
	if result.StructuredContent != nil {
		resultMap, ok := result.StructuredContent.(map[string]interface{})
		if ok {
			value, err := extractFloat64(resultMap, "value")
			if err != nil {
				return 0, fmt.Errorf("failed to extract ADX value: %w", err)
			}
			log.Debug().Float64("adx", value).Msg("ADX calculated")
			return value, nil
		}
	}

	// Fall back to parsing Content as JSON
	if len(result.Content) == 0 {
		return 0, fmt.Errorf("empty result content")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return 0, fmt.Errorf("expected TextContent, got %T", result.Content[0])
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return 0, fmt.Errorf("failed to parse JSON content: %w", err)
	}

	value, err := extractFloat64(resultMap, "value")
	if err != nil {
		return 0, fmt.Errorf("failed to extract ADX value: %w", err)
	}

	log.Debug().Float64("adx", value).Msg("ADX calculated")
	return value, nil
}

// fetchPriceData fetches historical price data from CoinGecko
func (a *TrendAgent) fetchPriceData(ctx context.Context, symbol string) ([]float64, float64, error) {
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
		latestPrice = price // Last price is current
	}

	// Limit to requested number of candles
	if len(prices) > a.lookbackCandles {
		prices = prices[len(prices)-a.lookbackCandles:]
	}

	return prices, latestPrice, nil
}

// publishSignal publishes a trend signal to NATS
func (a *TrendAgent) publishSignal(ctx context.Context, signal *TrendSignal) error {
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	if err := a.natsConn.Publish(a.natsTopic, data); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	log.Debug().
		Str("topic", a.natsTopic).
		Str("symbol", signal.Symbol).
		Str("signal", signal.Signal).
		Msg("Signal published to NATS")

	return nil
}

// getSymbolsToAnalyze returns symbols from config
func (a *TrendAgent) getSymbolsToAnalyze() []string {
	if len(a.symbols) > 0 {
		return a.symbols
	}

	config := a.GetConfig()
	if config == nil || config.Config == nil {
		return []string{"bitcoin"}
	}

	if symbolsRaw, ok := config.Config["symbols"].([]interface{}); ok {
		symbols := make([]string, 0, len(symbolsRaw))
		for _, s := range symbolsRaw {
			if sym, ok := s.(string); ok {
				symbols = append(symbols, sym)
			}
		}
		if len(symbols) > 0 {
			a.symbols = symbols
			return symbols
		}
	}

	return []string{"bitcoin"}
}

// Helper functions

func getIntFromConfig(config map[string]interface{}, key string, defaultVal int) int {
	if config == nil {
		return defaultVal
	}

	val, ok := config[key]
	if !ok {
		return defaultVal
	}

	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return defaultVal
	}
}

func getFloatFromConfig(config map[string]interface{}, key string, defaultVal float64) float64 {
	if config == nil {
		return defaultVal
	}

	val, ok := config[key]
	if !ok {
		return defaultVal
	}

	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	default:
		return defaultVal
	}
}

func extractFloat64(m map[string]interface{}, key string) (float64, error) {
	val, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("key '%s' not found", key)
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unsupported type %T", val)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	// Configure logging to stderr
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	viper.SetConfigName("agents")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../../../configs")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	// Extract trend agent configuration
	var agentConfig agents.AgentConfig

	trendConfig := viper.Sub("strategy_agents.trend")
	if trendConfig == nil {
		log.Fatal().Msg("Trend agent configuration not found in agents.yaml")
	}

	agentConfig.Name = trendConfig.GetString("name")
	agentConfig.Type = trendConfig.GetString("type")
	agentConfig.Version = trendConfig.GetString("version")
	agentConfig.Enabled = trendConfig.GetBool("enabled")

	stepIntervalStr := trendConfig.GetString("step_interval")
	stepInterval, err := time.ParseDuration(stepIntervalStr)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid step_interval")
	}
	agentConfig.StepInterval = stepInterval

	agentConfig.Config = trendConfig.Get("config").(map[string]interface{})

	metricsPort := viper.GetInt("global.metrics_port")
	if metricsPort == 0 {
		metricsPort = 9103
	}

	// Get MCP servers
	mcpServers := trendConfig.Get("mcp_servers")
	if mcpServers != nil {
		if servers, ok := mcpServers.([]interface{}); ok {
			agentConfig.MCPServers = make([]agents.MCPServerConfig, 0, len(servers))
			for _, srv := range servers {
				if server, ok := srv.(map[string]interface{}); ok {
					serverConfig := agents.MCPServerConfig{
						Name: server["name"].(string),
						Type: server["type"].(string),
					}

					switch serverConfig.Type {
					case "internal":
						if cmd, ok := server["command"].(string); ok {
							serverConfig.Command = cmd
						}
						if args, ok := server["args"].([]interface{}); ok {
							serverConfig.Args = make([]string, len(args))
							for i, arg := range args {
								serverConfig.Args[i] = arg.(string)
							}
						}
						if env, ok := server["env"].(map[string]interface{}); ok {
							serverConfig.Env = make(map[string]string, len(env))
							for k, v := range env {
								serverConfig.Env[k] = v.(string)
							}
						}
					case "external":
						if url, ok := server["url"].(string); ok {
							serverConfig.URL = url
						}
					}

					agentConfig.MCPServers = append(agentConfig.MCPServers, serverConfig)
				}
			}
		}
	}

	// Create agent
	agent, err := NewTrendAgent(&agentConfig, log.Logger, metricsPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create trend agent")
	}

	log.Info().
		Str("name", agentConfig.Name).
		Str("type", agentConfig.Type).
		Int("fast_ema", agent.fastEMAPeriod).
		Int("slow_ema", agent.slowEMAPeriod).
		Float64("adx_threshold", agent.adxThreshold).
		Msg("Starting trend following agent")

	// Initialize agent
	ctx := context.Background()
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run agent
	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Run(ctx)
	}()

	// Wait for shutdown or error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case err := <-errChan:
		if err != nil {
			log.Error().Err(err).Msg("Agent run error")
		}
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := agent.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Trend following agent shutdown complete")
}
