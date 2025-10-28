// Trend Following Agent
// Generates trading signals using EMA crossover and trend strength (ADX)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
)

// TrendAgent performs trend following strategy using EMA crossovers and ADX
type TrendAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn  *nats.Conn
	natsTopic string

	// Strategy configuration
	symbols         []string
	fastEMAPeriod   int
	slowEMAPeriod   int
	adxPeriod       int
	adxThreshold    float64 // Minimum ADX to consider trend strong
	lookbackCandles int     // Number of candles to fetch for analysis

	// Cached indicator values
	currentIndicators *TrendIndicators

	// Strategy state
	lastCrossover  string // "bullish", "bearish", "none"
	lastSignal     string // Last signal generated
	lastSignalTime time.Time
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
	Timestamp  time.Time        `json:"timestamp"`
	Symbol     string           `json:"symbol"`
	Signal     string           `json:"signal"`     // "BUY", "SELL", "HOLD"
	Confidence float64          `json:"confidence"` // 0.0 to 1.0
	Indicators *TrendIndicators `json:"indicators"`
	Reasoning  string           `json:"reasoning"`
	Price      float64          `json:"price"`
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

	// Extract EMA periods from config
	fastEMA := getIntFromConfig(agentConfig, "fast_ema_period", 9)
	slowEMA := getIntFromConfig(agentConfig, "slow_ema_period", 21)
	adxPeriod := getIntFromConfig(agentConfig, "adx_period", 14)
	adxThreshold := getFloatFromConfig(agentConfig, "adx_threshold", 25.0)
	lookback := getIntFromConfig(agentConfig, "lookback_candles", 100)

	return &TrendAgent{
		BaseAgent:       baseAgent,
		natsConn:        nc,
		natsTopic:       natsTopic,
		fastEMAPeriod:   fastEMA,
		slowEMAPeriod:   slowEMA,
		adxPeriod:       adxPeriod,
		adxThreshold:    adxThreshold,
		lookbackCandles: lookback,
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
func (a *TrendAgent) generateTrendSignal(ctx context.Context, symbol string, indicators *TrendIndicators, currentPrice float64) (*TrendSignal, error) {
	log.Debug().Str("symbol", symbol).Msg("Generating trend following signal")

	var signal string
	var confidence float64
	var reasoning string

	// Trend following strategy logic:
	// BUY: Fast EMA crosses above Slow EMA (golden cross) + strong trend (ADX > threshold)
	// SELL: Fast EMA crosses below Slow EMA (death cross) + strong trend (ADX > threshold)
	// HOLD: Weak trend (ADX < threshold) or no clear crossover

	if indicators.Strength == "strong" {
		// Strong trend detected
		if indicators.Trend == "uptrend" {
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
		} else if indicators.Trend == "downtrend" {
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
		} else {
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

	return &TrendSignal{
		Timestamp:  time.Now(),
		Symbol:     symbol,
		Signal:     signal,
		Confidence: confidence,
		Indicators: indicators,
		Reasoning:  reasoning,
		Price:      currentPrice,
	}, nil
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
