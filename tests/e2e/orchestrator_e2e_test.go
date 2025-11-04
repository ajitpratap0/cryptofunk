package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TradingDecision represents the output from orchestrator
type TradingDecision struct {
	Symbol              string             `json:"symbol"`
	Action              string             `json:"action"`
	Confidence          float64            `json:"confidence"`
	Consensus           float64            `json:"consensus"`
	Reasoning           string             `json:"reasoning"`
	VotingResults       map[string]float64 `json:"voting_results"`
	ParticipatingAgents int                `json:"participating_agents"`
	Timestamp           time.Time          `json:"timestamp"`
}

// AgentSignal represents a signal from an agent
type AgentSignal struct {
	AgentName  string    `json:"agent_name"`
	AgentType  string    `json:"agent_type"`
	Symbol     string    `json:"symbol"`
	Signal     string    `json:"signal"`
	Confidence float64   `json:"confidence"`
	Reasoning  string    `json:"reasoning"`
	Timestamp  time.Time `json:"timestamp"`
}

// TestE2E_OrchestratorWithAllAgents tests full system with orchestrator and all agents
func TestE2E_OrchestratorWithAllAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Start embedded NATS server for testing
	natsServer := startEmbeddedNATS(t)
	defer natsServer.Shutdown()

	natsURL := natsServer.ClientURL()

	// Build paths
	projectRoot := getProjectRoot(t)
	binDir := filepath.Join(projectRoot, "bin")

	// Ensure bin directory exists
	err := os.MkdirAll(binDir, 0755)
	require.NoError(t, err, "Failed to create bin directory")

	// Build orchestrator and all agents
	t.Log("Building orchestrator and agents...")
	buildOrchestrator(t, projectRoot, binDir)
	buildAgents(t, projectRoot, binDir)

	// Create context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start orchestrator
	t.Log("Starting orchestrator...")
	orchestratorCmd := startOrchestrator(t, ctx, binDir, natsURL, projectRoot)
	defer killProcess(orchestratorCmd)

	// Give orchestrator time to start
	time.Sleep(2 * time.Second)

	// Start all agents
	t.Log("Starting agents...")
	agentCmds := startAllAgents(t, ctx, binDir, natsURL, projectRoot)
	defer func() {
		for _, cmd := range agentCmds {
			killProcess(cmd)
		}
	}()

	// Give agents time to start and register
	time.Sleep(3 * time.Second)

	// Connect to NATS to subscribe to decisions
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer nc.Close()

	// Subscribe to decision topic
	decisionChan := make(chan *TradingDecision, 10)
	_, err = nc.Subscribe("cryptofunk.decisions", func(msg *nats.Msg) {
		var decision TradingDecision
		if err := json.Unmarshal(msg.Data, &decision); err == nil {
			t.Logf("Received decision: %s %s (confidence: %.2f, consensus: %.2f)",
				decision.Action, decision.Symbol, decision.Confidence, decision.Consensus)
			decisionChan <- &decision
		}
	})
	require.NoError(t, err)

	// Test Case 1: Simulate market event that should trigger a BUY decision
	t.Run("market_event_triggers_buy_decision", func(t *testing.T) {
		// Publish a market data event that agents should react to
		marketEvent := map[string]interface{}{
			"symbol":    "BTC/USDT",
			"price":     50000.0,
			"volume":    1000.0,
			"timestamp": time.Now().Unix(),
			"type":      "price_update",
		}
		marketData, err := json.Marshal(marketEvent)
		require.NoError(t, err)

		// Publish to market data topic (agents should be subscribed)
		err = nc.Publish("cryptofunk.market.updates", marketData)
		require.NoError(t, err)
		t.Log("Published market event for BTC/USDT")

		// Wait for orchestrator to make a decision based on agent signals
		// The agents should analyze the market event and send signals to orchestrator
		select {
		case decision := <-decisionChan:
			t.Logf("Decision received: %+v", decision)
			assert.Equal(t, "BTC/USDT", decision.Symbol)
			assert.NotEmpty(t, decision.Action)
			assert.GreaterOrEqual(t, decision.ParticipatingAgents, 1)
			assert.NotEmpty(t, decision.Reasoning)

			// Verify decision structure
			assert.NotZero(t, decision.Confidence)
			assert.NotZero(t, decision.Consensus)
			assert.NotEmpty(t, decision.VotingResults)

			t.Logf("✓ E2E test passed: Orchestrator made decision %s with %d agents",
				decision.Action, decision.ParticipatingAgents)

		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for decision - agents may not be responding to market events")
		}
	})

	// Test Case 2: Verify multiple decision cycles work
	t.Run("multiple_decision_cycles", func(t *testing.T) {
		decisionsReceived := 0
		timeout := time.After(20 * time.Second)
		targetDecisions := 2

		// Send multiple market events
		for i := 0; i < 3; i++ {
			marketEvent := map[string]interface{}{
				"symbol":    "ETH/USDT",
				"price":     3000.0 + float64(i*100),
				"volume":    500.0,
				"timestamp": time.Now().Unix(),
				"type":      "price_update",
			}
			marketData, err := json.Marshal(marketEvent)
			require.NoError(t, err)
			err = nc.Publish("cryptofunk.market.updates", marketData)
			require.NoError(t, err)
			time.Sleep(2 * time.Second)
		}

		// Collect decisions
		for decisionsReceived < targetDecisions {
			select {
			case decision := <-decisionChan:
				if decision.Symbol == "ETH/USDT" {
					decisionsReceived++
					t.Logf("Received decision %d/%d: %s %s",
						decisionsReceived, targetDecisions, decision.Action, decision.Symbol)
				}
			case <-timeout:
				t.Logf("Received %d/%d decisions before timeout", decisionsReceived, targetDecisions)
				// Don't fail if we got at least one decision
				if decisionsReceived == 0 {
					t.Fatal("No decisions received for ETH/USDT")
				}
				return
			}
		}

		assert.GreaterOrEqual(t, decisionsReceived, 1, "Should receive at least one decision")
		t.Logf("✓ Multiple decision cycles working: received %d decisions", decisionsReceived)
	})
}

// Helper functions

func getProjectRoot(t *testing.T) string {
	t.Helper()
	// Navigate up from tests/e2e to project root
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Go up two levels from tests/e2e
	projectRoot := filepath.Join(cwd, "..", "..")
	absRoot, err := filepath.Abs(projectRoot)
	require.NoError(t, err)

	t.Logf("Project root: %s", absRoot)
	return absRoot
}

func buildOrchestrator(t *testing.T, projectRoot, binDir string) {
	t.Helper()
	cmd := exec.Command("go", "build",
		"-o", filepath.Join(binDir, "orchestrator"),
		"./cmd/orchestrator")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output: %s", string(output))
	}
	require.NoError(t, err, "Failed to build orchestrator")
}

func buildAgents(t *testing.T, projectRoot, binDir string) {
	t.Helper()
	agents := []string{
		"technical-agent",
		"orderbook-agent",
		"sentiment-agent",
		"trend-agent",
		"reversion-agent",
		"arbitrage-agent",
	}

	for _, agent := range agents {
		cmd := exec.Command("go", "build",
			"-o", filepath.Join(binDir, agent),
			fmt.Sprintf("./cmd/agents/%s", agent))
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build output for %s: %s", agent, string(output))
		}
		require.NoError(t, err, "Failed to build %s", agent)
	}
}

func startOrchestrator(t *testing.T, ctx context.Context, binDir, natsURL, projectRoot string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, filepath.Join(binDir, "orchestrator"))
	cmd.Dir = projectRoot // Set working directory to project root
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CRYPTOFUNK_ORCHESTRATOR_NATS_URL=%s", natsURL),
		"CRYPTOFUNK_ORCHESTRATOR_SIGNAL_TOPIC=cryptofunk.signals",
		"CRYPTOFUNK_ORCHESTRATOR_DECISION_TOPIC=cryptofunk.decisions",
		"CRYPTOFUNK_ORCHESTRATOR_HEARTBEAT_TOPIC=cryptofunk.heartbeat",
	)

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	require.NoError(t, err, "Failed to start orchestrator")

	return cmd
}

func startAllAgents(t *testing.T, ctx context.Context, binDir, natsURL, projectRoot string) []*exec.Cmd {
	t.Helper()
	agents := []string{
		"technical-agent",
		"orderbook-agent",
		"sentiment-agent",
		"trend-agent",
		"reversion-agent",
		"arbitrage-agent",
	}

	var cmds []*exec.Cmd
	for _, agent := range agents {
		cmd := exec.CommandContext(ctx, filepath.Join(binDir, agent))
		cmd.Dir = projectRoot // Set working directory to project root
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("NATS_URL=%s", natsURL),
			"SIGNAL_TOPIC=cryptofunk.signals",
			"HEARTBEAT_TOPIC=cryptofunk.heartbeat",
		)

		// Capture output for debugging
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		require.NoError(t, err, "Failed to start %s", agent)

		cmds = append(cmds, cmd)
		t.Logf("Started %s (PID: %d)", agent, cmd.Process.Pid)
	}

	return cmds
}

func killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // Clean up zombie process
	}
}
