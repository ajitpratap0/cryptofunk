# CryptoFunk Architecture

A multi-agent AI trading platform supporting Binance crypto trading and Polymarket prediction markets.

## System Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                        External APIs                             │
│  Binance  │  CoinGecko  │  Polymarket (Gamma + CLOB)  │  LLMs  │
└─────┬─────┴──────┬──────┴──────────┬──────────────────┴────┬───┘
      │            │                 │                       │
┌─────▼────────────▼─────┐  ┌───────▼───────┐  ┌───────────▼────┐
│     MCP Servers         │  │  Gamma Client │  │  Bifrost LLM   │
│  market-data            │  │  (read-only)  │  │  Gateway       │
│  technical-indicators   │  └──┬────────┬───┘  └───────┬────────┘
│  risk-analyzer          │     │        │              │
│  order-executor         │     │        │              │
│  polymarket (CLOB+live) │     │        │              │
└─────────┬───────────────┘     │        │              │
          │                     │        │              │
┌─────────▼─────────────────────▼──┐  ┌──▼──────────────▼───────┐
│         BDI Agents (8)           │  │    CLI Tools             │
│  technical, sentiment, trend,    │  │  polymarket-scanner      │
│  reversion, orderbook, arb,      │  │  polymarket-analyzer     │
│  risk, polymarket                │  │  polymarket-paper        │
│         │ NATS signals           │  │  outcome-resolver        │
└─────────┬────────────────────────┘  └──────────┬───────────────┘
          │                                      │
┌─────────▼────────────┐                         │
│    Orchestrator       │                         │
│  consensus voting     │                         │
│  trade decisions      │                         │
└─────────┬─────────────┘                         │
          │                                       │
┌─────────▼───────────────────────────────────────▼──────────────┐
│                      PostgreSQL + TimescaleDB                   │
│  candlesticks, positions, orders, trades, llm_decisions,        │
│  agent_signals, polymarket_markets, polymarket_positions,       │
│  polymarket_trades, polymarket_predictions, ...                 │
└─────────┬──────────────────────────────────────────────────────┘
          │
┌─────────▼────────────┐     ┌──────────────────────┐
│    API Server (Gin)   │────▶│  Dashboard (Next.js) │
│    REST + WebSocket   │     │  Explainability UI   │
└───────────────────────┘     └──────────────────────┘
```

## Component Map

| Binary | Purpose |
|--------|---------|
| `cmd/api/` | REST API + WebSocket server (Gin). Serves all data to dashboards. |
| `cmd/orchestrator/` | Agent coordination, consensus voting, trade decisions. |
| `cmd/migrate/` | Database migration runner. |
| `cmd/backtest/` | Backtesting engine. |
| `cmd/telegram-bot/` | Telegram alerts and control. |
| `cmd/polymarket-paper/` | Interactive CLI for Polymarket paper trading. `--db` for DB mode. |
| `cmd/polymarket-scanner/` | Fetches/filters active markets. `--db` writes to DB. |
| `cmd/polymarket-analyzer/` | LLM analysis on a market. `--db` writes predictions to DB. |
| `cmd/outcome-resolver/` | Resolves Polymarket predictions against actual outcomes. |
| `cmd/agents/*-agent/` | BDI trading agents (8 total). Communicate via NATS. |
| `cmd/mcp-servers/` | MCP tool servers for market data, indicators, risk, orders, Polymarket. |

## Data Flow

### Binance Trading Path

```
CoinGecko/Binance → MCP Servers → Agents → NATS → Orchestrator
→ Consensus Vote → llm_decisions → order-executor → Binance
→ orders → trades → positions → API → Dashboard
```

### Polymarket Paper Trading Path

```
Gamma API → polymarket-scanner (--db) → polymarket_markets table
Gamma API → polymarket-analyzer (--db) → polymarket_predictions table
polymarket-paper (--db) → DBPaperEngine → polymarket_positions + polymarket_trades
outcome-resolver → checks resolved markets → updates polymarket_predictions
API /polymarket/* → Dashboard Polymarket page
```

### Polymarket Agent Path

```
MCP polymarket server → CLOB API (live trading)
polymarket-agent → MCP tools → NATS signals → Orchestrator
```

## API Endpoints

All under `/api/v1/`.

### Core
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/status` | System status |
| GET | `/ws` | WebSocket connection |

### Agents
| Method | Path | Description |
|--------|------|-------------|
| GET | `/agents` | List all agents |
| GET | `/agents/:name` | Get agent details |
| GET | `/agents/:name/status` | Agent health status |

### Trading
| Method | Path | Description |
|--------|------|-------------|
| GET | `/positions` | List positions |
| GET | `/positions/:symbol` | Get position |
| GET | `/orders` | List orders |
| GET | `/orders/:id` | Get order |
| POST | `/orders` | Place order |
| DELETE | `/orders/:id` | Cancel order |
| POST | `/trade/start` | Start trading |
| POST | `/trade/stop` | Stop trading |
| POST | `/trade/pause` | Pause trading |
| POST | `/trade/resume` | Resume trading |

### Decisions & Analytics
| Method | Path | Description |
|--------|------|-------------|
| GET | `/decisions` | List LLM decisions |
| GET | `/decisions/stats` | Decision statistics |
| GET | `/decisions/:id` | Get decision |
| GET | `/decisions/:id/similar` | Similar decisions (vector search) |
| POST | `/decisions/search` | Search decisions |
| GET | `/decisions/analytics` | Decision outcome analytics |
| GET | `/decisions/outcomes` | Outcome tracking |

### Polymarket
| Method | Path | Description |
|--------|------|-------------|
| GET | `/polymarket/markets` | List tracked markets |
| GET | `/polymarket/markets/:id` | Market detail |
| GET | `/polymarket/positions` | Paper positions |
| GET | `/polymarket/positions/:id` | Position detail |
| POST | `/polymarket/trade` | Execute paper trade |
| GET | `/polymarket/portfolio` | Portfolio summary |
| GET | `/polymarket/trades` | Trade history |
| GET | `/polymarket/predictions` | Prediction history |
| GET | `/polymarket/performance` | Prediction accuracy metrics |

### Dashboard & Other
| Method | Path | Description |
|--------|------|-------------|
| GET | `/dashboard` | Aggregated dashboard data |
| GET | `/dashboard/unified` | Unified cross-platform portfolio |
| GET | `/dashboard/positions` | Dashboard positions view |
| GET | `/dashboard/pnl` | P&L summary |
| GET | `/strategies/current` | Current strategy config |
| POST | `/backtest/run` | Run backtest |
| GET | `/feedback` | Decision feedback |

## Database Schema

**17 migrations** (001–017). Key tables:

| Table | Purpose |
|-------|---------|
| `candlesticks` | OHLCV data (TimescaleDB hypertable) |
| `trading_sessions` | Trading sessions with P&L |
| `positions` | Binance positions |
| `orders` | Exchange orders (linked to `llm_decisions` via `decision_id`) |
| `trades` | Executed fills |
| `agent_signals` | Agent trading signals with reasoning |
| `llm_decisions` | LLM decision history + vector embeddings + outcome tracking |
| `polymarket_markets` | Tracked Polymarket markets |
| `polymarket_positions` | Paper trading positions |
| `polymarket_trades` | Paper trade records |
| `polymarket_predictions` | LLM predictions with resolution tracking |
| `semantic_memory` | Agent semantic memory (pgvector) |
| `procedural_memory` | Agent rules and patterns |
| `strategies` | Strategy configurations |
| `backtest_jobs` | Backtest runs and results |

## Running Locally

### Prerequisites
- Go 1.22+, Node.js 20+, Docker

### Start Infrastructure
```bash
# Set required env vars
export POSTGRES_PASSWORD=your_password
export GRAFANA_ADMIN_PASSWORD=your_password
export JWT_SECRET=your_jwt_secret

# Start all services
docker compose up -d

# Wait for migrations to complete
docker compose logs -f migrate
```

### Start the API Server
```bash
export DATABASE_URL="postgresql://postgres:${POSTGRES_PASSWORD}@localhost:5432/cryptofunk?sslmode=disable"
go run ./cmd/api/
```

### Start the Dashboard
```bash
cd web/dashboard
npm install
npm run dev
# Open http://localhost:3000
```

## Paper Trading

### Via CLI
```bash
# Scan markets and save to DB
go run ./cmd/polymarket-scanner/ --db --min-volume 10000 --limit 20

# Analyze a specific market
go run ./cmd/polymarket-analyzer/ --db <market_id>

# Interactive paper trading (DB-backed)
go run ./cmd/polymarket-paper/ --db scan
go run ./cmd/polymarket-paper/ --db buy <market_id> YES 50 0.65
go run ./cmd/polymarket-paper/ --db portfolio

# Resolve outcomes
go run ./cmd/outcome-resolver/
```

### Via Dashboard
Navigate to the Polymarket page in the dashboard. Use the trade form to buy/sell positions. View portfolio, predictions, and performance metrics.

### Via API
```bash
# List markets
curl http://localhost:8082/api/v1/polymarket/markets

# Execute paper trade
curl -X POST http://localhost:8082/api/v1/polymarket/trade \
  -H 'Content-Type: application/json' \
  -d '{"market_id":"...","side":"YES","amount":50,"price":0.65}'

# View portfolio
curl http://localhost:8082/api/v1/polymarket/portfolio
```
