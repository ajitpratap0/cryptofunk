# MCP Server API Documentation

Complete API reference for all CryptoFunk MCP servers. This document provides detailed specifications for each tool, including input/output schemas, examples, and error handling.

## Table of Contents

1. [Overview](#overview)
2. [Market Data Server](#market-data-server)
3. [Technical Indicators Server](#technical-indicators-server)
4. [Risk Analyzer Server](#risk-analyzer-server)
5. [Order Executor Server](#order-executor-server)
6. [Error Codes](#error-codes)
7. [Integration Examples](#integration-examples)

## Overview

CryptoFunk uses 4 internal MCP servers that communicate via JSON-RPC 2.0 over stdio:

| Server | Purpose | Tools | Transport |
|--------|---------|-------|-----------|
| market-data | Real-time market data from Binance | 3 tools, 1 resource | stdio |
| technical-indicators | Technical analysis calculations | 5 tools | stdio |
| risk-analyzer | Risk management and portfolio analysis | 5 tools | stdio |
| order-executor | Order placement and session management | 7 tools | stdio |

### Protocol

All servers implement JSON-RPC 2.0 protocol:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "tool_name",
    "arguments": {
      "param1": "value1"
    }
  }
}
```

**Response Format**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Tool output"
      }
    ]
  }
}
```

### Starting Servers

```bash
# Start all servers via orchestrator
task run-orchestrator

# Or start individually for testing
./bin/market-data < request.json
./bin/technical-indicators < request.json
./bin/risk-analyzer < request.json
./bin/order-executor < request.json
```

---

## Market Data Server

**Server Name**: `market-data`
**Version**: 0.1.0
**Purpose**: Provides real-time market data from Binance API with caching and TimescaleDB sync
**Binary**: `bin/market-data`

### Configuration

```yaml
# configs/config.yaml
exchanges:
  binance:
    api_key: "${BINANCE_API_KEY}"
    api_secret: "${BINANCE_API_SECRET}"
    testnet: true  # Use testnet for development
```

### Tools

#### 1. get_current_price

Get the current price for a trading pair.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol (e.g., BTCUSDT, ETHUSDT)",
      "pattern": "^[A-Z]{6,10}$"
    }
  },
  "required": ["symbol"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_current_price",
    "arguments": {
      "symbol": "BTCUSDT"
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "symbol": "BTCUSDT",
    "price": "43521.50",
    "timestamp": 1704067200
  }
}
```

**Errors**:
- `-32602`: Invalid symbol format
- `-32000`: Binance API error (symbol not found, rate limit)

**Usage Example (Go)**:
```go
result, err := mcpClient.CallTool(ctx, "get_current_price", map[string]interface{}{
    "symbol": "BTCUSDT",
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Current price: %s\n", result["price"])
```

---

#### 2. get_ticker_24h

Get 24-hour ticker statistics including price change, volume, high/low.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol (e.g., BTCUSDT)",
      "pattern": "^[A-Z]{6,10}$"
    }
  },
  "required": ["symbol"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_ticker_24h",
    "arguments": {
      "symbol": "ETHUSDT"
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "symbol": "ETHUSDT",
    "price": "2341.50",
    "priceChangePercent": "3.45",
    "volume": "1234567.89",
    "high24h": "2385.00",
    "low24h": "2250.00",
    "timestamp": 1704067200
  }
}
```

**Errors**:
- `-32602`: Invalid symbol format
- `-32000`: Binance API error

**Curl Example**:
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_ticker_24h","arguments":{"symbol":"ETHUSDT"}}}' | \
  ./bin/market-data
```

---

#### 3. get_orderbook

Get order book depth (bid/ask levels) for a trading pair.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol (e.g., BTCUSDT)",
      "pattern": "^[A-Z]{6,10}$"
    },
    "limit": {
      "type": "number",
      "description": "Depth limit (5, 10, 20, 50, 100, 500, 1000)",
      "enum": [5, 10, 20, 50, 100, 500, 1000],
      "default": 20
    }
  },
  "required": ["symbol"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_orderbook",
    "arguments": {
      "symbol": "BTCUSDT",
      "limit": 10
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "symbol": "BTCUSDT",
    "bids": [
      ["43520.00", "1.234"],
      ["43519.50", "0.567"],
      ["43519.00", "2.345"]
    ],
    "asks": [
      ["43521.00", "0.987"],
      ["43521.50", "1.456"],
      ["43522.00", "3.210"]
    ],
    "timestamp": 1704067200
  }
}
```

**Response Format**:
- `bids`: Array of [price, quantity] ordered by price descending
- `asks`: Array of [price, quantity] ordered by price ascending

**Errors**:
- `-32602`: Invalid symbol or limit value
- `-32000`: Binance API error

---

### Resources

#### market://ticker/{symbol}

URI-based access to real-time ticker data.

**URI Pattern**: `market://ticker/{symbol}`

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/read",
  "params": {
    "uri": "market://ticker/BTCUSDT"
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "contents": [
      {
        "uri": "market://ticker/BTCUSDT",
        "mimeType": "application/json",
        "text": "{\"symbol\":\"BTCUSDT\",\"price\":\"43521.50\",\"priceChangePercent\":\"2.34\",\"volume\":\"9876543.21\",\"high24h\":\"44100.00\",\"low24h\":\"42800.00\",\"timestamp\":1704067200}"
      }
    ]
  }
}
```

---

## Technical Indicators Server

**Server Name**: `technical-indicators`
**Version**: 1.0.0
**Purpose**: Calculate technical analysis indicators (RSI, MACD, Bollinger Bands, EMA, ADX)
**Binary**: `bin/technical-indicators`
**Library**: Uses `github.com/cinar/indicator/v2` (60+ indicators)

### Tools

#### 1. calculate_rsi

Calculate Relative Strength Index (RSI) for trend strength analysis.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "prices": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of closing prices",
      "minItems": 14
    },
    "period": {
      "type": "number",
      "description": "RSI period (default: 14)",
      "default": 14,
      "minimum": 2,
      "maximum": 100
    }
  },
  "required": ["prices"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "calculate_rsi",
    "arguments": {
      "prices": [100, 102, 101, 105, 107, 106, 108, 110, 109, 111, 113, 112, 115, 117, 116],
      "period": 14
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "result": {
    "values": [55.2, 58.4, 61.3, 63.7],
    "period": 14,
    "interpretation": "neutral",
    "signal": "hold"
  }
}
```

**Interpretation**:
- RSI > 70: Overbought (potential sell signal)
- RSI < 30: Oversold (potential buy signal)
- 30-70: Neutral range

**Errors**:
- `-32602`: Invalid params (insufficient data points, invalid period)
- `-32000`: Calculation error

---

#### 2. calculate_macd

Calculate Moving Average Convergence Divergence (MACD) for trend analysis.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "prices": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of closing prices",
      "minItems": 26
    },
    "fast_period": {
      "type": "number",
      "description": "Fast EMA period (default: 12)",
      "default": 12,
      "minimum": 2
    },
    "slow_period": {
      "type": "number",
      "description": "Slow EMA period (default: 26)",
      "default": 26,
      "minimum": 2
    },
    "signal_period": {
      "type": "number",
      "description": "Signal line period (default: 9)",
      "default": 9,
      "minimum": 2
    }
  },
  "required": ["prices"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "calculate_macd",
    "arguments": {
      "prices": [100, 102, 101, 105, 107, 106, 108, 110, 109, 111, 113, 112, 115, 117, 116, 118, 120, 119, 122, 124, 123, 125, 127, 126, 128, 130, 129],
      "fast_period": 12,
      "slow_period": 26,
      "signal_period": 9
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "result": {
    "macd": [0.45, 0.52, 0.61, 0.68],
    "signal": [0.42, 0.48, 0.55, 0.62],
    "histogram": [0.03, 0.04, 0.06, 0.06],
    "interpretation": "bullish",
    "crossover": null
  }
}
```

**Interpretation**:
- MACD > Signal: Bullish (buy signal)
- MACD < Signal: Bearish (sell signal)
- Histogram crossover: Strong signal

**Errors**:
- `-32602`: Invalid params (insufficient data, invalid periods)

---

#### 3. calculate_bollinger_bands

Calculate Bollinger Bands for volatility analysis.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "prices": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of closing prices",
      "minItems": 20
    },
    "period": {
      "type": "number",
      "description": "Period for moving average (default: 20)",
      "default": 20,
      "minimum": 2
    },
    "std_dev": {
      "type": "number",
      "description": "Standard deviations (default: 2)",
      "default": 2,
      "minimum": 0.5,
      "maximum": 4
    }
  },
  "required": ["prices"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "calculate_bollinger_bands",
    "arguments": {
      "prices": [100, 102, 101, 105, 107, 106, 108, 110, 109, 111, 113, 112, 115, 117, 116, 118, 120, 119, 122, 124, 123],
      "period": 20,
      "std_dev": 2
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "result": {
    "upper": [125.4],
    "middle": [112.5],
    "lower": [99.6],
    "bandwidth": 22.91,
    "percent_b": 0.68,
    "interpretation": "neutral"
  }
}
```

**Interpretation**:
- Price near upper band: Overbought
- Price near lower band: Oversold
- Bandwidth: Measure of volatility (higher = more volatile)
- %B: Position within bands (0 = lower, 1 = upper)

**Errors**:
- `-32602`: Invalid params

---

#### 4. calculate_ema

Calculate Exponential Moving Average (EMA).

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "prices": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of closing prices"
    },
    "period": {
      "type": "number",
      "description": "EMA period",
      "minimum": 2
    }
  },
  "required": ["prices", "period"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "method": "tools/call",
  "params": {
    "name": "calculate_ema",
    "arguments": {
      "prices": [100, 102, 101, 105, 107, 106, 108, 110, 109, 111],
      "period": 5
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "result": {
    "values": [103.4, 104.6, 105.8, 107.2, 108.1, 109.4],
    "period": 5,
    "current": 109.4
  }
}
```

**Usage**: EMA crossovers (e.g., 12/26) signal trend changes.

---

#### 5. calculate_adx

Calculate Average Directional Index (ADX) for trend strength.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "high": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of high prices"
    },
    "low": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of low prices"
    },
    "close": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of closing prices"
    },
    "period": {
      "type": "number",
      "description": "ADX period (default: 14)",
      "default": 14,
      "minimum": 2
    }
  },
  "required": ["high", "low", "close"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "method": "tools/call",
  "params": {
    "name": "calculate_adx",
    "arguments": {
      "high": [105, 108, 107, 110, 112, 111, 114, 116, 115, 118, 120, 119, 122, 124, 123],
      "low": [100, 102, 101, 105, 107, 106, 108, 110, 109, 111, 113, 112, 115, 117, 116],
      "close": [102, 104, 103, 107, 109, 108, 111, 113, 112, 115, 117, 116, 119, 121, 120],
      "period": 14
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "result": {
    "adx": [18.5, 21.3, 24.7, 28.2],
    "plus_di": [25.4, 28.1, 31.2, 34.5],
    "minus_di": [18.2, 16.5, 14.8, 13.1],
    "interpretation": "trending",
    "strength": "moderate"
  }
}
```

**Interpretation**:
- ADX < 20: Weak trend (ranging market)
- ADX 20-40: Moderate trend
- ADX > 40: Strong trend
- +DI > -DI: Uptrend
- -DI > +DI: Downtrend

---

## Risk Analyzer Server

**Server Name**: `risk-analyzer`
**Version**: 1.0.0
**Purpose**: Portfolio risk management, position sizing, VaR, exposure limits, performance metrics
**Binary**: `bin/risk-analyzer`

### Tools

#### 1. calculate_position_size

Calculate optimal position size using Kelly Criterion.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "win_rate": {
      "type": "number",
      "description": "Win rate (0-1)",
      "minimum": 0,
      "maximum": 1
    },
    "avg_win": {
      "type": "number",
      "description": "Average win amount",
      "minimum": 0
    },
    "avg_loss": {
      "type": "number",
      "description": "Average loss amount",
      "minimum": 0
    },
    "capital": {
      "type": "number",
      "description": "Available capital",
      "minimum": 0
    },
    "max_risk_percent": {
      "type": "number",
      "description": "Maximum risk per trade (default: 0.02 = 2%)",
      "default": 0.02,
      "minimum": 0.001,
      "maximum": 0.1
    }
  },
  "required": ["win_rate", "avg_win", "avg_loss", "capital"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "method": "tools/call",
  "params": {
    "name": "calculate_position_size",
    "arguments": {
      "win_rate": 0.55,
      "avg_win": 150,
      "avg_loss": 100,
      "capital": 10000,
      "max_risk_percent": 0.02
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "result": {
    "kelly_fraction": 0.0833,
    "position_size": 833.33,
    "edge": 0.275,
    "recommended_size": 200.00,
    "risk_amount": 200.00,
    "warning": "Using max_risk_percent cap (Kelly suggests 8.33% but capped at 2%)"
  }
}
```

**Kelly Formula**: `f* = (p * W - (1-p)) / W`
- p: win rate
- W: win/loss ratio (avg_win/avg_loss)
- f*: optimal fraction of capital

**Safeguards**:
- Negative Kelly → position_size = 0 (no edge)
- Kelly > max_risk_percent → capped at max_risk_percent
- Minimum position size enforced

**Errors**:
- `-32602`: Invalid params (negative values, win_rate > 1)
- `-32000`: Calculation error

---

#### 2. calculate_var

Calculate Value at Risk (VaR) using historical simulation.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "returns": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of historical returns (e.g., daily % changes)",
      "minItems": 30
    },
    "confidence": {
      "type": "number",
      "description": "Confidence level (default: 0.95 = 95%)",
      "default": 0.95,
      "minimum": 0.8,
      "maximum": 0.99
    },
    "portfolio_value": {
      "type": "number",
      "description": "Current portfolio value",
      "minimum": 0
    }
  },
  "required": ["returns", "portfolio_value"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 21,
  "method": "tools/call",
  "params": {
    "name": "calculate_var",
    "arguments": {
      "returns": [0.02, -0.01, 0.03, -0.02, 0.01, -0.03, 0.04, -0.01, 0.02, -0.02, 0.01, 0.03, -0.04, 0.02, -0.01, 0.05, -0.02, 0.01, -0.03, 0.02, -0.01, 0.04, -0.02, 0.01, -0.01, 0.03, -0.02, 0.02, -0.01, 0.01],
      "confidence": 0.95,
      "portfolio_value": 50000
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 21,
  "result": {
    "var_percent": 0.0312,
    "var_amount": 1560.00,
    "confidence": 0.95,
    "interpretation": "95% confident that daily loss will not exceed $1,560",
    "percentile_return": -0.0312
  }
}
```

**Interpretation**:
- VaR at 95% confidence = $1,560 means:
  - 95% of the time, daily loss ≤ $1,560
  - 5% of the time, daily loss > $1,560
- Higher confidence = higher VaR (more conservative)

**Method**: Historical simulation using percentile method (sorts returns, finds percentile).

**Errors**:
- `-32602`: Insufficient data (< 30 returns), invalid confidence level

---

#### 3. check_portfolio_limits

Validate trade against portfolio risk limits (exposure, concentration, drawdown).

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol"
    },
    "quantity": {
      "type": "number",
      "description": "Order quantity",
      "minimum": 0
    },
    "price": {
      "type": "number",
      "description": "Order price",
      "minimum": 0
    },
    "side": {
      "type": "string",
      "description": "Order side",
      "enum": ["buy", "sell"]
    },
    "portfolio_value": {
      "type": "number",
      "description": "Current portfolio value",
      "minimum": 0
    },
    "current_exposure": {
      "type": "number",
      "description": "Current market exposure (0-1)",
      "minimum": 0,
      "maximum": 1
    },
    "current_drawdown": {
      "type": "number",
      "description": "Current drawdown (0-1)",
      "minimum": 0,
      "maximum": 1
    },
    "max_exposure": {
      "type": "number",
      "description": "Max portfolio exposure (default: 0.8)",
      "default": 0.8,
      "minimum": 0,
      "maximum": 1
    },
    "max_position_size": {
      "type": "number",
      "description": "Max single position size (default: 0.1)",
      "default": 0.1,
      "minimum": 0,
      "maximum": 1
    },
    "max_drawdown": {
      "type": "number",
      "description": "Max allowed drawdown (default: 0.2)",
      "default": 0.2,
      "minimum": 0,
      "maximum": 1
    }
  },
  "required": ["symbol", "quantity", "price", "side", "portfolio_value", "current_exposure", "current_drawdown"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 22,
  "method": "tools/call",
  "params": {
    "name": "check_portfolio_limits",
    "arguments": {
      "symbol": "BTCUSDT",
      "quantity": 0.5,
      "price": 43500,
      "side": "buy",
      "portfolio_value": 100000,
      "current_exposure": 0.65,
      "current_drawdown": 0.08,
      "max_exposure": 0.8,
      "max_position_size": 0.1,
      "max_drawdown": 0.2
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 22,
  "result": {
    "allowed": true,
    "position_size_percent": 0.2175,
    "new_exposure": 0.8675,
    "checks": {
      "position_size": {
        "passed": false,
        "limit": 0.1,
        "actual": 0.2175,
        "message": "Position size (21.75%) exceeds limit (10%)"
      },
      "exposure": {
        "passed": false,
        "limit": 0.8,
        "actual": 0.8675,
        "message": "New exposure (86.75%) exceeds limit (80%)"
      },
      "drawdown": {
        "passed": true,
        "limit": 0.2,
        "actual": 0.08,
        "message": "Drawdown within limits"
      }
    },
    "violations": ["position_size", "exposure"],
    "recommendation": "Reduce position size to 0.23 BTC to stay within limits"
  }
}
```

**Validation Checks**:
1. **Position Size**: Order value / portfolio value ≤ max_position_size
2. **Exposure**: (current_exposure + position_size) ≤ max_exposure
3. **Drawdown**: current_drawdown ≤ max_drawdown (circuit breaker)

**Errors**:
- `-32602`: Invalid params

---

#### 4. calculate_sharpe

Calculate annualized Sharpe Ratio (risk-adjusted returns).

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "returns": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of periodic returns",
      "minItems": 30
    },
    "risk_free_rate": {
      "type": "number",
      "description": "Risk-free rate (default: 0.02 = 2%/year)",
      "default": 0.02,
      "minimum": 0,
      "maximum": 0.1
    },
    "periods_per_year": {
      "type": "number",
      "description": "Periods per year (252 for daily, 12 for monthly)",
      "default": 252,
      "minimum": 1
    }
  },
  "required": ["returns"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 23,
  "method": "tools/call",
  "params": {
    "name": "calculate_sharpe",
    "arguments": {
      "returns": [0.02, -0.01, 0.03, -0.02, 0.01, -0.03, 0.04, -0.01, 0.02, -0.02, 0.01, 0.03, -0.04, 0.02, -0.01, 0.05, -0.02, 0.01, -0.03, 0.02, -0.01, 0.04, -0.02, 0.01, -0.01, 0.03, -0.02, 0.02, -0.01, 0.01, 0.02, -0.01],
      "risk_free_rate": 0.02,
      "periods_per_year": 252
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 23,
  "result": {
    "sharpe_ratio": 1.45,
    "annualized_return": 0.126,
    "annualized_volatility": 0.087,
    "interpretation": "good",
    "rating": "Above average risk-adjusted returns"
  }
}
```

**Sharpe Formula**: `(mean(returns) - risk_free_rate) / std(returns) * sqrt(periods_per_year)`

**Interpretation**:
- < 0: Poor (returns below risk-free rate)
- 0-1: Acceptable
- 1-2: Good
- 2-3: Very good
- \> 3: Excellent

**Errors**:
- `-32602`: Insufficient data

---

#### 5. calculate_drawdown

Calculate maximum and current drawdown with recovery tracking.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "equity_curve": {
      "type": "array",
      "items": {"type": "number"},
      "description": "Array of portfolio values over time",
      "minItems": 2
    }
  },
  "required": ["equity_curve"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 24,
  "method": "tools/call",
  "params": {
    "name": "calculate_drawdown",
    "arguments": {
      "equity_curve": [10000, 10500, 10200, 11000, 10800, 9800, 10100, 11500, 11200, 12000]
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 24,
  "result": {
    "max_drawdown": 0.1091,
    "max_drawdown_percent": 10.91,
    "current_drawdown": 0.0,
    "current_drawdown_percent": 0.0,
    "peak_value": 12000,
    "valley_value": 9800,
    "current_value": 12000,
    "is_recovering": false,
    "recovery_percent": 100.0,
    "interpretation": "Currently at all-time high",
    "alert": null
  }
}
```

**Drawdown Calculation**:
- Running peak: max value seen so far
- Drawdown = (peak - current) / peak
- Max drawdown = largest drawdown observed

**Alerts**:
- > 10%: "Moderate drawdown"
- > 20%: "Significant drawdown - review strategy"
- > 30%: "Severe drawdown - consider halting"

**Errors**:
- `-32602`: Insufficient data

---

## Order Executor Server

**Server Name**: `order-executor`
**Version**: 1.0.0
**Purpose**: Order placement, session management, paper/live trading
**Binary**: `bin/order-executor`
**Database**: Required (stores orders, positions, sessions)

### Configuration

```yaml
# configs/config.yaml
trading:
  mode: PAPER  # or LIVE (use PAPER for testing)

exchange:
  name: binance
  api_key: "${BINANCE_API_KEY}"
  api_secret: "${BINANCE_API_SECRET}"
  testnet: true

database:
  url: "${DATABASE_URL}"
```

### Tools

#### 1. place_market_order

Place a market order (immediate execution at current market price).

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol (e.g., 'BTCUSDT')",
      "pattern": "^[A-Z]{6,10}$"
    },
    "side": {
      "type": "string",
      "description": "Order side: 'buy' or 'sell'",
      "enum": ["buy", "sell"]
    },
    "quantity": {
      "type": "number",
      "description": "Order quantity",
      "minimum": 0,
      "exclusiveMinimum": true
    }
  },
  "required": ["symbol", "side", "quantity"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 30,
  "method": "tools/call",
  "params": {
    "name": "place_market_order",
    "arguments": {
      "symbol": "BTCUSDT",
      "side": "buy",
      "quantity": 0.01
    }
  }
}
```

**Example Response (Paper Trading)**:
```json
{
  "jsonrpc": "2.0",
  "id": 30,
  "result": {
    "order_id": "mock-order-1704067200-12345",
    "symbol": "BTCUSDT",
    "side": "buy",
    "type": "market",
    "status": "filled",
    "quantity": 0.01,
    "filled_quantity": 0.01,
    "price": 43521.50,
    "average_price": 43532.18,
    "slippage": 0.0246,
    "timestamp": 1704067200,
    "mode": "paper"
  }
}
```

**Paper Trading Behavior**:
- Simulates realistic fills with slippage (0.05%-0.3%)
- Larger orders → more slippage and potential partial fills
- Instant execution (no queue)

**Live Trading Behavior**:
- Sends order to exchange via CCXT
- Real slippage and fees apply
- Returns exchange order ID

**Errors**:
- `-32602`: Invalid params (negative quantity, invalid symbol)
- `-32603`: Insufficient balance
- `-32000`: Exchange error (rate limit, invalid symbol)

---

#### 2. place_limit_order

Place a limit order (executed at specified price or better).

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol (e.g., 'BTCUSDT')"
    },
    "side": {
      "type": "string",
      "description": "Order side: 'buy' or 'sell'",
      "enum": ["buy", "sell"]
    },
    "quantity": {
      "type": "number",
      "description": "Order quantity",
      "minimum": 0,
      "exclusiveMinimum": true
    },
    "price": {
      "type": "number",
      "description": "Limit price",
      "minimum": 0,
      "exclusiveMinimum": true
    }
  },
  "required": ["symbol", "side", "quantity", "price"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 31,
  "method": "tools/call",
  "params": {
    "name": "place_limit_order",
    "arguments": {
      "symbol": "ETHUSDT",
      "side": "sell",
      "quantity": 1.5,
      "price": 2350.00
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 31,
  "result": {
    "order_id": "mock-order-1704067201-54321",
    "symbol": "ETHUSDT",
    "side": "sell",
    "type": "limit",
    "status": "new",
    "quantity": 1.5,
    "filled_quantity": 0.0,
    "price": 2350.00,
    "timestamp": 1704067201,
    "mode": "paper"
  }
}
```

**Order Status Progression**:
- `new` → `partially_filled` → `filled`
- Can be `canceled` at any time before `filled`

**Errors**:
- `-32602`: Invalid params
- `-32603`: Insufficient balance

---

#### 3. cancel_order

Cancel an open or pending order.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "order_id": {
      "type": "string",
      "description": "Order ID to cancel"
    }
  },
  "required": ["order_id"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 32,
  "method": "tools/call",
  "params": {
    "name": "cancel_order",
    "arguments": {
      "order_id": "mock-order-1704067201-54321"
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 32,
  "result": {
    "order_id": "mock-order-1704067201-54321",
    "status": "canceled",
    "canceled_at": 1704067300,
    "message": "Order successfully canceled"
  }
}
```

**Errors**:
- `-32602`: Invalid order_id
- `-32000`: Order not found or already filled/canceled

---

#### 4. get_order_status

Get current status and details of an order.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "order_id": {
      "type": "string",
      "description": "Order ID to query"
    }
  },
  "required": ["order_id"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 33,
  "method": "tools/call",
  "params": {
    "name": "get_order_status",
    "arguments": {
      "order_id": "mock-order-1704067200-12345"
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 33,
  "result": {
    "order_id": "mock-order-1704067200-12345",
    "symbol": "BTCUSDT",
    "side": "buy",
    "type": "market",
    "status": "filled",
    "quantity": 0.01,
    "filled_quantity": 0.01,
    "price": 43521.50,
    "average_price": 43532.18,
    "created_at": 1704067200,
    "updated_at": 1704067201,
    "fills": [
      {
        "price": 43532.18,
        "quantity": 0.01,
        "timestamp": 1704067201
      }
    ]
  }
}
```

**Errors**:
- `-32602`: Invalid order_id
- `-32000`: Order not found

---

#### 5. start_session

Start a new trading session for paper trading.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Trading pair symbol (e.g., 'BTCUSDT')"
    },
    "initial_capital": {
      "type": "number",
      "description": "Initial capital for the trading session",
      "minimum": 0,
      "exclusiveMinimum": true
    },
    "config": {
      "type": "object",
      "description": "Optional configuration parameters for the session",
      "properties": {
        "max_position_size": {"type": "number"},
        "max_drawdown": {"type": "number"},
        "risk_per_trade": {"type": "number"}
      }
    }
  },
  "required": ["symbol", "initial_capital"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 34,
  "method": "tools/call",
  "params": {
    "name": "start_session",
    "arguments": {
      "symbol": "BTCUSDT",
      "initial_capital": 10000,
      "config": {
        "max_position_size": 0.1,
        "max_drawdown": 0.2,
        "risk_per_trade": 0.02
      }
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 34,
  "result": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "symbol": "BTCUSDT",
    "initial_capital": 10000,
    "current_capital": 10000,
    "mode": "paper",
    "status": "active",
    "started_at": 1704067200,
    "config": {
      "max_position_size": 0.1,
      "max_drawdown": 0.2,
      "risk_per_trade": 0.02
    }
  }
}
```

**Session Lifecycle**:
- `active` → trading allowed
- `paused` → manual pause
- `stopped` → session ended
- `halted` → circuit breaker triggered

**Errors**:
- `-32602`: Invalid params
- `-32000`: Session already active

---

#### 6. stop_session

Stop the current trading session and retrieve final statistics.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "final_capital": {
      "type": "number",
      "description": "Final capital at session end",
      "minimum": 0
    }
  },
  "required": ["final_capital"]
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 35,
  "method": "tools/call",
  "params": {
    "name": "stop_session",
    "arguments": {
      "final_capital": 10500
    }
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 35,
  "result": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "symbol": "BTCUSDT",
    "initial_capital": 10000,
    "final_capital": 10500,
    "total_return": 0.05,
    "total_return_percent": 5.0,
    "total_trades": 12,
    "winning_trades": 7,
    "losing_trades": 5,
    "win_rate": 0.5833,
    "max_drawdown": 0.03,
    "sharpe_ratio": 1.45,
    "started_at": 1704067200,
    "stopped_at": 1704153600,
    "duration_hours": 24,
    "status": "stopped"
  }
}
```

**Errors**:
- `-32602`: Invalid params
- `-32000`: No active session

---

#### 7. get_session_stats

Get current statistics for the active trading session.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {},
  "required": []
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 36,
  "method": "tools/call",
  "params": {
    "name": "get_session_stats",
    "arguments": {}
  }
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 36,
  "result": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "symbol": "BTCUSDT",
    "initial_capital": 10000,
    "current_capital": 10350,
    "unrealized_pnl": 150,
    "realized_pnl": 200,
    "total_return": 0.035,
    "total_return_percent": 3.5,
    "total_trades": 8,
    "winning_trades": 5,
    "losing_trades": 3,
    "win_rate": 0.625,
    "current_drawdown": 0.0,
    "max_drawdown": 0.02,
    "status": "active",
    "uptime_hours": 12.5
  }
}
```

**Real-time Metrics**:
- `unrealized_pnl`: Profit/loss on open positions
- `realized_pnl`: Profit/loss on closed positions
- `current_drawdown`: Current drawdown from peak
- `win_rate`: winning_trades / total_trades

**Errors**:
- `-32000`: No active session

---

## Error Codes

Standard JSON-RPC 2.0 error codes used across all servers:

| Code | Name | Description | Common Causes |
|------|------|-------------|---------------|
| -32700 | Parse error | Invalid JSON | Malformed request |
| -32600 | Invalid Request | Missing required fields | No `method` or `params` |
| -32601 | Method not found | Unknown method | Typo in tool name |
| -32602 | Invalid params | Invalid parameters | Wrong type, missing required param |
| -32603 | Internal error | Server-side error | Database error, runtime panic |
| -32000 | Server error | Application error | API rate limit, insufficient balance, order not found |

**Error Response Format**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params: symbol must be a string",
    "data": {
      "param": "symbol",
      "expected": "string",
      "got": "number"
    }
  }
}
```

### Common Error Scenarios

**Market Data Server**:
- `-32602`: Invalid symbol format (must be uppercase, 6-10 chars)
- `-32000`: Binance API rate limit exceeded (60 requests/minute)
- `-32000`: Symbol not found on exchange

**Technical Indicators Server**:
- `-32602`: Insufficient data points for indicator (need ≥ period length)
- `-32602`: Invalid period value (must be ≥ 2)
- `-32000`: Calculation error (e.g., division by zero)

**Risk Analyzer Server**:
- `-32602`: Win rate out of range (0-1)
- `-32602`: Negative capital or returns
- `-32000`: Insufficient historical data for VaR (need ≥ 30 points)

**Order Executor Server**:
- `-32603`: Insufficient balance for order
- `-32000`: Order not found (invalid order_id)
- `-32000`: Exchange API error (network, rate limit)
- `-32000`: Session not active (must call start_session first)

---

## Integration Examples

### Example 1: Complete Trading Flow

```go
// 1. Get market data
priceData, err := mcpClient.CallTool(ctx, "get_ticker_24h", map[string]interface{}{
    "symbol": "BTCUSDT",
})

// 2. Calculate technical indicators
rsi, err := mcpClient.CallTool(ctx, "calculate_rsi", map[string]interface{}{
    "prices": historicalPrices,
    "period": 14,
})

macd, err := mcpClient.CallTool(ctx, "calculate_macd", map[string]interface{}{
    "prices": historicalPrices,
})

// 3. Calculate position size
posSize, err := mcpClient.CallTool(ctx, "calculate_position_size", map[string]interface{}{
    "win_rate": 0.55,
    "avg_win": 150,
    "avg_loss": 100,
    "capital": 10000,
})

// 4. Check risk limits
limitsCheck, err := mcpClient.CallTool(ctx, "check_portfolio_limits", map[string]interface{}{
    "symbol": "BTCUSDT",
    "quantity": posSize["recommended_size"].(float64) / priceData["price"].(float64),
    "price": priceData["price"].(float64),
    "side": "buy",
    "portfolio_value": 10000,
    "current_exposure": 0.5,
    "current_drawdown": 0.05,
})

// 5. Place order if checks pass
if limitsCheck["allowed"].(bool) {
    order, err := mcpClient.CallTool(ctx, "place_market_order", map[string]interface{}{
        "symbol": "BTCUSDT",
        "side": "buy",
        "quantity": posSize["recommended_size"].(float64) / priceData["price"].(float64),
    })
}
```

### Example 2: Paper Trading Session

```bash
# Start session
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"start_session","arguments":{"symbol":"BTCUSDT","initial_capital":10000}}}' | ./bin/order-executor

# Get current stats
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_session_stats","arguments":{}}}' | ./bin/order-executor

# Place trades...
# (see order executor examples above)

# Stop session
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"stop_session","arguments":{"final_capital":10500}}}' | ./bin/order-executor
```

### Example 3: Multi-Indicator Strategy

```python
import json
import subprocess

def call_mcp_tool(binary, tool_name, args):
    request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": args
        }
    }

    result = subprocess.run(
        [binary],
        input=json.dumps(request),
        capture_output=True,
        text=True
    )

    return json.loads(result.stdout)["result"]

# Get historical prices
prices = [100, 102, 101, 105, 107, 106, 108, 110, 109, 111, 113, 112, 115, 117, 116]

# Calculate multiple indicators
rsi = call_mcp_tool("./bin/technical-indicators", "calculate_rsi", {
    "prices": prices,
    "period": 14
})

macd = call_mcp_tool("./bin/technical-indicators", "calculate_macd", {
    "prices": prices
})

bb = call_mcp_tool("./bin/technical-indicators", "calculate_bollinger_bands", {
    "prices": prices,
    "period": 20
})

# Generate signal based on indicators
if rsi["values"][-1] < 30 and macd["histogram"][-1] > 0:
    print("BUY SIGNAL: Oversold + MACD positive")
elif rsi["values"][-1] > 70 and macd["histogram"][-1] < 0:
    print("SELL SIGNAL: Overbought + MACD negative")
else:
    print("NEUTRAL: Wait for clearer signal")
```

### Example 4: Risk Management Workflow

```go
// Calculate current portfolio metrics
returns := []float64{0.02, -0.01, 0.03, -0.02, 0.01, /* ... */}
equityCurve := []float64{10000, 10200, 10100, 10300, /* ... */}

// 1. Check Sharpe Ratio
sharpe, _ := mcpClient.CallTool(ctx, "calculate_sharpe", map[string]interface{}{
    "returns": returns,
    "risk_free_rate": 0.02,
    "periods_per_year": 252,
})

// 2. Calculate VaR
var, _ := mcpClient.CallTool(ctx, "calculate_var", map[string]interface{}{
    "returns": returns,
    "confidence": 0.95,
    "portfolio_value": 50000,
})

// 3. Check drawdown
dd, _ := mcpClient.CallTool(ctx, "calculate_drawdown", map[string]interface{}{
    "equity_curve": equityCurve,
})

// 4. Make trading decision based on risk metrics
if sharpe["sharpe_ratio"].(float64) > 1.0 &&
   dd["current_drawdown"].(float64) < 0.15 {
    // Safe to trade
    log.Info("Risk metrics acceptable, proceeding with trade")
} else {
    // Pause trading
    log.Warn("Risk metrics concerning, pausing trading")
}
```

---

## Best Practices

### 1. Error Handling

Always check for errors and handle them gracefully:

```go
result, err := mcpClient.CallTool(ctx, "place_market_order", args)
if err != nil {
    if mcpErr, ok := err.(*MCPError); ok {
        switch mcpErr.Code {
        case -32602:
            log.Error("Invalid parameters", "error", mcpErr.Message)
        case -32603:
            log.Error("Insufficient balance", "error", mcpErr.Message)
        case -32000:
            log.Error("Exchange error", "error", mcpErr.Message)
        default:
            log.Error("Unknown error", "error", mcpErr.Message)
        }
    }
    return err
}
```

### 2. Parameter Validation

Validate inputs before calling tools:

```go
// Validate symbol format
if !regexp.MustCompile(`^[A-Z]{6,10}$`).MatchString(symbol) {
    return fmt.Errorf("invalid symbol format: %s", symbol)
}

// Validate numeric ranges
if winRate < 0 || winRate > 1 {
    return fmt.Errorf("win_rate must be between 0 and 1")
}
```

### 3. Caching

Market data servers cache responses. Avoid excessive calls:

```go
// Good: Call once, reuse data
ticker, _ := mcpClient.CallTool(ctx, "get_ticker_24h", args)
price := ticker["price"].(float64)

// Bad: Repeated calls for same data
price1, _ := mcpClient.CallTool(ctx, "get_current_price", args)
price2, _ := mcpClient.CallTool(ctx, "get_current_price", args) // Unnecessary
```

### 4. Paper Trading First

Always test strategies in paper mode before live trading:

```yaml
# configs/config.yaml
trading:
  mode: PAPER  # Start here!
```

### 5. Circuit Breakers

Monitor drawdown and halt trading if limits exceeded:

```go
dd, _ := mcpClient.CallTool(ctx, "calculate_drawdown", args)
if dd["current_drawdown"].(float64) > 0.20 {
    // Stop trading immediately
    mcpClient.CallTool(ctx, "stop_session", map[string]interface{}{
        "final_capital": currentCapital,
    })
    log.Error("Circuit breaker triggered: 20% drawdown")
}
```

### 6. Logging

All servers log to stderr (stdout reserved for MCP protocol):

```go
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
log.Info().Str("symbol", symbol).Msg("Processing order")
```

---

## Troubleshooting

### Server Not Responding

**Problem**: MCP server doesn't respond to requests

**Solutions**:
1. Check server is running: `ps aux | grep market-data`
2. Check stderr logs: `./bin/market-data 2>&1 | grep ERROR`
3. Verify request format (must be valid JSON-RPC 2.0)
4. Ensure stdout is not being used for debugging

### Invalid Tool Name

**Problem**: `-32601 Method not found`

**Solutions**:
1. List available tools: `echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./bin/market-data`
2. Check spelling and case (e.g., `calculate_rsi` not `calculateRSI`)
3. Verify server supports the tool

### Parameter Errors

**Problem**: `-32602 Invalid params`

**Solutions**:
1. Check required parameters are provided
2. Verify data types (number vs string)
3. Validate ranges (e.g., win_rate must be 0-1)
4. Check array lengths (e.g., RSI needs ≥14 prices)

### Database Connection Issues

**Problem**: Order executor fails with database error

**Solutions**:
1. Check database is running: `docker-compose ps postgres`
2. Verify connection string: `echo $DATABASE_URL`
3. Run migrations: `task db-migrate`
4. Check logs: `docker-compose logs postgres`

### Exchange API Errors

**Problem**: `-32000 Binance API error`

**Solutions**:
1. Check API keys are set: `echo $BINANCE_API_KEY`
2. Verify testnet mode: `configs/config.yaml` → `testnet: true`
3. Check rate limits (60 requests/minute)
4. Verify symbol exists on exchange

---

## Performance Considerations

### Latency

Expected tool latency (Paper mode):

| Tool | Latency | Notes |
|------|---------|-------|
| get_current_price | 50-100ms | Cached 60s |
| calculate_rsi | 1-5ms | In-memory calculation |
| calculate_var | 5-10ms | Percentile method |
| place_market_order | 10-20ms | Paper trading (instant fill) |
| place_market_order | 200-500ms | Live trading (network + exchange) |

### Caching

- Market data: 60s TTL for ticker, 5min for OHLCV
- Indicators: No caching (stateless calculations)
- Risk metrics: No caching (portfolio state changes)

### Concurrency

All servers are single-threaded (stdio transport). For high-throughput:
1. Use HTTP transport (Phase 9)
2. Deploy multiple server instances
3. Load balance with NGINX

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2024-01-01 | Initial release (Phase 2.4 complete) |
| - | - | Market Data, Technical Indicators, Risk Analyzer, Order Executor |

---

## Related Documentation

- [MCP_INTEGRATION.md](./MCP_INTEGRATION.md) - MCP architecture and integration guide
- [TASKS.md](../TASKS.md) - Implementation roadmap (10 phases, 244 tasks)
- [README.md](../README.md) - Project overview and quick start
- [CLAUDE.md](../CLAUDE.md) - Development guidelines for Claude Code

---

## Support

For issues or questions:
1. Check error codes in this document
2. Review server stderr logs
3. Verify configuration in `configs/config.yaml`
4. Check database schema: `migrations/001_initial_schema.sql`
5. Test with paper trading mode first

**Note**: This documentation reflects Phase 2.4 implementation status. Additional tools and features will be added in Phases 3-10.
