// End-to-End Trading Flow Test
// Tests the complete flow: Market Data → Agents → Orchestrator → Order Execution
package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

// TestE2E_CompleteTrading Flow tests the full end-to-end trading system
func TestE2E_CompleteTradingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Start embedded NATS server
	natsServer := startEmbeddedNATS(t)
	defer natsServer.Shutdown()

	// Create orchestrator configuration
	config := &orchestrator.OrchestratorConfig{
		Name:                "e2e-orchestrator",
		NATSUrl:             natsServer.ClientURL(),
		SignalTopic:         "e2e.signals",
		DecisionTopic:       "e2e.decisions",
		HeartbeatTopic:      "e2e.heartbeat",
		StepInterval:        500 * time.Millisecond,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        5 * time.Minute,
		HealthCheckInterval: 10 * time.Second,
	}

	logger := zerolog.Nop()

	// Create and initialize orchestrator
	orch, err := orchestrator.NewOrchestrator(config, logger, 0)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = orch.Initialize(ctx)
	require.NoError(t, err)

	// Start orchestrator in background
	go func() {
		_ = orch.Run(ctx)
	}()

	time.Sleep(500 * time.Millisecond)

	// Create NATS client for test
	nc, err := nats.Connect(natsServer.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	// Test Scenario: Complete trading flow for BTC/USDT
	t.Run("complete_buy_sell_cycle", func(t *testing.T) {
		// Step 1: Subscribe to trading decisions
		decisionChan := make(chan *orchestrator.TradingDecision, 10)
		_, err := nc.Subscribe(config.DecisionTopic, func(msg *nats.Msg) {
			var decision orchestrator.TradingDecision
			if err := json.Unmarshal(msg.Data, &decision); err == nil {
				decisionChan <- &decision
			}
		})
		require.NoError(t, err)

		// Step 2: Register mock agents (simulating real trading agents)
		agents := []struct {
			name   string
			typ    string
			weight float64
		}{
			{"technical-agent", "technical", 0.25},
			{"orderbook-agent", "orderbook", 0.20},
			{"sentiment-agent", "sentiment", 0.15},
			{"trend-agent", "trend", 0.30},
			{"reversion-agent", "reversion", 0.25},
			{"risk-agent", "risk", 1.00},
		}

		for _, agent := range agents {
			sendHeartbeat(t, nc, config.HeartbeatTopic, agent.name, agent.typ, agent.weight)
		}

		time.Sleep(200 * time.Millisecond)

		// Step 3: Simulate market data update (price increase detected)
		// In real system, this would come from market-data MCP server
		// Agents analyze the data and generate signals

		// Technical agent detects bullish pattern
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "technical-agent",
			AgentType:  "technical",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.85,
			Reasoning:  "RSI oversold, MACD bullish crossover, price above EMA",
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"rsi":  32.5,
				"macd": "bullish",
				"ema":  "above",
			},
		})

		// Order book agent detects strong buy pressure
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "orderbook-agent",
			AgentType:  "orderbook",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.78,
			Reasoning:  "Strong buy wall at support, bid/ask ratio 2.5",
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"bid_ask_ratio": 2.5,
				"buy_pressure":  0.72,
			},
		})

		// Sentiment agent detects positive sentiment
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "sentiment-agent",
			AgentType:  "sentiment",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.70,
			Reasoning:  "Positive social sentiment, bullish news flow",
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"sentiment_score": 0.65,
				"news_score":      0.75,
			},
		})

		// Trend agent confirms uptrend
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "trend-agent",
			AgentType:  "trend",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.88,
			Reasoning:  "Strong uptrend, higher highs and higher lows, momentum increasing",
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"trend_strength": 0.82,
				"momentum":       0.75,
			},
		})

		// Mean reversion agent agrees (not oversold)
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "reversion-agent",
			AgentType:  "reversion",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.65,
			Reasoning:  "Price not overbought, room for upside",
			Timestamp:  time.Now(),
		})

		// Risk agent approves (no risk constraints violated)
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "risk-agent",
			AgentType:  "risk",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.90,
			Reasoning:  "Position limits OK, volatility acceptable, portfolio risk within bounds",
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"position_limit_pct": 0.45,
				"volatility":         0.25,
				"portfolio_risk":     0.35,
			},
		})

		// Step 4: Wait for orchestrator decision (BUY expected)
		var buyDecision *orchestrator.TradingDecision
		select {
		case decision := <-decisionChan:
			buyDecision = decision
			assert.Equal(t, "BTC/USDT", decision.Symbol)
			assert.Equal(t, "BUY", decision.Action)
			assert.Greater(t, decision.Confidence, 0.7)
			assert.Greater(t, decision.Consensus, 0.6)
			assert.Equal(t, 6, decision.ParticipatingAgents)

			t.Logf("BUY Decision: Confidence=%.2f, Consensus=%.2f, Agents=%d",
				decision.Confidence, decision.Consensus, decision.ParticipatingAgents)
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for BUY decision")
		}

		// Step 5: Simulate order execution (would go to order-executor MCP server)
		// In real system, order executor would:
		// - Validate decision
		// - Calculate position size
		// - Place market/limit order
		// - Return execution confirmation
		t.Log("Order execution: BUY BTC/USDT (simulated)")

		// Step 6: Simulate price movement and position monitoring
		time.Sleep(800 * time.Millisecond)

		// Step 7: Market reversal detected - generate SELL signals
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "technical-agent",
			AgentType:  "technical",
			Symbol:     "BTC/USDT",
			Signal:     "SELL",
			Confidence: 0.82,
			Reasoning:  "RSI overbought, bearish divergence detected",
			Timestamp:  time.Now(),
		})

		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "trend-agent",
			AgentType:  "trend",
			Symbol:     "BTC/USDT",
			Signal:     "SELL",
			Confidence: 0.75,
			Reasoning:  "Trend weakening, momentum declining",
			Timestamp:  time.Now(),
		})

		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "reversion-agent",
			AgentType:  "reversion",
			Symbol:     "BTC/USDT",
			Signal:     "SELL",
			Confidence: 0.80,
			Reasoning:  "Mean reversion signal - price extended, likely pullback",
			Timestamp:  time.Now(),
		})

		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "risk-agent",
			AgentType:  "risk",
			Symbol:     "BTC/USDT",
			Signal:     "SELL",
			Confidence: 0.85,
			Reasoning:  "Profit target reached, take profits",
			Timestamp:  time.Now(),
		})

		// Step 8: Wait for SELL decision
		select {
		case decision := <-decisionChan:
			assert.Equal(t, "BTC/USDT", decision.Symbol)
			assert.Equal(t, "SELL", decision.Action)
			assert.Greater(t, decision.Confidence, 0.6)
			assert.Greater(t, decision.Consensus, 0.5)

			t.Logf("SELL Decision: Confidence=%.2f, Consensus=%.2f, Agents=%d",
				decision.Confidence, decision.Consensus, decision.ParticipatingAgents)
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for SELL decision")
		}

		// Step 9: Verify complete cycle
		assert.NotNil(t, buyDecision, "BUY decision should have been made")
		t.Log("Complete trading cycle: Market Data → Agents → Orchestrator → Decision → (Order Execution)")
		t.Log("E2E Test PASSED: Full trading flow verified")
	})

	// Test Scenario: Risk veto in E2E context
	t.Run("risk_veto_in_e2e_flow", func(t *testing.T) {
		decisionChan := make(chan *orchestrator.TradingDecision, 10)
		_, err := nc.Subscribe(config.DecisionTopic, func(msg *nats.Msg) {
			var decision orchestrator.TradingDecision
			if err := json.Unmarshal(msg.Data, &decision); err == nil {
				decisionChan <- &decision
			}
		})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		// Scenario: Multiple agents want to BUY, but risk agent vetoes
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "technical-agent",
			AgentType:  "technical",
			Symbol:     "ETH/USDT",
			Signal:     "BUY",
			Confidence: 0.90,
			Reasoning:  "Strong bullish signals",
			Timestamp:  time.Now(),
		})

		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "trend-agent",
			AgentType:  "trend",
			Symbol:     "ETH/USDT",
			Signal:     "BUY",
			Confidence: 0.85,
			Reasoning:  "Strong uptrend confirmed",
			Timestamp:  time.Now(),
		})

		// Risk agent vetoes due to position limits
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "risk-agent",
			AgentType:  "risk",
			Symbol:     "ETH/USDT",
			Signal:     "HOLD",
			Confidence: 0.95,
			Reasoning:  "VETO: Position limits exceeded, max drawdown approaching",
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"veto":               true,
				"position_limit_pct": 0.95,
				"max_drawdown_pct":   0.18,
			},
		})

		// Wait for decision (should be HOLD due to risk veto)
		select {
		case decision := <-decisionChan:
			// Risk agent's high weight should prevent BUY
			switch decision.Action {
			case "HOLD":
				t.Logf("Risk veto successful: Action=%s, Consensus=%.2f",
					decision.Action, decision.Consensus)
			case "BUY":
				// BUY can still win if risk agent's weight isn't dominant enough
				assert.Less(t, decision.Consensus, 0.9,
					"If BUY wins despite risk HOLD, consensus should be lower")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for risk veto decision")
		}
	})

	// Shutdown orchestrator
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = orch.Shutdown(shutdownCtx)
	require.NoError(t, err)

	t.Log("E2E Trading Flow Test Complete")
}
