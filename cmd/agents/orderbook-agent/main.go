// Order Book Analysis Agent
// Generates market insights from order book depth and imbalances
//
//nolint:goconst // Trading signals (BUY/SELL/HOLD) are domain-specific strings
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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
)

// Helper function for float64 min
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// OrderBookAgent analyzes order book depth and generates trading signals
type OrderBookAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn       *nats.Conn
	natsTopic      string
	heartbeatTopic string
	heartbeatStop  chan struct{}

	// Configuration
	symbol               string
	depthLevels          int
	largeOrderMultiplier float64
	imbalanceThreshold   float64
	spoofingWindow       time.Duration

	// BDI belief system
	beliefs *BeliefBase

	// Order book tracking for spoofing detection
	orderBookHistory []OrderBookSnapshot
	historyMutex     sync.RWMutex
}

// OrderBook represents the order book data structure
type OrderBook struct {
	Symbol    string           `json:"symbol"`
	Timestamp int64            `json:"timestamp"`
	Bids      []OrderBookLevel `json:"bids"` // [price, quantity]
	Asks      []OrderBookLevel `json:"asks"` // [price, quantity]
}

// OrderBookLevel represents a single price level in the order book
type OrderBookLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// OrderBookSnapshot stores a snapshot of the order book for historical tracking
type OrderBookSnapshot struct {
	Timestamp time.Time
	OrderBook *OrderBook
}

// PriceLevel represents a support/resistance level with cumulative volume
type PriceLevel struct {
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

// LargeOrder represents a large order detected in the order book
type LargeOrder struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
	Side  string  `json:"side"` // "bid" or "ask"
}

// OrderBookSignal represents a trading signal from order book analysis
type OrderBookSignal struct {
	Timestamp        time.Time    `json:"timestamp"`
	Symbol           string       `json:"symbol"`
	Action           string       `json:"action"` // BUY, SELL, HOLD
	Confidence       float64      `json:"confidence"`
	Imbalance        float64      `json:"imbalance"`
	SupportLevels    []PriceLevel `json:"support_levels"`
	ResistanceLevels []PriceLevel `json:"resistance_levels"`
	LargeOrders      []LargeOrder `json:"large_orders"`
	SpoofingFlags    int          `json:"spoofing_flags"`
	Reasoning        string       `json:"reasoning"`
	Price            float64      `json:"price"`
}

// Belief represents a single belief in the agent's belief base
type Belief struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Confidence float64     `json:"confidence"`
	Timestamp  time.Time   `json:"timestamp"`
	Source     string      `json:"source"`
}

// BeliefBase represents the agent's beliefs about the market
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

// NewOrderBookAgent creates a new order book analysis agent
func NewOrderBookAgent(config *agents.AgentConfig, log zerolog.Logger, metricsPort int) (*OrderBookAgent, error) {
	baseAgent := agents.NewBaseAgent(config, log, metricsPort)

	// Extract configuration
	agentConfig := config.Config

	// Read NATS configuration
	natsURL := viper.GetString("nats.url")
	if natsURL == "" {
		natsURL = "nats://localhost:4222" // Default
	}

	natsTopic := viper.GetString("communication.nats.topics.orderbook_signals")
	if natsTopic == "" {
		natsTopic = "agents.analysis.orderbook" // Default
	}

	// Connect to NATS
	log.Info().Str("url", natsURL).Str("topic", natsTopic).Msg("Connecting to NATS")
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info().Msg("Successfully connected to NATS")

	// Get heartbeat topic for agent registration with orchestrator
	heartbeatTopic := viper.GetString("analysis_agents.orderbook.heartbeat_topic")
	if heartbeatTopic == "" {
		heartbeatTopic = "cryptofunk.agent.heartbeat" // Default - matches orchestrator
	}

	// Parse configuration
	symbol := getStringConfig(agentConfig, "symbol", "bitcoin")
	depthLevels := getIntConfig(agentConfig, "depth_levels", 20)
	largeOrderMultiplier := getFloatConfig(agentConfig, "large_order_multiplier", 5.0)
	imbalanceThreshold := getFloatConfig(agentConfig, "imbalance_threshold", 0.3)

	spoofingWindowStr := getStringConfig(agentConfig, "spoofing_window", "5s")
	spoofingWindow, err := time.ParseDuration(spoofingWindowStr)
	if err != nil {
		log.Warn().Err(err).Str("value", spoofingWindowStr).Msg("Invalid spoofing_window, using default 5s")
		spoofingWindow = 5 * time.Second
	}

	return &OrderBookAgent{
		BaseAgent:            baseAgent,
		natsConn:             nc,
		natsTopic:            natsTopic,
		heartbeatTopic:       heartbeatTopic,
		heartbeatStop:        make(chan struct{}),
		symbol:               symbol,
		depthLevels:          depthLevels,
		largeOrderMultiplier: largeOrderMultiplier,
		imbalanceThreshold:   imbalanceThreshold,
		spoofingWindow:       spoofingWindow,
		beliefs:              NewBeliefBase(),
		orderBookHistory:     make([]OrderBookSnapshot, 0, 100),
	}, nil
}

// Step performs a single decision cycle - order book analysis
func (a *OrderBookAgent) Step(ctx context.Context) error {
	// Call parent Step to handle metrics
	if err := a.BaseAgent.Step(ctx); err != nil {
		return err
	}

	log.Debug().Msg("Executing order book analysis step")

	// Step 1: Fetch order book data
	orderBook, err := a.fetchOrderBook(ctx, a.symbol)
	if err != nil {
		log.Error().Err(err).Str("symbol", a.symbol).Msg("Failed to fetch order book")
		return fmt.Errorf("failed to fetch order book: %w", err)
	}

	if orderBook == nil || len(orderBook.Bids) == 0 || len(orderBook.Asks) == 0 {
		log.Warn().Str("symbol", a.symbol).Msg("Empty order book data")
		return nil
	}

	// Store snapshot for spoofing detection
	a.addOrderBookSnapshot(orderBook)

	// Calculate current price (mid-price)
	currentPrice := (orderBook.Bids[0].Price + orderBook.Asks[0].Price) / 2.0

	log.Debug().
		Str("symbol", a.symbol).
		Float64("price", currentPrice).
		Int("bid_levels", len(orderBook.Bids)).
		Int("ask_levels", len(orderBook.Asks)).
		Msg("Order book fetched")

	// Step 2: Calculate bid-ask imbalance
	imbalance := a.calculateImbalance(orderBook)
	a.beliefs.UpdateBelief("order_imbalance", imbalance, math.Abs(imbalance), "order_book")

	// Step 3: Analyze depth (support/resistance levels)
	supportLevels := a.analyzeDepth(orderBook.Bids, true)
	resistanceLevels := a.analyzeDepth(orderBook.Asks, false)
	a.beliefs.UpdateBelief("support_levels", supportLevels, calculateDepthConfidence(supportLevels), "order_book_depth")
	a.beliefs.UpdateBelief("resistance_levels", resistanceLevels, calculateDepthConfidence(resistanceLevels), "order_book_depth")

	// Step 4: Detect large orders
	largeOrders := a.detectLargeOrders(orderBook)
	a.beliefs.UpdateBelief("large_orders", largeOrders, calculateLargeOrderConfidence(largeOrders), "order_book_orders")

	// Step 5: Detect potential spoofing
	spoofingFlags := a.detectSpoofing()
	a.beliefs.UpdateBelief("spoofing_flags", spoofingFlags, calculateSpoofingConfidence(spoofingFlags), "spoofing_detection")

	// Update market state beliefs
	a.beliefs.UpdateBelief("current_price", currentPrice, 1.0, "order_book")
	a.beliefs.UpdateBelief("symbol", a.symbol, 1.0, "config")

	log.Info().
		Float64("imbalance", imbalance).
		Int("support_levels", len(supportLevels)).
		Int("resistance_levels", len(resistanceLevels)).
		Int("large_orders", len(largeOrders)).
		Int("spoofing_flags", spoofingFlags).
		Msg("Order book analysis complete")

	// Step 6: Generate trading signal
	signal, err := a.generateSignal(ctx, orderBook, imbalance, supportLevels, resistanceLevels, largeOrders, spoofingFlags, currentPrice)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate signal")
		return fmt.Errorf("signal generation failed: %w", err)
	}

	// Log the generated signal
	log.Info().
		Str("symbol", signal.Symbol).
		Str("signal", signal.Action).
		Float64("confidence", signal.Confidence).
		Str("reasoning", signal.Reasoning).
		Msg("Order book signal generated")

	// Step 7: Publish signal to NATS
	if err := a.publishSignal(ctx, signal); err != nil {
		log.Error().Err(err).Msg("Failed to publish signal to NATS")
	}

	return nil
}

// fetchOrderBook fetches order book data from Market Data Server
func (a *OrderBookAgent) fetchOrderBook(ctx context.Context, symbol string) (*OrderBook, error) {
	log.Debug().Str("symbol", symbol).Msg("Fetching order book from Market Data Server")

	// Call Market Data Server MCP tool
	result, err := a.CallMCPTool(ctx, "market_data", "get_order_book", map[string]interface{}{
		"symbol": symbol,
		"depth":  a.depthLevels,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from MCP result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from Market Data Server")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type from Market Data Server")
	}

	// Parse JSON result
	var orderBook OrderBook
	if err := json.Unmarshal([]byte(textContent.Text), &orderBook); err != nil {
		return nil, fmt.Errorf("failed to parse order book response: %w", err)
	}

	return &orderBook, nil
}

// calculateImbalance calculates the bid-ask imbalance ratio
// Returns a value between -1 (sell pressure) and +1 (buy pressure)
func (a *OrderBookAgent) calculateImbalance(orderBook *OrderBook) float64 {
	bidVolume := 0.0
	askVolume := 0.0

	// Sum volumes for configured depth levels
	maxLevels := min(a.depthLevels, min(len(orderBook.Bids), len(orderBook.Asks)))

	for i := 0; i < maxLevels; i++ {
		bidVolume += orderBook.Bids[i].Quantity
		askVolume += orderBook.Asks[i].Quantity
	}

	totalVolume := bidVolume + askVolume
	if totalVolume == 0 {
		return 0.0
	}

	// Imbalance = (bid_volume - ask_volume) / (bid_volume + ask_volume)
	imbalance := (bidVolume - askVolume) / totalVolume

	log.Debug().
		Float64("bid_volume", bidVolume).
		Float64("ask_volume", askVolume).
		Float64("imbalance", imbalance).
		Msg("Imbalance calculated")

	return imbalance
}

// analyzeDepth identifies significant support/resistance levels
func (a *OrderBookAgent) analyzeDepth(levels []OrderBookLevel, isBid bool) []PriceLevel {
	if len(levels) == 0 {
		return nil
	}

	// Calculate cumulative volume
	cumulativeVolume := 0.0

	for _, level := range levels {
		cumulativeVolume += level.Quantity
	}

	// Find levels with significant volume (>20% of total volume)
	totalVolume := cumulativeVolume
	significantLevels := make([]PriceLevel, 0)

	for i, level := range levels {
		if level.Quantity > totalVolume*0.05 { // Levels with >5% of total volume
			significantLevels = append(significantLevels, PriceLevel{
				Price:  level.Price,
				Volume: level.Quantity,
			})
		}

		// Only check first 10 levels to avoid too many results
		if i >= 10 {
			break
		}
	}

	// Sort by volume (descending) and return top 3
	sort.Slice(significantLevels, func(i, j int) bool {
		return significantLevels[i].Volume > significantLevels[j].Volume
	})

	if len(significantLevels) > 3 {
		significantLevels = significantLevels[:3]
	}

	side := "support"
	if !isBid {
		side = "resistance"
	}

	log.Debug().
		Str("side", side).
		Int("significant_levels", len(significantLevels)).
		Msg("Depth analysis complete")

	return significantLevels
}

// detectLargeOrders identifies orders significantly larger than average
func (a *OrderBookAgent) detectLargeOrders(orderBook *OrderBook) []LargeOrder {
	largeOrders := make([]LargeOrder, 0)

	// Calculate average order size across both sides
	totalSize := 0.0
	count := 0

	for _, bid := range orderBook.Bids {
		totalSize += bid.Quantity
		count++
	}
	for _, ask := range orderBook.Asks {
		totalSize += ask.Quantity
		count++
	}

	if count == 0 {
		return largeOrders
	}

	averageSize := totalSize / float64(count)
	threshold := averageSize * a.largeOrderMultiplier

	log.Debug().
		Float64("average_size", averageSize).
		Float64("threshold", threshold).
		Msg("Large order detection threshold calculated")

	// Check bids for large orders
	for _, bid := range orderBook.Bids {
		if bid.Quantity >= threshold {
			largeOrders = append(largeOrders, LargeOrder{
				Price: bid.Price,
				Size:  bid.Quantity,
				Side:  "bid",
			})
		}
	}

	// Check asks for large orders
	for _, ask := range orderBook.Asks {
		if ask.Quantity >= threshold {
			largeOrders = append(largeOrders, LargeOrder{
				Price: ask.Price,
				Size:  ask.Quantity,
				Side:  "ask",
			})
		}
	}

	log.Debug().
		Int("large_orders", len(largeOrders)).
		Msg("Large order detection complete")

	return largeOrders
}

// detectSpoofing detects potential spoofing behavior
// Basic implementation: tracks orders that appear and disappear quickly
func (a *OrderBookAgent) detectSpoofing() int {
	a.historyMutex.RLock()
	defer a.historyMutex.RUnlock()

	if len(a.orderBookHistory) < 2 {
		return 0
	}

	spoofingFlags := 0
	now := time.Now()

	// Get recent snapshot and compare with current
	for i := len(a.orderBookHistory) - 1; i >= 0; i-- {
		snapshot := a.orderBookHistory[i]

		// Only look at snapshots within the spoofing window
		if now.Sub(snapshot.Timestamp) > a.spoofingWindow {
			break
		}

		// Compare with next snapshot (if exists)
		if i > 0 {
			prevSnapshot := a.orderBookHistory[i-1]
			spoofingFlags += a.compareSnapshotsForSpoofing(prevSnapshot.OrderBook, snapshot.OrderBook)
		}
	}

	log.Debug().
		Int("spoofing_flags", spoofingFlags).
		Int("history_size", len(a.orderBookHistory)).
		Msg("Spoofing detection complete")

	return spoofingFlags
}

// compareSnapshotsForSpoofing compares two order book snapshots to detect spoofing
func (a *OrderBookAgent) compareSnapshotsForSpoofing(prev, current *OrderBook) int {
	flags := 0

	// Build map of current orders
	currentOrders := make(map[string]bool)
	for _, bid := range current.Bids {
		key := fmt.Sprintf("bid_%.8f_%.8f", bid.Price, bid.Quantity)
		currentOrders[key] = true
	}
	for _, ask := range current.Asks {
		key := fmt.Sprintf("ask_%.8f_%.8f", ask.Price, ask.Quantity)
		currentOrders[key] = true
	}

	// Check if large orders from previous snapshot disappeared
	avgBidSize := calculateAverageSize(prev.Bids)
	avgAskSize := calculateAverageSize(prev.Asks)

	for _, bid := range prev.Bids {
		if bid.Quantity >= avgBidSize*a.largeOrderMultiplier {
			key := fmt.Sprintf("bid_%.8f_%.8f", bid.Price, bid.Quantity)
			if !currentOrders[key] {
				flags++
			}
		}
	}

	for _, ask := range prev.Asks {
		if ask.Quantity >= avgAskSize*a.largeOrderMultiplier {
			key := fmt.Sprintf("ask_%.8f_%.8f", ask.Price, ask.Quantity)
			if !currentOrders[key] {
				flags++
			}
		}
	}

	return flags
}

// addOrderBookSnapshot adds a new snapshot to the history
func (a *OrderBookAgent) addOrderBookSnapshot(orderBook *OrderBook) {
	a.historyMutex.Lock()
	defer a.historyMutex.Unlock()

	snapshot := OrderBookSnapshot{
		Timestamp: time.Now(),
		OrderBook: orderBook,
	}

	a.orderBookHistory = append(a.orderBookHistory, snapshot)

	// Keep only recent history (last 100 snapshots)
	if len(a.orderBookHistory) > 100 {
		a.orderBookHistory = a.orderBookHistory[1:]
	}
}

// generateSignal combines all order book signals to produce a trading signal
func (a *OrderBookAgent) generateSignal(
	ctx context.Context,
	orderBook *OrderBook,
	imbalance float64,
	supportLevels []PriceLevel,
	resistanceLevels []PriceLevel,
	largeOrders []LargeOrder,
	spoofingFlags int,
	currentPrice float64,
) (*OrderBookSignal, error) {
	log.Debug().Str("symbol", a.symbol).Msg("Generating signal from order book analysis")

	// Component weights
	const (
		imbalanceWeight  = 0.40
		depthWeight      = 0.30
		largeOrderWeight = 0.20
		spoofingWeight   = 0.10
	)

	var signals []string
	var confidences []float64
	var weights []float64
	var reasoningParts []string

	// 1. Imbalance Signal
	imbalanceSignal, imbalanceConfidence, imbalanceReason := analyzeImbalance(imbalance, a.imbalanceThreshold)
	signals = append(signals, imbalanceSignal)
	confidences = append(confidences, imbalanceConfidence)
	weights = append(weights, imbalanceWeight)
	reasoningParts = append(reasoningParts, imbalanceReason)

	// 2. Depth Analysis Signal
	depthSignal, depthConfidence, depthReason := analyzeDepthProximity(currentPrice, supportLevels, resistanceLevels)
	signals = append(signals, depthSignal)
	confidences = append(confidences, depthConfidence)
	weights = append(weights, depthWeight)
	reasoningParts = append(reasoningParts, depthReason)

	// 3. Large Order Signal
	largeOrderSignal, largeOrderConfidence, largeOrderReason := analyzeLargeOrders(largeOrders)
	signals = append(signals, largeOrderSignal)
	confidences = append(confidences, largeOrderConfidence)
	weights = append(weights, largeOrderWeight)
	reasoningParts = append(reasoningParts, largeOrderReason)

	// 4. Spoofing Penalty
	spoofingSignal, spoofingConfidence, spoofingReason := analyzeSpoofing(spoofingFlags)
	signals = append(signals, spoofingSignal)
	confidences = append(confidences, spoofingConfidence)
	weights = append(weights, spoofingWeight)
	reasoningParts = append(reasoningParts, spoofingReason)

	// Combine signals with weighted confidence
	finalSignal, finalConfidence := combineSignals(signals, confidences, weights)

	// Build reasoning string
	reasoning := strings.Join(reasoningParts, "; ")

	signal := &OrderBookSignal{
		Timestamp:        time.Now(),
		Symbol:           a.symbol,
		Action:           finalSignal,
		Confidence:       finalConfidence,
		Imbalance:        imbalance,
		SupportLevels:    supportLevels,
		ResistanceLevels: resistanceLevels,
		LargeOrders:      largeOrders,
		SpoofingFlags:    spoofingFlags,
		Reasoning:        reasoning,
		Price:            currentPrice,
	}

	log.Debug().
		Str("signal", finalSignal).
		Float64("confidence", finalConfidence).
		Msg("Signal generated")

	return signal, nil
}

// analyzeImbalance interprets imbalance ratio to generate a signal
func analyzeImbalance(imbalance, threshold float64) (signal string, confidence float64, reasoning string) {
	absImbalance := math.Abs(imbalance)

	if imbalance > threshold {
		// Strong buy pressure
		confidence = minFloat(absImbalance, 1.0)
		return "BUY", confidence, fmt.Sprintf("Strong buy pressure (imbalance: %.2f)", imbalance)
	} else if imbalance < -threshold {
		// Strong sell pressure
		confidence = minFloat(absImbalance, 1.0)
		return "SELL", confidence, fmt.Sprintf("Strong sell pressure (imbalance: %.2f)", imbalance)
	} else {
		// Balanced order book
		confidence = 1.0 - absImbalance // Higher confidence when more balanced
		return "HOLD", confidence, fmt.Sprintf("Balanced order book (imbalance: %.2f)", imbalance)
	}
}

// analyzeDepthProximity analyzes proximity to support/resistance levels
func analyzeDepthProximity(currentPrice float64, supportLevels, resistanceLevels []PriceLevel) (signal string, confidence float64, reasoning string) {
	if len(supportLevels) == 0 && len(resistanceLevels) == 0 {
		return "HOLD", 0.3, "Insufficient depth data"
	}

	// Find closest support and resistance
	var closestSupport, closestResistance *PriceLevel
	minSupportDist := math.MaxFloat64
	minResistanceDist := math.MaxFloat64

	for i := range supportLevels {
		dist := math.Abs(currentPrice - supportLevels[i].Price)
		if dist < minSupportDist {
			minSupportDist = dist
			closestSupport = &supportLevels[i]
		}
	}

	for i := range resistanceLevels {
		dist := math.Abs(currentPrice - resistanceLevels[i].Price)
		if dist < minResistanceDist {
			minResistanceDist = dist
			closestResistance = &resistanceLevels[i]
		}
	}

	// Calculate relative distances (as percentage)
	supportDist := minSupportDist / currentPrice
	resistanceDist := minResistanceDist / currentPrice

	if closestSupport != nil && supportDist < 0.01 { // Within 1% of support
		confidence = 1.0 - (supportDist / 0.01) // Higher confidence closer to support
		return "BUY", confidence, fmt.Sprintf("Near strong support (%.2f)", closestSupport.Price)
	} else if closestResistance != nil && resistanceDist < 0.01 { // Within 1% of resistance
		confidence = 1.0 - (resistanceDist / 0.01)
		return "SELL", confidence, fmt.Sprintf("Near strong resistance (%.2f)", closestResistance.Price)
	}

	return "HOLD", 0.5, "Not near significant levels"
}

// analyzeLargeOrders interprets large order presence
func analyzeLargeOrders(largeOrders []LargeOrder) (signal string, confidence float64, reasoning string) {
	if len(largeOrders) == 0 {
		return "HOLD", 0.3, "No significant large orders"
	}

	bidCount := 0
	askCount := 0
	totalBidSize := 0.0
	totalAskSize := 0.0

	for _, order := range largeOrders {
		if order.Side == "bid" {
			bidCount++
			totalBidSize += order.Size
		} else {
			askCount++
			totalAskSize += order.Size
		}
	}

	if bidCount > askCount && totalBidSize > totalAskSize*1.5 {
		// Large buy walls indicate support
		confidence = minFloat(float64(bidCount)/float64(len(largeOrders)), 1.0)
		return "BUY", confidence, fmt.Sprintf("%d large buy orders detected", bidCount)
	} else if askCount > bidCount && totalAskSize > totalBidSize*1.5 {
		// Large sell walls indicate resistance
		confidence = minFloat(float64(askCount)/float64(len(largeOrders)), 1.0)
		return "SELL", confidence, fmt.Sprintf("%d large sell orders detected", askCount)
	}

	return "HOLD", 0.5, fmt.Sprintf("%d large orders (mixed)", len(largeOrders))
}

// analyzeSpoofing interprets spoofing flags
func analyzeSpoofing(spoofingFlags int) (signal string, confidence float64, reasoning string) {
	if spoofingFlags == 0 {
		return "HOLD", 1.0, "No spoofing detected"
	}

	if spoofingFlags >= 5 {
		// High spoofing activity - reduce confidence in all signals
		return "HOLD", 0.0, fmt.Sprintf("High spoofing activity detected (%d flags)", spoofingFlags)
	}

	// Moderate spoofing - reduce confidence proportionally
	confidence = 1.0 - (float64(spoofingFlags) / 5.0)
	return "HOLD", confidence, fmt.Sprintf("Potential spoofing detected (%d flags)", spoofingFlags)
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
	maxScore := buyScore
	finalSignal = "BUY"

	if sellScore > maxScore {
		maxScore = sellScore
		finalSignal = "SELL"
	}

	if holdScore > maxScore {
		maxScore = holdScore
		finalSignal = "HOLD"
	}

	finalConfidence = maxScore
	return finalSignal, finalConfidence
}

// startHeartbeat starts the heartbeat publishing goroutine
func (a *OrderBookAgent) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		// Publish immediately on start
		a.publishHeartbeat()
		for {
			select {
			case <-ticker.C:
				a.publishHeartbeat()
			case <-a.heartbeatStop:
				ticker.Stop()
				return
			}
		}
	}()
	log.Info().Str("topic", a.heartbeatTopic).Msg("Heartbeat publishing started")
}

// publishHeartbeat publishes a heartbeat message to the orchestrator
func (a *OrderBookAgent) publishHeartbeat() {
	heartbeat := struct {
		AgentName string    `json:"agent_name"`
		AgentType string    `json:"agent_type"`
		Timestamp time.Time `json:"timestamp"`
		Status    string    `json:"status"`
	}{
		AgentName: a.GetConfig().Name,
		AgentType: a.GetConfig().Type,
		Timestamp: time.Now(),
		Status:    "healthy",
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal heartbeat")
		return
	}

	if err := a.natsConn.Publish(a.heartbeatTopic, data); err != nil {
		log.Error().Err(err).Msg("Failed to publish heartbeat")
		return
	}

	log.Debug().Str("topic", a.heartbeatTopic).Msg("Heartbeat published")
}

// stopHeartbeat stops the heartbeat publishing goroutine
func (a *OrderBookAgent) stopHeartbeat() {
	close(a.heartbeatStop)
}

// publishSignal publishes an order book signal to NATS
func (a *OrderBookAgent) publishSignal(ctx context.Context, signal *OrderBookSignal) error {
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
		Str("signal", signal.Action).
		Float64("confidence", signal.Confidence).
		Msg("Signal published to NATS")

	return nil
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func calculateAverageSize(levels []OrderBookLevel) float64 {
	if len(levels) == 0 {
		return 0.0
	}

	total := 0.0
	for _, level := range levels {
		total += level.Quantity
	}
	return total / float64(len(levels))
}

func calculateDepthConfidence(levels []PriceLevel) float64 {
	if len(levels) == 0 {
		return 0.0
	}
	// More significant levels = higher confidence
	numLevels := len(levels)
	if numLevels > 3 {
		numLevels = 3
	}
	return float64(numLevels) / 3.0
}

func calculateLargeOrderConfidence(orders []LargeOrder) float64 {
	if len(orders) == 0 {
		return 0.3
	}
	// More large orders = higher confidence (up to 5 orders)
	numOrders := len(orders)
	if numOrders > 5 {
		numOrders = 5
	}
	return float64(numOrders) / 5.0
}

func calculateSpoofingConfidence(flags int) float64 {
	if flags == 0 {
		return 1.0
	}
	// More spoofing flags = lower confidence
	return math.Max(0.0, 1.0-(float64(flags)/10.0))
}

func getStringConfig(config map[string]interface{}, key string, defaultVal string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultVal
}

func getIntConfig(config map[string]interface{}, key string, defaultVal int) int {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

func getFloatConfig(config map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

func main() {
	// Configure logging to stderr (stdout reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	viper.SetConfigName("agents")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../../../configs") // From cmd/agents/orderbook-agent/

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	// Extract orderbook agent configuration
	var agentConfig agents.AgentConfig

	orderbookConfig := viper.Sub("analysis_agents.orderbook")
	if orderbookConfig == nil {
		log.Fatal().Msg("Order book agent configuration not found in agents.yaml")
	}

	agentConfig.Name = orderbookConfig.GetString("name")
	agentConfig.Type = orderbookConfig.GetString("type")
	agentConfig.Version = orderbookConfig.GetString("version")
	agentConfig.Enabled = orderbookConfig.GetBool("enabled")

	// Parse step interval
	stepIntervalStr := orderbookConfig.GetString("step_interval")
	stepInterval, err := time.ParseDuration(stepIntervalStr)
	if err != nil {
		log.Fatal().Err(err).Str("interval", stepIntervalStr).Msg("Invalid step_interval")
	}
	agentConfig.StepInterval = stepInterval

	// Get agent-specific config
	agentConfig.Config = orderbookConfig.Get("config").(map[string]interface{})

	// Get metrics port - use agent-specific port to avoid conflicts
	// Orderbook agent uses port 9105
	metricsPort := viper.GetInt("analysis_agents.orderbook.metrics_port")
	if metricsPort == 0 {
		metricsPort = 9105 // Default port for orderbook-agent
	}

	// Get MCP server configurations
	mcpServers := orderbookConfig.Get("mcp_servers")
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
					log.Info().
						Str("name", serverConfig.Name).
						Str("type", serverConfig.Type).
						Msg("Configured MCP server")
				}
			}
		}
	}

	// Create agent
	agent, err := NewOrderBookAgent(&agentConfig, log.Logger, metricsPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create orderbook agent")
	}

	log.Info().
		Str("name", agentConfig.Name).
		Str("type", agentConfig.Type).
		Str("version", agentConfig.Version).
		Dur("step_interval", agentConfig.StepInterval).
		Int("metrics_port", metricsPort).
		Msg("Starting order book analysis agent")

	// Initialize agent
	ctx := context.Background()
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	// Start heartbeat publishing for orchestrator registration
	agent.startHeartbeat()

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

	// Stop heartbeat publishing
	agent.stopHeartbeat()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := agent.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Order book analysis agent shutdown complete")
}
