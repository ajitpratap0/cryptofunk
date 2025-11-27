# Technical Indicators MCP Server

A Model Context Protocol (MCP) server that provides technical analysis indicators for cryptocurrency trading. This server exposes 5 core indicators through a JSON-RPC 2.0 interface via stdio.

## Features

This MCP server implements the following technical indicators:

1. **RSI (Relative Strength Index)** - Momentum indicator showing overbought/oversold conditions
2. **MACD (Moving Average Convergence Divergence)** - Trend-following momentum indicator
3. **Bollinger Bands** - Volatility indicator with upper/middle/lower bands
4. **EMA (Exponential Moving Average)** - Weighted moving average favoring recent prices
5. **ADX (Average Directional Index)** - Trend strength indicator

All calculations are powered by the `github.com/cinar/indicator/v2` library, ensuring accuracy and reliability.

## Architecture

### MCP Protocol Compliance

- **Transport**: stdio (JSON-RPC 2.0)
- **Logging**: All logs go to stderr; stdout is reserved for MCP protocol
- **Methods Supported**:
  - `initialize` - Initialize server connection
  - `tools/list` - List available indicator tools
  - `tools/call` - Execute indicator calculations

### Design Pattern

```
┌─────────────────────────────────────┐
│   MCP Client (Orchestrator/Agent)   │
└──────────────┬──────────────────────┘
               │ JSON-RPC 2.0 (stdio)
               ▼
┌─────────────────────────────────────┐
│  Technical Indicators MCP Server    │
│  ┌───────────────────────────────┐  │
│  │  MCP Request Handler          │  │
│  │  - initialize                 │  │
│  │  - tools/list                 │  │
│  │  - tools/call                 │  │
│  └──────────┬────────────────────┘  │
│             ▼                        │
│  ┌───────────────────────────────┐  │
│  │  Indicator Service            │  │
│  │  - CalculateRSI()             │  │
│  │  - CalculateMACD()            │  │
│  │  - CalculateBollingerBands()  │  │
│  │  - CalculateEMA()             │  │
│  │  - CalculateADX()             │  │
│  └──────────┬────────────────────┘  │
│             ▼                        │
│  ┌───────────────────────────────┐  │
│  │  cinar/indicator/v2 Library   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

## Building

```bash
# Build the server binary
go build -o bin/technical-indicators ./cmd/mcp-servers/technical-indicators/

# Or use Task
task build
```

## Testing

### Unit Tests

```bash
# Run all unit tests
go test -v ./cmd/mcp-servers/technical-indicators/

# Run specific test
go test -v -run TestCalculateRSI_ValidInput ./cmd/mcp-servers/technical-indicators/
```

### Integration Tests

```bash
# Run integration tests (tests actual indicator calculations)
go test -v -tags=integration ./cmd/mcp-servers/technical-indicators/
```

### End-to-End Tests

```bash
# Run E2E tests via stdio protocol
./cmd/mcp-servers/technical-indicators/test_e2e.sh
```

## Usage

### Running the Server

```bash
# Start the server (reads from stdin, writes to stdout)
./bin/technical-indicators
```

### MCP Protocol Examples

#### 1. Initialize

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "serverInfo": {
      "name": "technical-indicators",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {}
    }
  }
}
```

#### 2. List Tools

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "calculate_rsi",
        "description": "Calculate Relative Strength Index (RSI) for trend strength analysis",
        "inputSchema": {
          "type": "object",
          "properties": {
            "prices": {
              "type": "array",
              "items": {"type": "number"},
              "description": "Array of closing prices"
            },
            "period": {
              "type": "number",
              "description": "RSI period (default: 14)",
              "default": 14
            }
          },
          "required": ["prices"]
        }
      }
      // ... 4 more tools
    ]
  }
}
```

#### 3. Calculate RSI

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "calculate_rsi",
    "arguments": {
      "prices": [44.34, 44.09, 43.61, 43.03, 43.52, 43.13, 42.66, 42.82, 42.67, 43.13, 43.37, 43.23, 43.08, 42.07, 41.99, 42.18, 42.49, 42.28, 42.51, 43.13],
      "period": 14
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "value": 44.07,
    "signal": "neutral"
  }
}
```

**Signals:**
- `"oversold"` - RSI < 30 (potential buy signal)
- `"neutral"` - RSI between 30-70
- `"overbought"` - RSI > 70 (potential sell signal)

#### 4. Calculate MACD

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "calculate_macd",
    "arguments": {
      "prices": [/* 50 prices */],
      "fast_period": 12,
      "slow_period": 26,
      "signal_period": 9
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "macd": 3.5,
    "signal": 3.5,
    "histogram": 0.0,
    "crossover": "none"
  }
}
```

**Crossover Values:**
- `"bullish"` - MACD crosses above signal line (buy signal)
- `"bearish"` - MACD crosses below signal line (sell signal)
- `"none"` - No crossover

#### 5. Calculate Bollinger Bands

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "calculate_bollinger_bands",
    "arguments": {
      "prices": [/* 30 prices */],
      "period": 20,
      "std_dev": 2
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "upper": 110.24,
    "middle": 104.50,
    "lower": 98.76,
    "width": 10.99,
    "signal": "neutral"
  }
}
```

**Signals:**
- `"buy"` - Price at or below lower band (oversold)
- `"sell"` - Price at or above upper band (overbought)
- `"neutral"` - Price within bands

#### 6. Calculate EMA

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "calculate_ema",
    "arguments": {
      "prices": [/* 20 prices */],
      "period": 10
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "value": 114.50,
    "trend": "bullish"
  }
}
```

**Trend Values:**
- `"bullish"` - Current price above EMA
- `"bearish"` - Current price below EMA
- `"neutral"` - Current price equals EMA

#### 7. Calculate ADX

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "calculate_adx",
    "arguments": {
      "high": [/* 30 high prices */],
      "low": [/* 30 low prices */],
      "close": [/* 30 close prices */],
      "period": 14
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "result": {
    "value": 69.45,
    "strength": "very_strong"
  }
}
```

**Strength Classifications:**
- `"weak"` - ADX < 25 (weak or absent trend)
- `"strong"` - ADX 25-50 (strong trend)
- `"very_strong"` - ADX > 50 (very strong trend)

## Default Parameters

| Indicator | Parameter | Default Value |
|-----------|-----------|---------------|
| RSI | period | 14 |
| MACD | fast_period | 12 |
| MACD | slow_period | 26 |
| MACD | signal_period | 9 |
| Bollinger Bands | period | 20 |
| Bollinger Bands | std_dev | 2 |
| EMA | period | (required) |
| ADX | period | 14 |

## Error Handling

The server returns standard JSON-RPC 2.0 error responses:

```json
{
  "jsonrpc": "2.0",
  "id": 123,
  "error": {
    "code": -32000,
    "message": "invalid period: 14 (must be between 1 and 10)"
  }
}
```

**Common Error Codes:**
- `-32601` - Method not found
- `-32602` - Invalid params (malformed JSON)
- `-32000` - Server error (calculation error, validation error)

**Common Validation Errors:**
- Insufficient price data for the specified period
- Missing required parameters (e.g., prices, period for EMA)
- Invalid parameter types
- Mismatched array lengths (ADX requires equal-length high/low/close arrays)

## Implementation Details

### Indicator Calculations

All indicators use the `github.com/cinar/indicator/v2` library, which implements calculations using Go channels for efficient streaming computation. The server:

1. Converts input price arrays to channels
2. Passes data through cinar/indicator functions
3. Collects results from output channels
4. Returns the most recent calculated value(s)

### ADX Special Case

ADX is not directly available in cinar/indicator v2, so we implement it manually using the Wilder's smoothing method:

1. Calculate True Range (TR), +DM, -DM
2. Apply Wilder's smoothing
3. Calculate +DI and -DI
4. Calculate DX
5. Apply Wilder's smoothing to DX to get ADX

### Logging

- All logs use zerolog and write to stderr
- Log levels: Info (startup/results), Debug (calculations), Warn (fallbacks), Error (failures)
- Stdout is strictly reserved for MCP JSON-RPC protocol

## Integration with CryptoFunk

This MCP server is part of the CryptoFunk multi-agent AI trading system. It provides technical analysis capabilities to:

- **Technical Analysis Agent** - Primary consumer of these indicators
- **Trend Following Agent** - Uses MACD, EMA, ADX
- **Mean Reversion Agent** - Uses RSI, Bollinger Bands
- **MCP Orchestrator** - Routes indicator requests from agents

## Performance

- **Calculation Speed**: Sub-millisecond for typical datasets (20-50 prices)
- **Memory Usage**: ~8.7 MB binary size
- **Concurrency**: Safe for concurrent requests (stateless service)

## Future Enhancements

Potential additions (not yet implemented):

- Stochastic Oscillator
- Fibonacci Retracements
- Volume indicators (OBV, VWAP)
- Ichimoku Cloud
- Custom indicator period validation
- Batch calculation support
- Historical indicator caching

## References

- [MCP Specification](https://modelcontextprotocol.io/)
- [cinar/indicator Documentation](https://github.com/cinar/indicator)
- [RSI Calculation](https://www.investopedia.com/terms/r/rsi.asp)
- [MACD Explanation](https://www.investopedia.com/terms/m/macd.asp)
- [Bollinger Bands Guide](https://www.investopedia.com/terms/b/bollingerbands.asp)
- [ADX Methodology](https://www.investopedia.com/terms/a/adx.asp)

## License

Part of CryptoFunk - see repository root LICENSE file.
