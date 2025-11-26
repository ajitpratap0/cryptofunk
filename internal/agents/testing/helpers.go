package testing

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
)

// AgentTestHelper provides utilities for testing agents
type AgentTestHelper struct {
	t       *testing.T
	logger  zerolog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	cleanup []func()
}

// NewAgentTestHelper creates a new test helper
func NewAgentTestHelper(t *testing.T) *AgentTestHelper {
	t.Helper()

	// Create test logger (disabled by default, can be enabled with TEST_LOG=1)
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(zerolog.Disabled)

	if os.Getenv("TEST_LOG") == "1" {
		logger = logger.Level(zerolog.DebugLevel)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	return &AgentTestHelper{
		t:       t,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		cleanup: make([]func(), 0),
	}
}

// Cleanup runs all registered cleanup functions
func (h *AgentTestHelper) Cleanup() {
	h.t.Helper()

	// Cancel context
	if h.cancel != nil {
		h.cancel()
	}

	// Run cleanup functions in reverse order
	for i := len(h.cleanup) - 1; i >= 0; i-- {
		h.cleanup[i]()
	}
}

// AddCleanup registers a cleanup function
func (h *AgentTestHelper) AddCleanup(fn func()) {
	h.cleanup = append(h.cleanup, fn)
}

// Context returns the test context
func (h *AgentTestHelper) Context() context.Context {
	return h.ctx
}

// Logger returns the test logger
func (h *AgentTestHelper) Logger() zerolog.Logger {
	return h.logger
}

// CreateTestAgent creates a basic test agent with no MCP servers
func (h *AgentTestHelper) CreateTestAgent(name, agentType string, metricsPort int) *agents.BaseAgent {
	h.t.Helper()

	config := TestAgentConfig(name, agentType)
	agent := agents.NewBaseAgent(config, h.logger, metricsPort)

	require.NotNil(h.t, agent)

	// Register cleanup
	h.AddCleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = agent.Shutdown(shutdownCtx)
	})

	return agent
}

// CreateMockMarketDataServer creates a mock market data server with common tools
func (h *AgentTestHelper) CreateMockMarketDataServer() *MockMCPServer {
	h.t.Helper()

	server := NewMockMCPServer("market-data", "1.0.0")

	tools := CommonTools{}
	fixtures := MarketDataFixtures{}

	// Register get_price tool
	server.RegisterTool(tools.GetPriceTool(), func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return fixtures.SamplePrice(), nil
	})

	// Register get_ohlcv tool
	server.RegisterTool(tools.GetOHLCVTool(), func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return fixtures.SampleOHLCV(), nil
	})

	return server
}

// CreateMockTechnicalIndicatorsServer creates a mock technical indicators server
func (h *AgentTestHelper) CreateMockTechnicalIndicatorsServer() *MockMCPServer {
	h.t.Helper()

	server := NewMockMCPServer("technical-indicators", "1.0.0")

	tools := CommonTools{}
	fixtures := TechnicalIndicatorFixtures{}

	// Register calculate_rsi tool
	server.RegisterTool(tools.CalculateRSITool(), func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return fixtures.SampleRSI(), nil
	})

	return server
}

// CreateMockOrderExecutorServer creates a mock order executor server
func (h *AgentTestHelper) CreateMockOrderExecutorServer() *MockMCPServer {
	h.t.Helper()

	server := NewMockMCPServer("order-executor", "1.0.0")

	tools := CommonTools{}
	fixtures := OrderExecutionFixtures{}

	// Register place_market_order tool
	server.RegisterTool(tools.PlaceOrderTool(), func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return fixtures.SampleMarketOrder(), nil
	})

	return server
}

// SimulateAgentStep simulates a single agent step without actually running the agent
func (h *AgentTestHelper) SimulateAgentStep(agent *agents.BaseAgent) error {
	h.t.Helper()

	return agent.Step(h.ctx)
}

// WaitForCondition waits for a condition to be true or timeout
func (h *AgentTestHelper) WaitForCondition(condition func() bool, timeout time.Duration, message string) bool {
	h.t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return true
		}

		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				h.t.Logf("Timeout waiting for condition: %s", message)
				return false
			}
		case <-h.ctx.Done():
			h.t.Log("Context cancelled while waiting for condition")
			return false
		}
	}
}

// AssertMCPCallMade verifies that a tool was called on a mock server
func (h *AgentTestHelper) AssertMCPCallMade(server *MockMCPServer, toolName string, expectedCount int) {
	h.t.Helper()

	actualCount := server.GetCallCount(toolName)
	require.Equal(h.t, expectedCount, actualCount,
		"Expected %s to be called %d times, but was called %d times",
		toolName, expectedCount, actualCount)
}

// AssertMCPCallArguments verifies the arguments of the last call to a tool
func (h *AgentTestHelper) AssertMCPCallArguments(server *MockMCPServer, toolName string, expectedArgs map[string]interface{}) {
	h.t.Helper()

	call := server.GetLastCall(toolName)
	require.NotNil(h.t, call, "No calls found for tool %s", toolName)

	for key, expectedValue := range expectedArgs {
		actualValue, ok := call.Arguments[key]
		require.True(h.t, ok, "Argument %s not found in call", key)
		require.Equal(h.t, expectedValue, actualValue,
			"Argument %s mismatch: expected %v, got %v", key, expectedValue, actualValue)
	}
}

// GetTestMetricsPort returns a unique metrics port for testing
func GetTestMetricsPort(t *testing.T) int {
	t.Helper()

	// Use a high port range for testing (9900-9999)
	// In real scenarios, might want to use a port allocation library
	// For now, use a simple hash of the test name
	basePort := 9900
	offset := len(t.Name()) % 100
	return basePort + offset
}
