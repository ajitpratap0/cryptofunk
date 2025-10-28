# Agent Testing Framework

This package provides a comprehensive testing framework for CryptoFunk trading agents. It enables testing agents in isolation without requiring real MCP servers, external APIs, or message brokers.

## Overview

The testing framework consists of three main components:

1. **MockMCPServer** (`mock_mcp_server.go`) - Simulates MCP servers without spawning processes
2. **Test Fixtures** (`fixtures.go`) - Provides realistic sample data for all agent types
3. **Test Helpers** (`helpers.go`) - High-level utilities for test setup, teardown, and assertions

## Quick Start

```go
import (
    "testing"
    agenttesting "github.com/ajitpratap0/cryptofunk/internal/agents/testing"
)

func TestMyAgent(t *testing.T) {
    // Create test helper (handles cleanup automatically)
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    // Create a test agent
    agent := helper.CreateTestAgent("my-agent", "technical", 9900)

    // Create mock MCP server with tools
    server := helper.CreateMockMarketDataServer()

    // Call tool and verify behavior
    result, err := server.CallTool(helper.Context(), &mcp.CallToolParams{
        Name: "get_price",
        Arguments: map[string]interface{}{
            "symbol": "BTC/USDT",
        },
    })

    require.NoError(t, err)
    helper.AssertMCPCallMade(server, "get_price", 1)
}
```

## Components

### 1. MockMCPServer

The `MockMCPServer` simulates an MCP server for testing without spawning real processes.

#### Key Features

- **Thread-safe**: All operations are protected by mutex
- **Tool registration**: Register tools with custom handler functions
- **Call recording**: Automatically records all tool calls for verification
- **MCP protocol compliance**: Implements `CallTool` and `ListTools` interfaces

#### Usage

```go
// Create mock server
server := agenttesting.NewMockMCPServer("market-data", "1.0.0")

// Register a tool with handler
server.RegisterTool(
    tools.GetPriceTool(),
    func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
        return map[string]interface{}{
            "symbol": args["symbol"],
            "price":  42800.0,
        }, nil
    },
)

// Call tool
result, err := server.CallTool(ctx, &mcp.CallToolParams{
    Name: "get_price",
    Arguments: map[string]interface{}{"symbol": "BTC/USDT"},
})

// Verify calls
callCount := server.GetCallCount("get_price")
lastCall := server.GetLastCall("get_price")
allCalls := server.GetCalls()

// Reset call history
server.Reset()
```

### 2. Test Fixtures

Fixtures provide realistic sample data for testing. All fixtures return `map[string]interface{}` that can be easily customized.

#### Available Fixtures

**Market Data** (`MarketDataFixtures`):
```go
fixtures := agenttesting.MarketDataFixtures{}

// Get sample price data
price := fixtures.SamplePrice()
// Returns: {"symbol": "BTC/USDT", "price": 42800.0, "volume": 1520.8, "change": 1.42}

// Get sample OHLCV data (3 candles)
ohlcv := fixtures.SampleOHLCV()
// Returns: {"symbol": "BTC/USDT", "timeframe": "1h", "data": [[timestamp, o, h, l, c, v], ...]}

// Get sample order book
orderBook := fixtures.SampleOrderBook()
// Returns: {"symbol": "BTC/USDT", "bids": [[price, size], ...], "asks": [[price, size], ...]}
```

**Technical Indicators** (`TechnicalIndicatorFixtures`):
```go
fixtures := agenttesting.TechnicalIndicatorFixtures{}

// RSI data
rsi := fixtures.SampleRSI()
// Returns: {"indicator": "rsi", "period": 14, "values": [65.2, 68.5, 72.1, 69.8], "signal": "neutral"}

// MACD data
macd := fixtures.SampleMACD()
// Returns: {"indicator": "macd", "macd": [...], "signal": [...], "histogram": [...], "trend": "bullish"}

// Bollinger Bands
bollinger := fixtures.SampleBollinger()
// Returns: {"indicator": "bollinger", "upper": [...], "middle": [...], "lower": [...], "bandwidth": 4.7}
```

**News & Sentiment** (`NewsFixtures`):
```go
fixtures := agenttesting.NewsFixtures{}

// News articles with sentiment
news := fixtures.SampleNews()
// Returns: {"articles": [...], "overall_sentiment": "positive", "confidence": 0.68}

// Fear & Greed Index
fgi := fixtures.SampleFearGreedIndex()
// Returns: {"value": 65, "classification": "greed", "timestamp": 1234567890}
```

**Risk Management** (`RiskFixtures`):
```go
fixtures := agenttesting.RiskFixtures{}

// Risk limits
limits := fixtures.SampleRiskLimits()
// Returns: {"max_position_size": 10000.0, "max_leverage": 3.0, "max_drawdown": 0.15, ...}

// Portfolio data
portfolio := fixtures.SamplePortfolio()
// Returns: {"total_value": 50000.0, "cash": 20000.0, "positions": [...]}
```

**Order Execution** (`OrderExecutionFixtures`):
```go
fixtures := agenttesting.OrderExecutionFixtures{}

// Market order result
marketOrder := fixtures.SampleMarketOrder()
// Returns: {"order_id": "...", "symbol": "BTC/USDT", "side": "buy", "status": "filled", ...}

// Limit order result
limitOrder := fixtures.SampleLimitOrder()
// Returns: {"order_id": "...", "symbol": "BTC/USDT", "side": "sell", "type": "limit", ...}
```

#### Common MCP Tools

Pre-defined tool schemas for common operations:

```go
tools := agenttesting.CommonTools{}

// Market data tools
priceTool := tools.GetPriceTool()       // get_price tool definition
ohlcvTool := tools.GetOHLCVTool()       // get_ohlcv tool definition

// Technical indicator tools
rsiTool := tools.CalculateRSITool()     // calculate_rsi tool definition

// Order execution tools
orderTool := tools.PlaceOrderTool()     // place_market_order tool definition
```

### 3. Test Helpers

The `AgentTestHelper` provides high-level utilities for test setup, teardown, and assertions.

#### Creating Test Helper

```go
func TestMyFeature(t *testing.T) {
    // Create helper with automatic cleanup
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    // Helper provides:
    // - Context with 30s timeout: helper.Context()
    // - Structured logger: helper.Logger()
    // - Automatic cleanup registration
}
```

#### Logging

By default, logs are disabled during tests. Enable with environment variable:

```bash
TEST_LOG=1 go test ./...
```

#### Factory Methods

**Create Test Agent**:
```go
// Creates agent with automatic cleanup
agent := helper.CreateTestAgent("technical-agent", "technical", 9900)

// Agent is automatically shut down on test completion
```

**Create Mock Servers**:
```go
// Market data server with get_price and get_ohlcv tools
marketServer := helper.CreateMockMarketDataServer()

// Technical indicators server with calculate_rsi tool
indicatorServer := helper.CreateMockTechnicalIndicatorsServer()

// Order executor server with place_market_order tool
orderServer := helper.CreateMockOrderExecutorServer()
```

#### Assertion Utilities

**Verify Tool Calls**:
```go
// Assert tool was called specific number of times
helper.AssertMCPCallMade(server, "get_price", 3)

// Assert tool was called with specific arguments
helper.AssertMCPCallArguments(server, "get_ohlcv", map[string]interface{}{
    "symbol":    "BTC/USDT",
    "timeframe": "1h",
    "limit":     100,
})
```

**Wait for Conditions**:
```go
// Poll condition with timeout
success := helper.WaitForCondition(
    func() bool { return agent.IsRunning() },
    1000,  // milliseconds
    "agent should be running",
)
```

#### Cleanup Management

```go
// Register custom cleanup function
helper.AddCleanup(func() {
    // Custom cleanup code
})

// All cleanup functions run in reverse order on defer helper.Cleanup()
```

#### Metrics Port Allocation

```go
// Get unique metrics port for test
port := agenttesting.GetTestMetricsPort(t)
// Returns port in range 9900-9999 based on test name hash
```

## Usage Patterns

### Pattern 1: Testing Agent with Mock Tools

```go
func TestAgentUsesMarketData(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    // Create mock server
    server := helper.CreateMockMarketDataServer()

    // Simulate agent calling tools
    result, err := server.CallTool(helper.Context(), &mcp.CallToolParams{
        Name: "get_price",
        Arguments: map[string]interface{}{"symbol": "BTC/USDT"},
    })

    require.NoError(t, err)

    // Verify call was made
    helper.AssertMCPCallMade(server, "get_price", 1)
}
```

### Pattern 2: Testing with Custom Handlers

```go
func TestAgentHandlesErrors(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    server := agenttesting.NewMockMCPServer("test", "1.0.0")

    // Register tool with error handler
    tools := agenttesting.CommonTools{}
    server.RegisterTool(
        tools.GetPriceTool(),
        func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
            return nil, errors.New("API rate limit exceeded")
        },
    )

    // Test error handling
    _, err := server.CallTool(helper.Context(), &mcp.CallToolParams{
        Name: "get_price",
        Arguments: map[string]interface{}{"symbol": "BTC/USDT"},
    })

    require.Error(t, err)
    assert.Contains(t, err.Error(), "rate limit")
}
```

### Pattern 3: Testing Agent Configuration

```go
func TestAgentConfiguration(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    // Create custom config
    config := agenttesting.TestAgentConfig("custom-agent", "technical")
    config.Config["threshold"] = 0.7
    config.Config["timeframe"] = "1h"

    // Create agent with custom config
    agent := agents.NewBaseAgent(config, helper.Logger(), 9900)

    // Verify configuration
    assert.Equal(t, "custom-agent", agent.GetName())
    // Test agent behavior with custom config...
}
```

### Pattern 4: Testing Multiple Agents

```go
func TestMultiAgentCoordination(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    // Create multiple agents
    technical := helper.CreateTestAgent("technical", "technical", 9900)
    sentiment := helper.CreateTestAgent("sentiment", "sentiment", 9901)
    risk := helper.CreateTestAgent("risk", "risk", 9902)

    // Create shared mock server
    server := helper.CreateMockMarketDataServer()

    // Test coordination logic...
}
```

### Pattern 5: Testing with Realistic Data

```go
func TestAgentProcessesMarketData(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    // Use fixtures for realistic data
    fixtures := agenttesting.MarketDataFixtures{}
    ohlcv := fixtures.SampleOHLCV()

    server := agenttesting.NewMockMCPServer("market-data", "1.0.0")
    tools := agenttesting.CommonTools{}

    server.RegisterTool(
        tools.GetOHLCVTool(),
        func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
            return ohlcv, nil
        },
    )

    // Test agent processing...
}
```

## Best Practices

### 1. Always Use defer Cleanup

```go
func TestSomething(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()  // Critical: ensures cleanup even on test failure
    // ...
}
```

### 2. Use Factory Methods for Common Servers

```go
// Good: Use pre-configured factory methods
server := helper.CreateMockMarketDataServer()

// Avoid: Manually configuring every tool
server := agenttesting.NewMockMCPServer("market-data", "1.0.0")
server.RegisterTool(...) // Repetitive
server.RegisterTool(...) // Repetitive
```

### 3. Test One Scenario Per Test

```go
// Good: Focused test
func TestAgentHandlesRateLimit(t *testing.T) { ... }
func TestAgentHandlesTimeout(t *testing.T) { ... }

// Avoid: Testing multiple scenarios in one test
func TestAgentErrorHandling(t *testing.T) {
    // Tests rate limits, timeouts, invalid data...
}
```

### 4. Use Assertions for Verification

```go
// Good: Clear assertion with helper
helper.AssertMCPCallMade(server, "get_price", 1)

// Avoid: Manual verification
if server.GetCallCount("get_price") != 1 {
    t.Errorf("Expected 1 call, got %d", server.GetCallCount("get_price"))
}
```

### 5. Reset Mock State Between Subtests

```go
func TestMultipleScenarios(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()

    server := helper.CreateMockMarketDataServer()

    t.Run("scenario1", func(t *testing.T) {
        // ...
        server.Reset() // Clear call history
    })

    t.Run("scenario2", func(t *testing.T) {
        // ...
    })
}
```

## Integration with Existing Tests

This framework is designed to complement existing unit tests:

- **Unit tests**: Test individual functions in isolation (e.g., sentiment scoring logic)
- **Integration tests** (this framework): Test agent behavior with mocked dependencies
- **E2E tests** (future): Test full system with real infrastructure

## Examples

See `example_test.go` for comprehensive examples demonstrating:

- Basic agent testing
- Tool registration and calling
- Call history verification
- Error handling
- Using fixtures
- Multi-agent scenarios
- Condition waiting
- Custom handlers

## Troubleshooting

### "Tool not found" errors

Ensure the tool is registered before calling:

```go
server := agenttesting.NewMockMCPServer("test", "1.0.0")
tools := agenttesting.CommonTools{}
server.RegisterTool(tools.GetPriceTool(), handler)  // Must register first
```

### Context timeout errors

Default context timeout is 30 seconds. For longer tests, create custom context:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
// Use ctx instead of helper.Context()
```

### Cleanup not running

Always use `defer helper.Cleanup()`:

```go
func TestSomething(t *testing.T) {
    helper := agenttesting.NewAgentTestHelper(t)
    defer helper.Cleanup()  // Must be deferred immediately
    // ...
}
```

### Port conflicts in parallel tests

Use `GetTestMetricsPort(t)` to get unique port per test:

```go
port := agenttesting.GetTestMetricsPort(t)
agent := helper.CreateTestAgent("test", "technical", port)
```

## Future Enhancements

Planned improvements (see T074, T075 in TASKS.md):

- **Mock Orchestrator**: Simulate orchestrator for testing signal publishing
- **NATS Mocking**: Mock NATS message broker for event-driven testing
- **Viper Config Mocking**: Utilities for mocking global configuration
- **Performance Benchmarks**: Benchmark utilities for measuring agent performance
- **Test Recorder**: Record real MCP interactions for replay in tests

## Contributing

When adding new fixtures or helpers:

1. Add to appropriate file (`fixtures.go`, `helpers.go`, `mock_mcp_server.go`)
2. Include example usage in `example_test.go`
3. Document in this README
4. Ensure thread-safety for mock implementations
5. Follow existing naming conventions

## Related Documentation

- **TASKS.md** - See T073 (this framework), T074 (mock orchestrator), T075 (benchmarks)
- **internal/agents/base.go** - BaseAgent implementation
- **docs/MCP_INTEGRATION.md** - MCP protocol details
- **T072 Completion Report** - Background on testing requirements
