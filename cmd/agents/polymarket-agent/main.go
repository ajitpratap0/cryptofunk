// Polymarket Trading Agent
// Analyzes prediction markets on Polymarket and generates trading signals
// using LLM-based analysis for information edge detection
//
//nolint:goconst // Trading signals and market categories are domain-specific strings
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/exchange"
)

// ============================================================================
// BELIEF SYSTEM (BDI ARCHITECTURE)
// ============================================================================

// Belief represents a single belief in the agent's belief base
type Belief struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Confidence float64     `json:"confidence"`
	Timestamp  time.Time   `json:"timestamp"`
	Source     string      `json:"source"`
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
	result := make(map[string]*Belief, len(bb.beliefs))
	for k, v := range bb.beliefs {
		b := *v
		result[k] = &b
	}
	return result
}

// GetConfidence calculates overall confidence (average of all beliefs)
func (bb *BeliefBase) GetConfidence() float64 {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()
	if len(bb.beliefs) == 0 {
		return 0.0
	}
	total := 0.0
	for _, b := range bb.beliefs {
		total += b.Confidence
	}
	return total / float64(len(bb.beliefs))
}

// ============================================================================
// MARKET DOMAIN TYPES
// ============================================================================

// PolyMarket represents a single Polymarket prediction market
type PolyMarket struct {
	ID             string    `json:"id"`
	Question       string    `json:"question"`
	Category       string    `json:"category"`        // politics, crypto, tech, sports, etc.
	OutcomeType    string    `json:"outcome_type"`    // binary, multi
	YesPrice       float64   `json:"yes_price"`       // 0.0 - 1.0
	NoPrice        float64   `json:"no_price"`        // 0.0 - 1.0
	Spread         float64   `json:"spread"`          // yes_price + no_price - 1.0 (overround)
	Volume24h      float64   `json:"volume_24h"`      // USD volume last 24h
	Liquidity      float64   `json:"liquidity"`       // Total liquidity in the market
	ResolutionTime time.Time `json:"resolution_time"` // When the market resolves
	CreatedAt      time.Time `json:"created_at"`
	Active         bool      `json:"active"`
}

// Spread returns the bid-ask spread of the market
func (m *PolyMarket) BidAskSpread() float64 {
	return math.Abs((m.YesPrice + m.NoPrice) - 1.0)
}

// DaysUntilResolution returns days until market resolves
func (m *PolyMarket) DaysUntilResolution() float64 {
	return time.Until(m.ResolutionTime).Hours() / 24.0
}

// Position represents an open position in a Polymarket market
type Position struct {
	MarketID   string    `json:"market_id"`
	Question   string    `json:"question"`
	Side       string    `json:"side"`        // "YES" or "NO"
	EntryPrice float64   `json:"entry_price"` // Price at entry
	Size       float64   `json:"size"`        // Position size in USD
	CurrentPnL float64   `json:"current_pnl"` // Unrealized P&L
	OpenedAt   time.Time `json:"opened_at"`
	Confidence float64   `json:"confidence"` // Confidence at time of entry
}

// CurrentPrice returns current market price for this position's side
func (p *Position) CurrentPrice(market *PolyMarket) float64 {
	if p.Side == "YES" {
		return market.YesPrice
	}
	return market.NoPrice
}

// UnrealizedPnL calculates unrealized P&L based on current price
func (p *Position) UnrealizedPnL(currentPrice float64) float64 {
	shares := p.Size / p.EntryPrice
	return shares*currentPrice - p.Size
}

// UnrealizedPnLPct returns P&L as a percentage
func (p *Position) UnrealizedPnLPct(currentPrice float64) float64 {
	if p.Size == 0 {
		return 0
	}
	return (p.UnrealizedPnL(currentPrice) / p.Size) * 100.0
}

// LLMAnalysis represents the LLM's analysis of a market
type LLMAnalysis struct {
	MarketID       string  `json:"market_id"`
	Prediction     string  `json:"prediction"`       // "YES" or "NO"
	Confidence     float64 `json:"confidence"`       // 0.0 - 1.0
	FairPrice      float64 `json:"fair_price"`       // LLM's estimate of fair YES price
	Reasoning      string  `json:"reasoning"`        // Why the LLM thinks this
	InformationAge string  `json:"information_edge"` // What info advantage exists
	RiskFactors    string  `json:"risk_factors"`     // Key risks
}

// TradeSignal represents a signal to trade on a Polymarket market
type TradeSignal struct {
	Timestamp  time.Time          `json:"timestamp"`
	MarketID   string             `json:"market_id"`
	Question   string             `json:"question"`
	Signal     string             `json:"signal"`     // "BUY_YES", "BUY_NO", "SELL", "HOLD"
	Side       string             `json:"side"`       // "YES" or "NO"
	Price      float64            `json:"price"`      // Current market price for side
	FairPrice  float64            `json:"fair_price"` // LLM estimated fair price
	Edge       float64            `json:"edge"`       // fair_price - market_price
	Size       float64            `json:"size"`       // Suggested position size in USD
	Confidence float64            `json:"confidence"`
	Reasoning  string             `json:"reasoning"`
	Beliefs    map[string]*Belief `json:"beliefs,omitempty"`
}

// ============================================================================
// POLYMARKET AGENT
// ============================================================================

// PolymarketAgent trades prediction markets on Polymarket
type PolymarketAgent struct {
	*agents.BaseAgent

	// NATS connection
	natsConn  *nats.Conn
	natsTopic string
	heartbeat *agents.HeartbeatPublisher

	// BDI belief system
	beliefs *BeliefBase

	// Database persistence (optional, nil if DATABASE_URL not set)
	database  *db.DB
	sessionID uuid.UUID

	// Position management
	positions    map[string]*Position // marketID -> Position
	positionsMtx sync.RWMutex

	// Configuration
	pollInterval        time.Duration
	maxPositionSize     float64 // Max USD per position
	maxTotalExposure    float64 // Max total USD across all positions
	confidenceThreshold float64 // Min LLM confidence to trade
	profitTarget        float64 // Take profit percentage (e.g., 0.15 = 15%)
	stopLoss            float64 // Stop loss percentage (e.g., 0.10 = 10%)

	// Market selection
	minLiquidity        float64  // Minimum market liquidity in USD
	maxSpread           float64  // Maximum spread (avoid illiquid)
	minDaysToResolution float64  // Minimum days until resolution
	maxDaysToResolution float64  // Maximum days until resolution
	preferredCategories []string // Preferred market categories

	// Direct execution mode
	executeMode  bool
	polyExchange *exchange.PolymarketExchange
}

// ============================================================================
// AGENT LIFECYCLE
// ============================================================================

// Initialize sets up the agent
func (a *PolymarketAgent) Initialize(ctx context.Context) error {
	log.Info().Str("agent", "polymarket").Msg("Initializing Polymarket Agent")

	if err := a.BaseAgent.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize base agent: %w", err)
	}

	a.beliefs = NewBeliefBase()
	a.positions = make(map[string]*Position)

	// Initialize database if DATABASE_URL is set
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		database, err := db.New(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to connect to database, running without persistence")
		} else {
			a.database = database

			// Create a paper trading session for the agent
			session := &db.TradingSession{
				ID:             uuid.New(),
				Mode:           db.TradingModePaper,
				Symbol:         "POLYMARKET",
				Exchange:       "polymarket",
				StartedAt:      time.Now(),
				InitialCapital: a.maxTotalExposure,
			}
			if err := database.CreateSession(ctx, session); err != nil {
				log.Warn().Err(err).Msg("Failed to create agent session")
			} else {
				a.sessionID = session.ID
			}

			// Load existing positions from DB
			dbPositions, err := database.GetOpenPolymarketPositions(ctx)
			if err == nil {
				for _, p := range dbPositions {
					a.positions[p.MarketID] = &Position{
						MarketID:   p.MarketID,
						Side:       p.Side,
						EntryPrice: p.AvgPrice,
						Size:       p.CostBasis,
						OpenedAt:   p.OpenedAt,
					}
				}
				log.Info().Int("loaded", len(dbPositions)).Msg("Loaded existing positions from DB")
			}
		}
	}

	// Connect to NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = viper.GetString("nats.url")
	}
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	a.natsConn = nc
	a.natsTopic = viper.GetString("communication.nats.topics.polymarket_signals")
	if a.natsTopic == "" {
		a.natsTopic = "agents.strategy.polymarket"
	}

	// Create heartbeat publisher
	heartbeatTopic := viper.GetString("strategy_agents.polymarket.heartbeat_topic")
	if heartbeatTopic == "" {
		heartbeatTopic = "cryptofunk.agent.heartbeat"
	}
	hbConfig := agents.HeartbeatConfig{
		Interval: 30 * time.Second,
		Topic:    heartbeatTopic,
	}
	a.heartbeat = agents.NewHeartbeatPublisher(
		a.GetConfig().Name, a.GetConfig().Type, hbConfig, log.Logger,
	)
	a.heartbeat.SetNATSConn(nc)

	a.beliefs.UpdateBelief("agent_status", "initialized", 1.0, "system")
	a.beliefs.UpdateBelief("total_exposure", 0.0, 1.0, "position_manager")

	log.Info().
		Str("nats_url", natsURL).
		Str("topic", a.natsTopic).
		Float64("max_position", a.maxPositionSize).
		Float64("max_exposure", a.maxTotalExposure).
		Float64("confidence_threshold", a.confidenceThreshold).
		Msg("Polymarket Agent initialized")

	return nil
}

// Run starts the agent's main decision loop
func (a *PolymarketAgent) Run(ctx context.Context) error {
	log.Info().Str("agent", "polymarket").Msg("Starting Polymarket Agent")
	a.beliefs.UpdateBelief("agent_status", "running", 1.0, "system")

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	log.Info().
		Dur("poll_interval", a.pollInterval).
		Float64("confidence_threshold", a.confidenceThreshold).
		Msg("Agent running")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Agent context cancelled, shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := a.Step(ctx); err != nil {
				log.Error().Err(err).Msg("Error in agent step")
			}
		}
	}
}

// Step performs one decision cycle
func (a *PolymarketAgent) Step(ctx context.Context) error {
	log.Debug().Msg("Executing polymarket agent step")

	// Step 1: Manage existing positions (check profit/stop-loss)
	exitSignals := a.managePositions(ctx)
	for _, sig := range exitSignals {
		if err := a.publishSignal(sig); err != nil {
			log.Error().Err(err).Str("market", sig.MarketID).Msg("Failed to publish exit signal")
		}
	}

	// Step 2: Fetch active markets
	markets, err := a.fetchMarkets(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch markets: %w", err)
	}

	// Step 3: Filter markets by selection heuristics
	filtered := a.filterMarkets(markets)
	log.Debug().Int("total", len(markets)).Int("filtered", len(filtered)).Msg("Markets filtered")

	if len(filtered) == 0 {
		a.beliefs.UpdateBelief("opportunity_available", false, 0.8, "market_filter")
		return nil
	}

	// Step 4: Analyze each market with LLM
	var signals []*TradeSignal
	for _, market := range filtered {
		// Check exposure limits before analyzing
		if a.totalExposure() >= a.maxTotalExposure {
			log.Info().Float64("exposure", a.totalExposure()).Msg("Max exposure reached, skipping new entries")
			break
		}

		// Skip markets we already have positions in
		if a.hasPosition(market.ID) {
			continue
		}

		analysis := a.analyzeMarket(ctx, market)
		if analysis == nil {
			continue
		}

		// Update beliefs
		beliefKey := fmt.Sprintf("market:%s:analysis", market.ID)
		a.beliefs.UpdateBelief(beliefKey, analysis, analysis.Confidence, "llm_analysis")

		// Generate signal if confidence above threshold
		if analysis.Confidence >= a.confidenceThreshold {
			signal := a.generateSignal(market, analysis)
			if signal != nil {
				signals = append(signals, signal)
			}
		}
	}

	// Step 5: Sort by edge (best opportunities first) and publish
	sort.Slice(signals, func(i, j int) bool {
		return math.Abs(signals[i].Edge) > math.Abs(signals[j].Edge)
	})

	for _, sig := range signals {
		if a.totalExposure()+sig.Size > a.maxTotalExposure {
			log.Info().Str("market", sig.MarketID).Msg("Skipping signal, would exceed max exposure")
			continue
		}

		if err := a.publishSignal(sig); err != nil {
			log.Error().Err(err).Str("market", sig.MarketID).Msg("Failed to publish signal")
			continue
		}

		// Execute directly if in execute mode, otherwise just track
		if a.executeMode && a.polyExchange != nil {
			if err := a.executeSignal(ctx, sig); err != nil {
				log.Error().Err(err).Str("market", sig.MarketID).Msg("Failed to execute signal directly")
				continue
			}
		}
		a.openPosition(sig)
	}

	a.beliefs.UpdateBelief("last_step", time.Now().Format(time.RFC3339), 1.0, "system")
	return nil
}

// Shutdown performs cleanup
func (a *PolymarketAgent) Shutdown(ctx context.Context) error {
	log.Info().Str("agent", "polymarket").Msg("Shutting down Polymarket Agent")
	a.beliefs.UpdateBelief("agent_status", "shutdown", 1.0, "system")

	if a.natsConn != nil {
		a.natsConn.Close()
	}

	log.Info().Msg("Polymarket Agent shutdown complete")
	return nil
}

// ============================================================================
// MARKET FETCHING & FILTERING
// ============================================================================

// fetchMarkets retrieves active markets from Polymarket via MCP tool
func (a *PolymarketAgent) fetchMarkets(ctx context.Context) ([]*PolyMarket, error) {
	log.Debug().Msg("Fetching active Polymarket markets")

	result, err := a.CallMCPTool(ctx, "polymarket", "list_markets", map[string]interface{}{
		"active": true,
		"limit":  50,
	})
	if err != nil {
		// If MCP tool not available, return empty (agent is resilient)
		log.Warn().Err(err).Msg("Failed to fetch markets via MCP, using empty list")
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from list_markets")
	}

	// Parse text content
	var textContent string
	for _, c := range result.Content {
		if tc, ok := c.(interface{ GetText() string }); ok {
			textContent = tc.GetText()
			break
		}
		// Try JSON marshal/unmarshal to extract text
		raw, _ := json.Marshal(c)
		var obj map[string]interface{}
		if json.Unmarshal(raw, &obj) == nil {
			if t, ok := obj["text"].(string); ok {
				textContent = t
				break
			}
		}
	}

	if textContent == "" {
		return nil, fmt.Errorf("no text content in MCP result")
	}

	var markets []*PolyMarket
	if err := json.Unmarshal([]byte(textContent), &markets); err != nil {
		return nil, fmt.Errorf("failed to parse markets: %w", err)
	}

	a.beliefs.UpdateBelief("markets_fetched", len(markets), 1.0, "polymarket_api")
	return markets, nil
}

// filterMarkets applies selection heuristics to find tradeable markets
func (a *PolymarketAgent) filterMarkets(markets []*PolyMarket) []*PolyMarket {
	var filtered []*PolyMarket

	for _, m := range markets {
		if !m.Active {
			continue
		}

		// Prefer binary markets
		if m.OutcomeType != "binary" {
			continue
		}

		// Check liquidity
		if m.Liquidity < a.minLiquidity {
			continue
		}

		// Check spread (avoid illiquid markets)
		spread := m.BidAskSpread()
		if spread > a.maxSpread {
			continue
		}

		// Minimum spread for opportunity (at least 2% edge potential)
		if spread < 0.02 {
			// Market is too efficient, no edge
			continue
		}

		// Check resolution time window
		days := m.DaysUntilResolution()
		if days < a.minDaysToResolution || days > a.maxDaysToResolution {
			continue
		}

		// Prefer markets in categories where LLM has information edge
		if len(a.preferredCategories) > 0 {
			found := false
			for _, cat := range a.preferredCategories {
				if strings.EqualFold(m.Category, cat) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, m)
	}

	// Sort by volume (higher volume = more interesting)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Volume24h > filtered[j].Volume24h
	})

	// Limit to top 10 to avoid excessive LLM calls
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	return filtered
}

// ============================================================================
// LLM ANALYSIS
// ============================================================================

// analyzeMarket asks the LLM to analyze a market
func (a *PolymarketAgent) analyzeMarket(ctx context.Context, market *PolyMarket) *LLMAnalysis {
	log.Debug().Str("market", market.ID).Str("question", market.Question).Msg("Analyzing market with LLM")

	prompt := fmt.Sprintf(
		`Analyze this prediction market and provide your assessment:

Question: %s
Category: %s
Current YES price: $%.2f (implies %.0f%% probability)
Current NO price: $%.2f
24h Volume: $%.2f
Liquidity: $%.2f
Resolution: %s (%.1f days away)

Provide your analysis as JSON with these fields:
- prediction: "YES" or "NO" (your best guess)
- confidence: 0.0 to 1.0 (how confident you are)
- fair_price: 0.0 to 1.0 (your estimate of fair YES price)
- reasoning: brief explanation (max 200 chars)
- information_edge: what info advantage exists
- risk_factors: key risks (max 200 chars)

Return ONLY valid JSON, no markdown.`,
		market.Question,
		market.Category,
		market.YesPrice, market.YesPrice*100,
		market.NoPrice,
		market.Volume24h,
		market.Liquidity,
		market.ResolutionTime.Format("2006-01-02"),
		market.DaysUntilResolution(),
	)

	result, err := a.CallMCPTool(ctx, "bifrost", "chat", map[string]interface{}{
		"prompt":      prompt,
		"max_tokens":  500,
		"temperature": 0.3,
	})
	if err != nil {
		log.Warn().Err(err).Str("market", market.ID).Msg("LLM analysis failed")
		return nil
	}

	if len(result.Content) == 0 {
		return nil
	}

	// Extract text from result
	var textContent string
	for _, c := range result.Content {
		raw, _ := json.Marshal(c)
		var obj map[string]interface{}
		if json.Unmarshal(raw, &obj) == nil {
			if t, ok := obj["text"].(string); ok {
				textContent = t
				break
			}
		}
	}

	if textContent == "" {
		return nil
	}

	var analysis LLMAnalysis
	if err := json.Unmarshal([]byte(textContent), &analysis); err != nil {
		log.Warn().Err(err).Str("market", market.ID).Msg("Failed to parse LLM analysis")
		return nil
	}

	analysis.MarketID = market.ID

	// Validate
	if analysis.Confidence < 0 || analysis.Confidence > 1 {
		analysis.Confidence = 0
	}
	if analysis.FairPrice < 0 || analysis.FairPrice > 1 {
		analysis.FairPrice = 0.5
	}

	log.Info().
		Str("market", market.ID).
		Str("prediction", analysis.Prediction).
		Float64("confidence", analysis.Confidence).
		Float64("fair_price", analysis.FairPrice).
		Msg("LLM analysis complete")

	return &analysis
}

// ============================================================================
// SIGNAL GENERATION
// ============================================================================

// generateSignal creates a trade signal from market data and LLM analysis
func (a *PolymarketAgent) generateSignal(market *PolyMarket, analysis *LLMAnalysis) *TradeSignal {
	var side string
	var price float64
	var edge float64

	if analysis.Prediction == "YES" {
		side = "YES"
		price = market.YesPrice
		edge = analysis.FairPrice - market.YesPrice // Positive = underpriced
	} else {
		side = "NO"
		price = market.NoPrice
		edge = (1.0 - analysis.FairPrice) - market.NoPrice
	}

	// Only trade if there's a meaningful edge (>5%)
	if edge < 0.05 {
		log.Debug().
			Str("market", market.ID).
			Float64("edge", edge).
			Msg("Edge too small, skipping")
		return nil
	}

	// Calculate position size (Kelly-inspired but conservative)
	size := a.calculatePositionSize(edge, analysis.Confidence, price)
	if size < 1.0 { // Minimum $1
		return nil
	}

	signal := &TradeSignal{
		Timestamp:  time.Now(),
		MarketID:   market.ID,
		Question:   market.Question,
		Signal:     "BUY_" + side,
		Side:       side,
		Price:      price,
		FairPrice:  analysis.FairPrice,
		Edge:       edge,
		Size:       size,
		Confidence: analysis.Confidence,
		Reasoning:  analysis.Reasoning,
		Beliefs:    a.beliefs.GetAllBeliefs(),
	}

	log.Info().
		Str("market", market.ID).
		Str("signal", signal.Signal).
		Float64("price", price).
		Float64("fair_price", analysis.FairPrice).
		Float64("edge", edge).
		Float64("size", size).
		Msg("Generated trade signal")

	return signal
}

// calculatePositionSize uses a simplified Kelly criterion
func (a *PolymarketAgent) calculatePositionSize(edge, confidence, price float64) float64 {
	if price <= 0 || price >= 1 {
		return 0
	}

	// Kelly fraction: f = (bp - q) / b
	// where b = odds, p = probability of winning, q = 1 - p
	b := (1.0 / price) - 1.0 // Decimal odds
	p := confidence
	q := 1.0 - p

	kellyFraction := (b*p - q) / b
	if kellyFraction <= 0 {
		return 0
	}

	// Use quarter-Kelly for safety
	kellyFraction *= 0.25

	// Apply to max position size
	size := a.maxPositionSize * kellyFraction

	// Clamp
	if size > a.maxPositionSize {
		size = a.maxPositionSize
	}
	if size < 0 {
		size = 0
	}

	return math.Round(size*100) / 100 // Round to cents
}

// ============================================================================
// POSITION MANAGEMENT
// ============================================================================

// openPosition records a new position
func (a *PolymarketAgent) openPosition(signal *TradeSignal) {
	a.positionsMtx.Lock()
	defer a.positionsMtx.Unlock()

	a.positions[signal.MarketID] = &Position{
		MarketID:   signal.MarketID,
		Question:   signal.Question,
		Side:       signal.Side,
		EntryPrice: signal.Price,
		Size:       signal.Size,
		OpenedAt:   time.Now(),
		Confidence: signal.Confidence,
	}

	log.Info().
		Str("market", signal.MarketID).
		Str("side", signal.Side).
		Float64("entry_price", signal.Price).
		Float64("size", signal.Size).
		Msg("Position opened")

	// Persist to DB
	if a.database != nil {
		ctx := context.Background()
		shares := signal.Size / signal.Price

		// Upsert market
		_ = a.database.UpsertPolymarketMarket(ctx, &db.PolymarketMarket{
			ID:       signal.MarketID,
			Question: signal.Question,
			Active:   true,
		})

		// Insert position
		dbPos := &db.PolymarketPosition{
			ID:        uuid.New(),
			SessionID: a.sessionID,
			MarketID:  signal.MarketID,
			Side:      signal.Side,
			Shares:    shares,
			AvgPrice:  signal.Price,
			CostBasis: signal.Size,
			Status:    "OPEN",
		}
		if err := a.database.InsertPolymarketPosition(ctx, dbPos); err != nil {
			log.Warn().Err(err).Str("market", signal.MarketID).Msg("Failed to persist position to DB")
		}

		// Insert trade record
		_ = a.database.InsertPolymarketTrade(ctx, &db.PolymarketTrade{
			ID:         uuid.New(),
			PositionID: dbPos.ID,
			MarketID:   signal.MarketID,
			Action:     "BUY",
			Side:       signal.Side,
			Amount:     signal.Size,
			Price:      signal.Price,
			Shares:     shares,
			Timestamp:  time.Now(),
		})

		// Insert prediction
		_ = a.database.InsertPolymarketPrediction(ctx, &db.PolymarketPrediction{
			MarketID:      signal.MarketID,
			PredictedProb: signal.FairPrice,
			MarketPrice:   signal.Price,
			Edge:          signal.Edge,
			Confidence:    signal.Confidence,
			Reasoning:     &signal.Reasoning,
		})

	}

	a.beliefs.UpdateBelief("total_exposure", a.totalExposureUnsafe(), 1.0, "position_manager")
	a.beliefs.UpdateBelief(
		fmt.Sprintf("position:%s", signal.MarketID),
		signal.Side,
		signal.Confidence,
		"position_manager",
	)
}

// closePosition removes a position
func (a *PolymarketAgent) closePosition(marketID string) {
	a.positionsMtx.Lock()
	defer a.positionsMtx.Unlock()

	delete(a.positions, marketID)

	a.beliefs.UpdateBelief("total_exposure", a.totalExposureUnsafe(), 1.0, "position_manager")
}

// hasPosition checks if we have an open position
func (a *PolymarketAgent) hasPosition(marketID string) bool {
	a.positionsMtx.RLock()
	defer a.positionsMtx.RUnlock()
	_, exists := a.positions[marketID]
	return exists
}

// totalExposure returns total USD exposure across all positions
func (a *PolymarketAgent) totalExposure() float64 {
	a.positionsMtx.RLock()
	defer a.positionsMtx.RUnlock()
	return a.totalExposureUnsafe()
}

// totalExposureUnsafe returns total exposure without locking (caller must hold lock)
func (a *PolymarketAgent) totalExposureUnsafe() float64 {
	total := 0.0
	for _, pos := range a.positions {
		total += pos.Size
	}
	return total
}

// getPositions returns a copy of all positions
func (a *PolymarketAgent) getPositions() map[string]*Position {
	a.positionsMtx.RLock()
	defer a.positionsMtx.RUnlock()
	result := make(map[string]*Position, len(a.positions))
	for k, v := range a.positions {
		p := *v
		result[k] = &p
	}
	return result
}

// managePositions checks all open positions for profit target / stop loss
func (a *PolymarketAgent) managePositions(ctx context.Context) []*TradeSignal {
	positions := a.getPositions()
	if len(positions) == 0 {
		return nil
	}

	var exitSignals []*TradeSignal

	for _, pos := range positions {
		// Fetch current market price
		market, err := a.fetchMarketByID(ctx, pos.MarketID)
		if err != nil {
			log.Warn().Err(err).Str("market", pos.MarketID).Msg("Failed to fetch market for position management")
			continue
		}

		currentPrice := pos.CurrentPrice(market)
		pnlPct := pos.UnrealizedPnLPct(currentPrice)

		// Update P&L belief
		beliefKey := fmt.Sprintf("position:%s:pnl", pos.MarketID)
		a.beliefs.UpdateBelief(beliefKey, pnlPct, 1.0, "position_manager")

		log.Debug().
			Str("market", pos.MarketID).
			Str("side", pos.Side).
			Float64("entry", pos.EntryPrice).
			Float64("current", currentPrice).
			Float64("pnl_pct", pnlPct).
			Msg("Position status")

		// Check profit target
		if pnlPct >= a.profitTarget*100 {
			log.Info().
				Str("market", pos.MarketID).
				Float64("pnl_pct", pnlPct).
				Msg("Profit target hit, closing position")

			exitSignals = append(exitSignals, &TradeSignal{
				Timestamp:  time.Now(),
				MarketID:   pos.MarketID,
				Question:   pos.Question,
				Signal:     "SELL",
				Side:       pos.Side,
				Price:      currentPrice,
				Size:       pos.Size,
				Confidence: 1.0,
				Reasoning:  fmt.Sprintf("Profit target hit: %.1f%%", pnlPct),
			})
			a.closePosition(pos.MarketID)
			continue
		}

		// Check stop loss
		if pnlPct <= -a.stopLoss*100 {
			log.Info().
				Str("market", pos.MarketID).
				Float64("pnl_pct", pnlPct).
				Msg("Stop loss hit, closing position")

			exitSignals = append(exitSignals, &TradeSignal{
				Timestamp:  time.Now(),
				MarketID:   pos.MarketID,
				Question:   pos.Question,
				Signal:     "SELL",
				Side:       pos.Side,
				Price:      currentPrice,
				Size:       pos.Size,
				Confidence: 1.0,
				Reasoning:  fmt.Sprintf("Stop loss hit: %.1f%%", pnlPct),
			})
			a.closePosition(pos.MarketID)
			continue
		}

		// Check if resolution is imminent (< 2 hours) — close to avoid resolution risk
		if market.DaysUntilResolution() < 0.083 { // ~2 hours
			log.Info().
				Str("market", pos.MarketID).
				Float64("days", market.DaysUntilResolution()).
				Msg("Resolution imminent, closing position")

			exitSignals = append(exitSignals, &TradeSignal{
				Timestamp:  time.Now(),
				MarketID:   pos.MarketID,
				Question:   pos.Question,
				Signal:     "SELL",
				Side:       pos.Side,
				Price:      currentPrice,
				Size:       pos.Size,
				Confidence: 0.9,
				Reasoning:  "Resolution imminent, closing to avoid resolution risk",
			})
			a.closePosition(pos.MarketID)
		}
	}

	return exitSignals
}

// fetchMarketByID fetches a single market by ID
func (a *PolymarketAgent) fetchMarketByID(ctx context.Context, marketID string) (*PolyMarket, error) {
	result, err := a.CallMCPTool(ctx, "polymarket", "get_market", map[string]interface{}{
		"market_id": marketID,
	})
	if err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from get_market")
	}

	var textContent string
	for _, c := range result.Content {
		raw, _ := json.Marshal(c)
		var obj map[string]interface{}
		if json.Unmarshal(raw, &obj) == nil {
			if t, ok := obj["text"].(string); ok {
				textContent = t
				break
			}
		}
	}

	if textContent == "" {
		return nil, fmt.Errorf("no text content in MCP result")
	}

	var market PolyMarket
	if err := json.Unmarshal([]byte(textContent), &market); err != nil {
		return nil, fmt.Errorf("failed to parse market: %w", err)
	}

	return &market, nil
}

// ============================================================================
// SIGNAL PUBLISHING
// ============================================================================

// publishSignal publishes a trade signal to NATS
func (a *PolymarketAgent) publishSignal(signal *TradeSignal) error {
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	if err := a.natsConn.Publish(a.natsTopic, data); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	log.Info().
		Str("signal", signal.Signal).
		Str("market", signal.MarketID).
		Float64("confidence", signal.Confidence).
		Msg("Published polymarket signal")

	return nil
}

// ============================================================================
// DIRECT EXECUTION
// ============================================================================

// executeSignal directly executes a trade signal via the PolymarketExchange adapter.
func (a *PolymarketAgent) executeSignal(ctx context.Context, signal *TradeSignal) error {
	var side exchange.OrderSide
	if signal.Side == "YES" {
		side = exchange.OrderSideBuy
	} else {
		side = exchange.OrderSideSell
	}

	req := exchange.PlaceOrderRequest{
		Symbol:   signal.MarketID,
		Side:     side,
		Type:     exchange.OrderTypeLimit,
		Quantity: signal.Size / signal.Price, // Convert USD size to shares
		Price:    signal.Price,
	}

	resp, err := a.polyExchange.PlaceOrder(ctx, req)
	if err != nil {
		return fmt.Errorf("place order: %w", err)
	}

	log.Info().
		Str("order_id", resp.OrderID).
		Str("status", string(resp.Status)).
		Str("market", signal.MarketID).
		Str("side", signal.Side).
		Float64("price", signal.Price).
		Float64("size", signal.Size).
		Msg("Signal executed directly via PolymarketExchange")

	return nil
}

// ============================================================================
// MAIN ENTRY POINT
// ============================================================================

func main() {
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
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Extract agent configuration
	polyConfig := viper.Sub("strategy_agents.polymarket")
	if polyConfig == nil {
		log.Fatal().Msg("Polymarket agent configuration not found in agents.yaml")
	}

	var agentConfig agents.AgentConfig
	agentConfig.Name = polyConfig.GetString("name")
	agentConfig.Type = polyConfig.GetString("type")
	agentConfig.Version = polyConfig.GetString("version")
	agentConfig.Enabled = polyConfig.GetBool("enabled")

	stepIntervalStr := polyConfig.GetString("step_interval")
	stepInterval, err := time.ParseDuration(stepIntervalStr)
	if err != nil {
		stepInterval = 5 * time.Minute
	}
	agentConfig.StepInterval = stepInterval

	configMap := polyConfig.Get("config")
	if configMap != nil {
		if cm, ok := configMap.(map[string]interface{}); ok {
			agentConfig.Config = cm
		}
	}
	if agentConfig.Config == nil {
		agentConfig.Config = make(map[string]interface{})
	}

	// Parse MCP servers
	mcpServers := polyConfig.Get("mcp_servers")
	if mcpServers != nil {
		if servers, ok := mcpServers.([]interface{}); ok {
			agentConfig.MCPServers = make([]agents.MCPServerConfig, 0, len(servers))
			for _, srv := range servers {
				if server, ok := srv.(map[string]interface{}); ok {
					sc := agents.MCPServerConfig{}
					if n, ok := server["name"].(string); ok {
						sc.Name = n
					}
					if t, ok := server["type"].(string); ok {
						sc.Type = t
					}
					if u, ok := server["url"].(string); ok {
						sc.URL = u
					}
					agentConfig.MCPServers = append(agentConfig.MCPServers, sc)
				}
			}
		}
	}

	metricsPort := viper.GetInt("strategy_agents.polymarket.metrics_port")
	if metricsPort == 0 {
		metricsPort = 9109
	}

	baseAgent := agents.NewBaseAgent(&agentConfig, log.Logger, metricsPort)

	// Extract polymarket-specific config with defaults
	pollInterval := polyConfig.GetDuration("config.poll_interval")
	if pollInterval == 0 {
		pollInterval = 5 * time.Minute
	}

	maxPositionSize := polyConfig.GetFloat64("config.max_position_size")
	if maxPositionSize == 0 {
		maxPositionSize = 10.0
	}

	maxTotalExposure := polyConfig.GetFloat64("config.max_total_exposure")
	if maxTotalExposure == 0 {
		maxTotalExposure = 50.0
	}

	confidenceThreshold := polyConfig.GetFloat64("config.confidence_threshold")
	if confidenceThreshold == 0 {
		confidenceThreshold = 0.70
	}

	profitTarget := polyConfig.GetFloat64("config.profit_target")
	if profitTarget == 0 {
		profitTarget = 0.15
	}

	stopLoss := polyConfig.GetFloat64("config.stop_loss")
	if stopLoss == 0 {
		stopLoss = 0.10
	}

	preferredCategories := polyConfig.GetStringSlice("config.preferred_categories")
	if len(preferredCategories) == 0 {
		preferredCategories = []string{"politics", "crypto", "tech", "news"}
	}

	// Check --execute flag
	executeMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--execute" {
			executeMode = true
		}
	}

	// Initialize PolymarketExchange if in execute mode
	var polyExchange *exchange.PolymarketExchange
	if executeMode {
		polyPrivateKey := os.Getenv("POLYMARKET_PRIVATE_KEY")
		polyMode := os.Getenv("POLYMARKET_MODE")
		if polyMode == "" {
			polyMode = "paper" // Default to paper for safety
		}

		pe, err := exchange.NewPolymarketExchange(polyPrivateKey, polyMode == "paper", nil)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize PolymarketExchange for execute mode")
		}
		polyExchange = pe
		log.Info().Str("mode", polyMode).Msg("Execute mode enabled with PolymarketExchange")
	}

	agent := &PolymarketAgent{
		BaseAgent:           baseAgent,
		executeMode:         executeMode,
		polyExchange:        polyExchange,
		pollInterval:        pollInterval,
		maxPositionSize:     maxPositionSize,
		maxTotalExposure:    maxTotalExposure,
		confidenceThreshold: confidenceThreshold,
		profitTarget:        profitTarget,
		stopLoss:            stopLoss,
		minLiquidity:        polyConfig.GetFloat64("config.min_liquidity"),
		maxSpread:           0.10,
		minDaysToResolution: 1.0,
		maxDaysToResolution: 7.0,
		preferredCategories: preferredCategories,
	}
	if agent.minLiquidity == 0 {
		agent.minLiquidity = 1000.0
	}

	// Signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	// Share the agent's database connection with the exchange to avoid duplicate connections
	if agent.database != nil && agent.polyExchange != nil {
		agent.polyExchange.SetDB(agent.database)
	}

	agent.heartbeat.Start()

	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("Agent runtime error")
	}

	agent.heartbeat.Stop()

	if err := agent.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("Polymarket Agent terminated")
}
