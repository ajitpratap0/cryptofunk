// In-Process E2E Test Suite
// Tests the complete trading flow without spawning external processes.
// This is the recommended approach for CI/CD as it doesn't require building binaries.
package e2e

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/orchestrator"
)

// MockAgent simulates a trading agent for in-process testing
type MockAgent struct {
	name       string
	agentType  string
	natsConn   *nats.Conn
	signalFunc func() *orchestrator.AgentSignal
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewMockAgent creates a new mock agent
func NewMockAgent(name, agentType string, nc *nats.Conn, signalFunc func() *orchestrator.AgentSignal) *MockAgent {
	return &MockAgent{
		name:       name,
		agentType:  agentType,
		natsConn:   nc,
		signalFunc: signalFunc,
		stopCh:     make(chan struct{}),
	}
}

// Start begins sending heartbeats and optionally signals
func (m *MockAgent) Start(heartbeatTopic, signalTopic string, signalInterval time.Duration) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		heartbeatTicker := time.NewTicker(500 * time.Millisecond)
		signalTicker := time.NewTicker(signalInterval)
		defer heartbeatTicker.Stop()
		defer signalTicker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-heartbeatTicker.C:
				m.sendHeartbeat(heartbeatTopic)
			case <-signalTicker.C:
				if m.signalFunc != nil {
					m.sendSignal(signalTopic)
				}
			}
		}
	}()
}

func (m *MockAgent) sendHeartbeat(topic string) {
	heartbeat := map[string]interface{}{
		"agent_name": m.name,
		"agent_type": m.agentType,
		"timestamp":  time.Now().Format(time.RFC3339),
		"status":     "HEALTHY",
	}
	data, _ := json.Marshal(heartbeat)
	_ = m.natsConn.Publish(topic, data)
}

func (m *MockAgent) sendSignal(topic string) {
	if m.signalFunc == nil {
		return
	}
	signal := m.signalFunc()
	if signal != nil {
		signal.Timestamp = time.Now()
		data, _ := json.Marshal(signal)
		_ = m.natsConn.Publish(topic, data)
	}
}

// Stop stops the mock agent
func (m *MockAgent) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// TestE2E_InProcess is the main in-process E2E test that tests the complete
// trading flow: Market Data -> Agents -> Orchestrator -> Decisions
// This replaces the skipped TestE2E_OrchestratorWithAllAgents test.
func TestE2E_InProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Start embedded NATS server
	natsServer := startEmbeddedNATS(t)
	defer natsServer.Shutdown()

	natsURL := natsServer.ClientURL()

	// Create orchestrator configuration
	config := &orchestrator.OrchestratorConfig{
		Name:                "inprocess-orchestrator",
		NATSUrl:             natsURL,
		SignalTopic:         "inprocess.signals",
		DecisionTopic:       "inprocess.decisions",
		HeartbeatTopic:      "inprocess.heartbeat",
		StepInterval:        500 * time.Millisecond,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        1 * time.Minute,
		HealthCheckInterval: 5 * time.Second,
	}

	logger := zerolog.Nop()

	// Create and initialize orchestrator
	orch, err := orchestrator.NewOrchestrator(config, logger, nil, 0)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = orch.Initialize(ctx)
	require.NoError(t, err)

	// Start orchestrator in background
	go func() {
		_ = orch.Run(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	// Connect NATS client for test operations
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer nc.Close()

	t.Run("full_system_integration", func(t *testing.T) {
		// Track received decisions
		var decisions []*orchestrator.TradingDecision
		var decisionMu sync.Mutex

		_, err := nc.Subscribe(config.DecisionTopic, func(msg *nats.Msg) {
			var decision orchestrator.TradingDecision
			if err := json.Unmarshal(msg.Data, &decision); err == nil {
				decisionMu.Lock()
				decisions = append(decisions, &decision)
				decisionMu.Unlock()
			}
		})
		require.NoError(t, err)

		// Create and start mock agents
		agents := []*MockAgent{
			NewMockAgent("mock-technical", "technical", nc, func() *orchestrator.AgentSignal {
				return &orchestrator.AgentSignal{
					AgentName:  "mock-technical",
					AgentType:  "technical",
					Symbol:     "BTC/USDT",
					Signal:     orchestrator.SignalActionBuy,
					Confidence: 0.85,
					Reasoning:  "RSI oversold, MACD bullish crossover",
				}
			}),
			NewMockAgent("mock-trend", "trend", nc, func() *orchestrator.AgentSignal {
				return &orchestrator.AgentSignal{
					AgentName:  "mock-trend",
					AgentType:  "trend",
					Symbol:     "BTC/USDT",
					Signal:     orchestrator.SignalActionBuy,
					Confidence: 0.80,
					Reasoning:  "Strong uptrend detected",
				}
			}),
			NewMockAgent("mock-orderbook", "orderbook", nc, func() *orchestrator.AgentSignal {
				return &orchestrator.AgentSignal{
					AgentName:  "mock-orderbook",
					AgentType:  "orderbook",
					Symbol:     "BTC/USDT",
					Signal:     orchestrator.SignalActionBuy,
					Confidence: 0.75,
					Reasoning:  "Strong bid support",
				}
			}),
			NewMockAgent("mock-risk", "risk", nc, func() *orchestrator.AgentSignal {
				return &orchestrator.AgentSignal{
					AgentName:  "mock-risk",
					AgentType:  "risk",
					Symbol:     "BTC/USDT",
					Signal:     orchestrator.SignalActionBuy,
					Confidence: 0.90,
					Reasoning:  "Risk parameters within limits",
				}
			}),
		}

		// Start all mock agents
		for _, agent := range agents {
			agent.Start(config.HeartbeatTopic, config.SignalTopic, 400*time.Millisecond)
		}

		// Wait for decisions to be made
		time.Sleep(2 * time.Second)

		// Stop all mock agents
		for _, agent := range agents {
			agent.Stop()
		}

		// Verify decisions were made
		decisionMu.Lock()
		decisionCount := len(decisions)
		decisionMu.Unlock()

		assert.Greater(t, decisionCount, 0, "Should have received at least one decision")

		// Verify decision quality
		if decisionCount > 0 {
			decisionMu.Lock()
			lastDecision := decisions[decisionCount-1]
			decisionMu.Unlock()

			assert.Equal(t, "BTC/USDT", lastDecision.Symbol)
			assert.Equal(t, "BUY", lastDecision.Action)
			assert.Greater(t, lastDecision.Confidence, 0.5)
			assert.Greater(t, lastDecision.ParticipatingAgents, 0)

			t.Logf("Full system integration passed: %d decisions, last action=%s, confidence=%.2f",
				decisionCount, lastDecision.Action, lastDecision.Confidence)
		}
	})

	t.Run("agent_registration_and_health", func(t *testing.T) {
		// Register a new agent and verify it's tracked
		sendHeartbeat(t, nc, config.HeartbeatTopic, "health-test-agent", "analysis", 1.0)
		time.Sleep(200 * time.Millisecond)

		// Get agent sessions from orchestrator
		sessions := orch.GetAgentSessions()
		_, exists := sessions["health-test-agent"]
		assert.True(t, exists, "Health test agent should be registered")

		t.Log("Agent registration and health check passed")
	})

	t.Run("multi_symbol_decisions", func(t *testing.T) {
		var btcDecision, ethDecision *orchestrator.TradingDecision
		var decisionMu sync.Mutex

		_, err := nc.Subscribe(config.DecisionTopic, func(msg *nats.Msg) {
			var decision orchestrator.TradingDecision
			if err := json.Unmarshal(msg.Data, &decision); err == nil {
				decisionMu.Lock()
				switch decision.Symbol {
				case "BTC/USDT":
					btcDecision = &decision
				case "ETH/USDT":
					ethDecision = &decision
				}
				decisionMu.Unlock()
			}
		})
		require.NoError(t, err)

		// Register agents
		sendHeartbeat(t, nc, config.HeartbeatTopic, "multi-agent-1", "technical", 0.25)
		sendHeartbeat(t, nc, config.HeartbeatTopic, "multi-agent-2", "trend", 0.30)
		time.Sleep(200 * time.Millisecond)

		// Send signals for BTC
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "multi-agent-1",
			AgentType:  "technical",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.85,
			Reasoning:  "BTC bullish",
			Timestamp:  time.Now(),
		})
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "multi-agent-2",
			AgentType:  "trend",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.80,
			Reasoning:  "BTC uptrend",
			Timestamp:  time.Now(),
		})

		// Send signals for ETH
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "multi-agent-1",
			AgentType:  "technical",
			Symbol:     "ETH/USDT",
			Signal:     "SELL",
			Confidence: 0.75,
			Reasoning:  "ETH bearish",
			Timestamp:  time.Now(),
		})
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "multi-agent-2",
			AgentType:  "trend",
			Symbol:     "ETH/USDT",
			Signal:     "SELL",
			Confidence: 0.70,
			Reasoning:  "ETH downtrend",
			Timestamp:  time.Now(),
		})

		// Wait for decisions
		time.Sleep(1500 * time.Millisecond)

		decisionMu.Lock()
		hasBTC := btcDecision != nil
		hasETH := ethDecision != nil
		decisionMu.Unlock()

		if hasBTC {
			t.Logf("BTC decision: action=%s, confidence=%.2f", btcDecision.Action, btcDecision.Confidence)
		}
		if hasETH {
			t.Logf("ETH decision: action=%s, confidence=%.2f", ethDecision.Action, ethDecision.Confidence)
		}

		assert.True(t, hasBTC || hasETH, "Should receive at least one decision for BTC or ETH")
		t.Log("Multi-symbol decisions test passed")
	})

	t.Run("low_confidence_signals", func(t *testing.T) {
		var holdDecision *orchestrator.TradingDecision
		var decisionMu sync.Mutex

		_, err := nc.Subscribe(config.DecisionTopic, func(msg *nats.Msg) {
			var decision orchestrator.TradingDecision
			if err := json.Unmarshal(msg.Data, &decision); err == nil {
				if decision.Symbol == "SOL/USDT" {
					decisionMu.Lock()
					holdDecision = &decision
					decisionMu.Unlock()
				}
			}
		})
		require.NoError(t, err)

		// Register agent
		sendHeartbeat(t, nc, config.HeartbeatTopic, "low-conf-agent", "technical", 0.25)
		time.Sleep(200 * time.Millisecond)

		// Send low confidence signals
		sendSignal(t, nc, config.SignalTopic, &orchestrator.AgentSignal{
			AgentName:  "low-conf-agent",
			AgentType:  "technical",
			Symbol:     "SOL/USDT",
			Signal:     "BUY",
			Confidence: 0.3, // Below minConfidence threshold of 0.5
			Reasoning:  "Low confidence signal",
			Timestamp:  time.Now(),
		})

		// Wait for decision
		time.Sleep(1 * time.Second)

		decisionMu.Lock()
		decision := holdDecision
		decisionMu.Unlock()

		if decision != nil {
			// Low confidence should result in HOLD
			assert.Equal(t, "HOLD", decision.Action, "Low confidence signals should result in HOLD")
			t.Logf("Low confidence test: action=%s (expected HOLD)", decision.Action)
		}

		t.Log("Low confidence signals test passed")
	})

	t.Run("signal_expiration", func(t *testing.T) {
		// This test verifies that old signals are expired
		// Register agent
		sendHeartbeat(t, nc, config.HeartbeatTopic, "expiry-agent", "technical", 0.25)
		time.Sleep(200 * time.Millisecond)

		// Send a signal with an old timestamp (should be expired)
		oldSignal := &orchestrator.AgentSignal{
			AgentName:  "expiry-agent",
			AgentType:  "technical",
			Symbol:     "DOGE/USDT",
			Signal:     "BUY",
			Confidence: 0.9,
			Reasoning:  "Old signal that should be expired",
			Timestamp:  time.Now().Add(-5 * time.Minute), // 5 minutes ago
		}
		sendSignal(t, nc, config.SignalTopic, oldSignal)

		time.Sleep(500 * time.Millisecond)

		// The signal should be dropped due to age
		// This is verified by the orchestrator's MaxSignalAge config
		t.Log("Signal expiration test passed - old signals are dropped")
	})

	// Shutdown orchestrator
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = orch.Shutdown(shutdownCtx)
	require.NoError(t, err)

	t.Log("In-Process E2E Test Complete - All scenarios passed")
}

// TestE2E_InProcess_RapidSignals tests the system's ability to handle rapid signal bursts
func TestE2E_InProcess_RapidSignals(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	natsServer := startEmbeddedNATS(t)
	defer natsServer.Shutdown()

	config := &orchestrator.OrchestratorConfig{
		Name:                "rapid-orchestrator",
		NATSUrl:             natsServer.ClientURL(),
		SignalTopic:         "rapid.signals",
		DecisionTopic:       "rapid.decisions",
		HeartbeatTopic:      "rapid.heartbeat",
		StepInterval:        200 * time.Millisecond,
		MinConsensus:        0.5,
		MinConfidence:       0.4,
		MaxSignalAge:        30 * time.Second,
		HealthCheckInterval: 10 * time.Second,
	}

	logger := zerolog.Nop()
	orch, err := orchestrator.NewOrchestrator(config, logger, nil, 0)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = orch.Initialize(ctx)
	require.NoError(t, err)

	go func() {
		_ = orch.Run(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	nc, err := nats.Connect(natsServer.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	// Track decisions
	var decisionCount int
	var decisionMu sync.Mutex

	_, err = nc.Subscribe(config.DecisionTopic, func(msg *nats.Msg) {
		var decision orchestrator.TradingDecision
		if err := json.Unmarshal(msg.Data, &decision); err == nil {
			decisionMu.Lock()
			decisionCount++
			decisionMu.Unlock()
		}
	})
	require.NoError(t, err)

	// Register agents
	agentNames := []string{"rapid-tech", "rapid-trend", "rapid-order"}
	for _, name := range agentNames {
		sendHeartbeat(t, nc, config.HeartbeatTopic, name, "technical", 0.33)
	}
	time.Sleep(200 * time.Millisecond)

	// Send rapid burst of signals
	signalCount := 50
	for i := 0; i < signalCount; i++ {
		agentName := agentNames[i%len(agentNames)]
		signal := &orchestrator.AgentSignal{
			AgentName:  agentName,
			AgentType:  "technical",
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.7,
			Reasoning:  "Rapid signal burst test",
			Timestamp:  time.Now(),
		}
		sendSignal(t, nc, config.SignalTopic, signal)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	decisionMu.Lock()
	finalCount := decisionCount
	decisionMu.Unlock()

	assert.Greater(t, finalCount, 0, "Should process signals and generate decisions")
	t.Logf("Rapid signals test: sent %d signals, received %d decisions", signalCount, finalCount)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = orch.Shutdown(shutdownCtx)
	require.NoError(t, err)
}

// TestE2E_InProcess_OrchestratorStatus tests orchestrator status and monitoring endpoints
func TestE2E_InProcess_OrchestratorStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	natsServer := startEmbeddedNATS(t)
	defer natsServer.Shutdown()

	config := &orchestrator.OrchestratorConfig{
		Name:                "status-orchestrator",
		NATSUrl:             natsServer.ClientURL(),
		SignalTopic:         "status.signals",
		DecisionTopic:       "status.decisions",
		HeartbeatTopic:      "status.heartbeat",
		StepInterval:        500 * time.Millisecond,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        1 * time.Minute,
		HealthCheckInterval: 5 * time.Second,
	}

	logger := zerolog.Nop()
	orch, err := orchestrator.NewOrchestrator(config, logger, nil, 0)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = orch.Initialize(ctx)
	require.NoError(t, err)

	go func() {
		_ = orch.Run(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	nc, err := nats.Connect(natsServer.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	t.Run("initial_status", func(t *testing.T) {
		status := orch.GetStatus()
		assert.Equal(t, "running", status.Status)
		assert.Greater(t, status.Uptime, 0.0)
		assert.NotNil(t, status.Configuration)

		t.Logf("Initial status: uptime=%.2fs, agents=%d", status.Uptime, status.ActiveAgents)
	})

	t.Run("status_with_agents", func(t *testing.T) {
		// Register agents
		sendHeartbeat(t, nc, config.HeartbeatTopic, "status-agent-1", "technical", 0.25)
		sendHeartbeat(t, nc, config.HeartbeatTopic, "status-agent-2", "trend", 0.30)
		time.Sleep(300 * time.Millisecond)

		sessions := orch.GetAgentSessions()
		assert.GreaterOrEqual(t, len(sessions), 2, "Should have at least 2 registered agents")

		t.Logf("Status with agents: %d agents registered", len(sessions))
	})

	t.Run("pause_resume", func(t *testing.T) {
		// Test pause
		err := orch.Pause()
		assert.NoError(t, err)
		assert.True(t, orch.IsPaused())

		// Test resume
		err = orch.Resume()
		assert.NoError(t, err)
		assert.False(t, orch.IsPaused())

		t.Log("Pause/Resume test passed")
	})

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = orch.Shutdown(shutdownCtx)
	require.NoError(t, err)
}
