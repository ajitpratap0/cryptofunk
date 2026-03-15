package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

// ExecutorConfig holds configuration for the decision-to-order executor.
type ExecutorConfig struct {
	OrderExecutorURL string  // MCP endpoint for the order-executor server
	MinConfidence    float64 // Minimum confidence to place an order
	MinConsensus     float64 // Minimum consensus to place an order
	DefaultQuantity  float64 // Default order quantity (e.g. 0.001 BTC)
}

// Executor bridges orchestrator decisions to order execution via MCP.
// It subscribes to NATS decision topics and places orders through the
// order-executor MCP server.
type Executor struct {
	config       ExecutorConfig
	natsConn     *nats.Conn
	mcpClient    *mcp.Client
	session      *mcp.ClientSession
	sessionMu    sync.Mutex
	connecting   bool
	connectReady chan struct{} // signals when a concurrent connect finishes
	subscription *nats.Subscription
}

// NewExecutor creates a new decision-to-order executor.
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config: config,
	}
}

// ShouldExecute returns true if the decision warrants placing an order:
// action must be BUY or SELL, and confidence/consensus must meet thresholds.
func (e *Executor) ShouldExecute(decision *TradingDecision) bool {
	if decision == nil {
		return false
	}
	if decision.Action != SignalActionBuy && decision.Action != SignalActionSell {
		return false
	}
	if decision.Confidence < e.config.MinConfidence {
		return false
	}
	if decision.Consensus < e.config.MinConsensus {
		return false
	}
	return true
}

// Start initializes the MCP client and subscribes to the NATS decision topic.
// If the order-executor MCP server is unavailable, it logs a warning but still
// subscribes (it will retry MCP connection on first order).
func (e *Executor) Start(natsConn *nats.Conn, decisionTopic string) error {
	if natsConn == nil {
		return fmt.Errorf("NATS connection is nil")
	}
	e.natsConn = natsConn

	// Create MCP client for order-executor
	e.mcpClient = mcp.NewClient(&mcp.Implementation{
		Name:    "decision-executor",
		Version: "1.0.0",
	}, nil)

	// Attempt initial MCP connection (non-fatal if it fails)
	if err := e.connectMCP(); err != nil {
		log.Warn().Err(err).
			Str("url", e.config.OrderExecutorURL).
			Msg("Could not connect to order-executor — will retry on first order")
	}

	// Subscribe to decision topic
	sub, err := natsConn.Subscribe(decisionTopic, e.handleDecision)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", decisionTopic, err)
	}
	e.subscription = sub

	log.Info().
		Str("topic", decisionTopic).
		Str("order_executor_url", e.config.OrderExecutorURL).
		Float64("min_confidence", e.config.MinConfidence).
		Float64("min_consensus", e.config.MinConsensus).
		Float64("default_quantity", e.config.DefaultQuantity).
		Msg("Decision executor started")

	return nil
}

// Stop unsubscribes from NATS and closes the MCP session.
func (e *Executor) Stop() {
	if e.subscription != nil {
		if err := e.subscription.Unsubscribe(); err != nil {
			log.Warn().Err(err).Msg("Failed to unsubscribe decision executor from NATS")
		}
		e.subscription = nil
	}

	e.sessionMu.Lock()
	session := e.session
	e.session = nil
	e.sessionMu.Unlock()

	if session != nil {
		if err := session.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close order-executor MCP session")
		}
	}

	log.Info().Msg("Decision executor stopped")
}

// handleDecision processes a NATS message containing a TradingDecision.
func (e *Executor) handleDecision(msg *nats.Msg) {
	var decision TradingDecision
	if err := json.Unmarshal(msg.Data, &decision); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal trading decision")
		return
	}

	log.Info().
		Str("symbol", decision.Symbol).
		Str("action", string(decision.Action)).
		Float64("confidence", decision.Confidence).
		Float64("consensus", decision.Consensus).
		Int("agents", decision.ParticipatingAgents).
		Msg("Received trading decision")

	if !e.ShouldExecute(&decision) {
		log.Info().
			Str("symbol", decision.Symbol).
			Str("action", string(decision.Action)).
			Float64("confidence", decision.Confidence).
			Float64("consensus", decision.Consensus).
			Msg("Decision does not meet execution thresholds — skipping")
		return
	}

	side := strings.ToLower(string(decision.Action))
	if err := e.placeOrder(decision.Symbol, side, e.config.DefaultQuantity, decision.SessionID); err != nil {
		log.Error().Err(err).
			Str("symbol", decision.Symbol).
			Str("side", side).
			Float64("quantity", e.config.DefaultQuantity).
			Msg("Failed to place order")
		return
	}

	log.Info().
		Str("symbol", decision.Symbol).
		Str("side", side).
		Float64("quantity", e.config.DefaultQuantity).
		Msg("Order placed successfully")
}

// connectMCP establishes or re-establishes the MCP session to the order-executor.
// Thread-safe with a connecting guard to prevent concurrent reconnect attempts.
func (e *Executor) connectMCP() error {
	if e.mcpClient == nil {
		return fmt.Errorf("MCP client not initialized")
	}

	e.sessionMu.Lock()
	if e.connecting {
		// Another goroutine is already connecting — wait on its channel
		ch := e.connectReady
		e.sessionMu.Unlock()
		select {
		case <-ch:
			return nil
		case <-time.After(10 * time.Second):
			return fmt.Errorf("timed out waiting for concurrent reconnect")
		}
	}
	e.connecting = true
	e.connectReady = make(chan struct{})
	old := e.session
	e.session = nil
	e.sessionMu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			log.Debug().Err(err).Msg("Error closing stale order-executor session")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	transport := &mcp.StreamableClientTransport{Endpoint: e.config.OrderExecutorURL}
	session, err := e.mcpClient.Connect(ctx, transport, nil)

	e.sessionMu.Lock()
	e.connecting = false
	if err == nil {
		e.session = session
	}
	close(e.connectReady)
	e.sessionMu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to connect to order-executor: %w", err)
	}

	log.Info().Str("url", e.config.OrderExecutorURL).Msg("Connected to order-executor MCP server")
	return nil
}

// placeOrder calls the place_market_order tool via MCP.
func (e *Executor) placeOrder(symbol, side string, quantity float64, sessionID string) error {
	// Ensure we have a session
	e.sessionMu.Lock()
	needsConnect := e.session == nil
	e.sessionMu.Unlock()

	if needsConnect {
		if err := e.connectMCP(); err != nil {
			return fmt.Errorf("order-executor unavailable: %w", err)
		}
	}

	e.sessionMu.Lock()
	session := e.session
	e.sessionMu.Unlock()

	if session == nil {
		return fmt.Errorf("order-executor session not available after connect")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"symbol":   symbol,
		"side":     side,
		"quantity": quantity,
	}
	if sessionID != "" {
		args["session_id"] = sessionID
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "place_market_order",
		Arguments: args,
	})
	if err != nil {
		// Transport error — clear session so next call reconnects
		e.sessionMu.Lock()
		e.session = nil
		e.sessionMu.Unlock()
		return fmt.Errorf("order execution failed: %w", err)
	}

	if result.IsError {
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				return fmt.Errorf("order-executor error: %s", textContent.Text)
			}
		}
		return fmt.Errorf("order-executor error: tool returned error result")
	}

	return nil
}
