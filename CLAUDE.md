# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**CryptoFunk** is a Multi-Agent AI Trading System orchestrated by Model Context Protocol (MCP). It uses LLM-powered intelligence (Claude/GPT-4) through the Bifrost gateway to coordinate specialized trading agents that analyze markets, generate strategies, and execute trades.

**Key Innovation**: Instead of a monolithic trading bot, CryptoFunk orchestrates multiple AI agents - each with specialized expertise (technical analysis, order book analysis, sentiment, trend following, mean reversion, risk management) - that collaborate through MCP to make trading decisions via weighted voting and consensus.

**Current Status**: Phase 10 in progress (Production Hardening). Phases 1-9 substantially complete. Currently on feature/phase-10-production-hardening branch. See TASKS.md for detailed implementation progress across all 10 phases.

## Build System

This project uses **Task** (taskfile.dev) instead of Make. All build, test, and development commands are in `Taskfile.yml`.

**Why Task over Make**: See `docs/TASK_VS_MAKE.md` for rationale. TLDR: Better dependency management, native YAML, cleaner syntax.

### Essential Commands

```bash
# Development workflow
task dev                    # Setup complete dev environment (Docker + DB + migrations)
task docker-up              # Start infrastructure (PostgreSQL, Redis, NATS)
task db-migrate             # Run database migrations
task build                  # Build all binaries to bin/ directory

# Testing
task test                   # Run all tests with race detector and coverage
task test-unit              # Run unit tests only
task test-integration       # Run integration tests
task lint                   # Run golangci-lint
task check                  # Run fmt, lint, test (pre-commit)

# Running components
task run-orchestrator       # Run MCP orchestrator (coordinates agents)
task run-agent-technical    # Run technical analysis agent
task run-agent-trend        # Run trend following strategy agent
task run-agent-risk         # Run risk management agent
task run-paper              # Start paper trading mode (safe testing)
task run-live               # Start live trading (CAUTION: real money)

# Database operations
task db-status              # Show migration status
task db-reset               # Reset database (WARNING: destructive)
task db-shell               # Open PostgreSQL shell

# Docker management
task docker-down            # Stop all services
task docker-logs            # Show Docker logs
task docker-clean           # Remove volumes (fresh start)

# Building individual components
task build-orchestrator     # Build orchestrator only
task build-servers          # Build all MCP servers
task build-agents           # Build all agents

# Deployment (see Deployment section for details)
docker-compose up -d                                       # Local deployment
kubectl apply -f deployments/k8s/                          # Kubernetes deployment

# Health checks
curl http://localhost:8081/health                          # Check orchestrator health
curl http://localhost:8080/health                          # Check API health
```

## Architecture Overview

CryptoFunk follows a **Hybrid MCP Architecture** - using both external MCP servers (CoinGecko for market data) and internal MCP servers (custom tools) coordinated by a central orchestrator.

### High-Level Data Flow

```
External MCP Servers (e.g., CoinGecko)
    ↓ (market data tools: get_price, get_market_chart)
MCP Orchestrator (cmd/orchestrator/)
    ↓ (stdio JSON-RPC 2.0 communication)
Internal MCP Servers:
  ├─ Market Data Server (caching layer)
  ├─ Technical Indicators Server (RSI, MACD, Bollinger, EMA, ADX)
  ├─ Risk Analyzer Server (Kelly Criterion, VaR, limits, Sharpe, drawdown)
  └─ Order Executor Server (paper/live trading)
    ↓ (tool calls and context sharing)
Trading Agents (MCP clients):
  ├─ Analysis Agents (technical, orderbook, sentiment)
  └─ Strategy Agents (trend following, mean reversion, arbitrage)
    ↓ (generate signals with confidence and reasoning)
Risk Management Agent
    ↓ (veto power, position sizing, circuit breakers)
Order Execution
    ↓ (via CCXT unified API or mock exchange)
Exchange (Binance/multi-exchange) or Paper Trading
    ↓ (fills, trades, positions)
PostgreSQL + TimescaleDB + pgvector
    ↓ (persistence and analytics)
Monitoring: Prometheus + Grafana
```

### Architecture Layers (Vertical Stack)

1. **MCP Orchestrator** (`cmd/orchestrator/`): Coordinates all agents, manages sessions, implements weighted voting and consensus
2. **MCP Servers** (`cmd/mcp-servers/`): Expose tools via JSON-RPC 2.0 over stdio
   - Market Data: CoinGecko integration + Redis caching + TimescaleDB sync
   - Technical Indicators: Wraps cinar/indicator library (60+ indicators)
   - Risk Analyzer: Kelly Criterion, VaR, portfolio limits, Sharpe, drawdown
   - Order Executor: Paper trading (mock exchange) and live trading (CCXT)
3. **Trading Agents** (`cmd/agents/`): MCP clients that make decisions
   - **Analysis Agents**: Generate market insights (technical, orderbook, sentiment)
   - **Strategy Agents**: Generate trading decisions (trend, mean reversion, arbitrage)
   - **Risk Agent**: Evaluates and vetoes trades, calculates position sizes
4. **Data Layer** (`internal/db/`, `migrations/`):
   - PostgreSQL with TimescaleDB for time-series (candlesticks, metrics)
   - pgvector for semantic search (1536-dim OpenAI embeddings)
   - Redis for caching (CoinGecko API responses)
   - NATS for event-driven coordination

### Key Architectural Patterns

**MCP Communication**: All agents communicate via JSON-RPC 2.0 over stdio. Logs go to stderr, protocol messages to stdout. This is enforced throughout the codebase.

**Hybrid MCP**: External servers (CoinGecko) provide market data, internal servers provide custom trading tools. This avoids reinventing market data infrastructure.

**Weighted Voting**: Strategy agents vote on trades, orchestrator aggregates with confidence-based weights. Risk agent has veto power.

**BDI Agent Model**: Agents maintain Beliefs (market state), Desires (goals), and Intentions (planned actions). See Phase 3 in TASKS.md.

**Circuit Breakers**: System halts trading on max drawdown, high volatility, or excessive order rate. See Phase 6 in TASKS.md.

## Technology Stack

**Core Technologies**:
- **Language**: Go 1.21+ (requires generics for cinar/indicator v2)
- **MCP**: Model Context Protocol for agent coordination (github.com/modelcontextprotocol/go-sdk)
- **LLM Gateway**: Bifrost (unified Claude/GPT-4/Gemini API with automatic failover)
- **Exchanges**: CCXT (100+ exchanges unified API) + direct Binance SDK
- **Technical Analysis**: cinar/indicator v2 (60+ indicators with channel-based API)
- **Database**: PostgreSQL 15+ with TimescaleDB (hypertables, compression) and pgvector (vector search)
- **Caching**: Redis (CoinGecko responses, session state)
- **Messaging**: NATS (event-driven coordination)
- **Web**: Gin (REST API), Gorilla WebSocket (real-time updates)
- **Monitoring**: Prometheus + Grafana
- **Deployment**: Docker, Kubernetes

**Key Dependencies** (see `go.mod`):
- `github.com/modelcontextprotocol/go-sdk v1.0.0` - Official MCP SDK
- `github.com/adshao/go-binance/v2 v2.8.7` - Binance API
- `github.com/cinar/indicator/v2 v2.1.22` - Technical indicators
- `github.com/jackc/pgx/v5 v5.7.6` - PostgreSQL driver with connection pooling
- `github.com/redis/go-redis/v9 v9.16.0` - Redis client
- `github.com/nats-io/nats.go v1.47.0` - NATS messaging
- `github.com/rs/zerolog v1.34.0` - Structured logging

## Directory Structure

Understanding the relationship between these directories is crucial:

```
cryptofunk/
├── cmd/                           # All executable entry points
│   ├── orchestrator/              # MCP orchestrator (coordinates everything)
│   ├── mcp-servers/               # MCP tool servers (stdio JSON-RPC 2.0)
│   │   ├── market-data/           # Market data + CoinGecko integration
│   │   ├── technical-indicators/  # RSI, MACD, Bollinger, EMA, ADX
│   │   ├── risk-analyzer/         # Kelly, VaR, limits, Sharpe, drawdown
│   │   └── order-executor/        # Paper/live trading with CCXT
│   ├── agents/                    # Trading agents (MCP clients)
│   │   ├── technical-agent/       # Technical analysis signals
│   │   ├── orderbook-agent/       # Order book analysis
│   │   ├── sentiment-agent/       # Sentiment analysis
│   │   ├── trend-agent/           # Trend following strategy
│   │   ├── reversion-agent/       # Mean reversion strategy
│   │   └── risk-agent/            # Risk management with veto power
│   ├── api/                       # REST/WebSocket API server
│   └── migrate/                   # Database migration CLI tool
├── internal/                      # Private application code
│   ├── db/                        # Database layer (pgxpool connection)
│   ├── exchange/                  # Exchange abstraction + mock
│   ├── risk/                      # Risk management logic
│   ├── indicators/                # cinar/indicator wrappers
│   ├── market/                    # CoinGecko client + cache + sync
│   ├── agents/                    # Base agent infrastructure
│   ├── orchestrator/              # Orchestrator logic (voting, consensus)
│   ├── config/                    # Configuration + logging
│   └── memory/                    # Agent memory systems (future)
├── pkg/                           # Public libraries (if any)
├── migrations/                    # SQL migration files (001_*.sql)
│   └── 001_initial_schema.sql     # Complete schema with TimescaleDB + pgvector
├── configs/                       # Configuration files
│   ├── config.yaml                # Main config (exchange, DB, MCP)
│   ├── agents.yaml                # Agent configurations
│   └── orchestrator.yaml          # Orchestrator settings
├── scripts/                       # Utility scripts
├── deployments/                   # Deployment configurations
│   ├── k8s/                       # Kubernetes YAML
│   ├── grafana/                   # Grafana dashboards
│   └── prometheus/                # Prometheus configuration
├── docs/                          # Documentation
│   ├── OPEN_SOURCE_TOOLS.md       # Rationale for tool choices
│   ├── MCP_INTEGRATION.md         # MCP architecture details
│   ├── TASK_VS_MAKE.md            # Why Task over Make
│   └── LLM_AGENT_ARCHITECTURE.md  # Agent design patterns
├── docker-compose.yml             # Docker Compose for local development (all services)
├── Taskfile.yml                   # Task build definitions (PRIMARY BUILD FILE)
├── TASKS.md                       # Implementation plan (10 phases)
├── README.md                      # Project overview and quick start
└── CLAUDE.md                      # This file (guidance for Claude Code)
```

**Important Relationships**:
- `cmd/orchestrator/` starts and coordinates all agents in `cmd/agents/`
- Agents connect to MCP servers in `cmd/mcp-servers/` via stdio
- `internal/db/` is used by all components for persistence
- `migrations/` SQL files are applied by `cmd/migrate/`
- Configuration in `configs/` is loaded by all components via Viper

## Database Architecture

**Connection Pattern**: All components use `internal/db/db.go` which provides pgxpool connection pooling:
- MaxConns: 10, MinConns: 2
- MaxConnLifetime: 1 hour
- MaxConnIdleTime: 30 minutes
- HealthCheckPeriod: 1 minute

**Schema Highlights** (see `migrations/001_initial_schema.sql`):

1. **TimescaleDB Hypertables**:
   - `candlesticks` - OHLCV data partitioned by time (1-day chunks, 7-day compression)
   - `performance_metrics` - Time-series performance tracking

2. **Trading Tables**:
   - `trading_sessions` - Session tracking with performance metrics (UUID-based)
   - `positions` - Long/short positions with entry/exit tracking
   - `orders` - Order lifecycle (NEW → PARTIALLY_FILLED → FILLED/CANCELED)
   - `trades` - Individual fills (many-to-one with orders)

3. **Agent Tables**:
   - `agent_signals` - Agent decisions with LLM reasoning and context
   - `agent_status` - Real-time agent health and heartbeat
   - `llm_decisions` - LLM decision history with embeddings for learning

4. **pgvector Extension**:
   - `llm_decisions.prompt_embedding vector(1536)` - OpenAI embeddings for semantic search
   - IVFFlat index for fast vector similarity (cosine distance)

5. **Key Enums** (type safety):
   - `order_side`: BUY, SELL
   - `order_type`: MARKET, LIMIT, STOP_LOSS, etc.
   - `order_status`: NEW, PARTIALLY_FILLED, FILLED, CANCELED, REJECTED, EXPIRED
   - `trading_mode`: PAPER, LIVE

**Migration Pattern**: Migrations use `internal/db/migrate.go` which:
- Creates `schema_version` table for tracking
- Applies migrations in order (001_*, 002_*, etc.)
- Each migration runs in a transaction (rollback on error)
- Run via: `task db-migrate`

## MCP Server Implementation Pattern

All MCP servers follow this pattern (example from `cmd/mcp-servers/order-executor/main.go`):

```go
// 1. Logging to stderr (stdout reserved for MCP protocol)
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

// 2. Initialize dependencies (database, exchange, etc.)
database, err := db.New(ctx)
exchangeService := exchange.NewService(database)

// 3. Define MCP server with tool handlers
type MCPServer struct {
    service *exchange.Service
}

// 4. Implement JSON-RPC 2.0 over stdio
func (s *MCPServer) Run() error {
    decoder := json.NewDecoder(os.Stdin)
    encoder := json.NewEncoder(os.Stdout)

    for {
        var request MCPRequest
        decoder.Decode(&request)
        response := s.handleRequest(&request)
        encoder.Encode(response)
    }
}

// 5. Register tools in initialize response
func (s *MCPServer) handleInitialize() MCPResponse {
    return MCPResponse{
        Result: map[string]interface{}{
            "protocolVersion": "2024-11-05",
            "capabilities": map[string]interface{}{
                "tools": map[string][]map[string]interface{}{
                    "listChanged": true,
                },
            },
            "serverInfo": map[string]string{
                "name":    "order-executor",
                "version": "1.0.0",
            },
        },
    }
}

// 6. Tool handlers extract params and call service layer
func (s *MCPServer) handleToolCall(method string, params map[string]interface{}) interface{} {
    switch method {
    case "place_market_order":
        return s.placeMarketOrder(params)
    case "place_limit_order":
        return s.placeLimitOrder(params)
    // ...
    }
}
```

**Critical Details**:
- **stdout is for MCP protocol only** - all logs, debug output, and errors go to stderr
- **stdio transport** - JSON-RPC 2.0 messages over stdin/stdout (not HTTP)
- **Structured logging** - Use zerolog with `os.Stderr` output
- **Parameter extraction** - Support multiple numeric types (float64, int, string)
- **Error handling** - Return proper MCP error responses

**CRITICAL DEBUGGING TIP**: If you add any `fmt.Printf()`, `log.Println()`, `println()`, or similar output to stdout in MCP servers, the protocol will break immediately. ALL debugging output must use zerolog configured to stderr. Never print to stdout except for MCP protocol JSON-RPC messages. This is the #1 cause of "MCP server not responding" errors.

## Paper Trading vs Live Trading

The Order Executor Server supports both modes:

**Paper Trading** (`internal/exchange/mock.go`):
- Realistic order fills with slippage (0.05%-0.3% based on order size)
- Market impact simulation (larger orders = more slippage)
- Partial fills for large orders (split into 20-40% portions with price variation)
- Simulated latency and realistic timestamps
- Safe for testing strategies without risk

**Live Trading** (Phase 7, not yet complete):
- Real exchange API via CCXT (`exchange.createOrder()`)
- Testnet mode available (`exchange.setSandboxMode(true)`)
- Unified API across 100+ exchanges
- Proper error handling and retry logic

**Toggle**: Set `trading_mode: PAPER` or `LIVE` in `configs/config.yaml`

## Configuration Management

Configuration uses Viper with environment variable overrides:

```yaml
# configs/config.yaml
database:
  url: "${DATABASE_URL}"  # Env var interpolation

exchanges:
  binance:
    api_key: "${BINANCE_API_KEY}"
    api_secret: "${BINANCE_API_SECRET}"
    testnet: true  # Always start with testnet!

mcp:
  servers:
    coingecko:
      endpoint: "https://mcp.api.coingecko.com/mcp"
      transport: "http"

redis:
  addr: "localhost:6379"
  db: 0

nats:
  url: "nats://localhost:4222"
```

**Environment Variables**: See `.env.example` for required variables. Priority: env vars > config.yaml > defaults.

## Git Workflow

**Branch Strategy**: Feature branches are typically named after phases (e.g., `feature/phase-1-foundation`) but may contain work from multiple phases as development progresses. The main branch (`main`) contains stable releases.

**Commit Style**: Commits follow conventional format with phase tracking:
```
feat: Phase 7 Position Tracking & P&L Calculation (T144-T146) - Part 2
feat: Phase 6 Risk Management Agent (T119-T124)
fix: Correct slippage calculation in mock exchange
```

**Common Git Commands**:
```bash
# Check current status
git status

# Create feature branch
git checkout -b feature/phase-X-description

# Commit changes
git add .
git commit -m "feat: Phase X Description (TXXX)"

# Push to remote
git push origin feature/phase-X-description

# Merge to main (after PR approval)
git checkout main
git merge feature/phase-X-description
```

## Deployment

### Docker Compose (Local Development)

The project includes a comprehensive Docker Compose setup in the root `docker-compose.yml` for local development:

```bash
# Start all services including infrastructure
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

**Services included**:
- PostgreSQL (with TimescaleDB and pgvector)
- Redis (for caching)
- NATS (for event messaging)
- Prometheus (metrics collection)
- Grafana (visualization)
- All MCP servers (market-data, technical-indicators, risk-analyzer, order-executor)
- Orchestrator
- API server

### Kubernetes (Production)

Production-ready Kubernetes manifests are in `deployments/k8s/`:

```bash
# Deploy to Kubernetes
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/configmap.yaml
kubectl apply -f deployments/k8s/secrets.yaml  # Create this from secrets.yaml.example
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -n cryptofunk
kubectl get services -n cryptofunk

# View logs
kubectl logs -f deployment/orchestrator -n cryptofunk
```

**Key features**:
- Health checks and readiness probes on all services
- Resource limits and requests configured
- Horizontal Pod Autoscaling (HPA) ready
- Persistent volumes for PostgreSQL
- ConfigMaps for configuration management
- Secrets for sensitive data (API keys, database passwords)

**IMPORTANT**: Before deploying to Kubernetes:
1. Create secrets from `deployments/k8s/secrets.yaml.example`
2. Update ConfigMap with your environment-specific values
3. Ensure you have a Kubernetes cluster provisioned
4. Review resource limits based on your cluster capacity

## Common Development Workflows

### Starting Fresh Development Environment

```bash
# 1. Start infrastructure
task docker-up

# 2. Run migrations
task db-migrate

# 3. Verify database
task db-status

# Or do all at once:
task dev
```

### Adding a New MCP Tool

1. Add tool handler to `cmd/mcp-servers/*/main.go`
2. Register tool in `handleInitialize()` response
3. Implement tool logic in `internal/` service layer
4. Add tests in `*_test.go`
5. Update API documentation

### Running Paper Trading Test

```bash
# Start orchestrator
task run-orchestrator &

# Start agents (in separate terminals or background)
task run-agent-technical &
task run-agent-trend &
task run-agent-risk &

# Start paper trading
task run-paper
```

### Running Individual MCP Servers (for Testing/Debugging)

MCP servers communicate via JSON-RPC 2.0 over stdio. You can test them individually:

```bash
# Run server directly (will wait for stdin input)
./bin/market-data-server
./bin/technical-indicators-server
./bin/risk-analyzer-server
./bin/order-executor-server

# Test with sample initialize request
echo '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}' | ./bin/market-data-server

# Test with sample tool call (after initialize)
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_price","arguments":{"symbol":"BTC"}},"id":2}' | ./bin/market-data-server

# For interactive testing, use a JSON-RPC client or build a test harness
# Note: Remember that logs go to stderr, protocol messages to stdout
```

**Debugging MCP Servers**:
- All logs appear on stderr (use `2> debug.log` to capture)
- Protocol messages on stdout (pipe to `jq` for formatting: `| jq`)
- Use `MCP_TRACE=1` environment variable for detailed tracing (if implemented)
- Never add `fmt.Printf()` or similar - it breaks the protocol!

### Adding a Database Migration

```bash
# Create migration file: migrations/002_description.sql
# Then apply:
task db-migrate

# Check status:
task db-status
```

## Testing Strategy

**Test Coverage Requirements**:
- Unit tests: >80% coverage
- Integration tests: All MCP servers and agents
- E2E tests: Full orchestrator → agents → execution flow

**Running Tests**:
```bash
task test              # All tests with coverage
task test-unit         # Unit tests only
task test-integration  # Integration tests
task lint              # golangci-lint
task check             # Pre-commit checks (fmt, lint, test)

# Run tests for a specific package
go test -v ./internal/exchange/...

# Run a specific test function
go test -v -run TestPlaceMarketOrder ./internal/exchange/...

# Run tests with verbose output and race detection
go test -v -race ./internal/orchestrator/...
```

**Test Patterns**:
- Mock MCP servers for agent tests
- Mock exchange for order executor tests
- Use testcontainers for PostgreSQL integration tests
- Benchmark critical paths (indicator calculations, tool latency)

## Phase Status (Reference TASKS.md)

**Completed**:
- ✅ Phase 1: Foundation (project structure, Docker, database)
- ✅ Phase 2: MCP Servers (market data, technical indicators, risk analyzer, order executor)
- ✅ Phase 3: Analysis Agents (technical, orderbook, sentiment)
- ✅ Phase 4: Strategy Agents & Backtesting (trend, mean reversion, backtesting framework)
- ✅ Phase 5: Orchestrator (coordination, weighted voting, consensus)
- ✅ Phase 6: Risk Management Agent (circuit breakers, position sizing, veto power)
- ✅ Phase 7: Real Exchange Integration & Position Tracking (exchange connectivity, P&L calculation)
- ✅ Phase 8: Monitoring & Observability (Prometheus, Grafana, health checks)
- ✅ Phase 9: LLM Agent Intelligence (agent reasoning, explainability, prompt engineering)

**Current Focus**: Phase 10 - Production Readiness & Deployment (CRITICAL)
- Legal compliance (LICENSE, CONTRIBUTING) - pending
- Core functionality fixes (CoinGecko MCP, Technical Indicators) - pending
- Testing infrastructure (E2E tests, CI/CD) - pending
- Docker & Kubernetes (completed)
- Security hardening - in progress
- Production deployment preparation - in progress

**Phase 11** (Optional): Advanced Features - Already Complete ✅
- Semantic & procedural memory, agent hot-swapping, A/B testing
- All features implemented and tested
- Can be enabled post-launch for enhanced capabilities

**Phase 12** (Final): Monitoring & Observability - 0.5 weeks
- Prometheus metrics, Grafana dashboards, alerting
- Final step after production deployment

**Note**: Phases reorganized on 2025-11-04 to prioritize production readiness over advanced features. Phase 10 is now the critical path to production launch.

## Important Conventions

**Logging**: Always use zerolog. For MCP servers, output to stderr:
```go
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
log.Info().Msg("Server started")  // Goes to stderr, not stdout
```

**Error Handling**: Wrap errors with context:
```go
if err != nil {
    return fmt.Errorf("failed to execute order: %w", err)
}
```

**Database IDs**: Use UUIDs for business entities (sessions, positions, orders), serial IDs for time-series data.

**Channel-Based Indicators**: cinar/indicator v2 uses channels. Example:
```go
rsiChan := indicator.RSI(priceChan, period)
rsiValues := <-rsiChan  // Read from channel
```

**Trading Mode Safety**: Always verify `trading_mode` config before placing real orders. Default to PAPER if unclear.

## Performance Considerations

**TimescaleDB**: Use hypertables for time-series data. Compression policies reduce storage by 90%+.

**Connection Pooling**: pgxpool is configured in `internal/db/db.go`. Don't create new connections per query.

**Redis Caching**: CoinGecko responses cached for 60s (ticker), 5min (OHLCV). Adjust TTL in `internal/market/cache.go`.

**MCP Latency**: Tool calls should complete in <100ms. Use async patterns for long operations.

**Bifrost Gateway**: 50x faster than LiteLLM, <100µs overhead at 5k RPS. Semantic caching reduces LLM costs by 90%.

## Security Notes

**API Keys**: Never commit API keys. Use environment variables or HashiCorp Vault (Phase 10).

**Paper Trading First**: ALWAYS test strategies in paper mode before live trading.

**Circuit Breakers**: Implemented in Phase 6. System auto-halts on max drawdown or high volatility.

**Database Access**: Use parameterized queries (pgx handles this). Never construct SQL strings.

## Health Checks and Monitoring

All services expose health check endpoints for monitoring and orchestration:

**Health Check Endpoints**:
- Orchestrator: `http://localhost:8081/health`
- API Server: `http://localhost:8080/health`
- MCP Servers: `http://localhost:808X/health` (where X = 2-5 for different servers)

**Health Check Response**:
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:30:00Z",
  "version": "1.0.0",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "nats": "ok"
  }
}
```

**Monitoring Health**:
```bash
# Check all services
curl http://localhost:8081/health  # Orchestrator
curl http://localhost:8080/health  # API

# In Kubernetes
kubectl get pods -n cryptofunk  # Shows ready status based on health checks
```

**Readiness vs Liveness**:
- **Liveness probes**: Check if service is running (restart if failing)
- **Readiness probes**: Check if service can accept traffic (remove from load balancer if failing)
- Both are configured in Kubernetes manifests with appropriate delays and thresholds

## Troubleshooting

**MCP Server Not Responding**: Check stderr logs. Ensure stdout is not being used for debugging.

**Database Connection Issues**: Verify `docker-compose.yml` services are running (`docker-compose ps`). Check health endpoint: `curl http://localhost:8081/health`

**Migration Errors**: Check `task db-status`. Reset with `task db-reset` (WARNING: deletes all data).

**Agent Failures**: Check agent status in `agent_status` table. Orchestrator has health monitoring and auto-restart. Verify health endpoints are responding.

**Services Not Ready in Kubernetes**: Check pod logs with `kubectl logs -f pod/<pod-name> -n cryptofunk`. Verify readiness probes are passing. Common issues: database not ready, missing secrets, incorrect configuration.

## Additional Resources

- **TASKS.md**: Complete 10-phase implementation plan with 244 tasks
- **README.md**: Project overview, quick start, features
- **docs/OPEN_SOURCE_TOOLS.md**: Rationale for CCXT, cinar/indicator, TimescaleDB, etc.
- **docs/MCP_INTEGRATION.md**: Detailed MCP architecture and tool design
- **docs/LLM_AGENT_ARCHITECTURE.md**: Agent design patterns and prompt templates (Phase 9)
- **Taskfile.yml**: All available commands (50+ tasks)

## Questions for the User

When working with this codebase, ask the user for clarification on:
- Which phase they want to work on (reference TASKS.md)
- Paper trading vs live trading mode (default to paper)
- Which agent or MCP server to focus on
- Performance vs feature trade-offs

## Final Notes

This is a **sophisticated multi-agent trading platform** with real-world production considerations. Key priorities:

1. **Safety First**: Paper trading mode, circuit breakers, risk management
2. **MCP Protocol**: All agent communication follows JSON-RPC 2.0 over stdio
3. **Data Integrity**: TimescaleDB for time-series, proper indexing, connection pooling
4. **Observability**: Structured logging, Prometheus metrics, Grafana dashboards
5. **Incremental Development**: Follow the 10-phase plan in TASKS.md

When in doubt, refer to TASKS.md for the detailed implementation plan and acceptance criteria for each component.
