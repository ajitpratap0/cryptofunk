# CryptoFunk - AI Trading Platform Architecture

**Version:** 1.0
**Date:** 2025-10-27
**Status:** Design Phase

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [System Architecture](#2-system-architecture)
3. [Model Context Protocol (MCP) Integration](#3-model-context-protocol-mcp-integration)
4. [Technology Stack](#4-technology-stack)
5. [System Components](#5-system-components)
6. [Multi-Agent Orchestration](#6-multi-agent-orchestration)
7. [Data Flow](#7-data-flow)
8. [Code Examples](#8-code-examples)
9. [Configuration](#9-configuration)
10. [Deployment Strategy](#10-deployment-strategy)
11. [Testing Strategy](#11-testing-strategy)
12. [Risk Management](#12-risk-management)
13. [Monitoring & Observability](#13-monitoring--observability)
14. [Future Enhancements](#14-future-enhancements)

> **Note**: For detailed implementation tasks and roadmap, see [TASKS.md](TASKS.md)

---

## 1. Executive Summary

### 1.1 Project Overview

**CryptoFunk** is a state-of-the-art cryptocurrency AI trading platform built using Go, **LLM-powered agents** (Claude/GPT-4 via Bifrost), and orchestrated through the Model Context Protocol (MCP). The system employs a multi-agent architecture where specialized LLM-powered AI agents collaborate to make informed trading decisions with natural language reasoning in real-time.

### 1.2 Key Features

- **LLM-Powered Intelligence**: Agents use Claude/GPT-4 for sophisticated reasoning via Bifrost gateway
- **Bifrost Gateway**: Unified API for 12+ LLM providers with automatic failover and semantic caching
- **Multi-Agent System**: Specialized LLM agents for technical analysis, strategy, risk management, and execution
- **MCP Orchestration**: Standardized agent communication using the official MCP Go SDK
- **Real-time Trading**: WebSocket connections via CCXT to 100+ exchanges
- **Technical Analysis**: 60+ indicators via cinar/indicator library
- **Risk Controls**: Multi-layered LLM-powered risk management with circuit breakers
- **Explainable AI**: Every decision includes natural language reasoning and confidence scores
- **Paper Trading**: Safe testing environment before live trading
- **Scalable Architecture**: Containerized, cloud-ready deployment

### 1.3 Design Philosophy

- **LLM-First**: Leverage public LLMs (Claude/GPT-4) for MVP, train custom models post-MVP with collected data
- **Modularity**: Each agent is an independent process communicating via MCP
- **High Availability**: Automatic failover between LLM providers (Claude → GPT-4 → Gemini)
- **Cost Optimization**: Semantic caching reduces LLM costs by 90%
- **Type Safety**: Leveraging Go's type system with MCP's auto-schema generation
- **Observability**: Comprehensive logging and metrics at every layer
- **Explainability**: All decisions include natural language reasoning for auditability
- **Fail-Safe**: Multiple layers of risk controls and circuit breakers
- **Standards-Based**: Using official MCP SDK maintained by Anthropic and Google

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     REST/WebSocket API                          │
│         (Monitoring, Control, Explainability Dashboard)         │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                   MCP ORCHESTRATOR (Host)                       │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Core Functions:                                          │  │
│  │  - Agent Lifecycle Management                             │  │
│  │  - Context Aggregation & Distribution                     │  │
│  │  - Decision Coordination (Voting/LLM Consensus)           │  │
│  │  - Session State Management (Redis)                       │  │
│  │  - Risk Approval Pipeline (LLM-powered)                   │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────┬───────────────────────────────────────┘
                          │
         ┌────────────────┼────────────────┐
         │                │                │
    ┌────▼─────┐    ┌────▼─────┐    ┌────▼─────┐
    │ Analysis │    │ Strategy │    │   Risk   │
    │  Agents  │    │  Agents  │    │  Agent   │
    │  (LLM)   │    │  (LLM)   │    │  (LLM)   │
    └────┬─────┘    └────┬─────┘    └────┬─────┘
         │               │               │
         └───────────────┼───────────────┘
                         │
        ┌────────────────▼────────────────┐
        │    BIFROST LLM GATEWAY          │
        │  ┌───────────────────────────┐  │
        │  │ Claude Sonnet 4 (Primary) │  │
        │  │ GPT-4 Turbo (Fallback)    │  │
        │  │ Gemini Pro (Backup)       │  │
        │  └───────────────────────────┘  │
        │  • Automatic Failover          │
        │  • Semantic Caching (90%)      │
        │  • <100µs Overhead             │
        └────────────────┬────────────────┘
                         │
    ┌────────────────────▼────────────────┐
    │     MCP Tool/Resource Servers       │
    │  ┌──────────────┐  ┌──────────────┐│
    │  │ Market Data  │  │Tech Indicators││
    │  │(CCXT/Binance)│  │(cinar/indicator)│
    │  └──────────────┘  └──────────────┘│
    │  ┌──────────────┐  ┌──────────────┐│
    │  │Risk Analyzer │  │Order Executor││
    │  │(LLM+Rules)   │  │(CCXT)        ││
    │  └──────────────┘  └──────────────┘│
    └────────┬──────────────────┬─────────┘
             │                  │
    ┌────────▼──────────────────▼─────────┐
    │      External Services              │
    │  - CCXT (100+ Exchanges)            │
    │  - Coinbase API                           │
    │  - News APIs                              │
    │  - External MCP Servers (Optional)        │
    └───────────────────────────────────────────┘
```

### 2.2 Component Layers

#### Layer 1: API Gateway
- REST API for monitoring and control
- WebSocket for real-time updates
- Authentication and authorization

#### Layer 2: MCP Orchestrator
- Central coordination hub
- Manages all MCP client connections to agents
- Implements consensus and voting mechanisms
- Maintains session state

#### Layer 3: Agents (MCP Clients)
- Independent processes
- Specialized decision-making units
- Communicate via MCP protocol

#### Layer 4: MCP Servers
- Provide tools (functions) and resources (data)
- Stateless, reusable components
- Can be shared across multiple agents

#### Layer 5: Data & External Services
- Exchange APIs
- Databases (PostgreSQL, TimescaleDB, Redis)
- Message queues (NATS/Kafka)

---

## 3. Model Context Protocol (MCP) Integration

### 3.1 What is MCP?

The Model Context Protocol (MCP) is an open standard that enables standardized communication between AI agents and data sources/tools. Developed by Anthropic and maintained in collaboration with Google, MCP provides:

- **Standardized Communication**: Consistent protocol for agent interactions
- **Tool Discovery**: Automatic discovery of available tools and their schemas
- **Type Safety**: Schema validation for inputs/outputs
- **Transport Agnostic**: Support for stdio, SSE, HTTP
- **Resource Management**: Efficient access to data resources

### 3.2 Why MCP for Trading?

#### Traditional Approach Challenges
- Custom APIs for each agent
- No standardization
- Difficult to add/remove agents
- Complex inter-agent communication
- Schema mismatches

#### MCP Advantages
- **Plug-and-Play Agents**: Add new agents without modifying existing code
- **Standardized Context**: All agents speak the same protocol
- **Tool Reusability**: One technical indicator server serves all agents
- **Type Safety**: Auto-generated schemas from Go structs
- **Production Ready**: Used by Microsoft, Anthropic, and major enterprises
- **Community Ecosystem**: Leverage existing MCP servers (Binance, CCXT, etc.)

### 3.3 MCP Components in CryptoFunk

#### 3.3.1 MCP Servers (Tools & Resources Providers)

**Tools**: Functions that can be invoked by agents
```go
// Example: RSI Calculation Tool
type RSIInput struct {
    Symbol string `json:"symbol" jsonschema:"required,description=Trading pair"`
    Period int    `json:"period" jsonschema:"description=RSI period"`
}

type RSIOutput struct {
    Value  float64 `json:"value"`
    Signal string  `json:"signal"` // oversold/overbought/neutral
}
```

**Resources**: Data that can be read by agents
```
URI: market://candlesticks/BTCUSDT/1h
URI: market://orderbook/ETHUSDT
URI: portfolio://positions
```

#### 3.3.2 MCP Clients (Agents)

Each agent is an MCP client that:
- Connects to one or more MCP servers
- Invokes tools for computation
- Reads resources for data
- Publishes decisions to orchestrator

#### 3.3.3 MCP Host (Orchestrator)

The orchestrator acts as an MCP host that:
- Manages multiple agent (client) connections
- Aggregates agent outputs
- Implements coordination logic
- Maintains shared context

### 3.4 Official MCP Go SDK

**Repository**: `github.com/modelcontextprotocol/go-sdk`

**Key Features**:
- Generic `AddTool[In, Out]` with type safety
- Automatic JSON schema generation from Go structs
- Multiple transport support (stdio, in-memory for testing)
- Full MCP spec implementation
- Maintained by MCP organization + Google
- 261+ packages using it in production

**Example**:
```go
server := mcp.NewServer(&mcp.Implementation{
    Name:    "technical-indicators",
    Version: "v1.0.0",
}, nil)

mcp.AddTool(server, &mcp.Tool{
    Name:        "calculate_rsi",
    Description: "Calculate RSI indicator",
}, calculateRSIHandler)
```

---

## 4. Technology Stack

### 4.1 Core Technologies

| Component | Technology | Version | Rationale |
|-----------|-----------|---------|-----------|
| Language | Go | 1.21+ | Performance, concurrency, type safety |
| **LLM Gateway** | **Bifrost** | Latest | Unified API for Claude/GPT-4/Gemini, auto-failover, 50x faster |
| **LLM Primary** | **Claude Sonnet 4** | Latest | Best-in-class reasoning for trading decisions |
| **LLM Fallback** | **GPT-4 Turbo** | Latest | Automatic failover when Claude unavailable |
| MCP SDK | `modelcontextprotocol/go-sdk` | Latest | Official SDK, maintained by Anthropic + Google |
| **Exchange API** | **CCXT** | Latest | Unified API for 100+ exchanges |
| **Technical Indicators** | **cinar/indicator** | Latest | 60+ indicators in pure Go |
| Database | PostgreSQL | 15+ | Reliable, ACID compliant |
| Time-Series DB | **TimescaleDB** | 2.11+ | Extension for PostgreSQL, 1000x faster time-series queries |
| **Vector Search** | **pgvector** | Latest | PostgreSQL extension for semantic search |
| Cache | Redis | 7+ | Session state, pub/sub, fast lookups |
| Message Queue | NATS | 2.10+ | Lightweight, Go-native, high performance |
| Container | Docker | 24+ | Consistent deployment |
| Orchestration | Docker Compose / Kubernetes | - | Local dev / Production |
| Metrics | Prometheus | Latest | Industry standard metrics |
| Visualization | Grafana | Latest | Dashboard and alerting |

### 4.2 External Services

| Service | Purpose | Provider |
|---------|---------|----------|
| **LLM Providers** | **Agent reasoning** | **Anthropic (Claude), OpenAI (GPT-4), Google (Gemini)** |
| Exchange API | Real-time trading via CCXT | Binance, Coinbase, 100+ exchanges |
| Market Data | Historical data via CCXT | Multiple exchanges |
| News API | Sentiment analysis (future) | NewsAPI, CryptoPanic |

### 4.3 Go Dependencies

```go
// LLM (via Bifrost - OpenAI-compatible API)
// No SDK needed, just HTTP client

// Core MCP
github.com/modelcontextprotocol/go-sdk

// Exchange & Trading
github.com/ccxt/ccxt/go                    // NEW: Unified exchange API
github.com/cinar/indicator                  // NEW: Technical indicators

// Web & API
github.com/gin-gonic/gin
github.com/gorilla/websocket

// Database
github.com/jackc/pgx/v5
github.com/redis/go-redis/v9

// Message Queue
github.com/nats-io/nats.go

// Exchange APIs (Fallback)
github.com/adshao/go-binance/v2
github.com/preichenberger/go-coinbasepro/v2

// Metrics
github.com/prometheus/client_golang

// Configuration
github.com/spf13/viper

// Logging
github.com/rs/zerolog
```

---

## 5. System Components

### 5.1 MCP Servers (Tools & Resources)

#### 5.1.1 Market Data Server

**Purpose**: Provide real-time and historical market data

**Tools**:
- `get_current_price`: Get current ticker price
- `get_candlesticks`: Get OHLCV candlestick data
- `get_historical_data`: Fetch historical price data

**Resources**:
- `market://ticker/{symbol}`: Real-time ticker data
- `market://candlesticks/{symbol}/{interval}`: Candlestick stream
- `market://orderbook/{symbol}`: Order book depth
- `market://trades/{symbol}`: Recent trades

**Implementation**: `/cmd/mcp-servers/market-data/`

#### 5.1.2 Technical Indicators Server

**Purpose**: Calculate technical indicators

**Tools**:
- `calculate_rsi`: RSI (Relative Strength Index)
- `calculate_macd`: MACD (Moving Average Convergence Divergence)
- `calculate_bollinger`: Bollinger Bands
- `calculate_ema`: Exponential Moving Average
- `calculate_atr`: Average True Range
- `detect_patterns`: Chart pattern detection

**Implementation**: `/cmd/mcp-servers/technical-indicators/`

#### 5.1.3 Risk Analyzer Server

**Purpose**: Risk calculations and portfolio management

**Tools**:
- `calculate_position_size`: Kelly Criterion-based sizing
- `calculate_var`: Value at Risk
- `calculate_sharpe`: Sharpe ratio
- `check_portfolio_limits`: Validate against limits
- `calculate_drawdown`: Current drawdown

**Resources**:
- `portfolio://positions`: Current positions
- `portfolio://balance`: Account balance
- `portfolio://pnl`: Profit & Loss

**Implementation**: `/cmd/mcp-servers/risk-analyzer/`

#### 5.1.4 Order Executor Server

**Purpose**: Order placement and management

**Tools**:
- `place_market_order`: Place market order
- `place_limit_order`: Place limit order
- `cancel_order`: Cancel pending order
- `get_order_status`: Query order status
- `get_fills`: Get order fills

**Implementation**: `/cmd/mcp-servers/order-executor/`

### 5.2 Agents (MCP Clients)

#### 5.2.1 Analysis Agents

##### Technical Analysis Agent
**Purpose**: Analyze price action using technical indicators

**Connects To**:
- Market Data Server (candlesticks)
- Technical Indicators Server (RSI, MACD, etc.)

**Output**:
```go
type TechnicalSignal struct {
    Symbol     string
    Signal     string  // BUY/SELL/HOLD
    Confidence float64 // 0.0 - 1.0
    Indicators map[string]float64
    Timestamp  time.Time
}
```

**Implementation**: `/cmd/agents/technical-agent/`

##### Order Book Analysis Agent
**Purpose**: Analyze order book depth and liquidity

**Connects To**:
- Market Data Server (order book)

**Output**:
```go
type OrderBookSignal struct {
    Symbol          string
    BidAskImbalance float64
    Liquidity       string // HIGH/MEDIUM/LOW
    Spread          float64
    Signal          string // BUY_PRESSURE/SELL_PRESSURE/NEUTRAL
    Confidence      float64
    Timestamp       time.Time
}
```

**Implementation**: `/cmd/agents/orderbook-agent/`

##### Sentiment Analysis Agent
**Purpose**: Analyze market sentiment from news and social media

**Connects To**:
- News API (external)
- Sentiment analysis tools

**Output**:
```go
type SentimentSignal struct {
    Symbol     string
    Sentiment  float64 // -1.0 (bearish) to +1.0 (bullish)
    Confidence float64
    Sources    []string
    Timestamp  time.Time
}
```

**Implementation**: `/cmd/agents/sentiment-agent/`

#### 5.2.2 Strategy Agents

##### Trend Following Agent
**Purpose**: Identify and trade with trends

**Connects To**:
- Technical Indicators Server
- Market Data Server

**Strategy**:
- EMA crossovers
- ADX for trend strength
- Trailing stop-loss

**Output**:
```go
type StrategyDecision struct {
    Symbol       string
    Action       string  // BUY/SELL/HOLD
    Confidence   float64
    EntryPrice   float64
    StopLoss     float64
    TakeProfit   float64
    PositionSize float64
    Reasoning    string
    Timestamp    time.Time
}
```

**Implementation**: `/cmd/agents/strategy-trend/`

##### Mean Reversion Agent
**Purpose**: Trade oversold/overbought conditions

**Connects To**:
- Technical Indicators Server (RSI, Bollinger)
- Market Data Server

**Strategy**:
- Bollinger Band bounces
- RSI extremes
- Quick exits

**Output**: Same as `StrategyDecision`

**Implementation**: `/cmd/agents/strategy-reversion/`

##### Arbitrage Agent
**Purpose**: Identify cross-exchange arbitrage opportunities

**Connects To**:
- Market Data Server (multiple exchanges)

**Strategy**:
- Cross-exchange price differences
- Fee-adjusted profits
- Fast execution

**Output**: Same as `StrategyDecision`

**Implementation**: `/cmd/agents/strategy-arbitrage/`

#### 5.2.3 Risk Management Agent

**Purpose**: Final risk approval and position sizing

**Connects To**:
- Risk Analyzer Server
- Market Data Server

**Responsibilities**:
- Validate portfolio limits
- Calculate optimal position size
- Check drawdown limits
- Approve/reject trades
- Set dynamic stop-loss

**Output**:
```go
type RiskApproval struct {
    Approved      bool
    PositionSize  float64
    StopLoss      float64
    MaxLoss       float64
    RiskPercent   float64
    Reason        string
    Timestamp     time.Time
}
```

**Veto Power**: Can reject any trade

**Implementation**: `/cmd/agents/risk-agent/`

### 5.3 MCP Orchestrator

**Purpose**: Central coordination hub for all agents

**Responsibilities**:
1. **Agent Management**
   - Start/stop agents
   - Health monitoring
   - Connection pool management

2. **Coordination Patterns**
   - Sequential: Agent1 → Agent2 → Agent3
   - Concurrent: All agents run in parallel
   - Event-Driven: Triggered by market events

3. **Consensus Mechanism**
   - Weighted voting from strategy agents
   - Risk agent has veto power
   - Configurable thresholds

4. **Session Management**
   - Trading session lifecycle
   - Context sharing via Redis
   - State persistence

5. **Decision Pipeline**
   ```
   Market Event
      ↓
   Analysis Agents (parallel)
      ↓
   Strategy Agents (parallel)
      ↓
   Voting & Consensus
      ↓
   Risk Agent (approval)
      ↓
   Execution (if approved)
   ```

**Implementation**: `/cmd/orchestrator/`

### 5.4 Supporting Services

#### 5.4.1 API Gateway

**Endpoints**:
```
GET  /api/v1/status              - System status
GET  /api/v1/agents              - List agents
GET  /api/v1/positions           - Current positions
GET  /api/v1/orders              - Order history
POST /api/v1/trade/start         - Start trading
POST /api/v1/trade/stop          - Stop trading
POST /api/v1/config              - Update configuration
GET  /api/v1/metrics             - Prometheus metrics
WS   /api/v1/ws                  - WebSocket for real-time updates
```

**Implementation**: `/cmd/api/`

#### 5.4.2 Database Schema

**PostgreSQL Tables**:

```sql
-- Market data
CREATE TABLE candlesticks (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    interval VARCHAR(10) NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    open DECIMAL(20, 8),
    high DECIMAL(20, 8),
    low DECIMAL(20, 8),
    close DECIMAL(20, 8),
    volume DECIMAL(20, 8),
    close_time TIMESTAMPTZ
);

-- Convert to TimescaleDB hypertable
SELECT create_hypertable('candlesticks', 'open_time');

-- Orders
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(100) UNIQUE NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    type VARCHAR(20) NOT NULL,
    quantity DECIMAL(20, 8),
    price DECIMAL(20, 8),
    status VARCHAR(20),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Trades
CREATE TABLE trades (
    id BIGSERIAL PRIMARY KEY,
    trade_id VARCHAR(100) UNIQUE NOT NULL,
    order_id VARCHAR(100) REFERENCES orders(order_id),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    quantity DECIMAL(20, 8),
    price DECIMAL(20, 8),
    commission DECIMAL(20, 8),
    pnl DECIMAL(20, 8),
    executed_at TIMESTAMPTZ DEFAULT NOW()
);

-- Positions
CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    quantity DECIMAL(20, 8),
    entry_price DECIMAL(20, 8),
    current_price DECIMAL(20, 8),
    unrealized_pnl DECIMAL(20, 8),
    realized_pnl DECIMAL(20, 8),
    opened_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Agent signals
CREATE TABLE agent_signals (
    id BIGSERIAL PRIMARY KEY,
    agent_name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    signal VARCHAR(20) NOT NULL,
    confidence DECIMAL(5, 4),
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Sessions
CREATE TABLE trading_sessions (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID UNIQUE NOT NULL,
    status VARCHAR(20) NOT NULL,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    metadata JSONB
);
```

**Redis Data Structures**:

```
# Session state
session:{session_id}:state -> hash
session:{session_id}:agents -> set
session:{session_id}:signals -> list

# Agent status
agent:{agent_name}:status -> string
agent:{agent_name}:lastping -> timestamp

# Real-time cache
market:{symbol}:price -> string
market:{symbol}:orderbook -> sorted set
```

---

## 6. Multi-Agent Orchestration

### 6.1 Coordination Patterns

#### 6.1.1 Sequential Pattern
```
Market Data
    ↓
Technical Agent
    ↓
Sentiment Agent
    ↓
Order Book Agent
    ↓
Strategy Agent
    ↓
Risk Agent
    ↓
Execution
```

**Pros**: Simple, predictable, easy to debug
**Cons**: Slower, single point of failure
**Use Case**: Conservative trading, backtesting

#### 6.1.2 Concurrent Pattern (Recommended)
```
                Market Data
                     ↓
        ┌────────────┼────────────┐
        ↓            ↓            ↓
   Technical    Sentiment    Order Book
     Agent        Agent         Agent
        ↓            ↓            ↓
        └────────────┼────────────┘
                     ↓
              Aggregate Signals
                     ↓
        ┌────────────┼────────────┐
        ↓            ↓            ↓
    Trend        Reversion    Arbitrage
   Strategy      Strategy      Strategy
        ↓            ↓            ↓
        └────────────┼────────────┘
                     ↓
             Voting/Consensus
                     ↓
              Risk Agent
                     ↓
              Execution
```

**Pros**: Fast, parallel processing, resilient
**Cons**: More complex coordination
**Use Case**: Real-time trading

#### 6.1.3 Event-Driven Pattern
```
Market Event (NATS)
    ↓
Orchestrator receives event
    ↓
Publishes to agent topics
    ↓
Agents process in parallel
    ↓
Publish results to decision topic
    ↓
Orchestrator aggregates
    ↓
Risk approval
    ↓
Execution event
```

**Pros**: Most scalable, decoupled, fault-tolerant
**Cons**: Most complex, eventual consistency
**Use Case**: High-frequency trading, multiple symbols

### 6.2 Voting & Consensus

#### 6.2.1 Weighted Voting

Each strategy agent votes with:
- **Action**: BUY/SELL/HOLD
- **Confidence**: 0.0 - 1.0
- **Weight**: Configured per agent

**Formula**:
```
BUY_Score = Σ(confidence_i × weight_i) for all BUY votes
SELL_Score = Σ(confidence_i × weight_i) for all SELL votes
HOLD_Score = Σ(confidence_i × weight_i) for all HOLD votes

Final_Decision = max(BUY_Score, SELL_Score, HOLD_Score)
```

**Example**:
```go
type Vote struct {
    AgentName  string
    Action     string  // BUY/SELL/HOLD
    Confidence float64 // 0.0 - 1.0
    Weight     float64
}

votes := []Vote{
    {AgentName: "trend", Action: "BUY", Confidence: 0.8, Weight: 1.2},
    {AgentName: "reversion", Action: "HOLD", Confidence: 0.6, Weight: 1.0},
    {AgentName: "arbitrage", Action: "BUY", Confidence: 0.9, Weight: 0.8},
}

// BUY score = (0.8 × 1.2) + (0.9 × 0.8) = 0.96 + 0.72 = 1.68
// HOLD score = (0.6 × 1.0) = 0.6
// Result: BUY with score 1.68
```

#### 6.2.2 Consensus Threshold

**Configuration**:
```yaml
consensus:
  min_threshold: 0.7        # Minimum score to proceed
  min_agents: 2             # Minimum agents agreeing
  risk_veto: true           # Risk agent can veto
  min_confidence: 0.6       # Per-agent minimum confidence
```

**Logic**:
```go
if finalScore < minThreshold {
    return HOLD // Not enough confidence
}

if agentsAgreeing < minAgents {
    return HOLD // Not enough consensus
}

// Send to risk agent for approval
riskApproval := riskAgent.Approve(decision)
if !riskApproval.Approved {
    return HOLD // Risk agent vetoed
}

return EXECUTE
```

### 6.3 Context Sharing

#### 6.3.1 Session Context

Each trading session has shared context:

```go
type SessionContext struct {
    SessionID      string
    Symbol         string
    StartTime      time.Time
    MarketData     *MarketData
    AnalysisSignals map[string]*Signal
    Decisions      []*Decision
    ActivePosition *Position
    Metadata       map[string]interface{}
}
```

Stored in Redis, accessible to all agents.

#### 6.3.2 MCP Resources

Agents access shared data via MCP resources:

```
orchestrator://session/{session_id}
orchestrator://context/market_data
orchestrator://context/signals
orchestrator://context/position
```

---

## 7. Data Flow

### 7.1 Real-Time Trading Flow

```
1. Market Data Ingestion
   - WebSocket connection to Binance
   - Receive ticker, candlestick, order book updates
   - Store in Redis cache
   - Publish to NATS topic: market.{symbol}.update

2. Orchestrator Receives Event
   - Subscribe to NATS: market.*.update
   - Create new trading session
   - Initialize session context in Redis

3. Analysis Phase (Parallel)
   - Orchestrator spawns analysis agents
   - Each agent (MCP client) connects to servers:
     * Technical Agent → Market Data + Tech Indicators
     * Order Book Agent → Market Data
     * Sentiment Agent → News API
   - Agents invoke MCP tools and read resources
   - Agents publish signals to orchestrator

4. Strategy Phase (Parallel)
   - Orchestrator aggregates analysis signals
   - Updates session context
   - Spawns strategy agents
   - Each strategy agent:
     * Reads session context (MCP resource)
     * Invokes indicator tools
     * Generates decision with confidence
   - Agents publish decisions to orchestrator

5. Consensus Phase
   - Orchestrator collects all strategy votes
   - Applies weighted voting formula
   - Checks consensus threshold
   - Determines final decision

6. Risk Approval Phase
   - Send decision to Risk Agent
   - Risk Agent:
     * Checks portfolio limits via Risk Analyzer
     * Calculates position size (Kelly Criterion)
     * Validates stop-loss levels
     * Approves or vetoes trade
   - Return approval to orchestrator

7. Execution Phase
   - If approved:
     * Orchestrator sends to Order Executor
     * Executor places order via exchange API
     * Monitors order status
     * Updates position in database
     * Publishes trade event to NATS
   - If not approved:
     * Log reason
     * Update session status to SKIPPED

8. Monitoring Phase
   - Track position in real-time
   - Monitor stop-loss and take-profit
   - Update unrealized P&L
   - Send WebSocket updates to clients

9. Exit Flow
   - Triggered by:
     * Stop-loss hit
     * Take-profit hit
     * Strategy agent signals exit
     * Manual close via API
   - Follow same approval flow
   - Execute closing order
   - Calculate realized P&L
   - Update database
```

### 7.2 Backtesting Flow

```
1. Load Historical Data
   - Query candlesticks from TimescaleDB
   - Load into memory or streaming mode

2. Initialize Backtest Session
   - Create mock orchestrator
   - Initialize agents in replay mode
   - Set starting capital

3. Time-Step Simulation
   For each candlestick:
     - Update market data context
     - Run analysis agents
     - Run strategy agents
     - Apply consensus logic
     - Risk approval
     - Simulate order execution
     - Update paper positions
     - Calculate P&L
     - Record metrics

4. Generate Report
   - Total return
   - Sharpe ratio
   - Maximum drawdown
   - Win rate
   - Average profit/loss
   - Trade distribution
   - Agent performance breakdown

5. Optimization (Optional)
   - Grid search parameters
   - Genetic algorithm
   - Re-run backtest with new params
```

---

## 8. Code Examples

### 8.1 MCP Server: Technical Indicators

```go
// cmd/mcp-servers/technical-indicators/main.go
package main

import (
    "context"
    "log"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "cryptofunk/internal/indicators"
)

// Input/Output types with JSON schema tags
type RSIInput struct {
    Symbol string   `json:"symbol" jsonschema:"required,description=Trading pair symbol"`
    Period int      `json:"period" jsonschema:"description=RSI period (default 14)"`
    Prices []float64 `json:"prices" jsonschema:"required,description=Price array for calculation"`
}

type RSIOutput struct {
    Value  float64 `json:"value" jsonschema:"description=RSI value (0-100)"`
    Signal string  `json:"signal" jsonschema:"description=Signal: oversold|overbought|neutral"`
}

func calculateRSI(ctx context.Context, req *mcp.CallToolRequest, input RSIInput) (
    *mcp.CallToolResult, RSIOutput, error,
) {
    if input.Period == 0 {
        input.Period = 14 // default
    }

    // Calculate RSI
    rsiValue := indicators.CalculateRSI(input.Prices, input.Period)

    // Determine signal
    signal := "neutral"
    if rsiValue > 70 {
        signal = "overbought"
    } else if rsiValue < 30 {
        signal = "oversold"
    }

    return nil, RSIOutput{
        Value:  rsiValue,
        Signal: signal,
    }, nil
}

type MACDInput struct {
    Symbol     string    `json:"symbol" jsonschema:"required"`
    Prices     []float64 `json:"prices" jsonschema:"required"`
    FastPeriod int       `json:"fast_period" jsonschema:"description=Fast EMA period (default 12)"`
    SlowPeriod int       `json:"slow_period" jsonschema:"description=Slow EMA period (default 26)"`
    SignalPeriod int     `json:"signal_period" jsonschema:"description=Signal period (default 9)"`
}

type MACDOutput struct {
    MACD      float64 `json:"macd"`
    Signal    float64 `json:"signal"`
    Histogram float64 `json:"histogram"`
    CrossOver string  `json:"crossover"` // bullish|bearish|none
}

func calculateMACD(ctx context.Context, req *mcp.CallToolRequest, input MACDInput) (
    *mcp.CallToolResult, MACDOutput, error,
) {
    if input.FastPeriod == 0 {
        input.FastPeriod = 12
    }
    if input.SlowPeriod == 0 {
        input.SlowPeriod = 26
    }
    if input.SignalPeriod == 0 {
        input.SignalPeriod = 9
    }

    macd, signal, histogram := indicators.CalculateMACD(
        input.Prices,
        input.FastPeriod,
        input.SlowPeriod,
        input.SignalPeriod,
    )

    crossover := "none"
    if histogram > 0 && histogram > signal {
        crossover = "bullish"
    } else if histogram < 0 && histogram < signal {
        crossover = "bearish"
    }

    return nil, MACDOutput{
        MACD:      macd,
        Signal:    signal,
        Histogram: histogram,
        CrossOver: crossover,
    }, nil
}

func main() {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "technical-indicators",
        Version: "v1.0.0",
    }, nil)

    // Add RSI tool
    mcp.AddTool(server, &mcp.Tool{
        Name:        "calculate_rsi",
        Description: "Calculate Relative Strength Index (RSI) indicator",
    }, calculateRSI)

    // Add MACD tool
    mcp.AddTool(server, &mcp.Tool{
        Name:        "calculate_macd",
        Description: "Calculate MACD (Moving Average Convergence Divergence) indicator",
    }, calculateMACD)

    // Run server over stdio
    transport := mcp.NewStdioTransport()
    if err := server.Run(transport); err != nil {
        log.Fatal(err)
    }
}
```

### 8.2 MCP Client: Technical Analysis Agent

```go
// cmd/agents/technical-agent/main.go
package main

import (
    "context"
    "encoding/json"
    "log"
    "time"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "cryptofunk/internal/models"
)

type TechnicalAgent struct {
    indicatorClient *mcp.Client
    marketClient    *mcp.Client
}

func (a *TechnicalAgent) Analyze(ctx context.Context, symbol string) (*models.Signal, error) {
    // 1. Get price data from market data server
    pricesResp, err := a.marketClient.CallTool(ctx, &mcp.CallToolRequest{
        Params: mcp.CallToolRequestParams{
            Name: "get_candlesticks",
            Arguments: map[string]any{
                "symbol":   symbol,
                "interval": "1h",
                "limit":    100,
            },
        },
    })
    if err != nil {
        return nil, err
    }

    // Extract prices
    var prices []float64
    // ... parse response and extract close prices

    // 2. Calculate RSI via indicator server
    rsiResp, err := a.indicatorClient.CallTool(ctx, &mcp.CallToolRequest{
        Params: mcp.CallToolRequestParams{
            Name: "calculate_rsi",
            Arguments: map[string]any{
                "symbol": symbol,
                "period": 14,
                "prices": prices,
            },
        },
    })
    if err != nil {
        return nil, err
    }

    var rsiResult struct {
        Value  float64 `json:"value"`
        Signal string  `json:"signal"`
    }
    json.Unmarshal(rsiResp.Content[0].Text, &rsiResult)

    // 3. Calculate MACD via indicator server
    macdResp, err := a.indicatorClient.CallTool(ctx, &mcp.CallToolRequest{
        Params: mcp.CallToolRequestParams{
            Name: "calculate_macd",
            Arguments: map[string]any{
                "symbol": symbol,
                "prices": prices,
            },
        },
    })
    if err != nil {
        return nil, err
    }

    var macdResult struct {
        MACD      float64 `json:"macd"`
        Signal    float64 `json:"signal"`
        Histogram float64 `json:"histogram"`
        CrossOver string  `json:"crossover"`
    }
    json.Unmarshal(macdResp.Content[0].Text, &macdResult)

    // 4. Aggregate signals
    signal := a.aggregateSignals(rsiResult, macdResult)

    return &models.Signal{
        AgentName:  "technical-analysis",
        Symbol:     symbol,
        Signal:     signal.Action,
        Confidence: signal.Confidence,
        Indicators: map[string]float64{
            "rsi":       rsiResult.Value,
            "macd":      macdResult.MACD,
            "histogram": macdResult.Histogram,
        },
        Timestamp: time.Now(),
    }, nil
}

func (a *TechnicalAgent) aggregateSignals(rsi, macd interface{}) models.AgentDecision {
    // Simple aggregation logic
    // In production, this would be more sophisticated

    var score float64
    var action string

    // RSI oversold + MACD bullish = strong BUY
    // RSI overbought + MACD bearish = strong SELL
    // etc.

    // ... implementation

    return models.AgentDecision{
        Action:     action,
        Confidence: score,
    }
}

func NewTechnicalAgent() (*TechnicalAgent, error) {
    // Create client for indicator server
    indicatorClient := mcp.NewClient(nil)
    indicatorTransport := mcp.NewCommandTransport("./bin/technical-indicators-server")
    if err := indicatorClient.Run(indicatorTransport); err != nil {
        return nil, err
    }

    // Create client for market data server
    marketClient := mcp.NewClient(nil)
    marketTransport := mcp.NewCommandTransport("./bin/market-data-server")
    if err := marketClient.Run(marketTransport); err != nil {
        return nil, err
    }

    return &TechnicalAgent{
        indicatorClient: indicatorClient,
        marketClient:    marketClient,
    }, nil
}

func main() {
    agent, err := NewTechnicalAgent()
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Main loop: wait for orchestrator requests
    // In production, this would be event-driven via NATS
    for {
        signal, err := agent.Analyze(ctx, "BTCUSDT")
        if err != nil {
            log.Printf("Analysis error: %v", err)
            continue
        }

        log.Printf("Signal: %+v", signal)

        // Publish to orchestrator
        // ... publish signal via NATS or MCP

        time.Sleep(60 * time.Second)
    }
}
```

### 8.3 Orchestrator: Voting Logic

```go
// internal/orchestrator/voting.go
package orchestrator

import (
    "fmt"
    "cryptofunk/internal/models"
)

type VotingConfig struct {
    MinThreshold  float64
    MinAgents     int
    RiskVeto      bool
    MinConfidence float64
}

type Voter struct {
    config VotingConfig
}

func (v *Voter) Aggregate(votes []models.Vote) (*models.Decision, error) {
    if len(votes) == 0 {
        return nil, fmt.Errorf("no votes received")
    }

    // Calculate weighted scores for each action
    scores := make(map[string]float64)
    counts := make(map[string]int)

    for _, vote := range votes {
        // Skip low confidence votes
        if vote.Confidence < v.config.MinConfidence {
            continue
        }

        weightedScore := vote.Confidence * vote.Weight
        scores[vote.Action] += weightedScore
        counts[vote.Action]++
    }

    // Find action with highest score
    var maxAction string
    var maxScore float64

    for action, score := range scores {
        if score > maxScore {
            maxScore = score
            maxAction = action
        }
    }

    // Check threshold
    if maxScore < v.config.MinThreshold {
        return &models.Decision{
            Action:  "HOLD",
            Score:   maxScore,
            Reason:  "Score below threshold",
            Proceed: false,
        }, nil
    }

    // Check minimum agents
    if counts[maxAction] < v.config.MinAgents {
        return &models.Decision{
            Action:  "HOLD",
            Score:   maxScore,
            Reason:  "Not enough agents agree",
            Proceed: false,
        }, nil
    }

    return &models.Decision{
        Action:  maxAction,
        Score:   maxScore,
        Votes:   votes,
        Proceed: true,
    }, nil
}

func (v *Voter) ApplyRiskApproval(decision *models.Decision, approval *models.RiskApproval) *models.Decision {
    if !v.config.RiskVeto {
        return decision
    }

    if !approval.Approved {
        decision.Proceed = false
        decision.Reason = fmt.Sprintf("Risk veto: %s", approval.Reason)
    } else {
        decision.PositionSize = approval.PositionSize
        decision.StopLoss = approval.StopLoss
        decision.MaxLoss = approval.MaxLoss
    }

    return decision
}
```

---

## 9. Configuration

### 9.1 Main Configuration File

```yaml
# configs/config.yaml

# Application
app:
  name: "cryptofunk"
  version: "1.0.0"
  environment: "development" # development | staging | production
  log_level: "info" # debug | info | warn | error

# Trading
trading:
  mode: "paper" # paper | live
  default_symbol: "BTCUSDT"
  symbols:
    - "BTCUSDT"
    - "ETHUSDT"
  update_interval: "1m"
  max_concurrent_sessions: 5

# Exchange
exchange:
  binance:
    api_key: "${BINANCE_API_KEY}"
    secret_key: "${BINANCE_SECRET_KEY}"
    testnet: true
    base_url: "https://testnet.binance.vision"

  coinbase:
    api_key: "${COINBASE_API_KEY}"
    secret_key: "${COINBASE_SECRET_KEY}"
    passphrase: "${COINBASE_PASSPHRASE}"

# Database
database:
  postgres:
    host: "localhost"
    port: 5432
    database: "cryptofunk"
    user: "postgres"
    password: "${POSTGRES_PASSWORD}"
    ssl_mode: "disable"
    max_connections: 20

  redis:
    host: "localhost"
    port: 6379
    password: "${REDIS_PASSWORD}"
    db: 0
    max_retries: 3

# Message Queue
nats:
  url: "nats://localhost:4222"
  cluster_id: "cryptofunk"
  client_id: "orchestrator"

# MCP Servers
mcp_servers:
  market_data:
    command: "./bin/market-data-server"
    transport: "stdio"
    enabled: true

  technical_indicators:
    command: "./bin/technical-indicators-server"
    transport: "stdio"
    enabled: true

  risk_analyzer:
    command: "./bin/risk-analyzer-server"
    transport: "stdio"
    enabled: true

  order_executor:
    command: "./bin/order-executor-server"
    transport: "stdio"
    enabled: true

# Agents
agents:
  technical:
    name: "technical-analysis"
    command: "./bin/technical-agent"
    weight: 1.0
    enabled: true

  orderbook:
    name: "orderbook-analysis"
    command: "./bin/orderbook-agent"
    weight: 1.0
    enabled: true

  sentiment:
    name: "sentiment-analysis"
    command: "./bin/sentiment-agent"
    weight: 0.8
    enabled: false # disabled initially

  trend_strategy:
    name: "trend-following"
    command: "./bin/trend-agent"
    weight: 1.2
    enabled: true

  reversion_strategy:
    name: "mean-reversion"
    command: "./bin/reversion-agent"
    weight: 1.0
    enabled: true

  risk:
    name: "risk-management"
    command: "./bin/risk-agent"
    veto_power: true
    enabled: true

# Orchestrator
orchestrator:
  pattern: "concurrent" # sequential | concurrent | event_driven
  session_ttl: "300s"
  max_concurrent_sessions: 10
  health_check_interval: "30s"

  # Voting configuration
  voting:
    min_threshold: 0.7
    min_agents: 2
    risk_veto: true
    min_confidence: 0.6

# Risk Management
risk:
  max_position_size_percent: 10 # % of portfolio
  max_portfolio_risk_percent: 2 # % of portfolio per trade
  max_drawdown_percent: 20
  max_daily_trades: 50
  max_open_positions: 5

  circuit_breakers:
    max_drawdown_percent: 15
    max_orders_per_minute: 10
    volatility_threshold: 5.0

  position_sizing:
    method: "kelly" # kelly | fixed | percent
    kelly_fraction: 0.5
    max_leverage: 1.0

# API
api:
  host: "0.0.0.0"
  port: 8080
  cors_enabled: true
  rate_limit:
    enabled: true
    requests_per_minute: 100

  websocket:
    enabled: true
    path: "/ws"
    ping_interval: "30s"

# Monitoring
monitoring:
  prometheus:
    enabled: true
    port: 9090
    path: "/metrics"

  grafana:
    enabled: true
    port: 3000

# Logging
logging:
  format: "json" # json | console
  level: "info"
  output: "stdout" # stdout | file
  file_path: "/var/log/cryptofunk/app.log"
```

### 9.2 Environment Variables

```bash
# .env

# Exchange API Keys
BINANCE_API_KEY=your_binance_api_key
BINANCE_SECRET_KEY=your_binance_secret_key
COINBASE_API_KEY=your_coinbase_api_key
COINBASE_SECRET_KEY=your_coinbase_secret_key
COINBASE_PASSPHRASE=your_coinbase_passphrase

# Database
POSTGRES_PASSWORD=your_postgres_password
REDIS_PASSWORD=your_redis_password

# External APIs
NEWS_API_KEY=your_news_api_key

# Application
APP_ENV=development
LOG_LEVEL=info
```

---

## 10. Deployment Strategy

### 10.1 Local Development (Docker Compose)

```yaml
# docker-compose.yml
version: '3.9'

services:
  postgres:
    image: timescale/timescaledb:latest-pg15
    environment:
      POSTGRES_DB: cryptofunk
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  nats:
    image: nats:latest
    ports:
      - "4222:4222"
      - "8222:8222"

  prometheus:
    image: prom/prometheus:latest
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
    ports:
      - "9090:9090"
    volumes:
      - ./deployments/prometheus:/etc/prometheus
      - prometheus_data:/prometheus

  grafana:
    image: grafana/grafana:latest
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD}
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./deployments/grafana/dashboards:/etc/grafana/provisioning/dashboards

  orchestrator:
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile.orchestrator
    depends_on:
      - postgres
      - redis
      - nats
    environment:
      - APP_ENV=development
      - DATABASE_HOST=postgres
      - REDIS_HOST=redis
      - NATS_URL=nats://nats:4222
    volumes:
      - ./configs:/app/configs
    ports:
      - "8080:8080"

volumes:
  postgres_data:
  redis_data:
  prometheus_data:
  grafana_data:
```

### 10.2 Production (Kubernetes)

```yaml
# deployments/k8s/orchestrator-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cryptofunk-orchestrator
  namespace: cryptofunk
spec:
  replicas: 2
  selector:
    matchLabels:
      app: orchestrator
  template:
    metadata:
      labels:
        app: orchestrator
    spec:
      containers:
      - name: orchestrator
        image: cryptofunk/orchestrator:latest
        ports:
        - containerPort: 8080
        env:
        - name: APP_ENV
          value: "production"
        - name: DATABASE_HOST
          valueFrom:
            configMapKeyRef:
              name: cryptofunk-config
              key: database_host
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: cryptofunk-secrets
              key: postgres_password
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5

---
apiVersion: v1
kind: Service
metadata:
  name: orchestrator-service
  namespace: cryptofunk
spec:
  selector:
    app: orchestrator
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

### 10.3 CI/CD Pipeline (GitHub Actions)

```yaml
# .github/workflows/ci.yml
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Run tests
      run: |
        go test -v -race -coverprofile=coverage.out ./...
        go tool cover -html=coverage.out -o coverage.html

    - name: Upload coverage
      uses: codecov/codecov-action@v3

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v2

    - name: Login to DockerHub
      uses: docker/login-action@v2
      with:
        username: ${{ secrets.DOCKERHUB_USERNAME }}
        password: ${{ secrets.DOCKERHUB_TOKEN }}

    - name: Build and push
      uses: docker/build-push-action@v4
      with:
        context: .
        push: true
        tags: cryptofunk/orchestrator:${{ github.sha }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
    - name: Deploy to Kubernetes
      uses: azure/k8s-deploy@v4
      with:
        manifests: |
          deployments/k8s/
        images: |
          cryptofunk/orchestrator:${{ github.sha }}
```

---

## 11. Testing Strategy

### 11.1 Unit Tests

```go
// internal/indicators/rsi_test.go
package indicators

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCalculateRSI(t *testing.T) {
    prices := []float64{
        44.34, 44.09, 43.61, 44.33, 44.83,
        45.10, 45.42, 45.84, 46.08, 45.89,
        46.03, 45.61, 46.28, 46.28, 46.00,
    }

    rsi := CalculateRSI(prices, 14)

    // Expected RSI value (approximate)
    assert.InDelta(t, 70.0, rsi, 5.0)
}
```

### 11.2 Integration Tests

```go
// internal/orchestrator/orchestrator_test.go
package orchestrator

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestOrchestratorVoting(t *testing.T) {
    // Setup
    orchestrator := NewOrchestrator(testConfig)

    // Mock agent votes
    votes := []Vote{
        {AgentName: "trend", Action: "BUY", Confidence: 0.8, Weight: 1.2},
        {AgentName: "reversion", Action: "HOLD", Confidence: 0.6, Weight: 1.0},
        {AgentName: "arbitrage", Action: "BUY", Confidence: 0.9, Weight: 0.8},
    }

    // Execute
    decision, err := orchestrator.ProcessVotes(context.Background(), votes)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "BUY", decision.Action)
    assert.True(t, decision.Proceed)
}
```

### 11.3 Backtesting Tests

```go
// pkg/backtest/backtest_test.go
package backtest

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestBacktestTrendStrategy(t *testing.T) {
    // Load historical data
    data := loadHistoricalData("BTCUSDT",
        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
        time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

    // Run backtest
    engine := NewBacktestEngine(testConfig)
    results := engine.Run(data, &TrendStrategy{})

    // Assert
    assert.Greater(t, results.TotalReturn, 0.0)
    assert.Greater(t, results.SharpeRatio, 1.0)
    assert.Less(t, results.MaxDrawdown, 0.20)
}
```

### 11.4 E2E Tests

```go
// test/e2e/trading_flow_test.go
package e2e

import (
    "testing"
    "time"
)

func TestFullTradingFlow(t *testing.T) {
    // 1. Start orchestrator
    // 2. Start all agents
    // 3. Inject market data
    // 4. Wait for signal
    // 5. Verify order placed
    // 6. Verify position updated
    // 7. Verify database records
}
```

---

## 12. Risk Management

### 12.1 Risk Controls

#### Position Level
- Max position size: 10% of portfolio
- Max risk per trade: 2% of portfolio
- Stop-loss required for every position
- Take-profit recommended

#### Portfolio Level
- Max open positions: 5
- Max drawdown: 20%
- Max daily trades: 50
- Diversification requirements

#### System Level
- Circuit breakers
- Rate limiting
- Emergency kill switch
- Manual override capability

### 12.2 Circuit Breakers

```go
type CircuitBreaker struct {
    maxDrawdown        float64
    maxOrdersPerMinute int
    volatilityThreshold float64

    currentDrawdown    float64
    ordersLastMinute   int
    currentVolatility  float64

    tripped bool
}

func (cb *CircuitBreaker) Check() error {
    if cb.currentDrawdown > cb.maxDrawdown {
        cb.tripped = true
        return fmt.Errorf("circuit breaker: max drawdown exceeded")
    }

    if cb.ordersLastMinute > cb.maxOrdersPerMinute {
        cb.tripped = true
        return fmt.Errorf("circuit breaker: order rate limit exceeded")
    }

    if cb.currentVolatility > cb.volatilityThreshold {
        cb.tripped = true
        return fmt.Errorf("circuit breaker: high volatility detected")
    }

    return nil
}
```

### 12.3 Position Sizing

#### Kelly Criterion
```
f* = (bp - q) / b

Where:
f* = fraction of capital to wager
b = odds received (reward/risk ratio)
p = probability of winning
q = probability of losing (1 - p)
```

Implementation:
```go
func CalculateKellyPositionSize(
    winRate float64,
    avgWin float64,
    avgLoss float64,
    capital float64,
    kellyFraction float64,
) float64 {
    b := avgWin / avgLoss
    p := winRate
    q := 1 - p

    kelly := (b*p - q) / b

    // Apply fractional Kelly for safety
    adjustedKelly := kelly * kellyFraction

    // Ensure positive and capped
    if adjustedKelly < 0 {
        adjustedKelly = 0
    }
    if adjustedKelly > 0.25 { // Max 25% of capital
        adjustedKelly = 0.25
    }

    return capital * adjustedKelly
}
```

---

## 13. Monitoring & Observability

### 13.1 Prometheus Metrics

```go
// Orchestrator metrics
var (
    sessionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "cryptofunk_sessions_active",
        Help: "Number of active trading sessions",
    })

    decisionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "cryptofunk_decisions_total",
        Help: "Total trading decisions made",
    }, []string{"action", "symbol"})

    agentLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name: "cryptofunk_agent_latency_seconds",
        Help: "Agent processing latency",
    }, []string{"agent"})

    positionPnL = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "cryptofunk_position_pnl",
        Help: "Current position P&L",
    }, []string{"symbol"})
)
```

### 13.2 Grafana Dashboards

**System Overview Dashboard**:
- Active sessions
- Decisions per minute
- Agent health status
- System latency

**Trading Performance Dashboard**:
- Total P&L
- Win rate
- Sharpe ratio
- Drawdown chart
- Equity curve

**Agent Performance Dashboard**:
- Agent accuracy
- Agent latency
- Signal distribution
- Consensus rate

**Risk Dashboard**:
- Current drawdown
- Position sizes
- Risk exposure
- Circuit breaker status

### 13.3 Logging

```go
// Structured logging with zerolog
log.Info().
    Str("agent", "technical").
    Str("symbol", "BTCUSDT").
    Float64("rsi", 72.5).
    Str("signal", "overbought").
    Msg("Technical signal generated")

log.Warn().
    Str("reason", "low consensus").
    Float64("score", 0.55).
    Float64("threshold", 0.70).
    Msg("Trade rejected")

log.Error().
    Err(err).
    Str("exchange", "binance").
    Str("order_id", orderID).
    Msg("Order placement failed")
```

### 13.4 Alerting Rules

```yaml
# Prometheus alerting rules
groups:
- name: cryptofunk
  rules:
  - alert: HighDrawdown
    expr: cryptofunk_drawdown_percent > 15
    for: 5m
    annotations:
      summary: "High drawdown detected"

  - alert: AgentDown
    expr: up{job="agent"} == 0
    for: 1m
    annotations:
      summary: "Agent {{ $labels.agent }} is down"

  - alert: OrderFailureRate
    expr: rate(cryptofunk_orders_failed[5m]) > 0.1
    for: 5m
    annotations:
      summary: "High order failure rate"
```

---

## 14. Future Enhancements

### Phase 11: Advanced Features

1. **Machine Learning Integration**
   - Price prediction models (LSTM, Transformer)
   - Sentiment classification (NLP)
   - Reinforcement learning for strategy optimization
   - Python microservice via gRPC

2. **Multi-Timeframe Analysis**
   - Coordinate agents across 1m, 5m, 15m, 1h, 4h, 1d
   - Hierarchical decision making
   - Long-term trend + short-term execution

3. **Advanced Strategies**
   - Market making
   - Statistical arbitrage
   - Pairs trading
   - Options strategies

4. **Enhanced Risk Management**
   - Portfolio optimization (Markowitz)
   - Dynamic correlation analysis
   - Stress testing
   - Monte Carlo simulation

5. **Social Trading**
   - Copy trading features
   - Strategy marketplace
   - Performance leaderboard

6. **Multi-Exchange Support**
   - Binance, Coinbase, Kraken, FTX
   - Cross-exchange arbitrage
   - Unified order routing

7. **Advanced UI**
   - Real-time trading dashboard
   - Strategy builder (no-code)
   - Backtesting interface
   - Performance analytics

8. **Mobile App**
   - iOS/Android app
   - Push notifications
   - Remote control
   - Portfolio tracking

---

## Appendix A: Glossary

**MCP (Model Context Protocol)**: Standardized protocol for AI agent communication

**Agent**: Independent process that makes specialized decisions

**Orchestrator**: Central coordinator managing multiple agents

**Tool**: Function exposed by MCP server that can be invoked

**Resource**: Data exposed by MCP server that can be read

**Session**: Trading cycle with shared context

**Consensus**: Agreement mechanism among multiple agents

**Circuit Breaker**: Safety mechanism to halt trading under adverse conditions

**Paper Trading**: Simulated trading without real money

**Backtesting**: Testing strategies on historical data

**Sharpe Ratio**: Risk-adjusted return metric

**Drawdown**: Peak-to-trough decline during a period

**Kelly Criterion**: Position sizing formula based on win rate

---

## Appendix B: References

1. **MCP Resources**
   - Official SDK: https://github.com/modelcontextprotocol/go-sdk
   - MCP Specification: https://modelcontextprotocol.io
   - MCP Servers Registry: https://www.pulsemcp.com

2. **Go Resources**
   - Go Documentation: https://go.dev/doc
   - Effective Go: https://go.dev/doc/effective_go

3. **Trading Resources**
   - Binance API: https://binance-docs.github.io/apidocs
   - Technical Analysis Library: https://github.com/sdcoffey/techan

4. **Research Papers**
   - "Advancing Multi-Agent Systems Through Model Context Protocol"
   - "The Kelly Criterion in Blackjack Sports Betting, and the Stock Market"

---

## Document Control

**Version History**:
- v1.0 (2025-10-27): Initial architecture document

**Approvals**:
- [ ] Technical Lead
- [ ] Product Owner
- [ ] Security Review
- [ ] Compliance Review

**Next Review Date**: TBD

---

*End of Architecture Document*
