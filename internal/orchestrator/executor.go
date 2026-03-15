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

	"github.com/ajitpratap0/cryptofunk/internal/market"
)

// ExecutorConfig holds configuration for the decision-to-order executor.
type ExecutorConfig struct {
	OrderExecutorURL string        // MCP endpoint for the order-executor server
	MinConfidence    float64       // Minimum confidence to place an order
	MinConsensus     float64       // Minimum consensus to place an order
	DefaultQuantity  float64       // Default order quantity (e.g. 0.001 BTC)
	PaperOnly        bool          // If true, refuse to start when TradingMode is LIVE
	TradingMode      string        // "PAPER" or "LIVE" — passed from config, not read from Viper
	ReconnectBackoff time.Duration // Skip reconnect for this duration after failure (default 60s)
	OrderCooldown    time.Duration // Min time between orders for same symbol (default 5m)
}

// Executor bridges orchestrator decisions to order execution via MCP.
// It subscribes to NATS decision topics and places orders through the
// order-executor MCP server.
type Executor struct {
	config          ExecutorConfig
	natsConn        *nats.Conn
	mcpClient       *mcp.Client
	session         *mcp.ClientSession
	sessionMu       sync.Mutex
	connecting      bool
	connectReady    chan struct{} // signals when a concurrent connect finishes
	subscription    *nats.Subscription
	ctx             context.Context    // lifecycle context — cancelled on Stop()
	cancel          context.CancelFunc // cancels ctx
	lastConnectFail time.Time          // backoff: skip reconnect after failure

	// Deduplication + rate limiting
	recentOrders sync.Map // symbol → time.Time of last order (per-symbol cooldown)
}

// NewExecutor creates a new decision-to-order executor.
func NewExecutor(config ExecutorConfig) *Executor {
	// connectReady starts nil — allocated fresh in connectMCP on each connect attempt.
	// Callers entering the connecting==true branch check for nil before waiting.
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
	// Safety check first: refuse to auto-trade in LIVE mode unless explicitly allowed
	if e.config.PaperOnly && strings.ToUpper(e.config.TradingMode) == "LIVE" {
		return fmt.Errorf("executor refused to start: PaperOnly=true but trading_mode=LIVE")
	}

	if natsConn == nil {
		return fmt.Errorf("NATS connection is nil")
	}
	e.natsConn = natsConn
	e.ctx, e.cancel = context.WithCancel(context.Background())

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
	e.sessionMu.Lock()
	e.subscription = sub
	e.sessionMu.Unlock()

	log.Info().
		Str("topic", decisionTopic).
		Str("order_executor_url", e.config.OrderExecutorURL).
		Float64("min_confidence", e.config.MinConfidence).
		Float64("min_consensus", e.config.MinConsensus).
		Float64("default_quantity", e.config.DefaultQuantity).
		Msg("Decision executor started")

	return nil
}

// Stop drains the NATS subscription, cancels the lifecycle context, and closes the MCP session.
func (e *Executor) Stop() {
	// Cancel lifecycle context to abort in-flight placeOrder/connectMCP calls
	if e.cancel != nil {
		e.cancel()
	}

	e.sessionMu.Lock()
	sub := e.subscription
	e.subscription = nil
	e.sessionMu.Unlock()

	// Drain allows in-flight handlers to finish before unsubscribing
	if sub != nil {
		if err := sub.Drain(); err != nil {
			log.Warn().Err(err).Msg("Failed to drain decision executor NATS subscription")
		}
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

	log.Debug().
		Str("symbol", decision.Symbol).
		Str("action", string(decision.Action)).
		Float64("confidence", decision.Confidence).
		Float64("consensus", decision.Consensus).
		Int("agents", decision.ParticipatingAgents).
		Msg("Received trading decision")

	if !e.ShouldExecute(&decision) {
		log.Debug().
			Str("symbol", decision.Symbol).
			Str("action", string(decision.Action)).
			Float64("confidence", decision.Confidence).
			Float64("consensus", decision.Consensus).
			Msg("Decision does not meet execution thresholds — skipping")
		return
	}

	// Convert symbol from CoinGecko ID (e.g. "bitcoin") to Binance pair ("BTCUSDT")
	symbol := market.CoinGeckoIDToBinanceSymbol(decision.Symbol)

	// Per-symbol cooldown: skip if we placed an order for this symbol recently.
	// Prevents duplicate orders from NATS redelivery and unbounded position accumulation.
	cooldown := e.config.OrderCooldown
	if cooldown == 0 {
		cooldown = 5 * time.Minute
	}
	if lastOrder, ok := e.recentOrders.Load(symbol); ok {
		if time.Since(lastOrder.(time.Time)) < cooldown {
			log.Debug().Str("symbol", symbol).Dur("cooldown", cooldown).
				Msg("Order cooldown active — skipping")
			return
		}
	}

	// Dispatch to goroutine so we don't block the NATS callback goroutine
	// (placeOrder has a 30s timeout for MCP calls).
	side := strings.ToLower(string(decision.Action))
	qty := e.config.DefaultQuantity

	// Record cooldown immediately (before goroutine) to prevent duplicate dispatch
	e.recentOrders.Store(symbol, time.Now())

	go func() {
		if err := e.placeOrder(symbol, side, qty); err != nil {
			log.Error().Err(err).
				Str("symbol", symbol).
				Str("side", side).
				Float64("quantity", qty).
				Msg("Failed to place order")
			// Set a 60s backoff cooldown instead of clearing immediately
			// to prevent a retry storm from rapid-fire NATS redeliveries.
			e.recentOrders.Store(symbol, time.Now().Add(60*time.Second-cooldown))
			return
		}
		log.Info().
			Str("symbol", symbol).
			Str("side", side).
			Float64("quantity", qty).
			Msg("Order placed successfully")
	}()
}

// connectMCP establishes or re-establishes the MCP session to the order-executor.
// Thread-safe with a connecting guard to prevent concurrent reconnect attempts.
func (e *Executor) connectMCP() error {
	if e.mcpClient == nil {
		return fmt.Errorf("MCP client not initialized")
	}

	// Backoff: skip reconnect after a recent failure
	backoff := e.config.ReconnectBackoff
	if backoff == 0 {
		backoff = 60 * time.Second
	}
	e.sessionMu.Lock()
	if !e.lastConnectFail.IsZero() && time.Since(e.lastConnectFail) < backoff {
		e.sessionMu.Unlock()
		return fmt.Errorf("reconnect backoff: last failure was %s ago", time.Since(e.lastConnectFail).Truncate(time.Second))
	}

	if e.connecting {
		// Another goroutine is already connecting — wait on its channel
		ch := e.connectReady
		e.sessionMu.Unlock()
		if ch == nil {
			return fmt.Errorf("concurrent connect in progress but no ready channel")
		}
		select {
		case <-ch:
			// Check if the other goroutine's connect actually succeeded
			e.sessionMu.Lock()
			ok := e.session != nil
			e.sessionMu.Unlock()
			if !ok {
				return fmt.Errorf("concurrent connect attempt failed")
			}
			return nil
		case <-e.ctx.Done():
			return fmt.Errorf("executor shutting down")
		case <-time.After(10 * time.Second):
			return fmt.Errorf("timed out waiting for concurrent reconnect")
		}
	}
	e.connecting = true
	e.connectReady = make(chan struct{})
	old := e.session
	e.session = nil
	e.sessionMu.Unlock()

	// Ensure connecting flag is cleared and waiters are unblocked even on panic
	defer func() {
		e.sessionMu.Lock()
		e.connecting = false
		close(e.connectReady)
		e.sessionMu.Unlock()
	}()

	if old != nil {
		if err := old.Close(); err != nil {
			log.Debug().Err(err).Msg("Error closing stale order-executor session")
		}
	}

	connCtx, connCancel := context.WithTimeout(e.ctx, 10*time.Second)
	defer connCancel()

	transport := &mcp.StreamableClientTransport{Endpoint: e.config.OrderExecutorURL}
	session, err := e.mcpClient.Connect(connCtx, transport, nil)

	e.sessionMu.Lock()
	if err == nil {
		e.session = session
		e.lastConnectFail = time.Time{} // clear backoff on success
	} else {
		e.lastConnectFail = time.Now()
	}
	e.sessionMu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to connect to order-executor: %w", err)
	}

	log.Info().Str("url", e.config.OrderExecutorURL).Msg("Connected to order-executor MCP server")
	return nil
}

// placeOrder calls the place_market_order tool via MCP.
// Note: the executor delegates persistence entirely to the order-executor MCP server,
// which creates its own order/trade records. Unlike the API path (which pre-creates a
// tracking order, calls the executor, then marks FILLED), this path has no separate
// tracking record in the API's DB. This is intentional — the MCP server is the
// single source of truth for executor-placed orders.
func (e *Executor) placeOrder(symbol, side string, quantity float64) error {
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

	callCtx, callCancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer callCancel()

	result, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "place_market_order",
		Arguments: map[string]interface{}{
			"symbol":   symbol,
			"side":     side,
			"quantity": quantity,
		},
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
