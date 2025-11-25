package testing

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
)

// TestAgentTestHelper_BasicUsage demonstrates basic usage of AgentTestHelper
func TestAgentTestHelper_BasicUsage(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Create a test agent
	agent := helper.CreateTestAgent("test-agent", "technical", GetTestMetricsPort(t))
	require.NotNil(t, agent)

	// Verify agent properties
	assert.Equal(t, "test-agent", agent.GetName())
	assert.Equal(t, "technical", agent.GetType())
}

// TestMockMCPServer_ToolRegistration demonstrates tool registration and listing
func TestMockMCPServer_ToolRegistration(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Create mock server
	server := NewMockMCPServer("market-data", "1.0.0")

	// Register a tool
	tools := CommonTools{}
	fixtures := MarketDataFixtures{}

	server.RegisterTool(tools.GetPriceTool(), func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return fixtures.SamplePrice(), nil
	})

	// List tools
	result, err := server.ListTools(helper.Context(), &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, result.Tools, 1)
	assert.Equal(t, "get_price", result.Tools[0].Name)
}

// TestMockMCPServer_ToolCall demonstrates calling tools and verifying results
func TestMockMCPServer_ToolCall(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Create mock server with market data tools
	server := helper.CreateMockMarketDataServer()

	// Call get_price tool
	result, err := server.CallTool(helper.Context(), &mcp.CallToolParams{
		Name: "get_price",
		Arguments: map[string]interface{}{
			"symbol": "BTC/USDT",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	// Type assert to TextContent to access Text field
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "Expected TextContent")

	// Verify result contains expected price data
	assert.Contains(t, textContent.Text, "BTC/USDT")
	assert.Contains(t, textContent.Text, "42800")
}

// TestMockMCPServer_CallHistory demonstrates call recording and verification
func TestMockMCPServer_CallHistory(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Create mock server
	server := helper.CreateMockMarketDataServer()

	// Make multiple calls
	_, err := server.CallTool(helper.Context(), &mcp.CallToolParams{
		Name: "get_price",
		Arguments: map[string]interface{}{
			"symbol": "BTC/USDT",
		},
	})
	require.NoError(t, err)

	_, err = server.CallTool(helper.Context(), &mcp.CallToolParams{
		Name: "get_ohlcv",
		Arguments: map[string]interface{}{
			"symbol":    "BTC/USDT",
			"timeframe": "1h",
			"limit":     100,
		},
	})
	require.NoError(t, err)

	// Verify call counts
	helper.AssertMCPCallMade(server, "get_price", 1)
	helper.AssertMCPCallMade(server, "get_ohlcv", 1)

	// Verify last call arguments
	helper.AssertMCPCallArguments(server, "get_ohlcv", map[string]interface{}{
		"symbol":    "BTC/USDT",
		"timeframe": "1h",
		"limit":     100,
	})
}

// TestMockMCPServer_ErrorHandling demonstrates error scenarios
func TestMockMCPServer_ErrorHandling(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	server := NewMockMCPServer("test-server", "1.0.0")

	// Call non-existent tool
	_, err := server.CallTool(helper.Context(), &mcp.CallToolParams{
		Name:      "non_existent_tool",
		Arguments: map[string]interface{}{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool non_existent_tool not found")
}

// TestFixtures_MarketData demonstrates using market data fixtures
func TestFixtures_MarketData(t *testing.T) {
	fixtures := MarketDataFixtures{}

	// Test price data
	price := fixtures.SamplePrice()
	assert.Equal(t, "BTC/USDT", price["symbol"])
	assert.Equal(t, 42800.0, price["price"])

	// Test OHLCV data
	ohlcv := fixtures.SampleOHLCV()
	assert.Equal(t, "BTC/USDT", ohlcv["symbol"])
	assert.Equal(t, "1h", ohlcv["timeframe"])

	data, ok := ohlcv["data"].([][]interface{})
	require.True(t, ok)
	require.Len(t, data, 3)

	// Verify OHLCV structure [timestamp, open, high, low, close, volume]
	firstCandle := data[0]
	require.Len(t, firstCandle, 6)
	assert.Equal(t, 42000.0, firstCandle[1]) // open
	assert.Equal(t, 42500.0, firstCandle[2]) // high
	assert.Equal(t, 41800.0, firstCandle[3]) // low
	assert.Equal(t, 42200.0, firstCandle[4]) // close
	assert.Equal(t, 1250.5, firstCandle[5])  // volume

	// Test order book data
	orderBook := fixtures.SampleOrderBook()
	assert.Equal(t, "BTC/USDT", orderBook["symbol"])
	assert.NotNil(t, orderBook["bids"])
	assert.NotNil(t, orderBook["asks"])
}

// TestFixtures_TechnicalIndicators demonstrates using technical indicator fixtures
func TestFixtures_TechnicalIndicators(t *testing.T) {
	fixtures := TechnicalIndicatorFixtures{}

	// Test RSI data
	rsi := fixtures.SampleRSI()
	assert.Equal(t, "rsi", rsi["indicator"])
	assert.Equal(t, 14, rsi["period"])
	assert.Equal(t, "neutral", rsi["signal"])

	values, ok := rsi["values"].([]float64)
	require.True(t, ok)
	require.Len(t, values, 4)

	// Test MACD data
	macd := fixtures.SampleMACD()
	assert.Equal(t, "macd", macd["indicator"])
	assert.Equal(t, "bullish", macd["trend"])

	// Test Bollinger Bands data
	bollinger := fixtures.SampleBollinger()
	assert.Equal(t, "bollinger", bollinger["indicator"])
	assert.NotNil(t, bollinger["upper"])
	assert.NotNil(t, bollinger["middle"])
	assert.NotNil(t, bollinger["lower"])
}

// TestFixtures_News demonstrates using news/sentiment fixtures
func TestFixtures_News(t *testing.T) {
	fixtures := NewsFixtures{}

	// Test news data
	news := fixtures.SampleNews()
	assert.Equal(t, "positive", news["overall_sentiment"])
	assert.Equal(t, 0.68, news["confidence"])

	articles, ok := news["articles"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, articles, 2)

	// Verify article structure
	firstArticle := articles[0]
	assert.Equal(t, "Bitcoin Surges Past $43K", firstArticle["title"])
	assert.Equal(t, "positive", firstArticle["sentiment"])
	assert.Equal(t, 0.75, firstArticle["score"])

	// Test Fear & Greed Index
	fgi := fixtures.SampleFearGreedIndex()
	assert.Equal(t, 65, fgi["value"])
	assert.Equal(t, "greed", fgi["classification"])
}

// TestFixtures_Risk demonstrates using risk management fixtures
func TestFixtures_Risk(t *testing.T) {
	fixtures := RiskFixtures{}

	// Test risk limits
	limits := fixtures.SampleRiskLimits()
	assert.Equal(t, 10000.0, limits["max_position_size"])
	assert.Equal(t, 3.0, limits["max_leverage"])
	assert.Equal(t, 0.15, limits["max_drawdown"])

	// Test portfolio data
	portfolio := fixtures.SamplePortfolio()
	assert.Equal(t, 50000.0, portfolio["total_value"])
	assert.Equal(t, 20000.0, portfolio["cash"])

	positions, ok := portfolio["positions"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, positions, 2)

	// Verify position structure
	btcPosition := positions[0]
	assert.Equal(t, "BTC/USDT", btcPosition["symbol"])
	assert.Equal(t, 0.5, btcPosition["size"])
	assert.Equal(t, 1200.0, btcPosition["pnl"])
}

// TestFixtures_OrderExecution demonstrates using order execution fixtures
func TestFixtures_OrderExecution(t *testing.T) {
	fixtures := OrderExecutionFixtures{}

	// Test market order
	marketOrder := fixtures.SampleMarketOrder()
	assert.Equal(t, "buy", marketOrder["side"])
	assert.Equal(t, "market", marketOrder["type"])
	assert.Equal(t, "filled", marketOrder["status"])
	assert.Equal(t, 0.1, marketOrder["quantity"])
	assert.Equal(t, 42800.0, marketOrder["price"])

	// Test limit order
	limitOrder := fixtures.SampleLimitOrder()
	assert.Equal(t, "sell", limitOrder["side"])
	assert.Equal(t, "limit", limitOrder["type"])
	assert.Equal(t, "open", limitOrder["status"])
	assert.Equal(t, 43000.0, limitOrder["price"])
}

// TestHelpers_WaitForCondition demonstrates condition waiting utility
func TestHelpers_WaitForCondition(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	counter := 0

	// Condition that becomes true after 3 checks
	condition := func() bool {
		counter++
		return counter >= 3
	}

	// Should succeed quickly
	success := helper.WaitForCondition(condition, 1000*time.Millisecond, "counter reaches 3")
	assert.True(t, success)
	assert.GreaterOrEqual(t, counter, 3)
}

// TestHelpers_WaitForCondition_Timeout demonstrates timeout behavior
func TestHelpers_WaitForCondition_Timeout(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Condition that never becomes true
	condition := func() bool {
		return false
	}

	// Should timeout
	success := helper.WaitForCondition(condition, 50, "impossible condition")
	assert.False(t, success)
}

// TestIntegration_TechnicalAnalysisAgent demonstrates testing a technical analysis agent
func TestIntegration_TechnicalAnalysisAgent(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Create agent config with mock servers
	config := TestAgentConfig("technical-agent", "technical")

	// Add mock MCP server configurations
	// Note: In real tests, these would be actual mock server processes
	// For this example, we demonstrate the pattern
	AddMockMCPServer(config, "market-data", "mock-server", []string{"market-data"})
	AddMockMCPServer(config, "technical-indicators", "mock-server", []string{"technical-indicators"})

	// Create agent
	agent := agents.NewBaseAgent(config, helper.Logger(), GetTestMetricsPort(t))
	require.NotNil(t, agent)

	helper.AddCleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5)
		defer cancel()
		_ = agent.Shutdown(ctx)
	})

	// Verify agent configuration
	assert.Equal(t, "technical-agent", agent.GetName())
	assert.Equal(t, "technical", agent.GetType())
}

// TestIntegration_MultipleAgents demonstrates coordinating multiple agents
func TestIntegration_MultipleAgents(t *testing.T) {
	helper := NewAgentTestHelper(t)
	defer helper.Cleanup()

	// Create multiple agents
	technicalAgent := helper.CreateTestAgent("technical", "technical", 9900)
	sentimentAgent := helper.CreateTestAgent("sentiment", "sentiment", 9901)
	riskAgent := helper.CreateTestAgent("risk", "risk", 9902)

	require.NotNil(t, technicalAgent)
	require.NotNil(t, sentimentAgent)
	require.NotNil(t, riskAgent)

	// Verify each agent has unique identity
	assert.Equal(t, "technical", technicalAgent.GetName())
	assert.Equal(t, "sentiment", sentimentAgent.GetName())
	assert.Equal(t, "risk", riskAgent.GetName())
}

// TestHelpers_GetTestMetricsPort demonstrates metrics port allocation
func TestHelpers_GetTestMetricsPort(t *testing.T) {
	// Different test names should get different ports
	port1 := GetTestMetricsPort(t)
	assert.GreaterOrEqual(t, port1, 9900)
	assert.LessOrEqual(t, port1, 9999)
}
