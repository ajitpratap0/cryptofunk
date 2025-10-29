# CryptoFunk System Architecture

**Version**: 2.0
**Last Updated**: 2025-10-27
**Status**: Updated with CoinGecko MCP Integration

---

## Table of Contents

1. [Overview](#overview)
2. [Hybrid MCP Architecture](#hybrid-mcp-architecture)
3. [System Components](#system-components)
4. [Data Flow](#data-flow)
5. [Technology Stack](#technology-stack)
6. [Security Architecture](#security-architecture)
7. [Deployment Architecture](#deployment-architecture)

---

## Overview

CryptoFunk is an **AI-powered cryptocurrency trading platform** that uses a **multi-agent system orchestrated via the Model Context Protocol (MCP)**. The system leverages both **external MCP servers** (like CoinGecko) and **custom MCP servers** to provide comprehensive trading capabilities.

### Key Architectural Principles

1. **Hybrid MCP Architecture** - Combine external and custom MCP servers
2. **Agent-Based Intelligence** - Specialized agents with distinct responsibilities
3. **LLM-First Approach** - Use Claude/GPT-4 for decision-making (MVP)
4. **Time-Series Optimized** - TimescaleDB for OHLCV data with automatic compression
5. **Distributed Coordination** - NATS for event-driven agent communication
6. **Unified LLM Gateway** - Bifrost for automatic failover across providers
7. **Multi-Exchange Ready** - CCXT for unified exchange APIs

---

## Hybrid MCP Architecture

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    MCP Client Orchestrator                  │
│                  (Central Coordination)                     │
└───────────┬─────────────────────────┬───────────────────────┘
            │                         │
     ┌──────▼─────────┐        ┌─────▼──────────────────┐
     │  External MCP  │        │   Custom MCP Servers   │
     │    Servers     │        │                        │
     └────────────────┘        └────────────────────────┘
            │                         │
            │                    ┌────┴────┬─────────┬──────────┐
            │                    │         │         │          │
            │              ┌─────▼──┐ ┌───▼───┐ ┌──▼────┐ ┌───▼────┐
            │              │ Order  │ │ Risk  │ │ Tech  │ │ Market │
            │              │ Exec   │ │Analyzer│ │Indic  │ │ Data*  │
            │              └────────┘ └───────┘ └───────┘ └────────┘
            │                                              *Optional
    ┌───────▼──────────┐
    │  CoinGecko MCP   │
    │                  │
    │  ✓ Prices        │ ← 76+ Market Data Tools
    │  ✓ Historical    │ ← Multi-exchange aggregation
    │  ✓ Market Charts │ ← Real-time price feeds
    │  ✓ NFT/DeFi Data │ ← No API key required
    │  ✓ Multi-Exchange│
    └──────────────────┘
```

### Division of Responsibilities

#### External MCP Servers

**CoinGecko MCP** (`https://mcp.api.coingecko.com/mcp`)
- **Purpose**: Market intelligence and data aggregation
- **Provides**:
  - Current prices across thousands of cryptocurrencies
  - Historical OHLCV data for backtesting
  - Multi-exchange price aggregation
  - Market trends, rankings, and statistics
  - NFT collection analytics
  - DeFi pools and liquidity data
  - Token on-chain information
- **Benefits**:
  - 76+ pre-built tools (saves 32+ hours of development)
  - No authentication required for basic usage
  - Multi-exchange data aggregation out of the box
  - Maintained and updated by CoinGecko team
- **Rate Limits**: Public tier has rate limits; Pro tier available
- **Connection**: HTTP Streaming transport

#### Custom MCP Servers (Internal)

1. **Order Executor Server**
   - **Purpose**: Trade execution on exchanges
   - **Why Custom**: Needs direct Binance API for actual trading
   - **Tools**:
     - `place_market_order` - Execute market orders
     - `place_limit_order` - Place limit orders
     - `cancel_order` - Cancel pending orders
     - `get_order_status` - Query order state
     - `get_positions` - Current positions
   - **Tech**: CCXT for unified exchange API

2. **Risk Analyzer Server**
   - **Purpose**: Portfolio risk management
   - **Why Custom**: Custom risk rules and calculations
   - **Tools**:
     - `calculate_position_size` - Kelly Criterion sizing
     - `calculate_var` - Value at Risk
     - `check_portfolio_limits` - Limit enforcement
     - `calculate_sharpe` - Sharpe ratio
     - `calculate_drawdown` - Drawdown metrics
   - **Tech**: Custom Go implementation

3. **Technical Indicators Server**
   - **Purpose**: Technical analysis indicators
   - **Why Custom**: Specialized indicator calculations
   - **Tools**:
     - `calculate_rsi` - Relative Strength Index
     - `calculate_macd` - MACD indicator
     - `calculate_bollinger_bands` - Bollinger Bands
     - `calculate_ema` - Exponential Moving Average
     - `calculate_adx` - Average Directional Index
     - `detect_patterns` - Candlestick patterns
   - **Tech**: cinar/indicator library (60+ indicators)

4. **Market Data Server** *(Optional)*
   - **Purpose**: Binance-specific features if needed
   - **Why Optional**: CoinGecko MCP covers most use cases
   - **Use Case**: When you need exchange-specific order book depth
   - **Tech**: go-binance SDK

### Why Hybrid Architecture?

| Aspect | External (CoinGecko) | Custom (Internal) |
|--------|---------------------|-------------------|
| **Development Time** | Zero (already built) | Custom development |
| **Maintenance** | Handled by CoinGecko | Our responsibility |
| **Flexibility** | Limited to provided tools | Full control |
| **Cost** | Free tier + Pro tier | Infrastructure only |
| **Use Case** | Market data, trends | Execution, risk, custom logic |
| **Reliability** | High (CoinGecko SLA) | Depends on our ops |
| **Latency** | External API call | Local/internal network |
| **Best For** | Data gathering | Trading execution |

**Decision**: Use CoinGecko MCP for **market intelligence**, custom servers for **execution and analysis**.

---

## System Components

### 1. MCP Client Orchestrator

**Location**: `cmd/orchestrator/`

**Responsibilities**:
- Connect to all MCP servers (external + internal)
- Coordinate agent lifecycle (start, stop, monitor)
- Aggregate signals from analysis agents
- Route decisions to strategy agents
- Enforce risk management rules
- Execute weighted voting and consensus
- Publish decisions to execution layer

**Key Features**:
- Multi-agent coordination patterns (sequential, concurrent, event-driven)
- Health monitoring and auto-restart
- Session management
- Circuit breakers for risk protection
- Metrics collection (Prometheus)

**Technology**:
- Go with official MCP SDK
- Redis for session state
- NATS for event streaming
- WebSocket for real-time updates

### 2. Analysis Agents

**Purpose**: Analyze market data and generate signals

#### 2.1 Technical Analysis Agent
- **Connects to**: CoinGecko MCP (prices), Technical Indicators Server
- **Analyzes**: Price action, indicators, patterns
- **Outputs**: BUY/SELL/HOLD signals with confidence
- **LLM Integration**: Claude analyzes indicators and generates reasoning

#### 2.2 Order Book Analysis Agent
- **Connects to**: CoinGecko MCP or Market Data Server
- **Analyzes**: Bid-ask imbalance, depth, large orders
- **Outputs**: Pressure signals (buying/selling)
- **LLM Integration**: Claude interprets order flow

#### 2.3 Sentiment Analysis Agent
- **Connects to**: News APIs, Fear & Greed Index
- **Analyzes**: Market sentiment, social media, news
- **Outputs**: Sentiment scores
- **LLM Integration**: Claude performs sentiment analysis

### 3. Strategy Agents

**Purpose**: Make trading decisions based on analysis

#### 3.1 Trend Following Agent
- **Strategy**: EMA crossovers + ADX confirmation
- **Connects to**: Technical Indicators Server
- **Outputs**: Entry/exit decisions with stop-loss
- **LLM Integration**: Claude evaluates trend strength

#### 3.2 Mean Reversion Agent
- **Strategy**: Bollinger Bands + RSI extremes
- **Connects to**: Technical Indicators Server
- **Outputs**: Mean reversion opportunities
- **LLM Integration**: Claude identifies ranging markets

#### 3.3 Arbitrage Agent
- **Strategy**: Cross-exchange price differences
- **Connects to**: CoinGecko MCP (multi-exchange prices)
- **Outputs**: Arbitrage opportunities
- **LLM Integration**: Claude assesses execution risk

### 4. Risk Management Agent

**Purpose**: Veto power over all trades

- **Connects to**: Risk Analyzer Server
- **Evaluates**: Position size, portfolio limits, risk/reward
- **Outputs**: APPROVE/REJECT with reasoning
- **Circuit Breakers**: Drawdown, rate limiting, volatility
- **LLM Integration**: Claude performs risk assessment

### 5. LLM Gateway (Bifrost)

**Purpose**: Unified interface to multiple LLM providers

**Configuration**:
```yaml
providers:
  - name: claude
    model: claude-sonnet-4-5
    priority: 1
  - name: openai
    model: gpt-4-turbo
    priority: 2
  - name: gemini
    model: gemini-pro
    priority: 3
```

**Features**:
- Automatic failover (Claude → GPT-4 → Gemini)
- Semantic caching (reduces costs)
- <100µs overhead at 5k RPS
- Single OpenAI-compatible API
- Observability (latency, costs, cache hits)

### 6. Data Layer

#### PostgreSQL + TimescaleDB
- **Candlesticks** - Hypertable with 1-day chunks, auto-compression after 7 days
- **Orders** - Order history and state tracking
- **Positions** - Current and historical positions
- **Agent Decisions** - Decision log with pgvector embeddings
- **System Metrics** - Performance and health data

#### Redis
- **Session State** - Active trading sessions
- **Cache Layer** - Market data caching (TTL-based)
- **Circuit Breaker State** - Risk management state
- **Rate Limiting** - API rate limit tracking

#### NATS
- **Event Streaming** - Market events, agent signals
- **Pub/Sub** - Agent-to-agent communication
- **JetStream** - Persistent event log

---

## Data Flow

### 1. Market Data Flow (with CoinGecko MCP)

```
┌──────────────┐
│  CoinGecko   │
│     MCP      │ ← External API calls
└──────┬───────┘
       │
       │ HTTP Streaming
       │
┌──────▼────────────┐
│  MCP Client       │ ← Connect to CoinGecko MCP
│  (Orchestrator)   │
└──────┬────────────┘
       │
       │ Call tools: get_price, get_market_chart, etc.
       │
┌──────▼────────────┐
│  Redis Cache      │ ← Cache responses (reduce API calls)
└──────┬────────────┘
       │
       │ Cached data
       │
┌──────▼────────────┐
│  Analysis Agents  │ ← Use market data for analysis
└───────────────────┘
```

**Benefits**:
- No custom WebSocket implementation
- No exchange API management
- Multi-exchange aggregation built-in
- Reduced infrastructure complexity

### 2. Signal Aggregation Flow

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│  Technical   │   │  Order Book  │   │  Sentiment   │
│   Agent      │   │   Agent      │   │   Agent      │
└──────┬───────┘   └──────┬───────┘   └──────┬───────┘
       │                  │                   │
       │ Signal           │ Signal            │ Signal
       │ (BUY, 0.8)      │ (BUY, 0.6)       │ (HOLD, 0.5)
       │                  │                   │
       └──────────────────┼───────────────────┘
                          │
                    ┌─────▼─────┐
                    │  Context  │
                    │Aggregator │
                    └─────┬─────┘
                          │
                    Market Context
                          │
       ┌──────────────────┼───────────────────┐
       │                  │                   │
┌──────▼───────┐   ┌──────▼───────┐   ┌──────▼───────┐
│   Trend      │   │  Reversion   │   │  Arbitrage   │
│   Agent      │   │   Agent      │   │   Agent      │
└──────┬───────┘   └──────┬───────┘   └──────┬───────┘
       │ Decision         │ Decision          │ Decision
       │ (BUY, 0.9)      │ (HOLD, 0.4)      │ (HOLD, 0.3)
       │                  │                   │
       └──────────────────┼───────────────────┘
                          │
                    ┌─────▼─────┐
                    │  Voting   │
                    │ (Weighted)│
                    └─────┬─────┘
                          │
                    Final Decision
                          │
                    ┌─────▼─────┐
                    │   Risk    │
                    │   Agent   │ ← Veto power
                    └─────┬─────┘
                          │
                    APPROVE/REJECT
                          │
                    ┌─────▼─────┐
                    │  Order    │
                    │  Executor │
                    └───────────┘
```

### 3. Order Execution Flow

```
┌──────────────┐
│ Orchestrator │
└──────┬───────┘
       │ Approved Decision
       │
┌──────▼────────────┐
│  Order Executor   │
│  MCP Server       │
└──────┬────────────┘
       │
       │ CCXT API call
       │
┌──────▼────────────┐
│  Binance API      │
│  (or other        │
│   exchange)       │
└──────┬────────────┘
       │
       │ Order Confirmation
       │
┌──────▼────────────┐
│  Database         │
│  (orders table)   │
└──────┬────────────┘
       │
       │ Position Update
       │
┌──────▼────────────┐
│  WebSocket API    │ ← Notify clients
└───────────────────┘
```

---

## Technology Stack

### Core Language
- **Go 1.21+** - Main backend language

### MCP Integration
- **Official MCP Go SDK v1.0.0** - Client and server implementations
- **CoinGecko MCP** - External market data server (76+ tools)
- **HTTP Streaming** - MCP transport protocol

### LLM Integration
- **Bifrost** - Unified LLM gateway
- **Claude Sonnet 4** - Primary reasoning (via Bifrost)
- **GPT-4 Turbo** - Fallback (via Bifrost)
- **Gemini Pro** - Backup (via Bifrost)

### Databases
- **PostgreSQL 17** - Primary database
- **TimescaleDB** - Time-series extension for candlesticks
- **pgvector** - Vector search for semantic memory
- **Redis 7** - Caching and session state
- **NATS 2** - Event streaming (JetStream)

### Exchange APIs
- **CCXT** - Unified exchange API (100+ exchanges)
- **go-binance** - Direct Binance integration (fallback)

### Technical Analysis
- **cinar/indicator** - 60+ technical indicators

### Configuration & Logging
- **Viper** - Configuration management
- **Zerolog** - Structured logging

### API Framework
- **Gin** - REST API framework
- **Gorilla WebSocket** - WebSocket implementation

### Monitoring
- **Prometheus** - Metrics collection
- **Grafana** - Visualization dashboards

### Infrastructure
- **Docker & Docker Compose** - Containerization
- **Kubernetes** - Production orchestration (optional)
- **GitHub Actions** - CI/CD pipeline

---

## Security Architecture

### 1. API Key Management
- **Environment Variables** - Never commit keys
- **Viper** - Secure config loading
- **Future**: HashiCorp Vault integration

### 2. Authentication & Authorization
- **JWT Tokens** - API authentication
- **Role-Based Access Control** - Permission management
- **API Keys** - Service-to-service auth

### 3. Network Security
- **TLS/SSL** - All external communication
- **mTLS** - Internal service communication (future)
- **Private Docker Network** - Service isolation

### 4. Database Security
- **Connection Pooling** - pgx with secure connections
- **Prepared Statements** - SQL injection prevention
- **Row-Level Security** - PostgreSQL RLS (future)

### 5. Rate Limiting
- **API Rate Limits** - Per-client limits
- **Circuit Breakers** - Prevent runaway trading
- **Order Rate Limiter** - Max orders per minute

---

## Deployment Architecture

### Development Environment

```
docker-compose up -d
```

**Services**:
- PostgreSQL + TimescaleDB
- Redis
- NATS
- Bifrost (LLM Gateway)
- Prometheus
- Grafana

### Production Environment (Kubernetes)

```
┌─────────────────────────────────────────┐
│            Load Balancer                │
└─────────────┬───────────────────────────┘
              │
┌─────────────▼───────────────────────────┐
│            Ingress                      │
└─────────────┬───────────────────────────┘
              │
    ┌─────────┼─────────┐
    │         │         │
┌───▼──┐ ┌───▼──┐ ┌───▼──┐
│ API  │ │ WS   │ │Orchs │  ← Pods (replicas)
│ Pod  │ │ Pod  │ │ Pod  │
└──────┘ └──────┘ └──┬───┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
┌───────▼──┐  ┌───────▼──┐  ┌──────▼───┐
│  MCP     │  │  Agents  │  │  Risk    │
│ Servers  │  │  Pods    │  │  Pod     │
└──────────┘  └──────────┘  └──────────┘
        │             │             │
        └─────────────┼─────────────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
┌───────▼──┐  ┌───────▼──┐  ┌──────▼───┐
│PostgreSQL│  │  Redis   │  │  NATS    │
│StatefulS │  │StatefulS │  │StatefulS │
└──────────┘  └──────────┘  └──────────┘
```

**Key Components**:
- **API Pods** - Horizontal scaling
- **Orchestrator Pods** - Leader election
- **Agent Pods** - Independent scaling
- **StatefulSets** - Databases (persistent storage)
- **ConfigMaps** - Configuration management
- **Secrets** - API keys and credentials

---

## Performance Considerations

### 1. Latency Targets
- **Market Data**: <100ms (via CoinGecko MCP + Redis cache)
- **Indicator Calculation**: <50ms (cinar/indicator)
- **Agent Decision**: <500ms (LLM via Bifrost)
- **Order Execution**: <200ms (CCXT direct exchange API)
- **End-to-End Decision**: <2 seconds

### 2. Throughput Targets
- **Concurrent Agents**: 10-20 agents
- **Decisions per Minute**: 100+
- **Orders per Minute**: 50 (with rate limiter)
- **API Requests**: 1000 req/sec
- **WebSocket Connections**: 1000 concurrent

### 3. Optimization Strategies
- **Redis Caching** - Reduce CoinGecko MCP calls
- **TimescaleDB Compression** - 95% storage reduction
- **Bifrost Semantic Cache** - Reduce LLM costs
- **NATS JetStream** - Async event processing
- **Connection Pooling** - Reuse database connections
- **Horizontal Scaling** - Scale agents independently

---

## Disaster Recovery

### 1. Backup Strategy
- **Database**: Daily full backup + continuous WAL archiving
- **Configuration**: Git-versioned
- **Trading State**: Redis persistence + snapshot
- **Logs**: Centralized logging (future: ELK stack)

### 2. High Availability
- **Database**: PostgreSQL replication
- **Redis**: Redis Sentinel or Redis Cluster
- **NATS**: Clustered JetStream
- **Stateless Services**: Multi-replica deployments

### 3. Circuit Breakers
- **Max Drawdown** - Halt trading at 10% drawdown
- **Order Rate Limiter** - Max 50 orders/minute
- **Volatility Breaker** - Pause in extreme volatility
- **Manual Override** - Kill switch for emergency

---

## Future Enhancements

### Phase 11+ (Post-MVP)

1. **Custom RL Models**
   - Train models on collected trading data
   - Replace some LLM decisions with faster RL agents
   - A/B test LLM vs RL performance

2. **Multi-Exchange Support**
   - Add Coinbase, Kraken, Binance.US
   - Cross-exchange arbitrage
   - CCXT makes this trivial

3. **Advanced Strategies**
   - Market making
   - Statistical arbitrage
   - Options trading

4. **Web Dashboard**
   - React/Vue frontend
   - Real-time position tracking
   - Strategy backtesting UI

5. **Mobile App**
   - iOS and Android apps
   - Push notifications
   - Remote control

---

## Conclusion

CryptoFunk's **hybrid MCP architecture** combines the best of both worlds:

- **External MCP servers** (CoinGecko) for comprehensive market data
- **Custom MCP servers** for execution and custom logic
- **LLM-powered agents** for intelligent decision-making
- **Distributed coordination** via official MCP SDK
- **Production-ready** infrastructure with monitoring and security

**Time Savings**: By using CoinGecko MCP, we save **32+ hours** of development time while gaining access to 76+ pre-built market data tools covering multiple exchanges, NFTs, DeFi, and more.

---

**Next Steps**: See [MCP_INTEGRATION.md](MCP_INTEGRATION.md) for detailed integration guide.
