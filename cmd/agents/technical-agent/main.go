// Technical Analysis Agent
// Generates market insights using technical indicators
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

// TechnicalAgent performs technical analysis and generates trading signals
type TechnicalAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn  *nats.Conn
	natsTopic string

	// LLM client for AI-powered analysis
	llmClient       llm.LLMClient        // Interface supports both Client and FallbackClient
	promptBuilder   *llm.PromptBuilder
	useLLM          bool                 // Enable/disable LLM reasoning
	decisionTracker *llm.DecisionTracker // Track LLM decisions (optional)

	// Technical analysis configuration
	symbols           []string
	rsiConfig         map[string]interface{}
	macdConfig        map[string]interface{}
	bollingerConfig   map[string]interface{}
	emaConfig         map[string]interface{}
	adxConfig         map[string]interface{}
	lookbackPeriods   map[string]string
	confidenceWeights map[string]float64

	// Cached indicator values (most recent)
	currentIndicators *IndicatorValues

	// BDI belief system
	beliefs *BeliefBase
}

// Candlestick represents OHLCV data for a time period
type Candlestick struct {
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// IndicatorValues holds all calculated indicator values
type IndicatorValues struct {
	RSI            *RSIResult            `json:"rsi,omitempty"`
	MACD           *MACDResult           `json:"macd,omitempty"`
	BollingerBands *BollingerBandsResult `json:"bollinger_bands,omitempty"`
	EMA            map[int]float64       `json:"ema,omitempty"` // EMA by period
	ADX            *ADXResult            `json:"adx,omitempty"`
	Timestamp      time.Time             `json:"timestamp"`
}

// RSIResult represents RSI calculation result
type RSIResult struct {
	Value  float64 `json:"value"`
	Signal string  `json:"signal"` // "oversold", "overbought", "neutral"
}

// MACDResult represents MACD calculation result
type MACDResult struct {
	MACD      float64 `json:"macd"`
	Signal    float64 `json:"signal"`
	Histogram float64 `json:"histogram"`
	Crossover string  `json:"crossover"` // "bullish", "bearish", "none"
}

// BollingerBandsResult represents Bollinger Bands calculation result
type BollingerBandsResult struct {
	Upper  float64 `json:"upper"`
	Middle float64 `json:"middle"`
	Lower  float64 `json:"lower"`
	Width  float64 `json:"width"`
	Signal string  `json:"signal"` // "buy", "sell", "neutral"
}

// ADXResult represents ADX calculation result (placeholder for future)
type ADXResult struct {
	Value  float64 `json:"value"`
	Signal string  `json:"signal"` // "trending", "ranging"
}

// Belief represents a single belief in the agent's belief base
type Belief struct {
	Key        string      `json:"key"`        // Belief identifier (e.g., "market_trend", "rsi_signal")
	Value      interface{} `json:"value"`      // Belief value (can be string, number, bool, etc.)
	Confidence float64     `json:"confidence"` // Confidence level (0.0 to 1.0)
	Timestamp  time.Time   `json:"timestamp"`  // When belief was last updated
	Source     string      `json:"source"`     // Source of belief (e.g., "RSI", "MACD", "price_action")
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

// TechnicalSignal represents a trading signal generated from technical analysis
type TechnicalSignal struct {
	Timestamp  time.Time        `json:"timestamp"`
	Symbol     string           `json:"symbol"`
	Signal     string           `json:"signal"`     // "BUY", "SELL", "HOLD"
	Confidence float64          `json:"confidence"` // 0.0 to 1.0
	Indicators *IndicatorValues `json:"indicators"`
	Reasoning  string           `json:"reasoning"` // Human-readable explanation
	Price      float64          `json:"price"`     // Current price at signal generation
}

// NewTechnicalAgent creates a new technical analysis agent
func NewTechnicalAgent(config *agents.AgentConfig, log zerolog.Logger, metricsPort int) (*TechnicalAgent, error) {
	baseAgent := agents.NewBaseAgent(config, log, metricsPort)

	// Extract technical analysis configuration
	agentConfig := config.Config

	// Read NATS configuration
	natsURL := viper.GetString("nats.url")
	if natsURL == "" {
		natsURL = "nats://localhost:4222" // Default
	}

	natsTopic := viper.GetString("communication.nats.topics.technical_signals")
	if natsTopic == "" {
		natsTopic = "agents.analysis.technical" // Default
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
				Msg("LLM fallback client initialized for technical analysis")
		} else {
			// Create basic Client
			llmClient = llm.NewClient(primaryConfig)
			log.Info().
				Str("endpoint", primaryConfig.Endpoint).
				Str("model", primaryConfig.Model).
				Msg("LLM client initialized for technical analysis")
		}

		promptBuilder = llm.NewPromptBuilder(llm.AgentTypeTechnical)
	} else {
		log.Info().Msg("LLM reasoning disabled - using rule-based analysis only")
	}

	return &TechnicalAgent{
		BaseAgent:         baseAgent,
		natsConn:          nc,
		natsTopic:         natsTopic,
		llmClient:         llmClient,
		promptBuilder:     promptBuilder,
		useLLM:            useLLM,
		rsiConfig:         getMapConfig(agentConfig, "indicators.rsi"),
		macdConfig:        getMapConfig(agentConfig, "indicators.macd"),
		bollingerConfig:   getMapConfig(agentConfig, "indicators.bollinger"),
		emaConfig:         getMapConfig(agentConfig, "indicators.ema"),
		adxConfig:         getMapConfig(agentConfig, "indicators.adx"),
		lookbackPeriods:   getMapStringConfig(agentConfig, "lookback_periods"),
		confidenceWeights: getMapFloatConfig(agentConfig, "confidence_weights"),
		beliefs:           NewBeliefBase(),
	}, nil
}

// updateBeliefs updates the agent's beliefs based on current market observations
// This implements the BDI architecture's belief update mechanism
func (a *TechnicalAgent) updateBeliefs(symbol string, indicators *IndicatorValues, currentPrice float64) {
	log.Debug().Msg("Updating agent beliefs from market observations")

	// Update market trend belief from multiple indicators
	var trendSignals []string
	var trendConfidence float64

	// RSI signal contributes to trend belief
	if indicators.RSI != nil {
		a.beliefs.UpdateBelief(
			"rsi_signal",
			indicators.RSI.Signal,
			calculateRSIConfidence(indicators.RSI.Value),
			"RSI",
		)
		trendSignals = append(trendSignals, indicators.RSI.Signal)
		trendConfidence += calculateRSIConfidence(indicators.RSI.Value) * a.confidenceWeights["rsi"]
	}

	// MACD signal contributes to trend belief
	if indicators.MACD != nil {
		a.beliefs.UpdateBelief(
			"macd_signal",
			indicators.MACD.Crossover,
			calculateMACDConfidence(indicators.MACD.Histogram),
			"MACD",
		)
		trendSignals = append(trendSignals, indicators.MACD.Crossover)
		trendConfidence += calculateMACDConfidence(indicators.MACD.Histogram) * a.confidenceWeights["macd"]
	}

	// Bollinger Bands signal contributes to trend belief
	if indicators.BollingerBands != nil {
		a.beliefs.UpdateBelief(
			"bollinger_signal",
			indicators.BollingerBands.Signal,
			calculateBollingerConfidence(currentPrice, indicators.BollingerBands),
			"Bollinger Bands",
		)
		trendSignals = append(trendSignals, indicators.BollingerBands.Signal)
		trendConfidence += calculateBollingerConfidence(currentPrice, indicators.BollingerBands) * a.confidenceWeights["bollinger"]
	}

	// ADX signal contributes to trend strength belief
	if indicators.ADX != nil {
		a.beliefs.UpdateBelief(
			"trend_strength",
			indicators.ADX.Signal,
			indicators.ADX.Value/100.0, // Normalize ADX (0-100) to 0-1
			"ADX",
		)
	}

	// Aggregate market trend belief
	var overallTrend string
	buyCount := 0
	sellCount := 0
	for _, signal := range trendSignals {
		if signal == "buy" {
			buyCount++
		} else if signal == "sell" {
			sellCount++
		}
	}

	if buyCount > sellCount {
		overallTrend = "bullish"
	} else if sellCount > buyCount {
		overallTrend = "bearish"
	} else {
		overallTrend = "neutral"
	}

	// Normalize trend confidence to 0-1 range
	if trendConfidence > 1.0 {
		trendConfidence = 1.0
	}

	a.beliefs.UpdateBelief(
		"market_trend",
		overallTrend,
		trendConfidence,
		"aggregated_indicators",
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
		Str("market_trend", overallTrend).
		Float64("trend_confidence", trendConfidence).
		Float64("overall_confidence", a.beliefs.GetConfidence()).
		Int("belief_count", len(a.beliefs.GetAllBeliefs())).
		Msg("Beliefs updated successfully")
}

// calculateRSIConfidence calculates confidence from RSI value
func calculateRSIConfidence(rsi float64) float64 {
	// RSI extremes (0-30, 70-100) have higher confidence
	if rsi <= 30 {
		return (30 - rsi) / 30 // 0-30: 0.0 to 1.0
	} else if rsi >= 70 {
		return (rsi - 70) / 30 // 70-100: 0.0 to 1.0
	}
	// Neutral zone has lower confidence
	return 0.3
}

// calculateMACDConfidence calculates confidence from MACD histogram
func calculateMACDConfidence(histogram float64) float64 {
	// Larger histogram absolute value = higher confidence
	absHist := histogram
	if absHist < 0 {
		absHist = -absHist
	}
	// Normalize: 0.1 histogram = full confidence
	confidence := absHist / 0.1
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// calculateBollingerConfidence calculates confidence from Bollinger position
func calculateBollingerConfidence(price float64, bb *BollingerBandsResult) float64 {
	// Price near bands = higher confidence
	width := bb.Upper - bb.Lower
	if width == 0 {
		return 0.3
	}

	// Distance from middle band
	distanceFromMiddle := price - bb.Middle
	if distanceFromMiddle < 0 {
		distanceFromMiddle = -distanceFromMiddle
	}

	// Normalize to band width
	relativeDistance := distanceFromMiddle / (width / 2)

	// Closer to bands = higher confidence (0.0 at middle, 1.0 at bands)
	return relativeDistance
}

// Step performs a single decision cycle - technical analysis
func (a *TechnicalAgent) Step(ctx context.Context) error {
	// Call parent Step to handle metrics
	if err := a.BaseAgent.Step(ctx); err != nil {
		return err
	}

	log.Debug().Msg("Executing technical analysis step")

	// Step 1: Fetch market data from CoinGecko
	symbols := a.getSymbolsToAnalyze()
	if len(symbols) == 0 {
		log.Warn().Msg("No symbols to analyze")
		return nil
	}

	// For now, analyze first symbol (multi-symbol support can be added later)
	symbol := symbols[0]
	log.Debug().Str("symbol", symbol).Msg("Analyzing symbol")

	// Fetch candlestick data (50 hourly candles for indicator calculations)
	// 50 periods is sufficient for most indicators (RSI=14, MACD=26, Bollinger=20, EMA=50)
	candlesticks, err := a.fetchCandlesticks(ctx, symbol, "hourly", 50)
	if err != nil {
		log.Error().Err(err).Str("symbol", symbol).Msg("Failed to fetch candlesticks")
		return fmt.Errorf("failed to fetch market data: %w", err)
	}

	if len(candlesticks) == 0 {
		log.Warn().Str("symbol", symbol).Msg("No candlestick data available")
		return nil
	}

	// Extract close prices for indicator calculations
	prices := make([]float64, len(candlesticks))
	for i, candle := range candlesticks {
		prices[i] = candle.Close
	}

	log.Debug().
		Str("symbol", symbol).
		Int("price_count", len(prices)).
		Float64("latest_price", prices[len(prices)-1]).
		Msg("Retrieved price data from CoinGecko")

	// Step 2: Calculate all technical indicators
	indicators, err := a.calculateIndicators(ctx, prices)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate indicators")
		return fmt.Errorf("indicator calculation failed: %w", err)
	}

	// Store calculated indicators
	a.currentIndicators = indicators

	// Step 2.5: Update agent beliefs from observations (BDI architecture)
	a.updateBeliefs(symbol, indicators, prices[len(prices)-1])

	// Log indicator summary
	log.Info().
		Interface("rsi", indicators.RSI).
		Interface("macd", indicators.MACD).
		Interface("bollinger", indicators.BollingerBands).
		Msg("Indicators calculated successfully")

	// Step 3: Generate trading signal from indicators
	signal, err := a.generateSignal(ctx, symbol, indicators, prices[len(prices)-1])
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate signal")
		return fmt.Errorf("signal generation failed: %w", err)
	}

	// Log the generated signal
	log.Info().
		Str("symbol", signal.Symbol).
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Str("reasoning", signal.Reasoning).
		Msg("Technical signal generated")

	// Step 4: Publish signal to NATS
	if err := a.publishSignal(ctx, signal); err != nil {
		log.Error().Err(err).Msg("Failed to publish signal to NATS")
		// Don't fail the step - log and continue
		// Signal was still generated successfully
	}

	return nil
}

// calculateIndicators calls MCP server tools to calculate all technical indicators
func (a *TechnicalAgent) calculateIndicators(ctx context.Context, prices []float64) (*IndicatorValues, error) {
	indicators := &IndicatorValues{
		Timestamp: time.Now(),
		EMA:       make(map[int]float64),
	}

	// Calculate RSI
	rsi, err := a.calculateRSI(ctx, prices)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate RSI")
		// Continue with other indicators even if one fails
	} else {
		indicators.RSI = rsi
	}

	// Calculate MACD
	macd, err := a.calculateMACD(ctx, prices)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate MACD")
	} else {
		indicators.MACD = macd
	}

	// Calculate Bollinger Bands
	bollinger, err := a.calculateBollingerBands(ctx, prices)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate Bollinger Bands")
	} else {
		indicators.BollingerBands = bollinger
	}

	// Calculate EMA for configured periods
	if emaPeriods, ok := a.emaConfig["periods"].([]interface{}); ok {
		for _, p := range emaPeriods {
			var period int
			switch v := p.(type) {
			case int:
				period = v
			case float64:
				period = int(v)
			default:
				continue
			}

			ema, err := a.calculateEMA(ctx, prices, period)
			if err != nil {
				log.Warn().Err(err).Int("period", period).Msg("Failed to calculate EMA")
			} else {
				indicators.EMA[period] = ema
			}
		}
	}

	// ADX calculation requires high, low, close data - skip for now
	// Will be implemented when we have full OHLCV data

	return indicators, nil
}

// calculateRSI calls the calculate_rsi MCP tool
func (a *TechnicalAgent) calculateRSI(ctx context.Context, prices []float64) (*RSIResult, error) {
	period := getIntFromConfig(a.rsiConfig, "period", 14)

	// Call MCP tool
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_rsi", map[string]interface{}{
		"prices": prices,
		"period": period,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP call failed: %w", err)
	}

	// Parse result
	rsi, err := parseRSIResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSI result: %w", err)
	}

	log.Debug().
		Float64("value", rsi.Value).
		Str("signal", rsi.Signal).
		Msg("RSI calculated")

	return rsi, nil
}

// calculateMACD calls the calculate_macd MCP tool
func (a *TechnicalAgent) calculateMACD(ctx context.Context, prices []float64) (*MACDResult, error) {
	fastPeriod := getIntFromConfig(a.macdConfig, "fast_period", 12)
	slowPeriod := getIntFromConfig(a.macdConfig, "slow_period", 26)
	signalPeriod := getIntFromConfig(a.macdConfig, "signal_period", 9)

	// Call MCP tool
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_macd", map[string]interface{}{
		"prices":        prices,
		"fast_period":   fastPeriod,
		"slow_period":   slowPeriod,
		"signal_period": signalPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP call failed: %w", err)
	}

	// Parse result
	macd, err := parseMACDResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MACD result: %w", err)
	}

	log.Debug().
		Float64("macd", macd.MACD).
		Float64("signal", macd.Signal).
		Str("crossover", macd.Crossover).
		Msg("MACD calculated")

	return macd, nil
}

// calculateBollingerBands calls the calculate_bollinger_bands MCP tool
func (a *TechnicalAgent) calculateBollingerBands(ctx context.Context, prices []float64) (*BollingerBandsResult, error) {
	period := getIntFromConfig(a.bollingerConfig, "period", 20)
	stdDev := getFloatFromConfig(a.bollingerConfig, "std_dev", 2.0)

	// Call MCP tool
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_bollinger_bands", map[string]interface{}{
		"prices":  prices,
		"period":  period,
		"std_dev": stdDev,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP call failed: %w", err)
	}

	// Parse result
	bollinger, err := parseBollingerBandsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Bollinger Bands result: %w", err)
	}

	log.Debug().
		Float64("upper", bollinger.Upper).
		Float64("middle", bollinger.Middle).
		Float64("lower", bollinger.Lower).
		Str("signal", bollinger.Signal).
		Msg("Bollinger Bands calculated")

	return bollinger, nil
}

// calculateEMA calls the calculate_ema MCP tool
func (a *TechnicalAgent) calculateEMA(ctx context.Context, prices []float64, period int) (float64, error) {
	// Call MCP tool
	result, err := a.CallMCPTool(ctx, "technical_indicators", "calculate_ema", map[string]interface{}{
		"prices": prices,
		"period": period,
	})
	if err != nil {
		return 0, fmt.Errorf("MCP call failed: %w", err)
	}

	// Parse result - EMA returns a single float64 value
	emaValue, err := parseEMAResult(result)
	if err != nil {
		return 0, fmt.Errorf("failed to parse EMA result: %w", err)
	}

	log.Debug().
		Int("period", period).
		Float64("value", emaValue).
		Msg("EMA calculated")

	return emaValue, nil
}

// generateSignalWithLLM uses LLM to analyze indicators and generate a trading signal
func (a *TechnicalAgent) generateSignalWithLLM(ctx context.Context, symbol string, indicators *IndicatorValues, currentPrice float64) (*TechnicalSignal, error) {
	log.Debug().Str("symbol", symbol).Msg("Generating LLM-powered trading signal")

	// Build market context for LLM
	indicatorMap := make(map[string]float64)
	if indicators.RSI != nil {
		indicatorMap["RSI"] = indicators.RSI.Value
	}
	if indicators.MACD != nil {
		indicatorMap["MACD"] = indicators.MACD.MACD
		indicatorMap["MACD_Signal"] = indicators.MACD.Signal
		indicatorMap["MACD_Histogram"] = indicators.MACD.Histogram
	}
	if indicators.BollingerBands != nil {
		indicatorMap["Bollinger_Upper"] = indicators.BollingerBands.Upper
		indicatorMap["Bollinger_Middle"] = indicators.BollingerBands.Middle
		indicatorMap["Bollinger_Lower"] = indicators.BollingerBands.Lower
		indicatorMap["Bollinger_Width"] = indicators.BollingerBands.Width
	}
	for period, value := range indicators.EMA {
		indicatorMap[fmt.Sprintf("EMA_%d", period)] = value
	}

	marketCtx := llm.MarketContext{
		Symbol:         symbol,
		CurrentPrice:   currentPrice,
		PriceChange24h: 0, // Could be calculated from historical data
		Volume24h:      0, // Could be fetched from market data
		Indicators:     indicatorMap,
		Timestamp:      time.Now(),
	}

	// Build prompt
	userPrompt := a.promptBuilder.BuildTechnicalAnalysisPrompt(marketCtx)
	systemPrompt := a.promptBuilder.GetSystemPrompt()

	// Call LLM with retry
	response, err := a.llmClient.CompleteWithRetry(ctx, []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 2) // Max 2 retries

	if err != nil {
		log.Error().Err(err).Msg("LLM call failed")
		// Fall back to rule-based signal generation
		return a.generateSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	if len(response.Choices) == 0 {
		log.Warn().Msg("No choices in LLM response, falling back to rule-based")
		return a.generateSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	// Parse LLM response
	content := response.Choices[0].Message.Content
	var llmSignal llm.Signal
	if err := a.llmClient.ParseJSONResponse(content, &llmSignal); err != nil {
		log.Error().Err(err).Str("content", content).Msg("Failed to parse LLM response")
		// Fall back to rule-based signal generation
		return a.generateSignalRuleBased(ctx, symbol, indicators, currentPrice)
	}

	// Convert LLM signal to TechnicalSignal
	signal := &TechnicalSignal{
		Timestamp:  time.Now(),
		Symbol:     symbol,
		Signal:     llmSignal.Side, // "BUY", "SELL", or use action field
		Confidence: llmSignal.Confidence,
		Indicators: indicators,
		Reasoning:  llmSignal.Reasoning,
		Price:      currentPrice,
	}

	// Normalize signal if needed
	if signal.Signal != "BUY" && signal.Signal != "SELL" && signal.Signal != "HOLD" {
		// Check if it's in the action field instead
		if llmSignal.Metadata != nil {
			if action, ok := llmSignal.Metadata["action"].(string); ok {
				signal.Signal = action
			}
		}
	}

	// Ensure signal is valid
	if signal.Signal != "BUY" && signal.Signal != "SELL" && signal.Signal != "HOLD" {
		log.Warn().Str("signal", signal.Signal).Msg("Invalid signal from LLM, defaulting to HOLD")
		signal.Signal = "HOLD"
	}

	log.Info().
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Str("reasoning", signal.Reasoning).
		Msg("LLM signal generated successfully")

	return signal, nil
}

// generateSignal combines indicator signals with confidence weights to produce a trading signal
// This method now routes to LLM or rule-based generation depending on configuration
func (a *TechnicalAgent) generateSignal(ctx context.Context, symbol string, indicators *IndicatorValues, currentPrice float64) (*TechnicalSignal, error) {
	// Use LLM if enabled
	if a.useLLM && a.llmClient != nil {
		return a.generateSignalWithLLM(ctx, symbol, indicators, currentPrice)
	}

	// Fall back to rule-based generation
	return a.generateSignalRuleBased(ctx, symbol, indicators, currentPrice)
}

// generateSignalRuleBased is the original rule-based signal generation logic
func (a *TechnicalAgent) generateSignalRuleBased(ctx context.Context, symbol string, indicators *IndicatorValues, currentPrice float64) (*TechnicalSignal, error) {
	log.Debug().Str("symbol", symbol).Msg("Generating rule-based trading signal from indicators")

	// Get confidence weights from agent's stored config
	confidenceWeights := a.confidenceWeights
	if len(confidenceWeights) == 0 {
		// Fallback to defaults if not configured
		confidenceWeights = map[string]float64{
			"rsi":       0.25,
			"macd":      0.25,
			"bollinger": 0.20,
			"trend":     0.20,
			"volume":    0.10,
		}
	}

	// Convert to map[string]interface{} for helper function
	confidenceWeightsInterface := make(map[string]interface{})
	for k, v := range confidenceWeights {
		confidenceWeightsInterface[k] = v
	}

	// Individual indicator signals and confidences
	var signals []string
	var confidences []float64
	var weights []float64
	var reasoningParts []string

	// 1. RSI Signal Analysis
	if indicators.RSI != nil {
		rsiWeight := getFloat64FromConfig(confidenceWeightsInterface, "rsi", 0.25)
		rsiSignal, rsiConfidence, rsiReason := analyzeRSI(indicators.RSI, a.rsiConfig)
		signals = append(signals, rsiSignal)
		confidences = append(confidences, rsiConfidence)
		weights = append(weights, rsiWeight)
		reasoningParts = append(reasoningParts, rsiReason)
	}

	// 2. MACD Signal Analysis
	if indicators.MACD != nil {
		macdWeight := getFloat64FromConfig(confidenceWeightsInterface, "macd", 0.25)
		macdSignal, macdConfidence, macdReason := analyzeMACD(indicators.MACD)
		signals = append(signals, macdSignal)
		confidences = append(confidences, macdConfidence)
		weights = append(weights, macdWeight)
		reasoningParts = append(reasoningParts, macdReason)
	}

	// 3. Bollinger Bands Signal Analysis
	if indicators.BollingerBands != nil {
		bollingerWeight := getFloat64FromConfig(confidenceWeightsInterface, "bollinger", 0.20)
		bollingerSignal, bollingerConfidence, bollingerReason := analyzeBollingerBands(indicators.BollingerBands)
		signals = append(signals, bollingerSignal)
		confidences = append(confidences, bollingerConfidence)
		weights = append(weights, bollingerWeight)
		reasoningParts = append(reasoningParts, bollingerReason)
	}

	// 4. EMA Trend Analysis
	if len(indicators.EMA) >= 2 {
		trendWeight := getFloat64FromConfig(confidenceWeightsInterface, "trend", 0.20)
		trendSignal, trendConfidence, trendReason := analyzeEMATrend(indicators.EMA)
		signals = append(signals, trendSignal)
		confidences = append(confidences, trendConfidence)
		weights = append(weights, trendWeight)
		reasoningParts = append(reasoningParts, trendReason)
	}

	// 5. Combine signals with weighted confidence
	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	// Build reasoning string
	reasoning := strings.Join(reasoningParts, "; ")

	// Create technical signal
	signal := &TechnicalSignal{
		Timestamp:  time.Now(),
		Symbol:     symbol,
		Signal:     finalSignal,
		Confidence: finalConfidence,
		Indicators: indicators,
		Reasoning:  reasoning,
		Price:      currentPrice,
	}

	log.Debug().
		Str("signal", finalSignal).
		Float64("confidence", finalConfidence).
		Msg("Signal generated")

	return signal, nil
}

// analyzeRSI interprets RSI values to generate a signal
func analyzeRSI(rsi *RSIResult, config map[string]interface{}) (signal string, confidence float64, reasoning string) {
	overbought := getFloat64FromConfig(config, "overbought", 70.0)
	oversold := getFloat64FromConfig(config, "oversold", 30.0)

	if rsi.Value <= oversold {
		// Oversold - BUY signal
		intensity := (oversold - rsi.Value) / oversold // How far below oversold
		confidence = 0.5 + (intensity * 0.5)           // 0.5 to 1.0
		if confidence > 1.0 {
			confidence = 1.0
		}
		return "BUY", confidence, fmt.Sprintf("RSI oversold at %.2f (<%d)", rsi.Value, int(oversold))
	} else if rsi.Value >= overbought {
		// Overbought - SELL signal
		intensity := (rsi.Value - overbought) / (100 - overbought)
		confidence = 0.5 + (intensity * 0.5)
		if confidence > 1.0 {
			confidence = 1.0
		}
		return "SELL", confidence, fmt.Sprintf("RSI overbought at %.2f (>%d)", rsi.Value, int(overbought))
	} else {
		// Neutral zone - HOLD
		// Confidence decreases as RSI approaches extremes
		distanceFromOversold := rsi.Value - oversold
		distanceFromOverbought := overbought - rsi.Value
		minDistance := distanceFromOversold
		if distanceFromOverbought < minDistance {
			minDistance = distanceFromOverbought
		}
		confidence = minDistance / ((overbought - oversold) / 2) // 0 to 1
		if confidence > 1.0 {
			confidence = 1.0
		}
		return "HOLD", confidence, fmt.Sprintf("RSI neutral at %.2f", rsi.Value)
	}
}

// analyzeMACD interprets MACD crossovers to generate a signal
func analyzeMACD(macd *MACDResult) (signal string, confidence float64, reasoning string) {
	// MACD line crossing above signal line = bullish (BUY)
	// MACD line crossing below signal line = bearish (SELL)
	diff := macd.MACD - macd.Signal

	if diff > 0 && macd.Histogram > 0 {
		// Bullish: MACD above signal line with positive histogram
		confidence = math.Min(math.Abs(macd.Histogram)*10, 1.0) // Scale histogram to confidence
		return "BUY", confidence, fmt.Sprintf("MACD bullish crossover (MACD:%.4f > Signal:%.4f)", macd.MACD, macd.Signal)
	} else if diff < 0 && macd.Histogram < 0 {
		// Bearish: MACD below signal line with negative histogram
		confidence = math.Min(math.Abs(macd.Histogram)*10, 1.0)
		return "SELL", confidence, fmt.Sprintf("MACD bearish crossover (MACD:%.4f < Signal:%.4f)", macd.MACD, macd.Signal)
	} else {
		// No clear crossover - HOLD
		confidence = 0.3 // Low confidence in neutral state
		return "HOLD", confidence, fmt.Sprintf("MACD neutral (MACD:%.4f, Signal:%.4f)", macd.MACD, macd.Signal)
	}
}

// analyzeBollingerBands interprets Bollinger Band position to generate a signal
func analyzeBollingerBands(bb *BollingerBandsResult) (signal string, confidence float64, reasoning string) {
	// Bollinger Bands already provide a signal from the MCP server
	// "buy": price near lower band (oversold)
	// "sell": price near upper band (overbought)
	// "neutral": price in middle range

	switch bb.Signal {
	case "buy":
		// Calculate confidence based on how close to lower band
		bandWidth := bb.Upper - bb.Lower
		distanceFromLower := bb.Middle - bb.Lower
		confidence = 1.0 - (distanceFromLower / bandWidth)
		if confidence < 0.5 {
			confidence = 0.5
		}
		return "BUY", confidence, fmt.Sprintf("Price near lower Bollinger Band (%.2f)", bb.Lower)
	case "sell":
		bandWidth := bb.Upper - bb.Lower
		distanceFromUpper := bb.Upper - bb.Middle
		confidence = 1.0 - (distanceFromUpper / bandWidth)
		if confidence < 0.5 {
			confidence = 0.5
		}
		return "SELL", confidence, fmt.Sprintf("Price near upper Bollinger Band (%.2f)", bb.Upper)
	default:
		return "HOLD", 0.5, "Price in middle Bollinger Band range"
	}
}

// analyzeEMATrend analyzes EMA crossovers to determine trend
func analyzeEMATrend(emas map[int]float64) (signal string, confidence float64, reasoning string) {
	// Get short-term and long-term EMAs (9 and 50 are common)
	ema9, has9 := emas[9]
	ema21, has21 := emas[21]
	ema50, has50 := emas[50]
	ema200, has200 := emas[200]

	// Use the most reliable pairing available
	var shortEMA, longEMA float64
	var shortPeriod, longPeriod int

	if has9 && has50 {
		shortEMA, longEMA = ema9, ema50
		shortPeriod, longPeriod = 9, 50
	} else if has9 && has21 {
		shortEMA, longEMA = ema9, ema21
		shortPeriod, longPeriod = 9, 21
	} else if has21 && has50 {
		shortEMA, longEMA = ema21, ema50
		shortPeriod, longPeriod = 21, 50
	} else if has50 && has200 {
		shortEMA, longEMA = ema50, ema200
		shortPeriod, longPeriod = 50, 200
	} else {
		return "HOLD", 0.3, "Insufficient EMA data for trend analysis"
	}

	// Calculate crossover strength
	diff := shortEMA - longEMA
	percentDiff := (diff / longEMA) * 100

	if diff > 0 {
		// Short EMA above long EMA = uptrend (BUY)
		confidence = math.Min(math.Abs(percentDiff)*0.5, 1.0)
		if confidence < 0.5 {
			confidence = 0.5
		}
		return "BUY", confidence, fmt.Sprintf("EMA%d (%.2f) > EMA%d (%.2f) - uptrend", shortPeriod, shortEMA, longPeriod, longEMA)
	} else if diff < 0 {
		// Short EMA below long EMA = downtrend (SELL)
		confidence = math.Min(math.Abs(percentDiff)*0.5, 1.0)
		if confidence < 0.5 {
			confidence = 0.5
		}
		return "SELL", confidence, fmt.Sprintf("EMA%d (%.2f) < EMA%d (%.2f) - downtrend", shortPeriod, shortEMA, longPeriod, longEMA)
	} else {
		return "HOLD", 0.3, "EMAs converged - no clear trend"
	}
}

// combineSignals aggregates individual signals with weighted confidence
func combineSignals(signals []string, confidences []float64, weights []float64) (finalSignal string, finalConfidence float64) {
	if len(signals) == 0 {
		return "HOLD", 0.0
	}

	// Calculate weighted scores for each signal type
	buyScore := 0.0
	sellScore := 0.0
	holdScore := 0.0
	totalWeight := 0.0

	for i, signal := range signals {
		weightedConfidence := confidences[i] * weights[i]
		totalWeight += weights[i]

		switch signal {
		case "BUY":
			buyScore += weightedConfidence
		case "SELL":
			sellScore += weightedConfidence
		case "HOLD":
			holdScore += weightedConfidence
		}
	}

	// Normalize scores
	if totalWeight > 0 {
		buyScore /= totalWeight
		sellScore /= totalWeight
		holdScore /= totalWeight
	}

	// Determine final signal based on highest score
	// Use >= to handle ties properly, with conservative bias: HOLD > SELL > BUY
	maxScore := buyScore
	finalSignal = "BUY"

	if sellScore >= maxScore {
		maxScore = sellScore
		finalSignal = "SELL"
	}

	if holdScore >= maxScore {
		maxScore = holdScore
		finalSignal = "HOLD"
	}

	// Final confidence is the winning score
	finalConfidence = maxScore

	return finalSignal, finalConfidence
}

// publishSignal publishes a technical signal to NATS for other agents to consume
func (a *TechnicalAgent) publishSignal(ctx context.Context, signal *TechnicalSignal) error {
	// Marshal signal to JSON
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	// Publish to NATS
	if err := a.natsConn.Publish(a.natsTopic, data); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	log.Debug().
		Str("topic", a.natsTopic).
		Str("symbol", signal.Symbol).
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Msg("Signal published to NATS")

	return nil
}

// getFloat64FromConfig extracts a float64 value from config map
func getFloat64FromConfig(config map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return defaultValue
}

// fetchCurrentPrice fetches current price for a symbol from CoinGecko
func (a *TechnicalAgent) fetchCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	log.Debug().Str("symbol", symbol).Msg("Fetching current price from CoinGecko")

	// Call CoinGecko MCP tool
	result, err := a.CallMCPTool(ctx, "coingecko", "get_price", map[string]interface{}{
		"ids":           symbol,
		"vs_currencies": "usd",
	})
	if err != nil {
		return 0, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from MCP result
	if len(result.Content) == 0 {
		return 0, fmt.Errorf("empty result from CoinGecko")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return 0, fmt.Errorf("invalid content type from CoinGecko")
	}

	// Parse JSON result - CoinGecko returns {symbol: {usd: price}}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return 0, fmt.Errorf("failed to parse CoinGecko response: %w", err)
	}

	symbolData, ok := resultMap[symbol].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("symbol data not found for %s", symbol)
	}

	price, err := extractFloat64(symbolData, "usd")
	if err != nil {
		return 0, fmt.Errorf("failed to extract price: %w", err)
	}

	log.Debug().Str("symbol", symbol).Float64("price", price).Msg("Price fetched")
	return price, nil
}

// fetchCandlesticks fetches historical OHLCV data from CoinGecko
func (a *TechnicalAgent) fetchCandlesticks(ctx context.Context, symbol string, interval string, limit int) ([]Candlestick, error) {
	log.Debug().
		Str("symbol", symbol).
		Str("interval", interval).
		Int("limit", limit).
		Msg("Fetching candlesticks from CoinGecko")

	// Calculate days needed based on interval and limit
	days := a.calculateDaysForInterval(interval, limit)

	// Call CoinGecko MCP tool
	result, err := a.CallMCPTool(ctx, "coingecko", "get_market_chart", map[string]interface{}{
		"id":          symbol,
		"vs_currency": "usd",
		"days":        days,
		"interval":    interval,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from MCP result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from CoinGecko")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type from CoinGecko")
	}

	// Parse JSON result - CoinGecko returns {prices: [[timestamp, price], ...], total_volumes: [[timestamp, volume], ...]}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return nil, fmt.Errorf("failed to parse CoinGecko response: %w", err)
	}

	// Convert to candlesticks
	candlesticks, err := a.convertToCandlesticks(resultMap, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to candlesticks: %w", err)
	}

	log.Debug().
		Str("symbol", symbol).
		Int("count", len(candlesticks)).
		Msg("Candlesticks fetched")

	return candlesticks, nil
}

// calculateDaysForInterval determines how many days of data to request
func (a *TechnicalAgent) calculateDaysForInterval(interval string, limit int) int {
	// CoinGecko intervals: "5m", "hourly", "daily"
	switch interval {
	case "5m":
		// 5-minute candles: need ~12 per hour, 288 per day
		return max(1, (limit+287)/288)
	case "hourly":
		// Hourly candles: 24 per day
		return max(1, (limit+23)/24)
	case "daily":
		// Daily candles: 1 per day
		return max(1, limit)
	default:
		// Default to hourly
		return max(1, (limit+23)/24)
	}
}

// convertToCandlesticks converts CoinGecko price/volume data to OHLCV candlesticks
func (a *TechnicalAgent) convertToCandlesticks(data map[string]interface{}, limit int) ([]Candlestick, error) {
	// Extract prices array
	pricesRaw, ok := data["prices"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("prices field not found or invalid type")
	}

	// Extract volumes array
	volumesRaw, ok := data["total_volumes"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("total_volumes field not found or invalid type")
	}

	// Parse price data points
	type pricePoint struct {
		timestamp int64
		price     float64
	}

	pricePoints := make([]pricePoint, 0, len(pricesRaw))
	for _, p := range pricesRaw {
		point, ok := p.([]interface{})
		if !ok || len(point) != 2 {
			continue
		}

		timestamp, ok := point[0].(float64)
		if !ok {
			continue
		}

		price, ok := point[1].(float64)
		if !ok {
			continue
		}

		pricePoints = append(pricePoints, pricePoint{
			timestamp: int64(timestamp),
			price:     price,
		})
	}

	// Parse volume data points
	volumeMap := make(map[int64]float64)
	for _, v := range volumesRaw {
		point, ok := v.([]interface{})
		if !ok || len(point) != 2 {
			continue
		}

		timestamp, ok := point[0].(float64)
		if !ok {
			continue
		}

		volume, ok := point[1].(float64)
		if !ok {
			continue
		}

		volumeMap[int64(timestamp)] = volume
	}

	// Group price points into candlesticks by time interval
	// For simplicity, we'll create one candlestick per data point
	// In a production system, you'd aggregate multiple points into proper OHLCV bars
	candlesticks := make([]Candlestick, 0, len(pricePoints))
	for _, pp := range pricePoints {
		candlestick := Candlestick{
			Timestamp: pp.timestamp,
			Open:      pp.price,
			High:      pp.price,
			Low:       pp.price,
			Close:     pp.price,
			Volume:    volumeMap[pp.timestamp],
		}
		candlesticks = append(candlesticks, candlestick)
	}

	// Limit to requested number of candlesticks (most recent)
	if len(candlesticks) > limit {
		candlesticks = candlesticks[len(candlesticks)-limit:]
	}

	return candlesticks, nil
}

// getSymbolsToAnalyze returns list of symbols to analyze from config
func (a *TechnicalAgent) getSymbolsToAnalyze() []string {
	// If symbols are already cached, return them
	if len(a.symbols) > 0 {
		return a.symbols
	}

	// Try to get from config
	config := a.GetConfig()
	if config == nil || config.Config == nil {
		// Default to BTC if no config
		log.Warn().Msg("No symbols configured, defaulting to bitcoin")
		return []string{"bitcoin"}
	}

	// Extract symbols from config (if present)
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

	// Default to bitcoin
	log.Warn().Msg("No symbols in config, defaulting to bitcoin")
	return []string{"bitcoin"}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Result parser functions - convert MCP CallToolResult to typed indicator results

// parseRSIResult parses RSI calculation result from MCP tool
func parseRSIResult(result interface{}) (*RSIResult, error) {
	// MCP SDK returns result in Content field as []Content
	// Each Content has Type and embedded data
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result type")
	}

	// The actual data is in the result map directly
	value, err := extractFloat64(resultMap, "value")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'value' field: %w", err)
	}

	signal, ok := resultMap["signal"].(string)
	if !ok {
		signal = "neutral" // Default if not provided
	}

	return &RSIResult{
		Value:  value,
		Signal: signal,
	}, nil
}

// parseMACDResult parses MACD calculation result from MCP tool
func parseMACDResult(result interface{}) (*MACDResult, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result type")
	}

	macdValue, err := extractFloat64(resultMap, "macd")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'macd' field: %w", err)
	}

	signalValue, err := extractFloat64(resultMap, "signal")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'signal' field: %w", err)
	}

	histogram, err := extractFloat64(resultMap, "histogram")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'histogram' field: %w", err)
	}

	crossover, ok := resultMap["crossover"].(string)
	if !ok {
		crossover = "none" // Default if not provided
	}

	return &MACDResult{
		MACD:      macdValue,
		Signal:    signalValue,
		Histogram: histogram,
		Crossover: crossover,
	}, nil
}

// parseBollingerBandsResult parses Bollinger Bands calculation result from MCP tool
func parseBollingerBandsResult(result interface{}) (*BollingerBandsResult, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result type")
	}

	upper, err := extractFloat64(resultMap, "upper")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'upper' field: %w", err)
	}

	middle, err := extractFloat64(resultMap, "middle")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'middle' field: %w", err)
	}

	lower, err := extractFloat64(resultMap, "lower")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'lower' field: %w", err)
	}

	width, err := extractFloat64(resultMap, "width")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'width' field: %w", err)
	}

	signal, ok := resultMap["signal"].(string)
	if !ok {
		signal = "neutral" // Default if not provided
	}

	return &BollingerBandsResult{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
		Width:  width,
		Signal: signal,
	}, nil
}

// parseEMAResult parses EMA calculation result from MCP tool
func parseEMAResult(result interface{}) (float64, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid result type")
	}

	// EMA returns a single value
	value, err := extractFloat64(resultMap, "value")
	if err != nil {
		return 0, fmt.Errorf("missing or invalid 'value' field: %w", err)
	}

	return value, nil
}

// extractFloat64 extracts a float64 value from a map, handling different numeric types
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
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("failed to convert json.Number to float64: %w", err)
		}
		return f, nil
	case string:
		// Try to parse string as float64
		var f float64
		if err := json.Unmarshal([]byte(v), &f); err != nil {
			return 0, fmt.Errorf("failed to parse string as float64: %w", err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("unsupported type %T for key '%s'", val, key)
	}
}

// Configuration helper functions

// getIntFromConfig extracts an integer from config map with default value
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
	case float32:
		return int(v)
	default:
		return defaultVal
	}
}

// getFloatFromConfig extracts a float64 from config map with default value
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
	case int64:
		return float64(v)
	default:
		return defaultVal
	}
}

// Helper functions to extract nested configuration

func getMapConfig(config map[string]interface{}, path string) map[string]interface{} {
	if config == nil {
		return make(map[string]interface{})
	}

	// Simple path traversal (e.g., "indicators.rsi")
	// For production, use a proper path library
	current := config
	keys := splitPath(path)

	for i, key := range keys {
		if i == len(keys)-1 {
			if val, ok := current[key].(map[string]interface{}); ok {
				return val
			}
			return make(map[string]interface{})
		}

		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return make(map[string]interface{})
		}
	}

	return make(map[string]interface{})
}

func getMapStringConfig(config map[string]interface{}, key string) map[string]string {
	if val, ok := config[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, v := range val {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return make(map[string]string)
}

func getMapFloatConfig(config map[string]interface{}, key string) map[string]float64 {
	if val, ok := config[key].(map[string]interface{}); ok {
		result := make(map[string]float64)
		for k, v := range val {
			switch num := v.(type) {
			case float64:
				result[k] = num
			case int:
				result[k] = float64(num)
			}
		}
		return result
	}
	return make(map[string]float64)
}

func splitPath(path string) []string {
	result := []string{}
	current := ""
	for _, ch := range path {
		if ch == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func main() {
	// Configure logging to stderr (stdout reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	viper.SetConfigName("agents")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../../../configs") // From cmd/agents/technical-agent/

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	// Extract technical agent configuration
	var agentConfig agents.AgentConfig

	// Get technical agent config from analysis_agents.technical
	technicalConfig := viper.Sub("analysis_agents.technical")
	if technicalConfig == nil {
		log.Fatal().Msg("Technical agent configuration not found in agents.yaml")
	}

	agentConfig.Name = technicalConfig.GetString("name")
	agentConfig.Type = technicalConfig.GetString("type")
	agentConfig.Version = technicalConfig.GetString("version")
	agentConfig.Enabled = technicalConfig.GetBool("enabled")

	// Parse step interval
	stepIntervalStr := technicalConfig.GetString("step_interval")
	stepInterval, err := time.ParseDuration(stepIntervalStr)
	if err != nil {
		log.Fatal().Err(err).Str("interval", stepIntervalStr).Msg("Invalid step_interval")
	}
	agentConfig.StepInterval = stepInterval

	// Get agent-specific config
	agentConfig.Config = technicalConfig.Get("config").(map[string]interface{})

	// Get metrics port from global config
	metricsPort := viper.GetInt("global.metrics_port")
	if metricsPort == 0 {
		metricsPort = 9101 // Default port
	}

	// Get MCP server configurations
	mcpServers := technicalConfig.Get("mcp_servers")
	if mcpServers != nil {
		log.Debug().Interface("mcp_servers", mcpServers).Msg("MCP servers configured")

		// Parse MCP server list into MCPServerConfig structs
		if servers, ok := mcpServers.([]interface{}); ok {
			agentConfig.MCPServers = make([]agents.MCPServerConfig, 0, len(servers))
			for _, srv := range servers {
				if server, ok := srv.(map[string]interface{}); ok {
					serverConfig := agents.MCPServerConfig{
						Name: server["name"].(string),
						Type: server["type"].(string),
					}

					// Set fields based on server type
					if serverConfig.Type == "internal" {
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
					} else if serverConfig.Type == "external" {
						// DEBUG: Log raw server map to see all fields
						log.Debug().Interface("server_map", server).Msg("Raw server map for external type")

						// DEBUG: Check if URL key exists and what value it has
						urlValue, urlExists := server["url"]
						log.Debug().
							Bool("url_exists", urlExists).
							Interface("url_value", urlValue).
							Str("url_type", fmt.Sprintf("%T", urlValue)).
							Msg("URL field check")

						// Existing URL extraction with debug logging
						if url, ok := server["url"].(string); ok {
							serverConfig.URL = url
							log.Debug().Str("extracted_url", url).Msg("Successfully extracted URL")
						} else {
							log.Warn().Msg("Failed to extract URL from server map - type assertion failed")
						}
					}

					agentConfig.MCPServers = append(agentConfig.MCPServers, serverConfig)
					log.Info().
						Str("name", serverConfig.Name).
						Str("type", serverConfig.Type).
						Str("url", serverConfig.URL).
						Str("command", serverConfig.Command).
						Msg("Configured MCP server")
				}
			}
		}
	}

	// Create agent
	agent, err := NewTechnicalAgent(&agentConfig, log.Logger, metricsPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create technical agent")
	}

	log.Info().
		Str("name", agentConfig.Name).
		Str("type", agentConfig.Type).
		Str("version", agentConfig.Version).
		Dur("step_interval", agentConfig.StepInterval).
		Int("metrics_port", metricsPort).
		Msg("Starting technical analysis agent")

	// Initialize agent
	ctx := context.Background()
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run agent in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Run(ctx)
	}()

	// Wait for shutdown signal or error
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

	log.Info().Msg("Technical analysis agent shutdown complete")
}
