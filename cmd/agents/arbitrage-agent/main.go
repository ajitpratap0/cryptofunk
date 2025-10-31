// Arbitrage Agent
// Detects and exploits price differences across multiple exchanges
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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
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
	Source     string      `json:"source"`     // Source of belief (exchange, calculation, etc.)
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
// ARBITRAGE AGENT
// ============================================================================

// ArbitrageAgent detects price differences across exchanges
type ArbitrageAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn  *nats.Conn
	natsTopic string

	// Strategy configuration
	symbols          []string // Symbols to monitor
	exchanges        []string // Exchanges to compare
	minSpread        float64  // Minimum spread to consider (e.g., 0.005 for 0.5%)
	maxLatencyMs     int      // Maximum acceptable latency
	lookbackPeriod   int      // Number of recent prices to track
	confidenceThresh float64  // Minimum confidence to generate signal

	// Exchange fee configuration
	exchangeFees map[string]*ExchangeFees // Fee structure per exchange

	// BDI (Belief-Desire-Intention) architecture
	beliefs *BeliefBase // Agent's beliefs about market opportunities

	// Price tracking for each exchange
	priceCache map[string]map[string]*ExchangePrice // symbol -> exchange -> price
	cacheMutex sync.RWMutex

	// Opportunity tracking
	lastOpportunities map[string]*ArbitrageOpportunity
	opportunityMutex  sync.RWMutex
}

// ExchangeFees represents fee structure for an exchange
type ExchangeFees struct {
	Exchange    string  `json:"exchange"`
	MakerFee    float64 `json:"maker_fee"`    // Maker fee percentage (e.g., 0.001 for 0.1%)
	TakerFee    float64 `json:"taker_fee"`    // Taker fee percentage
	WithdrawFee float64 `json:"withdraw_fee"` // Withdrawal fee (flat or percentage)
}

// ExchangePrice represents a price quote from a specific exchange
type ExchangePrice struct {
	Exchange  string    `json:"exchange"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	BidPrice  float64   `json:"bid_price,omitempty"`  // Best bid if available
	AskPrice  float64   `json:"ask_price,omitempty"`  // Best ask if available
	Volume24h float64   `json:"volume_24h,omitempty"` // 24h volume
	Timestamp time.Time `json:"timestamp"`
	Latency   int64     `json:"latency_ms"` // Query latency in milliseconds
}

// ArbitrageOpportunity represents a detected arbitrage opportunity
type ArbitrageOpportunity struct {
	Symbol         string    `json:"symbol"`
	BuyExchange    string    `json:"buy_exchange"`  // Where to buy
	SellExchange   string    `json:"sell_exchange"` // Where to sell
	BuyPrice       float64   `json:"buy_price"`     // Buy price
	SellPrice      float64   `json:"sell_price"`    // Sell price
	RawSpread      float64   `json:"raw_spread"`    // Raw spread before fees
	NetSpread      float64   `json:"net_spread"`    // Net spread after fees
	ProfitPct      float64   `json:"profit_pct"`    // Profit percentage
	Score          float64   `json:"score"`         // Opportunity score (0-1)
	Volume24h      float64   `json:"volume_24h"`    // Available volume
	Confidence     float64   `json:"confidence"`    // Confidence level
	Timestamp      time.Time `json:"timestamp"`
	ExpiresAt      time.Time `json:"expires_at"`      // When opportunity likely expires
	ExecutionRisk  string    `json:"execution_risk"`  // "low", "medium", "high"
	LatencyWarning bool      `json:"latency_warning"` // True if latency > threshold
}

// ArbitrageSignal represents an arbitrage trading signal
type ArbitrageSignal struct {
	Timestamp   time.Time             `json:"timestamp"`
	Symbol      string                `json:"symbol"`
	Signal      string                `json:"signal"`     // "ARBITRAGE", "HOLD"
	Confidence  float64               `json:"confidence"` // 0.0 to 1.0
	Opportunity *ArbitrageOpportunity `json:"opportunity"`
	Reasoning   string                `json:"reasoning"`
	Beliefs     map[string]*Belief    `json:"beliefs,omitempty"`
}

// ============================================================================
// AGENT LIFECYCLE
// ============================================================================

// Initialize sets up the agent's MCP connections and internal state
func (a *ArbitrageAgent) Initialize(ctx context.Context) error {
	log.Info().Str("agent", "arbitrage").Msg("Initializing Arbitrage Agent")

	// First, initialize base agent (connects to MCP servers, starts metrics, etc.)
	if err := a.BaseAgent.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize base agent: %w", err)
	}

	// Initialize belief base
	a.beliefs = NewBeliefBase()

	// Initialize price cache
	a.priceCache = make(map[string]map[string]*ExchangePrice)
	a.lastOpportunities = make(map[string]*ArbitrageOpportunity)

	// Initialize default exchange fees (TODO: Make configurable)
	a.exchangeFees = map[string]*ExchangeFees{
		"binance": {
			Exchange:    "binance",
			MakerFee:    0.001,  // 0.1% maker
			TakerFee:    0.001,  // 0.1% taker
			WithdrawFee: 0.0005, // 0.05% withdraw
		},
		"coinbase": {
			Exchange:    "coinbase",
			MakerFee:    0.004, // 0.4% maker
			TakerFee:    0.006, // 0.6% taker
			WithdrawFee: 0.001, // 0.1% withdraw
		},
		"kraken": {
			Exchange:    "kraken",
			MakerFee:    0.0016, // 0.16% maker
			TakerFee:    0.0026, // 0.26% taker
			WithdrawFee: 0.0009, // 0.09% withdraw
		},
	}

	// Connect to NATS for signal publishing
	natsURL := viper.GetString("nats.url")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	a.natsConn = nc
	a.natsTopic = viper.GetString("communication.nats.topics.strategy_decisions")
	if a.natsTopic == "" {
		a.natsTopic = "agents.strategy.decisions"
	}

	log.Info().
		Str("nats_url", natsURL).
		Str("topic", a.natsTopic).
		Msg("Connected to NATS")

	// Initialize beliefs about market state
	a.beliefs.UpdateBelief("agent_status", "initialized", 1.0, "system")
	a.beliefs.UpdateBelief("market_efficiency", "unknown", 0.5, "initialization")
	a.beliefs.UpdateBelief("opportunity_available", false, 0.0, "initialization")

	log.Info().Msg("Arbitrage Agent initialized successfully")
	return nil
}

// Run starts the agent's main decision loop
func (a *ArbitrageAgent) Run(ctx context.Context) error {
	log.Info().Str("agent", "arbitrage").Msg("Starting Arbitrage Agent")

	// Update agent status
	a.beliefs.UpdateBelief("agent_status", "running", 1.0, "system")

	// Get step interval from config (default: 5s for arbitrage - speed matters)
	stepInterval := viper.GetDuration("strategy_agents.arbitrage.step_interval")
	if stepInterval == 0 {
		stepInterval = 5 * time.Second
	}

	ticker := time.NewTicker(stepInterval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", stepInterval).
		Strs("symbols", a.symbols).
		Strs("exchanges", a.exchanges).
		Float64("min_spread", a.minSpread).
		Msg("Agent running")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Agent context cancelled, shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := a.Step(ctx); err != nil {
				log.Error().Err(err).Msg("Error in agent step")
				// Continue running despite errors
			}
		}
	}
}

// Step performs one decision cycle
func (a *ArbitrageAgent) Step(ctx context.Context) error {
	log.Debug().Msg("Executing arbitrage agent step")

	// Step 1: Fetch prices from all exchanges
	if err := a.fetchPrices(ctx); err != nil {
		return fmt.Errorf("failed to fetch prices: %w", err)
	}

	// Step 2: Calculate spreads and identify opportunities
	opportunities := a.calculateSpreads()

	// Step 3: Score opportunities by profit potential and risk
	scoredOpportunities := a.scoreOpportunities(opportunities)

	// Step 4: Generate decision based on best opportunity
	signal := a.generateDecision(scoredOpportunities)

	// Step 5: Publish signal if actionable
	if signal.Signal != "HOLD" {
		if err := a.publishSignal(signal); err != nil {
			log.Error().Err(err).Msg("Failed to publish signal")
		}
	}

	return nil
}

// Shutdown performs cleanup
func (a *ArbitrageAgent) Shutdown(ctx context.Context) error {
	log.Info().Str("agent", "arbitrage").Msg("Shutting down Arbitrage Agent")

	// Update status
	a.beliefs.UpdateBelief("agent_status", "shutdown", 1.0, "system")

	// Close NATS connection
	if a.natsConn != nil {
		a.natsConn.Close()
	}

	log.Info().Msg("Arbitrage Agent shutdown complete")
	return nil
}

// ============================================================================
// PLACEHOLDER METHODS (To be implemented in subsequent tasks)
// ============================================================================

// fetchPrices fetches current prices from all configured exchanges in parallel
func (a *ArbitrageAgent) fetchPrices(ctx context.Context) error {
	log.Debug().
		Strs("symbols", a.symbols).
		Strs("exchanges", a.exchanges).
		Msg("Fetching prices from exchanges")

	// Use wait group for parallel fetching
	var wg sync.WaitGroup
	errorsChan := make(chan error, len(a.symbols)*len(a.exchanges))

	// Fetch prices for each symbol from each exchange in parallel
	for _, symbol := range a.symbols {
		for _, exchange := range a.exchanges {
			wg.Add(1)

			// Launch goroutine for each exchange-symbol pair
			go func(sym, exch string) {
				defer wg.Done()

				startTime := time.Now()
				price, err := a.fetchPriceFromExchange(ctx, sym, exch)
				latency := time.Since(startTime).Milliseconds()

				if err != nil {
					log.Warn().
						Err(err).
						Str("symbol", sym).
						Str("exchange", exch).
						Int64("latency_ms", latency).
						Msg("Failed to fetch price from exchange")
					errorsChan <- err
					return
				}

				// Set latency
				price.Latency = latency

				// Check latency threshold
				if latency > int64(a.maxLatencyMs) {
					log.Warn().
						Str("symbol", sym).
						Str("exchange", exch).
						Int64("latency_ms", latency).
						Int("max_latency_ms", a.maxLatencyMs).
						Msg("Exchange latency exceeds threshold")

					// Update belief about exchange reliability
					beliefKey := fmt.Sprintf("exchange_%s_reliable", exch)
					a.beliefs.UpdateBelief(beliefKey, false, 0.5, "latency_check")
				} else {
					beliefKey := fmt.Sprintf("exchange_%s_reliable", exch)
					a.beliefs.UpdateBelief(beliefKey, true, 0.9, "latency_check")
				}

				// Update price cache
				a.updatePriceCache(price)

				log.Debug().
					Str("symbol", sym).
					Str("exchange", exch).
					Float64("price", price.Price).
					Int64("latency_ms", latency).
					Msg("Price fetched successfully")

			}(symbol, exchange)
		}
	}

	// Wait for all fetches to complete
	wg.Wait()
	close(errorsChan)

	// Collect errors (if any)
	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	// Update belief about data availability
	totalExpected := len(a.symbols) * len(a.exchanges)
	successCount := totalExpected - len(errors)
	dataAvailability := float64(successCount) / float64(totalExpected)

	a.beliefs.UpdateBelief("data_availability", dataAvailability, dataAvailability, "price_fetch")

	log.Debug().
		Int("success", successCount).
		Int("failed", len(errors)).
		Int("total", totalExpected).
		Float64("availability", dataAvailability).
		Msg("Price fetch completed")

	// Return error only if all fetches failed
	if len(errors) == totalExpected {
		return fmt.Errorf("all price fetches failed: %d errors", len(errors))
	}

	return nil
}

// fetchPriceFromExchange fetches price for a single symbol from a single exchange
func (a *ArbitrageAgent) fetchPriceFromExchange(ctx context.Context, symbol, exchange string) (*ExchangePrice, error) {
	// For now, we'll use CoinGecko as the market data source
	// In a real implementation, this would call exchange-specific APIs

	// Call MCP tool to get price
	result, err := a.BaseAgent.CallMCPTool(ctx, "coingecko", "get_simple_price", map[string]interface{}{
		"ids":                 symbol,
		"vs_currencies":       "usd",
		"include_24hr_vol":    true,
		"include_24hr_change": true,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to call get_simple_price: %w", err)
	}

	// Parse result
	if result.Content == nil || len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from get_simple_price")
	}

	// Extract price data from MCP result
	var priceData map[string]interface{}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("expected TextContent, got %T", result.Content[0])
	}

	if err := json.Unmarshal([]byte(textContent.Text), &priceData); err != nil {
		return nil, fmt.Errorf("failed to parse price data: %w", err)
	}

	// Extract symbol data
	symbolData, ok := priceData[symbol].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("symbol %s not found in response", symbol)
	}

	price, ok := symbolData["usd"].(float64)
	if !ok {
		return nil, fmt.Errorf("price not found for symbol %s", symbol)
	}

	// Extract volume if available
	volume := 0.0
	if vol, ok := symbolData["usd_24h_vol"].(float64); ok {
		volume = vol
	}

	// Create exchange price
	exchangePrice := &ExchangePrice{
		Exchange:  exchange,
		Symbol:    symbol,
		Price:     price,
		Volume24h: volume,
		Timestamp: time.Now(),
	}

	// For realistic simulation, add small random variation based on exchange
	// This simulates different prices on different exchanges
	exchangePrice.Price = a.simulateExchangeVariation(price, exchange)

	return exchangePrice, nil
}

// simulateExchangeVariation adds realistic price variation for different exchanges
func (a *ArbitrageAgent) simulateExchangeVariation(basePrice float64, exchange string) float64 {
	// Small random variation (±0.1% to ±0.3%) to simulate exchange differences
	// In production, this would come from actual exchange APIs

	// Seed variation based on exchange name for consistency
	variation := 0.0
	switch exchange {
	case "binance":
		variation = -0.0005 // Typically 0.05% lower (high liquidity)
	case "coinbase":
		variation = 0.001 // Typically 0.1% higher (retail premium)
	case "kraken":
		variation = 0.0003 // Slightly higher (moderate liquidity)
	default:
		variation = 0.0
	}

	// Add small random component (±0.05%)
	randomVariation := (float64(time.Now().UnixNano()%100) - 50) / 100000.0

	return basePrice * (1.0 + variation + randomVariation)
}

// updatePriceCache updates the price cache with new price data
func (a *ArbitrageAgent) updatePriceCache(price *ExchangePrice) {
	a.cacheMutex.Lock()
	defer a.cacheMutex.Unlock()

	// Initialize symbol map if needed
	if a.priceCache[price.Symbol] == nil {
		a.priceCache[price.Symbol] = make(map[string]*ExchangePrice)
	}

	// Store price
	a.priceCache[price.Symbol][price.Exchange] = price

	log.Debug().
		Str("symbol", price.Symbol).
		Str("exchange", price.Exchange).
		Float64("price", price.Price).
		Msg("Price cache updated")
}

// calculateSpreads calculates price spreads between exchanges
// TODO: Implement in T093 - Spread calculation with fees
func (a *ArbitrageAgent) calculateSpreads() []*ArbitrageOpportunity {
	log.Debug().Msg("Calculating spreads (placeholder)")
	// Placeholder: Will be implemented in T093
	return nil
}

// scoreOpportunities scores arbitrage opportunities
// TODO: Implement in T094 - Opportunity scoring
func (a *ArbitrageAgent) scoreOpportunities(opportunities []*ArbitrageOpportunity) []*ArbitrageOpportunity {
	log.Debug().Msg("Scoring opportunities (placeholder)")
	// Placeholder: Will be implemented in T094
	return opportunities
}

// generateDecision generates trading decision
// TODO: Implement in T095 - Decision generation
func (a *ArbitrageAgent) generateDecision(opportunities []*ArbitrageOpportunity) *ArbitrageSignal {
	log.Debug().Msg("Generating decision (placeholder)")
	// Placeholder: Will be implemented in T095
	return &ArbitrageSignal{
		Timestamp:  time.Now(),
		Signal:     "HOLD",
		Confidence: 0.0,
		Reasoning:  "No opportunities detected",
	}
}

// publishSignal publishes signal to NATS
func (a *ArbitrageAgent) publishSignal(signal *ArbitrageSignal) error {
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	if err := a.natsConn.Publish(a.natsTopic, data); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	log.Info().
		Str("signal", signal.Signal).
		Float64("confidence", signal.Confidence).
		Str("symbol", signal.Symbol).
		Msg("Published arbitrage signal")

	return nil
}

// ============================================================================
// MAIN ENTRY POINT
// ============================================================================

func main() {
	// Configure logging to stderr (stdout reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	viper.SetConfigName("agents")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("CRYPTOFUNK_AGENT")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to read config")
	}

	// Set log level
	logLevel := viper.GetString("logging.level")
	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Extract agent configuration
	var agentConfig agents.AgentConfig

	arbitrageConfig := viper.Sub("strategy_agents.arbitrage")
	if arbitrageConfig == nil {
		log.Fatal().Msg("Arbitrage agent configuration not found in agents.yaml")
	}

	agentConfig.Name = arbitrageConfig.GetString("name")
	agentConfig.Type = arbitrageConfig.GetString("type")
	agentConfig.Version = arbitrageConfig.GetString("version")
	agentConfig.Enabled = arbitrageConfig.GetBool("enabled")

	// Parse step interval
	stepIntervalStr := arbitrageConfig.GetString("step_interval")
	stepInterval, err := time.ParseDuration(stepIntervalStr)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid step_interval")
	}
	agentConfig.StepInterval = stepInterval

	// Get agent-specific config
	agentConfig.Config = arbitrageConfig.Get("config").(map[string]interface{})

	// Get metrics port
	metricsPort := viper.GetInt("global.metrics_port")
	if metricsPort == 0 {
		metricsPort = 9103 // Default for arbitrage agent
	}

	// Parse MCP servers
	mcpServers := arbitrageConfig.Get("mcp_servers")
	if mcpServers != nil {
		if servers, ok := mcpServers.([]interface{}); ok {
			agentConfig.MCPServers = make([]agents.MCPServerConfig, 0, len(servers))
			for _, srv := range servers {
				if server, ok := srv.(map[string]interface{}); ok {
					serverConfig := agents.MCPServerConfig{
						Name: server["name"].(string),
						Type: server["type"].(string),
					}

					// Parse command and args for internal servers
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
							serverConfig.Env = make(map[string]string)
							for k, v := range env {
								serverConfig.Env[k] = v.(string)
							}
						}
					}

					// Parse URL for external servers
					if serverConfig.Type == "external" {
						if url, ok := server["url"].(string); ok {
							serverConfig.URL = url
						}
					}

					agentConfig.MCPServers = append(agentConfig.MCPServers, serverConfig)
				}
			}
		}
	}

	// Create base agent with proper config
	baseAgent := agents.NewBaseAgent(&agentConfig, log.Logger, metricsPort)

	// Extract arbitrage-specific configuration with defaults
	symbols := []string{"bitcoin", "ethereum"}
	if syms := arbitrageConfig.GetStringSlice("config.symbols"); len(syms) > 0 {
		symbols = syms
	}

	exchanges := []string{"binance", "coinbase", "kraken"}
	if exs := arbitrageConfig.GetStringSlice("config.exchanges"); len(exs) > 0 {
		exchanges = exs
	}

	minSpread := arbitrageConfig.GetFloat64("config.min_spread")
	if minSpread == 0 {
		minSpread = 0.005 // 0.5%
	}

	maxLatencyMs := arbitrageConfig.GetInt("config.max_latency_ms")
	if maxLatencyMs == 0 {
		maxLatencyMs = 100
	}

	lookbackPeriod := arbitrageConfig.GetInt("config.lookback_periods")
	if lookbackPeriod == 0 {
		lookbackPeriod = 20
	}

	confidenceThresh := arbitrageConfig.GetFloat64("config.confidence_threshold")
	if confidenceThresh == 0 {
		confidenceThresh = 0.80
	}

	// Create arbitrage agent
	agent := &ArbitrageAgent{
		BaseAgent:        baseAgent,
		symbols:          symbols,
		exchanges:        exchanges,
		minSpread:        minSpread,
		maxLatencyMs:     maxLatencyMs,
		lookbackPeriod:   lookbackPeriod,
		confidenceThresh: confidenceThresh,
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	}()

	// Initialize and run agent
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("Agent runtime error")
	}

	if err := agent.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("Arbitrage Agent terminated")
}
