# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**CryptoFunk** is a Multi-Agent AI Trading System orchestrated by Model Context Protocol (MCP). It uses LLM-powered intelligence (Claude/GPT-4) through the Bifrost gateway to coordinate specialized trading agents that analyze markets, generate strategies, and execute trades.

**Key Innovation**: Instead of a monolithic trading bot, CryptoFunk orchestrates multiple AI agents - each with specialized expertise (technical analysis, order book analysis, sentiment, trend following, mean reversion, risk management) - that collaborate through MCP to make trading decisions via weighted voting and consensus.

**Status**: All 10 core phases complete - Production Ready. See TASKS.md for implementation details.

## Build System

This project uses **Task** (taskfile.dev) instead of Make. All commands are in `Taskfile.yml`.

### Essential Commands

```bash
# Development workflow
task dev                    # Setup complete dev environment (Docker + DB + migrations)
task docker-up              # Start infrastructure (PostgreSQL, Redis, NATS)
task db-migrate             # Run database migrations
task build                  # Build all binaries to bin/ directory
task validate               # Quick validation (format, vet, build)

# Testing
task test                   # Run all tests with race detector and coverage
task test-unit              # Run unit tests only (go test -short)
task test-integration       # Run integration tests (go test -tags=integration)
task test-fast              # Quick validation checks (skip slow tests)
task lint                   # Run golangci-lint
task check                  # Run fmt, lint, test (pre-commit)

# Run specific test
go test -v -run TestPlaceMarketOrder ./internal/exchange/...

# Running components
task run-orchestrator       # Run MCP orchestrator
task run-agent-technical    # Run technical analysis agent
task run-agent-trend        # Run trend following agent
task run-agent-risk         # Run risk management agent
task run-telegram-bot       # Run Telegram bot
task run-paper              # Start paper trading mode (safe)
task run-live               # Start live trading (CAUTION: real money)

# Database
task db-status              # Show migration status
task db-reset               # Reset database (WARNING: destructive)
task db-shell               # Open PostgreSQL shell

# Git Hooks
task install-hooks          # Install pre-commit hooks
task pre-commit             # Run pre-commit checks manually

# Health checks
curl http://localhost:8081/health        # Orchestrator basic health
curl http://localhost:8081/liveness      # Orchestrator K8s liveness probe
curl http://localhost:8081/readiness     # Orchestrator K8s readiness probe (checks DB/NATS/Redis)
curl http://localhost:8081/api/v1/status # Orchestrator status (active agents, sessions)
curl http://localhost:8081/metrics       # Orchestrator Prometheus metrics
curl http://localhost:8080/health        # API
```

## Architecture Overview

CryptoFunk follows a **Hybrid MCP Architecture** - external MCP servers (CoinGecko) for market data and internal MCP servers for custom tools, coordinated by a central orchestrator.

### Data Flow

```
External MCP Servers (CoinGecko)
    ↓ (market data tools)
MCP Orchestrator (cmd/orchestrator/)
    ↓ (stdio JSON-RPC 2.0)
Internal MCP Servers:
  ├─ Market Data Server (Redis caching)
  ├─ Technical Indicators Server (RSI, MACD, Bollinger, 60+ indicators)
  ├─ Risk Analyzer Server (Kelly, VaR, Sharpe, drawdown)
  └─ Order Executor Server (paper/live trading)
    ↓
Trading Agents (MCP clients):
  ├─ Analysis Agents (technical, orderbook, sentiment)
  └─ Strategy Agents (trend, mean reversion, arbitrage)
    ↓ (signals with confidence)
Risk Management Agent (veto power, position sizing)
    ↓
Order Execution (CCXT or mock exchange)
    ↓
PostgreSQL + TimescaleDB + pgvector
    ↓
Monitoring: Prometheus + Grafana
```

### Key Architectural Patterns

**MCP Communication**: All agents communicate via JSON-RPC 2.0 over stdio. Logs go to stderr, protocol messages to stdout. This is enforced throughout.

**Weighted Voting**: Strategy agents vote on trades, orchestrator aggregates with confidence-based weights. Risk agent has veto power.

**BDI Agent Model**: Agents maintain Beliefs (market state), Desires (goals), and Intentions (planned actions).

**Circuit Breakers**: System halts trading on max drawdown, high volatility, or excessive order rate.

### Port Allocation Strategy

All services use fixed ports to avoid conflicts and enable Prometheus scraping:

| Port  | Service            | Purpose                    |
|-------|--------------------|-----------------------------|
| 8080  | API Server         | REST/WebSocket API          |
| 8081  | Orchestrator       | MCP coordination + metrics  |
| 9101  | technical-agent    | Technical analysis metrics  |
| 9102  | trend-agent        | Trend following metrics     |
| 9104  | sentiment-agent    | Sentiment analysis metrics  |
| 9105  | orderbook-agent    | Order book analysis metrics |
| 9106  | reversion-agent    | Mean reversion metrics      |
| 9107  | arbitrage-agent    | Arbitrage strategy metrics  |
| 9108  | risk-agent         | Risk management metrics     |

**Note**: Port 9103 is reserved/skipped. Prometheus scrape config in `deployments/prometheus/prometheus.yml` must match these ports.

## Technology Stack

- **Language**: Go 1.24+ (requires generics for cinar/indicator v2)
- **MCP**: `github.com/modelcontextprotocol/go-sdk v1.0.0`
- **LLM Gateway**: Bifrost (unified Claude/GPT-4/Gemini with failover)
- **Exchanges**: CCXT (100+ exchanges) + Binance SDK (`github.com/adshao/go-binance/v2`)
- **Technical Analysis**: `github.com/cinar/indicator/v2` (60+ indicators, channel-based)
- **Database**: PostgreSQL 15+ with TimescaleDB + pgvector
- **Cache/Messaging**: Redis, NATS
- **Web**: Gin, Gorilla WebSocket
- **Monitoring**: Prometheus + Grafana

## Directory Structure

```
cryptofunk/
├── cmd/
│   ├── orchestrator/              # MCP orchestrator (coordinates everything)
│   ├── mcp-servers/               # MCP tool servers (stdio JSON-RPC 2.0)
│   │   ├── market-data/
│   │   ├── technical-indicators/
│   │   ├── risk-analyzer/
│   │   └── order-executor/
│   ├── agents/                    # Trading agents (MCP clients)
│   │   ├── technical-agent/
│   │   ├── orderbook-agent/
│   │   ├── sentiment-agent/
│   │   ├── trend-agent/
│   │   ├── reversion-agent/
│   │   ├── arbitrage-agent/
│   │   └── risk-agent/
│   ├── api/                       # REST/WebSocket API server
│   ├── telegram-bot/              # Telegram bot for notifications
│   └── migrate/                   # Database migration CLI
├── internal/
│   ├── agents/                    # Base agent infrastructure
│   ├── alerts/                    # Alert management
│   ├── api/                       # API helpers
│   ├── audit/                     # Audit logging
│   ├── config/                    # Configuration + logging
│   ├── db/                        # Database layer (pgxpool)
│   ├── deps/                      # Dependency injection
│   ├── exchange/                  # Exchange abstraction + mock
│   ├── indicators/                # cinar/indicator wrappers
│   ├── llm/                       # LLM integration
│   ├── market/                    # CoinGecko client + cache
│   ├── memory/                    # Agent memory (semantic + procedural)
│   ├── metrics/                   # Prometheus metrics
│   ├── orchestrator/              # Orchestrator logic (voting, consensus)
│   ├── risk/                      # Risk management logic
│   ├── telegram/                  # Telegram bot logic
│   └── validation/                # Input validation
├── migrations/                    # SQL migrations
│   ├── 001_initial_schema.sql     # Core schema with TimescaleDB + pgvector
│   ├── 002_semantic_memory.sql
│   ├── 003_procedural_memory.sql
│   ├── 004_llm_decisions_enhancement.sql
│   └── 005_audit_logs.sql
├── configs/                       # Configuration files
├── deployments/
│   ├── k8s/                       # Kubernetes manifests
│   ├── grafana/                   # Dashboards
│   └── prometheus/                # Config
├── tests/e2e/                     # End-to-end tests
├── docker-compose.yml
├── Taskfile.yml                   # PRIMARY BUILD FILE
└── TASKS.md                       # Implementation plan
```

## Database Architecture

**Connection**: `internal/db/db.go` provides pgxpool (MaxConns: 10, MinConns: 2).

**Schema Highlights** (`migrations/001_initial_schema.sql`):
- **TimescaleDB Hypertables**: `candlesticks`, `performance_metrics`
- **Trading Tables**: `trading_sessions`, `positions`, `orders`, `trades`
- **Agent Tables**: `agent_signals`, `agent_status`, `llm_decisions`
- **pgvector**: `llm_decisions.prompt_embedding vector(1536)` with IVFFlat index

**Enums**: `order_side` (BUY/SELL), `order_type`, `order_status`, `trading_mode` (PAPER/LIVE)

## MCP Server Implementation Pattern

**CRITICAL**: stdout is for MCP protocol only. ALL logs must go to stderr.

```go
// 1. Logging to stderr (stdout reserved for MCP protocol)
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

// 2. JSON-RPC 2.0 over stdio
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
```

**DEBUGGING TIP**: If you add `fmt.Printf()`, `log.Println()`, or `println()` to stdout in MCP servers, the protocol breaks immediately. This is the #1 cause of "MCP server not responding" errors.

## Paper Trading vs Live Trading

**Paper Trading** (`internal/exchange/mock.go`):
- Realistic slippage (0.05%-0.3%), market impact, partial fills
- Safe for testing

**Live Trading**: Real exchange API via CCXT. Toggle in `configs/config.yaml`:
```yaml
trading_mode: PAPER  # or LIVE
```

## Important Conventions

**Logging**: Always use zerolog to stderr for MCP servers:
```go
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
```

**Error Handling**: Wrap with context:
```go
return fmt.Errorf("failed to execute order: %w", err)
```

**Database IDs**: UUIDs for business entities, serial IDs for time-series.

**Channel-Based Indicators**: cinar/indicator v2 uses channels:
```go
rsiChan := indicator.RSI(priceChan, period)
rsiValues := <-rsiChan
```

**Trading Safety**: Always verify `trading_mode` before placing real orders. Default to PAPER.

## Testing

**Build Tags**: Integration tests use `-tags=integration`. Test commands are in Essential Commands section above.

## Deployment

**Local (Docker Compose)**:
```bash
docker-compose up -d
docker-compose logs -f
```

**Kubernetes**:
```bash
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/
kubectl get pods -n cryptofunk
```

## Troubleshooting

**Debug Mode**:
```bash
LOG_LEVEL=debug ./bin/orchestrator    # Enable debug logging
MCP_TRACE=1 ./bin/technical-agent     # Enable MCP message tracing
```

**MCP Server Not Responding**: Check stderr logs. Ensure no stdout debugging.

**Database Connection**: Verify Docker services (`docker-compose ps`). Check health: `curl http://localhost:8081/health`

**Migration Errors**: Run `task db-status`. Reset with `task db-reset` (WARNING: destructive).

## Additional Resources

- **TASKS.md**: Complete implementation plan (244 tasks across 10 phases)
- **docs/MCP_INTEGRATION.md**: MCP architecture details
- **docs/LLM_AGENT_ARCHITECTURE.md**: Agent design patterns
- **docs/OPEN_SOURCE_TOOLS.md**: Technology choices rationale
