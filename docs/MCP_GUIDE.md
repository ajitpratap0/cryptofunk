# MCP Development Guide

**Version:** 1.0.0
**Last Updated:** 2025-01-15

Complete guide for building custom MCP servers and agents in CryptoFunk.

## Table of Contents

- [Introduction to MCP](#introduction-to-mcp)
- [Architecture Overview](#architecture-overview)
- [Building Custom MCP Servers](#building-custom-mcp-servers)
- [Creating New Agents](#creating-new-agents)
- [MCP Tool Registration](#mcp-tool-registration)
- [Resource Patterns](#resource-patterns)
- [Testing MCP Servers](#testing-mcp-servers)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

---

## Introduction to MCP

### What is MCP?

**Model Context Protocol (MCP)** is an open protocol for standardized communication between AI agents and tools/resources. It enables:

- **Tool Calling**: Agents invoke tools (functions) with structured inputs
- **Resource Sharing**: Servers expose data sources (market data, databases, APIs)
- **Context Management**: Maintain conversation context across tool calls
- **Standardization**: Universal protocol across different LLM providers

### Why MCP in CryptoFunk?

CryptoFunk uses MCP to:
1. **Decouple agents from tools**: Agents don't need to know implementation details
2. **Enable tool reuse**: Multiple agents can use the same tools
3. **Simplify testing**: Mock MCP servers for unit tests
4. **Support hot-swapping**: Replace agents/tools without system restart
5. **Standardize communication**: JSON-RPC 2.0 over stdio

### MCP in CryptoFunk Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Orchestrator                          │
│          (MCP Client Coordinator)                        │
└───────────────────┬─────────────────────────────────────┘
                    │
       ┌────────────┼────────────┐
       │            │            │
       ▼            ▼            ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│  External   │ │  Internal   │ │   Trading   │
│ MCP Servers │ │ MCP Servers │ │   Agents    │
│             │ │             │ │             │
│ • CoinGecko │ │ • Market    │ │ • Technical │
│             │ │ • Indicators│ │ • Trend     │
│             │ │ • Risk      │ │ • Risk      │
│             │ │ • Executor  │ │ (+ 3 more)  │
└─────────────┘ └─────────────┘ └─────────────┘
                                       │
                                       ▼
                               Decision Making
                               (via MCP tools)
```

**Key Components:**
- **External MCP Servers**: Third-party services (CoinGecko, exchange APIs)
- **Internal MCP Servers**: Custom tools (technical indicators, risk analysis, order execution)
- **Trading Agents**: Decision-making entities that use MCP tools
- **Orchestrator**: Coordinates agent communication and voting

---

## Architecture Overview

### Communication Protocol

All MCP communication uses **JSON-RPC 2.0 over stdio**:

```
Agent (MCP Client)
    ↓ (JSON-RPC request via stdin)
MCP Server
    ↓ (Processes request, calls internal logic)
Response via stdout
    ↓ (JSON-RPC response)
Agent receives result
```

**Critical Rule**: **stdout is ONLY for MCP protocol messages**. All logs, debug output, and errors MUST go to stderr.

### MCP Request/Response Format

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_price",
    "arguments": {
      "symbol": "BTC"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "symbol": "BTC",
    "price": 42000.00,
    "timestamp": "2025-01-15T10:00:00Z"
  }
}
```

**Error Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Failed to fetch price: API rate limit exceeded"
  }
}
```

---

## Building Custom MCP Servers

### Step 1: Project Structure

```
cmd/mcp-servers/my-custom-server/
└── main.go               # Server entry point

internal/my-service/
├── service.go            # Business logic
└── service_test.go       # Unit tests
```

### Step 2: Server Template

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func main() {
    // CRITICAL: Setup logging to stderr (stdout is reserved for MCP protocol)
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    log.Info().Msg("My Custom MCP Server starting...")

    // Initialize dependencies
    ctx := context.Background()
    // database, err := db.New(ctx) // If you need database

    // Create server
    server := &MCPServer{
        // service: yourService,
    }

    // Start server
    if err := server.Run(); err != nil {
        log.Fatal().Err(err).Msg("Server failed")
    }
}

// MCPServer handles MCP protocol over stdio
type MCPServer struct {
    // Add your service dependencies here
    // service *myservice.Service
}

// Run starts the MCP server and processes requests
func (s *MCPServer) Run() error {
    log.Info().Msg("MCP server ready, listening on stdio")

    // Create JSON decoder/encoder for stdin/stdout
    decoder := json.NewDecoder(os.Stdin)
    encoder := json.NewEncoder(os.Stdout)

    // Main request loop
    for {
        var request MCPRequest
        if err := decoder.Decode(&request); err != nil {
            if err.Error() == "EOF" {
                log.Info().Msg("Client disconnected")
                return nil
            }
            log.Error().Err(err).Msg("Failed to decode request")
            continue
        }

        log.Debug().
            Str("method", request.Method).
            Str("tool", request.Params.Name).
            Msg("Received request")

        // Handle request
        response := s.handleRequest(&request)

        // Send response (ONLY to stdout)
        if err := encoder.Encode(response); err != nil {
            log.Error().Err(err).Msg("Failed to encode response")
            return err
        }
    }
}

// MCPRequest represents an MCP tool call request
type MCPRequest struct {
    JSONRPC string `json:"jsonrpc"`
    ID      int    `json:"id"`
    Method  string `json:"method"`
    Params  struct {
        Name      string                 `json:"name"`
        Arguments map[string]interface{} `json:"arguments"`
    } `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      int         `json:"id"`
    Result  interface{} `json:"result,omitempty"`
    Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

// handleRequest routes the request to the appropriate handler
func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
    resp := &MCPResponse{
        JSONRPC: "2.0",
        ID:      req.ID,
    }

    switch req.Method {
    case "tools/list":
        resp.Result = s.listTools()
    case "tools/call":
        result, err := s.callTool(req.Params.Name, req.Params.Arguments)
        if err != nil {
            resp.Error = &MCPError{
                Code:    -32603, // Internal error
                Message: err.Error(),
            }
        } else {
            resp.Result = result
        }
    default:
        resp.Error = &MCPError{
            Code:    -32601, // Method not found
            Message: fmt.Sprintf("Method not found: %s", req.Method),
        }
    }

    return resp
}

// listTools returns the list of available tools
func (s *MCPServer) listTools() interface{} {
    return map[string]interface{}{
        "tools": []map[string]interface{}{
            {
                "name":        "my_tool",
                "description": "Description of what this tool does",
                "inputSchema": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "param1": map[string]interface{}{
                            "type":        "string",
                            "description": "First parameter",
                        },
                        "param2": map[string]interface{}{
                            "type":        "number",
                            "description": "Second parameter",
                        },
                    },
                    "required": []string{"param1"},
                },
            },
        },
    }
}

// callTool executes the requested tool
func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
    switch name {
    case "my_tool":
        return s.handleMyTool(args)
    default:
        return nil, fmt.Errorf("tool not found: %s", name)
    }
}

// handleMyTool implements the "my_tool" tool
func (s *MCPServer) handleMyTool(args map[string]interface{}) (interface{}, error) {
    // Extract parameters
    param1, ok := args["param1"].(string)
    if !ok {
        return nil, fmt.Errorf("param1 must be a string")
    }

    // Optional parameter with default
    param2 := 100.0
    if val, ok := args["param2"].(float64); ok {
        param2 = val
    }

    log.Info().
        Str("param1", param1).
        Float64("param2", param2).
        Msg("Executing my_tool")

    // Implement your tool logic here
    result := map[string]interface{}{
        "success": true,
        "data":    param1,
        "value":   param2,
    }

    return result, nil
}
```

### Step 3: Parameter Extraction

MCP arguments come as `map[string]interface{}`. Handle type conversions carefully:

```go
func extractParams(args map[string]interface{}) (string, float64, error) {
    // String parameter (required)
    symbol, ok := args["symbol"].(string)
    if !ok {
        return "", 0, fmt.Errorf("symbol must be a string")
    }

    // Number parameter - handle multiple types
    var quantity float64
    switch v := args["quantity"].(type) {
    case float64:
        quantity = v
    case int:
        quantity = float64(v)
    case string:
        parsed, err := strconv.ParseFloat(v, 64)
        if err != nil {
            return "", 0, fmt.Errorf("quantity must be a number")
        }
        quantity = parsed
    default:
        return "", 0, fmt.Errorf("quantity must be a number")
    }

    return symbol, quantity, nil
}
```

### Step 4: Error Handling

Use standard JSON-RPC 2.0 error codes:

```go
const (
    ParseError     = -32700 // Invalid JSON
    InvalidRequest = -32600 // Invalid JSON-RPC
    MethodNotFound = -32601 // Method doesn't exist
    InvalidParams  = -32602 // Invalid method parameters
    InternalError  = -32603 // Internal server error
)

func errorResponse(id int, code int, message string) *MCPResponse {
    return &MCPResponse{
        JSONRPC: "2.0",
        ID:      id,
        Error: &MCPError{
            Code:    code,
            Message: message,
        },
    }
}
```

### Step 5: Register Server

Add to Taskfile.yml:
```yaml
build-server-my-custom:
  desc: "Build my custom server"
  sources:
    - cmd/mcp-servers/my-custom-server/**/*.go
    - internal/**/*.go
  generates:
    - "{{.BINARY_DIR}}/my-custom-server"
  cmds:
    - mkdir -p {{.BINARY_DIR}}
    - go build {{.GO_FLAGS}} -o {{.BINARY_DIR}}/my-custom-server cmd/mcp-servers/my-custom-server/main.go
```

Add to Docker:
```dockerfile
# deployments/docker/Dockerfile.my-custom-server
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /my-custom-server cmd/mcp-servers/my-custom-server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /my-custom-server /my-custom-server
USER 1000
ENTRYPOINT ["/my-custom-server"]
```

---

## Creating New Agents

### Step 1: Agent Structure

```
cmd/agents/my-agent/
└── main.go               # Agent entry point

internal/agents/
├── base_agent.go         # Base agent implementation (already exists)
└── base_agent_test.go
```

### Step 2: Agent Template

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/nats-io/nats.go"

    "github.com/ajitpratap0/cryptofunk/internal/agents"
    "github.com/ajitpratap0/cryptofunk/internal/llm"
)

type MyAgent struct {
    *agents.BaseAgent

    // NATS connection for signal publishing
    natsConn  *nats.Conn
    natsTopic string

    // LLM client for AI-powered analysis
    llmClient     llm.LLMClient
    promptBuilder *llm.PromptBuilder

    // Agent-specific configuration
    symbols []string
    // ... other config
}

func main() {
    // Setup logging
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    log.Info().Msg("My Agent starting...")

    // Create context with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Initialize NATS connection
    natsConn, err := nats.Connect(os.Getenv("NATS_URL"))
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to connect to NATS")
    }
    defer natsConn.Close()

    // Initialize LLM client (optional)
    llmClient, err := llm.NewClient(ctx, llm.ClientConfig{
        Gateway:       "bifrost",
        Endpoint:      os.Getenv("LLM_ENDPOINT"),
        APIKey:        os.Getenv("ANTHROPIC_API_KEY"),
        PrimaryModel:  "claude-sonnet-4",
        FallbackModel: "gpt-4-turbo",
        Temperature:   0.7,
        MaxTokens:     2000,
    })
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create LLM client")
    }

    // Create base agent
    baseAgent := agents.NewBaseAgent(ctx, agents.BaseAgentConfig{
        Name:    "my-agent",
        Version: "1.0.0",
        Type:    "analysis", // or "strategy"
    })

    // Create specialized agent
    agent := &MyAgent{
        BaseAgent: baseAgent,
        natsConn:  natsConn,
        natsTopic: "agent.signals.my-agent",
        llmClient: llmClient,
        promptBuilder: llm.NewPromptBuilder("my-agent"),
        symbols:   []string{"BTC/USDT", "ETH/USDT"},
    }

    // Start agent
    go agent.Run(ctx)

    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Info().Msg("Shutting down agent...")
}

// Run is the main agent loop
func (a *MyAgent) Run(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Main agent logic
            a.analyze(ctx)
        }
    }
}

// analyze performs the agent's main analysis
func (a *MyAgent) analyze(ctx context.Context) {
    for _, symbol := range a.symbols {
        // 1. Fetch market data (via MCP tool)
        data, err := a.fetchMarketData(ctx, symbol)
        if err != nil {
            log.Error().Err(err).Str("symbol", symbol).Msg("Failed to fetch data")
            continue
        }

        // 2. Calculate indicators (via MCP tool)
        indicators, err := a.calculateIndicators(ctx, symbol, data)
        if err != nil {
            log.Error().Err(err).Str("symbol", symbol).Msg("Failed to calculate indicators")
            continue
        }

        // 3. Generate signal (with LLM reasoning)
        signal, err := a.generateSignal(ctx, symbol, data, indicators)
        if err != nil {
            log.Error().Err(err).Str("symbol", symbol).Msg("Failed to generate signal")
            continue
        }

        // 4. Publish signal to NATS
        if err := a.publishSignal(ctx, signal); err != nil {
            log.Error().Err(err).Msg("Failed to publish signal")
        }
    }
}

// fetchMarketData calls the market-data MCP server
func (a *MyAgent) fetchMarketData(ctx context.Context, symbol string) (interface{}, error) {
    // Use MCP client to call tool
    // Implementation depends on your MCP client wrapper
    return nil, nil
}

// calculateIndicators calls the technical-indicators MCP server
func (a *MyAgent) calculateIndicators(ctx context.Context, symbol string, data interface{}) (interface{}, error) {
    // Use MCP client to call tools
    return nil, nil
}

// generateSignal uses LLM to analyze indicators and generate trading signal
func (a *MyAgent) generateSignal(ctx context.Context, symbol string, data, indicators interface{}) (*agents.TradingSignal, error) {
    // Build LLM prompt
    prompt := a.promptBuilder.BuildAnalysisPrompt(symbol, data, indicators)

    // Call LLM
    response, err := a.llmClient.Complete(ctx, prompt)
    if err != nil {
        return nil, err
    }

    // Parse response into signal
    signal := &agents.TradingSignal{
        AgentName:  a.Name,
        Symbol:     symbol,
        Action:     response.Decision,     // "BUY", "SELL", "HOLD"
        Confidence: response.Confidence,   // 0.0 to 1.0
        Reasoning:  response.Reasoning,    // LLM's explanation
        Timestamp:  time.Now(),
    }

    return signal, nil
}

// publishSignal publishes signal to NATS
func (a *MyAgent) publishSignal(ctx context.Context, signal *agents.TradingSignal) error {
    data, err := json.Marshal(signal)
    if err != nil {
        return err
    }

    return a.natsConn.Publish(a.natsTopic, data)
}
```

### Step 3: Agent Configuration

Add to `configs/agents.yaml`:
```yaml
agents:
  my-agent:
    enabled: true
    type: analysis
    symbols:
      - BTC/USDT
      - ETH/USDT
    llm:
      enabled: true
      model: claude-sonnet-4
      temperature: 0.7
    nats:
      topic: agent.signals.my-agent
```

### Step 4: Register Agent

Add to orchestrator's agent registry:
```go
// internal/orchestrator/registry.go
func (r *AgentRegistry) RegisterAgents() {
    r.Register("technical-agent", &AgentConfig{...})
    r.Register("trend-agent", &AgentConfig{...})
    r.Register("my-agent", &AgentConfig{...}) // Your agent
}
```

---

## MCP Tool Registration

### Tool Schema

All tools must define an **inputSchema** using JSON Schema:

```go
{
    "name": "calculate_rsi",
    "description": "Calculate Relative Strength Index for a symbol",
    "inputSchema": {
        "type": "object",
        "properties": {
            "symbol": {
                "type": "string",
                "description": "Trading pair symbol (e.g., 'BTC/USDT')"
            },
            "period": {
                "type": "integer",
                "description": "RSI period (typically 14)",
                "default": 14
            },
            "data": {
                "type": "array",
                "description": "Array of closing prices",
                "items": {
                    "type": "number"
                }
            }
        },
        "required": ["symbol", "data"]
    }
}
```

### Common JSON Schema Types

```go
// String
map[string]interface{}{
    "type": "string",
    "description": "Description",
    "enum": []string{"option1", "option2"}, // Optional
}

// Number (integer or float)
map[string]interface{}{
    "type": "number",
    "description": "Description",
    "minimum": 0,
    "maximum": 100,
}

// Integer
map[string]interface{}{
    "type": "integer",
    "description": "Description",
}

// Boolean
map[string]interface{}{
    "type": "boolean",
    "description": "Description",
}

// Array
map[string]interface{}{
    "type": "array",
    "description": "Description",
    "items": map[string]interface{}{
        "type": "number",
    },
}

// Object
map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "field1": {...},
        "field2": {...},
    },
    "required": []string{"field1"},
}
```

---

## Resource Patterns

### Resources vs Tools

**Tools**: Functions that perform actions (place orders, calculate indicators)
**Resources**: Data sources that can be read (market data, configuration, historical trades)

### Implementing Resources (Future Feature)

```go
// listResources returns available resources
func (s *MCPServer) listResources() interface{} {
    return map[string]interface{}{
        "resources": []map[string]interface{}{
            {
                "uri":         "market://BTC-USDT/price",
                "name":        "BTC/USDT Current Price",
                "description": "Real-time BTC/USDT price",
                "mimeType":    "application/json",
            },
            {
                "uri":         "market://BTC-USDT/history",
                "name":        "BTC/USDT Historical Data",
                "description": "Historical OHLCV data",
                "mimeType":    "application/json",
            },
        },
    }
}

// readResource fetches resource data
func (s *MCPServer) readResource(uri string) (interface{}, error) {
    // Parse URI and fetch resource
    // Example: market://BTC-USDT/price
    return nil, nil
}
```

---

## Testing MCP Servers

### Unit Testing

```go
package main

import (
    "encoding/json"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestMCPServer_ListTools(t *testing.T) {
    server := &MCPServer{}

    result := server.listTools()
    tools, ok := result.(map[string]interface{})["tools"]
    assert.True(t, ok, "listTools should return tools array")

    toolArray := tools.([]map[string]interface{})
    assert.Greater(t, len(toolArray), 0, "Should have at least one tool")
}

func TestMCPServer_CallTool(t *testing.T) {
    server := &MCPServer{}

    args := map[string]interface{}{
        "symbol":   "BTC/USDT",
        "quantity": 0.1,
    }

    result, err := server.callTool("my_tool", args)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Testing

```go
func TestMCPServer_IntegrationFlow(t *testing.T) {
    // Start server in goroutine with pipes
    stdinR, stdinW := io.Pipe()
    stdoutR, stdoutW := io.Pipe()

    server := &MCPServer{}
    go func() {
        // Override stdin/stdout
        oldStdin, oldStdout := os.Stdin, os.Stdout
        os.Stdin, os.Stdout = stdinR, stdoutW
        defer func() {
            os.Stdin, os.Stdout = oldStdin, oldStdout
        }()

        server.Run()
    }()

    // Send request
    request := MCPRequest{
        JSONRPC: "2.0",
        ID:      1,
        Method:  "tools/call",
        Params: struct {
            Name      string                 `json:"name"`
            Arguments map[string]interface{} `json:"arguments"`
        }{
            Name: "my_tool",
            Arguments: map[string]interface{}{
                "symbol": "BTC/USDT",
            },
        },
    }

    encoder := json.NewEncoder(stdinW)
    encoder.Encode(request)

    // Read response
    decoder := json.NewDecoder(stdoutR)
    var response MCPResponse
    err := decoder.Decode(&response)
    assert.NoError(t, err)
    assert.NotNil(t, response.Result)
}
```

### Manual Testing with echo

```bash
# Test initialize
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | ./bin/my-custom-server

# Test tool call
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"my_tool","arguments":{"symbol":"BTC/USDT"}},"id":2}' | ./bin/my-custom-server

# Capture stderr logs separately
./bin/my-custom-server 2> server.log | jq
```

---

## Best Practices

### 1. Logging Rules

```go
// GOOD: Logging to stderr
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
log.Info().Msg("Server started")

// BAD: Never do this in MCP servers
fmt.Println("Server started") // Goes to stdout - breaks protocol
```

### 2. Error Handling

```go
// GOOD: Return structured errors
if err != nil {
    return nil, fmt.Errorf("failed to fetch price: %w", err)
}

// GOOD: Use appropriate error codes
resp.Error = &MCPError{
    Code:    -32603, // Internal error
    Message: "Database connection failed",
}

// BAD: Panic instead of error return
if err != nil {
    panic(err) // Don't panic in production
}
```

### 3. Parameter Validation

```go
// GOOD: Validate all parameters
func (s *MCPServer) validateParams(args map[string]interface{}) error {
    symbol, ok := args["symbol"].(string)
    if !ok || symbol == "" {
        return fmt.Errorf("symbol is required and must be a string")
    }

    quantity, ok := args["quantity"].(float64)
    if !ok || quantity <= 0 {
        return fmt.Errorf("quantity must be a positive number")
    }

    return nil
}
```

### 4. Context Management

```go
// GOOD: Use context for cancellation
func (s *MCPServer) fetchData(ctx context.Context, symbol string) (interface{}, error) {
    // Check if context is cancelled
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Fetch data with timeout
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // ... fetch logic
}
```

### 5. Tool Documentation

```go
// GOOD: Comprehensive tool description
{
    "name": "place_market_order",
    "description": "Place a market order for immediate execution at current market price. Market orders execute quickly but may have higher slippage. Use for: (1) urgent entries/exits, (2) highly liquid markets, (3) when execution speed is priority over price.",
    "inputSchema": {
        // ... schema
    }
}

// BAD: Vague description
{
    "name": "place_order",
    "description": "Places an order",
    // ...
}
```

### 6. Thread Safety

```go
// GOOD: Protect shared state with mutex
type MCPServer struct {
    cache map[string]interface{}
    mutex sync.RWMutex
}

func (s *MCPServer) getCached(key string) (interface{}, bool) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    val, ok := s.cache[key]
    return val, ok
}
```

---

## Troubleshooting

### Issue 1: MCP Server Not Responding

**Symptoms**: Agent hangs, no response from server

**Debugging**:
```bash
# Check server is running
ps aux | grep my-custom-server

# Check stderr logs
tail -f /path/to/server.log

# Test manually
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | ./bin/my-custom-server | jq
```

**Common Causes**:
- Server crashed during startup (check logs)
- stdout contamination (remove all `fmt.Printf()`, `println()`, etc.)
- JSON parsing error (validate request format)

### Issue 2: Tool Returns Error

**Debugging**:
```bash
# Enable debug logging
export LOG_LEVEL=debug

# Run server and check stderr
./bin/my-custom-server 2>&1 | grep ERROR
```

**Common Causes**:
- Invalid parameter types (check type assertions)
- Missing required parameters
- Database connection failure
- API rate limit exceeded

### Issue 3: Slow Tool Response

**Debugging**:
```go
// Add timing logs
start := time.Now()
result, err := s.service.DoSomething()
log.Info().
    Dur("duration", time.Since(start)).
    Msg("Tool execution completed")
```

**Common Causes**:
- Database query without indexes
- External API timeout
- Large data transfers
- Missing caching

### Issue 4: Memory Leak

**Debugging**:
```bash
# Monitor memory usage
docker stats my-custom-server

# Go profiling
go tool pprof http://localhost:6060/debug/pprof/heap
```

**Common Causes**:
- Goroutine leak (not closing channels)
- Large cached data (implement cache eviction)
- Database connections not closed

---

## Examples

### Example 1: Simple Price Fetcher Server

```go
// Fetches cryptocurrency prices from an external API
type PriceFetcherServer struct {
    apiClient *http.Client
}

func (s *PriceFetcherServer) listTools() interface{} {
    return map[string]interface{}{
        "tools": []map[string]interface{}{
            {
                "name":        "get_price",
                "description": "Get current price for a cryptocurrency",
                "inputSchema": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "symbol": map[string]interface{}{
                            "type":        "string",
                            "description": "Symbol (e.g., 'BTC', 'ETH')",
                        },
                    },
                    "required": []string{"symbol"},
                },
            },
        },
    }
}

func (s *PriceFetcherServer) getPrice(symbol string) (interface{}, error) {
    // Fetch price from API
    resp, err := s.apiClient.Get(fmt.Sprintf("https://api.example.com/price/%s", symbol))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)

    return map[string]interface{}{
        "symbol": symbol,
        "price":  result["price"],
        "time":   time.Now().Unix(),
    }, nil
}
```

### Example 2: Simple Analysis Agent

```go
// Analyzes RSI and generates signals
type SimpleAnalysisAgent struct {
    *agents.BaseAgent
    natsConn *nats.Conn
}

func (a *SimpleAnalysisAgent) analyze(ctx context.Context, symbol string) {
    // 1. Fetch RSI (via MCP tool)
    rsi, err := a.fetchRSI(ctx, symbol)
    if err != nil {
        log.Error().Err(err).Msg("Failed to fetch RSI")
        return
    }

    // 2. Simple signal logic
    var action string
    var confidence float64

    if rsi < 30 {
        action = "BUY"
        confidence = 0.8
    } else if rsi > 70 {
        action = "SELL"
        confidence = 0.8
    } else {
        action = "HOLD"
        confidence = 0.5
    }

    // 3. Publish signal
    signal := &agents.TradingSignal{
        AgentName:  "simple-analysis",
        Symbol:     symbol,
        Action:     action,
        Confidence: confidence,
        Reasoning:  fmt.Sprintf("RSI is %.2f", rsi),
        Timestamp:  time.Now(),
    }

    a.publishSignal(ctx, signal)
}
```

---

## Additional Resources

- **MCP Specification**: https://github.com/modelcontextprotocol/specification
- **CryptoFunk Architecture**: [docs/ARCHITECTURE.md](ARCHITECTURE.md)
- **Existing MCP Servers**: `cmd/mcp-servers/`
- **Existing Agents**: `cmd/agents/`
- **API Documentation**: [docs/API.md](API.md)

---

**Last Updated:** 2025-01-15
**Maintained By:** CryptoFunk Team
**Version:** 1.0.0
